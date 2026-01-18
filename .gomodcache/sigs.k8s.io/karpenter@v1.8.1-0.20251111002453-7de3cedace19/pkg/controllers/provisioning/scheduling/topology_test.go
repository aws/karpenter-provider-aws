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

package scheduling_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("Topology", func() {
	var nodePool *v1.NodePool
	labels := map[string]string{"test": "test"}
	BeforeEach(func() {
		nodePool = test.NodePool(v1.NodePool{
			Spec: v1.NodePoolSpec{
				Template: v1.NodeClaimTemplate{
					Spec: v1.NodeClaimTemplateSpec{
						Requirements: []v1.NodeSelectorRequirementWithMinValues{
							{
								NodeSelectorRequirement: corev1.NodeSelectorRequirement{
									Key:      v1.CapacityTypeLabelKey,
									Operator: corev1.NodeSelectorOpExists,
								},
							},
						},
					},
				},
			},
		})
	})

	It("should ignore unknown topology keys", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		pods := []*corev1.Pod{
			test.UnschedulablePod(
				test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: []corev1.TopologySpreadConstraint{{
					TopologyKey:       "unknown",
					WhenUnsatisfiable: corev1.DoNotSchedule,
					LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
					MaxSkew:           1,
				}}},
			),
			test.UnschedulablePod(),
		}
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
		ExpectNotScheduled(ctx, env.Client, pods[0])
		ExpectScheduled(ctx, env.Client, pods[1])
	})

	It("should not spread an invalid label selector", func() {
		if env.Version.Minor() >= 24 {
			Skip("Invalid label selector now is denied by admission in K8s >= 1.27.x")
		}
		topology := []corev1.TopologySpreadConstraint{{
			TopologyKey:       corev1.LabelTopologyZone,
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/name": "{{ zqfmgb }}"}},
			MaxSkew:           1,
		}}
		ExpectApplied(ctx, env.Client, nodePool)
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
			test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 2)...)
		ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2))
	})

	It("should not spread when a nil label selector is defined", func() {
		topology := []corev1.TopologySpreadConstraint{{
			TopologyKey:       corev1.LabelTopologyZone,
			WhenUnsatisfiable: corev1.DoNotSchedule,
			MaxSkew:           1,
		}}
		pods := test.UnschedulablePods(test.PodOptions{
			ObjectMeta:                metav1.ObjectMeta{Labels: labels},
			TopologySpreadConstraints: topology,
		}, 2)
		ExpectApplied(ctx, env.Client, nodePool)
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
		ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2))
	})

	Context("Zonal", func() {
		It("should balance pods across zones (match labels)", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 4)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 1, 2))
		})
		It("should balance pods across zones (match expressions)", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "test",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"test"},
						},
					},
				},
				MaxSkew: 1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 4)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 1, 2))
		})
		It("should respect NodePool zonal constraints", func() {
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3"}}}}
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 4)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 1, 2))
		})
		It("should respect NodePool zonal constraints (subset) with requirements", func() {
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2"}}}}
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 4)...,
			)
			// should spread the two pods evenly across the only valid zones in our universe (the two zones from our single nodePool)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2, 2))
		})
		It("should respect NodePool zonal constraints (subset) with labels", func() {
			nodePool.Spec.Template.Labels = lo.Assign(nodePool.Spec.Template.Labels, map[string]string{corev1.LabelTopologyZone: "test-zone-1"})
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 4)...,
			)
			// should spread the two pods evenly across the only valid zones in our universe (the two zones from our single nodePool)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(4))
		})
		It("should respect NodePool zonal constraints (subset) with requirements and labels", func() {
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2"}}}}
			nodePool.Spec.Template.Labels = lo.Assign(nodePool.Spec.Template.Labels, map[string]string{corev1.LabelTopologyZone: "test-zone-1"})
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 4)...,
			)
			// should spread the two pods evenly across the only valid zones in our universe (the two zones from our single nodePool)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(4))
		})
		It("should respect NodePool zonal constraints (subset) with labels across NodePools", func() {
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2"}}}}
			nodePool.Spec.Template.Labels = lo.Assign(nodePool.Spec.Template.Labels, map[string]string{corev1.LabelTopologyZone: "test-zone-1"})
			nodePool2 := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						ObjectMeta: v1.ObjectMeta{
							Labels: map[string]string{
								corev1.LabelTopologyZone: "test-zone-2",
							},
						},
					},
				},
			})
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool, nodePool2)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 4)...,
			)
			// should spread the two pods evenly across the only valid zones in our universe (the two zones from our single nodePool)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2, 2))
		})
		It("should respect NodePool zonal constraints (existing pod)", func() {
			ExpectApplied(ctx, env.Client, nodePool)
			// need enough resource requests that the first node we create fills a node and can't act as an in-flight
			// node for the other pods
			rr := corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.1"),
				},
			}
			pod := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels},
				ResourceRequirements: rr,
				NodeSelector: map[string]string{
					corev1.LabelTopologyZone: "test-zone-3",
				},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)

			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2"}}}}
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, ResourceRequirements: rr, TopologySpreadConstraints: topology}, 6)...,
			)
			// we should have unschedulable pods now, the nodePool can only schedule to zone-1/zone-2, but because of the existing
			// pod in zone-3 it can put a max of two per zone before it would violate max skew
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 2, 2))
		})
		It("should schedule to the non-minimum domain if its all that's available", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           5,
			}}
			rr := corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.1"),
				},
			}
			// force this pod onto zone-1
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1"}}}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels},
					ResourceRequirements: rr, TopologySpreadConstraints: topology}))
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1))

			// force this pod onto zone-2
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-2"}}}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels},
					ResourceRequirements: rr, TopologySpreadConstraints: topology}))
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 1))

			// now only allow scheduling pods on zone-3
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-3"}}}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels},
					ResourceRequirements: rr, TopologySpreadConstraints: topology}, 10)...,
			)

			// max skew of 5, so test-zone-1/2 will have 1 pod each, test-zone-3 will have 6, and the rest will fail to schedule
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 1, 6))
		})
		It("should only schedule to minimum domains if already violating max skew", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			rr := corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.1"),
				},
			}
			createPods := func(count int) []*corev1.Pod {
				var pods []*corev1.Pod
				for i := 0; i < count; i++ {
					pods = append(pods, test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels},
						ResourceRequirements: rr, TopologySpreadConstraints: topology}))
				}
				return pods
			}
			// Spread 9 pods
			ExpectApplied(ctx, env.Client, nodePool)
			pods := createPods(9)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(3, 3, 3))

			// Delete pods to create a skew
			for _, pod := range pods {
				node := ExpectScheduled(ctx, env.Client, pod)
				if node.Labels[corev1.LabelTopologyZone] != "test-zone-1" {
					ExpectDeleted(ctx, env.Client, pod)
				}
			}
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(3))

			// Create 3 more pods, skew should recover
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, createPods(3)...)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(3, 1, 2))
		})
		It("should not violate max-skew when unsat = do not schedule", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			rr := corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.1"),
				},
			}
			// force this pod onto zone-1
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1"}}}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels},
					ResourceRequirements: rr, TopologySpreadConstraints: topology}))
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1))

			// now only allow scheduling pods on zone-2 and zone-3
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-2", "test-zone-3"}}}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels},
					ResourceRequirements: rr, TopologySpreadConstraints: topology}, 10)...,
			)

			// max skew of 1, so test-zone-2/3 will have 2 nodes each and the rest of the pods will fail to schedule
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 2, 2))
		})
		It("should not violate max-skew when unsat = do not schedule (discover domains)", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			rr := corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.1"),
				},
			}
			// force this pod onto zone-1
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1"}}}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, ResourceRequirements: rr}))

			// now only allow scheduling pods on zone-2 and zone-3
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-2", "test-zone-3"}}}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels},
					TopologySpreadConstraints: topology, ResourceRequirements: rr}, 10)...,
			)

			// max skew of 1, so test-zone-2/3 will have 2 nodes each and the rest of the pods will fail to schedule since
			// test-zone-1 has 1 pods in it.
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 2, 2))
		})
		It("should only count running/scheduled pods with matching labels scheduled to nodes with a corresponding domain", func() {
			wrongNamespace := test.RandomName()
			firstNode := test.Node(test.NodeOptions{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{corev1.LabelTopologyZone: "test-zone-1"}}})
			secondNode := test.Node(test.NodeOptions{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{corev1.LabelTopologyZone: "test-zone-2"}}})
			thirdNode := test.Node(test.NodeOptions{}) // missing topology domain
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool, firstNode, secondNode, thirdNode, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: wrongNamespace}})
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(firstNode))
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(secondNode))
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(thirdNode))
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.Pod(test.PodOptions{NodeName: firstNode.Name}),                                                                                               // ignored, missing labels
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}}),                                                                          // ignored, pending
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: thirdNode.Name}),                                                // ignored, no domain on node
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels, Namespace: wrongNamespace}, NodeName: firstNode.Name}),                     // ignored, wrong namespace
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels, DeletionTimestamp: &metav1.Time{Time: time.Now().Add(10 * time.Second)}}}), // ignored, terminating
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: firstNode.Name, Phase: corev1.PodFailed}),                       // ignored, phase=Failed
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: firstNode.Name, Phase: corev1.PodSucceeded}),                    // ignored, phase=Succeeded
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: firstNode.Name}),
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: firstNode.Name}),
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: secondNode.Name}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
			)
			nodes := corev1.NodeList{}
			Expect(env.Client.List(ctx, &nodes)).To(Succeed())
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2, 2, 1))
		})
		It("should match all pods when labelSelector is not specified", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePod(),
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1))
		})
		It("should handle interdependent selectors", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			pods := test.UnschedulablePods(test.PodOptions{TopologySpreadConstraints: topology}, 5)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				pods...,
			)
			// This is weird, but the topology label selector is used for determining domain counts. The pod that
			// owns the topology is what the spread actually applies to.  In this test case, there are no pods matching
			// the label selector, so the max skew is zero.  This means we can pack all the pods onto the same node since
			// it doesn't violate the topology spread constraint (i.e. adding new pods doesn't increase skew since the
			// pods we are adding don't count toward skew). This behavior is called out at
			// https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/ , though it's not
			// recommended for users.
			nodeNames := sets.NewString()
			for _, p := range pods {
				nodeNames.Insert(p.Spec.NodeName)
			}
			Expect(nodeNames).To(HaveLen(1))
		})
		It("should respect minDomains constraints", func() {
			if env.Version.Minor() < 24 {
				Skip("MinDomains TopologySpreadConstraint is only available starting in K8s >= 1.24.x")
			}
			var minDomains int32 = 3
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2"}}}}
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
				MinDomains:        &minDomains,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 3)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 1))
		})
		It("satisfied minDomains constraints (equal) should allow expected pod scheduling", func() {
			if env.Version.Minor() < 24 {
				Skip("MinDomains TopologySpreadConstraint is only available starting in K8s >= 1.24.x")
			}
			var minDomains int32 = 3
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3"}}}}
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
				MinDomains:        &minDomains,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 11)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(4, 4, 3))
		})
		It("satisfied minDomains constraints (greater than minimum) should allow expected pod scheduling", func() {
			if env.Version.Minor() < 24 {
				Skip("MinDomains TopologySpreadConstraint is only available starting in K8s >= 1.24.x")
			}
			var minDomains int32 = 2
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3"}}}}
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
				MinDomains:        &minDomains,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 11)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(4, 4, 3))
		})
	})

	Context("Hostname", func() {
		It("should balance pods across nodes", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 4)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 1, 1, 1))
		})
		It("should balance pods on the same hostname up to maxskew", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           4,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 4)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(4))
		})
		It("balance multiple deployments with hostname topology spread", func() {
			// Issue #1425
			spreadPod := func(appName string) test.PodOptions {
				return test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": appName,
						},
					},
					TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
						{
							MaxSkew:           1,
							TopologyKey:       corev1.LabelHostname,
							WhenUnsatisfiable: corev1.DoNotSchedule,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": appName},
							},
						},
					},
				}
			}

			ExpectApplied(ctx, env.Client, nodePool)
			pods := []*corev1.Pod{
				test.UnschedulablePod(spreadPod("app1")), test.UnschedulablePod(spreadPod("app1")),
				test.UnschedulablePod(spreadPod("app2")), test.UnschedulablePod(spreadPod("app2")),
			}
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			for _, p := range pods {
				ExpectScheduled(ctx, env.Client, p)
			}
			nodes := corev1.NodeList{}
			Expect(env.Client.List(ctx, &nodes)).To(Succeed())
			// this wasn't part of #1425, but ensures that we launch the minimum number of nodes
			Expect(nodes.Items).To(HaveLen(2))
		})
		It("balance multiple deployments with hostname topology spread & varying arch", func() {
			// Issue #1425
			spreadPod := func(appName, arch string) test.PodOptions {
				return test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": appName,
						},
					},
					NodeRequirements: []corev1.NodeSelectorRequirement{
						{
							Key:      corev1.LabelArchStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{arch},
						},
					},
					TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
						{
							MaxSkew:           1,
							TopologyKey:       corev1.LabelHostname,
							WhenUnsatisfiable: corev1.DoNotSchedule,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": appName},
							},
						},
					},
				}
			}

			ExpectApplied(ctx, env.Client, nodePool)
			pods := []*corev1.Pod{
				test.UnschedulablePod(spreadPod("app1", v1.ArchitectureAmd64)), test.UnschedulablePod(spreadPod("app1", v1.ArchitectureAmd64)),
				test.UnschedulablePod(spreadPod("app2", v1.ArchitectureArm64)), test.UnschedulablePod(spreadPod("app2", v1.ArchitectureArm64)),
			}
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			for _, p := range pods {
				ExpectScheduled(ctx, env.Client, p)
			}
			nodes := corev1.NodeList{}
			Expect(env.Client.List(ctx, &nodes)).To(Succeed())
			// same test as the previous one, but now the architectures are different so we need four nodes in total
			Expect(nodes.Items).To(HaveLen(4))
		})
	})

	Context("CapacityType", func() {
		It("should balance pods across capacity types", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       v1.CapacityTypeLabelKey,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 4)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2, 2))
		})
		It("should respect NodePool capacity type constraints", func() {
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: v1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{v1.CapacityTypeSpot, v1.CapacityTypeOnDemand}}}}
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       v1.CapacityTypeLabelKey,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 4)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2, 2))
		})
		It("should not violate max-skew when unsat = do not schedule (capacity type)", func() {
			// this test can pass in a flaky manner if we don't restrict our min domain selection to valid choices
			// per the nodePool spec
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       v1.CapacityTypeLabelKey,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			rr := corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.1"),
				},
			}
			// force this pod onto spot
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: v1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{v1.CapacityTypeSpot}}}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels},
					ResourceRequirements: rr, TopologySpreadConstraints: topology}))
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1))

			// now only allow scheduling pods on on-demand
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: v1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{v1.CapacityTypeOnDemand}}}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels},
					ResourceRequirements: rr, TopologySpreadConstraints: topology}, 5)...,
			)

			// max skew of 1, so on-demand will have 2 pods and the rest of the pods will fail to schedule
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 2))
		})
		It("should violate max-skew when unsat = schedule anyway (capacity type)", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       v1.CapacityTypeLabelKey,
				WhenUnsatisfiable: corev1.ScheduleAnyway,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			rr := corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU: resource.MustParse("1.1"),
				},
			}
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: v1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{v1.CapacityTypeSpot}}}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels},
					ResourceRequirements: rr, TopologySpreadConstraints: topology}))
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1))

			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: v1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{v1.CapacityTypeOnDemand}}}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels},
					ResourceRequirements: rr, TopologySpreadConstraints: topology}, 5)...,
			)

			// max skew of 1, on-demand will end up with 5 pods even though spot has a single pod
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 5))
		})
		It("should only count running/scheduled pods with matching labels scheduled to nodes with a corresponding domain", func() {
			wrongNamespace := test.RandomName()
			firstNode := test.Node(test.NodeOptions{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot}}})
			secondNode := test.Node(test.NodeOptions{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeOnDemand}}})
			thirdNode := test.Node(test.NodeOptions{}) // missing topology capacity type
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       v1.CapacityTypeLabelKey,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool, firstNode, secondNode, thirdNode, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: wrongNamespace}})
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(firstNode))
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(secondNode))
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(thirdNode))
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.Pod(test.PodOptions{NodeName: firstNode.Name}),                                                                                               // ignored, missing labels
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}}),                                                                          // ignored, pending
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: thirdNode.Name}),                                                // ignored, no domain on node
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels, Namespace: wrongNamespace}, NodeName: firstNode.Name}),                     // ignored, wrong namespace
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels, DeletionTimestamp: &metav1.Time{Time: time.Now().Add(10 * time.Second)}}}), // ignored, terminating
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: firstNode.Name, Phase: corev1.PodFailed}),                       // ignored, phase=Failed
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: firstNode.Name, Phase: corev1.PodSucceeded}),                    // ignored, phase=Succeeded
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: firstNode.Name}),
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: firstNode.Name}),
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: secondNode.Name}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
			)
			nodes := corev1.NodeList{}
			Expect(env.Client.List(ctx, &nodes)).To(Succeed())
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2, 3))
		})
		It("should match all pods when labelSelector is not specified", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       v1.CapacityTypeLabelKey,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePod(),
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1))
		})
		It("should handle interdependent selectors", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			pods := test.UnschedulablePods(test.PodOptions{TopologySpreadConstraints: topology}, 5)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			// This is weird, but the topology label selector is used for determining domain counts. The pod that
			// owns the topology is what the spread actually applies to.  In this test case, there are no pods matching
			// the label selector, so the max skew is zero.  This means we can pack all the pods onto the same node since
			// it doesn't violate the topology spread constraint (i.e. adding new pods doesn't increase skew since the
			// pods we are adding don't count toward skew). This behavior is called out at
			// https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/ , though it's not
			// recommended for users.
			nodeNames := sets.NewString()
			for _, p := range pods {
				nodeNames.Insert(p.Spec.NodeName)
			}
			Expect(nodeNames).To(HaveLen(1))
		})
		It("should balance pods across capacity-types (node required affinity constrained)", func() {
			ExpectApplied(ctx, env.Client, nodePool)
			pods := test.UnschedulablePods(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				NodeRequirements: []corev1.NodeSelectorRequirement{
					// launch this on-demand pod in zone-1
					{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1"}},
					{Key: v1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{"on-demand"}},
				},
			}, 1)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			ExpectScheduled(ctx, env.Client, pods[0])

			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       v1.CapacityTypeLabelKey,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			// Try to run 5 pods, with a node selector restricted to test-zone-2, they should all schedule on the same
			// spot node. This doesn't violate the max-skew of 1 as the node selector requirement here excludes the
			// existing on-demand pod from counting within this topology.
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: labels},
					// limit our nodePool to only creating spot nodes
					NodeRequirements: []corev1.NodeSelectorRequirement{
						{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-2"}},
						{Key: v1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{"spot"}},
					},
					TopologySpreadConstraints: topology,
				}, 5)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 5))
		})
		It("should balance pods across capacity-types (no constraints)", func() {
			rr := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
			}
			ExpectApplied(ctx, env.Client, nodePool)
			pod := test.UnschedulablePod(test.PodOptions{
				ObjectMeta:   metav1.ObjectMeta{Labels: labels},
				NodeSelector: map[string]string{corev1.LabelInstanceTypeStable: "single-pod-instance-type"},
				NodeRequirements: []corev1.NodeSelectorRequirement{
					{
						Key:      v1.CapacityTypeLabelKey,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"on-demand"},
					},
				},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)

			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       v1.CapacityTypeLabelKey,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			// limit our nodePool to only creating spot nodes
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: v1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{"spot"}}},
			}

			// since there is no node selector on this pod, the topology can see the single on-demand node that already
			// exists and that limits us to scheduling 2 more spot pods before we would violate max-skew
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{
					ObjectMeta:                metav1.ObjectMeta{Labels: labels},
					ResourceRequirements:      rr,
					TopologySpreadConstraints: topology,
				}, 5)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 2))
		})
		It("should balance pods across arch (no constraints)", func() {
			rr := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
			}
			ExpectApplied(ctx, env.Client, nodePool)
			pod := test.UnschedulablePod(test.PodOptions{
				ObjectMeta:   metav1.ObjectMeta{Labels: labels},
				NodeSelector: map[string]string{corev1.LabelInstanceTypeStable: "single-pod-instance-type"},
				NodeRequirements: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelArchStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"amd64"},
					},
				},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)

			ExpectScheduled(ctx, env.Client, pod)

			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelArchStable,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			// limit our nodePool to only creating arm64 nodes
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"arm64"}}}}

			// since there is no node selector on this pod, the topology can see the single arm64 node that already
			// exists and that limits us to scheduling 2 more spot pods before we would violate max-skew
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{
					ObjectMeta:                metav1.ObjectMeta{Labels: labels},
					ResourceRequirements:      rr,
					TopologySpreadConstraints: topology,
				}, 5)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 2))
		})
	})

	Context("Combined Hostname and Zonal Topology", func() {
		It("should spread pods while respecting both constraints (hostname and zonal)", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}, {
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           3,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 2)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 1))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 3)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2, 2, 1))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 5)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(4, 3, 3))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 11)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(7, 7, 7))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 3)))
		})
		It("should balance pods across NodePool requirements", func() {
			spotNodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						Spec: v1.NodeClaimTemplateSpec{
							Requirements: []v1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      v1.CapacityTypeLabelKey,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{v1.CapacityTypeSpot},
									},
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      "capacity.spread.4-1",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"2", "3", "4", "5"},
									},
								},
							},
						},
					},
				},
			})
			onDemandNodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						Spec: v1.NodeClaimTemplateSpec{
							Requirements: []v1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      v1.CapacityTypeLabelKey,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{v1.CapacityTypeOnDemand},
									},
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      "capacity.spread.4-1",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"1"},
									},
								},
							},
						},
					},
				},
			})

			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       "capacity.spread.4-1",
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, spotNodePool, onDemandNodePool)
			pods := test.UnschedulablePods(test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: topology,
			}, 20)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			for _, p := range pods {
				ExpectScheduled(ctx, env.Client, p)
			}

			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(4, 4, 4, 4, 4))
			// due to the spread across nodePools, we've forced a 4:1 spot to on-demand spread
			ExpectSkew(ctx, env.Client, "default", &corev1.TopologySpreadConstraint{
				TopologyKey:       v1.CapacityTypeLabelKey,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}).To(ConsistOf(4, 16))
		})

		It("should spread pods while respecting both constraints", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}, {
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.ScheduleAnyway,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2"}}}}

			// create a second nodePool that can't provision at all
			nodePoolB := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						Spec: v1.NodeClaimTemplateSpec{
							Requirements: []v1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-3"},
									},
								},
							},
						},
					},
					Limits: v1.Limits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("0")}),
				},
			})
			ExpectApplied(ctx, env.Client, nodePool, nodePoolB)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 10)...,
			)

			// should get one pod per zone, can't schedule to test-zone-3 since that nodePool is effectively disabled
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 1))
			// and one pod per node
			ExpectSkew(ctx, env.Client, "default", &topology[1]).To(ConsistOf(1, 1))
		})

		It("should spread pods while respecting both constraints", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       v1.CapacityTypeLabelKey,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}, {
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           3,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 2)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 1))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 3)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(3, 2))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 5)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(5, 5))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 11)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(11, 10))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 3)))
		})
	})

	Context("MatchLabelKeys", func() {
		BeforeEach(func() {
			if env.Version.Minor() < 27 {
				Skip("MatchLabelKeys only enabled by default forK8S >= 1.27.x")
			}
		})

		It("should support matchLabelKeys", func() {
			matchedLabel := "test-label"
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MatchLabelKeys:    []string{matchedLabel},
				MaxSkew:           1,
			}}
			count := 2
			pods := test.UnschedulablePods(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: lo.Assign(labels, map[string]string{matchedLabel: "value-a"}),
				},
				TopologySpreadConstraints: topology,
			}, count)
			pods = append(pods, test.UnschedulablePods(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: lo.Assign(labels, map[string]string{matchedLabel: "value-b"}),
				},
				TopologySpreadConstraints: topology,
			}, count)...)
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			// Expect two nodes to be created, each with a single pod from each "deployment". If matchLabelKeys didn't work as
			// expected, we would see 4 nodes, each with 1 pod.
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2, 2))
		})

		It("should ignore unknown labels specified in matchLabelKeys", func() {
			matchedLabel := "test-label"
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MatchLabelKeys:    []string{matchedLabel},
				MaxSkew:           1,
			}}
			pods := test.UnschedulablePods(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				TopologySpreadConstraints: topology,
			}, 4)
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 1, 1, 1))
		})
	})

	Context("NodeTaintsPolicy", func() {
		BeforeEach(func() {
			if env.Version.Minor() < 26 {
				Skip("NodeTaintsPolicy only enabled by default for K8s >= 1.26.x")
			}
		})

		It("should balance pods across a label (NodeTaintsPolicy=ignore)", func() {

			const spreadLabel = "fake-label"
			nodePool.Spec.Template.Labels = map[string]string{
				spreadLabel: "baz",
			}
			node1 := test.Node(test.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						spreadLabel: "foo",
					},
				},
				Taints: []corev1.Taint{
					{
						Key:    "taintname",
						Value:  "taintvalue",
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				}})

			node2 := test.Node(test.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						spreadLabel: "bar",
					},
				},
				Taints: []corev1.Taint{
					{
						Key:    "taintname",
						Value:  "taintvalue",
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				}})
			ExpectApplied(ctx, env.Client, nodePool, node1, node2)

			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node1))
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node2))

			// there are now three domains for spreadLabel that Karpenter should know about, foo/bar from the two existing
			// nodes and baz from the node pool
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov)
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       spreadLabel,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
				NodeTaintsPolicy:  lo.ToPtr(corev1.NodeInclusionPolicyIgnore),
			}}

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: labels},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
					TopologySpreadConstraints: topology,
				}, 5)...,
			)
			// we're aware of three domains, but can only schedule a single pod to the domain where we're allowed
			// to create nodes
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1))
		})
		It("should balance pods across a label (NodeTaintsPolicy=honor)", func() {
			const spreadLabel = "fake-label"
			nodePool.Spec.Template.Labels = map[string]string{
				spreadLabel: "baz",
			}
			node1 := test.Node(test.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						spreadLabel: "foo",
					},
				},
				Taints: []corev1.Taint{
					{
						Key:    "taintname",
						Value:  "taintvalue",
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				}})

			node2 := test.Node(test.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						spreadLabel: "bar",
					},
				},
				Taints: []corev1.Taint{
					{
						Key:    "taintname",
						Value:  "taintvalue",
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				}})
			ExpectApplied(ctx, env.Client, nodePool, node1, node2)

			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node1))
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node2))

			// since two of the nodes are tainted, Karpenter should not consider them for topology domain discovery
			// purposes
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov)
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       spreadLabel,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
				NodeTaintsPolicy:  lo.ToPtr(corev1.NodeInclusionPolicyHonor),
			}}

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: labels},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
					TopologySpreadConstraints: topology,
				}, 5)...,
			)
			// and should schedule all of the pods on the same node
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(5))
		})
		It("should balance pods across a label when discovered from the nodepool (NodeTaintsPolicy=ignore)", func() {
			const spreadLabel = "fake-label"
			const taintKey = "taint-key"
			nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, v1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      spreadLabel,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"foo"},
				},
			})
			taintedNodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						Spec: v1.NodeClaimTemplateSpec{
							Taints: []corev1.Taint{
								{
									Key:    taintKey,
									Value:  "taint-value",
									Effect: corev1.TaintEffectNoSchedule,
								},
							},
							Requirements: []v1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      v1.CapacityTypeLabelKey,
										Operator: corev1.NodeSelectorOpExists,
									},
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      spreadLabel,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"bar"},
									},
								},
							},
						},
					},
				},
			})

			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       spreadLabel,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
				NodeTaintsPolicy:  lo.ToPtr(corev1.NodeInclusionPolicyIgnore),
			}}

			pods := test.UnschedulablePods(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				TopologySpreadConstraints: topology,
			}, 2)

			ExpectApplied(ctx, env.Client, nodePool, taintedNodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)

			// should succeed to schedule one pod to domain "foo", but fail to schedule the other to domain "bar"
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1))
		})
		It("should balance pods across a label when discovered from the nodepool (NodeTaintsPolicy=honor)", func() {
			const spreadLabel = "fake-label"
			const taintKey = "taint-key"
			nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, v1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      spreadLabel,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"foo"},
				},
			})
			taintedNodePool := test.NodePool(v1.NodePool{
				Spec: v1.NodePoolSpec{
					Template: v1.NodeClaimTemplate{
						Spec: v1.NodeClaimTemplateSpec{
							Taints: []corev1.Taint{
								{
									Key:    taintKey,
									Value:  "taint-value",
									Effect: corev1.TaintEffectNoSchedule,
								},
							},
							Requirements: []v1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      v1.CapacityTypeLabelKey,
										Operator: corev1.NodeSelectorOpExists,
									},
								},
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      spreadLabel,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"bar"},
									},
								},
							},
						},
					},
				},
			})

			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       spreadLabel,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
				NodeTaintsPolicy:  lo.ToPtr(corev1.NodeInclusionPolicyHonor),
			}}

			pods := test.UnschedulablePods(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				TopologySpreadConstraints: topology,
			}, 2)

			ExpectApplied(ctx, env.Client, nodePool, taintedNodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)

			// should schedule all pods to domain "foo", ignoring bar since pods don't tolerate
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2))
		})
		It("should balance pods across a label when mutually exclusive NodePools (by taints) share domains (NodeTaintsPolicy=honor)", func() {
			const spreadLabel = "fake-label"
			const taintKey = "taint-key"

			nodePools := lo.Map([][]string{{"foo", "bar"}, {"foo", "baz"}}, func(domains []string, i int) *v1.NodePool {
				return test.NodePool(v1.NodePool{
					Spec: v1.NodePoolSpec{
						Template: v1.NodeClaimTemplate{
							Spec: v1.NodeClaimTemplateSpec{
								Taints: []corev1.Taint{
									{
										Key:    taintKey,
										Value:  fmt.Sprintf("nodepool-%d", i),
										Effect: corev1.TaintEffectNoSchedule,
									},
								},
								Requirements: []v1.NodeSelectorRequirementWithMinValues{
									{
										NodeSelectorRequirement: corev1.NodeSelectorRequirement{
											Key:      v1.CapacityTypeLabelKey,
											Operator: corev1.NodeSelectorOpExists,
										},
									},
									{
										NodeSelectorRequirement: corev1.NodeSelectorRequirement{
											Key:      spreadLabel,
											Operator: corev1.NodeSelectorOpIn,
											Values:   domains,
										},
									},
								},
							},
						},
					},
				})
			})

			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       spreadLabel,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
				NodeTaintsPolicy:  lo.ToPtr(corev1.NodeInclusionPolicyHonor),
			}}

			// Create two pods which tolerate the first nodepool's taint, and four which tolerate the second's
			pods := lo.Flatten(lo.Map(nodePools, func(np *v1.NodePool, i int) []*corev1.Pod {
				return test.UnschedulablePods(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
					},
					TopologySpreadConstraints: topology,
					Tolerations: []corev1.Toleration{{
						Key:    taintKey,
						Effect: corev1.TaintEffectNoSchedule,
						Value:  np.Spec.Template.Spec.Taints[0].Value,
					}},
				}, (i+1)*2)
			}))

			ExpectApplied(ctx, env.Client, nodePools[0], nodePools[1])
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)

			// There are three total domains (foo, bar, and baz) available across two NodePools. Each NodePool can create nodes
			// in domain "foo", but "bar" and "baz" are exclusive per NodePool. Each NodePool is also tainted.
			// There are two "deployments", one consists of two pods and the other of four. Each deployment has a toleration for
			// one of the NodePools. Since the taint policy is honor, deployment one should be spread across domains "foo" and
			// "bar", and deployment two should be spread across domains "foo" and "baz". As a result, we should see the
			// following skew:
			// - foo: 2 (nodepool 1 or 2)
			// - bar: 1 (nodepool 1)
			// - baz: 3 (nodepool 2)
			// The pods in deployment one only count against nodes with domain foo and bar, which has a skew of one. The pods in
			// deployment two only count against nodes with domain foo and baz, which also has a skew of one. It's important to
			// remember that all of the pods select against each other, the only filtering is based on the node.
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(3, 2, 1))
		})
	})

	Context("NodeAffinityPolicy", func() {
		BeforeEach(func() {
			if env.Version.Minor() < 26 {
				Skip("NodeAffinityPolicy ony enabled by default for K8s >= 1.26.x")
			}
		})
		It("should balance pods across a label (NodeAffinityPolicy=ignore)", func() {
			const spreadLabel = "fake-label"
			const affinityLabel = "selector"
			const affinityMismatch = "mismatch"
			const affinityMatch = "value"

			nodePool.Spec.Template.Labels = map[string]string{
				spreadLabel:   "baz",
				affinityLabel: affinityMatch,
			}
			node1 := test.Node(test.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						spreadLabel:   "foo",
						affinityLabel: affinityMismatch,
					},
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				}})

			node2 := test.Node(test.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						spreadLabel:   "bar",
						affinityLabel: affinityMismatch,
					},
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				}})
			ExpectApplied(ctx, env.Client, nodePool, node1, node2)

			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node1))
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node2))

			// there are now three domains for spreadLabel that Karpenter should know about since we are ignoring the
			// fact that the pod can't schedule to two of them because of a required node affinity. foo/bar from the two
			// existing nodes with an affinityLabel=affinityMismatch label and baz from the node pool
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov)
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:        spreadLabel,
				WhenUnsatisfiable:  corev1.DoNotSchedule,
				LabelSelector:      &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:            1,
				NodeAffinityPolicy: lo.ToPtr(corev1.NodeInclusionPolicyIgnore),
			}}

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: labels},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
					NodeSelector: map[string]string{
						affinityLabel: affinityMatch,
					},
					TopologySpreadConstraints: topology,
				}, 5)...,
			)
			// we're aware of three domains, but can only schedule a single pod to the domain where we're allowed
			// to create nodes
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1))
		})
		It("should balance pods across a label (NodeAffinityPolicy=honor)", func() {
			const spreadLabel = "fake-label"
			const affinityLabel = "selector"
			const affinityMismatch = "mismatch"
			const affinityMatch = "value"

			nodePool.Spec.Template.Labels = map[string]string{
				spreadLabel:   "baz",
				affinityLabel: affinityMatch,
			}
			node1 := test.Node(test.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						spreadLabel:   "foo",
						affinityLabel: affinityMismatch,
					},
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				}})

			node2 := test.Node(test.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						spreadLabel:   "bar",
						affinityLabel: affinityMismatch,
					},
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				}})
			ExpectApplied(ctx, env.Client, nodePool, node1, node2)

			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node1))
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node2))

			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov)
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:        spreadLabel,
				WhenUnsatisfiable:  corev1.DoNotSchedule,
				LabelSelector:      &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:            1,
				NodeAffinityPolicy: lo.ToPtr(corev1.NodeInclusionPolicyHonor),
			}}

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: labels},
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("1"),
						},
					},
					NodeSelector: map[string]string{
						affinityLabel: affinityMatch,
					},
					TopologySpreadConstraints: topology,
				}, 5)...,
			)
			// we're only aware of the single domain, since we honor the node affinity policy and all of the pods
			// should schedule there
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(5))
		})
	})
	Context("Combined Zonal and Capacity Type Topology", func() {
		It("should spread pods while respecting both constraints", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       v1.CapacityTypeLabelKey,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}, {
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 2)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).ToNot(ContainElements(BeNumerically(">", 1)))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 1)))

			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 3)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).ToNot(ContainElements(BeNumerically(">", 3)))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 2)))

			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 3)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).ToNot(ContainElements(BeNumerically(">", 5)))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 4)))

			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, 11)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).ToNot(ContainElements(BeNumerically(">", 11)))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 7)))
		})
	})

	Context("Combined Hostname, Zonal, and Capacity Type Topology", func() {
		It("should spread pods while respecting all constraints", func() {
			// ensure we've got an instance type for every zone/capacity-type pair
			cloudProvider.InstanceTypes = fake.InstanceTypesAssorted()
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       v1.CapacityTypeLabelKey,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}, {
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           2,
			}, {
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           3,
			}}

			// add varying numbers of pods, checking after each scheduling to ensure that our max required max skew
			// has not been violated for each constraint
			for i := 1; i < 15; i++ {
				pods := test.UnschedulablePods(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}, i)
				ExpectApplied(ctx, env.Client, nodePool)
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
				ExpectMaxSkew(ctx, env.Client, "default", &topology[0]).To(BeNumerically("<=", 1))
				ExpectMaxSkew(ctx, env.Client, "default", &topology[1]).To(BeNumerically("<=", 2))
				ExpectMaxSkew(ctx, env.Client, "default", &topology[2]).To(BeNumerically("<=", 3))
				for _, pod := range pods {
					ExpectScheduled(ctx, env.Client, pod)
				}
			}
		})
	})

	// https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/#interaction-with-node-affinity-and-node-selectors
	Context("Combined Zonal Topology and Node Affinity", func() {
		It("should limit spread options by nodeSelector", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				append(
					test.UnschedulablePods(test.PodOptions{
						ObjectMeta:                metav1.ObjectMeta{Labels: labels},
						TopologySpreadConstraints: topology,
						NodeSelector:              map[string]string{corev1.LabelTopologyZone: "test-zone-1"},
					}, 5),
					test.UnschedulablePods(test.PodOptions{
						ObjectMeta:                metav1.ObjectMeta{Labels: labels},
						TopologySpreadConstraints: topology,
						NodeSelector:              map[string]string{corev1.LabelTopologyZone: "test-zone-2"},
					}, 10)...,
				)...,
			)
			// we limit the zones of each pod via node selectors, which causes the topology spreads to only consider
			// the single zone as the only valid domain for the topology spread allowing us to schedule multiple pods per domain
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(5, 10))
		})
		It("should limit spread options by node requirements", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{
					ObjectMeta:                metav1.ObjectMeta{Labels: labels},
					TopologySpreadConstraints: topology,
					NodeRequirements: []corev1.NodeSelectorRequirement{
						{
							Key:      corev1.LabelTopologyZone,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"test-zone-1", "test-zone-2"},
						},
					},
				}, 10)...)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(5, 5))
		})
		It("should limit spread options by required node affinity", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{
					ObjectMeta:                metav1.ObjectMeta{Labels: labels},
					TopologySpreadConstraints: topology,
					NodeRequirements: []corev1.NodeSelectorRequirement{{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{
						"test-zone-1", "test-zone-2",
					}}},
				}, 6)...)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(3, 3))

			// open the nodePool back to up so it can see all zones again
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3"}}}}

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, test.UnschedulablePod(test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: topology,
				NodeRequirements: []corev1.NodeSelectorRequirement{{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{
					"test-zone-2", "test-zone-3",
				}}},
			}))

			// it will schedule on the currently empty zone-3 even though max-skew is violated as it improves max-skew
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(3, 3, 1))

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{
					ObjectMeta:                metav1.ObjectMeta{Labels: labels},
					TopologySpreadConstraints: topology,
				}, 5)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(4, 4, 4))
		})
		It("should not limit spread options by preferred node affinity", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{
					ObjectMeta:                metav1.ObjectMeta{Labels: labels},
					TopologySpreadConstraints: topology,
					NodePreferences: []corev1.NodeSelectorRequirement{{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{
						"test-zone-1", "test-zone-2",
					}}},
				}, 6)...)

			// scheduling shouldn't be affected since it's a preferred affinity
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2, 2, 2))
		})
	})

	// https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/#interaction-with-node-affinity-and-node-selectors
	Context("Combined Capacity Type Topology and Node Affinity", func() {
		It("should limit spread options by nodeSelector", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       v1.CapacityTypeLabelKey,
				WhenUnsatisfiable: corev1.ScheduleAnyway,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				append(
					test.UnschedulablePods(test.PodOptions{
						ObjectMeta:                metav1.ObjectMeta{Labels: labels},
						TopologySpreadConstraints: topology,
						NodeSelector:              map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot},
					}, 5),
					test.UnschedulablePods(test.PodOptions{
						ObjectMeta:                metav1.ObjectMeta{Labels: labels},
						TopologySpreadConstraints: topology,
						NodeSelector:              map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeOnDemand},
					}, 5)...,
				)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(5, 5))
		})
		It("should limit spread options by node affinity (capacity type)", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       v1.CapacityTypeLabelKey,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			// need to limit the rules to spot or else it will know that on-demand has 0 pods and won't violate the max-skew
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{
					ObjectMeta:                metav1.ObjectMeta{Labels: labels},
					TopologySpreadConstraints: topology,
					NodeRequirements: []corev1.NodeSelectorRequirement{
						{Key: v1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{v1.CapacityTypeSpot}},
					},
				}, 3)...)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(3))

			// open the rules back to up so it can see all capacity types
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, test.UnschedulablePod(test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: topology,
				NodeRequirements: []corev1.NodeSelectorRequirement{
					{Key: v1.CapacityTypeLabelKey, Operator: corev1.NodeSelectorOpIn, Values: []string{v1.CapacityTypeOnDemand, v1.CapacityTypeSpot}},
				},
			}))

			// it will schedule on the currently empty on-demand even though max-skew is violated as it improves max-skew
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(3, 1))

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov,
				test.UnschedulablePods(test.PodOptions{
					ObjectMeta:                metav1.ObjectMeta{Labels: labels},
					TopologySpreadConstraints: topology,
				}, 5)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(5, 4))
		})
	})

	Context("Pod Affinity/Anti-Affinity", func() {
		It("should schedule a pod with empty pod affinity and anti-affinity", func() {
			ExpectApplied(ctx, env.Client)
			ExpectApplied(ctx, env.Client, nodePool)
			pod := test.UnschedulablePod(test.PodOptions{
				PodRequirements:     []corev1.PodAffinityTerm{},
				PodAntiRequirements: []corev1.PodAffinityTerm{},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should respect pod affinity (hostname)", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			affLabels := map[string]string{"security": "s2"}

			affPod1 := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})
			// affPod2 will try to get scheduled with affPod1
			affPod2 := test.UnschedulablePod(test.PodOptions{PodRequirements: []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: affLabels,
				},
				TopologyKey: corev1.LabelHostname,
			}}})

			var pods []*corev1.Pod
			pods = append(pods, test.UnschedulablePods(test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: topology,
			}, 10)...)
			pods = append(pods, affPod1)
			pods = append(pods, affPod2)

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			n1 := ExpectScheduled(ctx, env.Client, affPod1)
			n2 := ExpectScheduled(ctx, env.Client, affPod2)
			// should be scheduled on the same node
			Expect(n1.Name).To(Equal(n2.Name))
		})
		It("should respect pod affinity (arch)", func() {
			affLabels := map[string]string{"security": "s2"}
			tsc := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: affLabels},
				MaxSkew:           1,
			}}

			affPod1 := test.UnschedulablePod(test.PodOptions{
				TopologySpreadConstraints: tsc,
				ObjectMeta:                metav1.ObjectMeta{Labels: affLabels},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
				},
				NodeSelector: map[string]string{
					corev1.LabelArchStable: "arm64",
				}})
			// affPod2 will try to get scheduled with affPod1
			affPod2 := test.UnschedulablePod(test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: affLabels},
				TopologySpreadConstraints: tsc,
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
				},
				PodRequirements: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: corev1.LabelArchStable,
				}}})

			pods := []*corev1.Pod{affPod1, affPod2}

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			n1 := ExpectScheduled(ctx, env.Client, affPod1)
			n2 := ExpectScheduled(ctx, env.Client, affPod2)
			// should be scheduled on a node with the same arch
			Expect(n1.Labels[corev1.LabelArchStable]).To(Equal(n2.Labels[corev1.LabelArchStable]))
			// but due to TSC, not on the same node
			Expect(n1.Name).ToNot(Equal(n2.Name))
		})
		It("should respect self pod affinity (hostname)", func() {
			affLabels := map[string]string{"security": "s2"}

			pods := test.UnschedulablePods(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: affLabels,
				},
				PodRequirements: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: corev1.LabelHostname,
				}},
			}, 3)

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			nodeNames := map[string]struct{}{}
			for _, p := range pods {
				n := ExpectScheduled(ctx, env.Client, p)
				nodeNames[n.Name] = struct{}{}
			}
			Expect(len(nodeNames)).To(Equal(1))
		})
		It("should respect self pod affinity for first empty topology domain only (hostname)", func() {
			affLabels := map[string]string{"security": "s2"}
			createPods := func() []*corev1.Pod {
				return test.UnschedulablePods(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: affLabels,
					},
					PodRequirements: []corev1.PodAffinityTerm{{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: affLabels,
						},
						TopologyKey: corev1.LabelHostname,
					}},
				}, 10)
			}
			ExpectApplied(ctx, env.Client, nodePool)
			pods := createPods()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			nodeNames := map[string]struct{}{}
			unscheduledCount := 0
			scheduledCount := 0
			for _, p := range pods {
				p = ExpectPodExists(ctx, env.Client, p.Name, p.Namespace)
				if p.Spec.NodeName == "" {
					unscheduledCount++
				} else {
					nodeNames[p.Spec.NodeName] = struct{}{}
					scheduledCount++
				}
			}
			// the node can only hold 5 pods, so we should get a single node with 5 pods and 5 unschedulable pods from that batch
			Expect(len(nodeNames)).To(Equal(1))
			Expect(scheduledCount).To(BeNumerically("==", 5))
			Expect(unscheduledCount).To(BeNumerically("==", 5))

			// and pods in a different batch should not schedule as well even if the node is not ready yet
			pods = createPods()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			for _, p := range pods {
				ExpectNotScheduled(ctx, env.Client, p)
			}
		})
		It("should respect self pod affinity for first empty topology domain only (hostname/constrained zones)", func() {
			affLabels := map[string]string{"security": "s2"}
			// put one pod in test-zone-1, this does affect pod affinity even though we have different node selectors.
			// The node selector and required node affinity restrictions to topology counting only apply to topology spread.
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, test.UnschedulablePod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: affLabels,
				},
				NodeSelector: map[string]string{
					corev1.LabelTopologyZone: "test-zone-1",
				},
				PodRequirements: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: corev1.LabelHostname,
				}},
			}))

			pods := test.UnschedulablePods(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: affLabels,
				},
				NodeRequirements: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelTopologyZone,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"test-zone-2", "test-zone-3"},
					},
				},
				PodRequirements: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: corev1.LabelHostname,
				}},
			}, 10)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			for _, p := range pods {
				// none of this should schedule
				ExpectNotScheduled(ctx, env.Client, p)
			}
		})
		It("should respect self pod affinity (zone)", func() {
			affLabels := map[string]string{"security": "s2"}

			pods := test.UnschedulablePods(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: affLabels,
				},
				PodRequirements: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: corev1.LabelTopologyZone,
				}},
			}, 3)

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			nodeNames := map[string]struct{}{}
			for _, p := range pods {
				n := ExpectScheduled(ctx, env.Client, p)
				nodeNames[n.Name] = struct{}{}
			}
			Expect(len(nodeNames)).To(Equal(1))
		})
		It("should respect self pod affinity (zone w/ constraint)", func() {
			affLabels := map[string]string{"security": "s2"}
			// the pod needs to provide it's own zonal affinity, but we further limit it to only being on test-zone-3
			pods := test.UnschedulablePods(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: affLabels,
				},
				PodRequirements: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: corev1.LabelTopologyZone,
				}},
				NodeRequirements: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelTopologyZone,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"test-zone-3"},
					},
				},
			}, 3)
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			nodeNames := map[string]struct{}{}
			for _, p := range pods {
				n := ExpectScheduled(ctx, env.Client, p)
				nodeNames[n.Name] = struct{}{}
				Expect(n.Labels[corev1.LabelTopologyZone]).To(Equal("test-zone-3"))
			}
			Expect(len(nodeNames)).To(Equal(1))
		})
		It("should allow two nodes to be created when two pods with matching affinities have incompatible selectors", func() {
			affLabels := map[string]string{"security": "s1"}
			// the pod needs to provide it's own zonal affinity, but we further limit it to only being on test-zone-2
			pod1 := test.UnschedulablePod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: affLabels,
				},
				PodRequirements: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: corev1.LabelTopologyZone,
				}},
				NodeRequirements: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelTopologyZone,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"test-zone-2"},
					},
				},
			})
			// the pod needs to provide it's own zonal affinity, but we further limit it to only being on test-zone-3
			pod2 := test.UnschedulablePod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: affLabels,
				},
				PodRequirements: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: corev1.LabelTopologyZone,
				}},
				NodeRequirements: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelTopologyZone,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"test-zone-3"},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1, pod2)
			nodeNames := map[string]struct{}{}

			n1 := ExpectScheduled(ctx, env.Client, pod1)
			nodeNames[n1.Name] = struct{}{}
			Expect(n1.Labels[corev1.LabelTopologyZone]).To(Equal("test-zone-2"))

			n2 := ExpectScheduled(ctx, env.Client, pod2)
			nodeNames[n2.Name] = struct{}{}
			Expect(n2.Labels[corev1.LabelTopologyZone]).To(Equal("test-zone-3"))
			Expect(len(nodeNames)).To(Equal(2))
		})
		It("should allow violation of preferred pod affinity", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			affPod2 := test.UnschedulablePod(test.PodOptions{PodPreferences: []corev1.WeightedPodAffinityTerm{{
				Weight: 50,
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"security": "s2"},
					},
					TopologyKey: corev1.LabelHostname,
				},
			}}})

			var pods []*corev1.Pod
			pods = append(pods, test.UnschedulablePods(test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: topology,
			}, 10)...)

			pods = append(pods, affPod2)

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			// should be scheduled as the pod it has affinity to doesn't exist, but it's only a preference and not a
			// hard constraints
			ExpectScheduled(ctx, env.Client, affPod2)

		})
		It("should allow violation of preferred pod anti-affinity", func() {
			affPods := test.UnschedulablePods(test.PodOptions{PodAntiPreferences: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 50,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
						TopologyKey: corev1.LabelTopologyZone,
					},
				},
			}}, 10)

			var pods []*corev1.Pod
			pods = append(pods, test.UnschedulablePods(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: []corev1.TopologySpreadConstraint{{
					TopologyKey:       corev1.LabelTopologyZone,
					WhenUnsatisfiable: corev1.DoNotSchedule,
					LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
					MaxSkew:           1,
				}},
			}, 3)...)

			pods = append(pods, affPods...)

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			for _, aff := range affPods {
				ExpectScheduled(ctx, env.Client, aff)
			}

		})
		It("should separate nodes using simple pod anti-affinity on hostname", func() {
			affLabels := map[string]string{"security": "s2"}
			// pod affinity/anti-affinity are bidirectional, so run this a few times to ensure we handle it regardless
			// of pod scheduling order
			ExpectApplied(ctx, env.Client, nodePool)
			for i := 0; i < 10; i++ {
				affPod1 := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})
				// affPod2 will avoid affPod1
				affPod2 := test.UnschedulablePod(test.PodOptions{PodAntiRequirements: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: corev1.LabelHostname,
				}}})

				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, affPod2, affPod1)
				n1 := ExpectScheduled(ctx, env.Client, affPod1)
				n2 := ExpectScheduled(ctx, env.Client, affPod2)
				// should not be scheduled on the same node
				Expect(n1.Name).ToNot(Equal(n2.Name))
			}
		})
		It("should not violate pod anti-affinity on zone", func() {
			affLabels := map[string]string{"security": "s2"}
			zone1Pod := test.UnschedulablePod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: affLabels},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
				},
				NodeSelector: map[string]string{corev1.LabelTopologyZone: "test-zone-1"}})
			zone2Pod := test.UnschedulablePod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: affLabels},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
				},
				NodeSelector: map[string]string{corev1.LabelTopologyZone: "test-zone-2"}})
			zone3Pod := test.UnschedulablePod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: affLabels},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
				},
				NodeSelector: map[string]string{corev1.LabelTopologyZone: "test-zone-3"}})

			affPod := test.UnschedulablePod(test.PodOptions{
				PodAntiRequirements: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: corev1.LabelTopologyZone,
				}}})

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, zone1Pod, zone2Pod, zone3Pod, affPod)
			// the three larger zone specific pods should get scheduled first due to first fit descending onto one
			// node per zone.
			ExpectScheduled(ctx, env.Client, zone1Pod)
			ExpectScheduled(ctx, env.Client, zone2Pod)
			ExpectScheduled(ctx, env.Client, zone3Pod)
			// the pod with anti-affinity
			ExpectNotScheduled(ctx, env.Client, affPod)
		})
		It("should not violate pod anti-affinity on zone (other schedules first)", func() {
			affLabels := map[string]string{"security": "s2"}
			pod := test.UnschedulablePod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: affLabels},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
				}})
			affPod := test.UnschedulablePod(test.PodOptions{
				PodAntiRequirements: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: corev1.LabelTopologyZone,
				}}})

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod, affPod)
			// the pod we need to avoid schedules first, but we don't know where.
			ExpectScheduled(ctx, env.Client, pod)
			// the pod with anti-affinity
			ExpectNotScheduled(ctx, env.Client, affPod)
		})
		It("should not violate pod anti-affinity (arch)", func() {
			affLabels := map[string]string{"security": "s2"}
			tsc := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: affLabels},
				MaxSkew:           1,
			}}

			affPod1 := test.UnschedulablePod(test.PodOptions{
				TopologySpreadConstraints: tsc,
				ObjectMeta:                metav1.ObjectMeta{Labels: affLabels},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
				},
				NodeSelector: map[string]string{
					corev1.LabelArchStable: "arm64",
				}})

			// affPod2 will try to get scheduled on a node with a different archi from affPod1. Due to resource
			// requests we try to schedule it last
			affPod2 := test.UnschedulablePod(test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: affLabels},
				TopologySpreadConstraints: tsc,
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
				},
				PodAntiRequirements: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: corev1.LabelArchStable,
				}}})

			pods := []*corev1.Pod{affPod1, affPod2}

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			n1 := ExpectScheduled(ctx, env.Client, affPod1)
			n2 := ExpectScheduled(ctx, env.Client, affPod2)
			// should not be scheduled on nodes with the same arch
			Expect(n1.Labels[corev1.LabelArchStable]).ToNot(Equal(n2.Labels[corev1.LabelArchStable]))
		})
		It("should violate preferred pod anti-affinity on zone (inverse)", func() {
			affLabels := map[string]string{"security": "s2"}
			anti := []corev1.WeightedPodAffinityTerm{
				{
					Weight: 10,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: affLabels,
						},
						TopologyKey: corev1.LabelTopologyZone,
					},
				},
			}
			rr := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
			}
			zone1Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiPreferences:   anti,
				NodeSelector:         map[string]string{corev1.LabelTopologyZone: "test-zone-1"}})
			zone2Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiPreferences:   anti,
				NodeSelector:         map[string]string{corev1.LabelTopologyZone: "test-zone-2"}})
			zone3Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiPreferences:   anti,
				NodeSelector:         map[string]string{corev1.LabelTopologyZone: "test-zone-3"}})

			affPod := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, zone1Pod, zone2Pod, zone3Pod, affPod)
			// three pods with anti-affinity will schedule first due to first fit-descending
			ExpectScheduled(ctx, env.Client, zone1Pod)
			ExpectScheduled(ctx, env.Client, zone2Pod)
			ExpectScheduled(ctx, env.Client, zone3Pod)
			// the anti-affinity was a preference, so this can schedule
			ExpectScheduled(ctx, env.Client, affPod)
		})
		It("should not violate pod anti-affinity on zone (inverse)", func() {
			affLabels := map[string]string{"security": "s2"}
			anti := []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: affLabels,
				},
				TopologyKey: corev1.LabelTopologyZone,
			}}
			rr := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
			}
			zone1Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiRequirements:  anti,
				NodeSelector:         map[string]string{corev1.LabelTopologyZone: "test-zone-1"}})
			zone2Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiRequirements:  anti,
				NodeSelector:         map[string]string{corev1.LabelTopologyZone: "test-zone-2"}})
			zone3Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiRequirements:  anti,
				NodeSelector:         map[string]string{corev1.LabelTopologyZone: "test-zone-3"}})

			affPod := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, zone1Pod, zone2Pod, zone3Pod, affPod)
			// three pods with anti-affinity will schedule first due to first fit-descending
			ExpectScheduled(ctx, env.Client, zone1Pod)
			ExpectScheduled(ctx, env.Client, zone2Pod)
			ExpectScheduled(ctx, env.Client, zone3Pod)
			// this pod with no anti-affinity rules can't schedule. It has no anti-affinity rules, but every zone has a
			// pod with anti-affinity rules that prevent it from scheduling
			ExpectNotScheduled(ctx, env.Client, affPod)
		})
		It("should not violate pod anti-affinity on zone (Schrödinger)", func() {
			affLabels := map[string]string{"security": "s2"}
			anti := []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: affLabels,
				},
				TopologyKey: corev1.LabelTopologyZone,
			}}
			zoneAnywherePod := test.UnschedulablePod(test.PodOptions{
				PodAntiRequirements: anti,
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
				},
			})

			affPod := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, zoneAnywherePod, affPod)
			// the pod with anti-affinity will schedule first due to first fit-descending, but we don't know which zone it landed in
			node1 := ExpectScheduled(ctx, env.Client, zoneAnywherePod)

			// this pod cannot schedule since the pod with anti-affinity could potentially be in any zone
			affPod = ExpectNotScheduled(ctx, env.Client, affPod)

			// a second batching will now allow the pod to schedule as the zoneAnywherePod has been committed to a zone
			// by the actual node creation
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, affPod)
			node2 := ExpectScheduled(ctx, env.Client, affPod)
			Expect(node1.Labels[corev1.LabelTopologyZone]).ToNot(Equal(node2.Labels[corev1.LabelTopologyZone]))
		})
		It("should not violate pod anti-affinity on zone (inverse w/existing nodes)", func() {
			affLabels := map[string]string{"security": "s2"}
			anti := []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: affLabels,
				},
				TopologyKey: corev1.LabelTopologyZone,
			}}
			rr := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
			}
			zone1Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiRequirements:  anti,
				NodeSelector:         map[string]string{corev1.LabelTopologyZone: "test-zone-1"}})
			zone2Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiRequirements:  anti,
				NodeSelector:         map[string]string{corev1.LabelTopologyZone: "test-zone-2"}})
			zone3Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiRequirements:  anti,
				NodeSelector:         map[string]string{corev1.LabelTopologyZone: "test-zone-3"}})

			affPod := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})

			// provision these so we get three nodes that exist in the cluster with anti-affinity to a pod that we will
			// then try to schedule
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, zone1Pod, zone2Pod, zone3Pod)
			node1 := ExpectScheduled(ctx, env.Client, zone1Pod)
			node2 := ExpectScheduled(ctx, env.Client, zone2Pod)
			node3 := ExpectScheduled(ctx, env.Client, zone3Pod)

			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node1))
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node2))
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node3))
			ExpectReconcileSucceeded(ctx, podStateController, client.ObjectKeyFromObject(zone1Pod))
			ExpectReconcileSucceeded(ctx, podStateController, client.ObjectKeyFromObject(zone2Pod))
			ExpectReconcileSucceeded(ctx, podStateController, client.ObjectKeyFromObject(zone3Pod))

			ExpectReconcileSucceeded(ctx, podStateController, client.ObjectKeyFromObject(zone1Pod))
			ExpectReconcileSucceeded(ctx, podStateController, client.ObjectKeyFromObject(zone2Pod))
			ExpectReconcileSucceeded(ctx, podStateController, client.ObjectKeyFromObject(zone3Pod))

			// this pod with no anti-affinity rules can't schedule. It has no anti-affinity rules, but every zone has an
			// existing pod (not from this batch) with anti-affinity rules that prevent it from scheduling
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, affPod)
			ExpectNotScheduled(ctx, env.Client, affPod)
		})
		It("should violate preferred pod anti-affinity on zone (inverse w/existing nodes)", func() {
			affLabels := map[string]string{"security": "s2"}
			anti := []corev1.WeightedPodAffinityTerm{
				{
					Weight: 10,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: affLabels,
						},
						TopologyKey: corev1.LabelTopologyZone,
					},
				},
			}
			rr := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
			}
			zone1Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiPreferences:   anti,
				NodeSelector:         map[string]string{corev1.LabelTopologyZone: "test-zone-1"}})
			zone2Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiPreferences:   anti,
				NodeSelector:         map[string]string{corev1.LabelTopologyZone: "test-zone-2"}})
			zone3Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiPreferences:   anti,
				NodeSelector:         map[string]string{corev1.LabelTopologyZone: "test-zone-3"}})

			affPod := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})

			// provision these so we get three nodes that exist in the cluster with anti-affinity to a pod that we will
			// then try to schedule
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, zone1Pod, zone2Pod, zone3Pod)
			node1 := ExpectScheduled(ctx, env.Client, zone1Pod)
			node2 := ExpectScheduled(ctx, env.Client, zone2Pod)
			node3 := ExpectScheduled(ctx, env.Client, zone3Pod)

			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node1))
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node2))
			ExpectReconcileSucceeded(ctx, nodeStateController, client.ObjectKeyFromObject(node3))
			ExpectReconcileSucceeded(ctx, podStateController, client.ObjectKeyFromObject(zone1Pod))
			ExpectReconcileSucceeded(ctx, podStateController, client.ObjectKeyFromObject(zone2Pod))
			ExpectReconcileSucceeded(ctx, podStateController, client.ObjectKeyFromObject(zone3Pod))

			// this pod with no anti-affinity rules can schedule, though it couldn't if the anti-affinity were required
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, affPod)
			ExpectScheduled(ctx, env.Client, affPod)
		})
		It("should allow violation of a pod affinity preference with a conflicting required constraint", func() {
			affLabels := map[string]string{"security": "s2"}
			constraint := corev1.TopologySpreadConstraint{
				MaxSkew:           1,
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: labels,
				},
			}
			affPod1 := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})
			affPods := test.UnschedulablePods(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				// limit these pods to one per host
				TopologySpreadConstraints: []corev1.TopologySpreadConstraint{constraint},
				// with a preference to the other pod
				PodPreferences: []corev1.WeightedPodAffinityTerm{{
					Weight: 50,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: affLabels,
						},
						TopologyKey: corev1.LabelHostname,
					},
				}}}, 3)
			ExpectApplied(ctx, env.Client, nodePool)
			pods := append(affPods, affPod1)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			// all pods should be scheduled since the affinity term is just a preference
			for _, pod := range pods {
				ExpectScheduled(ctx, env.Client, pod)
			}
			// and we'll get three nodes due to the topology spread
			ExpectSkew(ctx, env.Client, "", &constraint).To(ConsistOf(1, 1, 1))
		})
		It("should support pod anti-affinity with a zone topology", func() {
			affLabels := map[string]string{"security": "s2"}

			// affPods will avoid being scheduled in the same zone
			createPods := func() []*corev1.Pod {
				return test.UnschedulablePods(test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: affLabels},
					PodAntiRequirements: []corev1.PodAffinityTerm{{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: affLabels,
						},
						TopologyKey: corev1.LabelTopologyZone,
					}}}, 3)
			}

			top := &corev1.TopologySpreadConstraint{TopologyKey: corev1.LabelTopologyZone}

			// One of the downsides of late committal is that absent other constraints, it takes multiple batches of
			// scheduling for zonal anti-affinities to work themselves out.  The first schedule, we know that the pod
			// will land in test-zone-1, test-zone-2, or test-zone-3, but don't know which it collapses to until the
			// node is actually created.

			// one pod pod will schedule
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, createPods()...)
			ExpectSkew(ctx, env.Client, "default", top).To(ConsistOf(1))
			// delete all of the unscheduled ones as provisioning will only bind pods passed into the provisioning call
			// the scheduler looks at all pods though, so it may assume a pod from this batch schedules and no others do
			ExpectDeleteAllUnscheduledPods(ctx, env.Client)

			// second pod in a second zone
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, createPods()...)
			ExpectSkew(ctx, env.Client, "default", top).To(ConsistOf(1, 1))
			ExpectDeleteAllUnscheduledPods(ctx, env.Client)

			// third pod in the last zone
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, createPods()...)
			ExpectSkew(ctx, env.Client, "default", top).To(ConsistOf(1, 1, 1))
			ExpectDeleteAllUnscheduledPods(ctx, env.Client)

			// and nothing else can schedule
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, createPods()...)
			ExpectSkew(ctx, env.Client, "default", top).To(ConsistOf(1, 1, 1))
			ExpectDeleteAllUnscheduledPods(ctx, env.Client)
		})
		It("should not schedule pods with affinity to a non-existent pod", func() {
			affLabels := map[string]string{"security": "s2"}
			affPods := test.UnschedulablePods(test.PodOptions{
				PodRequirements: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: corev1.LabelTopologyZone,
				}}}, 10)

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, affPods...)
			// the pod we have affinity to is not in the cluster, so all of these pods are unschedulable
			for _, p := range affPods {
				ExpectNotScheduled(ctx, env.Client, p)
			}
		})
		It("should support pod affinity with zone topology (unconstrained target)", func() {
			affLabels := map[string]string{"security": "s2"}

			// the pod that the others have an affinity to
			targetPod := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})

			// affPods all want to schedule in the same zone as targetPod, but can't as it's zone is undetermined
			affPods := test.UnschedulablePods(test.PodOptions{
				PodRequirements: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: corev1.LabelTopologyZone,
				}}}, 10)

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, append(affPods, targetPod)...)
			top := &corev1.TopologySpreadConstraint{TopologyKey: corev1.LabelTopologyZone}
			// these pods can't schedule as the pod they have affinity to isn't limited to any particular zone
			for i := range affPods {
				ExpectNotScheduled(ctx, env.Client, affPods[i])
				affPods[i] = ExpectPodExists(ctx, env.Client, affPods[i].Name, affPods[i].Namespace)
			}
			ExpectSkew(ctx, env.Client, "default", top).To(ConsistOf(1))

			// now that targetPod has been scheduled to a node, it's zone is committed and the pods with affinity to it
			// should schedule in the same zone
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, affPods...)
			for _, pod := range affPods {
				ExpectScheduled(ctx, env.Client, pod)
			}
			ExpectSkew(ctx, env.Client, "default", top).To(ConsistOf(11))
		})
		It("should support pod affinity with zone topology (constrained target)", func() {
			affLabels := map[string]string{"security": "s2"}

			// the pod that the others have an affinity to
			affPod1 := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels},
				NodeRequirements: []corev1.NodeSelectorRequirement{
					{
						Key:      corev1.LabelTopologyZone,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"test-zone-1"},
					},
				}})

			// affPods will all be scheduled in the same zone as affPod1
			affPods := test.UnschedulablePods(test.PodOptions{
				PodRequirements: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: corev1.LabelTopologyZone,
				}}}, 10)

			affPods = append(affPods, affPod1)

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, affPods...)
			top := &corev1.TopologySpreadConstraint{TopologyKey: corev1.LabelTopologyZone}
			ExpectSkew(ctx, env.Client, "default", top).To(ConsistOf(11))
		})
		It("should handle multiple dependent affinities", func() {
			dbLabels := map[string]string{"type": "db", "spread": "spread"}
			webLabels := map[string]string{"type": "web", "spread": "spread"}
			cacheLabels := map[string]string{"type": "cache", "spread": "spread"}
			uiLabels := map[string]string{"type": "ui", "spread": "spread"}
			for i := 0; i < 50; i++ {
				ExpectApplied(ctx, env.Client, nodePool.DeepCopy())
				// we have to schedule DB -> Web -> Cache -> UI in that order or else there are pod affinity violations
				pods := []*corev1.Pod{
					test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: dbLabels}}),
					test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: webLabels},
						PodRequirements: []corev1.PodAffinityTerm{{
							LabelSelector: &metav1.LabelSelector{MatchLabels: dbLabels},
							TopologyKey:   corev1.LabelHostname},
						}}),
					test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: cacheLabels},
						PodRequirements: []corev1.PodAffinityTerm{{
							LabelSelector: &metav1.LabelSelector{MatchLabels: webLabels},
							TopologyKey:   corev1.LabelHostname},
						}}),
					test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: uiLabels},
						PodRequirements: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{MatchLabels: cacheLabels},
								TopologyKey:   corev1.LabelHostname},
						}}),
				}
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
				for i := range pods {
					ExpectScheduled(ctx, env.Client, pods[i])
				}
				ExpectCleanedUp(ctx, env.Client)
				cluster.Reset()
			}
		})
		It("should fail to schedule pods with unsatisfiable dependencies", func() {
			dbLabels := map[string]string{"type": "db", "spread": "spread"}
			webLabels := map[string]string{"type": "web", "spread": "spread"}
			ExpectApplied(ctx, env.Client, nodePool)
			// this pods wants to schedule with a non-existent pod, this test just ensures that the scheduling loop
			// doesn't infinite loop
			pod := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: dbLabels},
				PodRequirements: []corev1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{MatchLabels: webLabels},
						TopologyKey:   corev1.LabelHostname,
					},
				}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should filter pod affinity topologies by namespace, no matching pods", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			ExpectApplied(ctx, env.Client, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other-ns-no-match"}})
			affLabels := map[string]string{"security": "s2"}

			affPod1 := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels, Namespace: "other-ns-no-match"}})
			// affPod2 will try to get scheduled with affPod1
			affPod2 := test.UnschedulablePod(test.PodOptions{PodRequirements: []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: affLabels,
				},
				TopologyKey: corev1.LabelHostname,
			}}})

			var pods []*corev1.Pod
			// creates 10 nodes due to topo spread
			pods = append(pods, test.UnschedulablePods(test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: topology,
			}, 10)...)
			pods = append(pods, affPod1)
			pods = append(pods, affPod2)

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)

			// the target pod gets scheduled
			ExpectScheduled(ctx, env.Client, affPod1)
			// but the one with affinity does not since the target pod is not in the same namespace and doesn't
			// match the namespace list or namespace selector
			ExpectNotScheduled(ctx, env.Client, affPod2)
		})
		It("should filter pod affinity topologies by namespace, matching pods namespace list", func() {
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			ExpectApplied(ctx, env.Client, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other-ns-list"}})
			affLabels := map[string]string{"security": "s2"}

			affPod1 := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels, Namespace: "other-ns-list"}})
			// affPod2 will try to get scheduled with affPod1
			affPod2 := test.UnschedulablePod(test.PodOptions{PodRequirements: []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: affLabels,
				},
				Namespaces:  []string{"other-ns-list"},
				TopologyKey: corev1.LabelHostname,
			}}})

			var pods []*corev1.Pod
			// create 10 nodes
			pods = append(pods, test.UnschedulablePods(test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: topology,
			}, 10)...)
			// put our target pod on one of them
			pods = append(pods, affPod1)
			// and our pod with affinity should schedule on the same node
			pods = append(pods, affPod2)

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			n1 := ExpectScheduled(ctx, env.Client, affPod1)
			n2 := ExpectScheduled(ctx, env.Client, affPod2)
			// should be scheduled on the same node
			Expect(n1.Name).To(Equal(n2.Name))
		})
		It("should filter pod affinity topologies by namespace, empty namespace selector", func() {
			if env.Version.Minor() < 21 {
				Skip("namespace selector is only supported on K8s >= 1.21.x")
			}
			topology := []corev1.TopologySpreadConstraint{{
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			ExpectApplied(ctx, env.Client, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "empty-ns-selector", Labels: map[string]string{"foo": "bar"}}})
			affLabels := map[string]string{"security": "s2"}

			affPod1 := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels, Namespace: "empty-ns-selector"}})
			// affPod2 will try to get scheduled with affPod1
			affPod2 := test.UnschedulablePod(test.PodOptions{PodRequirements: []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: affLabels,
				},
				// select all pods in all namespaces since the selector is empty
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{}},
				TopologyKey:       corev1.LabelHostname,
			}}})

			var pods []*corev1.Pod
			// create 10 nodes
			pods = append(pods, test.UnschedulablePods(test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: topology,
			}, 10)...)
			// put our target pod on one of them
			pods = append(pods, affPod1)
			// and our pod with affinity should schedule on the same node
			pods = append(pods, affPod2)

			ExpectApplied(ctx, env.Client, nodePool)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			n1 := ExpectScheduled(ctx, env.Client, affPod1)
			n2 := ExpectScheduled(ctx, env.Client, affPod2)
			// should be scheduled on the same node due to the empty namespace selector
			Expect(n1.Name).To(Equal(n2.Name))
		})
	})
})

var _ = Describe("Taints", func() {
	var nodePool *v1.NodePool
	BeforeEach(func() {
		nodePool = test.NodePool(v1.NodePool{
			Spec: v1.NodePoolSpec{
				Template: v1.NodeClaimTemplate{
					Spec: v1.NodeClaimTemplateSpec{
						Requirements: []v1.NodeSelectorRequirementWithMinValues{
							{
								NodeSelectorRequirement: corev1.NodeSelectorRequirement{
									Key:      v1.CapacityTypeLabelKey,
									Operator: corev1.NodeSelectorOpExists,
								},
							},
						},
					},
				},
			},
		})
	})
	It("should taint nodes with NodePool taints", func() {
		nodePool.Spec.Template.Spec.Taints = []corev1.Taint{{Key: "test", Value: "bar", Effect: corev1.TaintEffectNoSchedule}}
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod(
			test.PodOptions{Tolerations: []corev1.Toleration{{Effect: corev1.TaintEffectNoSchedule, Operator: corev1.TolerationOpExists}}},
		)
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Spec.Taints).To(ContainElement(nodePool.Spec.Template.Spec.Taints[0]))
	})
	It("should schedule pods that tolerate NodePool constraints", func() {
		nodePool.Spec.Template.Spec.Taints = []corev1.Taint{{Key: "test-key", Value: "test-value", Effect: corev1.TaintEffectNoSchedule}}
		ExpectApplied(ctx, env.Client, nodePool)
		pods := []*corev1.Pod{
			// Tolerates with OpExists
			test.UnschedulablePod(test.PodOptions{Tolerations: []corev1.Toleration{{Key: "test-key", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}}}),
			// Tolerates with OpEqual
			test.UnschedulablePod(test.PodOptions{Tolerations: []corev1.Toleration{{Key: "test-key", Value: "test-value", Operator: corev1.TolerationOpEqual, Effect: corev1.TaintEffectNoSchedule}}}),
		}
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
		for _, pod := range pods {
			ExpectScheduled(ctx, env.Client, pod)
		}
		ExpectApplied(ctx, env.Client, nodePool)
		otherPods := []*corev1.Pod{
			// Missing toleration
			test.UnschedulablePod(),
			// key mismatch with OpExists
			test.UnschedulablePod(test.PodOptions{Tolerations: []corev1.Toleration{{Key: "invalid", Operator: corev1.TolerationOpExists}}}),
			// value mismatch
			test.UnschedulablePod(test.PodOptions{Tolerations: []corev1.Toleration{{Key: "test-key", Operator: corev1.TolerationOpEqual, Effect: corev1.TaintEffectNoSchedule}}}),
		}
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, otherPods...)
		for _, pod := range otherPods {
			ExpectNotScheduled(ctx, env.Client, pod)
		}
	})
	It("should provision nodes with taints and schedule pods if the taint is only a startup taint", func() {
		nodePool.Spec.Template.Spec.StartupTaints = []corev1.Taint{{Key: "ignore-me", Value: "nothing-to-see-here", Effect: corev1.TaintEffectNoSchedule}}

		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)
	})
	It("should not generate taints for OpExists", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod(test.PodOptions{Tolerations: []corev1.Toleration{{Key: "test-key", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute}}})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Spec.Taints).To(HaveLen(1)) // Expect no taints generated beyond the default
	})
})
