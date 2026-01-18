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

package integration_test

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"
	"sigs.k8s.io/karpenter/test/pkg/debug"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
)

var _ = Describe("Integration", func() {
	Describe("DaemonSet", func() {
		var limitrange *corev1.LimitRange
		var priorityclass *schedulingv1.PriorityClass
		var daemonset *appsv1.DaemonSet
		var dep *appsv1.Deployment

		BeforeEach(func() {
			nodePool.Spec.Disruption.ConsolidationPolicy = v1.ConsolidationPolicyWhenEmptyOrUnderutilized
			nodePool.Spec.Disruption.ConsolidateAfter = v1.MustParseNillableDuration("0s")
			priorityclass = &schedulingv1.PriorityClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "high-priority-daemonsets",
				},
				Value:         int32(10000000),
				GlobalDefault: false,
				Description:   "This priority class should be used for daemonsets.",
			}
			limitrange = &corev1.LimitRange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "limitrange",
					Namespace: "default",
				},
			}
			daemonset = test.DaemonSet(test.DaemonSetOptions{
				PodOptions: test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")}},
					PriorityClassName:    "high-priority-daemonsets",
				},
			})
			numPods := 1
			dep = test.Deployment(test.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "large-app"},
					},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("4")},
					},
				},
			})
		})
		It("should account for LimitRange Default on daemonSet pods for resources", func() {
			limitrange.Spec.Limits = []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					Default: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
			}

			podSelector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
			daemonSetSelector := labels.SelectorFromSet(daemonset.Spec.Selector.MatchLabels)
			env.ExpectCreated(nodeClass, nodePool, limitrange, priorityclass, daemonset, dep)

			// Eventually expect a single node to exist and both the deployment pod and the daemonset pod to schedule to it
			Eventually(func(g Gomega) {
				nodeList := &corev1.NodeList{}
				g.Expect(env.Client.List(env, nodeList, client.HasLabels{"testing/cluster"})).To(Succeed())
				g.Expect(nodeList.Items).To(HaveLen(1))

				deploymentPods := env.Monitor.RunningPods(podSelector)
				g.Expect(deploymentPods).To(HaveLen(1))

				daemonSetPods := env.Monitor.RunningPods(daemonSetSelector)
				g.Expect(daemonSetPods).To(HaveLen(1))

				g.Expect(deploymentPods[0].Spec.NodeName).To(Equal(nodeList.Items[0].Name))
				g.Expect(daemonSetPods[0].Spec.NodeName).To(Equal(nodeList.Items[0].Name))
			}).Should(Succeed())
		})
		It("should account for LimitRange DefaultRequest on daemonSet pods for resources", func() {
			limitrange.Spec.Limits = []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					DefaultRequest: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
			}

			podSelector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
			daemonSetSelector := labels.SelectorFromSet(daemonset.Spec.Selector.MatchLabels)
			env.ExpectCreated(nodeClass, nodePool, limitrange, priorityclass, daemonset, dep)

			// Eventually expect a single node to exist and both the deployment pod and the daemonset pod to schedule to it
			Eventually(func(g Gomega) {
				nodeList := &corev1.NodeList{}
				g.Expect(env.Client.List(env, nodeList, client.HasLabels{"testing/cluster"})).To(Succeed())
				g.Expect(nodeList.Items).To(HaveLen(1))

				deploymentPods := env.Monitor.RunningPods(podSelector)
				g.Expect(deploymentPods).To(HaveLen(1))

				daemonSetPods := env.Monitor.RunningPods(daemonSetSelector)
				g.Expect(daemonSetPods).To(HaveLen(1))

				g.Expect(deploymentPods[0].Spec.NodeName).To(Equal(nodeList.Items[0].Name))
				g.Expect(daemonSetPods[0].Spec.NodeName).To(Equal(nodeList.Items[0].Name))
			}).Should(Succeed())
		})
	})
	Describe("CRD Hash", func() {
		It("should have NodePool hash", func() {
			env.ExpectCreated(nodeClass, nodePool)

			Eventually(func(g Gomega) {
				np := &v1.NodePool{}
				err := env.Client.Get(env, client.ObjectKeyFromObject(nodePool), np)
				g.Expect(err).ToNot(HaveOccurred())

				hash, found := np.Annotations[v1.NodePoolHashAnnotationKey]
				g.Expect(found).To(BeTrue())
				g.Expect(hash).To(Equal(np.Hash()))
			})
		})
	})
	Describe("Utilization", Label(debug.NoWatch), Label(debug.NoEvents), func() {
		It("should provision one pod per node", func() {
			label := map[string]string{"app": "large-app"}
			deployment := test.Deployment(test.DeploymentOptions{
				Replicas: 100,
				PodOptions: test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							v1.DoNotDisruptAnnotationKey: "true",
						},
						Labels: label,
					},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: func() resource.Quantity {
								dsOverhead := env.GetDaemonSetOverhead(nodePool)
								base := lo.ToPtr(resource.MustParse("1800m"))
								base.Sub(*dsOverhead.Cpu())
								return *base
							}(),
						},
					},
					PodAntiRequirements: []corev1.PodAffinityTerm{{
						TopologyKey: corev1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: label,
						},
					}},
				},
			})

			env.ExpectCreated(nodeClass, nodePool, deployment)
			env.EventuallyExpectHealthyPodCountWithTimeout(time.Minute*10, labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels), int(*deployment.Spec.Replicas))
			env.ExpectCreatedNodeCount("==", int(*deployment.Spec.Replicas)) // One pod per node enforced by instance size
		})
	})
	Describe("Validation", func() {
		Context("NodePool", func() {
			It("should error when a restricted label is used in labels (karpenter.sh/nodepool)", func() {
				nodePool.Spec.Template.Labels = map[string]string{
					v1.NodePoolLabelKey: "my-custom-nodepool",
				}
				Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
			})
			It("should error when a restricted label is used in labels (kubernetes.io/custom-label)", func() {
				nodePool.Spec.Template.Labels = map[string]string{
					"kubernetes.io/custom-label": "custom-value",
				}
				Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
			})
			It("should allow a restricted label exception to be used in labels (node-restriction.kubernetes.io/custom-label)", func() {
				nodePool.Spec.Template.Labels = map[string]string{
					corev1.LabelNamespaceNodeRestriction + "/custom-label": "custom-value",
				}
				Expect(env.Client.Create(env.Context, nodePool)).To(Succeed())
			})
			It("should allow a restricted label exception to be used in labels ([*].node-restriction.kubernetes.io/custom-label)", func() {
				nodePool.Spec.Template.Labels = map[string]string{
					"subdomain" + corev1.LabelNamespaceNodeRestriction + "/custom-label": "custom-value",
				}
				Expect(env.Client.Create(env.Context, nodePool)).To(Succeed())
			})
			It("should error when a requirement references a restricted label (karpenter.sh/nodepool)", func() {
				nodePool = test.ReplaceRequirements(nodePool, v1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      v1.NodePoolLabelKey,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"default"},
					}})
				Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
			})
			It("should error when a requirement uses In but has no values", func() {
				nodePool = test.ReplaceRequirements(nodePool, v1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{},
					}})
				Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
			})
			It("should error when a requirement uses an unknown operator", func() {
				nodePool = test.ReplaceRequirements(nodePool, v1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      v1.CapacityTypeLabelKey,
						Operator: "within",
						Values:   []string{v1.CapacityTypeSpot},
					}})
				Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
			})
			It("should error when Gt is used with multiple integer values", func() {
				nodePool = test.ReplaceRequirements(nodePool, v1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpGt,
						Values:   []string{"1000000", "2000000"},
					}})
				Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
			})
			It("should error when Lt is used with multiple integer values", func() {
				nodePool = test.ReplaceRequirements(nodePool, v1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpLt,
						Values:   []string{"1000000", "2000000"},
					}})
				Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
			})
			It("should error when consolidateAfter is negative", func() {
				nodePool.Spec.Disruption.ConsolidationPolicy = v1.ConsolidationPolicyWhenEmpty
				nodePool.Spec.Disruption.ConsolidateAfter = v1.MustParseNillableDuration("-1s")
				Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
			})
			It("should succeed when ConsolidationPolicy=WhenEmptyOrUnderutilized is used with consolidateAfter", func() {
				nodePool.Spec.Disruption.ConsolidationPolicy = v1.ConsolidationPolicyWhenEmptyOrUnderutilized
				nodePool.Spec.Disruption.ConsolidateAfter = v1.MustParseNillableDuration("1m")
				Expect(env.Client.Create(env.Context, nodePool)).To(Succeed())
			})
			It("should error when minValues for a requirement key is negative", func() {
				nodePool = test.ReplaceRequirements(nodePool, v1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"insance-type-1", "insance-type-2"},
					},
					MinValues: lo.ToPtr(-1)},
				)
				Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
			})
			It("should error when minValues for a requirement key is zero", func() {
				nodePool = test.ReplaceRequirements(nodePool, v1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"insance-type-1", "insance-type-2"},
					},
					MinValues: lo.ToPtr(0)},
				)
				Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
			})
			It("should error when minValues for a requirement key is more than 50", func() {
				nodePool = test.ReplaceRequirements(nodePool, v1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"insance-type-1", "insance-type-2"},
					},
					MinValues: lo.ToPtr(51)},
				)
				Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
			})
			It("should error when minValues for a requirement key is greater than the values specified within In operator", func() {
				nodePool = test.ReplaceRequirements(nodePool, v1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"insance-type-1", "insance-type-2"},
					},
					MinValues: lo.ToPtr(3)},
				)
				Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
			})
		})
		Describe("Repair Policy", func() {
			var selector labels.Selector
			var dep *appsv1.Deployment
			var numPods int
			var unhealthyCondition corev1.NodeCondition

			BeforeEach(func() {
				unhealthyCondition = corev1.NodeCondition{
					Type:               corev1.NodeReady,
					Status:             corev1.ConditionFalse,
					LastTransitionTime: metav1.Time{Time: time.Now().Add(-31 * time.Minute)},
				}
				numPods = 1
				// Add pods with a do-not-disrupt annotation so that we can check node metadata before we disrupt
				dep = test.Deployment(test.DeploymentOptions{
					Replicas: int32(numPods),
					PodOptions: test.PodOptions{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "my-app",
							},
							Annotations: map[string]string{
								v1.DoNotDisruptAnnotationKey: "true",
							},
						},
						TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
					},
				})
				selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
			})
			DescribeTable("Conditions", func(unhealthyCondition corev1.NodeCondition) {
				env.ExpectCreated(nodeClass, nodePool, dep)
				pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
				node := env.ExpectCreatedNodeCount("==", 1)[0]
				env.EventuallyExpectInitializedNodeCount("==", 1)

				node = env.ReplaceNodeConditions(node, unhealthyCondition)
				env.ExpectStatusUpdated(node)

				env.EventuallyExpectNotFound(pod, node)
				env.EventuallyExpectHealthyPodCount(selector, numPods)
			},
				// Kubelet Supported Conditions
				Entry("Node Ready False", corev1.NodeCondition{
					Type:               corev1.NodeReady,
					Status:             corev1.ConditionFalse,
					LastTransitionTime: metav1.Time{Time: time.Now().Add(-31 * time.Hour)},
				}),
				Entry("Node Ready Unknown", corev1.NodeCondition{
					Type:               corev1.NodeReady,
					Status:             corev1.ConditionUnknown,
					LastTransitionTime: metav1.Time{Time: time.Now().Add(-31 * time.Hour)},
				}),
			)
			It("should ignore disruption budgets", func() {
				nodePool.Spec.Disruption.Budgets = []v1.Budget{
					{
						Nodes: "0",
					},
				}
				env.ExpectCreated(nodeClass, nodePool, dep)
				pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
				node := env.ExpectCreatedNodeCount("==", 1)[0]
				env.EventuallyExpectInitializedNodeCount("==", 1)

				node = env.ReplaceNodeConditions(node, unhealthyCondition)
				env.ExpectStatusUpdated(node)

				env.EventuallyExpectNotFound(pod, node)
				env.EventuallyExpectHealthyPodCount(selector, numPods)
			})
			It("should ignore do-not-disrupt annotation on node", func() {
				env.ExpectCreated(nodeClass, nodePool, dep)
				pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
				node := env.ExpectCreatedNodeCount("==", 1)[0]
				env.EventuallyExpectInitializedNodeCount("==", 1)

				node.Annotations[v1.DoNotDisruptAnnotationKey] = "true"
				env.ExpectUpdated(node)

				node = env.ReplaceNodeConditions(node, unhealthyCondition)
				env.ExpectStatusUpdated(node)

				env.EventuallyExpectNotFound(pod, node)
				env.EventuallyExpectHealthyPodCount(selector, numPods)
			})
			It("should ignore terminationGracePeriod on the nodepool", func() {
				nodePool.Spec.Template.Spec.TerminationGracePeriod = &metav1.Duration{Duration: time.Hour}
				env.ExpectCreated(nodeClass, nodePool, dep)
				pod := env.EventuallyExpectHealthyPodCount(selector, numPods)[0]
				node := env.ExpectCreatedNodeCount("==", 1)[0]
				env.EventuallyExpectInitializedNodeCount("==", 1)

				node = env.ReplaceNodeConditions(node, unhealthyCondition)
				env.ExpectStatusUpdated(node)

				env.EventuallyExpectNotFound(pod, node)
				env.EventuallyExpectHealthyPodCount(selector, numPods)
			})
		})
	})
})
