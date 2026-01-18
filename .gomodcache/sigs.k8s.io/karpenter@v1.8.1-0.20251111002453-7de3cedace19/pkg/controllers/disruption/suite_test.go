/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package disruption_test

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"

	pscheduling "sigs.k8s.io/karpenter/pkg/controllers/provisioning/scheduling"

	"sigs.k8s.io/karpenter/pkg/metrics"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	clock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coreapis "sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/disruption"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/controllers/state/informer"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	disruptionutils "sigs.k8s.io/karpenter/pkg/utils/disruption"
	"sigs.k8s.io/karpenter/pkg/utils/pdb"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var env *test.Environment
var cluster *state.Cluster
var disruptionController *disruption.Controller
var prov *provisioning.Provisioner
var cloudProvider *fake.CloudProvider
var nodeStateController *informer.NodeController
var nodeClaimStateController *informer.NodeClaimController
var fakeClock *clock.FakeClock
var recorder *test.EventRecorder
var queue *disruption.Queue
var allKnownDisruptionReasons []v1.DisruptionReason

var onDemandInstances []*cloudprovider.InstanceType
var spotInstances []*cloudprovider.InstanceType
var leastExpensiveInstance, mostExpensiveInstance *cloudprovider.InstanceType
var leastExpensiveOffering, mostExpensiveOffering *cloudprovider.Offering
var leastExpensiveSpotInstance, mostExpensiveSpotInstance *cloudprovider.InstanceType
var leastExpensiveSpotOffering, mostExpensiveSpotOffering *cloudprovider.Offering

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Disruption")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(test.WithCRDs(coreapis.CRDs...), test.WithCRDs(v1alpha1.CRDs...))
	ctx = options.ToContext(ctx, test.Options())
	cloudProvider = fake.NewCloudProvider()
	fakeClock = clock.NewFakeClock(time.Now())
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	nodeStateController = informer.NewNodeController(env.Client, cluster)
	nodeClaimStateController = informer.NewNodeClaimController(env.Client, cloudProvider, cluster)
	recorder = test.NewEventRecorder()
	prov = provisioning.NewProvisioner(env.Client, recorder, cloudProvider, cluster, fakeClock)
	queue = disruption.NewQueue(env.Client, recorder, cluster, fakeClock, prov)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	cloudProvider.Reset()
	cloudProvider.InstanceTypes = fake.InstanceTypesAssorted()

	recorder.Reset() // Reset the events that we captured during the run

	// Ensure that we reset the disruption controller's methods after each test run
	disruptionController = disruption.NewController(fakeClock, env.Client, prov, cloudProvider, recorder, cluster, queue, disruption.WithMethods(NewMethodsWithNopValidator()...))
	fakeClock.SetTime(time.Now())
	cluster.Reset()
	*queue = lo.FromPtr(disruption.NewQueue(env.Client, recorder, cluster, fakeClock, prov))
	cluster.MarkUnconsolidated()

	// Reset Feature Flags to test defaults
	ctx = options.ToContext(ctx, test.Options())

	onDemandInstances = lo.Filter(cloudProvider.InstanceTypes, func(i *cloudprovider.InstanceType, _ int) bool {
		for _, o := range i.Offerings.Available() {
			if o.Requirements.Get(v1.CapacityTypeLabelKey).Any() == v1.CapacityTypeOnDemand {
				return true
			}
		}
		return false
	})
	// Sort the on-demand instances by pricing from low to high
	sort.Slice(onDemandInstances, func(i, j int) bool {
		return onDemandInstances[i].Offerings.Cheapest().Price < onDemandInstances[j].Offerings.Cheapest().Price
	})
	leastExpensiveInstance, mostExpensiveInstance = onDemandInstances[0], onDemandInstances[len(onDemandInstances)-1]
	leastExpensiveOffering, mostExpensiveOffering = leastExpensiveInstance.Offerings[0], mostExpensiveInstance.Offerings[0]
	spotInstances = lo.Filter(cloudProvider.InstanceTypes, func(i *cloudprovider.InstanceType, _ int) bool {
		for _, o := range i.Offerings.Available() {
			if o.Requirements.Get(v1.CapacityTypeLabelKey).Any() == v1.CapacityTypeSpot {
				return true
			}
		}
		return false
	})
	// Sort the spot instances by pricing from low to high
	sort.Slice(spotInstances, func(i, j int) bool {
		return spotInstances[i].Offerings.Cheapest().Price < spotInstances[j].Offerings.Cheapest().Price
	})
	leastExpensiveSpotInstance, mostExpensiveSpotInstance = spotInstances[0], spotInstances[len(spotInstances)-1]
	leastExpensiveSpotOffering, mostExpensiveSpotOffering = leastExpensiveSpotInstance.Offerings[0], mostExpensiveSpotInstance.Offerings[0]
	allKnownDisruptionReasons = []v1.DisruptionReason{
		v1.DisruptionReasonEmpty,
		v1.DisruptionReasonUnderutilized,
		v1.DisruptionReasonDrifted,
	}
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)

	// Reset the metrics collectors
	disruption.DecisionsPerformedTotal.Reset()
})

var _ = Describe("Simulate Scheduling", func() {
	var nodePool *v1.NodePool
	BeforeEach(func() {
		nodePool = test.NodePool(v1.NodePool{
			Spec: v1.NodePoolSpec{
				Disruption: v1.Disruption{
					ConsolidateAfter:    v1.MustParseNillableDuration("0s"),
					ConsolidationPolicy: v1.ConsolidationPolicyWhenEmptyOrUnderutilized,
				},
			},
		})
	})
	It("should allow pods on deleting nodes to reschedule to uninitialized nodes", func() {
		numNodes := 10
		nodeClaims, nodes := test.NodeClaimsAndNodes(numNodes, v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"karpenter.sh/test-finalizer"},
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
			Status: v1.NodeClaimStatus{
				Allocatable: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:  resource.MustParse("3"),
					corev1.ResourcePods: resource.MustParse("100"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool)

		for i := 0; i < numNodes; i++ {
			ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
		}
		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)
		pod := test.Pod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					// 2 cpu so each node can only fit one pod.
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				},
			},
		})
		nodePool.Spec.Disruption.ConsolidateAfter = v1.MustParseNillableDuration("Never")
		ExpectApplied(ctx, env.Client, pod)
		ExpectManualBinding(ctx, env.Client, pod, nodes[0])

		nodePoolMap, nodePoolToInstanceTypesMap, err := disruption.BuildNodePoolMap(ctx, env.Client, cloudProvider)
		Expect(err).To(Succeed())

		// Mark all nodeclaims as marked for deletion
		for i, nc := range nodeClaims {
			ExpectReconcileSucceeded(ctx, nodeClaimStateController, client.ObjectKeyFromObject(nc))
			cluster.MarkForDeletion(nodeClaims[i].Status.ProviderID)
		}
		cluster.UnmarkForDeletion(nodeClaims[0].Status.ProviderID)
		// Mark all nodes as marked for deletion
		for _, n := range nodes {
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(n))
		}

		pdbs, err := pdb.NewLimits(ctx, env.Client)
		Expect(err).To(Succeed())

		// Generate a candidate
		stateNode := ExpectStateNodeExists(cluster, nodes[0])
		candidate, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, stateNode, pdbs, nodePoolMap, nodePoolToInstanceTypesMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(Succeed())

		results, err := disruption.SimulateScheduling(ctx, env.Client, cluster, prov, candidate)
		Expect(err).To(Succeed())
		Expect(results.PodErrors[pod]).To(BeNil())
	})
	It("should allow multiple replace operations to happen successively", func() {
		numNodes := 10
		nodeClaims, nodes := test.NodeClaimsAndNodes(numNodes, v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"karpenter.sh/test-finalizer"},
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
			Spec: v1.NodeClaimSpec{
				ExpireAfter: v1.MustParseNillableDuration("5m"),
			},
			Status: v1.NodeClaimStatus{
				Allocatable: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:  resource.MustParse("3"),
					corev1.ResourcePods: resource.MustParse("100"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool)

		for i := 0; i < numNodes; i++ {
			ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
		}

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)

		// Create a pod for each node
		pods := test.Pods(10, test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				},
			},
		})
		// Set a partition so that each node pool fits one node
		nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, v1.NodeSelectorRequirementWithMinValues{
			NodeSelectorRequirement: corev1.NodeSelectorRequirement{
				Key:      "test-partition",
				Operator: corev1.NodeSelectorOpExists,
			},
		})

		nodePool.Spec.Disruption.ConsolidateAfter = v1.MustParseNillableDuration("Never")
		nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "3"}}
		ExpectApplied(ctx, env.Client, nodePool)

		// Mark all nodeclaims as drifted
		for _, nc := range nodeClaims {
			nc.StatusConditions().SetTrue(v1.ConditionTypeDrifted)
			ExpectApplied(ctx, env.Client, nc)
			ExpectReconcileSucceeded(ctx, nodeClaimStateController, client.ObjectKeyFromObject(nc))
		}
		// Add a partition label into each node so we have 10 distinct scheduling requiments for each pod/node pair
		for i, n := range nodes {
			n.Labels = lo.Assign(n.Labels, map[string]string{"test-partition": fmt.Sprintf("%d", i)})
			ExpectApplied(ctx, env.Client, n)
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(n))
		}

		for i := range pods {
			pods[i].Spec.NodeSelector = lo.Assign(pods[i].Spec.NodeSelector, map[string]string{"test-partition": fmt.Sprintf("%d", i)})
			ExpectApplied(ctx, env.Client, pods[i])
			ExpectManualBinding(ctx, env.Client, pods[i], nodes[i])
		}

		// Get a set of the node claim names so that it's easy to check if a new one is made
		nodeClaimNames := sets.New(lo.Map(nodeClaims, func(nc *v1.NodeClaim, _ int) string { return nc.Name })...)
		ExpectSingletonReconciled(ctx, disruptionController)

		// Expect a replace action
		ExpectTaintedNodeCount(ctx, env.Client, 1)
		ncs := ExpectNodeClaims(ctx, env.Client)
		// which would create one more node claim
		Expect(len(ncs)).To(Equal(11))
		nc, ok := lo.Find(ncs, func(nc *v1.NodeClaim) bool {
			return !nodeClaimNames.Has(nc.Name)
		})
		Expect(ok).To(BeTrue())
		// which needs to be deployed
		ExpectNodeClaimDeployedAndStateUpdated(ctx, env.Client, cluster, cloudProvider, nc)
		nodeClaimNames[nc.Name] = struct{}{}
		ExpectSingletonReconciled(ctx, disruptionController)

		// Another replacement disruption action
		ncs = ExpectNodeClaims(ctx, env.Client)
		Expect(len(ncs)).To(Equal(12))
		nc, ok = lo.Find(ncs, func(nc *v1.NodeClaim) bool {
			return !nodeClaimNames.Has(nc.Name)
		})
		Expect(ok).To(BeTrue())
		ExpectNodeClaimDeployedAndStateUpdated(ctx, env.Client, cluster, cloudProvider, nc)
		nodeClaimNames[nc.Name] = struct{}{}

		ExpectSingletonReconciled(ctx, disruptionController)

		// One more replacement disruption action
		ncs = ExpectNodeClaims(ctx, env.Client)
		Expect(len(ncs)).To(Equal(13))
		nc, ok = lo.Find(ncs, func(nc *v1.NodeClaim) bool {
			return !nodeClaimNames.Has(nc.Name)
		})
		Expect(ok).To(BeTrue())
		ExpectNodeClaimDeployedAndStateUpdated(ctx, env.Client, cluster, cloudProvider, nc)
		nodeClaimNames[nc.Name] = struct{}{}

		// Try one more time, but fail since the budgets only allow 3 disruptions.
		ExpectSingletonReconciled(ctx, disruptionController)

		ncs = ExpectNodeClaims(ctx, env.Client)
		Expect(len(ncs)).To(Equal(13))
	})
	It("can replace node with a local PV (ignoring hostname affinity)", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
			Status: v1.NodeClaimStatus{
				Allocatable: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:  resource.MustParse("32"),
					corev1.ResourcePods: resource.MustParse("100"),
				},
			},
		})
		nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
		labels := map[string]string{
			"app": "test",
		}
		// create our RS so we can link a pod to it
		ss := test.StatefulSet()
		ExpectApplied(ctx, env.Client, ss)

		// StorageClass that references "no-provisioner" and is used for local volume storage
		storageClass := test.StorageClass(test.StorageClassOptions{
			ObjectMeta: metav1.ObjectMeta{
				Name: "local-path",
			},
			Provisioner: lo.ToPtr("kubernetes.io/no-provisioner"),
		})
		persistentVolume := test.PersistentVolume(test.PersistentVolumeOptions{UseLocal: true})
		persistentVolume.Spec.NodeAffinity = &corev1.VolumeNodeAffinity{
			Required: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						// This PV is only valid for use against this node
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      corev1.LabelHostname,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{node.Name},
							},
						},
					},
				},
			},
		}
		persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{VolumeName: persistentVolume.Name, StorageClassName: &storageClass.Name})
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: labels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "StatefulSet",
						Name:               ss.Name,
						UID:                ss.UID,
						Controller:         lo.ToPtr(true),
						BlockOwnerDeletion: lo.ToPtr(true),
					},
				},
			},
			PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
		})
		ExpectApplied(ctx, env.Client, ss, pod, nodeClaim, node, nodePool, storageClass, persistentVolume, persistentVolumeClaim)

		// bind pods to node
		ExpectManualBinding(ctx, env.Client, pod, node)

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		ExpectSingletonReconciled(ctx, disruptionController)

		// disruption won't delete the old node until the new node is ready
		Expect(queue.GetCommands()).To(HaveLen(1))
		ExpectMakeNewNodeClaimsReady(ctx, env.Client, cluster, cloudProvider, queue.GetCommands()[0])

		// Process the item so that the nodes can be deleted.
		ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)
		// Cascade any deletion of the nodeClaim to the node
		ExpectNodeClaimsCascadeDeletion(ctx, env.Client, nodeClaim)

		// Expect that the new nodeClaim was created, and it's different than the original
		// We should succeed in getting a replacement, since we assume that the node affinity requirement will be invalid
		// once we spin-down the old node
		ExpectNotFound(ctx, env.Client, nodeClaim, node)
		nodeclaims := ExpectNodeClaims(ctx, env.Client)
		nodes := ExpectNodes(ctx, env.Client)
		Expect(nodeclaims).To(HaveLen(1))
		Expect(nodes).To(HaveLen(1))
		Expect(nodeclaims[0].Name).ToNot(Equal(nodeClaim.Name))
		Expect(nodes[0].Name).ToNot(Equal(node.Name))
	})
	It("should ensure that we do not duplicate capacity for disrupted nodes with provisioning", func() {
		// We create a client that hangs Create() so that when we try to create replacements
		// we give ourselves time to check that we wouldn't provision additional capacity before the replacements are made
		hangCreateClient := newHangCreateClient(env.Client)
		defer hangCreateClient.Stop()

		p := provisioning.NewProvisioner(hangCreateClient, recorder, cloudProvider, cluster, fakeClock)
		q := disruption.NewQueue(hangCreateClient, recorder, cluster, fakeClock, p)
		dc := disruption.NewController(fakeClock, hangCreateClient, p, cloudProvider, recorder, cluster, q)

		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
			Status: v1.NodeClaimStatus{
				ProviderID: test.RandomProviderID(),
				Allocatable: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:  resource.MustParse("32"),
					corev1.ResourcePods: resource.MustParse("100"),
				},
			},
		})
		nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeDrifted)
		labels := map[string]string{
			"app": "test",
		}
		// create our RS so we can link a pod to it
		rs := test.ReplicaSet()
		ExpectApplied(ctx, env.Client, rs)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: labels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "ReplicaSet",
						Name:               rs.Name,
						UID:                rs.UID,
						Controller:         lo.ToPtr(true),
						BlockOwnerDeletion: lo.ToPtr(true),
					},
				}}})

		ExpectApplied(ctx, env.Client, rs, pod, nodeClaim, node, nodePool)

		// bind the pods to the node
		ExpectManualBinding(ctx, env.Client, pod, node)

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		// Expect the disruption controller to attempt to create a replacement and hang creation when we try to create the replacement
		go ExpectSingletonReconciled(ctx, dc)
		Eventually(func(g Gomega) {
			g.Expect(hangCreateClient.HasWaiter()).To(BeTrue())
		}, time.Second*5).Should(Succeed())

		// If our code works correctly, the provisioner should not try to create a new NodeClaim since we shouldn't have marked
		// our nodes for disruption until the new NodeClaims have been successfully launched
		results, err := prov.Schedule(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(results.NewNodeClaims).To(BeEmpty())
	})
})

var _ = Describe("Disruption Taints", func() {
	var nodePool *v1.NodePool
	var nodeClaim *v1.NodeClaim
	var node *corev1.Node
	BeforeEach(func() {
		currentInstance := fake.NewInstanceType(fake.InstanceTypeOptions{
			Name: "current-on-demand",
			Offerings: []*cloudprovider.Offering{
				{
					Available:    false,
					Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeOnDemand, corev1.LabelTopologyZone: "test-zone-1a"}),
					Price:        1.5,
				},
			},
		})
		replacementInstance := fake.NewInstanceType(fake.InstanceTypeOptions{
			Name: "spot-replacement",
			Offerings: []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1a"}),
					Price:        1.0,
				},
				{
					Available:    true,
					Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1b"}),
					Price:        0.2,
				},
				{
					Available:    true,
					Requirements: scheduling.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1c"}),
					Price:        0.4,
				},
			},
		})
		nodePool = test.NodePool()
		nodeClaim, node = test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					corev1.LabelInstanceTypeStable: currentInstance.Name,
					v1.CapacityTypeLabelKey:        currentInstance.Offerings[0].Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       currentInstance.Offerings[0].Requirements.Get(corev1.LabelTopologyZone).Any(),
					v1.NodePoolLabelKey:            nodePool.Name,
				},
			},
			Status: v1.NodeClaimStatus{
				ProviderID: test.RandomProviderID(),
				Allocatable: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:  resource.MustParse("32"),
					corev1.ResourcePods: resource.MustParse("100"),
				},
			},
		})
		cloudProvider.InstanceTypes = []*cloudprovider.InstanceType{
			currentInstance,
			replacementInstance,
		}
		nodePool.Spec.Disruption.ConsolidateAfter.Duration = lo.ToPtr(time.Duration(0))
		nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
		ExpectApplied(ctx, env.Client, nodeClaim, nodePool)
	})
	It("should remove taints from NodeClaims that were left tainted from a previous disruption action", func() {
		pod := test.Pod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				},
			},
		})
		nodePool.Spec.Disruption.ConsolidateAfter = v1.MustParseNillableDuration("Never")
		node.Spec.Taints = append(node.Spec.Taints, v1.DisruptedNoScheduleTaint)
		nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeDisruptionReason)
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod)
		ExpectManualBinding(ctx, env.Client, pod, node)

		// inform cluster state about nodes and nodeClaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
		ExpectSingletonReconciled(ctx, disruptionController)
		node = ExpectNodeExists(ctx, env.Client, node.Name)
		Expect(node.Spec.Taints).ToNot(ContainElement(v1.DisruptedNoScheduleTaint))

		nodeClaims := lo.Filter(ExpectNodeClaims(ctx, env.Client), func(nc *v1.NodeClaim, _ int) bool {
			return nc.Status.ProviderID == node.Spec.ProviderID
		})
		Expect(nodeClaims).To(HaveLen(1))
		Expect(nodeClaims[0].StatusConditions().Get(v1.ConditionTypeDisruptionReason)).To(BeNil())
	})
	It("should add and remove taints from NodeClaims that fail to disrupt", func() {
		nodePool.Spec.Disruption.ConsolidationPolicy = v1.ConsolidationPolicyWhenEmptyOrUnderutilized
		pod := test.Pod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod)
		ExpectManualBinding(ctx, env.Client, pod, node)

		// inform cluster state about nodes and nodeClaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		ExpectSingletonReconciled(ctx, disruptionController)

		// Process the item so that the nodes can be deleted.
		cmds := queue.GetCommands()
		Expect(cmds).To(HaveLen(1))

		node = ExpectNodeExists(ctx, env.Client, node.Name)
		Expect(node.Spec.Taints).To(ContainElement(v1.DisruptedNoScheduleTaint))
		nodeClaims := lo.Filter(ExpectNodeClaims(ctx, env.Client), func(nc *v1.NodeClaim, _ int) bool {
			return nc.Status.ProviderID == node.Spec.ProviderID
		})
		Expect(nodeClaims).To(HaveLen(1))
		Expect(nodeClaims[0].StatusConditions().Get(v1.ConditionTypeDisruptionReason)).ToNot(BeNil())
		Expect(nodeClaims[0].StatusConditions().Get(v1.ConditionTypeDisruptionReason).IsTrue()).To(BeTrue())

		createdNodeClaim := lo.Reject(ExpectNodeClaims(ctx, env.Client), func(nc *v1.NodeClaim, _ int) bool {
			return nc.Name == nodeClaim.Name
		})
		ExpectDeleted(ctx, env.Client, createdNodeClaim[0])
		ExpectNodeClaimsCascadeDeletion(ctx, env.Client, createdNodeClaim[0])
		ExpectNotFound(ctx, env.Client, createdNodeClaim[0])
		cluster.DeleteNodeClaim(createdNodeClaim[0].Name)

		ExpectObjectReconciled(ctx, env.Client, queue, nodeClaim)

		node = ExpectNodeExists(ctx, env.Client, node.Name)
		Expect(node.Spec.Taints).ToNot(ContainElement(v1.DisruptedNoScheduleTaint))

		nodeClaims = lo.Filter(ExpectNodeClaims(ctx, env.Client), func(nc *v1.NodeClaim, _ int) bool {
			return nc.Status.ProviderID == node.Spec.ProviderID
		})
		Expect(nodeClaims).To(HaveLen(1))
		Expect(nodeClaims[0].StatusConditions().Get(v1.ConditionTypeDisruptionReason)).To(BeNil())
	})
})

var _ = Describe("BuildDisruptionBudgetMapping", func() {
	var nodePool *v1.NodePool
	var nodeClaims []*v1.NodeClaim
	var nodes []*corev1.Node
	var numNodes int
	BeforeEach(func() {
		numNodes = 10
		nodePool = test.NodePool()
		nodeClaims, nodes = test.NodeClaimsAndNodes(numNodes, v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"karpenter.sh/test-finalizer"},
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
			Status: v1.NodeClaimStatus{
				Allocatable: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:  resource.MustParse("32"),
					corev1.ResourcePods: resource.MustParse("100"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool)

		for i := 0; i < numNodes; i++ {
			ExpectApplied(ctx, env.Client, nodeClaims[i], nodes[i])
		}

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nodes, nodeClaims)
	})
	It("should not consider nodes that are not managed as part of disruption count", func() {
		nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "100%"}}
		ExpectApplied(ctx, env.Client, nodePool)
		unmanaged := test.Node()
		ExpectApplied(ctx, env.Client, unmanaged)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{unmanaged}, []*v1.NodeClaim{})
		for _, reason := range allKnownDisruptionReasons {
			budgets, err := disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, reason)
			Expect(err).To(Succeed())
			// This should not bring in the unmanaged node.
			Expect(budgets[nodePool.Name]).To(Equal(10))
		}
	})
	It("should not consider nodes that are not initialized as part of disruption count", func() {
		nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "100%"}}
		ExpectApplied(ctx, env.Client, nodePool)
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"karpenter.sh/test-finalizer"},
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
			Status: v1.NodeClaimStatus{
				Allocatable: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:  resource.MustParse("32"),
					corev1.ResourcePods: resource.MustParse("100"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodeClaim, node)
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, nodeClaimStateController, client.ObjectKeyFromObject(nodeClaim))

		for _, reason := range allKnownDisruptionReasons {
			budgets, err := disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, reason)
			Expect(err).To(Succeed())
			// This should not bring in the uninitialized node.
			Expect(budgets[nodePool.Name]).To(Equal(10))
		}
	})
	It("should not consider nodes that have the terminating status condition as part of disruption count", func() {
		nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "100%"}}
		ExpectApplied(ctx, env.Client, nodePool)
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"karpenter.sh/test-finalizer"},
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
			Status: v1.NodeClaimStatus{
				Allocatable: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:  resource.MustParse("32"),
					corev1.ResourcePods: resource.MustParse("100"),
				},
			},
		})
		nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeInstanceTerminating)
		ExpectApplied(ctx, env.Client, nodeClaim, node)
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, nodeClaimStateController, client.ObjectKeyFromObject(nodeClaim))

		for _, reason := range allKnownDisruptionReasons {
			budgets, err := disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, reason)
			Expect(err).To(Succeed())
			// This should not bring in the terminating node.
			Expect(budgets[nodePool.Name]).To(Equal(10))
		}
	})
	It("should not return a negative disruption value", func() {
		nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "10%"}}
		ExpectApplied(ctx, env.Client, nodePool)

		// Mark all nodeclaims as marked for deletion
		for _, i := range nodeClaims {
			Expect(env.Client.Delete(ctx, i)).To(Succeed())
			ExpectReconcileSucceeded(ctx, nodeClaimStateController, client.ObjectKeyFromObject(i))
		}
		// Mark all nodes as marked for deletion
		for _, i := range nodes {
			Expect(env.Client.Delete(ctx, i)).To(Succeed())
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(i))
		}

		for _, reason := range allKnownDisruptionReasons {
			budgets, err := disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, reason)
			Expect(err).To(Succeed())
			Expect(budgets[nodePool.Name]).To(Equal(0))
		}
	})
	It("should consider nodes with a deletion timestamp set and MarkedForDeletion to the disruption count", func() {
		nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "100%"}}
		ExpectApplied(ctx, env.Client, nodePool)

		// Delete one node and nodeclaim
		Expect(env.Client.Delete(ctx, nodeClaims[0])).To(Succeed())
		Expect(env.Client.Delete(ctx, nodes[0])).To(Succeed())
		cluster.MarkForDeletion(nodeClaims[1].Status.ProviderID)

		// Mark all nodeclaims as marked for deletion
		for _, i := range nodeClaims {
			ExpectReconcileSucceeded(ctx, nodeClaimStateController, client.ObjectKeyFromObject(i))
		}
		// Mark all nodes as marked for deletion
		for _, i := range nodes {
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(i))
		}

		for _, reason := range allKnownDisruptionReasons {
			budgets, err := disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, reason)
			Expect(err).To(Succeed())
			Expect(budgets[nodePool.Name]).To(Equal(8))
		}
	})
	It("should consider not ready nodes to the disruption count", func() {
		nodePool.Spec.Disruption.Budgets = []v1.Budget{{Nodes: "100%"}}
		ExpectApplied(ctx, env.Client, nodePool)

		ExpectMakeNodesNotReady(ctx, env.Client, nodes[0], nodes[1])

		// Mark all nodeclaims as marked for deletion
		for _, i := range nodeClaims {
			ExpectReconcileSucceeded(ctx, nodeClaimStateController, client.ObjectKeyFromObject(i))
		}
		// Mark all nodes as marked for deletion
		for _, i := range nodes {
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(i))
		}

		for _, reason := range allKnownDisruptionReasons {
			budgets, err := disruption.BuildDisruptionBudgetMapping(ctx, cluster, fakeClock, env.Client, cloudProvider, recorder, reason)
			Expect(err).To(Succeed())
			Expect(budgets[nodePool.Name]).To(Equal(8))
		}
	})
})

var _ = Describe("Pod Eviction Cost", func() {
	const standardPodCost = 1.0
	It("should have a standard disruptionCost for a pod with no priority or disruptionCost specified", func() {
		cost := disruptionutils.EvictionCost(ctx, &corev1.Pod{})
		Expect(cost).To(BeNumerically("==", standardPodCost))
	})
	It("should have a higher disruptionCost for a pod with a positive deletion disruptionCost", func() {
		cost := disruptionutils.EvictionCost(ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				corev1.PodDeletionCost: "100",
			}},
		})
		Expect(cost).To(BeNumerically(">", standardPodCost))
	})
	It("should have a lower disruptionCost for a pod with a positive deletion disruptionCost", func() {
		cost := disruptionutils.EvictionCost(ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				corev1.PodDeletionCost: "-100",
			}},
		})
		Expect(cost).To(BeNumerically("<", standardPodCost))
	})
	It("should have higher costs for higher deletion costs", func() {
		cost1 := disruptionutils.EvictionCost(ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				corev1.PodDeletionCost: "101",
			}},
		})
		cost2 := disruptionutils.EvictionCost(ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				corev1.PodDeletionCost: "100",
			}},
		})
		cost3 := disruptionutils.EvictionCost(ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				corev1.PodDeletionCost: "99",
			}},
		})
		Expect(cost1).To(BeNumerically(">", cost2))
		Expect(cost2).To(BeNumerically(">", cost3))
	})
	It("should have a higher disruptionCost for a pod with a higher priority", func() {
		cost := disruptionutils.EvictionCost(ctx, &corev1.Pod{
			Spec: corev1.PodSpec{Priority: lo.ToPtr(int32(1))},
		})
		Expect(cost).To(BeNumerically(">", standardPodCost))
	})
	It("should have a lower disruptionCost for a pod with a lower priority", func() {
		cost := disruptionutils.EvictionCost(ctx, &corev1.Pod{
			Spec: corev1.PodSpec{Priority: lo.ToPtr(int32(-1))},
		})
		Expect(cost).To(BeNumerically("<", standardPodCost))
	})
})

var _ = Describe("Candidate Filtering", func() {
	var nodePool *v1.NodePool
	var nodePoolMap map[string]*v1.NodePool
	var nodePoolInstanceTypeMap map[string]map[string]*cloudprovider.InstanceType
	var pdbLimits pdb.Limits
	BeforeEach(func() {
		nodePool = test.NodePool()
		nodePoolMap = map[string]*v1.NodePool{
			nodePool.Name: nodePool,
		}
		nodePoolInstanceTypeMap = map[string]map[string]*cloudprovider.InstanceType{
			nodePool.Name: lo.SliceToMap(cloudProvider.InstanceTypes, func(i *cloudprovider.InstanceType) (string, *cloudprovider.InstanceType) {
				return i.Name, i
			}),
		}
		var err error
		pdbLimits, err = pdb.NewLimits(ctx, env.Client)
		Expect(err).ToNot(HaveOccurred())
	})
	It("should not consider candidates that have do-not-disrupt pods scheduled and no terminationGracePeriod", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1.DoNotDisruptAnnotationKey: "true",
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod)
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf(`pod has "karpenter.sh/do-not-disrupt" annotation (Pod=%s)`, client.ObjectKeyFromObject(pod))))
		Expect(recorder.DetectedEvent(fmt.Sprintf(`Pod has "karpenter.sh/do-not-disrupt" annotation (Pod=%s)`, client.ObjectKeyFromObject(pod)))).To(BeTrue())
	})
	It("should not consider candidates that have do-not-disrupt mirror pods scheduled", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1.DoNotDisruptAnnotationKey: "true",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Node",
						Name:       node.Name,
						UID:        node.UID,
						Controller: lo.ToPtr(true),
					},
				},
			},
		})
		ExpectApplied(ctx, env.Client, pod)
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf(`pod has "karpenter.sh/do-not-disrupt" annotation (Pod=%s)`, client.ObjectKeyFromObject(pod))))
		Expect(recorder.DetectedEvent(fmt.Sprintf(`Pod has "karpenter.sh/do-not-disrupt" annotation (Pod=%s)`, client.ObjectKeyFromObject(pod)))).To(BeTrue())
	})
	It("should not consider candidates that have do-not-disrupt daemonset pods scheduled", func() {
		daemonSet := test.DaemonSet()
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, daemonSet)
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1.DoNotDisruptAnnotationKey: "true",
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "DaemonSet",
						Name:       daemonSet.Name,
						UID:        daemonSet.UID,
						Controller: lo.ToPtr(true),
					},
				},
			},
		})
		ExpectApplied(ctx, env.Client, pod)
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf(`pod has "karpenter.sh/do-not-disrupt" annotation (Pod=%s)`, client.ObjectKeyFromObject(pod))))
		Expect(recorder.DetectedEvent(fmt.Sprintf(`Pod has "karpenter.sh/do-not-disrupt" annotation (Pod=%s)`, client.ObjectKeyFromObject(pod)))).To(BeTrue())
	})
	It("should consider candidates that have do-not-disrupt pods scheduled with a terminationGracePeriod set for eventual disruption", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		nodeClaim.Spec.TerminationGracePeriod = &metav1.Duration{Duration: time.Second * 300}
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1.DoNotDisruptAnnotationKey: "true",
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod)
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		c, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.EventualDisruptionClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(c.NodeClaim).ToNot(BeNil())
		Expect(c.Node).ToNot(BeNil())
	})
	It("should consider candidates that have PDB-blocked pods scheduled with a terminationGracePeriod set for eventual disruption", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		nodeClaim.Spec.TerminationGracePeriod = &metav1.Duration{Duration: time.Second * 300}
		podLabels := map[string]string{"test": "value"}
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
		})
		budget := test.PodDisruptionBudget(test.PDBOptions{
			Labels:         podLabels,
			MaxUnavailable: fromInt(0),
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod, budget)
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		c, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.EventualDisruptionClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(c.NodeClaim).ToNot(BeNil())
		Expect(c.Node).ToNot(BeNil())
	})
	It("should not consider candidates that have do-not-disrupt pods scheduled with a terminationGracePeriod set for graceful disruption", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		nodeClaim.Spec.TerminationGracePeriod = &metav1.Duration{Duration: time.Second * 300}
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1.DoNotDisruptAnnotationKey: "true",
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod)
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf(`pod has "karpenter.sh/do-not-disrupt" annotation (Pod=%s)`, client.ObjectKeyFromObject(pod))))
		Expect(recorder.DetectedEvent(fmt.Sprintf(`Pod has "karpenter.sh/do-not-disrupt" annotation (Pod=%s)`, client.ObjectKeyFromObject(pod)))).To(BeTrue())
	})
	It("should not consider candidates that have PDB-blocked pods scheduled with a terminationGracePeriod set for graceful disruption", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		nodeClaim.Spec.TerminationGracePeriod = &metav1.Duration{Duration: time.Second * 300}
		podLabels := map[string]string{"test": "value"}
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
		})
		budget := test.PodDisruptionBudget(test.PDBOptions{
			Labels:         podLabels,
			MaxUnavailable: fromInt(0),
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod, budget)
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		var err error
		pdbLimits, err = pdb.NewLimits(ctx, env.Client)
		Expect(err).ToNot(HaveOccurred())

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err = disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf(`pdb prevents pod evictions (PodDisruptionBudget=[%s])`, client.ObjectKeyFromObject(budget))))
		Expect(recorder.DetectedEvent(fmt.Sprintf(`Pdb prevents pod evictions (PodDisruptionBudget=[%s])`, client.ObjectKeyFromObject(budget)))).To(BeTrue())
	})
	It("should not consider candidates that have do-not-disrupt pods scheduled without a terminationGracePeriod set for eventual disruption", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1.DoNotDisruptAnnotationKey: "true",
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod)
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.EventualDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf(`pod has "karpenter.sh/do-not-disrupt" annotation (Pod=%s)`, client.ObjectKeyFromObject(pod))))
		Expect(recorder.DetectedEvent(fmt.Sprintf(`Pod has "karpenter.sh/do-not-disrupt" annotation (Pod=%s)`, client.ObjectKeyFromObject(pod)))).To(BeTrue())
	})
	It("should not consider candidates that have PDB-blocked pods scheduled without a terminationGracePeriod set for eventual disruption", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		podLabels := map[string]string{"test": "value"}
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
		})
		budget := test.PodDisruptionBudget(test.PDBOptions{
			Labels:         podLabels,
			MaxUnavailable: fromInt(0),
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod, budget)
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		var err error
		pdbLimits, err = pdb.NewLimits(ctx, env.Client)
		Expect(err).ToNot(HaveOccurred())

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err = disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.EventualDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf(`pdb prevents pod evictions (PodDisruptionBudget=[%s])`, client.ObjectKeyFromObject(budget))))
		Expect(recorder.DetectedEvent(fmt.Sprintf(`Pdb prevents pod evictions (PodDisruptionBudget=[%s])`, client.ObjectKeyFromObject(budget)))).To(BeTrue())
	})
	It("should consider candidates that have do-not-disrupt terminating pods", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1.DoNotDisruptAnnotationKey: "true",
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod)
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		ExpectDeletionTimestampSet(ctx, env.Client, pod)

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		c, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(c.NodeClaim).ToNot(BeNil())
		Expect(c.Node).ToNot(BeNil())
	})
	It("should consider candidates that have do-not-disrupt terminal pods", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		podSucceeded := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1.DoNotDisruptAnnotationKey: "true",
				},
			},
			Phase: corev1.PodSucceeded,
		})
		podFailed := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1.DoNotDisruptAnnotationKey: "true",
				},
			},
			Phase: corev1.PodFailed,
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, podSucceeded, podFailed)
		ExpectManualBinding(ctx, env.Client, podSucceeded, node)
		ExpectManualBinding(ctx, env.Client, podFailed, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		c, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(c.NodeClaim).ToNot(BeNil())
		Expect(c.Node).ToNot(BeNil())
	})
	It("should not consider candidates that have do-not-disrupt on nodes", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1.DoNotDisruptAnnotationKey: "true",
				},
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(`disruption is blocked through the "karpenter.sh/do-not-disrupt" annotation`))
		Expect(recorder.DetectedEvent(`Disruption is blocked through the "karpenter.sh/do-not-disrupt" annotation`)).To(BeTrue())
	})
	It("should not consider candidates that have multiple PDBs on the same pod", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		podLabels := map[string]string{"test": "value"}
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
		})
		budget1 := test.PodDisruptionBudget(test.PDBOptions{
			ObjectMeta:     metav1.ObjectMeta{Name: "pdb1"},
			Labels:         podLabels,
			MaxUnavailable: fromInt(0),
		})
		budget2 := test.PodDisruptionBudget(test.PDBOptions{
			ObjectMeta:     metav1.ObjectMeta{Name: "pdb2"},
			Labels:         podLabels,
			MaxUnavailable: fromInt(0),
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod, budget1, budget2)
		ExpectManualBinding(ctx, env.Client, pod, node)

		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		var err error
		pdbLimits, err = pdb.NewLimits(ctx, env.Client)
		Expect(err).ToNot(HaveOccurred())

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err = disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		// Since we don't want to assume the ordering of the PDBs in the message, we validate the primary error message as well as check that both the budgets are in the message.
		Expect(err.Error()).To(ContainSubstring("eviction does not support multiple PDBs"))
		Expect(err.Error()).To(ContainSubstring(client.ObjectKeyFromObject(budget1).String()))
		Expect(err.Error()).To(ContainSubstring(client.ObjectKeyFromObject(budget2).String()))
		e := recorder.Events()
		// Same event is published on both the node and nodeclaim.
		Expect(e).To(HaveLen(2))
		Expect(e[0].Message).To(ContainSubstring("Eviction does not support multiple PDBs"))
		Expect(e[0].Message).To(ContainSubstring(client.ObjectKeyFromObject(budget1).String()))
		Expect(e[0].Message).To(ContainSubstring(client.ObjectKeyFromObject(budget2).String()))
	})
	It("should not consider candidates that have fully blocking PDBs", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		podLabels := map[string]string{"test": "value"}
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
		})
		budget := test.PodDisruptionBudget(test.PDBOptions{
			Labels:         podLabels,
			MaxUnavailable: fromInt(0),
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod, budget)
		ExpectManualBinding(ctx, env.Client, pod, node)

		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		var err error
		pdbLimits, err = pdb.NewLimits(ctx, env.Client)
		Expect(err).ToNot(HaveOccurred())

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err = disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf(`pdb prevents pod evictions (PodDisruptionBudget=[%s])`, client.ObjectKeyFromObject(budget))))
		Expect(recorder.DetectedEvent(fmt.Sprintf(`Pdb prevents pod evictions (PodDisruptionBudget=[%s])`, client.ObjectKeyFromObject(budget)))).To(BeTrue())
	})
	It("should not consider candidates that have fully blocking PDBs on daemonset pods", func() {
		daemonSet := test.DaemonSet()
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, daemonSet)
		podLabels := map[string]string{"test": "value"}
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "DaemonSet",
						Name:       daemonSet.Name,
						UID:        daemonSet.UID,
						Controller: lo.ToPtr(true),
					},
				},
			},
		})
		budget := test.PodDisruptionBudget(test.PDBOptions{
			Labels:         podLabels,
			MaxUnavailable: fromInt(0),
		})
		ExpectApplied(ctx, env.Client, pod, budget)
		ExpectManualBinding(ctx, env.Client, pod, node)

		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		var err error
		pdbLimits, err = pdb.NewLimits(ctx, env.Client)
		Expect(err).ToNot(HaveOccurred())

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err = disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf(`pdb prevents pod evictions (PodDisruptionBudget=[%s])`, client.ObjectKeyFromObject(budget))))
		Expect(recorder.DetectedEvent(fmt.Sprintf(`Pdb prevents pod evictions (PodDisruptionBudget=[%s])`, client.ObjectKeyFromObject(budget)))).To(BeTrue())
	})
	It("should consider candidates that have fully blocking PDBs on mirror pods", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
		podLabels := map[string]string{"test": "value"}
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "v1",
						Kind:       "Node",
						Name:       node.Name,
						UID:        node.UID,
						Controller: lo.ToPtr(true),
					},
				},
			},
		})
		budget := test.PodDisruptionBudget(test.PDBOptions{
			Labels:         podLabels,
			MaxUnavailable: fromInt(0),
		})
		ExpectApplied(ctx, env.Client, pod, budget)
		ExpectManualBinding(ctx, env.Client, pod, node)

		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		var err error
		pdbLimits, err = pdb.NewLimits(ctx, env.Client)
		Expect(err).ToNot(HaveOccurred())

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		c, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(c.NodeClaim).ToNot(BeNil())
		Expect(c.Node).ToNot(BeNil())
	})
	It("should not consider candidates that have do-not-disrupt pods without a terminationGracePeriod set for graceful disruption", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1.DoNotDisruptAnnotationKey: "true",
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod)
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
		var err error
		pdbLimits, err = pdb.NewLimits(ctx, env.Client)
		Expect(err).ToNot(HaveOccurred())

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err = disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf(`pod has "karpenter.sh/do-not-disrupt" annotation (Pod=%s)`, client.ObjectKeyFromObject(pod))))
		Expect(recorder.DetectedEvent(fmt.Sprintf(`Pod has "karpenter.sh/do-not-disrupt" annotation (Pod=%s)`, client.ObjectKeyFromObject(pod)))).To(BeTrue())
	})
	It("should not consider candidates that have fully blocking PDBs without a terminationGracePeriod set for graceful disruption", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		podLabels := map[string]string{"test": "value"}
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
		})
		budget := test.PodDisruptionBudget(test.PDBOptions{
			Labels:         podLabels,
			MaxUnavailable: fromInt(0),
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod, budget)
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
		var err error
		pdbLimits, err = pdb.NewLimits(ctx, env.Client)
		Expect(err).ToNot(HaveOccurred())

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err = disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf(`pdb prevents pod evictions (PodDisruptionBudget=[%s])`, client.ObjectKeyFromObject(budget))))
		Expect(recorder.DetectedEvent(fmt.Sprintf(`Pdb prevents pod evictions (PodDisruptionBudget=[%s])`, client.ObjectKeyFromObject(budget)))).To(BeTrue())
	})
	It("should consider candidates that have fully blocking PDBs on terminal pods", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		podLabels := map[string]string{"test": "value"}
		succeededPod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
			Phase: corev1.PodSucceeded,
		})
		failedPod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
			Phase: corev1.PodFailed,
		})
		budget := test.PodDisruptionBudget(test.PDBOptions{
			Labels:         podLabels,
			MaxUnavailable: fromInt(0),
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, succeededPod, failedPod, budget)
		ExpectManualBinding(ctx, env.Client, succeededPod, node)
		ExpectManualBinding(ctx, env.Client, failedPod, node)

		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		var err error
		pdbLimits, err = pdb.NewLimits(ctx, env.Client)
		Expect(err).ToNot(HaveOccurred())

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		c, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(c.NodeClaim).ToNot(BeNil())
		Expect(c.Node).ToNot(BeNil())
	})
	It("should consider candidates that have fully blocking PDBs on terminating pods", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		podLabels := map[string]string{"test": "value"}
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
		})
		budget := test.PodDisruptionBudget(test.PDBOptions{
			Labels:         podLabels,
			MaxUnavailable: fromInt(0),
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node, pod, budget)
		ExpectManualBinding(ctx, env.Client, pod, node)

		ExpectDeletionTimestampSet(ctx, env.Client, pod)

		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		var err error
		pdbLimits, err = pdb.NewLimits(ctx, env.Client)
		Expect(err).ToNot(HaveOccurred())

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		c, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(c.NodeClaim).ToNot(BeNil())
		Expect(c.Node).ToNot(BeNil())
	})
	It("should not consider candidates that has just a Node representation", func() {
		_, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, nil)

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("node isn't managed by karpenter"))
	})
	It("should not consider candidate that has just a NodeClaim representation", func() {
		nodeClaim, _ := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, nil, []*v1.NodeClaim{nodeClaim})

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("nodeclaim does not have an associated node"))
	})
	It("should not consider candidates that are nominated", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
		cluster.NominateNodeForPod(ctx, node.Spec.ProviderID)

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("node is nominated for a pending pod"))
		Expect(recorder.DetectedEvent("Node is nominated for a pending pod")).To(BeTrue())
	})
	It("should not consider candidates that are deleting", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		ExpectDeletionTimestampSet(ctx, env.Client, nodeClaim)
		ExpectReconcileSucceeded(ctx, nodeClaimStateController, client.ObjectKeyFromObject(nodeClaim))

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("node is deleting or marked for deletion"))
	})
	It("should not consider candidates that are MarkedForDeletion", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		cluster.MarkForDeletion(node.Spec.ProviderID)

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("node is deleting or marked for deletion"))
	})
	It("should not consider candidates that aren't yet initialized", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, nodeClaimStateController, client.ObjectKeyFromObject(nodeClaim))

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("node isn't initialized"))
	})
	It("should not consider candidates that are not owned by a NodePool (no karpenter.sh/nodepool label)", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(`node doesn't have required label (label=karpenter.sh/nodepool)`))
		Expect(recorder.DetectedEvent(`Node doesn't have required label (label=karpenter.sh/nodepool)`)).To(BeTrue())
	})
	It("should not consider candidates that are have a non-existent NodePool", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		// Don't apply the NodePool
		ExpectApplied(ctx, env.Client, nodeClaim, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		// Mock the NodePool not existing by removing it from the nodePool and nodePoolInstanceTypes maps
		delete(nodePoolMap, nodePool.Name)
		delete(nodePoolInstanceTypeMap, nodePool.Name)

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(fmt.Sprintf("nodepool not found (NodePool=%s)", nodePool.Name)))
		Expect(recorder.DetectedEvent(fmt.Sprintf("NodePool not found (NodePool=%s)", nodePool.Name))).To(BeTrue())
	})
	It("should consider candidates that do not have the karpenter.sh/capacity-type label", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).ToNot(HaveOccurred())
	})
	It("should consider candidates that do not have the topology.kubernetes.io/zone label", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).ToNot(HaveOccurred())
	})
	It("should consider candidates that do not have the node.kubernetes.io/instance-type label", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:      nodePool.Name,
					v1.CapacityTypeLabelKey:  mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone: mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).ToNot(HaveOccurred())
	})
	It("should consider candidates that have an instance type that cannot be resolved", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		// Mock the InstanceType not existing by removing it from the nodePoolInstanceTypes map
		delete(nodePoolInstanceTypeMap[nodePool.Name], mostExpensiveInstance.Name)

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).ToNot(HaveOccurred())
	})
	It("should not consider candidates that are actively being processed in the queue", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})

		Expect(cluster.DeepCopyNodes()).To(HaveLen(1))
		cmd := &disruption.Command{Method: disruption.NewDrift(env.Client, cluster, prov, recorder), Results: pscheduling.Results{}, Candidates: []*disruption.Candidate{{StateNode: cluster.DeepCopyNodes()[0], NodePool: nodePool}}, Replacements: nil}
		Expect(queue.StartCommand(ctx, cmd))

		_, err := disruption.NewCandidate(ctx, env.Client, recorder, fakeClock, cluster.DeepCopyNodes()[0], pdbLimits, nodePoolMap, nodePoolInstanceTypeMap, queue, disruption.GracefulDisruptionClass)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("candidate is already being disrupted"))
	})
})

var _ = Describe("Metrics", func() {
	var nodePool *v1.NodePool
	var labels = map[string]string{
		"app": "test",
	}
	var nodeClaims []*v1.NodeClaim
	var nodes []*corev1.Node
	BeforeEach(func() {
		nodePool = test.NodePool(v1.NodePool{
			Spec: v1.NodePoolSpec{
				Disruption: v1.Disruption{
					ConsolidationPolicy: v1.ConsolidationPolicyWhenEmptyOrUnderutilized,
					ConsolidateAfter:    v1.MustParseNillableDuration("0s"),
					// Disrupt away!
					Budgets: []v1.Budget{{
						Nodes: "100%",
					}},
				},
			},
		})
		nodeClaims, nodes = test.NodeClaimsAndNodes(3, v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: leastExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        leastExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       leastExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
			Status: v1.NodeClaimStatus{
				Allocatable: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:  resource.MustParse("32"),
					corev1.ResourcePods: resource.MustParse("100"),
				},
			},
		})
		for _, nc := range nodeClaims {
			nc.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
		}
	})
	It("should fire metrics for single node empty disruption", func() {
		nodeClaim, node := nodeClaims[0], nodes[0]
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim, node)

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
		ExpectSingletonReconciled(ctx, disruptionController)
		ExpectMetricCounterValue(disruption.DecisionsPerformedTotal, 1, map[string]string{
			"decision":          "delete",
			metrics.ReasonLabel: "empty",
		})
	})
	It("should fire metrics for single node delete disruption", func() {
		nodeClaims, nodes = nodeClaims[:2], nodes[:2]
		pods := test.Pods(4, test.PodOptions{})

		// only allow one node to be disruptable
		nodeClaims[0].StatusConditions().SetTrue(v1.ConditionTypeDrifted)
		Expect(nodeClaims[1].StatusConditions().Clear(v1.ConditionTypeConsolidatable)).To(Succeed())

		ExpectApplied(ctx, env.Client, pods[0], pods[1], pods[2], pods[3], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodePool)

		// bind pods to nodes
		ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
		ExpectManualBinding(ctx, env.Client, pods[1], nodes[0])
		ExpectManualBinding(ctx, env.Client, pods[2], nodes[1])
		ExpectManualBinding(ctx, env.Client, pods[3], nodes[1])

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1]})
		ExpectSingletonReconciled(ctx, disruptionController)

		ExpectMetricCounterValue(disruption.DecisionsPerformedTotal, 1, map[string]string{
			"decision":          "delete",
			metrics.ReasonLabel: "drifted",
		})
	})
	It("should fire metrics for single node replace disruption", func() {
		nodeClaim, node := nodeClaims[0], nodes[0]
		nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeDrifted)

		pods := test.Pods(4, test.PodOptions{})

		ExpectApplied(ctx, env.Client, pods[0], pods[1], pods[2], pods[3], nodeClaim, node, nodePool)

		// bind pods to nodes
		ExpectManualBinding(ctx, env.Client, pods[0], node)
		ExpectManualBinding(ctx, env.Client, pods[1], node)
		ExpectManualBinding(ctx, env.Client, pods[2], node)
		ExpectManualBinding(ctx, env.Client, pods[3], node)

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{node}, []*v1.NodeClaim{nodeClaim})
		ExpectSingletonReconciled(ctx, disruptionController)

		ExpectMetricCounterValue(disruption.DecisionsPerformedTotal, 1, map[string]string{
			"decision":          "replace",
			metrics.ReasonLabel: "drifted",
		})
	})
	It("should fire metrics for multi-node empty disruption", func() {
		ExpectApplied(ctx, env.Client, nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodeClaims[2], nodes[2], nodePool)

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1], nodes[2]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1], nodeClaims[2]})
		ExpectSingletonReconciled(ctx, disruptionController)
		ExpectMetricCounterValue(disruption.DecisionsPerformedTotal, 1, map[string]string{
			"decision":           "delete",
			metrics.ReasonLabel:  "empty",
			"consolidation_type": "empty",
		})
	})
	It("should fire metrics for multi-node delete disruption", func() {
		// create our RS so we can link a pod to it
		rs := test.ReplicaSet()
		ExpectApplied(ctx, env.Client, rs)
		pods := test.Pods(4, test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: labels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "ReplicaSet",
						Name:               rs.Name,
						UID:                rs.UID,
						Controller:         lo.ToPtr(true),
						BlockOwnerDeletion: lo.ToPtr(true),
					},
				},
			},
		})

		ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], pods[3], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodeClaims[2], nodes[2], nodePool)

		// bind pods to nodes
		ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
		ExpectManualBinding(ctx, env.Client, pods[1], nodes[1])
		ExpectManualBinding(ctx, env.Client, pods[2], nodes[2])

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1], nodes[2]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1], nodeClaims[2]})
		ExpectSingletonReconciled(ctx, disruptionController)
		ExpectMetricCounterValue(disruption.DecisionsPerformedTotal, 1, map[string]string{
			"decision":           "delete",
			metrics.ReasonLabel:  "underutilized",
			"consolidation_type": "multi",
		})
	})
	It("should fire metrics for multi-node replace disruption", func() {
		nodeClaims, nodes = test.NodeClaimsAndNodes(3, v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
			Status: v1.NodeClaimStatus{
				Allocatable: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:  resource.MustParse("32"),
					corev1.ResourcePods: resource.MustParse("100"),
				},
			},
		})
		for _, nc := range nodeClaims {
			nc.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
		}
		// create our RS so we can link a pod to it
		rs := test.ReplicaSet()
		ExpectApplied(ctx, env.Client, rs)
		pods := test.Pods(4, test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: labels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "ReplicaSet",
						Name:               rs.Name,
						UID:                rs.UID,
						Controller:         lo.ToPtr(true),
						BlockOwnerDeletion: lo.ToPtr(true),
					},
				},
			},
		})

		ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], pods[3], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodeClaims[2], nodes[2], nodePool)

		// bind pods to nodes
		ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
		ExpectManualBinding(ctx, env.Client, pods[1], nodes[1])
		ExpectManualBinding(ctx, env.Client, pods[2], nodes[2])
		ExpectManualBinding(ctx, env.Client, pods[3], nodes[2])

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1], nodes[2]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1], nodeClaims[2]})
		ExpectSingletonReconciled(ctx, disruptionController)
		ExpectMetricCounterValue(disruption.DecisionsPerformedTotal, 1, map[string]string{
			"decision":           "replace",
			metrics.ReasonLabel:  "underutilized",
			"consolidation_type": "multi",
		})
	})
	It("should stop multi-node consolidation after context deadline is reached", func() {
		nodeClaims, nodes = test.NodeClaimsAndNodes(3, v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: mostExpensiveInstance.Name,
					v1.CapacityTypeLabelKey:        mostExpensiveOffering.Requirements.Get(v1.CapacityTypeLabelKey).Any(),
					corev1.LabelTopologyZone:       mostExpensiveOffering.Requirements.Get(corev1.LabelTopologyZone).Any(),
				},
			},
			Status: v1.NodeClaimStatus{
				Allocatable: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:  resource.MustParse("32"),
					corev1.ResourcePods: resource.MustParse("100"),
				},
			},
		})
		for _, nc := range nodeClaims {
			nc.StatusConditions().SetTrue(v1.ConditionTypeConsolidatable)
		}
		// create our RS so we can link a pod to it
		rs := test.ReplicaSet()
		ExpectApplied(ctx, env.Client, rs)
		pods := test.Pods(4, test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: labels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "ReplicaSet",
						Name:               rs.Name,
						UID:                rs.UID,
						Controller:         lo.ToPtr(true),
						BlockOwnerDeletion: lo.ToPtr(true),
					},
				},
			},
		})

		ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], pods[3], nodeClaims[0], nodes[0], nodeClaims[1], nodes[1], nodeClaims[2], nodes[2], nodePool)

		// bind pods to nodes
		ExpectManualBinding(ctx, env.Client, pods[0], nodes[0])
		ExpectManualBinding(ctx, env.Client, pods[1], nodes[1])
		ExpectManualBinding(ctx, env.Client, pods[2], nodes[2])
		ExpectManualBinding(ctx, env.Client, pods[3], nodes[2])

		// inform cluster state about nodes and nodeclaims
		ExpectMakeNodesAndNodeClaimsInitializedAndStateUpdated(ctx, env.Client, nodeStateController, nodeClaimStateController, []*corev1.Node{nodes[0], nodes[1], nodes[2]}, []*v1.NodeClaim{nodeClaims[0], nodeClaims[1], nodeClaims[2]})
		// create timeout in the past
		timeoutCtx, cancel := context.WithTimeout(ctx, -disruption.MultiNodeConsolidationTimeoutDuration)
		defer cancel()

		ExpectSingletonReconciled(timeoutCtx, disruptionController)
		// expect that due to timeout zero nodes were tainted in consolidation
		ExpectTaintedNodeCount(ctx, env.Client, 0)
	})
})

func leastExpensiveInstanceWithZone(zone string) *cloudprovider.InstanceType {
	for _, elem := range onDemandInstances {
		if len(elem.Offerings.Compatible(scheduling.NewRequirements(scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, zone)))) > 0 {
			return elem
		}
	}
	return onDemandInstances[len(onDemandInstances)-1]
}

func mostExpensiveInstanceWithZone(zone string) *cloudprovider.InstanceType {
	for i := len(onDemandInstances) - 1; i >= 0; i-- {
		elem := onDemandInstances[i]
		if len(elem.Offerings.Compatible(scheduling.NewRequirements(scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, zone)))) > 0 {
			return elem
		}
	}
	return onDemandInstances[0]
}

//nolint:unparam
func fromInt(i int32) *intstr.IntOrString {
	v := intstr.FromInt32(i)
	return &v
}

// ExpectTaintedNodeCount will assert the number of nodes and tainted nodes in the cluster and return the tainted nodes.
func ExpectTaintedNodeCount(ctx context.Context, c client.Client, numTainted int) []*corev1.Node {
	GinkgoHelper()
	tainted := lo.Filter(ExpectNodes(ctx, c), func(n *corev1.Node, _ int) bool {
		return lo.Contains(n.Spec.Taints, v1.DisruptedNoScheduleTaint)
	})
	Expect(len(tainted)).To(Equal(numTainted))
	return tainted
}

// ExpectNewNodeClaimsDeleted simulates the nodeClaims being created and then removed, similar to what would happen
// during an ICE error on the created nodeClaim
func ExpectNewNodeClaimsDeleted(ctx context.Context, c client.Client, wg *sync.WaitGroup, numNewNodeClaims int) {
	GinkgoHelper()
	existingNodeClaims := ExpectNodeClaims(ctx, c)
	existingNodeClaimNames := sets.NewString(lo.Map(existingNodeClaims, func(nc *v1.NodeClaim, _ int) string {
		return nc.Name
	})...)

	wg.Add(1)
	go func() {
		GinkgoHelper()
		nodeClaimsDeleted := 0
		ctx, cancel := context.WithTimeout(ctx, time.Second*30) // give up after 30s
		defer GinkgoRecover()
		defer wg.Done()
		defer cancel()
		for {
			select {
			case <-time.After(50 * time.Millisecond):
				nodeClaimList := &v1.NodeClaimList{}
				if err := c.List(ctx, nodeClaimList); err != nil {
					continue
				}
				for i := range nodeClaimList.Items {
					m := &nodeClaimList.Items[i]
					if existingNodeClaimNames.Has(m.Name) {
						continue
					}
					Expect(client.IgnoreNotFound(c.Delete(ctx, m))).To(Succeed())
					nodeClaimsDeleted++
					if nodeClaimsDeleted == numNewNodeClaims {
						return
					}
				}
			case <-ctx.Done():
				Fail(fmt.Sprintf("waiting for nodeclaims to be deleted, %s", ctx.Err()))
			}
		}
	}()
}

func ExpectMakeNewNodeClaimsReady(ctx context.Context, c client.Client, cluster *state.Cluster, cloudProvider cloudprovider.CloudProvider, cmd *disruption.Command) {
	GinkgoHelper()

	for _, replacement := range cmd.Replacements {
		nc := &v1.NodeClaim{}
		Expect(c.Get(ctx, types.NamespacedName{Name: replacement.Name}, nc)).To(Succeed())
		nc, n := ExpectNodeClaimDeployedAndStateUpdated(ctx, c, cluster, cloudProvider, nc)
		ExpectMakeNodeClaimsInitialized(ctx, c, nc)
		ExpectMakeNodesInitialized(ctx, c, n)
	}
}

type hangCreateClient struct {
	client.Client
	hasWaiter atomic.Bool
	stop      chan struct{}
}

func newHangCreateClient(c client.Client) *hangCreateClient {
	return &hangCreateClient{Client: c, stop: make(chan struct{})}
}

func (h *hangCreateClient) HasWaiter() bool {
	return h.hasWaiter.Load()
}

func (h *hangCreateClient) Stop() {
	close(h.stop)
}

func (h *hangCreateClient) Create(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
	h.hasWaiter.Store(true)
	<-h.stop
	h.hasWaiter.Store(false)
	return nil
}
