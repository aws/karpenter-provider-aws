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

package state_test

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/utils/resources"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"

	"github.com/aws/karpenter/pkg/test"

	. "github.com/aws/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
)

var ctx context.Context
var env *test.Environment
var cluster *state.Cluster
var nodeController *state.NodeController
var podController *state.PodController

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers/State")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	cluster = state.NewCluster(ctx, env.Client)
	nodeController = state.NewNodeController(env.Client, cluster)
	podController = state.NewPodController(env.Client, cluster)
})
var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Node Resource Level", func() {
	It("should not count pods not bound to nodes", func() {
		pod1 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("1.5"),
				}},
		})
		pod2 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("2"),
				}},
		})
		node := test.Node(test.NodeOptions{Allocatable: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU: resource.MustParse("4"),
		}})
		ExpectApplied(ctx, env.Client, pod1, pod2)
		ExpectApplied(ctx, env.Client, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))

		// two pods, but neither is bound to the node so the node's CPU requests should be zero
		ExpectNodeResourceRequest(node, v1.ResourceCPU, "0.0")
	})
	It("should count new pods bound to nodes", func() {
		pod1 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("1.5"),
				}},
		})
		pod2 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("2"),
				}},
		})
		node := test.Node(test.NodeOptions{Allocatable: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU: resource.MustParse("4"),
		}})
		ExpectApplied(ctx, env.Client, pod1, pod2)
		ExpectApplied(ctx, env.Client, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))

		ExpectManualBinding(ctx, env.Client, pod1, node)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))

		ExpectNodeResourceRequest(node, v1.ResourceCPU, "1.5")

		ExpectManualBinding(ctx, env.Client, pod2, node)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))
		ExpectNodeResourceRequest(node, v1.ResourceCPU, "3.5")
	})
	It("should count existing pods bound to nodes", func() {
		pod1 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("1.5"),
				}},
		})
		pod2 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("2"),
				}},
		})
		node := test.Node(test.NodeOptions{Allocatable: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU: resource.MustParse("4"),
		}})

		// simulate a node that already exists in our cluster
		ExpectApplied(ctx, env.Client, pod1, pod2)
		ExpectApplied(ctx, env.Client, node)
		ExpectManualBinding(ctx, env.Client, pod1, node)
		ExpectManualBinding(ctx, env.Client, pod2, node)

		// that we just noticed
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectNodeResourceRequest(node, v1.ResourceCPU, "3.5")
	})
	It("should subtract requests if the pod is deleted", func() {
		pod1 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("1.5"),
				}},
		})
		pod2 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("2"),
				}},
		})
		node := test.Node(test.NodeOptions{Allocatable: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU: resource.MustParse("4"),
		}})
		ExpectApplied(ctx, env.Client, pod1, pod2)
		ExpectApplied(ctx, env.Client, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))

		ExpectManualBinding(ctx, env.Client, pod1, node)
		ExpectManualBinding(ctx, env.Client, pod2, node)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))

		ExpectNodeResourceRequest(node, v1.ResourceCPU, "3.5")

		// delete the pods and the CPU usage should go down
		ExpectDeleted(ctx, env.Client, pod2)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))
		ExpectNodeResourceRequest(node, v1.ResourceCPU, "1.5")

		ExpectDeleted(ctx, env.Client, pod1)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))
		ExpectNodeResourceRequest(node, v1.ResourceCPU, "0")
	})
	It("should stop tracking nodes that are deleted", func() {
		pod1 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("1.5"),
				}},
		})
		node := test.Node(test.NodeOptions{Allocatable: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU: resource.MustParse("4"),
		}})
		ExpectApplied(ctx, env.Client, pod1)
		ExpectApplied(ctx, env.Client, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))

		ExpectManualBinding(ctx, env.Client, pod1, node)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))

		cluster.ForEachNode(func(n *state.Node) bool {
			available := n.Available
			requested := resources.Subtract(n.Node.Status.Allocatable, available)
			Expect(available.Cpu().AsApproximateFloat64()).To(BeNumerically("~", 2.5))
			Expect(requested.Cpu().AsApproximateFloat64()).To(BeNumerically("~", 1.5))
			return true
		})

		// delete the node and the internal state should disappear as well
		ExpectDeleted(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		cluster.ForEachNode(func(n *state.Node) bool {
			Fail("shouldn't be called as the node was deleted")
			return true
		})
	})
	It("should track pods correctly if we miss events or they are consolidated", func() {
		pod1 := test.UnschedulablePod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{Name: "stateful-set-pod"},
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("1.5"),
				}},
		})

		node1 := test.Node(test.NodeOptions{Allocatable: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU: resource.MustParse("4"),
		}})
		ExpectApplied(ctx, env.Client, pod1, node1)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node1))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))

		ExpectManualBinding(ctx, env.Client, pod1, node1)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))

		cluster.ForEachNode(func(n *state.Node) bool {
			available := n.Available
			requested := resources.Subtract(n.Node.Status.Allocatable, available)
			Expect(available.Cpu().AsApproximateFloat64()).To(BeNumerically("~", 2.5))
			Expect(requested.Cpu().AsApproximateFloat64()).To(BeNumerically("~", 1.5))
			return true
		})

		ExpectDeleted(ctx, env.Client, pod1)

		// second node has more capacity
		node2 := test.Node(test.NodeOptions{Allocatable: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU: resource.MustParse("8"),
		}})

		// and the pod can only bind to node2 due to the resource request
		pod2 := test.UnschedulablePod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{Name: "stateful-set-pod"},
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("5.0"),
				}},
		})

		ExpectApplied(ctx, env.Client, pod2, node2)
		ExpectManualBinding(ctx, env.Client, pod2, node2)
		// deleted the pod and then recreated it, but simulated only receiving an event on the new pod after it has
		// bound and not getting the new node event entirely
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod2))

		cluster.ForEachNode(func(n *state.Node) bool {
			available := n.Available
			requested := resources.Subtract(n.Node.Status.Allocatable, available)
			if n.Node.Name == node1.Name {
				// not on node1 any longer, so it should be fully free
				Expect(available.Cpu().AsApproximateFloat64()).To(BeNumerically("~", 4))
				Expect(requested.Cpu().AsApproximateFloat64()).To(BeNumerically("~", 0))
			} else {
				Expect(available.Cpu().AsApproximateFloat64()).To(BeNumerically("~", 3.0))
				Expect(requested.Cpu().AsApproximateFloat64()).To(BeNumerically("~", 5.0))
			}
			return true
		})

	})
	It("should maintain a correct count of resource usage as pods are deleted/added", func() {
		var pods []*v1.Pod
		for i := 0; i < 100; i++ {
			pods = append(pods, test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceCPU: resource.MustParse(fmt.Sprintf("%1.1f", rand.Float64()*2)),
					}},
			}))
		}
		node := test.Node(test.NodeOptions{Allocatable: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU:  resource.MustParse("200"),
			v1.ResourcePods: resource.MustParse("500"),
		}})
		ExpectApplied(ctx, env.Client, node)
		ExpectNodeResourceRequest(node, v1.ResourceCPU, "0.0")
		ExpectNodeResourceRequest(node, v1.ResourcePods, "0")

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
			ExpectNodeResourceRequest(node, v1.ResourceCPU, fmt.Sprintf("%1.1f", sum))
			ExpectNodeResourceRequest(node, v1.ResourcePods, fmt.Sprintf("%d", podCount))
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
			ExpectNodeResourceRequest(node, v1.ResourceCPU, fmt.Sprintf("%1.1f", sum))
			ExpectNodeResourceRequest(node, v1.ResourcePods, fmt.Sprintf("%d", podCount))
		}
		ExpectNodeResourceRequest(node, v1.ResourceCPU, "0.0")
		ExpectNodeResourceRequest(node, v1.ResourcePods, "0")
	})
	It("should track daemonset requested resources separately", func() {
		ds := test.DaemonSet(
			test.DaemonSetOptions{PodOptions: test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("1"),
					v1.ResourceMemory: resource.MustParse("2Gi")}},
			}},
		)
		ExpectApplied(ctx, env.Client, ds)
		Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(ds), ds)).To(Succeed())

		pod1 := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("1.5"),
				}},
		})

		dsPod := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU:    resource.MustParse("1"),
					v1.ResourceMemory: resource.MustParse("2Gi"),
				}},
		})
		dsPod.OwnerReferences = append(dsPod.OwnerReferences, metav1.OwnerReference{
			APIVersion:         "apps/v1",
			Kind:               "DaemonSet",
			Name:               ds.Name,
			UID:                ds.UID,
			Controller:         aws.Bool(true),
			BlockOwnerDeletion: aws.Bool(true),
		})

		node := test.Node(test.NodeOptions{Allocatable: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU:    resource.MustParse("4"),
			v1.ResourceMemory: resource.MustParse("8Gi"),
		}})
		ExpectApplied(ctx, env.Client, pod1, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

		ExpectManualBinding(ctx, env.Client, pod1, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod1))

		// daemonset pod isn't bound yet
		ExpectNodeDaemonsetRequested(node, v1.ResourceCPU, "0")
		ExpectNodeDaemonsetRequested(node, v1.ResourceMemory, "0")
		ExpectNodeResourceRequest(node, v1.ResourceCPU, "1.5")

		ExpectApplied(ctx, env.Client, dsPod)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(dsPod))
		ExpectManualBinding(ctx, env.Client, dsPod, node)
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(dsPod))

		// just the DS request portion
		ExpectNodeDaemonsetRequested(node, v1.ResourceCPU, "1")
		ExpectNodeDaemonsetRequested(node, v1.ResourceMemory, "2Gi")
		// total request
		ExpectNodeResourceRequest(node, v1.ResourceCPU, "2.5")
		ExpectNodeResourceRequest(node, v1.ResourceMemory, "2Gi")
	})
})

var _ = Describe("Pod Anti-Affinity", func() {
	It("should track pods with required anti-affinity", func() {
		pod := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("1.5"),
				}},
			PodAntiRequirements: []v1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					TopologyKey: v1.LabelTopologyZone,
				},
			},
		})

		node := test.Node(test.NodeOptions{Allocatable: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU: resource.MustParse("4"),
		}})

		ExpectApplied(ctx, env.Client, pod)
		ExpectApplied(ctx, env.Client, node)
		ExpectManualBinding(ctx, env.Client, pod, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod))
		foundPodCount := 0
		cluster.ForPodsWithAntiAffinity(func(p *v1.Pod, n *v1.Node) bool {
			foundPodCount++
			Expect(p.Name).To(Equal(pod.Name))
			return true
		})
		Expect(foundPodCount).To(BeNumerically("==", 1))
	})
	It("should not track pods with preferred anti-affinity", func() {
		pod := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("1.5"),
				}},
			PodAntiPreferences: []v1.WeightedPodAffinityTerm{
				{
					Weight: 15,
					PodAffinityTerm: v1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"foo": "bar"},
						},
						TopologyKey: v1.LabelTopologyZone,
					},
				},
			},
		})

		node := test.Node(test.NodeOptions{Allocatable: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU: resource.MustParse("4"),
		}})

		ExpectApplied(ctx, env.Client, pod)
		ExpectApplied(ctx, env.Client, node)
		ExpectManualBinding(ctx, env.Client, pod, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod))
		foundPodCount := 0
		cluster.ForPodsWithAntiAffinity(func(p *v1.Pod, n *v1.Node) bool {
			foundPodCount++
			Fail("shouldn't track pods with preferred anti-affinity")
			return true
		})
		Expect(foundPodCount).To(BeNumerically("==", 0))
	})
	It("should stop tracking pods with required anti-affinity if the pod is deleted", func() {
		pod := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("1.5"),
				}},
			PodAntiRequirements: []v1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					TopologyKey: v1.LabelTopologyZone,
				},
			},
		})

		node := test.Node(test.NodeOptions{Allocatable: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU: resource.MustParse("4"),
		}})

		ExpectApplied(ctx, env.Client, pod)
		ExpectApplied(ctx, env.Client, node)
		ExpectManualBinding(ctx, env.Client, pod, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod))
		foundPodCount := 0
		cluster.ForPodsWithAntiAffinity(func(p *v1.Pod, n *v1.Node) bool {
			foundPodCount++
			Expect(p.Name).To(Equal(pod.Name))
			return true
		})
		Expect(foundPodCount).To(BeNumerically("==", 1))

		ExpectDeleted(ctx, env.Client, client.Object(pod))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod))
		foundPodCount = 0
		cluster.ForPodsWithAntiAffinity(func(p *v1.Pod, n *v1.Node) bool {
			foundPodCount++
			Fail("should not be called as the pod was deleted")
			return true
		})
		Expect(foundPodCount).To(BeNumerically("==", 0))
	})
	It("should handle events out of order", func() {
		pod := test.UnschedulablePod(test.PodOptions{
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU: resource.MustParse("1.5"),
				}},
			PodAntiRequirements: []v1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					TopologyKey: v1.LabelTopologyZone,
				},
			},
		})

		node := test.Node(test.NodeOptions{Allocatable: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU: resource.MustParse("4"),
		}})

		ExpectApplied(ctx, env.Client, pod)
		ExpectApplied(ctx, env.Client, node)
		ExpectManualBinding(ctx, env.Client, pod, node)

		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))
		ExpectReconcileSucceeded(ctx, podController, client.ObjectKeyFromObject(pod))

		// simulate receiving the node deletion before the pod deletion
		ExpectDeleted(ctx, env.Client, node)
		ExpectReconcileSucceeded(ctx, nodeController, client.ObjectKeyFromObject(node))

		foundPodCount := 0
		cluster.ForPodsWithAntiAffinity(func(p *v1.Pod, n *v1.Node) bool {
			foundPodCount++
			return true
		})
		Expect(foundPodCount).To(BeNumerically("==", 0))
	})

})

func ExpectNodeResourceRequest(node *v1.Node, resourceName v1.ResourceName, amount string) {
	cluster.ForEachNode(func(n *state.Node) bool {
		if n.Node.Name != node.Name {
			return true
		}
		requested := resources.Subtract(n.Node.Status.Allocatable, n.Available)

		nodeRequest := requested[resourceName]
		expected := resource.MustParse(amount)
		Expect(nodeRequest.AsApproximateFloat64()).To(BeNumerically("~", expected.AsApproximateFloat64(), 0.001))
		return false
	})
}
func ExpectNodeDaemonsetRequested(node *v1.Node, resourceName v1.ResourceName, amount string) {
	cluster.ForEachNode(func(n *state.Node) bool {
		if n.Node.Name != node.Name {
			return true
		}
		dsReq := n.DaemonSetRequested[resourceName]
		expected := resource.MustParse(amount)
		Expect(dsReq.AsApproximateFloat64()).To(BeNumerically("~", expected.AsApproximateFloat64(), 0.001))
		return false
	})
}
