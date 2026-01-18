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

package state_test

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	cloudproviderapi "k8s.io/cloud-provider/api"
	clock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/controllers/state/informer"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var env *test.Environment
var fakeClock *clock.FakeClock
var cluster *state.Cluster
var nodeClaimController *informer.NodeClaimController
var nodeController *informer.NodeController
var podController *informer.PodController
var nodePoolController *informer.NodePoolController
var daemonsetController *informer.DaemonSetController
var cloudProvider *fake.CloudProvider
var nodePool *v1.NodePool

const csiProvider = "fake.csi.provider"

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers/State")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(test.WithCRDs(apis.CRDs...), test.WithCRDs(v1alpha1.CRDs...), test.WithConfigOptions(func(config *rest.Config) {
		config.QPS = -1
	}))
	ctx = options.ToContext(ctx, test.Options())
	cloudProvider = fake.NewCloudProvider()
	fakeClock = clock.NewFakeClock(time.Now())
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	nodeClaimController = informer.NewNodeClaimController(env.Client, cloudProvider, cluster)
	nodeController = informer.NewNodeController(env.Client, cluster)
	podController = informer.NewPodController(env.Client, cluster)
	nodePoolController = informer.NewNodePoolController(env.Client, cloudProvider, cluster)
	daemonsetController = informer.NewDaemonSetController(env.Client, cluster)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	fakeClock.SetTime(time.Now())
	state.ClusterStateUnsyncedTimeSeconds.Reset()
	cloudProvider.InstanceTypes = fake.InstanceTypesAssorted()
	nodePool = test.NodePool(v1.NodePool{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	ExpectApplied(ctx, env.Client, nodePool)
})
var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
	cluster.Reset()
	cloudProvider.Reset()
})
var _ = Describe("Pod Healthy NodePool", func() {
	It("should not store pod schedulable time if the nodePool that pod is scheduled to does not have NodeRegistrationHealthy=true", func() {
		pod := test.Pod()
		ExpectApplied(ctx, env.Client, pod, nodePool)
		cluster.MarkPodSchedulingDecisions(ctx, nil, map[string][]*corev1.Pod{nodePool.Name: {pod}}, nil)
		setTime := cluster.PodSchedulingSuccessTimeRegistrationHealthyCheck(client.ObjectKeyFromObject(pod))
		Expect(setTime.IsZero()).To(BeTrue())
	})
	It("should store pod schedulable time if the nodePool that pod is scheduled to has NodeRegistrationHealthy=true", func() {
		pod := test.Pod()
		nodePool.StatusConditions().SetTrue(v1.ConditionTypeNodeRegistrationHealthy)
		ExpectApplied(ctx, env.Client, pod, nodePool)

		cluster.MarkPodSchedulingDecisions(ctx, nil, map[string][]*corev1.Pod{nodePool.Name: {pod}}, nil)
		setTime := cluster.PodSchedulingSuccessTimeRegistrationHealthyCheck(client.ObjectKeyFromObject(pod))
		Expect(setTime.IsZero()).To(BeFalse())
	})
	It("should not update the pod schedulable time if it is already stored for a pod", func() {
		pod := test.Pod()
		nodePool.StatusConditions().SetTrue(v1.ConditionTypeNodeRegistrationHealthy)
		ExpectApplied(ctx, env.Client, pod, nodePool)

		// This will store the pod schedulable time
		cluster.MarkPodSchedulingDecisions(ctx, nil, map[string][]*corev1.Pod{nodePool.Name: {pod}}, nil)
		setTime := cluster.PodSchedulingSuccessTimeRegistrationHealthyCheck(client.ObjectKeyFromObject(pod))
		Expect(setTime.IsZero()).To(BeFalse())

		fakeClock.Step(time.Minute)
		// We try to update pod schedulable time, but it should not change as we have already stored it
		cluster.MarkPodSchedulingDecisions(ctx, nil, map[string][]*corev1.Pod{nodePool.Name: {pod}}, nil)
		Expect(cluster.PodSchedulingSuccessTimeRegistrationHealthyCheck(client.ObjectKeyFromObject(pod))).To(Equal(setTime))
	})
	It("should delete the pod schedulable time if the pod is deleted", func() {
		pod := test.Pod()
		nodePool.StatusConditions().SetTrue(v1.ConditionTypeNodeRegistrationHealthy)
		ExpectApplied(ctx, env.Client, pod, nodePool)

		// This will store the pod schedulable time
		cluster.MarkPodSchedulingDecisions(ctx, nil, map[string][]*corev1.Pod{nodePool.Name: {pod}}, nil)
		setTime := cluster.PodSchedulingSuccessTimeRegistrationHealthyCheck(client.ObjectKeyFromObject(pod))
		Expect(setTime.IsZero()).To(BeFalse())

		// Delete the pod
		cluster.DeletePod(client.ObjectKeyFromObject(pod))
		Expect(cluster.PodSchedulingSuccessTimeRegistrationHealthyCheck(client.ObjectKeyFromObject(pod)).IsZero()).To(BeTrue())
	})
})

var _ = Describe("Pod Ack", func() {
	It("should only mark pods as schedulable once", func() {
		pod := test.Pod()
		ExpectApplied(ctx, env.Client, pod)
		nn := client.ObjectKeyFromObject(pod)

		setTime := cluster.PodSchedulingSuccessTime(nn)
		Expect(setTime.IsZero()).To(BeTrue())

		cluster.MarkPodSchedulingDecisions(ctx, nil, map[string][]*corev1.Pod{"n1": {pod}}, map[string][]*corev1.Pod{"nc1": {pod}})
		setTime = cluster.PodSchedulingSuccessTime(nn)
		Expect(setTime.IsZero()).To(BeFalse())

		newTime := cluster.PodSchedulingSuccessTime(nn)
		Expect(newTime.Compare(setTime)).To(Equal(0))
		Expect(cluster.PodNodeClaimMapping(nn)).To(BeEquivalentTo("nc1"))
	})
	It("should delete pod schedulable time and pod to nodeClaim mapping if we get error for the pod", func() {
		pod := test.Pod()
		ExpectApplied(ctx, env.Client, pod)
		nn := client.ObjectKeyFromObject(pod)

		setTime := cluster.PodSchedulingSuccessTime(nn)
		Expect(setTime.IsZero()).To(BeTrue())
		cluster.MarkPodSchedulingDecisions(ctx, nil, map[string][]*corev1.Pod{"n1": {pod}}, nil)
		setTime = cluster.PodSchedulingSuccessTime(nn)
		Expect(setTime.IsZero()).To(BeFalse())

		cluster.MarkPodSchedulingDecisions(ctx, map[*corev1.Pod]error{
			pod: fmt.Errorf("ignoring pod"),
		}, nil, nil)
		Expect(cluster.PodSchedulingSuccessTime(nn).IsZero()).To(BeTrue())
		Expect(cluster.PodNodeClaimMapping(nn)).To(BeEquivalentTo(""))
	})
	It("should delete the pod mappings from memory when the pod is deleted", func() {
		pod := test.Pod()
		nodePool.StatusConditions().SetTrue(v1.ConditionTypeNodeRegistrationHealthy)
		ExpectApplied(ctx, env.Client, pod, nodePool)

		nn := client.ObjectKeyFromObject(pod)
		// This will store the pod mappings
		cluster.MarkPodSchedulingDecisions(ctx, nil, map[string][]*corev1.Pod{"np1": {pod}}, map[string][]*corev1.Pod{"nc1": {pod}})
		Expect(cluster.PodSchedulingSuccessTime(nn).IsZero()).To(BeFalse())
		Expect(cluster.PodSchedulingDecisionTime(nn).IsZero()).To(BeFalse())
		Expect(cluster.PodNodeClaimMapping(nn)).To(BeEquivalentTo("nc1"))

		// Delete the pod
		cluster.DeletePod(client.ObjectKeyFromObject(pod))
		Expect(cluster.PodSchedulingSuccessTime(nn).IsZero()).To(BeTrue())
		Expect(cluster.PodSchedulingDecisionTime(nn).IsZero()).To(BeTrue())
		Expect(cluster.PodNodeClaimMapping(nn)).To(BeEquivalentTo(""))
	})
})

var _ = Describe("Volume Usage/Limits", func() {
	var nodeClaim *v1.NodeClaim
	var node *corev1.Node
	var csiNode *storagev1.CSINode
	var sc *storagev1.StorageClass
	BeforeEach(func() {
		instanceType := cloudProvider.InstanceTypes[0]
		nodeClaim, node = test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: instanceType.Name,
			}},
			Status: v1.NodeClaimStatus{
				ProviderID: test.RandomProviderID(),
			},
		})
		sc = test.StorageClass(test.StorageClassOptions{
			ObjectMeta:  metav1.ObjectMeta{Name: "my-storage-class"},
			Provisioner: lo.ToPtr(csiProvider),
			Zones:       []string{"test-zone-1"},
		})
		csiNode = &storagev1.CSINode{
			ObjectMeta: metav1.ObjectMeta{
				Name: node.Name,
			},
			Spec: storagev1.CSINodeSpec{
				Drivers: []storagev1.CSINodeDriver{
					{
						Name:   csiProvider,
						NodeID: "fake-node-id",
						Allocatable: &storagev1.VolumeNodeResources{
							Count: lo.ToPtr(int32(10)),
						},
					},
				},
			},
		}
	})
	It("should hydrate the volume usage on a Node update", func() {
		ExpectApplied(ctx, env.Client, sc, node, csiNode)
		for i := 0; i < 10; i++ {
			pvc := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
				StorageClassName: lo.ToPtr(sc.Name),
			})
			pod := test.Pod(test.PodOptions{
				PersistentVolumeClaims: []string{pvc.Name},
			})
			ExpectApplied(ctx, env.Client, pvc, pod)
			ExpectManualBinding(ctx, env.Client, pod, node)
		}
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectStateNodeCount("==", 1)
		stateNode := ExpectStateNodeExists(cluster, node)

		// Adding more volumes should cause an error since we are at the volume limits
		Expect(stateNode.VolumeUsage().ExceedsLimits(scheduling.Volumes{
			csiProvider: sets.New("test"),
		})).ToNot(BeNil())
	})
	It("should maintain the volume usage state when receiving NodeClaim updates", func() {
		ExpectApplied(ctx, env.Client, sc, nodeClaim, node, csiNode)
		for i := 0; i < 10; i++ {
			pvc := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
				StorageClassName: lo.ToPtr(sc.Name),
			})
			pod := test.Pod(test.PodOptions{
				PersistentVolumeClaims: []string{pvc.Name},
			})
			ExpectApplied(ctx, env.Client, pvc, pod)
			ExpectManualBinding(ctx, env.Client, pod, node)
		}
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectStateNodeCount("==", 1)
		stateNode := ExpectStateNodeExists(cluster, node)

		// Adding more volumes should cause an error since we are at the volume limits
		Expect(stateNode.VolumeUsage().ExceedsLimits(scheduling.Volumes{
			csiProvider: sets.New("test"),
		})).ToNot(BeNil())

		// Reconcile the nodeclaim one more time to ensure that we maintain our volume usage state
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))

		// Ensure that we still consider adding another volume to the node breaching our volume limits
		Expect(stateNode.VolumeUsage().ExceedsLimits(scheduling.Volumes{
			csiProvider: sets.New("test"),
		})).ToNot(BeNil())
	})
	It("should ignore the volume usage limits breach if the pod update is for an already tracked pod", func() {
		ExpectApplied(ctx, env.Client, sc, nodeClaim, node, csiNode)
		var pvcs []*corev1.PersistentVolumeClaim
		for i := 0; i < 10; i++ {
			pvc := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{
				StorageClassName: lo.ToPtr(sc.Name),
			})
			pod := test.Pod(test.PodOptions{
				PersistentVolumeClaims: []string{pvc.Name},
			})
			pvcs = append(pvcs, pvc)
			ExpectApplied(ctx, env.Client, pvc, pod)
			ExpectManualBinding(ctx, env.Client, pod, node)
		}
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectStateNodeCount("==", 1)
		stateNode := ExpectStateNodeExists(cluster, node)

		// Adding more volumes should not cause an error since this PVC volume is already tracked
		Expect(stateNode.VolumeUsage().ExceedsLimits(scheduling.Volumes{
			csiProvider: sets.New(client.ObjectKeyFromObject(pvcs[5]).String()),
		})).To(BeNil())
	})
})

var _ = Describe("HostPort Usage", func() {
	var nodeClaim *v1.NodeClaim
	var node *corev1.Node
	BeforeEach(func() {
		instanceType := cloudProvider.InstanceTypes[0]
		nodeClaim, node = test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: instanceType.Name,
			}},
			Status: v1.NodeClaimStatus{
				ProviderID: test.RandomProviderID(),
			},
		})
	})
	It("should hydrate the HostPort usage on a Node update", func() {
		ExpectApplied(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		for i := 0; i < 10; i++ {
			pod := test.Pod(test.PodOptions{
				HostPorts: []int32{int32(i)},
			})
			ExpectApplied(ctx, env.Client, pod)
			ExpectManualBinding(ctx, env.Client, pod, node)
		}
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectStateNodeCount("==", 1)
		stateNode := ExpectStateNodeExists(cluster, node)

		// Adding a conflicting host port should cause an error
		Expect(stateNode.HostPortUsage().Conflicts(test.Pod(), []scheduling.HostPort{
			{
				IP:       net.IP("0.0.0.0"),
				Port:     int32(5),
				Protocol: corev1.ProtocolTCP,
			},
		})).ToNot(BeNil())
	})
	It("should maintain the host port usage state when receiving NodeClaim updates", func() {
		ExpectApplied(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		for i := 0; i < 10; i++ {
			pod := test.Pod(test.PodOptions{
				HostPorts: []int32{int32(i)},
			})
			ExpectApplied(ctx, env.Client, pod)
			ExpectManualBinding(ctx, env.Client, pod, node)
		}
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectStateNodeCount("==", 1)
		stateNode := ExpectStateNodeExists(cluster, node)

		// Adding a conflicting host port should cause an error
		Expect(stateNode.HostPortUsage().Conflicts(test.Pod(), []scheduling.HostPort{
			{
				IP:       net.IP("0.0.0.0"),
				Port:     int32(5),
				Protocol: corev1.ProtocolTCP,
			},
		})).ToNot(BeNil())

		// Reconcile the nodeclaim one more time to ensure that we maintain our volume usage state
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))

		// Ensure that we still consider the host port usage addition an error
		Expect(stateNode.HostPortUsage().Conflicts(test.Pod(), []scheduling.HostPort{
			{
				IP:       net.IP("0.0.0.0"),
				Port:     int32(5),
				Protocol: corev1.ProtocolTCP,
			},
		})).ToNot(BeNil())
	})
	It("should ignore the host port usage conflict if the pod update is for an already tracked pod", func() {
		ExpectApplied(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		var pods []*corev1.Pod
		for i := 0; i < 10; i++ {
			pod := test.Pod(test.PodOptions{
				HostPorts: []int32{int32(i)},
			})
			pods = append(pods, pod)
			ExpectApplied(ctx, env.Client, pod)
			ExpectManualBinding(ctx, env.Client, pod, node)
		}
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectStateNodeCount("==", 1)
		stateNode := ExpectStateNodeExists(cluster, node)

		// Adding a conflicting host port should not cause an error since this port is already tracked for the pod
		Expect(stateNode.HostPortUsage().Conflicts(pods[5], []scheduling.HostPort{
			{
				IP:       net.IP("0.0.0.0"),
				Port:     int32(5),
				Protocol: corev1.ProtocolTCP,
			},
		})).To(BeNil())
	})
})

var _ = Describe("Node Deletion", func() {
	It("should not leak a state node when the NodeClaim and Node names match", func() {
		nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
				},
			},
		})
		node.Name = nodeClaim.Name

		ExpectApplied(ctx, env.Client, nodeClaim, node)
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

		ExpectStateNodeCount("==", 1)

		// Expect that the node isn't leaked due to names matching
		ExpectDeleted(ctx, env.Client, nodeClaim)
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
		ExpectStateNodeCount("==", 1)
		ExpectDeleted(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectStateNodeCount("==", 0)
	})
})

var _ = Describe("Node Resource Level", func() {
	It("should not count pods not bound to nodes", func() {
		pod1 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.5"),
				}},
		})
		pod2 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("2"),
				}},
		})
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
			}},
			Allocatable: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU: resource.MustParse("4"),
			},
			ProviderID: test.RandomProviderID(),
		})
		ExpectApplied(ctx, env.Client, pod1, pod2)
		ExpectApplied(ctx, env.Client, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))

		// two pods, but neither is bound to the node so the node's CPU requests should be zero
		ExpectResources(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("0.0")}, ExpectStateNodeExists(cluster, node).PodRequests())
	})
	It("should count new pods bound to nodes", func() {
		pod1 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.5"),
				}},
		})
		pod2 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("2"),
				}},
		})
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
			}},
			Allocatable: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU: resource.MustParse("4"),
			},
			ProviderID: test.RandomProviderID(),
		})
		ExpectApplied(ctx, env.Client, pod1, pod2)
		ExpectApplied(ctx, env.Client, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))

		ExpectManualBinding(ctx, env.Client, pod1, node)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))

		ExpectResources(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1.5")}, ExpectStateNodeExists(cluster, node).PodRequests())

		ExpectManualBinding(ctx, env.Client, pod2, node)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))
		ExpectResources(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("3.5")}, ExpectStateNodeExists(cluster, node).PodRequests())
	})
	It("should count existing pods bound to nodes", func() {
		pod1 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.5"),
				}},
		})
		pod2 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("2"),
				}},
		})
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
			}},
			Allocatable: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU: resource.MustParse("4"),
			},
			ProviderID: test.RandomProviderID(),
		})

		// simulate a node that already exists in our cluster
		ExpectApplied(ctx, env.Client, pod1, pod2)
		ExpectApplied(ctx, env.Client, node)
		ExpectManualBinding(ctx, env.Client, pod1, node)
		ExpectManualBinding(ctx, env.Client, pod2, node)

		// that we just noticed
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectResources(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("3.5")}, ExpectStateNodeExists(cluster, node).PodRequests())
	})
	It("should subtract requests if the pod is deleted", func() {
		pod1 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.5"),
				}},
		})
		pod2 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("2"),
				}},
		})
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
			}},
			Allocatable: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU: resource.MustParse("4"),
			},
			ProviderID: test.RandomProviderID(),
		})
		ExpectApplied(ctx, env.Client, pod1, pod2)
		ExpectApplied(ctx, env.Client, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))

		ExpectManualBinding(ctx, env.Client, pod1, node)
		ExpectManualBinding(ctx, env.Client, pod2, node)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))

		ExpectResources(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("3.5")}, ExpectStateNodeExists(cluster, node).PodRequests())

		// delete the pods and the CPU usage should go down
		ExpectDeleted(ctx, env.Client, pod2)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))
		ExpectResources(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1.5")}, ExpectStateNodeExists(cluster, node).PodRequests())

		ExpectDeleted(ctx, env.Client, pod1)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))
		ExpectResources(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("0")}, ExpectStateNodeExists(cluster, node).PodRequests())
	})
	It("should not add requests if the pod is terminal", func() {
		pod1 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.5"),
				}},
			Phase: corev1.PodFailed,
		})
		pod2 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("2"),
				}},
			Phase: corev1.PodSucceeded,
		})
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
			}},
			Allocatable: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU: resource.MustParse("4"),
			},
			ProviderID: test.RandomProviderID(),
		})
		ExpectApplied(ctx, env.Client, pod1, pod2)
		ExpectApplied(ctx, env.Client, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))

		ExpectManualBinding(ctx, env.Client, pod1, node)
		ExpectManualBinding(ctx, env.Client, pod2, node)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))

		ExpectResources(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("0")}, ExpectStateNodeExists(cluster, node).PodRequests())
	})
	It("should stop tracking nodes that are deleted", func() {
		pod1 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.5"),
				}},
		})
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
			}},
			Allocatable: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU: resource.MustParse("4"),
			},
			ProviderID: test.RandomProviderID(),
		})
		ExpectApplied(ctx, env.Client, pod1)
		ExpectApplied(ctx, env.Client, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))

		ExpectManualBinding(ctx, env.Client, pod1, node)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))

		for n := range cluster.Nodes() {
			ExpectResources(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2.5")}, n.Available())
			ExpectResources(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1.5")}, n.PodRequests())
		}

		// delete the node and the internal state should disappear as well
		ExpectDeleted(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		for range cluster.Nodes() {
			Fail("shouldn't be called as the node was deleted")
		}
	})
	It("should track pods correctly if we miss events or they are consolidated", func() {
		pod1 := test.UnschedulablePod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{Name: "stateful-set-pod"},
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.5"),
				}},
		})

		node1 := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
			}},
			Allocatable: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU: resource.MustParse("4"),
			},
			ProviderID: test.RandomProviderID(),
		})
		ExpectApplied(ctx, env.Client, pod1, node1)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node1))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))

		ExpectManualBinding(ctx, env.Client, pod1, node1)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))

		for n := range cluster.Nodes() {
			ExpectResources(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2.5")}, n.Available())
			ExpectResources(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1.5")}, n.PodRequests())
		}

		ExpectDeleted(ctx, env.Client, pod1)

		// second node has more capacity
		node2 := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
			}},
			Allocatable: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU: resource.MustParse("8"),
			},
			ProviderID: test.RandomProviderID(),
		})

		// and the pod can only bind to node2 due to the resource request
		pod2 := test.UnschedulablePod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{Name: "stateful-set-pod"},
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("5.0"),
				}},
		})

		ExpectApplied(ctx, env.Client, pod2, node2)
		ExpectManualBinding(ctx, env.Client, pod2, node2)
		// deleted the pod and then recreated it, but simulated only receiving an event on the new pod after it has
		// bound and not getting the new node event entirely
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node2))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))

		for n := range cluster.Nodes() {
			if n.Node.Name == node1.Name {
				// not on node1 any longer, so it should be fully free
				ExpectResources(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4")}, n.Available())
				ExpectResources(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("0")}, n.PodRequests())
			} else {
				ExpectResources(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("3")}, n.Available())
				ExpectResources(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("5")}, n.PodRequests())
			}
		}

	})
	// nolint:gosec
	It("should maintain a correct count of resource usage as pods are deleted/added", func() {
		var pods []*corev1.Pod
		for i := 0; i < 100; i++ {
			pods = append(pods, test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU: resource.MustParse(fmt.Sprintf("%1.1f", rand.Float64()*2)),
					}},
			}))
		}
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
			}},
			Allocatable: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:  resource.MustParse("200"),
				corev1.ResourcePods: resource.MustParse("500"),
			},
			ProviderID: test.RandomProviderID(),
		})
		ExpectApplied(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:  resource.MustParse("0"),
			corev1.ResourcePods: resource.MustParse("0"),
		}, ExpectStateNodeExists(cluster, node).PodRequests())
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

		sum := 0.0
		podCount := 0
		for _, pod := range pods {
			ExpectApplied(ctx, env.Client, pod)
			ExpectManualBinding(ctx, env.Client, pod, node)
			podCount++

			// extra reconciles shouldn't cause it to be multiply counted
			nReconciles := rand.Intn(3) + 1 // 1 to 3 reconciles
			for i := 0; i < nReconciles; i++ {
				ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod))
			}
			sum += pod.Spec.Containers[0].Resources.Requests.Cpu().AsApproximateFloat64()
			ExpectResources(corev1.ResourceList{
				corev1.ResourceCPU:  resource.MustParse(fmt.Sprintf("%1.1f", sum)),
				corev1.ResourcePods: resource.MustParse(fmt.Sprintf("%d", podCount)),
			}, ExpectStateNodeExists(cluster, node).PodRequests())
		}

		for _, pod := range pods {
			ExpectDeleted(ctx, env.Client, pod)
			nReconciles := rand.Intn(3) + 1
			// or multiply removed
			for i := 0; i < nReconciles; i++ {
				ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod))
			}
			sum -= pod.Spec.Containers[0].Resources.Requests.Cpu().AsApproximateFloat64()
			podCount--
			ExpectResources(corev1.ResourceList{
				corev1.ResourceCPU:  resource.MustParse(fmt.Sprintf("%1.1f", sum)),
				corev1.ResourcePods: resource.MustParse(fmt.Sprintf("%d", podCount)),
			}, ExpectStateNodeExists(cluster, node).PodRequests())
		}
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:  resource.MustParse("0"),
			corev1.ResourcePods: resource.MustParse("0"),
		}, ExpectStateNodeExists(cluster, node).PodRequests())
	})
	It("should track daemonset requested resources separately", func() {
		ds := test.DaemonSet(
			test.DaemonSetOptions{PodOptions: test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("2Gi")}},
			}},
		)
		ExpectApplied(ctx, env.Client, ds)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(ds), ds)).To(Succeed())

		pod1 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.5"),
				}},
		})

		dsPod := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				}},
		})
		dsPod.OwnerReferences = append(dsPod.OwnerReferences, metav1.OwnerReference{
			APIVersion:         "apps/v1",
			Kind:               "DaemonSet",
			Name:               ds.Name,
			UID:                ds.UID,
			Controller:         lo.ToPtr(true),
			BlockOwnerDeletion: lo.ToPtr(true),
		})

		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
			}},
			Allocatable: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
			ProviderID: test.RandomProviderID(),
		})
		ExpectApplied(ctx, env.Client, pod1, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

		ExpectManualBinding(ctx, env.Client, pod1, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))

		// daemonset pod isn't bound yet
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("0"),
			corev1.ResourceMemory: resource.MustParse("0"),
		}, ExpectStateNodeExists(cluster, node).DaemonSetRequests())
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1.5"),
		}, ExpectStateNodeExists(cluster, node).PodRequests())

		ExpectApplied(ctx, env.Client, dsPod)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(dsPod))
		ExpectManualBinding(ctx, env.Client, dsPod, node)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(dsPod))

		// just the DS request portion
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		}, ExpectStateNodeExists(cluster, node).DaemonSetRequests())
		// total request
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2.5"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		}, ExpectStateNodeExists(cluster, node).PodRequests())
	})
	It("should mark node for deletion when node is deleted", func() {
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
				},
				Finalizers: []string{v1.TerminationFinalizer},
			},
			Allocatable: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU: resource.MustParse("4"),
			},
			ProviderID: test.RandomProviderID(),
		})
		ExpectApplied(ctx, env.Client, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectStateNodeCount("==", 1)

		Expect(env.Client.Delete(ctx, node)).To(Succeed())

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectNodeExists(ctx, env.Client, node.Name)
		Expect(ExpectStateNodeExists(cluster, node).MarkedForDeletion()).To(BeTrue())
	})
	It("should mark node for deletion when nodeclaim is deleted", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{v1.TerminationFinalizer},
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
				},
			},
			Spec: v1.NodeClaimSpec{
				Requirements: []v1.NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      corev1.LabelInstanceTypeStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{cloudProvider.InstanceTypes[0].Name},
						},
					},
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      corev1.LabelTopologyZone,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"test-zone-1"},
						},
					},
				},
				NodeClassRef: &v1.NodeClassReference{
					Group: "karpenter.test.sh",
					Kind:  "TestNodeClass",
					Name:  "default",
				},
			},
			Status: v1.NodeClaimStatus{
				ProviderID: test.RandomProviderID(),
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("2"),
					corev1.ResourceMemory:           resource.MustParse("32Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("20Gi"),
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("1"),
					corev1.ResourceMemory:           resource.MustParse("30Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("18Gi"),
				},
			},
		})
		node := test.NodeClaimLinkedNode(nodeClaim)
		ExpectApplied(ctx, env.Client, nodeClaim, node)
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectStateNodeCount("==", 1)

		Expect(env.Client.Delete(ctx, nodeClaim)).To(Succeed())
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
		ExpectExists(ctx, env.Client, nodeClaim)

		Expect(ExpectStateNodeExistsForNodeClaim(cluster, nodeClaim).MarkedForDeletion()).To(BeTrue())
		Expect(ExpectStateNodeExists(cluster, node).MarkedForDeletion()).To(BeTrue())
	})
	It("should nominate the node until the nomination time passes", func() {
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            nodePool.Name,
					corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
				},
				Finalizers: []string{v1.TerminationFinalizer},
			},
			Allocatable: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU: resource.MustParse("4"),
			},
			ProviderID: test.RandomProviderID(),
		})
		ExpectApplied(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

		cluster.NominateNodeForPod(ctx, node.Spec.ProviderID)

		// Expect that the node is now nominated
		Expect(ExpectStateNodeExists(cluster, node).Nominated()).To(BeTrue())
		time.Sleep(time.Second * 10) // nomination window is 20s so it should still be nominated
		Expect(ExpectStateNodeExists(cluster, node).Nominated()).To(BeTrue())
		time.Sleep(time.Second * 11) // past 20s, node should no longer be nominated
		Expect(ExpectStateNodeExists(cluster, node).Nominated()).To(BeFalse())
	})
	It("should handle a node changing from no providerID to registering a providerID", func() {
		node := test.Node()
		ExpectApplied(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

		ExpectStateNodeCount("==", 1)
		ExpectStateNodeExists(cluster, node)

		// Change the providerID; this mocks CCM adding the providerID onto the node after registration
		node.Spec.ProviderID = fmt.Sprintf("fake://%s", node.Name)
		ExpectApplied(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

		ExpectStateNodeCount("==", 1)
		ExpectStateNodeExists(cluster, node)
	})
})

var _ = Describe("Pod Anti-Affinity", func() {
	It("should track pods with required anti-affinity", func() {
		pod := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.5"),
				}},
			PodAntiRequirements: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					TopologyKey: corev1.LabelTopologyZone,
				},
			},
		})

		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
			}},
			Allocatable: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU: resource.MustParse("4"),
			},
			ProviderID: test.RandomProviderID(),
		})

		ExpectApplied(ctx, env.Client, pod)
		ExpectApplied(ctx, env.Client, node)
		ExpectManualBinding(ctx, env.Client, pod, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod))
		foundPodCount := 0
		cluster.ForPodsWithAntiAffinity(func(p *corev1.Pod, n *corev1.Node) bool {
			foundPodCount++
			Expect(p.Name).To(Equal(pod.Name))
			return true
		})
		Expect(foundPodCount).To(BeNumerically("==", 1))
	})
	It("should not track pods with preferred anti-affinity", func() {
		pod := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.5"),
				}},
			PodAntiPreferences: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 15,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"foo": "bar"},
						},
						TopologyKey: corev1.LabelTopologyZone,
					},
				},
			},
		})

		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
			}},
			Allocatable: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU: resource.MustParse("4"),
			},
			ProviderID: test.RandomProviderID(),
		})

		ExpectApplied(ctx, env.Client, pod)
		ExpectApplied(ctx, env.Client, node)
		ExpectManualBinding(ctx, env.Client, pod, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod))
		foundPodCount := 0
		cluster.ForPodsWithAntiAffinity(func(p *corev1.Pod, n *corev1.Node) bool {
			foundPodCount++
			Fail("shouldn't track pods with preferred anti-affinity")
			return true
		})
		Expect(foundPodCount).To(BeNumerically("==", 0))
	})
	It("should stop tracking pods with required anti-affinity if the pod is deleted", func() {
		pod := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.5"),
				}},
			PodAntiRequirements: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					TopologyKey: corev1.LabelTopologyZone,
				},
			},
		})

		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
			}},
			Allocatable: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU: resource.MustParse("4"),
			},
			ProviderID: test.RandomProviderID(),
		})

		ExpectApplied(ctx, env.Client, pod)
		ExpectApplied(ctx, env.Client, node)
		ExpectManualBinding(ctx, env.Client, pod, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod))
		foundPodCount := 0
		cluster.ForPodsWithAntiAffinity(func(p *corev1.Pod, n *corev1.Node) bool {
			foundPodCount++
			Expect(p.Name).To(Equal(pod.Name))
			return true
		})
		Expect(foundPodCount).To(BeNumerically("==", 1))

		ExpectDeleted(ctx, env.Client, client.Object(pod))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod))
		foundPodCount = 0
		cluster.ForPodsWithAntiAffinity(func(p *corev1.Pod, n *corev1.Node) bool {
			foundPodCount++
			Fail("should not be called as the pod was deleted")
			return true
		})
		Expect(foundPodCount).To(BeNumerically("==", 0))
	})
	It("should handle events out of order", func() {
		pod := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.5"),
				}},
			PodAntiRequirements: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					TopologyKey: corev1.LabelTopologyZone,
				},
			},
		})

		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				v1.NodePoolLabelKey:            nodePool.Name,
				corev1.LabelInstanceTypeStable: cloudProvider.InstanceTypes[0].Name,
			}},
			Allocatable: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU: resource.MustParse("4"),
			},
			ProviderID: test.RandomProviderID(),
		})

		ExpectApplied(ctx, env.Client, pod)
		ExpectApplied(ctx, env.Client, node)
		ExpectManualBinding(ctx, env.Client, pod, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod))

		// simulate receiving the node deletion before the pod deletion
		ExpectDeleted(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

		foundPodCount := 0
		cluster.ForPodsWithAntiAffinity(func(p *corev1.Pod, n *corev1.Node) bool {
			foundPodCount++
			return true
		})
		Expect(foundPodCount).To(BeNumerically("==", 0))
	})
})

var _ = Describe("Cluster State Sync", func() {
	It("should consider the cluster state synced when all nodes are tracked", func() {
		// Deploy 1000 nodes and sync them all with the cluster
		var wg sync.WaitGroup
		for range 1000 {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				node := test.Node(test.NodeOptions{
					ProviderID: test.RandomProviderID(),
				})
				ExpectApplied(ctx, env.Client, node)
				ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
			}()
		}
		wg.Wait()

		Expect(cluster.Synced(ctx)).To(BeTrue())
		ExpectMetricGaugeValue(state.ClusterStateSynced, 1.0, nil)
		ExpectMetricGaugeValue(state.ClusterStateNodesCount, 1000.0, nil)
		metric, found := FindMetricWithLabelValues("karpenter_cluster_state_unsynced_time_seconds", map[string]string{})
		Expect(found).To(BeTrue())
		Expect(metric.GetGauge().GetValue()).To(BeEquivalentTo(0))
	})
	It("should emit cluster_state_unsynced_time_seconds metric when cluster state is unsynced", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			Status: v1.NodeClaimStatus{
				ProviderID: "",
			},
		})
		nodeClaim.Status.ProviderID = ""
		ExpectApplied(ctx, env.Client, nodeClaim)
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
		Expect(cluster.Synced(ctx)).To(BeFalse())

		fakeClock.Step(2 * time.Minute)
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
		Expect(cluster.Synced(ctx)).To(BeFalse())
		metric, found := FindMetricWithLabelValues("karpenter_cluster_state_unsynced_time_seconds", map[string]string{})
		Expect(found).To(BeTrue())
		Expect(metric.GetGauge().GetValue()).To(BeNumerically(">=", 120))
	})
	It("should consider the cluster state synced when nodes don't have provider id", func() {
		// Deploy 1000 nodes and sync them all with the cluster
		var wg sync.WaitGroup
		for range 1000 {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				node := test.Node()
				ExpectApplied(ctx, env.Client, node)
				ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
			}()
		}
		wg.Wait()
		Expect(cluster.Synced(ctx)).To(BeTrue())
		ExpectMetricGaugeValue(state.ClusterStateSynced, 1.0, nil)
		ExpectMetricGaugeValue(state.ClusterStateNodesCount, 1000.0, nil)

	})
	It("should consider the cluster state synced when nodes register provider id", func() {
		// Deploy 1000 nodes and sync them all with the cluster
		nodes := make([]*corev1.Node, 1000)
		var wg sync.WaitGroup
		for i := range 1000 {
			wg.Add(1)
			go func(index int) {
				defer GinkgoRecover()
				defer wg.Done()
				node := test.Node()
				ExpectApplied(ctx, env.Client, node)
				ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
				nodes[index] = node
			}(i)
		}
		wg.Wait()
		ExpectMetricGaugeValue(state.ClusterStateNodesCount, 1000.0, nil)
		Expect(cluster.Synced(ctx)).To(BeTrue())
		for i := range 1000 {
			wg.Add(1)
			go func(index int) {
				defer GinkgoRecover()
				defer wg.Done()
				nodes[index].Spec.ProviderID = test.RandomProviderID()
				ExpectApplied(ctx, env.Client, nodes[index])
				ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(nodes[index]))
			}(i)
		}
		wg.Wait()
		Expect(cluster.Synced(ctx)).To(BeTrue())
		ExpectMetricGaugeValue(state.ClusterStateSynced, 1.0, nil)
		ExpectMetricGaugeValue(state.ClusterStateNodesCount, 1000.0, nil)
	})
	It("should consider the cluster state synced when all nodeclaims are tracked", func() {
		// Deploy 1000 nodeClaims and sync them all with the cluster
		var wg sync.WaitGroup
		for range 1000 {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				nodeClaim := test.NodeClaim(v1.NodeClaim{
					Status: v1.NodeClaimStatus{
						ProviderID: test.RandomProviderID(),
					},
				})
				ExpectApplied(ctx, env.Client, nodeClaim)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
			}()
		}
		wg.Wait()
		Expect(cluster.Synced(ctx)).To(BeTrue())
	})
	It("should consider the cluster state synced when a combination of nodeclaims and node are tracked", func() {
		// Deploy 250 nodes to the cluster that also have nodeclaims
		var wg sync.WaitGroup
		for range 250 {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				node := test.Node(test.NodeOptions{
					ProviderID: test.RandomProviderID(),
				})
				nodeClaim := test.NodeClaim(v1.NodeClaim{
					Status: v1.NodeClaimStatus{
						ProviderID: node.Spec.ProviderID,
					},
				})
				ExpectApplied(ctx, env.Client, nodeClaim, node)
				ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
			}()
		}
		wg.Wait()
		// Deploy 250 nodes to the cluster
		for range 250 {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				node := test.Node(test.NodeOptions{
					ProviderID: test.RandomProviderID(),
				})
				ExpectApplied(ctx, env.Client, node)
				ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
			}()
		}
		wg.Wait()
		// Deploy 500 nodeclaims and sync them all with the cluster
		for range 500 {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				nodeClaim := test.NodeClaim(v1.NodeClaim{
					Status: v1.NodeClaimStatus{
						ProviderID: test.RandomProviderID(),
					},
				})
				ExpectApplied(ctx, env.Client, nodeClaim)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
			}()
		}
		wg.Wait()
		Expect(cluster.Synced(ctx)).To(BeTrue())
	})
	It("should consider the cluster state synced when the representation of nodes is the same", func() {
		// Deploy 500 nodeClaims to the cluster, apply the linked nodes, but don't sync them
		var wg sync.WaitGroup
		for range 500 {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				nodeClaim := test.NodeClaim(v1.NodeClaim{
					Status: v1.NodeClaimStatus{
						ProviderID: test.RandomProviderID(),
					},
				})
				node := test.Node(test.NodeOptions{
					ProviderID: nodeClaim.Status.ProviderID,
				})
				ExpectApplied(ctx, env.Client, nodeClaim, node)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
				ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
			}()
		}
		wg.Wait()
		Expect(cluster.Synced(ctx)).To(BeTrue())
	})
	It("shouldn't consider the cluster state synced if a nodeclaim hasn't resolved its provider id", func() {
		// Deploy 1000 nodeClaims and sync them all with the cluster
		var wg sync.WaitGroup
		for i := range 1000 {
			wg.Add(1)
			go func(index int) {
				defer GinkgoRecover()
				defer wg.Done()
				nodeClaim := test.NodeClaim(v1.NodeClaim{
					Status: v1.NodeClaimStatus{
						ProviderID: test.RandomProviderID(),
					},
				})
				// One of them doesn't have its providerID
				if index == 900 {
					nodeClaim.Status.ProviderID = ""
				}
				ExpectApplied(ctx, env.Client, nodeClaim)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
			}(i)
		}
		wg.Wait()
		Expect(cluster.Synced(ctx)).To(BeFalse())
	})
	It("shouldn't consider the cluster state synced if a nodeclaim isn't tracked", func() {
		// Deploy 1000 nodeClaims and sync them all with the cluster
		var wg sync.WaitGroup
		for i := range 1000 {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				nodeClaim := test.NodeClaim(v1.NodeClaim{
					Status: v1.NodeClaimStatus{
						ProviderID: test.RandomProviderID(),
					},
				})
				ExpectApplied(ctx, env.Client, nodeClaim)

				// One of them doesn't get synced with the reconciliation
				if i != 900 {
					ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
				}
			}()
		}
		wg.Wait()
		Expect(cluster.Synced(ctx)).To(BeFalse())
	})
	It("shouldn't consider the cluster state synced if a node isn't tracked", func() {
		// Deploy 1000 nodes and sync them all with the cluster
		var wg sync.WaitGroup
		for i := range 1000 {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				node := test.Node(test.NodeOptions{
					ProviderID: test.RandomProviderID(),
				})
				ExpectApplied(ctx, env.Client, node)

				// One of them doesn't get synced with the reconciliation
				if i != 900 {
					ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
				}
			}()
		}
		wg.Wait()
		Expect(cluster.Synced(ctx)).To(BeFalse())
		ExpectMetricGaugeValue(state.ClusterStateSynced, 0, nil)
	})
	It("shouldn't consider the cluster state synced if a nodeclaim is added manually with UpdateNodeClaim", func() {
		nodeClaim := test.NodeClaim()
		nodeClaim.Status.ProviderID = ""

		cluster.UpdateNodeClaim(nodeClaim)
		Expect(cluster.Synced(ctx)).To(BeFalse())
	})
	It("shouldn't consider the cluster state synced if a nodeclaim without a providerID is deleted", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{})
		nodeClaim.Status.ProviderID = ""

		cluster.UpdateNodeClaim(nodeClaim)
		Expect(cluster.Synced(ctx)).To(BeFalse())
		ExpectMetricGaugeValue(state.ClusterStateSynced, 0, nil)

		ExpectApplied(ctx, env.Client, nodeClaim)
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
		Expect(cluster.Synced(ctx)).To(BeFalse())
		ExpectMetricGaugeValue(state.ClusterStateSynced, 0, nil)

		ExpectDeleted(ctx, env.Client, nodeClaim)
		ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
		Expect(cluster.Synced(ctx)).To(BeTrue())
		ExpectMetricGaugeValue(state.ClusterStateSynced, 1, nil)
	})
	// also this test takes a while still
	It("should consider the cluster state synced when a new node is added after the initial sync", func() {
		// Deploy 250 nodes to the cluster that also have nodeclaims
		var wg sync.WaitGroup
		for range 250 {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				node := test.Node(test.NodeOptions{
					ProviderID: test.RandomProviderID(),
				})
				nodeClaim := test.NodeClaim(v1.NodeClaim{
					Status: v1.NodeClaimStatus{
						ProviderID: node.Spec.ProviderID,
					},
				})
				ExpectApplied(ctx, env.Client, node, nodeClaim)
				ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
			}()
		}
		wg.Wait()
		// Deploy 250 nodes to the cluster
		for range 250 {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				node := test.Node(test.NodeOptions{
					ProviderID: test.RandomProviderID(),
				})
				ExpectApplied(ctx, env.Client, node)
				ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
			}()
		}
		wg.Wait()
		Expect(cluster.Synced(ctx)).To(BeTrue())

		// Add a new node but don't reconcile it
		node := test.Node(test.NodeOptions{
			ProviderID: test.RandomProviderID(),
		})
		ExpectApplied(ctx, env.Client, node)

		// Cluster state should still be synced because we already synced our changes
		Expect(cluster.Synced(ctx)).To(BeTrue())
	})
})

var _ = Describe("DaemonSet Controller", func() {
	It("should not update daemonsetCache when daemonset pod is not present", func() {
		daemonset := test.DaemonSet(
			test.DaemonSetOptions{PodOptions: test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
			}},
		)
		ExpectApplied(ctx, env.Client, daemonset)
		ExpectReconcileSucceeded(ctx, daemonsetController, client.ObjectKeyFromObject(daemonset))
		daemonsetPod := cluster.GetDaemonSetPod(daemonset)
		Expect(daemonsetPod).To(BeNil())
	})
	It("should update daemonsetCache when daemonset pod is created", func() {
		daemonset := test.DaemonSet(
			test.DaemonSetOptions{PodOptions: test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
			}},
		)
		ExpectApplied(ctx, env.Client, daemonset)
		daemonsetPod := test.UnschedulablePod(
			test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "apps/v1",
							Kind:               "DaemonSet",
							Name:               daemonset.Name,
							UID:                daemonset.UID,
							Controller:         lo.ToPtr(true),
							BlockOwnerDeletion: lo.ToPtr(true),
						},
					},
				},
			})
		daemonsetPod.Spec = daemonset.Spec.Template.Spec
		ExpectApplied(ctx, env.Client, daemonsetPod)
		ExpectReconcileSucceeded(ctx, daemonsetController, client.ObjectKeyFromObject(daemonset))

		Expect(cluster.GetDaemonSetPod(daemonset)).To(Equal(daemonsetPod))
	})
	It("should update daemonsetCache with the newest created pod", func() {
		daemonset := test.DaemonSet(
			test.DaemonSetOptions{PodOptions: test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
			}},
		)
		ExpectApplied(ctx, env.Client, daemonset)
		daemonsetPod1 := test.UnschedulablePod(
			test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "apps/v1",
							Kind:               "DaemonSet",
							Name:               daemonset.Name,
							UID:                daemonset.UID,
							Controller:         lo.ToPtr(true),
							BlockOwnerDeletion: lo.ToPtr(true),
						},
					},
				},
			})
		daemonsetPod1.Spec = daemonset.Spec.Template.Spec
		ExpectApplied(ctx, env.Client, daemonsetPod1)
		ExpectReconcileSucceeded(ctx, daemonsetController, client.ObjectKeyFromObject(daemonset))

		Expect(cluster.GetDaemonSetPod(daemonset)).To(Equal(daemonsetPod1))

		daemonsetPod2 := test.UnschedulablePod(
			test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "apps/v1",
							Kind:               "DaemonSet",
							Name:               daemonset.Name,
							UID:                daemonset.UID,
							Controller:         lo.ToPtr(true),
							BlockOwnerDeletion: lo.ToPtr(true),
						},
					},
				},
			})
		time.Sleep(time.Second) // Making sure the two pods have different creationTime
		daemonsetPod2.Spec = daemonset.Spec.Template.Spec
		ExpectApplied(ctx, env.Client, daemonsetPod2)
		ExpectReconcileSucceeded(ctx, daemonsetController, client.ObjectKeyFromObject(daemonset))
		Expect(cluster.GetDaemonSetPod(daemonset)).To(Equal(daemonsetPod2))
	})
	It("should delete daemonset in cache when daemonset is deleted", func() {
		daemonset := test.DaemonSet(
			test.DaemonSetOptions{PodOptions: test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
			}},
		)
		ExpectApplied(ctx, env.Client, daemonset)
		daemonsetPod := test.UnschedulablePod(
			test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "apps/v1",
							Kind:               "DaemonSet",
							Name:               daemonset.Name,
							UID:                daemonset.UID,
							Controller:         lo.ToPtr(true),
							BlockOwnerDeletion: lo.ToPtr(true),
						},
					},
				},
			})
		daemonsetPod.Spec = daemonset.Spec.Template.Spec
		ExpectApplied(ctx, env.Client, daemonsetPod)
		ExpectReconcileSucceeded(ctx, daemonsetController, client.ObjectKeyFromObject(daemonset))

		Expect(cluster.GetDaemonSetPod(daemonset)).To(Equal(daemonsetPod))

		ExpectDeleted(ctx, env.Client, daemonset, daemonsetPod)
		ExpectReconcileSucceeded(ctx, daemonsetController, client.ObjectKeyFromObject(daemonset))

		Expect(cluster.GetDaemonSetPod(daemonset)).To(BeNil())
	})
	It("should only return daemonset pods from the daemonset cache", func() {
		daemonset := test.DaemonSet(
			test.DaemonSetOptions{PodOptions: test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")}},
			}},
		)
		ExpectApplied(ctx, env.Client, daemonset)
		otherPods := test.Pods(1000, test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: daemonset.Namespace,
			},
		})
		ExpectApplied(ctx, env.Client, lo.Map(otherPods, func(p *corev1.Pod, _ int) client.Object { return p })...)
		ExpectReconcileSucceeded(ctx, daemonsetController, client.ObjectKeyFromObject(daemonset))
		Expect(cluster.GetDaemonSetPod(daemonset)).To(BeNil())
	})
})

var _ = Describe("Consolidated State", func() {
	It("should update the consolidated value when setting consolidation", func() {
		state := cluster.ConsolidationState()
		Expect(cluster.ConsolidationState()).To(Equal(state))

		// time must pass
		fakeClock.Step(1 * time.Second)

		cluster.MarkUnconsolidated()
		Expect(cluster.ConsolidationState()).ToNot(Equal(state))
	})
	It("should update the consolidated value when state timeout (5m) has passed and state hasn't changed", func() {
		state := cluster.ConsolidationState()

		fakeClock.Step(time.Minute)
		Expect(cluster.ConsolidationState()).To(Equal(state))

		fakeClock.Step(time.Minute * 2)
		Expect(cluster.ConsolidationState()).To(Equal(state))

		fakeClock.Step(time.Minute * 2)
		Expect(cluster.ConsolidationState()).ToNot(Equal(state))
	})
	It("should cause consolidation state to change when a NodePool is updated", func() {
		cluster.MarkUnconsolidated()
		fakeClock.Step(time.Minute)
		ExpectApplied(ctx, env.Client, nodePool)
		state := cluster.ConsolidationState()
		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		Expect(cluster.ConsolidationState()).ToNot(Equal(state))
	})
})

var _ = Describe("Data Races", func() {
	var wg sync.WaitGroup
	var cancelCtx context.Context
	var cancel context.CancelFunc
	BeforeEach(func() {
		cancelCtx, cancel = context.WithCancel(ctx)
	})
	AfterEach(func() {
		cancel()
		wg.Wait()
	})
	It("should ensure that calling Synced() is valid while making updates to Nodes", func() {
		// Keep calling Synced for the entirety of this test
		wg.Add(1)
		go func() {
			defer GinkgoRecover()
			defer wg.Done()
			for {
				_ = cluster.Synced(ctx)
				if cancelCtx.Err() != nil {
					return
				}
			}
		}()

		// Call UpdateNode on 100 nodes (enough to trigger a DATA RACE)
		for i := 0; i < 100; i++ {
			node := test.Node(test.NodeOptions{
				ProviderID: test.RandomProviderID(),
			})
			ExpectApplied(ctx, env.Client, node)
			ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		}
	})
	It("should ensure that calling Synced() is valid while making updates to NodeClaims", func() {
		// Keep calling Synced for the entirety of this test
		wg.Add(1)
		go func() {
			defer GinkgoRecover()
			defer wg.Done()
			for {
				_ = cluster.Synced(ctx)
				if cancelCtx.Err() != nil {
					return
				}
			}
		}()

		// Call UpdateNodeClaim on 100 NodeClaims (enough to trigger a DATA RACE)
		for i := 0; i < 100; i++ {
			nodeClaim := test.NodeClaim(v1.NodeClaim{
				Status: v1.NodeClaimStatus{
					ProviderID: test.RandomProviderID(),
				},
			})
			ExpectApplied(ctx, env.Client, nodeClaim)
			ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
		}
	})
})

var _ = Describe("Taints", func() {
	var nodeClaim *v1.NodeClaim
	var node *corev1.Node
	BeforeEach(func() {
		instanceType := cloudProvider.InstanceTypes[0]
		nodeClaim, node = test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
				corev1.LabelInstanceTypeStable: instanceType.Name,
			}},
			Status: v1.NodeClaimStatus{
				ProviderID: test.RandomProviderID(),
			},
		})
	})
	Context("Managed", func() {
		It("should not consider ephemeral taints on a managed node that isn't initialized", func() {
			node.Spec.Taints = []corev1.Taint{
				{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoSchedule},
				{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoExecute},
				{Key: corev1.TaintNodeUnreachable, Effect: corev1.TaintEffectNoSchedule},
				{Key: cloudproviderapi.TaintExternalCloudProvider, Effect: corev1.TaintEffectNoSchedule, Value: "true"},
			}
			ExpectApplied(ctx, env.Client, nodeClaim, node)
			ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
			ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

			stateNode := ExpectStateNodeExists(cluster, node)
			Expect(stateNode.Taints()).To(HaveLen(0))
		})
		It("should consider ephemeral taints on a managed node after the node is initialized", func() {
			ExpectApplied(ctx, env.Client, nodeClaim, node)
			ExpectMakeNodesInitialized(ctx, env.Client, node)
			ExpectMakeNodeClaimsInitialized(ctx, env.Client, nodeClaim)

			node = ExpectExists(ctx, env.Client, node)
			node.Spec.Taints = []corev1.Taint{
				{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoSchedule},
				{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoExecute},
				{Key: corev1.TaintNodeUnreachable, Effect: corev1.TaintEffectNoSchedule},
				{Key: cloudproviderapi.TaintExternalCloudProvider, Effect: corev1.TaintEffectNoSchedule, Value: "true"},
			}
			ExpectApplied(ctx, env.Client, node)

			ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
			ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

			stateNode := ExpectStateNodeExists(cluster, node)
			Expect(stateNode.Taints()).To(HaveLen(4))
			Expect(stateNode.Taints()).To(ContainElements(
				corev1.Taint{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoSchedule},
				corev1.Taint{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoExecute},
				corev1.Taint{Key: corev1.TaintNodeUnreachable, Effect: corev1.TaintEffectNoSchedule},
				corev1.Taint{Key: cloudproviderapi.TaintExternalCloudProvider, Effect: corev1.TaintEffectNoSchedule, Value: "true"},
			))
		})
		It("should consider startup taints on a managed node that isn't initialized", func() {
			nodeClaim.Spec.StartupTaints = []corev1.Taint{
				{Key: "taint-key", Value: "taint-value", Effect: corev1.TaintEffectNoSchedule},
				{Key: "taint-key2", Value: "taint-value2", Effect: corev1.TaintEffectNoExecute},
			}
			node.Spec.Taints = []corev1.Taint{
				{Key: "taint-key", Value: "taint-value", Effect: corev1.TaintEffectNoSchedule},
				{Key: "taint-key2", Value: "taint-value2", Effect: corev1.TaintEffectNoExecute},
			}
			ExpectApplied(ctx, env.Client, nodeClaim, node)
			ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
			ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

			stateNode := ExpectStateNodeExists(cluster, node)
			Expect(stateNode.Taints()).To(HaveLen(0))
		})
		It("should consider startup taints on a managed node after the node is initialized", func() {
			nodeClaim.Spec.StartupTaints = []corev1.Taint{
				{Key: "taint-key", Value: "taint-value", Effect: corev1.TaintEffectNoSchedule},
				{Key: "taint-key2", Value: "taint-value2", Effect: corev1.TaintEffectNoExecute},
			}
			node.Spec.Taints = []corev1.Taint{
				{Key: "taint-key", Value: "taint-value", Effect: corev1.TaintEffectNoSchedule},
				{Key: "taint-key2", Value: "taint-value2", Effect: corev1.TaintEffectNoExecute},
			}
			ExpectApplied(ctx, env.Client, nodeClaim, node)
			ExpectMakeNodesInitialized(ctx, env.Client, node)
			ExpectMakeNodeClaimsInitialized(ctx, env.Client, nodeClaim)

			ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
			ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

			stateNode := ExpectStateNodeExists(cluster, node)
			Expect(stateNode.Taints()).To(HaveLen(2))
			Expect(stateNode.Taints()).To(ContainElements(
				corev1.Taint{Key: "taint-key", Value: "taint-value", Effect: corev1.TaintEffectNoSchedule},
				corev1.Taint{Key: "taint-key2", Value: "taint-value2", Effect: corev1.TaintEffectNoExecute},
			))
		})
	})
	Context("Unmanaged", func() {
		It("should consider ephemeral taints on an unmanaged node that isn't initialized", func() {
			node.Spec.Taints = []corev1.Taint{
				{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoSchedule},
				{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoExecute},
				{Key: corev1.TaintNodeUnreachable, Effect: corev1.TaintEffectNoSchedule},
				{Key: cloudproviderapi.TaintExternalCloudProvider, Effect: corev1.TaintEffectNoSchedule, Value: "true"},
			}
			ExpectApplied(ctx, env.Client, node)
			ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

			stateNode := ExpectStateNodeExists(cluster, node)
			Expect(stateNode.Taints()).To(HaveLen(4))
			Expect(stateNode.Taints()).To(ContainElements(
				corev1.Taint{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoSchedule},
				corev1.Taint{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoExecute},
				corev1.Taint{Key: corev1.TaintNodeUnreachable, Effect: corev1.TaintEffectNoSchedule},
				corev1.Taint{Key: cloudproviderapi.TaintExternalCloudProvider, Effect: corev1.TaintEffectNoSchedule, Value: "true"},
			))
		})
		It("should consider ephemeral taints on an unmanaged node after the node is initialized", func() {
			ExpectApplied(ctx, env.Client, node)
			ExpectMakeNodesInitialized(ctx, env.Client, node)

			node = ExpectExists(ctx, env.Client, node)
			node.Spec.Taints = []corev1.Taint{
				{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoSchedule},
				{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoExecute},
				{Key: corev1.TaintNodeUnreachable, Effect: corev1.TaintEffectNoSchedule},
				{Key: cloudproviderapi.TaintExternalCloudProvider, Effect: corev1.TaintEffectNoSchedule, Value: "true"},
			}

			ExpectApplied(ctx, env.Client, node)
			ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

			stateNode := ExpectStateNodeExists(cluster, node)
			Expect(stateNode.Taints()).To(HaveLen(4))
			Expect(stateNode.Taints()).To(ContainElements(
				corev1.Taint{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoSchedule},
				corev1.Taint{Key: corev1.TaintNodeNotReady, Effect: corev1.TaintEffectNoExecute},
				corev1.Taint{Key: corev1.TaintNodeUnreachable, Effect: corev1.TaintEffectNoSchedule},
				corev1.Taint{Key: cloudproviderapi.TaintExternalCloudProvider, Effect: corev1.TaintEffectNoSchedule, Value: "true"},
			))
		})
	})
})

var _ = Describe("NodePool Resources", func() {
	It("should calculate nodepool resources for multiple nodepools", func() {
		nodePool1NodeClaims, nodePool1Nodes := test.NodeClaimsAndNodes(3, v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            test.RandomName(),
					v1.NodeRegisteredLabelKey:      "true",
					corev1.LabelInstanceTypeStable: "m5.large",
				},
			},
			Status: v1.NodeClaimStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("2"),
					corev1.ResourceMemory:           resource.MustParse("2Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("1"),
					corev1.ResourceMemory:           resource.MustParse("1Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
				},
			},
		})
		nodePool2NodeClaims, nodePool2Nodes := test.NodeClaimsAndNodes(3, v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            test.RandomName(),
					v1.NodeRegisteredLabelKey:      "true",
					corev1.LabelInstanceTypeStable: "m5.large",
				},
			},
			Status: v1.NodeClaimStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("4"),
					corev1.ResourceMemory:           resource.MustParse("4Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("4Gi"),
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("3"),
					corev1.ResourceMemory:           resource.MustParse("3Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("3Gi"),
				},
			},
		})
		nodePool3NodeClaims, nodePool3Nodes := test.NodeClaimsAndNodes(3, v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            test.RandomName(),
					v1.NodeRegisteredLabelKey:      "true",
					corev1.LabelInstanceTypeStable: "m5.large",
				},
			},
			Status: v1.NodeClaimStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("6"),
					corev1.ResourceMemory:           resource.MustParse("6Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("6Gi"),
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("5"),
					corev1.ResourceMemory:           resource.MustParse("5Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("5Gi"),
				},
			},
		})
		for _, n := range lo.Flatten([][]*v1.NodeClaim{nodePool1NodeClaims, nodePool2NodeClaims, nodePool3NodeClaims}) {
			cluster.UpdateNodeClaim(n.DeepCopy())
		}
		for _, n := range lo.Flatten([][]*corev1.Node{nodePool1Nodes, nodePool2Nodes, nodePool3Nodes}) {
			Expect(cluster.UpdateNode(ctx, n.DeepCopy())).To(Succeed())
		}
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("6"),
			corev1.ResourceMemory:           resource.MustParse("6Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("6Gi"),
		}, cluster.NodePoolResourcesFor(nodePool1NodeClaims[0].Labels[v1.NodePoolLabelKey]))
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("12"),
			corev1.ResourceMemory:           resource.MustParse("12Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("12Gi"),
		}, cluster.NodePoolResourcesFor(nodePool2NodeClaims[0].Labels[v1.NodePoolLabelKey]))
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("18"),
			corev1.ResourceMemory:           resource.MustParse("18Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("18Gi"),
		}, cluster.NodePoolResourcesFor(nodePool3NodeClaims[0].Labels[v1.NodePoolLabelKey]))

		// Now delete the Nodes and ensure that we keep the resources up-to-date
		cluster.DeleteNode(nodePool1Nodes[len(nodePool1Nodes)-1].Name)
		cluster.DeleteNode(nodePool2Nodes[len(nodePool2Nodes)-1].Name)
		cluster.DeleteNode(nodePool3Nodes[len(nodePool3Nodes)-1].Name)

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("6"),
			corev1.ResourceMemory:           resource.MustParse("6Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("6Gi"),
		}, cluster.NodePoolResourcesFor(nodePool1NodeClaims[0].Labels[v1.NodePoolLabelKey]))
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("12"),
			corev1.ResourceMemory:           resource.MustParse("12Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("12Gi"),
		}, cluster.NodePoolResourcesFor(nodePool2NodeClaims[0].Labels[v1.NodePoolLabelKey]))
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("18"),
			corev1.ResourceMemory:           resource.MustParse("18Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("18Gi"),
		}, cluster.NodePoolResourcesFor(nodePool3NodeClaims[0].Labels[v1.NodePoolLabelKey]))

		// Now delete the NodeClaims to fully delete the state node
		cluster.DeleteNodeClaim(nodePool1NodeClaims[len(nodePool1NodeClaims)-1].Name)
		cluster.DeleteNodeClaim(nodePool2NodeClaims[len(nodePool2NodeClaims)-1].Name)
		cluster.DeleteNodeClaim(nodePool3NodeClaims[len(nodePool3NodeClaims)-1].Name)

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("4"),
			corev1.ResourceMemory:           resource.MustParse("4Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("4Gi"),
		}, cluster.NodePoolResourcesFor(nodePool1NodeClaims[0].Labels[v1.NodePoolLabelKey]))
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("8"),
			corev1.ResourceMemory:           resource.MustParse("8Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("8Gi"),
		}, cluster.NodePoolResourcesFor(nodePool2NodeClaims[0].Labels[v1.NodePoolLabelKey]))
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("12"),
			corev1.ResourceMemory:           resource.MustParse("12Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("12Gi"),
		}, cluster.NodePoolResourcesFor(nodePool3NodeClaims[0].Labels[v1.NodePoolLabelKey]))

		// Now delete all nodes and expect resources across all NodePools is zero
		for _, n := range lo.Flatten([][]*v1.NodeClaim{nodePool1NodeClaims, nodePool2NodeClaims, nodePool3NodeClaims}) {
			cluster.DeleteNodeClaim(n.Name)
		}
		for _, n := range lo.Flatten([][]*corev1.Node{nodePool1Nodes, nodePool2Nodes, nodePool3Nodes}) {
			cluster.DeleteNode(n.Name)
		}

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("0"),
			corev1.ResourceMemory:           resource.MustParse("0Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("0Gi"),
		}, cluster.NodePoolResourcesFor(nodePool1NodeClaims[0].Labels[v1.NodePoolLabelKey]))
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("0"),
			corev1.ResourceMemory:           resource.MustParse("0Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("0Gi"),
		}, cluster.NodePoolResourcesFor(nodePool2NodeClaims[0].Labels[v1.NodePoolLabelKey]))
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("0"),
			corev1.ResourceMemory:           resource.MustParse("0Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("0Gi"),
		}, cluster.NodePoolResourcesFor(nodePool3NodeClaims[0].Labels[v1.NodePoolLabelKey]))
	})
	It("should update nodepool resources when a node switches from one nodepool to another", func() {
		oldNodePoolName := test.RandomName()
		nodeClaim1, node1 := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            oldNodePoolName,
					v1.NodeRegisteredLabelKey:      "true",
					corev1.LabelInstanceTypeStable: "m5.large",
				},
			},
			Status: v1.NodeClaimStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("2"),
					corev1.ResourceMemory:           resource.MustParse("2Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("1"),
					corev1.ResourceMemory:           resource.MustParse("1Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
				},
			},
		})
		cluster.UpdateNodeClaim(nodeClaim1.DeepCopy())
		Expect(cluster.UpdateNode(ctx, node1.DeepCopy())).To(Succeed())

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("2"),
			corev1.ResourceMemory:           resource.MustParse("2Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
		}, cluster.NodePoolResourcesFor(oldNodePoolName))

		newNodePoolName := test.RandomName()
		nodeClaim1.Labels[v1.NodePoolLabelKey] = newNodePoolName

		// Update the NodeClaim to change the NodePool that it's assigned to
		// Since the NodeClaim NodePool label
		cluster.UpdateNodeClaim(nodeClaim1.DeepCopy())

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("2"),
			corev1.ResourceMemory:           resource.MustParse("2Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
		}, cluster.NodePoolResourcesFor(oldNodePoolName))
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("0"),
			corev1.ResourceMemory:           resource.MustParse("0Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("0Gi"),
		}, cluster.NodePoolResourcesFor(newNodePoolName))

		node1.Labels[v1.NodePoolLabelKey] = newNodePoolName

		// Update the Node to change the NodePool that it's assigned to
		Expect(cluster.UpdateNode(ctx, node1.DeepCopy())).To(Succeed())

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("0"),
			corev1.ResourceMemory:           resource.MustParse("0Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("0Gi"),
		}, cluster.NodePoolResourcesFor(oldNodePoolName))
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("2"),
			corev1.ResourceMemory:           resource.MustParse("2Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
		}, cluster.NodePoolResourcesFor(newNodePoolName))
	})
	It("should update nodepool resources when the node changes providerID", func() {
		nodeClaim1, node1 := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            test.RandomName(),
					v1.NodeRegisteredLabelKey:      "true",
					corev1.LabelInstanceTypeStable: "m5.large",
				},
			},
			Status: v1.NodeClaimStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("2"),
					corev1.ResourceMemory:           resource.MustParse("2Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("1"),
					corev1.ResourceMemory:           resource.MustParse("1Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
				},
			},
		})
		cluster.UpdateNodeClaim(nodeClaim1.DeepCopy())
		Expect(cluster.UpdateNode(ctx, node1.DeepCopy())).To(Succeed())

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("2"),
			corev1.ResourceMemory:           resource.MustParse("2Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))

		// NodeClaim changes providerID for some reason
		nodeClaim1.Status.ProviderID = test.RandomProviderID()
		cluster.UpdateNodeClaim(nodeClaim1.DeepCopy())

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("4"),
			corev1.ResourceMemory:           resource.MustParse("4Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("4Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))

		// Node changes providerID and now matches
		node1.Spec.ProviderID = nodeClaim1.Status.ProviderID
		Expect(cluster.UpdateNode(ctx, node1.DeepCopy())).To(Succeed())

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("2"),
			corev1.ResourceMemory:           resource.MustParse("2Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))
	})
	It("should handle nodepool resources when node inside of the state node is removed", func() {
		nodeClaim1, node1 := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            test.RandomName(),
					v1.NodeRegisteredLabelKey:      "true",
					corev1.LabelInstanceTypeStable: "m5.large",
				},
			},
			Status: v1.NodeClaimStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("2"),
					corev1.ResourceMemory:           resource.MustParse("2Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
				},
			},
		})
		node1.Status.Capacity = corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("1"),
			corev1.ResourceMemory:           resource.MustParse("1Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
		}
		cluster.UpdateNodeClaim(nodeClaim1.DeepCopy())
		Expect(cluster.UpdateNode(ctx, node1.DeepCopy())).To(Succeed())

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("1"),
			corev1.ResourceMemory:           resource.MustParse("1Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))

		cluster.DeleteNode(node1.Name)

		// Should flip to use the NodeClaim capacity once we delete the Node
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("2"),
			corev1.ResourceMemory:           resource.MustParse("2Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))
	})
	It("should handle nodepool resources when node inside of the state node is removed", func() {
		nodeClaim1, node1 := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            test.RandomName(),
					v1.NodeRegisteredLabelKey:      "true",
					corev1.LabelInstanceTypeStable: "m5.large",
				},
			},
			Status: v1.NodeClaimStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("2"),
					corev1.ResourceMemory:           resource.MustParse("2Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
				},
			},
		})
		node1.Status.Capacity = corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("1"),
			corev1.ResourceMemory:           resource.MustParse("1Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
		}
		cluster.UpdateNodeClaim(nodeClaim1.DeepCopy())
		Expect(cluster.UpdateNode(ctx, node1.DeepCopy())).To(Succeed())

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("1"),
			corev1.ResourceMemory:           resource.MustParse("1Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))

		cluster.DeleteNodeClaim(nodeClaim1.Name)

		// Should continue to use the Node capacity once we delete the Node
		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("1"),
			corev1.ResourceMemory:           resource.MustParse("1Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))
	})
	It("should update nodepool resources when node is terminating (deletionTimestamp set)", func() {
		nodeClaim1, node1 := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            test.RandomName(),
					v1.NodeRegisteredLabelKey:      "true",
					corev1.LabelInstanceTypeStable: "m5.large",
				},
			},
			Status: v1.NodeClaimStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("2"),
					corev1.ResourceMemory:           resource.MustParse("2Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
				},
			},
		})
		cluster.UpdateNodeClaim(nodeClaim1.DeepCopy())
		Expect(cluster.UpdateNode(ctx, node1.DeepCopy())).To(Succeed())

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("2"),
			corev1.ResourceMemory:           resource.MustParse("2Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))

		// Set the deletionTimestamp
		nodeClaim1.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		cluster.UpdateNodeClaim(nodeClaim1.DeepCopy())

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("0"),
			corev1.ResourceMemory:           resource.MustParse("0Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("0Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))
	})
	It("should update nodepool resources when node is marked/unmarked for deletion", func() {
		nodeClaim1, node1 := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            test.RandomName(),
					v1.NodeRegisteredLabelKey:      "true",
					corev1.LabelInstanceTypeStable: "m5.large",
				},
			},
			Status: v1.NodeClaimStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("2"),
					corev1.ResourceMemory:           resource.MustParse("2Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
				},
			},
		})
		cluster.UpdateNodeClaim(nodeClaim1.DeepCopy())
		Expect(cluster.UpdateNode(ctx, node1.DeepCopy())).To(Succeed())

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("2"),
			corev1.ResourceMemory:           resource.MustParse("2Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))

		// MarkForDeletion and expect the count of resources to reduce
		cluster.MarkForDeletion(nodeClaim1.Status.ProviderID)

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("0"),
			corev1.ResourceMemory:           resource.MustParse("0Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("0Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))

		// UnmarkForDeletion and expect the count of resources to be restored
		cluster.UnmarkForDeletion(nodeClaim1.Status.ProviderID)

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("2"),
			corev1.ResourceMemory:           resource.MustParse("2Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))
	})
	It("should not double subtract resources when marking for deletion and then deleting", func() {
		nodeClaim1, node1 := test.NodeClaimAndNode(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1.NodePoolLabelKey:            test.RandomName(),
					v1.NodeRegisteredLabelKey:      "true",
					corev1.LabelInstanceTypeStable: "m5.large",
				},
			},
			Status: v1.NodeClaimStatus{
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("2"),
					corev1.ResourceMemory:           resource.MustParse("2Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
				},
			},
		})
		cluster.UpdateNodeClaim(nodeClaim1.DeepCopy())
		Expect(cluster.UpdateNode(ctx, node1.DeepCopy())).To(Succeed())

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("2"),
			corev1.ResourceMemory:           resource.MustParse("2Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))

		// MarkForDeletion and expect the count of resources to reduce
		cluster.MarkForDeletion(nodeClaim1.Status.ProviderID)

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("0"),
			corev1.ResourceMemory:           resource.MustParse("0Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("0Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))

		nodeClaim1.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		cluster.UpdateNodeClaim(nodeClaim1.DeepCopy())

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("0"),
			corev1.ResourceMemory:           resource.MustParse("0Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("0Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))

		node1.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		Expect(cluster.UpdateNode(ctx, node1.DeepCopy())).To(Succeed())

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("0"),
			corev1.ResourceMemory:           resource.MustParse("0Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("0Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))

		cluster.DeleteNodeClaim(nodeClaim1.Name)

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("0"),
			corev1.ResourceMemory:           resource.MustParse("0Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("0Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))

		cluster.DeleteNode(node1.Name)

		ExpectResources(corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("0"),
			corev1.ResourceMemory:           resource.MustParse("0Gi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("0Gi"),
		}, cluster.NodePoolResourcesFor(nodeClaim1.Labels[v1.NodePoolLabelKey]))
	})
})

var _ = Describe("NodePoolState Tracking", func() {
	var nodeClaim *v1.NodeClaim
	var nodeClaim2 *v1.NodeClaim
	var nodePool2 *v1.NodePool

	BeforeEach(func() {
		nodePool2 = test.NodePool(v1.NodePool{ObjectMeta: metav1.ObjectMeta{Name: "nodepool2"}})
		ExpectApplied(ctx, env.Client, nodePool2)

		nodeClaim = test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-nodeclaim",
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool.Name,
				},
			},
			Status: v1.NodeClaimStatus{
				ProviderID: test.RandomProviderID(),
			},
		})

		nodeClaim2 = test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-nodeclaim-2",
				Labels: map[string]string{
					v1.NodePoolLabelKey: nodePool2.Name,
				},
			},
			Status: v1.NodeClaimStatus{
				ProviderID: test.RandomProviderID(),
			},
		})
	})

	Context("UpdateNodeClaim", func() {
		Context("New NodeClaim gets added", func() {
			It("should track NodeClaim in running state when created with ProviderID", func() {
				ExpectApplied(ctx, env.Client, nodeClaim)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))

				running, deleting, pendingdisruption := cluster.NodePoolState.GetNodeCount(nodePool.Name)
				Expect(running).To(Equal(1))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))

				Expect(cluster.NodeClaimExists(nodeClaim.Name)).To(BeTrue())
			})

			It("should track NodeClaim without ProviderID", func() {
				nodeClaimWithoutProvider := test.NodeClaim(v1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-nodeclaim-no-provider",
						Labels: map[string]string{
							v1.NodePoolLabelKey: nodePool.Name,
						},
					},
					// No ProviderID set
				})

				ExpectApplied(ctx, env.Client, nodeClaimWithoutProvider)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaimWithoutProvider))

				running, deleting, pendingdisruption := cluster.NodePoolState.GetNodeCount(nodePool.Name)
				Expect(running).To(Equal(1))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))

				Expect(cluster.NodeClaimExists(nodeClaimWithoutProvider.Name)).To(BeTrue())
			})

			It("should not track NodeClaim that has no nodepool", func() {
				nodeClaimWithoutNodePool := test.NodeClaim(v1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-nodeclaim-no-provider",
					},
					Status: v1.NodeClaimStatus{
						ProviderID: test.RandomProviderID(),
					}})

				ExpectApplied(ctx, env.Client, nodeClaimWithoutNodePool)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaimWithoutNodePool))

				running, deleting, pendingdisruption := cluster.NodePoolState.GetNodeCount("")
				Expect(running).To(Equal(0))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))

				Expect(cluster.NodeClaimExists(nodeClaimWithoutNodePool.Name)).To(BeTrue())
			})

			It("should track multiple NodeClaims in the same NodePool", func() {
				nodeClaim3 := test.NodeClaim(v1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-nodeclaim-2",
						Labels: map[string]string{
							v1.NodePoolLabelKey: nodePool.Name,
						},
					},
					Status: v1.NodeClaimStatus{
						ProviderID: test.RandomProviderID(),
					},
				})

				ExpectApplied(ctx, env.Client, nodeClaim, nodeClaim3)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim3))

				running, deleting, pendingdisruption := cluster.NodePoolState.GetNodeCount(nodePool.Name)
				Expect(running).To(Equal(2))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))
			})

			It("should track NodeClaims across different NodePools", func() {
				ExpectApplied(ctx, env.Client, nodeClaim, nodeClaim2)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim2))

				running1, deleting1, pendingdisruption1 := cluster.NodePoolState.GetNodeCount(nodePool.Name)
				running2, deleting2, pendingdisruption2 := cluster.NodePoolState.GetNodeCount(nodePool2.Name)

				Expect(running1).To(Equal(1))
				Expect(deleting1).To(Equal(0))
				Expect(pendingdisruption1).To(Equal(0))

				Expect(running2).To(Equal(1))
				Expect(deleting2).To(Equal(0))
				Expect(pendingdisruption2).To(Equal(0))

			})
		})

		Context("Updates to existing NodeClaim", func() {
			BeforeEach(func() {
				ExpectApplied(ctx, env.Client, nodeClaim)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
			})

			It("should be a no-op when NodeClaim is already tracked and no state change", func() {
				// Verify initial state
				running, deleting, pendingdisruption := cluster.NodePoolState.GetNodeCount(nodePool.Name)
				Expect(running).To(Equal(1))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))

				// Update NodeClaim with annotation change (no state change)
				nodeClaim.Annotations = map[string]string{"test": "annotation"}
				ExpectApplied(ctx, env.Client, nodeClaim)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))

				// State should remain the same
				running, deleting, pendingdisruption = cluster.NodePoolState.GetNodeCount(nodePool.Name)
				Expect(running).To(Equal(1))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))
			})

			It("should handle NodeClaim ProviderID change", func() {
				// Verify initial state
				running, deleting, pendingdisruption := cluster.NodePoolState.GetNodeCount(nodePool.Name)
				Expect(running).To(Equal(1))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))

				// Change ProviderID
				originalProviderID := nodeClaim.Status.ProviderID
				newProviderID := test.RandomProviderID()
				nodeClaim.Status.ProviderID = newProviderID
				ExpectApplied(ctx, env.Client, nodeClaim)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))

				// Should still track the NodeClaim correctly
				running, deleting, pendingdisruption = cluster.NodePoolState.GetNodeCount(nodePool.Name)
				Expect(running).To(Equal(1))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))

				// Old ProviderID should not be tracked for deletion
				cluster.MarkForDeletion(originalProviderID)
				running, deleting, pendingdisruption = cluster.NodePoolState.GetNodeCount(nodePool.Name)
				Expect(running).To(Equal(1)) // Should not change
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))

				// New ProviderID should work for deletion
				cluster.MarkForDeletion(newProviderID)
				running, deleting, pendingdisruption = cluster.NodePoolState.GetNodeCount(nodePool.Name)
				Expect(running).To(Equal(0))
				Expect(deleting).To(Equal(1))
				Expect(pendingdisruption).To(Equal(0))
			})

			It("should move NodeClaim to deleting state when Node is marked for deletion", func() {
				// Verify initial running state
				running, deleting, pendingdisruption := cluster.NodePoolState.GetNodeCount(nodePool.Name)
				Expect(running).To(Equal(1))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))

				// Mark the node for deletion via cluster state
				cluster.MarkForDeletion(nodeClaim.Status.ProviderID)

				// Update NodeClaim - should detect it's marked for deletion
				nodeClaim.Annotations = map[string]string{"updated": "true"}
				ExpectApplied(ctx, env.Client, nodeClaim)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))

				// Should be in deleting state
				running, deleting, pendingdisruption = cluster.NodePoolState.GetNodeCount(nodePool.Name)
				Expect(running).To(Equal(0))
				Expect(deleting).To(Equal(1))
				Expect(pendingdisruption).To(Equal(0))
			})

			It("should handle NodeClaim cleanup correctly", func() {
				ExpectApplied(ctx, env.Client, nodeClaim)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))

				running, deleting, pendingdisruption := cluster.NodePoolState.GetNodeCount(nodePool.Name)
				Expect(running).To(Equal(1))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))

				// Delete the NodeClaim
				ExpectDeleted(ctx, env.Client, nodeClaim)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))

				running, deleting, pendingdisruption = cluster.NodePoolState.GetNodeCount(nodePool.Name)
				Expect(running).To(Equal(0))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))
			})
		})

		Context("Mark NodeClaims pendingdisruption", func() {
			BeforeEach(func() {
				ExpectApplied(ctx, env.Client, nodeClaim)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
			})

			It("should handle marking multiple NodeClaims", func() {
				nodeClaim2 := test.NodeClaim(v1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-nodeclaim-2",
						Labels: map[string]string{
							v1.NodePoolLabelKey: nodePool.Name,
						},
					},
					Status: v1.NodeClaimStatus{
						ProviderID: test.RandomProviderID(),
					},
				})

				ExpectApplied(ctx, env.Client, nodeClaim2)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim2))

				running, deleting, pendingdisruption := cluster.NodePoolState.GetNodeCount(nodePool.Name)

				Expect(running).To(Equal(2))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))

				cluster.NodePoolState.MarkNodeClaimPendingDisruption(nodePool.Name, nodeClaim.Name)
				running, deleting, pendingdisruption = cluster.NodePoolState.GetNodeCount(nodePool.Name)

				Expect(running).To(Equal(1))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(1))

				cluster.NodePoolState.MarkNodeClaimPendingDisruption(nodePool.Name, nodeClaim2.Name)
				running, deleting, pendingdisruption = cluster.NodePoolState.GetNodeCount(nodePool.Name)

				Expect(running).To(Equal(0))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(2))
			})
		})

		Context("DeleteNodeClaim", func() {
			BeforeEach(func() {
				ExpectApplied(ctx, env.Client, nodeClaim)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
			})

			It("should remove NodeClaim from nodepool state", func() {
				running, deleting, pendingdisruption := cluster.NodePoolState.GetNodeCount(nodePool.Name)
				Expect(running).To(Equal(1))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))
				Expect(cluster.NodeClaimExists(nodeClaim.Name)).To(BeTrue())

				ExpectDeleted(ctx, env.Client, nodeClaim)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))

				running, deleting, pendingdisruption = cluster.NodePoolState.GetNodeCount(nodePool.Name)
				Expect(running).To(Equal(0))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))
				Expect(cluster.NodeClaimExists(nodeClaim.Name)).To(BeFalse())
			})

		})

		Context("MarkForDeletion", func() {
			BeforeEach(func() {
				ExpectApplied(ctx, env.Client, nodeClaim)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
			})

			It("should handle marking multiple NodeClaims for deletion", func() {
				nodeClaim2 := test.NodeClaim(v1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-nodeclaim-2",
						Labels: map[string]string{
							v1.NodePoolLabelKey: nodePool.Name,
						},
					},
					Status: v1.NodeClaimStatus{
						ProviderID: test.RandomProviderID(),
					},
				})

				ExpectApplied(ctx, env.Client, nodeClaim2)
				ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim2))

				running, deleting, pendingdisruption := cluster.NodePoolState.GetNodeCount(nodePool.Name)

				Expect(running).To(Equal(2))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))

				cluster.MarkForDeletion(nodeClaim.Status.ProviderID, nodeClaim2.Status.ProviderID)
				running, deleting, pendingdisruption = cluster.NodePoolState.GetNodeCount(nodePool.Name)

				Expect(running).To(Equal(0))
				Expect(deleting).To(Equal(2))
				Expect(pendingdisruption).To(Equal(0))

			})

			It("should move NodeClaim from running to deleting state", func() {
				running, deleting, pendingdisruption := cluster.NodePoolState.GetNodeCount(nodePool.Name)
				Expect(running).To(Equal(1))
				Expect(deleting).To(Equal(0))
				Expect(pendingdisruption).To(Equal(0))

				cluster.MarkForDeletion(nodeClaim.Status.ProviderID)
				running, deleting, pendingdisruption = cluster.NodePoolState.GetNodeCount(nodePool.Name)

				Expect(running).To(Equal(0))
				Expect(deleting).To(Equal(1))
				Expect(pendingdisruption).To(Equal(0))
			})
		})
	})

	Context("UnmarkForDeletion", func() {
		BeforeEach(func() {
			ExpectApplied(ctx, env.Client, nodeClaim)
			ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))
			cluster.MarkForDeletion(nodeClaim.Status.ProviderID)
		})

		It("should move NodeClaim from deleting to running state", func() {
			running, deleting, pendingdisruption := cluster.NodePoolState.GetNodeCount(nodePool.Name)
			Expect(running).To(Equal(0))
			Expect(deleting).To(Equal(1))
			Expect(pendingdisruption).To(Equal(0))

			cluster.UnmarkForDeletion(nodeClaim.Status.ProviderID)
			running, deleting, pendingdisruption = cluster.NodePoolState.GetNodeCount(nodePool.Name)

			Expect(running).To(Equal(1))
			Expect(deleting).To(Equal(0))
			Expect(pendingdisruption).To(Equal(0))

		})

		It("should handle unmarking multiple NodeClaims", func() {
			nodeClaim2 := test.NodeClaim(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-nodeclaim-2",
					Labels: map[string]string{
						v1.NodePoolLabelKey: nodePool.Name,
					},
				},
				Status: v1.NodeClaimStatus{
					ProviderID: test.RandomProviderID(),
				},
			})

			ExpectApplied(ctx, env.Client, nodeClaim2)
			ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim2))

			cluster.MarkForDeletion(nodeClaim2.Status.ProviderID)
			running, deleting, pendingdisruption := cluster.NodePoolState.GetNodeCount(nodePool.Name)

			Expect(running).To(Equal(0))
			Expect(deleting).To(Equal(2))
			Expect(pendingdisruption).To(Equal(0))

			cluster.UnmarkForDeletion(nodeClaim.Status.ProviderID, nodeClaim2.Status.ProviderID)
			running, deleting, pendingdisruption = cluster.NodePoolState.GetNodeCount(nodePool.Name)

			Expect(running).To(Equal(2))
			Expect(deleting).To(Equal(0))
			Expect(pendingdisruption).To(Equal(0))
		})
	})

	Context("Transitions", func() {
		It("should handle concurrent NodeClaim updates and state changes", func() {
			nodeClaim := test.NodeClaim(v1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "race-test-nodeclaim",
					Labels: map[string]string{
						v1.NodePoolLabelKey: nodePool.Name,
					},
				},
				Status: v1.NodeClaimStatus{
					ProviderID: test.RandomProviderID(),
				},
			})

			ExpectApplied(ctx, env.Client, nodeClaim)
			ExpectReconcileSucceeded(ctx, nodeClaimController, client.ObjectKeyFromObject(nodeClaim))

			var wg sync.WaitGroup
			numOperations := 50

			// Concurrent mark/unmark operations
			for i := 0; i < numOperations; i++ {
				wg.Add(1)
				go func(iteration int) {
					defer wg.Done()

					switch iteration % 3 {
					case 0:
						cluster.MarkForDeletion(nodeClaim.Status.ProviderID)
					case 1:
						cluster.NodePoolState.MarkNodeClaimPendingDisruption(nodePool.Name, nodeClaim.Name)
					case 2:
						cluster.UnmarkForDeletion(nodeClaim.Status.ProviderID)
					}
				}(i)
			}

			wg.Wait()

			// Final state should be consistent
			running, deleting, pendingdisruption := cluster.NodePoolState.GetNodeCount(nodePool.Name)
			Expect(running + deleting + pendingdisruption).To(Equal(1)) // Should have exactly one NodeClaim
		})
	})
})

func ExpectStateNodeCount(comparator string, count int) int {
	GinkgoHelper()
	c := 0
	for range cluster.Nodes() {
		c++
	}
	Expect(c).To(BeNumerically(comparator, count))
	return c
}
