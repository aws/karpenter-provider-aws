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

package v1alpha5

import (
	"context"
	"strings"
	"testing"

	"github.com/Pallinder/go-randomdata"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/ptr"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

var ctx context.Context

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validation")
}

var _ = Describe("Validation", func() {
	var provisioner *Provisioner

	BeforeEach(func() {
		provisioner = &Provisioner{
			ObjectMeta: metav1.ObjectMeta{
				Name: strings.ToLower(randomdata.SillyName()),
			},
			Spec: ProvisionerSpec{
				Limits: Limits{
					Resources: v1.ResourceList{
						v1.ResourceCPU: *resource.NewScaledQuantity(10, 0),
					},
				},
			},
		}
	})

	It("should fail on negative expiry ttl", func() {
		provisioner.Spec.TTLSecondsUntilExpired = ptr.Int64(-1)
		Expect(provisioner.Validate(ctx)).ToNot(Succeed())
	})

	It("should fail on negative empty ttl", func() {
		provisioner.Spec.TTLSecondsAfterEmpty = ptr.Int64(-1)
		Expect(provisioner.Validate(ctx)).ToNot(Succeed())
	})

	Context("Limits", func() {
		It("should allow undefined limits", func() {
			provisioner.Spec.Limits = Limits{}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should allow empty limits", func() {
			provisioner.Spec.Limits = Limits{Resources: v1.ResourceList{}}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
	})

	Context("Labels", func() {
		It("should allow unrecognized labels", func() {
			provisioner.Spec.Labels = map[string]string{"foo": randomdata.SillyName()}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should fail for invalid label keys", func() {
			provisioner.Spec.Labels = map[string]string{"spaces are not allowed": randomdata.SillyName()}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail for invalid label values", func() {
			provisioner.Spec.Labels = map[string]string{randomdata.SillyName(): "/ is not allowed"}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail for restricted labels", func() {
			for label := range RestrictedLabels {
				provisioner.Spec.Labels = map[string]string{label: randomdata.SillyName()}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
		})
		It("should fail for restricted label domains", func() {
			for label := range RestrictedLabelDomains {
				provisioner.Spec.Labels = map[string]string{label + "/unknown": randomdata.SillyName()}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
		})
		It("should allow labels kOps require", func() {
			provisioner.Spec.Labels = map[string]string{
				"kops.k8s.io/instancegroup": "karpenter-nodes",
				"kops.k8s.io/gpu":           "1",
			}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
	})
	Context("Taints", func() {
		It("should succeed for valid taints", func() {
			provisioner.Spec.Taints = []v1.Taint{
				{Key: "a", Value: "b", Effect: v1.TaintEffectNoSchedule},
				{Key: "c", Value: "d", Effect: v1.TaintEffectNoExecute},
				{Key: "e", Value: "f", Effect: v1.TaintEffectPreferNoSchedule},
				{Key: "key-only", Effect: v1.TaintEffectNoExecute},
			}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should fail for invalid taint keys", func() {
			provisioner.Spec.Taints = []v1.Taint{{Key: "???"}}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail for missing taint key", func() {
			provisioner.Spec.Taints = []v1.Taint{{Effect: v1.TaintEffectNoSchedule}}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail for invalid taint value", func() {
			provisioner.Spec.Taints = []v1.Taint{{Key: "invalid-value", Effect: v1.TaintEffectNoSchedule, Value: "???"}}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail for invalid taint effect", func() {
			provisioner.Spec.Taints = []v1.Taint{{Key: "invalid-effect", Effect: "???"}}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
	})
	Context("Requirements", func() {
		It("should allow supported ops", func() {
			provisioner.Spec.Requirements = NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"bar"}},
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpExists, Values: []string{}},
				v1.NodeSelectorRequirement{Key: v1.LabelOSStable, Operator: v1.NodeSelectorOpDoesNotExist, Values: []string{}},
			)
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should fail for unsupported ops", func() {
			for _, op := range []v1.NodeSelectorOperator{v1.NodeSelectorOpGt, v1.NodeSelectorOpLt} {
				provisioner.Spec.Requirements = NewRequirements(
					v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: op, Values: []string{"test"}},
				)
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
		})
		It("should allow well known labels", func() {
			for label := range WellKnownLabels {
				provisioner.Spec.Requirements = NewRequirements(
					v1.NodeSelectorRequirement{Key: label, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
				)
				Expect(provisioner.Validate(ctx)).To(Succeed())
			}
		})
		It("should fail for unknown labels", func() {
			for label := range sets.NewString("unknown", "invalid", "rejected") {
				provisioner.Spec.Requirements = NewRequirements(
					v1.NodeSelectorRequirement{Key: label, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
				)
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
		})
		It("should fail because no feasible value", func() {
			provisioner.Spec.Requirements = NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"bar"}},
			)
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail because In and NotIn cancel out", func() {
			provisioner.Spec.Requirements = NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test"}},
			)
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should allow empty requirements", func() {
			provisioner.Spec.Requirements = NewRequirements()
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should fail because In and DoesNotExists conflicting", func() {
			provisioner.Spec.Requirements = NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpDoesNotExist, Values: []string{}},
			)
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should normalize aliased labels", func() {
			provisioner.Spec.Requirements = NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelFailureDomainBetaZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
			)
			Expect(provisioner.Spec.Requirements.Keys()).To(Equal([]string{v1.LabelTopologyZone}))
		})
		It("A should be compatible to B, <In, In> operator", func() {
			A := NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test", "foo"}},
			)
			B := NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"foo"}},
			)
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <In, In> operator", func() {
			A := NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test", "foo"}},
			)
			B := NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"bar"}},
			)
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <In, NotIn> operator", func() {
			A := NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test", "foo"}},
			)
			B := NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"foo"}},
			)
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <Empty, NotIn> operator", func() {
			A := NewRequirements()
			B := NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyRegion, Operator: v1.NodeSelectorOpNotIn, Values: []string{"foo"}},
			)
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <Empty, In> operator", func() {
			A := NewRequirements()
			B := NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyRegion, Operator: v1.NodeSelectorOpIn, Values: []string{"foo"}},
			)
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should fail to be compatible to B, <In, Exist> operator", func() {
			A := NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test", "foo"}},
			)
			B := NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyRegion, Operator: v1.NodeSelectorOpExists, Values: []string{}},
			)
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <In, DoesNotExist> operator", func() {
			A := NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test", "foo"}},
			)
			B := NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyRegion, Operator: v1.NodeSelectorOpDoesNotExist, Values: []string{}},
			)
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <In, DoesNotExist> operator", func() {
			A := NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test", "foo"}},
			)
			B := NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpDoesNotExist, Values: []string{}},
			)
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
	})
})
