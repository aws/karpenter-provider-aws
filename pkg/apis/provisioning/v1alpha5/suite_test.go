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
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/Pallinder/go-randomdata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/ptr"

	v1 "k8s.io/api/core/v1"
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
	It("should succeed on a valid empty ttl", func() {
		provisioner.Spec.TTLSecondsAfterEmpty = aws.Int64(30)
		Expect(provisioner.Validate(ctx)).To(Succeed())
	})
	It("should fail if both consolidation and TTLSecondsAfterEmpty are enabled", func() {
		provisioner.Spec.TTLSecondsAfterEmpty = ptr.Int64(30)
		provisioner.Spec.Consolidation = &Consolidation{Enabled: aws.Bool(true)}
		Expect(provisioner.Validate(ctx)).ToNot(Succeed())
	})
	It("should succeed if consolidation is off and TTLSecondsAfterEmpty is set", func() {
		provisioner.Spec.TTLSecondsAfterEmpty = ptr.Int64(30)
		provisioner.Spec.Consolidation = &Consolidation{Enabled: aws.Bool(false)}
		Expect(provisioner.Validate(ctx)).To(Succeed())
	})
	It("should succeed if consolidation is on and TTLSecondsAfterEmpty is not set", func() {
		provisioner.Spec.TTLSecondsAfterEmpty = nil
		provisioner.Spec.Consolidation = &Consolidation{Enabled: aws.Bool(true)}
		Expect(provisioner.Validate(ctx)).To(Succeed())
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
	Context("Provider", func() {
		It("should not allow provider and providerRef", func() {
			provisioner.Spec.Provider = &Provider{}
			provisioner.Spec.ProviderRef = &ProviderRef{Name: "providerRef"}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
	})
	Context("Labels", func() {
		It("should allow unrecognized labels", func() {
			provisioner.Spec.Labels = map[string]string{"foo": randomdata.SillyName()}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should fail for the provisioner name label", func() {
			provisioner.Spec.Labels = map[string]string{ProvisionerNameLabelKey: randomdata.SillyName()}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
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
	Context("Requirements", func() {
		It("should fail for the provisioner name label", func() {
			provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
				{Key: ProvisionerNameLabelKey, Operator: v1.NodeSelectorOpIn, Values: []string{randomdata.SillyName()}},
			}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should allow supported ops", func() {
			provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpGt, Values: []string{"1"}},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpLt, Values: []string{"1"}},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpExists},
			}
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should fail for unsupported ops", func() {
			for _, op := range []v1.NodeSelectorOperator{"unknown"} {
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
			for label := range WellKnownLabels.Difference(sets.NewString(ProvisionerNameLabelKey)) {
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
		It("should fail with invalid GT or LT values", func() {
			for _, requirement := range []v1.NodeSelectorRequirement{
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpGt, Values: []string{}},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpGt, Values: []string{"1", "2"}},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpGt, Values: []string{"a"}},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpGt, Values: []string{"-1"}},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpLt, Values: []string{}},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpLt, Values: []string{"1", "2"}},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpLt, Values: []string{"a"}},
				{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpLt, Values: []string{"-1"}},
			} {
				provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{requirement}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
		})
	})
	Context("KubeletConfiguration", func() {
		It("should fail on kubeReserved with invalid keys", func() {
			provisioner.Spec.KubeletConfiguration = &KubeletConfiguration{
				KubeReserved: v1.ResourceList{
					v1.ResourcePods: resource.MustParse("2"),
				},
			}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail on systemReserved with invalid keys", func() {
			provisioner.Spec.KubeletConfiguration = &KubeletConfiguration{
				SystemReserved: v1.ResourceList{
					v1.ResourcePods: resource.MustParse("2"),
				},
			}
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		Context("Eviction Signals", func() {
			Context("Eviction Hard", func() {
				It("should succeed on evictionHard with valid keys", func() {
					provisioner.Spec.KubeletConfiguration = &KubeletConfiguration{
						EvictionHard: map[string]string{
							"memory.available":   "5%",
							"nodefs.available":   "10%",
							"nodefs.inodesFree":  "15%",
							"imagefs.available":  "5%",
							"imagefs.inodesFree": "5%",
							"pid.available":      "5%",
						},
					}
					Expect(provisioner.Validate(ctx)).To(Succeed())
				})
				It("should fail on evictionHard with invalid keys", func() {
					provisioner.Spec.KubeletConfiguration = &KubeletConfiguration{
						EvictionHard: map[string]string{
							"memory": "5%",
						},
					}
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
				It("should fail on invalid formatted percentage value in evictionHard", func() {
					provisioner.Spec.KubeletConfiguration = &KubeletConfiguration{
						EvictionHard: map[string]string{
							"memory.available": "5%3",
						},
					}
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
				It("should fail on invalid percentage value (too large) in evictionHard", func() {
					provisioner.Spec.KubeletConfiguration = &KubeletConfiguration{
						EvictionHard: map[string]string{
							"memory.available": "110%",
						},
					}
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
				It("should fail on invalid quantity value in evictionHard", func() {
					provisioner.Spec.KubeletConfiguration = &KubeletConfiguration{
						EvictionHard: map[string]string{
							"memory.available": "110GB",
						},
					}
					Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				})
			})
		})
		Context("Eviction Soft", func() {
			It("should succeed on evictionSoft with valid keys", func() {
				provisioner.Spec.KubeletConfiguration = &KubeletConfiguration{
					EvictionSoft: map[string]string{
						"memory.available":   "5%",
						"nodefs.available":   "10%",
						"nodefs.inodesFree":  "15%",
						"imagefs.available":  "5%",
						"imagefs.inodesFree": "5%",
						"pid.available":      "5%",
					},
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory.available":   {Duration: time.Minute},
						"nodefs.available":   {Duration: time.Second * 90},
						"nodefs.inodesFree":  {Duration: time.Minute * 5},
						"imagefs.available":  {Duration: time.Hour},
						"imagefs.inodesFree": {Duration: time.Hour * 24},
						"pid.available":      {Duration: time.Minute},
					},
				}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			It("should fail on evictionSoft with invalid keys", func() {
				provisioner.Spec.KubeletConfiguration = &KubeletConfiguration{
					EvictionSoft: map[string]string{
						"memory": "5%",
					},
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory": {Duration: time.Minute},
					},
				}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should fail on invalid formatted percentage value in evictionSoft", func() {
				provisioner.Spec.KubeletConfiguration = &KubeletConfiguration{
					EvictionSoft: map[string]string{
						"memory.available": "5%3",
					},
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory.available": {Duration: time.Minute},
					},
				}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should fail on invalid percentage value (too large) in evictionSoft", func() {
				provisioner.Spec.KubeletConfiguration = &KubeletConfiguration{
					EvictionSoft: map[string]string{
						"memory.available": "110%",
					},
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory.available": {Duration: time.Minute},
					},
				}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should fail on invalid quantity value in evictionSoft", func() {
				provisioner.Spec.KubeletConfiguration = &KubeletConfiguration{
					EvictionSoft: map[string]string{
						"memory.available": "110GB",
					},
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory.available": {Duration: time.Minute},
					},
				}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should fail when eviction soft doesn't have matching grace period", func() {
				provisioner.Spec.KubeletConfiguration = &KubeletConfiguration{
					EvictionSoft: map[string]string{
						"memory.available": "200Mi",
					},
				}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
		})
		Context("Eviction Soft Grace Period", func() {
			It("should succeed on evictionSoftGracePeriod with valid keys", func() {
				provisioner.Spec.KubeletConfiguration = &KubeletConfiguration{
					EvictionSoft: map[string]string{
						"memory.available":   "5%",
						"nodefs.available":   "10%",
						"nodefs.inodesFree":  "15%",
						"imagefs.available":  "5%",
						"imagefs.inodesFree": "5%",
						"pid.available":      "5%",
					},
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory.available":   {Duration: time.Minute},
						"nodefs.available":   {Duration: time.Second * 90},
						"nodefs.inodesFree":  {Duration: time.Minute * 5},
						"imagefs.available":  {Duration: time.Hour},
						"imagefs.inodesFree": {Duration: time.Hour * 24},
						"pid.available":      {Duration: time.Minute},
					},
				}
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			It("should fail on evictionSoftGracePeriod with invalid keys", func() {
				provisioner.Spec.KubeletConfiguration = &KubeletConfiguration{
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory": {Duration: time.Minute},
					},
				}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should fail when eviction soft grace period doesn't have matching threshold", func() {
				provisioner.Spec.KubeletConfiguration = &KubeletConfiguration{
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory.available": {Duration: time.Minute},
					},
				}
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
		})
	})
})
