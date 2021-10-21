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

package v1alpha4

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			Spec: ProvisionerSpec{},
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
			for _, label := range RestrictedLabels {
				provisioner.Spec.Labels = map[string]string{label: randomdata.SillyName()}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
		})
		It("should succeed for well known label values", func() {
			WellKnownLabels[v1.LabelTopologyZone] = []string{"test-1", "test1"}
			WellKnownLabels[v1.LabelInstanceTypeStable] = []string{"test-1", "test1"}
			WellKnownLabels[v1.LabelArchStable] = []string{"test-1", "test1"}
			WellKnownLabels[v1.LabelOSStable] = []string{"test-1", "test1"}
			for key, values := range WellKnownLabels {
				for _, value := range values {
					provisioner.Spec.Labels = map[string]string{key: value}
					Expect(provisioner.Validate(ctx)).To(Succeed())
				}
			}
		})
		It("should fail for invalid well known label values", func() {
			for key := range WellKnownLabels {
				provisioner.Spec.Labels = map[string]string{key: "unknown"}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
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
			provisioner.Spec.Requirements = Requirements{
				{Key: "test", Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
				{Key: "test", Operator: v1.NodeSelectorOpNotIn, Values: []string{"bar"}},
			}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should fail for unsupported ops", func() {
			for _, op := range []v1.NodeSelectorOperator{v1.NodeSelectorOpExists, v1.NodeSelectorOpDoesNotExist, v1.NodeSelectorOpGt, v1.NodeSelectorOpLt} {
				provisioner.Spec.Requirements = Requirements{{Key: "test", Operator: op, Values: []string{"test"}}}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
		})
		It("should validate well known labels", func() {
			WellKnownLabels[v1.LabelTopologyZone] = []string{"test"}
			provisioner.Spec.Requirements = Requirements{{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}}}
			Expect(provisioner.Validate(ctx)).To(Succeed())
			provisioner.Spec.Labels = map[string]string{}
			provisioner.Spec.Requirements = Requirements{{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}}}
			Expect(provisioner.Validate(ctx)).To(Succeed())
			provisioner.Spec.Labels = map[string]string{v1.LabelTopologyZone: "test"}
			provisioner.Spec.Requirements = Requirements{{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}}}
			Expect(provisioner.Validate(ctx)).To(Succeed())
			provisioner.Spec.Labels = map[string]string{v1.LabelTopologyZone: "test"}
			provisioner.Spec.Requirements = Requirements{{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test"}}}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			provisioner.Spec.Labels = map[string]string{v1.LabelTopologyZone: "test"}
			provisioner.Spec.Requirements = Requirements{{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"unknown"}}}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should validate custom labels", func() {
			provisioner.Spec.Requirements = Requirements{{Key: "test", Operator: v1.NodeSelectorOpIn, Values: []string{"test"}}}
			Expect(provisioner.Validate(ctx)).To(Succeed())
			provisioner.Spec.Labels = map[string]string{}
			provisioner.Spec.Requirements = Requirements{{Key: "test", Operator: v1.NodeSelectorOpIn, Values: []string{"test"}}}
			Expect(provisioner.Validate(ctx)).To(Succeed())
			provisioner.Spec.Labels = map[string]string{"test": "test"}
			provisioner.Spec.Requirements = Requirements{{Key: "test", Operator: v1.NodeSelectorOpIn, Values: []string{"test"}}}
			Expect(provisioner.Validate(ctx)).To(Succeed())
			provisioner.Spec.Labels = map[string]string{"test": "test"}
			provisioner.Spec.Requirements = Requirements{{Key: "test", Operator: v1.NodeSelectorOpNotIn, Values: []string{"test"}}}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			provisioner.Spec.Labels = map[string]string{"test": "test"}
			provisioner.Spec.Requirements = Requirements{{Key: "test", Operator: v1.NodeSelectorOpIn, Values: []string{"unknown"}}}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
	})
})
