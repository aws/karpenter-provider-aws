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
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	awstest "github.com/aws/karpenter/pkg/test"
)

var _ = Describe("Webhooks", func() {
	Context("Defaulting Webhook", func() {
		It("should set the default requirements when none are specified", func() {
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			}})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			})
			env.ExpectCreated(provisioner)
			env.ExpectFound(provisioner)

			Expect(len(provisioner.Spec.Requirements)).To(Equal(2))
			Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
				Key:      v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.CapacityTypeOnDemand},
			}))
			Expect(provisioner.Spec.Requirements).To(ContainElement(v1.NodeSelectorRequirement{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"amd64"},
			}))
		})
	})
	Context("Validating Webhook", func() {
		It("should deny the request when provider and providerRef are combined", func() {
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			}})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				Provider: v1alpha1.AWS{
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
				},
				ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			})
			Expect(env.Client.Create(env, provisioner)).ToNot(Succeed())
		})
		It("should deny the request when a restricted label is used in labels (karpenter.sh/provisioner-name)", func() {
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			}})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: "my-custom-provisioner",
				},
			})
			Expect(env.Client.Create(env, provisioner)).ToNot(Succeed())
		})
		It("should deny the request when a restricted label is used in labels (kubernetes.io/custom-label)", func() {
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			}})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
				Labels: map[string]string{
					"kubernetes.io/custom-label": "custom-value",
				},
			})
			Expect(env.Client.Create(env, provisioner)).ToNot(Succeed())
		})
		It("should deny the request when a requirement references a restricted label (karpenter.sh/provisioner-name)", func() {
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			}})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1alpha5.ProvisionerNameLabelKey,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"default"},
					},
				},
			})
			Expect(env.Client.Create(env, provisioner)).ToNot(Succeed())
		})
		It("should deny the request when a requirement uses In but has no values", func() {
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			}})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1.LabelInstanceTypeStable,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{},
					},
				},
			})
			Expect(env.Client.Create(env, provisioner)).ToNot(Succeed())
		})
		It("should deny the request when a requirement uses an unknown operator", func() {
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			}})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1alpha5.LabelCapacityType,
						Operator: "within",
						Values:   []string{v1alpha5.CapacityTypeSpot},
					},
				},
			})
			Expect(env.Client.Create(env, provisioner)).ToNot(Succeed())
		})
		It("should deny the request when Gt is used with multiple integer values", func() {
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			}})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1alpha1.LabelInstanceMemory,
						Operator: v1.NodeSelectorOpGt,
						Values:   []string{"1000000", "2000000"},
					},
				},
			})
			Expect(env.Client.Create(env, provisioner)).ToNot(Succeed())
		})
		It("should deny the request when Lt is used with multiple integer values", func() {
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			}})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1alpha1.LabelInstanceMemory,
						Operator: v1.NodeSelectorOpLt,
						Values:   []string{"1000000", "2000000"},
					},
				},
			})
			Expect(env.Client.Create(env, provisioner)).ToNot(Succeed())
		})
		It("should deny the request when consolidation and ttlSecondAfterEmpty are combined", func() {
			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			}})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
				Consolidation: &v1alpha5.Consolidation{
					Enabled: ptr.Bool(true),
				},
				TTLSecondsAfterEmpty: ptr.Int64(60),
			})
			Expect(env.Client.Create(env, provisioner)).ToNot(Succeed())
		})
	})
})
