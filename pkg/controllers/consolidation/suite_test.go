/*
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

package consolidation_test

import (
	"context"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	clock "k8s.io/utils/clock/testing"
	. "knative.dev/pkg/logging/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"

	"github.com/aws/karpenter/pkg/cloudproviders/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudproviders/common/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudproviders/common/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/controllers/consolidation"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/test"
	. "github.com/aws/karpenter/pkg/test/expectations"
)

var ctx context.Context
var env *test.Environment
var cluster *state.Cluster
var controller *consolidation.Controller
var provisioningController *provisioning.Controller
var provisioner *provisioning.Provisioner
var cloudProvider *fake.CloudProvider
var clientSet *kubernetes.Clientset
var recorder *test.Recorder
var nodeStateController *state.NodeController
var fakeClock *clock.FakeClock
var cfg *test.Config
var onDemandInstances []cloudprovider.InstanceType
var mostExpensiveInstance cloudprovider.InstanceType
var mostExpensiveOffering cloudprovider.Offering
var leastExpensiveInstance cloudprovider.InstanceType
var leastExpensiveOffering cloudprovider.Offering

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Consolidation")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		cloudProvider = &fake.CloudProvider{}
		cfg = test.NewConfig()
		fakeClock = clock.NewFakeClock(time.Now())
		cluster = state.NewCluster(fakeClock, cfg, env.Client, cloudProvider)
		nodeStateController = state.NewNodeController(env.Client, cluster)
		clientSet = kubernetes.NewForConfigOrDie(e.Config)
		recorder = test.NewEventRecorder()
		provisioner = provisioning.NewProvisioner(ctx, cfg, env.Client, clientSet.CoreV1(), recorder, cloudProvider, cluster)
		provisioningController = provisioning.NewController(env.Client, provisioner, recorder)
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	cloudProvider.CreateCalls = nil
	cloudProvider.InstanceTypes = fake.InstanceTypesAssorted()
	onDemandInstances = lo.Filter(cloudProvider.InstanceTypes, func(i cloudprovider.InstanceType, _ int) bool {
		for _, o := range cloudprovider.AvailableOfferings(i) {
			if o.CapacityType == v1alpha1.CapacityTypeOnDemand {
				return true
			}
		}
		return false
	})
	// Sort the instances by pricing from low to high
	sort.Slice(onDemandInstances, func(i, j int) bool {
		return cheapestOffering(onDemandInstances[i].Offerings()).Price < cheapestOffering(onDemandInstances[j].Offerings()).Price
	})
	leastExpensiveInstance = onDemandInstances[0]
	leastExpensiveOffering = leastExpensiveInstance.Offerings()[0]
	mostExpensiveInstance = onDemandInstances[len(onDemandInstances)-1]
	mostExpensiveOffering = mostExpensiveInstance.Offerings()[0]

	recorder.Reset()
	fakeClock.SetTime(time.Now())
	controller = consolidation.NewController(fakeClock, env.Client, provisioner, cloudProvider, recorder, cluster)
})
var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
	var nodes []client.ObjectKey
	cluster.ForEachNode(func(n *state.Node) bool {
		nodes = append(nodes, client.ObjectKeyFromObject(n.Node))
		return true
	})

	// inform cluster state of node deletion
	for _, nodeKey := range nodes {
		ExpectReconcileSucceeded(ctx, nodeStateController, nodeKey)
	}
})

var _ = Describe("Pod Eviction Cost", func() {
	const standardPodCost = 1.0
	It("should have a standard disruptionCost for a pod with no priority or disruptionCost specified", func() {
		cost := consolidation.GetPodEvictionCost(ctx, &v1.Pod{})
		Expect(cost).To(BeNumerically("==", standardPodCost))
	})
	It("should have a higher disruptionCost for a pod with a positive deletion disruptionCost", func() {
		cost := consolidation.GetPodEvictionCost(ctx, &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				v1.PodDeletionCost: "100",
			}},
		})
		Expect(cost).To(BeNumerically(">", standardPodCost))
	})
	It("should have a lower disruptionCost for a pod with a positive deletion disruptionCost", func() {
		cost := consolidation.GetPodEvictionCost(ctx, &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				v1.PodDeletionCost: "-100",
			}},
		})
		Expect(cost).To(BeNumerically("<", standardPodCost))
	})
	It("should have higher costs for higher deletion costs", func() {
		cost1 := consolidation.GetPodEvictionCost(ctx, &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				v1.PodDeletionCost: "101",
			}},
		})
		cost2 := consolidation.GetPodEvictionCost(ctx, &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				v1.PodDeletionCost: "100",
			}},
		})
		cost3 := consolidation.GetPodEvictionCost(ctx, &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				v1.PodDeletionCost: "99",
			}},
		})
		Expect(cost1).To(BeNumerically(">", cost2))
		Expect(cost2).To(BeNumerically(">", cost3))
	})
	It("should have a higher disruptionCost for a pod with a higher priority", func() {
		cost := consolidation.GetPodEvictionCost(ctx, &v1.Pod{
			Spec: v1.PodSpec{Priority: aws.Int32(1)},
		})
		Expect(cost).To(BeNumerically(">", standardPodCost))
	})
	It("should have a lower disruptionCost for a pod with a lower priority", func() {
		cost := consolidation.GetPodEvictionCost(ctx, &v1.Pod{
			Spec: v1.PodSpec{Priority: aws.Int32(-1)},
		})
		Expect(cost).To(BeNumerically("<", standardPodCost))
	})
})

var _ = Describe("Replace Nodes", func() {
	It("can replace node", func() {
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
						Controller:         aws.Bool(true),
						BlockOwnerDeletion: aws.Bool(true),
					},
				}}})

		prov := test.Provisioner(test.ProvisionerOptions{Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)}})
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: resource.MustParse("32")}})

		ExpectApplied(ctx, env.Client, rs, pod, node, prov)
		ExpectMakeNodesReady(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node))
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectScheduled(ctx, env.Client, pod)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())

		// consolidation won't delete the old node until the new node is ready
		wg := ExpectMakeNewNodesReady(ctx, env.Client, 1, node)
		fakeClock.Step(10 * time.Minute)
		_, err := controller.ProcessCluster(ctx)
		Expect(err).ToNot(HaveOccurred())
		wg.Wait()

		// should create a new node as there is a cheaper one that can hold the pod
		Expect(cloudProvider.CreateCalls).To(HaveLen(1))
		// and delete the old one
		ExpectNotFound(ctx, env.Client, node)
	})
	It("can replace nodes, considers PDB", func() {
		labels := map[string]string{
			"app": "test",
		}
		// create our RS so we can link a pod to it
		rs := test.ReplicaSet()
		ExpectApplied(ctx, env.Client, rs)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

		pods := test.Pods(3, test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "ReplicaSet",
						Name:               rs.Name,
						UID:                rs.UID,
						Controller:         aws.Bool(true),
						BlockOwnerDeletion: aws.Bool(true),
					},
				}}})

		pdb := test.PodDisruptionBudget(test.PDBOptions{
			Labels:         labels,
			MaxUnavailable: fromInt(0),
			Status: &policyv1.PodDisruptionBudgetStatus{
				ObservedGeneration: 1,
				DisruptionsAllowed: 0,
				CurrentHealthy:     1,
				DesiredHealthy:     1,
				ExpectedPods:       1,
			},
		})

		prov := test.Provisioner(test.ProvisionerOptions{Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)}})
		node1 := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:  resource.MustParse("32"),
				v1.ResourcePods: resource.MustParse("100"),
			}})

		ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], node1, prov, pdb)
		ExpectApplied(ctx, env.Client, node1)
		// all pods on node1
		ExpectManualBinding(ctx, env.Client, pods[0], node1)
		ExpectManualBinding(ctx, env.Client, pods[1], node1)
		ExpectManualBinding(ctx, env.Client, pods[2], node1)
		ExpectScheduled(ctx, env.Client, pods[0])
		ExpectScheduled(ctx, env.Client, pods[1])
		ExpectScheduled(ctx, env.Client, pods[2])
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node1))

		// inform cluster state about the nodes
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node1))
		fakeClock.Step(10 * time.Minute)
		_, err := controller.ProcessCluster(ctx)
		Expect(err).ToNot(HaveOccurred())

		// we don't need a new node
		Expect(cloudProvider.CreateCalls).To(HaveLen(0))
		// and can't delete the node due to the PDB
		ExpectNodeExists(ctx, env.Client, node1.Name)
	})
	It("can replace nodes, considers do-not-consolidate annotation", func() {
		labels := map[string]string{
			"app": "test",
		}

		// create our RS so we can link a pod to it
		rs := test.ReplicaSet()
		ExpectApplied(ctx, env.Client, rs)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

		pods := test.Pods(3, test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: labels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "ReplicaSet",
						Name:               rs.Name,
						UID:                rs.UID,
						Controller:         aws.Bool(true),
						BlockOwnerDeletion: aws.Bool(true),
					},
				}}})

		prov := test.Provisioner(test.ProvisionerOptions{TTLSecondsUntilExpired: aws.Int64(30), Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)}})
		regularNode := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:  resource.MustParse("32"),
				v1.ResourcePods: resource.MustParse("100"),
			}})

		annotatedNode := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1alpha5.DoNotConsolidateNodeAnnotationKey: "true",
				},
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:  resource.MustParse("32"),
				v1.ResourcePods: resource.MustParse("100"),
			}})

		ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], prov)
		ExpectApplied(ctx, env.Client, regularNode, annotatedNode)
		ExpectApplied(ctx, env.Client, regularNode, annotatedNode)
		ExpectMakeNodesReady(ctx, env.Client, regularNode, annotatedNode)
		ExpectManualBinding(ctx, env.Client, pods[0], regularNode)
		ExpectManualBinding(ctx, env.Client, pods[1], regularNode)
		ExpectManualBinding(ctx, env.Client, pods[2], annotatedNode)
		ExpectScheduled(ctx, env.Client, pods[0])
		ExpectScheduled(ctx, env.Client, pods[1])
		ExpectScheduled(ctx, env.Client, pods[2])

		// inform cluster state about the nodes
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(regularNode))
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(annotatedNode))
		fakeClock.Step(10 * time.Minute)
		_, err := controller.ProcessCluster(ctx)
		Expect(err).ToNot(HaveOccurred())

		Expect(cloudProvider.CreateCalls).To(HaveLen(0))
		// we should delete the non-annotated node
		ExpectNotFound(ctx, env.Client, regularNode)
	})
	It("won't replace node if any spot replacement is more expensive", func() {
		currentInstance := fake.NewInstanceType(fake.InstanceTypeOptions{
			Name: "current-on-demand",
			Offerings: []cloudprovider.Offering{
				{
					CapacityType: v1alpha1.CapacityTypeOnDemand,
					Zone:         "test-zone-1a",
					Price:        0.5,
					Available:    false,
				},
			},
		})
		replacementInstance := fake.NewInstanceType(fake.InstanceTypeOptions{
			Name: "potential-spot-replacement",
			Offerings: []cloudprovider.Offering{
				{
					CapacityType: v1alpha1.CapacityTypeSpot,
					Zone:         "test-zone-1a",
					Price:        1.0,
					Available:    true,
				},
				{
					CapacityType: v1alpha1.CapacityTypeSpot,
					Zone:         "test-zone-1b",
					Price:        0.2,
					Available:    true,
				},
				{
					CapacityType: v1alpha1.CapacityTypeSpot,
					Zone:         "test-zone-1c",
					Price:        0.4,
					Available:    true,
				},
			},
		})
		cloudProvider.InstanceTypes = []cloudprovider.InstanceType{
			currentInstance,
			replacementInstance,
		}

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
						Controller:         aws.Bool(true),
						BlockOwnerDeletion: aws.Bool(true),
					},
				}}})

		prov := test.Provisioner(test.ProvisionerOptions{Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)}})
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       currentInstance.Name(),
					v1alpha5.LabelCapacityType:       currentInstance.Offerings()[0].CapacityType,
					v1.LabelTopologyZone:             currentInstance.Offerings()[0].Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: resource.MustParse("32")}})

		ExpectApplied(ctx, env.Client, rs, pod, node, prov)
		ExpectMakeNodesReady(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node))
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectScheduled(ctx, env.Client, pod)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())

		fakeClock.Step(10 * time.Minute)
		_, err := controller.ProcessCluster(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(cloudProvider.CreateCalls).To(HaveLen(0))
		ExpectNodeExists(ctx, env.Client, node.Name)
	})
	It("won't replace on-demand node if on-demand replacement is more expensive", func() {
		currentInstance := fake.NewInstanceType(fake.InstanceTypeOptions{
			Name: "current-on-demand",
			Offerings: []cloudprovider.Offering{
				{
					CapacityType: v1alpha1.CapacityTypeOnDemand,
					Zone:         "test-zone-1a",
					Price:        0.5,
					Available:    false,
				},
			},
		})
		replacementInstance := fake.NewInstanceType(fake.InstanceTypeOptions{
			Name: "on-demand-replacement",
			Offerings: []cloudprovider.Offering{
				{
					CapacityType: v1alpha1.CapacityTypeOnDemand,
					Zone:         "test-zone-1a",
					Price:        0.6,
					Available:    true,
				},
				{
					CapacityType: v1alpha1.CapacityTypeOnDemand,
					Zone:         "test-zone-1b",
					Price:        0.6,
					Available:    true,
				},
				{
					CapacityType: v1alpha1.CapacityTypeSpot,
					Zone:         "test-zone-1b",
					Price:        0.2,
					Available:    true,
				},
				{
					CapacityType: v1alpha1.CapacityTypeSpot,
					Zone:         "test-zone-1c",
					Price:        0.3,
					Available:    true,
				},
			},
		})

		cloudProvider.InstanceTypes = []cloudprovider.InstanceType{
			currentInstance,
			replacementInstance,
		}

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
						Controller:         aws.Bool(true),
						BlockOwnerDeletion: aws.Bool(true),
					},
				}}})

		// provisioner should require on-demand instance for this test case
		prov := test.Provisioner(test.ProvisionerOptions{
			Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha1.CapacityTypeOnDemand},
				},
			},
		})
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       currentInstance.Name(),
					v1alpha5.LabelCapacityType:       currentInstance.Offerings()[0].CapacityType,
					v1.LabelTopologyZone:             currentInstance.Offerings()[0].Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: resource.MustParse("32")}})

		ExpectApplied(ctx, env.Client, rs, pod, node, prov)
		ExpectMakeNodesReady(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node))
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectScheduled(ctx, env.Client, pod)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())

		fakeClock.Step(10 * time.Minute)
		_, err := controller.ProcessCluster(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(cloudProvider.CreateCalls).To(HaveLen(0))
		ExpectNodeExists(ctx, env.Client, node.Name)
	})
	It("waits for node deletion to finish", func() {
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
						Controller:         aws.Bool(true),
						BlockOwnerDeletion: aws.Bool(true),
					},
				}}})

		prov := test.Provisioner(test.ProvisionerOptions{Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)}})
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{"unit-test.com/block-deletion"},
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: resource.MustParse("32")}})

		ExpectApplied(ctx, env.Client, rs, pod, node, prov)
		ExpectMakeNodesReady(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node))
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectScheduled(ctx, env.Client, pod)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())

		// consolidation won't delete the old node until the new node is ready
		wg := ExpectMakeNewNodesReady(ctx, env.Client, 1, node)
		fakeClock.Step(10 * time.Minute)

		var consolidationFinished atomic.Bool
		go func() {
			_, err := controller.ProcessCluster(ctx)
			Expect(err).ToNot(HaveOccurred())
			consolidationFinished.Store(true)
		}()
		wg.Wait()

		// node should still exist
		ExpectNodeExists(ctx, env.Client, node.Name)
		// and consolidation should still be running waiting on the node's deletion
		Expect(consolidationFinished.Load()).To(BeFalse())

		// fetch the latest node object and remove the finalizer
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		node.SetFinalizers([]string{})
		Expect(env.Client.Update(ctx, node)).To(Succeed())

		// consolidation should complete now that the finalizer on the node is gone and it can
		// was actually deleted
		Eventually(consolidationFinished.Load, 10*time.Second).Should(BeTrue())
		ExpectNotFound(ctx, env.Client, node)

		// should create a new node as there is a cheaper one that can hold the pod
		Expect(cloudProvider.CreateCalls).To(HaveLen(1))
	})
})

var _ = Describe("Delete Node", func() {
	It("can delete nodes", func() {
		labels := map[string]string{
			"app": "test",
		}
		// create our RS so we can link a pod to it
		rs := test.ReplicaSet()
		ExpectApplied(ctx, env.Client, rs)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

		pods := test.Pods(3, test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: labels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "ReplicaSet",
						Name:               rs.Name,
						UID:                rs.UID,
						Controller:         aws.Bool(true),
						BlockOwnerDeletion: aws.Bool(true),
					},
				}}})

		prov := test.Provisioner(test.ProvisionerOptions{Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)}})
		node1 := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:  resource.MustParse("32"),
				v1.ResourcePods: resource.MustParse("100"),
			}})

		node2 := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:  resource.MustParse("32"),
				v1.ResourcePods: resource.MustParse("100"),
			}})

		ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], node1, node2, prov)
		ExpectMakeNodesReady(ctx, env.Client, node1, node2)

		ExpectManualBinding(ctx, env.Client, pods[0], node1)
		ExpectManualBinding(ctx, env.Client, pods[1], node1)
		ExpectManualBinding(ctx, env.Client, pods[2], node2)
		ExpectScheduled(ctx, env.Client, pods[0])
		ExpectScheduled(ctx, env.Client, pods[1])
		ExpectScheduled(ctx, env.Client, pods[2])

		// inform cluster state about the nodes
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node1))
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node2))
		fakeClock.Step(10 * time.Minute)
		_, err := controller.ProcessCluster(ctx)
		Expect(err).ToNot(HaveOccurred())

		// we don't need a new node, but we should evict everything off one of node2 which only has a single pod
		Expect(cloudProvider.CreateCalls).To(HaveLen(0))
		// and delete the old one
		ExpectNotFound(ctx, env.Client, node2)
	})
	It("can delete nodes, considers PDB", func() {
		var nl v1.NodeList
		Expect(env.Client.List(ctx, &nl)).To(Succeed())
		Expect(nl.Items).To(HaveLen(0))
		labels := map[string]string{
			"app": "test",
		}
		// create our RS so we can link a pod to it
		rs := test.ReplicaSet()
		ExpectApplied(ctx, env.Client, rs)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

		pods := test.Pods(3, test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "ReplicaSet",
						Name:               rs.Name,
						UID:                rs.UID,
						Controller:         aws.Bool(true),
						BlockOwnerDeletion: aws.Bool(true),
					},
				}}})

		// only pod[2] is covered by the PDB
		pods[2].Labels = labels
		pdb := test.PodDisruptionBudget(test.PDBOptions{
			Labels:         labels,
			MaxUnavailable: fromInt(0),
			Status: &policyv1.PodDisruptionBudgetStatus{
				ObservedGeneration: 1,
				DisruptionsAllowed: 0,
				CurrentHealthy:     1,
				DesiredHealthy:     1,
				ExpectedPods:       1,
			},
		})

		prov := test.Provisioner(test.ProvisionerOptions{Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)}})
		node1 := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:  resource.MustParse("32"),
				v1.ResourcePods: resource.MustParse("100"),
			}})

		node2 := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:  resource.MustParse("32"),
				v1.ResourcePods: resource.MustParse("100"),
			}})

		ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], node1, node2, prov, pdb)
		ExpectMakeNodesReady(ctx, env.Client, node1, node2)
		// two pods on node 1
		ExpectManualBinding(ctx, env.Client, pods[0], node1)
		ExpectManualBinding(ctx, env.Client, pods[1], node1)
		// one on node 2, but it has a PDB with zero disruptions allowed
		ExpectManualBinding(ctx, env.Client, pods[2], node2)
		ExpectScheduled(ctx, env.Client, pods[0])
		ExpectScheduled(ctx, env.Client, pods[1])
		ExpectScheduled(ctx, env.Client, pods[2])

		// inform cluster state about the nodes
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node1))
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node2))
		fakeClock.Step(10 * time.Minute)
		_, err := controller.ProcessCluster(ctx)
		Expect(err).ToNot(HaveOccurred())

		// we don't need a new node
		Expect(cloudProvider.CreateCalls).To(HaveLen(0))
		// but we expect to delete the nmode with more pods (node1) as the pod on node2 has a PDB preventing
		// eviction
		ExpectNotFound(ctx, env.Client, node1)
	})
	It("can delete nodes, considers do-not-evict", func() {
		// create our RS so we can link a pod to it
		rs := test.ReplicaSet()
		ExpectApplied(ctx, env.Client, rs)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

		pods := test.Pods(3, test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "ReplicaSet",
						Name:               rs.Name,
						UID:                rs.UID,
						Controller:         aws.Bool(true),
						BlockOwnerDeletion: aws.Bool(true),
					},
				}}})

		// only pod[2] has a do not evict annotation
		pods[2].Annotations = map[string]string{
			v1alpha5.DoNotEvictPodAnnotationKey: "true",
		}

		prov := test.Provisioner(test.ProvisionerOptions{Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)}})
		node1 := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:  resource.MustParse("32"),
				v1.ResourcePods: resource.MustParse("100"),
			}})

		node2 := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:  resource.MustParse("32"),
				v1.ResourcePods: resource.MustParse("100"),
			}})

		ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], node1, node2, prov)
		ExpectMakeNodesReady(ctx, env.Client, node1, node2)
		// two pods on node 1
		ExpectManualBinding(ctx, env.Client, pods[0], node1)
		ExpectManualBinding(ctx, env.Client, pods[1], node1)
		// one on node 2, but it has a do-not-evict annotation
		ExpectManualBinding(ctx, env.Client, pods[2], node2)
		ExpectScheduled(ctx, env.Client, pods[0])
		ExpectScheduled(ctx, env.Client, pods[1])
		ExpectScheduled(ctx, env.Client, pods[2])

		// inform cluster state about the nodes
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node1))
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node2))
		fakeClock.Step(10 * time.Minute)
		_, err := controller.ProcessCluster(ctx)
		Expect(err).ToNot(HaveOccurred())

		// we don't need a new node
		Expect(cloudProvider.CreateCalls).To(HaveLen(0))
		// but we expect to delete the node with more pods (node1) as the pod on node2 has a do-not-evict annotation
		ExpectNotFound(ctx, env.Client, node1)
	})
	It("can delete nodes, doesn't evict standalone pods", func() {
		// create our RS so we can link a pod to it
		rs := test.ReplicaSet()
		ExpectApplied(ctx, env.Client, rs)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

		pods := test.Pods(3, test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "ReplicaSet",
						Name:               rs.Name,
						UID:                rs.UID,
						Controller:         aws.Bool(true),
						BlockOwnerDeletion: aws.Bool(true),
					},
				}}})

		// pod[2] is a stand-alone (non ReplicaSet) pod
		pods[2].OwnerReferences = nil

		prov := test.Provisioner(test.ProvisionerOptions{Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)}})
		node1 := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:  resource.MustParse("32"),
				v1.ResourcePods: resource.MustParse("100"),
			}})

		node2 := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:  resource.MustParse("32"),
				v1.ResourcePods: resource.MustParse("100"),
			}})

		ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], node1, node2, prov)
		ExpectMakeNodesReady(ctx, env.Client, node1, node2)
		// two pods on node 1
		ExpectManualBinding(ctx, env.Client, pods[0], node1)
		ExpectManualBinding(ctx, env.Client, pods[1], node1)
		// one on node 2, but it's a standalone pod
		ExpectManualBinding(ctx, env.Client, pods[2], node2)
		ExpectScheduled(ctx, env.Client, pods[0])
		ExpectScheduled(ctx, env.Client, pods[1])
		ExpectScheduled(ctx, env.Client, pods[2])

		// inform cluster state about the nodes
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node1))
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node2))
		fakeClock.Step(10 * time.Minute)
		_, err := controller.ProcessCluster(ctx)
		Expect(err).ToNot(HaveOccurred())

		// we don't need a new node
		Expect(cloudProvider.CreateCalls).To(HaveLen(0))
		// but we expect to delete the node with more pods (node1) as the pod on node2 doesn't have a controller to
		// recreate it
		ExpectNotFound(ctx, env.Client, node1)
	})
})

var _ = Describe("Node Lifetime Consideration", func() {
	It("should consider node lifetime remaining when calculating disruption cost", func() {
		labels := map[string]string{
			"app": "test",
		}
		// create our RS so we can link a pod to it
		rs := test.ReplicaSet()
		ExpectApplied(ctx, env.Client, rs)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

		pods := test.Pods(3, test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: labels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "ReplicaSet",
						Name:               rs.Name,
						UID:                rs.UID,
						Controller:         aws.Bool(true),
						BlockOwnerDeletion: aws.Bool(true),
					},
				}}})

		prov := test.Provisioner(test.ProvisionerOptions{TTLSecondsUntilExpired: aws.Int64(3), Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)}})
		node1 := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:  resource.MustParse("32"),
				v1.ResourcePods: resource.MustParse("100"),
			}})

		node2 := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:  resource.MustParse("32"),
				v1.ResourcePods: resource.MustParse("100"),
			}})

		ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], prov)
		ExpectApplied(ctx, env.Client, node1) // ensure node1 is the oldest node
		time.Sleep(2 * time.Second)           // this sleep is unfortunate, but necessary.  The creation time is from etcd and we can't mock it, so we
		// need to sleep to force the second node to be created a bit after the first node.
		ExpectApplied(ctx, env.Client, node2)
		ExpectMakeNodesReady(ctx, env.Client, node1, node2)
		// two pods on node 1, one on node 2
		ExpectManualBinding(ctx, env.Client, pods[0], node1)
		ExpectManualBinding(ctx, env.Client, pods[1], node1)
		ExpectManualBinding(ctx, env.Client, pods[2], node2)
		ExpectScheduled(ctx, env.Client, pods[0])
		ExpectScheduled(ctx, env.Client, pods[1])
		ExpectScheduled(ctx, env.Client, pods[2])

		// inform cluster state about the nodes
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node1))
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node2))
		fakeClock.SetTime(time.Now())
		_, err := controller.ProcessCluster(ctx)
		Expect(err).ToNot(HaveOccurred())

		// the second node has more pods so it would normally not be picked for consolidation, except it very little
		// lifetime remaining so it should be deleted
		Expect(cloudProvider.CreateCalls).To(HaveLen(0))
		ExpectNotFound(ctx, env.Client, node1)
	})
})

var _ = Describe("Topology Consideration", func() {
	It("can replace node maintaining zonal topology spread", func() {
		labels := map[string]string{
			"app": "test-zonal-spread",
		}

		// create our RS so we can link a pod to it
		rs := test.ReplicaSet()
		ExpectApplied(ctx, env.Client, rs)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

		tsc := v1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       v1.LabelTopologyZone,
			WhenUnsatisfiable: v1.DoNotSchedule,
			LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
		}
		pods := test.Pods(4, test.PodOptions{
			ResourceRequirements:      v1.ResourceRequirements{Requests: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: resource.MustParse("1")}},
			TopologySpreadConstraints: []v1.TopologySpreadConstraint{tsc},
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "ReplicaSet",
						Name:               rs.Name,
						UID:                rs.UID,
						Controller:         aws.Bool(true),
						BlockOwnerDeletion: aws.Bool(true),
					},
				}}})

		testZone1Instance := leastExpensiveInstanceWithZone("test-zone-1")
		testZone2Instance := mostExpensiveInstanceWithZone("test-zone-2")
		testZone3Instance := leastExpensiveInstanceWithZone("test-zone-3")

		prov := test.Provisioner(test.ProvisionerOptions{Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)}})
		zone1Node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelTopologyZone:             "test-zone-1",
					v1.LabelInstanceTypeStable:       testZone1Instance.Name(),
					v1alpha5.LabelCapacityType:       testZone1Instance.Offerings()[0].CapacityType,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: resource.MustParse("1")}})

		zone2Node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelTopologyZone:             "test-zone-2",
					v1.LabelInstanceTypeStable:       testZone2Instance.Name(),
					v1alpha5.LabelCapacityType:       testZone2Instance.Offerings()[0].CapacityType,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: resource.MustParse("1")}})

		zone3Node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelTopologyZone:             "test-zone-3",
					v1.LabelInstanceTypeStable:       testZone3Instance.Name(),
					v1alpha5.LabelCapacityType:       testZone1Instance.Offerings()[0].CapacityType,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: resource.MustParse("1")}})

		ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], zone1Node, zone2Node, zone3Node, prov)
		ExpectMakeNodesReady(ctx, env.Client, zone1Node, zone2Node, zone3Node)
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(zone1Node))
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(zone2Node))
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(zone3Node))
		ExpectManualBinding(ctx, env.Client, pods[0], zone1Node)
		ExpectManualBinding(ctx, env.Client, pods[1], zone2Node)
		ExpectManualBinding(ctx, env.Client, pods[2], zone3Node)
		ExpectScheduled(ctx, env.Client, pods[0])
		ExpectScheduled(ctx, env.Client, pods[1])
		ExpectScheduled(ctx, env.Client, pods[2])

		ExpectSkew(ctx, env.Client, "default", &tsc).To(ConsistOf(1, 1, 1))

		// consolidation won't delete the old node until the new node is ready
		wg := ExpectMakeNewNodesReady(ctx, env.Client, 1, zone1Node, zone2Node, zone3Node)
		fakeClock.Step(10 * time.Minute)
		_, err := controller.ProcessCluster(ctx)
		Expect(err).ToNot(HaveOccurred())
		wg.Wait()

		// should create a new node as there is a cheaper one that can hold the pod
		Expect(cloudProvider.CreateCalls).To(HaveLen(1))

		// we need to emulate the replicaset controller and bind a new pod to the newly created node
		ExpectApplied(ctx, env.Client, pods[3])
		var nodes v1.NodeList
		Expect(env.Client.List(ctx, &nodes)).To(Succeed())
		Expect(nodes.Items).To(HaveLen(3))
		for i, n := range nodes.Items {
			// bind the pod to the new node we don't recognize as it is the one that consolidation created
			if n.Name != zone1Node.Name && n.Name != zone2Node.Name && n.Name != zone3Node.Name {
				ExpectManualBinding(ctx, env.Client, pods[3], &nodes.Items[i])
			}
		}
		// we should maintain our skew, the new node must be in the same zone as the old node it replaced
		ExpectSkew(ctx, env.Client, "default", &tsc).To(ConsistOf(1, 1, 1))
	})
	It("won't delete node if it would violate pod anti-affinity", func() {
		labels := map[string]string{
			"app": "test",
		}
		// create our RS so we can link a pod to it
		rs := test.ReplicaSet()
		ExpectApplied(ctx, env.Client, rs)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

		pods := test.Pods(3, test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{Requests: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: resource.MustParse("1")}},
			PodAntiRequirements: []v1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{MatchLabels: labels},
					TopologyKey:   v1.LabelHostname,
				},
			},
			ObjectMeta: metav1.ObjectMeta{Labels: labels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "ReplicaSet",
						Name:               rs.Name,
						UID:                rs.UID,
						Controller:         aws.Bool(true),
						BlockOwnerDeletion: aws.Bool(true),
					},
				}}})

		testZone1Instance := leastExpensiveInstanceWithZone("test-zone-1")
		testZone2Instance := leastExpensiveInstanceWithZone("test-zone-2")
		testZone3Instance := leastExpensiveInstanceWithZone("test-zone-3")

		prov := test.Provisioner(test.ProvisionerOptions{Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)}})
		zone1Node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelTopologyZone:             "test-zone-1",
					v1.LabelInstanceTypeStable:       testZone1Instance.Name(),
					v1alpha5.LabelCapacityType:       testZone1Instance.Offerings()[0].CapacityType,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: resource.MustParse("1")}})

		zone2Node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelTopologyZone:             "test-zone-2",
					v1.LabelInstanceTypeStable:       testZone2Instance.Name(),
					v1alpha5.LabelCapacityType:       testZone2Instance.Offerings()[0].CapacityType,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: resource.MustParse("1")}})

		zone3Node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelTopologyZone:             "test-zone-3",
					v1.LabelInstanceTypeStable:       testZone3Instance.Name(),
					v1alpha5.LabelCapacityType:       testZone3Instance.Offerings()[0].CapacityType,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: resource.MustParse("1")}})

		ExpectApplied(ctx, env.Client, rs, pods[0], pods[1], pods[2], zone1Node, zone2Node, zone3Node, prov)
		ExpectMakeNodesReady(ctx, env.Client, zone1Node, zone2Node, zone3Node)
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(zone1Node))
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(zone2Node))
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(zone3Node))
		ExpectManualBinding(ctx, env.Client, pods[0], zone1Node)
		ExpectManualBinding(ctx, env.Client, pods[1], zone2Node)
		ExpectManualBinding(ctx, env.Client, pods[2], zone3Node)
		ExpectScheduled(ctx, env.Client, pods[0])
		ExpectScheduled(ctx, env.Client, pods[1])
		ExpectScheduled(ctx, env.Client, pods[2])

		wg := ExpectMakeNewNodesReady(ctx, env.Client, 1, zone1Node, zone2Node, zone3Node)
		fakeClock.Step(10 * time.Minute)
		_, err := controller.ProcessCluster(ctx)
		Expect(err).ToNot(HaveOccurred())
		wg.Wait()

		// our nodes are already the cheapest available, so we can't replace them.  If we delete, it would
		// violate the anti-affinity rule so we can't do anything.
		Expect(cloudProvider.CreateCalls).To(HaveLen(0))
		ExpectNodeExists(ctx, env.Client, zone1Node.Name)
		ExpectNodeExists(ctx, env.Client, zone2Node.Name)
		ExpectNodeExists(ctx, env.Client, zone3Node.Name)

	})
})

var _ = Describe("Empty Nodes", func() {
	It("can delete empty nodes", func() {
		prov := test.Provisioner(test.ProvisionerOptions{Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)}})

		node1 := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelNodeInitialized:    "true",
				},
			},
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:  resource.MustParse("32"),
				v1.ResourcePods: resource.MustParse("100"),
			}})

		ExpectApplied(ctx, env.Client, node1, prov)

		// inform cluster state about the nodes
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node1))
		fakeClock.Step(10 * time.Minute)
		_, err := controller.ProcessCluster(ctx)
		Expect(err).ToNot(HaveOccurred())

		// we don't need any new nodes
		Expect(cloudProvider.CreateCalls).To(HaveLen(0))
		// and should delete the empty one
		ExpectNotFound(ctx, env.Client, node1)
	})
	It("can delete multiple empty nodes", func() {
		prov := test.Provisioner(test.ProvisionerOptions{Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)}})

		node1 := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:  resource.MustParse("32"),
				v1.ResourcePods: resource.MustParse("100"),
			}})
		node2 := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				}},
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:  resource.MustParse("32"),
				v1.ResourcePods: resource.MustParse("100"),
			}})

		ExpectApplied(ctx, env.Client, node1, node2, prov)
		ExpectMakeNodesReady(ctx, env.Client, node1, node2)

		// inform cluster state about the nodes
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node1))
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node2))
		fakeClock.Step(10 * time.Minute)
		_, err := controller.ProcessCluster(ctx)
		Expect(err).ToNot(HaveOccurred())

		// we don't need any new nodes
		Expect(cloudProvider.CreateCalls).To(HaveLen(0))
		// and should delete both empty ones
		ExpectNotFound(ctx, env.Client, node1)
		ExpectNotFound(ctx, env.Client, node2)
	})
})

var _ = Describe("Parallelization", func() {
	It("should schedule an additional node when receiving pending pods while consolidating", func() {
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
						Controller:         aws.Bool(true),
						BlockOwnerDeletion: aws.Bool(true),
					},
				}}})

		prov := test.Provisioner(test.ProvisionerOptions{Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)}})

		// Add a finalizer to the node so that it sticks around for the scheduling loop
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: prov.Name,
					v1.LabelInstanceTypeStable:       mostExpensiveInstance.Name(),
					v1alpha5.LabelCapacityType:       mostExpensiveOffering.CapacityType,
					v1.LabelTopologyZone:             mostExpensiveOffering.Zone,
				},
				Finalizers: []string{"karpenter.sh/test-finalizer"},
			},
			Allocatable: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: resource.MustParse("32")}})

		ExpectApplied(ctx, env.Client, rs, pod, node, prov)
		ExpectMakeNodesReady(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node))
		ExpectManualBinding(ctx, env.Client, pod, node)
		ExpectScheduled(ctx, env.Client, pod)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())

		fakeClock.Step(10 * time.Minute)

		// Run the processing loop in parallel in the background with environment context
		go func() {
			_, err := controller.ProcessCluster(env.Ctx)
			Expect(err).ToNot(HaveOccurred())
		}()

		Eventually(func(g Gomega) {
			// should create a new node as there is a cheaper one that can hold the pod
			nodes := &v1.NodeList{}
			g.Expect(env.Client.List(ctx, nodes)).To(Succeed())
			g.Expect(len(nodes.Items)).To(Equal(2))
		}, time.Second*10).Should(Succeed())

		// Add a new pending pod that should schedule while node is not yet deleted
		pods := ExpectProvisionedNoBinding(ctx, env.Client, provisioningController, test.UnschedulablePod())
		nodes := &v1.NodeList{}
		Expect(env.Client.List(ctx, nodes)).To(Succeed())
		Expect(len(nodes.Items)).To(Equal(3))
		Expect(pods[0].Spec.NodeName).NotTo(Equal(node.Name))
	})
	It("should not consolidate a node that is launched for pods on a deleting node", func() {
		labels := map[string]string{
			"app": "test",
		}
		// create our RS so we can link a pod to it
		rs := test.ReplicaSet()
		ExpectApplied(ctx, env.Client, rs)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(rs), rs)).To(Succeed())

		prov := test.Provisioner(test.ProvisionerOptions{Consolidation: &v1alpha5.Consolidation{Enabled: aws.Bool(true)}})
		podOpts := test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         "apps/v1",
						Kind:               "ReplicaSet",
						Name:               rs.Name,
						UID:                rs.UID,
						Controller:         aws.Bool(true),
						BlockOwnerDeletion: aws.Bool(true),
					},
				},
			},
			ResourceRequirements: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("1"),
				},
			},
		}

		var pods []*v1.Pod
		for i := 0; i < 5; i++ {
			pod := test.UnschedulablePod(podOpts)
			pods = append(pods, pod)
		}
		ExpectApplied(ctx, env.Client, rs, prov)
		ExpectProvisioned(ctx, env.Client, provisioningController, pods...)

		nodeList := &v1.NodeList{}
		Expect(env.Client.List(ctx, nodeList)).To(Succeed())
		Expect(len(nodeList.Items)).To(Equal(1))

		// Update cluster state with new node
		ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(&nodeList.Items[0]))

		// Reset the bindings so we can re-record bindings
		recorder.ResetBindings()

		// Mark the node for deletion and re-trigger reconciliation
		oldNodeName := nodeList.Items[0].Name
		cluster.MarkForDeletion(nodeList.Items[0].Name)
		ExpectProvisionedNoBinding(ctx, env.Client, provisioningController)

		// Make sure that the cluster state is aware of the current node state
		Expect(env.Client.List(ctx, nodeList)).To(Succeed())
		Expect(len(nodeList.Items)).To(Equal(2))
		newNode, _ := lo.Find(nodeList.Items, func(n v1.Node) bool { return n.Name != oldNodeName })

		for i := range nodeList.Items {
			node := nodeList.Items[i]
			ExpectMakeNodesReady(ctx, env.Client, &node)
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(&node))
		}

		// Wait for the nomination cache to expire
		time.Sleep(time.Second * 11)

		// Re-create the pods to re-bind them
		for i := 0; i < 2; i++ {
			ExpectDeleted(ctx, env.Client, pods[i])
			pod := test.UnschedulablePod(podOpts)
			ExpectApplied(ctx, env.Client, pod)
			ExpectManualBinding(ctx, env.Client, pod, &newNode)
		}

		// Trigger a reconciliation run which should take into account the deleting node
		// Consolidation shouldn't trigger additional actions
		fakeClock.Step(10 * time.Minute)
		result, err := controller.ProcessCluster(env.Ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(consolidation.ProcessResultNothingToDo))
	})
})

func leastExpensiveInstanceWithZone(zone string) cloudprovider.InstanceType {
	for _, elem := range onDemandInstances {
		if hasZone(elem.Offerings(), zone) {
			return elem
		}
	}
	return onDemandInstances[len(onDemandInstances)-1]
}

func mostExpensiveInstanceWithZone(zone string) cloudprovider.InstanceType {
	for i := len(onDemandInstances) - 1; i >= 0; i-- {
		elem := onDemandInstances[i]
		if hasZone(elem.Offerings(), zone) {
			return elem
		}
	}
	return onDemandInstances[0]
}

// hasZone checks whether any of the passed offerings have a zone matching
// the passed zone
func hasZone(ofs []cloudprovider.Offering, zone string) bool {
	for _, elem := range ofs {
		if elem.Zone == zone {
			return true
		}
	}
	return false
}

func fromInt(i int) *intstr.IntOrString {
	v := intstr.FromInt(i)
	return &v
}

func ExpectMakeNewNodesReady(ctx context.Context, client client.Client, numNewNodes int, existingNodes ...*v1.Node) *sync.WaitGroup {
	var wg sync.WaitGroup

	existingNodeNames := sets.NewString()
	for _, existing := range existingNodes {
		existingNodeNames.Insert(existing.Name)
	}
	wg.Add(1)
	go func() {
		defer GinkgoRecover()
		defer wg.Done()
		start := time.Now()
		for {
			select {
			case <-time.After(50 * time.Millisecond):
				// give up after 10 seconds
				if time.Since(start) > 10*time.Second {
					return
				}
				var nodeList v1.NodeList
				err := client.List(ctx, &nodeList)
				if err != nil {
					continue
				}
				nodesMadeReady := 0
				for i := range nodeList.Items {
					n := &nodeList.Items[i]
					if existingNodeNames.Has(n.Name) {
						continue
					}
					ExpectMakeNodesReady(ctx, env.Client, n)
					nodesMadeReady++
					// did we make all of the nodes ready that we expected?
					if nodesMadeReady == numNewNodes {
						return
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return &wg
}

func ExpectMakeNodesReady(ctx context.Context, c client.Client, nodes ...*v1.Node) {
	for _, node := range nodes {
		var n v1.Node
		Expect(c.Get(ctx, client.ObjectKeyFromObject(node), &n)).To(Succeed())
		n.Status.Phase = v1.NodeRunning
		n.Status.Conditions = []v1.NodeCondition{
			{
				Type:               v1.NodeReady,
				Status:             v1.ConditionTrue,
				LastHeartbeatTime:  metav1.Now(),
				LastTransitionTime: metav1.Now(),
				Reason:             "KubeletReady",
			},
		}
		if n.Labels == nil {
			n.Labels = map[string]string{}
		}
		n.Labels[v1alpha5.LabelNodeInitialized] = "true"
		n.Spec.Taints = nil
		ExpectApplied(ctx, c, &n)
	}
}

// cheapestOffering grabs the cheapest offering from the passed offerings
func cheapestOffering(ofs []cloudprovider.Offering) cloudprovider.Offering {
	offering := cloudprovider.Offering{Price: math.MaxFloat64}
	for _, of := range ofs {
		if of.Price < offering.Price {
			offering = of
		}
	}
	return offering
}
