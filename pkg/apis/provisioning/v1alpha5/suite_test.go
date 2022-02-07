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
	"k8s.io/apimachinery/pkg/util/sets"
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
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName())},
			Spec:       ProvisionerSpec{},
		}
	})

	It("should fail on negative expiry ttl", func() {
		provisioner.Spec.TTLSecondsUntilExpired = ptr.Int64(-1)
		Expect(provisioner.Validate(ctx)).ToNot(Succeed())
	})
	It("should succeed on a missing expiry ttl", func() {
		// this already is true, but to be explicit
		provisioner.Spec.TTLSecondsUntilExpired = nil
		Expect(provisioner.Validate(ctx)).To(Succeed())
	})
	It("should fail on negative empty ttl", func() {
		provisioner.Spec.TTLSecondsAfterEmpty = ptr.Int64(-1)
		Expect(provisioner.Validate(ctx)).ToNot(Succeed())
	})
	It("should succeed on a missing empty ttl", func() {
		provisioner.Spec.TTLSecondsAfterEmpty = nil
		Expect(provisioner.Validate(ctx)).To(Succeed())
	})

	Context("MaintenanceWindows", func() {
		It("should allow supported week days", func() {
			for weekday := range sets.NewString("SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT") {
				provisioner.Spec.MaintenanceWindows = MaintenanceWindows{
					{WeekDays: []string{weekday}, StartTime: "00:00", TimeZone: "America/New_York", Duration: "24:00"},
				}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			}
			provisioner.Spec.MaintenanceWindows = MaintenanceWindows{
				{WeekDays: []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}, StartTime: "00:00", TimeZone: "America/New_York", Duration: "24:00"},
			}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should fail empty week days", func() {
			provisioner.Spec.MaintenanceWindows = MaintenanceWindows{
				{WeekDays: []string{}, StartTime: "00:00", TimeZone: "America/New_York", Duration: "24:00"},
				{WeekDays: nil, StartTime: "00:00", TimeZone: "America/New_York", Duration: "24:00"},
			}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail unsupported week days", func() {
			for weekday := range sets.NewString("MONDAY", "monday", "mon", "m0n") {
				provisioner.Spec.MaintenanceWindows = MaintenanceWindows{
					{WeekDays: []string{weekday}, StartTime: "00:00", TimeZone: "America/New_York", Duration: "24:00"},
				}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
		})
		It("should allow supported start time", func() {
			for startTime := range sets.NewString("00:00", "09:09", "19:50", "23:59") {
				provisioner.Spec.MaintenanceWindows = MaintenanceWindows{
					{WeekDays: []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}, StartTime: startTime, TimeZone: "America/New_York", Duration: "24:00"},
				}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			}
		})
		It("should allow empty start time", func() {
			provisioner.Spec.MaintenanceWindows = MaintenanceWindows{
				{WeekDays: []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}, StartTime: "", TimeZone: "America/New_York", Duration: "24:00"},
			}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should allow unsupported start time", func() {
			for startTime := range sets.NewString("25:00", "24:01", "00:60", "30:00", "000:00", "000:000") {
				provisioner.Spec.MaintenanceWindows = MaintenanceWindows{
					{WeekDays: []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}, StartTime: startTime, TimeZone: "America/New_York", Duration: "24:00"},
				}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
		})
		It("should allow supported timezone", func() {
			for timeZone := range sets.NewString("America/New_York", "US/Central", "GMT") {
				provisioner.Spec.MaintenanceWindows = MaintenanceWindows{
					{WeekDays: []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}, StartTime: "00:00", TimeZone: timeZone, Duration: "24:00"},
				}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			}
		})
		It("should allow empty timezone", func() {
			provisioner.Spec.MaintenanceWindows = MaintenanceWindows{
				{WeekDays: []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}, StartTime: "00:00", TimeZone: "", Duration: "24:00"},
			}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail unsupported timezone", func() {
			for timeZone := range sets.NewString("New_Yor", "Central", "GMTX") {
				provisioner.Spec.MaintenanceWindows = MaintenanceWindows{
					{WeekDays: []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}, StartTime: "00:00", TimeZone: timeZone, Duration: "24:00"},
				}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
		})
		It("should allow supported window", func() {
			provisioner.Spec.MaintenanceWindows = MaintenanceWindows{
				{WeekDays: []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}, StartTime: "00:00", TimeZone: "America/New_York", Duration: "00:00"},
				{WeekDays: []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}, StartTime: "00:00", TimeZone: "America/New_York", Duration: "09:09"},
				{WeekDays: []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}, StartTime: "00:00", TimeZone: "America/New_York", Duration: "19:50"},
				{WeekDays: []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}, StartTime: "00:00", TimeZone: "America/New_York", Duration: "23:59"},
			}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should fail empty duration", func() {
			provisioner.Spec.MaintenanceWindows = MaintenanceWindows{
				{WeekDays: []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}, StartTime: "00:00", TimeZone: "America/New_York", Duration: ""},
			}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail unsupported duration format", func() {
			for duration := range sets.NewString("25:00", "24:01", "00:60", "30:00", "000:00", "00:000", "000:000") {
				provisioner.Spec.MaintenanceWindows = MaintenanceWindows{
					{WeekDays: []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}, StartTime: "00:00", TimeZone: "America/New_York", Duration: duration},
				}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
		})
	})

	Context("Limits", func() {
		It("should allow undefined limits", func() {
			provisioner.Spec.Limits = &Limits{}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should allow empty limits", func() {
			provisioner.Spec.Limits = &Limits{Resources: v1.ResourceList{}}
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
		It("should allow labels in restricted domains exceptions list", func() {
			for label := range LabelDomainExceptions {
				provisioner.Spec.Labels = map[string]string{
					label: "test-value",
				}
				Expect(provisioner.Validate(ctx)).To(Succeed())
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
		It("should not fail for same key with different effects", func() {
			provisioner.Spec.Taints = []v1.Taint{
				{Key: "a", Effect: v1.TaintEffectNoSchedule},
				{Key: "a", Effect: v1.TaintEffectNoExecute},
			}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should fail for duplicate taint key/effect pairs", func() {
			provisioner.Spec.Taints = []v1.Taint{
				{Key: "a", Effect: v1.TaintEffectNoSchedule},
				{Key: "a", Effect: v1.TaintEffectNoSchedule},
			}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			provisioner.Spec.Taints = []v1.Taint{
				{Key: "a", Effect: v1.TaintEffectNoSchedule},
			}
			provisioner.Spec.StartupTaints = []v1.Taint{
				{Key: "a", Effect: v1.TaintEffectNoSchedule},
			}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
	})
	Context("Validation", func() {
		It("should allow supported ops", func() {
			provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpExists},
			}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should fail for unsupported ops", func() {
			for _, op := range []v1.NodeSelectorOperator{v1.NodeSelectorOpDoesNotExist, v1.NodeSelectorOpGt, v1.NodeSelectorOpLt} {
				provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: op, Values: []string{"test"}},
				}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
		})
		It("should fail for restricted domains", func() {
			for label := range RestrictedLabelDomains {
				provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
					{Key: label + "/test", Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
				}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
		})
		It("should allow restricted domains exceptions", func() {
			for label := range LabelDomainExceptions {
				provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
					{Key: label + "/test", Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
				}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			}
		})
		It("should allow well known label exceptions", func() {
			for label := range WellKnownLabels {
				provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
					{Key: label, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
				}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			}
		})
		It("should allow non-empty set after removing overlapped value", func() {
			provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test", "foo"}},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test", "bar"}},
			}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should allow empty requirements", func() {
			provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should fail because DoesNotExists conflicting", func() {
			for _, op := range []v1.NodeSelectorOperator{v1.NodeSelectorOpIn, v1.NodeSelectorOpNotIn, v1.NodeSelectorOpExists} {
				provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: op, Values: []string{"test"}},
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpDoesNotExist},
				}
			}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
	})
})
