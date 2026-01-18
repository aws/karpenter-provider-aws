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

package pdb_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/apis"
	"sigs.k8s.io/karpenter/pkg/utils/pdb"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	karpenterv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
)

var (
	ctx       context.Context
	env       *test.Environment
	podLabels = map[string]string{"pdb-test": "value"}
)

func Test(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "PDBUtils")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(test.WithCRDs(apis.CRDs...), test.WithCRDs(v1alpha1.CRDs...), test.WithFieldIndexers(test.NodeClaimProviderIDFieldIndexer(ctx)))
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("CanEvictPods", func() {
	It("can evict unhealthy pods when UnhealthyPodEvictionPolicy is set to always allow", func() {
		if env.Version.Minor() < 27 {
			Skip("PDB UnhealthyPodEvictionPolicy is only supported in 1.27+")
		}
		podDisruptionBudget := test.PodDisruptionBudget(test.PDBOptions{
			Labels:                     podLabels,
			MinAvailable:               lo.ToPtr(intstr.FromString("100%")),
			UnhealthyPodEvictionPolicy: lo.ToPtr(policyv1.AlwaysAllow),
		})
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
			Conditions: []v1.PodCondition{{Type: v1.PodReady, Status: v1.ConditionFalse}}})
		ExpectApplied(ctx, env.Client, podDisruptionBudget, pod)

		limits, err := pdb.NewLimits(ctx, env.Client)
		Expect(err).NotTo(HaveOccurred())

		violatingPDBs, canEvict := limits.CanEvictPods([]*v1.Pod{pod})
		Expect(violatingPDBs).To(HaveLen(0))
		Expect(canEvict).To(BeTrue())
	})
	It("can't evict unhealthy pods when UnhealthyPodEvictionPolicy is not set", func() {
		podDisruptionBudget := test.PodDisruptionBudget(test.PDBOptions{
			Labels:       podLabels,
			MinAvailable: lo.ToPtr(intstr.FromString("100%")),
		})
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
			Conditions: []v1.PodCondition{{Type: v1.PodReady, Status: v1.ConditionFalse}}})
		ExpectApplied(ctx, env.Client, podDisruptionBudget, pod)

		limits, err := pdb.NewLimits(ctx, env.Client)
		Expect(err).NotTo(HaveOccurred())

		violatingPDBs, canEvict := limits.CanEvictPods([]*v1.Pod{pod})
		Expect(violatingPDBs).To(HaveLen(1))
		Expect(violatingPDBs).To(ContainElement(client.ObjectKeyFromObject(podDisruptionBudget)))
		Expect(canEvict).To(BeFalse())
	})
	It("can evict pods when no PDBs match", func() {
		podDisruptionBudget := test.PodDisruptionBudget(test.PDBOptions{
			Labels: map[string]string{"other": "value"},
		})
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			}})
		ExpectApplied(ctx, env.Client, podDisruptionBudget, pod)

		limits, err := pdb.NewLimits(ctx, env.Client)
		Expect(err).NotTo(HaveOccurred())

		violatingPDBs, canEvict := limits.CanEvictPods([]*v1.Pod{pod})
		Expect(violatingPDBs).To(HaveLen(0))
		Expect(canEvict).To(BeTrue())
	})
	DescribeTable("can't evict pods when disruptions are not allowed for every pod in the list",
		func(podDisruptionBudgets ...*policyv1.PodDisruptionBudget) {
			pod1 := test.Pod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
			})
			pod2 := test.Pod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
			})
			lo.ForEach(podDisruptionBudgets, func(pdb *policyv1.PodDisruptionBudget, _ int) { ExpectApplied(ctx, env.Client, pdb) })
			ExpectApplied(ctx, env.Client, pod1, pod2)

			limits, err := pdb.NewLimits(ctx, env.Client)
			Expect(err).NotTo(HaveOccurred())

			violatingPDBs, canEvict := limits.CanEvictPods([]*v1.Pod{pod1, pod2})
			Expect(violatingPDBs).To(HaveLen(len(podDisruptionBudgets)))
			lo.ForEach(podDisruptionBudgets, func(pdb *policyv1.PodDisruptionBudget, _ int) {
				Expect(violatingPDBs).To(ContainElement(client.ObjectKeyFromObject(pdb)))
			})
			Expect(canEvict).To(BeFalse())
		},
		Entry("100% min available", test.PodDisruptionBudget(test.PDBOptions{
			Labels:       podLabels,
			MinAvailable: lo.ToPtr(intstr.FromString("100%")),
		})),
		Entry("0% max unavailable", test.PodDisruptionBudget(test.PDBOptions{
			Labels:         podLabels,
			MaxUnavailable: lo.ToPtr(intstr.FromString("0%")),
		})),
		Entry("0 max unavailable", test.PodDisruptionBudget(test.PDBOptions{
			Labels:         podLabels,
			MaxUnavailable: lo.ToPtr(intstr.FromInt(0)),
		})),
		Entry("multiple PDBs on the same pod",
			test.PodDisruptionBudget(test.PDBOptions{
				ObjectMeta:   metav1.ObjectMeta{Name: "pdb-1"},
				Labels:       podLabels,
				MinAvailable: lo.ToPtr(intstr.FromString("100%")),
			}),
			test.PodDisruptionBudget(test.PDBOptions{
				ObjectMeta:   metav1.ObjectMeta{Name: "pdb-2"},
				Labels:       podLabels,
				MinAvailable: lo.ToPtr(intstr.FromString("100%")),
			}),
		),
	)
	DescribeTable("can't evict pods when disruptions are not allowed for one pod in the list",
		func(podDisruptionBudgets ...*policyv1.PodDisruptionBudget) {
			pod1 := test.Pod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
			})
			pod2 := test.Pod(test.PodOptions{})
			lo.ForEach(podDisruptionBudgets, func(pdb *policyv1.PodDisruptionBudget, _ int) { ExpectApplied(ctx, env.Client, pdb) })
			ExpectApplied(ctx, env.Client, pod1, pod2)

			limits, err := pdb.NewLimits(ctx, env.Client)
			Expect(err).NotTo(HaveOccurred())

			violatingPDBs, canEvict := limits.CanEvictPods([]*v1.Pod{pod1, pod2})
			Expect(violatingPDBs).To(HaveLen(len(podDisruptionBudgets)))
			lo.ForEach(podDisruptionBudgets, func(pdb *policyv1.PodDisruptionBudget, _ int) {
				Expect(violatingPDBs).To(ContainElement(client.ObjectKeyFromObject(pdb)))
			})
			Expect(canEvict).To(BeFalse())
		},
		Entry("100% min available", test.PodDisruptionBudget(test.PDBOptions{
			Labels:       podLabels,
			MinAvailable: lo.ToPtr(intstr.FromString("100%")),
		})),
		Entry("0% max unavailable", test.PodDisruptionBudget(test.PDBOptions{
			Labels:         podLabels,
			MaxUnavailable: lo.ToPtr(intstr.FromString("0%")),
		})),
		Entry("0 max unavailable", test.PodDisruptionBudget(test.PDBOptions{
			Labels:         podLabels,
			MaxUnavailable: lo.ToPtr(intstr.FromInt(0)),
		})),
		Entry("multiple PDBs on the same pod",
			test.PodDisruptionBudget(test.PDBOptions{
				ObjectMeta:   metav1.ObjectMeta{Name: "pdb-1"},
				Labels:       podLabels,
				MinAvailable: lo.ToPtr(intstr.FromString("100%")),
			}),
			test.PodDisruptionBudget(test.PDBOptions{
				ObjectMeta:   metav1.ObjectMeta{Name: "pdb-2"},
				Labels:       podLabels,
				MinAvailable: lo.ToPtr(intstr.FromString("100%")),
			}),
		),
	)
})

var _ = Describe("IsCurrentlyReschedulable", func() {
	It("considers unhealthy pod as currently reschedulable when UnhealthyPodEvictionPolicy is set to always allow", func() {
		if env.Version.Minor() < 27 {
			Skip("PDB UnhealthyPodEvictionPolicy is only supported in 1.27+")
		}
		podDisruptionBudget := test.PodDisruptionBudget(test.PDBOptions{
			Labels:                     podLabels,
			MinAvailable:               lo.ToPtr(intstr.FromString("100%")),
			UnhealthyPodEvictionPolicy: lo.ToPtr(policyv1.AlwaysAllow),
		})
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
			Conditions: []v1.PodCondition{{Type: v1.PodReady, Status: v1.ConditionFalse}}})
		ExpectApplied(ctx, env.Client, podDisruptionBudget, pod)

		limits, err := pdb.NewLimits(ctx, env.Client)
		Expect(err).NotTo(HaveOccurred())

		Expect(limits.IsCurrentlyReschedulable(pod)).To(BeTrue())
	})
	It("does not consider unhealthy pod as currently reschedulable when UnhealthyPodEvictionPolicy is not set", func() {
		podDisruptionBudget := test.PodDisruptionBudget(test.PDBOptions{
			Labels:       podLabels,
			MinAvailable: lo.ToPtr(intstr.FromString("100%")),
		})
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			},
			Conditions: []v1.PodCondition{{Type: v1.PodReady, Status: v1.ConditionFalse}}})
		ExpectApplied(ctx, env.Client, podDisruptionBudget, pod)

		limits, err := pdb.NewLimits(ctx, env.Client)
		Expect(err).NotTo(HaveOccurred())

		Expect(limits.IsCurrentlyReschedulable(pod)).To(BeFalse())
	})
	It("considers pod as currently reschedulable when no PDBs match", func() {
		podDisruptionBudget := test.PodDisruptionBudget(test.PDBOptions{
			Labels: map[string]string{"other": "value"},
		})
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: podLabels,
			}})
		ExpectApplied(ctx, env.Client, podDisruptionBudget, pod)

		limits, err := pdb.NewLimits(ctx, env.Client)
		Expect(err).NotTo(HaveOccurred())

		Expect(limits.IsCurrentlyReschedulable(pod)).To(BeTrue())
	})
	DescribeTable("pods which are not currently reschedulable due to PDBs",
		func(podDisruptionBudgets ...*policyv1.PodDisruptionBudget) {
			pod := test.Pod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
			})
			lo.ForEach(podDisruptionBudgets, func(pdb *policyv1.PodDisruptionBudget, _ int) { ExpectApplied(ctx, env.Client, pdb) })
			ExpectApplied(ctx, env.Client, pod)

			limits, err := pdb.NewLimits(ctx, env.Client)
			Expect(err).NotTo(HaveOccurred())

			Expect(limits.IsCurrentlyReschedulable(pod)).To(BeFalse())
		},
		Entry("100% min available", test.PodDisruptionBudget(test.PDBOptions{
			Labels:       podLabels,
			MinAvailable: lo.ToPtr(intstr.FromString("100%")),
		})),
		Entry("0% max unavailable", test.PodDisruptionBudget(test.PDBOptions{
			Labels:         podLabels,
			MaxUnavailable: lo.ToPtr(intstr.FromString("0%")),
		})),
		Entry("0 max unavailable", test.PodDisruptionBudget(test.PDBOptions{
			Labels:         podLabels,
			MaxUnavailable: lo.ToPtr(intstr.FromInt(0)),
		})),
		Entry("multiple PDBs on the same pod",
			test.PodDisruptionBudget(test.PDBOptions{
				ObjectMeta:   metav1.ObjectMeta{Name: "pdb-1"},
				Labels:       podLabels,
				MinAvailable: lo.ToPtr(intstr.FromString("100%")),
			}),
			test.PodDisruptionBudget(test.PDBOptions{
				ObjectMeta:   metav1.ObjectMeta{Name: "pdb-2"},
				Labels:       podLabels,
				MinAvailable: lo.ToPtr(intstr.FromString("100%")),
			}),
		),
	)
	It("does not consider pod with do-not-disrupt annotation as currently reschedulable", func() {
		pod := test.Pod(test.PodOptions{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{karpenterv1.DoNotDisruptAnnotationKey: "true"},
				Labels:      podLabels,
			},
		})
		ExpectApplied(ctx, env.Client, pod)

		limits, err := pdb.NewLimits(ctx, env.Client)
		Expect(err).NotTo(HaveOccurred())

		Expect(limits.IsCurrentlyReschedulable(pod)).To(BeFalse())
	})
})
