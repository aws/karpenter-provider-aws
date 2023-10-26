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

package integration_test

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awstest "github.com/aws/karpenter/pkg/test"
)

var _ = Describe("Webhooks", func() {
	Context("Provisioner", func() {
		Context("Defaulting", func() {
			It("should set the default requirements when none are specified", func() {
				provisioner := test.Provisioner(test.ProvisionerOptions{
					ProviderRef: &v1alpha5.MachineTemplateRef{Name: "test"},
				})
				env.ExpectCreated(provisioner)
				env.ExpectExists(provisioner)

				Expect(len(provisioner.Spec.Requirements)).To(Equal(5))
				Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
					Key:      v1.LabelOSStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{string(v1.Linux)},
				}))
				Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
					Key:      v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha5.CapacityTypeOnDemand},
				}))
				Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
					Key:      v1.LabelArchStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha5.ArchitectureAmd64},
				}))
				Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"c", "m", "r"},
				}))
				Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
					Key:      v1alpha1.LabelInstanceGeneration,
					Operator: v1.NodeSelectorOpGt,
					Values:   []string{"2"},
				}))
			})
			It("shouldn't default if requirements are set", func() {
				provisioner := test.Provisioner(test.ProvisionerOptions{
					ProviderRef: &v1alpha5.MachineTemplateRef{Name: "test"},
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelOSStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{string(v1.Windows)},
						},
						{
							Key:      v1alpha5.LabelCapacityType,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{v1alpha5.CapacityTypeSpot},
						},
						{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{v1alpha5.ArchitectureArm64},
						},
						{
							Key:      v1alpha1.LabelInstanceCategory,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"g"},
						},
						{
							Key:      v1alpha1.LabelInstanceGeneration,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"4"},
						},
					},
				})
				env.ExpectCreated(provisioner)
				env.ExpectExists(provisioner)

				Expect(len(provisioner.Spec.Requirements)).To(Equal(5))
				Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
					Key:      v1.LabelOSStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{string(v1.Windows)},
				}))
				Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
					Key:      v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha5.CapacityTypeSpot},
				}))
				Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
					Key:      v1.LabelArchStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha5.ArchitectureArm64},
				}))
				Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
					Key:      v1alpha1.LabelInstanceCategory,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"g"},
				}))
				Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
					Key:      v1alpha1.LabelInstanceGeneration,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"4"},
				}))
			})
		})
		Context("Validation", func() {
			It("should error when provider and providerRef are combined", func() {
				Expect(env.Client.Create(env, test.Provisioner(test.ProvisionerOptions{
					Provider: v1alpha1.AWS{
						SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
						SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
					},
					ProviderRef: &v1alpha5.MachineTemplateRef{Name: "test"},
				}))).ToNot(Succeed())
			})
			It("should error when a restricted label is used in labels (karpenter.sh/provisioner-name)", func() {
				Expect(env.Client.Create(env, test.Provisioner(test.ProvisionerOptions{
					ProviderRef: &v1alpha5.MachineTemplateRef{Name: "test"},
					Labels: map[string]string{
						v1alpha5.ProvisionerNameLabelKey: "my-custom-provisioner",
					},
				}))).ToNot(Succeed())
			})
			It("should error when a restricted label is used in labels (kubernetes.io/custom-label)", func() {
				Expect(env.Client.Create(env, test.Provisioner(test.ProvisionerOptions{
					ProviderRef: &v1alpha5.MachineTemplateRef{Name: "test"},
					Labels: map[string]string{
						"kubernetes.io/custom-label": "custom-value",
					},
				}))).ToNot(Succeed())
			})
			It("should allow a restricted label exception to be used in labels (node-restriction.kubernetes.io/custom-label)", func() {
				Expect(env.Client.Create(env, test.Provisioner(test.ProvisionerOptions{
					ProviderRef: &v1alpha5.MachineTemplateRef{Name: "test"},
					Labels: map[string]string{
						v1.LabelNamespaceNodeRestriction + "/custom-label": "custom-value",
					},
				}))).To(Succeed())
			})
			It("should error when a requirement references a restricted label (karpenter.sh/provisioner-name)", func() {
				Expect(env.Client.Create(env, test.Provisioner(test.ProvisionerOptions{
					ProviderRef: &v1alpha5.MachineTemplateRef{Name: "test"},
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1alpha5.ProvisionerNameLabelKey,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"default"},
						},
					},
				}))).ToNot(Succeed())
			})
			It("should error when a requirement uses In but has no values", func() {
				Expect(env.Client.Create(env, test.Provisioner(test.ProvisionerOptions{
					ProviderRef: &v1alpha5.MachineTemplateRef{Name: "test"},
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelInstanceTypeStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{},
						},
					},
				}))).ToNot(Succeed())
			})
			It("should error when a requirement uses an unknown operator", func() {
				Expect(env.Client.Create(env, test.Provisioner(test.ProvisionerOptions{
					ProviderRef: &v1alpha5.MachineTemplateRef{Name: "test"},
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1alpha5.LabelCapacityType,
							Operator: "within",
							Values:   []string{v1alpha5.CapacityTypeSpot},
						},
					},
				}))).ToNot(Succeed())
			})
			It("should error when Gt is used with multiple integer values", func() {
				Expect(env.Client.Create(env, test.Provisioner(test.ProvisionerOptions{
					ProviderRef: &v1alpha5.MachineTemplateRef{Name: "test"},
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1alpha1.LabelInstanceMemory,
							Operator: v1.NodeSelectorOpGt,
							Values:   []string{"1000000", "2000000"},
						},
					},
				}))).ToNot(Succeed())
			})
			It("should error when Lt is used with multiple integer values", func() {
				Expect(env.Client.Create(env, test.Provisioner(test.ProvisionerOptions{
					ProviderRef: &v1alpha5.MachineTemplateRef{Name: "test"},
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1alpha1.LabelInstanceMemory,
							Operator: v1.NodeSelectorOpLt,
							Values:   []string{"1000000", "2000000"},
						},
					},
				}))).ToNot(Succeed())
			})
			It("should error when ttlSecondAfterEmpty is negative", func() {
				Expect(env.Client.Create(env, test.Provisioner(test.ProvisionerOptions{
					ProviderRef:          &v1alpha5.MachineTemplateRef{Name: "test"},
					TTLSecondsAfterEmpty: ptr.Int64(-5),
				}))).ToNot(Succeed())
			})
			It("should error when consolidation and ttlSecondAfterEmpty are combined", func() {
				Expect(env.Client.Create(env, test.Provisioner(test.ProvisionerOptions{
					ProviderRef:          &v1alpha5.MachineTemplateRef{Name: "test"},
					Consolidation:        &v1alpha5.Consolidation{Enabled: ptr.Bool(true)},
					TTLSecondsAfterEmpty: ptr.Int64(60),
				}))).ToNot(Succeed())
			})
			It("should error if imageGCHighThresholdPercent is less than imageGCLowThresholdPercent", func() {
				Expect(env.Client.Create(env, test.Provisioner(test.ProvisionerOptions{
					ProviderRef: &v1alpha5.MachineTemplateRef{Name: "test"},
					Kubelet: &v1alpha5.KubeletConfiguration{
						ImageGCHighThresholdPercent: ptr.Int32(10),
						ImageGCLowThresholdPercent:  ptr.Int32(60),
					},
				}))).ToNot(Succeed())
			})
			It("should error if imageGCHighThresholdPercent or imageGCLowThresholdPercent is negative", func() {
				Expect(env.Client.Create(env, test.Provisioner(test.ProvisionerOptions{
					ProviderRef: &v1alpha5.MachineTemplateRef{Name: "test"},
					Kubelet: &v1alpha5.KubeletConfiguration{
						ImageGCHighThresholdPercent: ptr.Int32(-10),
					},
				}))).ToNot(Succeed())
				Expect(env.Client.Create(env, test.Provisioner(test.ProvisionerOptions{
					ProviderRef: &v1alpha5.MachineTemplateRef{Name: "test"},
					Kubelet: &v1alpha5.KubeletConfiguration{
						ImageGCLowThresholdPercent: ptr.Int32(-10),
					},
				}))).ToNot(Succeed())
			})
		})
	})
	Context("AWSNodeTemplate", func() {
		Context("Validation", func() {
			It("should error when amiSelector is not defined for amiFamily Custom", func() {
				Expect(env.Client.Create(env, awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
					AMIFamily:             &v1alpha1.AMIFamilyCustom,
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
				}}))).ToNot(Succeed())
			})
			It("should fail if both userdata and launchTemplate are set", func() {
				Expect(env.Client.Create(env, awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
					LaunchTemplate:        v1alpha1.LaunchTemplate{LaunchTemplateName: ptr.String("lt")},
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
				},
					UserData: ptr.String("data"),
				}))).ToNot(Succeed())
			})
			It("should fail if both amiSelector and launchTemplate are set", func() {
				Expect(env.Client.Create(env, awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
					LaunchTemplate:        v1alpha1.LaunchTemplate{LaunchTemplateName: ptr.String("lt")},
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
				},
					AMISelector: map[string]string{"foo": "bar"},
				}))).ToNot(Succeed())
			})
			It("should fail for poorly formatted aws-ids", func() {
				Expect(env.Client.Create(env, awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
				},
					AMISelector: map[string]string{"aws-ids": "must-start-with-ami"},
				}))).ToNot(Succeed())
			})
			It("should succeed when tags don't contain restricted keys", func() {
				Expect(env.Client.Create(env, awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
					Tags:                  map[string]string{"karpenter.sh/custom-key": "custom-value", "kubernetes.io/role/key": "custom-value"},
				},
				}))).To(Succeed())
			})
			It("should error when tags contains a restricted key", func() {
				Expect(env.Client.Create(env, awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
					Tags:                  map[string]string{"karpenter.sh/provisioner-name": "custom-value"},
				},
				}))).ToNot(Succeed())

				Expect(env.Client.Create(env, awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
					Tags:                  map[string]string{"karpenter.sh/managed-by": env.ClusterName},
				},
				}))).ToNot(Succeed())

				Expect(env.Client.Create(env, awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
					Tags:                  map[string]string{fmt.Sprintf("kubernetes.io/cluster/%s", env.ClusterName): "owned"},
				},
				}))).ToNot(Succeed())
			})
			It("should fail when securityGroupSelector has id and other filters", func() {
				Expect(env.Client.Create(env, awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
					AWS: v1alpha1.AWS{
						SecurityGroupSelector: map[string]string{
							"karpenter.sh/discovery": env.ClusterName,
							"aws-ids":                "sg-12345",
						},
						SubnetSelector: map[string]string{
							"karpenter.sh/discovery": env.ClusterName,
						},
					},
				}))).ToNot(Succeed())
				Expect(env.Client.Create(env, awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
					AWS: v1alpha1.AWS{
						SecurityGroupSelector: map[string]string{
							"karpenter.sh/discovery": env.ClusterName,
							"aws-ids":                "sg-12345",
						},
						SubnetSelector: map[string]string{
							"karpenter.sh/discovery": env.ClusterName,
						},
					},
				}))).ToNot(Succeed())
			})
			It("should fail when subnetSelector has id and other filters", func() {
				Expect(env.Client.Create(env, awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
					AWS: v1alpha1.AWS{
						SecurityGroupSelector: map[string]string{
							"karpenter.sh/discovery": env.ClusterName,
						},
						SubnetSelector: map[string]string{
							"karpenter.sh/discovery": env.ClusterName,
							"aws-ids":                "subnet-12345",
						},
					},
				}))).ToNot(Succeed())
				Expect(env.Client.Create(env, awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
					AWS: v1alpha1.AWS{
						SecurityGroupSelector: map[string]string{
							"karpenter.sh/discovery": env.ClusterName,
						},
						SubnetSelector: map[string]string{
							"karpenter.sh/discovery": env.ClusterName,
							"aws-ids":                "subnet-12345",
						},
					},
				}))).ToNot(Succeed())
			})
			It("should fail when amiSelector has id and other filters", func() {
				Expect(env.Client.Create(env, awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
					AWS: v1alpha1.AWS{
						SecurityGroupSelector: map[string]string{
							"karpenter.sh/discovery": env.ClusterName,
						},
						SubnetSelector: map[string]string{
							"karpenter.sh/discovery": env.ClusterName,
						},
					},
					AMISelector: map[string]string{
						"foo":     "bar",
						"aws-ids": "ami-12345",
					},
				}))).ToNot(Succeed())
				Expect(env.Client.Create(env, awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
					AWS: v1alpha1.AWS{
						SecurityGroupSelector: map[string]string{
							"karpenter.sh/discovery": env.ClusterName,
						},
						SubnetSelector: map[string]string{
							"karpenter.sh/discovery": env.ClusterName,
						},
					},
					AMISelector: map[string]string{
						"foo":      "bar",
						"aws::ids": "ami-12345",
					},
				}))).ToNot(Succeed())
			})
		})
	})
})
