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
	"time"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/ptr"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Validation", func() {
	Context("NodePool", func() {
		It("should error when a restricted label is used in labels (karpenter.sh/nodepool)", func() {
			nodePool.Spec.Template.Labels = map[string]string{
				corev1beta1.NodePoolLabelKey: "my-custom-nodepool",
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
				v1.LabelNamespaceNodeRestriction + "/custom-label": "custom-value",
			}
			Expect(env.Client.Create(env.Context, nodePool)).To(Succeed())
		})
		It("should allow a restricted label exception to be used in labels ([*].node-restriction.kubernetes.io/custom-label)", func() {
			nodePool.Spec.Template.Labels = map[string]string{
				"subdomain" + v1.LabelNamespaceNodeRestriction + "/custom-label": "custom-value",
			}
			Expect(env.Client.Create(env.Context, nodePool)).To(Succeed())
		})
		It("should error when a requirement references a restricted label (karpenter.sh/nodepool)", func() {
			nodePool = coretest.ReplaceRequirements(nodePool, corev1beta1.NodeSelectorRequirementWithFlexibility{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      corev1beta1.NodePoolLabelKey,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"default"},
				}})
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
		})
		It("should error when a requirement uses In but has no values", func() {
			nodePool = coretest.ReplaceRequirements(nodePool, corev1beta1.NodeSelectorRequirementWithFlexibility{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceTypeStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{},
				}})
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
		})
		It("should error when a requirement uses an unknown operator", func() {
			nodePool = coretest.ReplaceRequirements(nodePool, corev1beta1.NodeSelectorRequirementWithFlexibility{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      corev1beta1.CapacityTypeLabelKey,
					Operator: "within",
					Values:   []string{corev1beta1.CapacityTypeSpot},
				}})
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
		})
		It("should error when Gt is used with multiple integer values", func() {
			nodePool = coretest.ReplaceRequirements(nodePool, corev1beta1.NodeSelectorRequirementWithFlexibility{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1beta1.LabelInstanceMemory,
					Operator: v1.NodeSelectorOpGt,
					Values:   []string{"1000000", "2000000"},
				}})
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
		})
		It("should error when Lt is used with multiple integer values", func() {
			nodePool = coretest.ReplaceRequirements(nodePool, corev1beta1.NodeSelectorRequirementWithFlexibility{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1beta1.LabelInstanceMemory,
					Operator: v1.NodeSelectorOpLt,
					Values:   []string{"1000000", "2000000"},
				}})
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
		})
		It("should error when ttlSecondAfterEmpty is negative", func() {
			nodePool.Spec.Disruption.ConsolidationPolicy = corev1beta1.ConsolidationPolicyWhenEmpty
			nodePool.Spec.Disruption.ConsolidateAfter = &corev1beta1.NillableDuration{Duration: lo.ToPtr(-time.Second)}
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
		})
		It("should error when ConsolidationPolicy=WhenUnderutilized is used with consolidateAfter", func() {
			nodePool.Spec.Disruption.ConsolidationPolicy = corev1beta1.ConsolidationPolicyWhenUnderutilized
			nodePool.Spec.Disruption.ConsolidateAfter = &corev1beta1.NillableDuration{Duration: lo.ToPtr(time.Minute)}
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
		})
		It("should error if imageGCHighThresholdPercent is less than imageGCLowThresholdPercent", func() {
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				ImageGCHighThresholdPercent: ptr.Int32(10),
				ImageGCLowThresholdPercent:  ptr.Int32(60),
			}
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
		})
		It("should error if imageGCHighThresholdPercent or imageGCLowThresholdPercent is negative", func() {
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				ImageGCHighThresholdPercent: ptr.Int32(-10),
			}
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				ImageGCLowThresholdPercent: ptr.Int32(-10),
			}
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
		})
		It("should error when minValues for a requirement key is negative", func() {
			nodePool = coretest.ReplaceRequirements(nodePool, corev1beta1.NodeSelectorRequirementWithFlexibility{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceTypeStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"c4.large", "c4.xlarge"},
				},
				MinValues: lo.ToPtr(-1)},
			)
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
		})
		It("should error when minValues for a requirement key is more than 100", func() {
			nodePool = coretest.ReplaceRequirements(nodePool, corev1beta1.NodeSelectorRequirementWithFlexibility{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceTypeStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"c4.large", "c4.xlarge"},
				},
				MinValues: lo.ToPtr(101)},
			)
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
		})
		It("should error when minValues for a requirement key is greater than the values specified within In operator", func() {
			nodePool = coretest.ReplaceRequirements(nodePool, corev1beta1.NodeSelectorRequirementWithFlexibility{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceTypeStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"c4.large", "c4.xlarge"},
				},
				MinValues: lo.ToPtr(3)},
			)
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
		})
	})
	Context("EC2NodeClass", func() {
		It("should error when amiSelectorTerms are not defined for amiFamily Custom", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
		})
		It("should fail for poorly formatted AMI ids", func() {
			nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					ID: "must-start-with-ami",
				},
			}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
		})
		It("should succeed when tags don't contain restricted keys", func() {
			nodeClass.Spec.Tags = map[string]string{"karpenter.sh/custom-key": "custom-value", "kubernetes.io/role/key": "custom-value"}
			Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())
		})
		It("should error when tags contains a restricted key", func() {
			nodeClass.Spec.Tags = map[string]string{"karpenter.sh/nodepool": "custom-value"}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())

			nodeClass.Spec.Tags = map[string]string{"karpenter.sh/managed-by": env.ClusterName}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())

			nodeClass.Spec.Tags = map[string]string{fmt.Sprintf("kubernetes.io/cluster/%s", env.ClusterName): "owned"}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
		})
		It("should fail when securityGroupSelectorTerms has id and other filters", func() {
			nodeClass.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{"karpenter.sh/discovery": env.ClusterName},
					ID:   "sg-12345",
				},
			}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
		})
		It("should fail when subnetSelectorTerms has id and other filters", func() {
			nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					Tags: map[string]string{"karpenter.sh/discovery": env.ClusterName},
					ID:   "subnet-12345",
				},
			}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
		})
		It("should fail when amiSelectorTerms has id and other filters", func() {
			nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					Tags: map[string]string{"karpenter.sh/discovery": env.ClusterName},
					ID:   "ami-12345",
				},
			}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
		})
		It("should fail when specifying role and instanceProfile at the same time", func() {
			nodeClass.Spec.Role = "test-role"
			nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
		})
		It("should fail when specifying none of role and instanceProfile", func() {
			nodeClass.Spec.Role = ""
			nodeClass.Spec.InstanceProfile = nil
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
		})
		It("should fail to switch between an unmanaged and managed instance profile", func() {
			nodeClass.Spec.Role = ""
			nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())

			nodeClass.Spec.Role = "test-role"
			nodeClass.Spec.InstanceProfile = nil
			Expect(env.Client.Update(env.Context, nodeClass)).ToNot(Succeed())
		})
		It("should fail to switch between a managed and unmanaged instance profile", func() {
			nodeClass.Spec.Role = "test-role"
			nodeClass.Spec.InstanceProfile = nil
			Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())

			nodeClass.Spec.Role = ""
			nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(env.Client.Update(env.Context, nodeClass)).ToNot(Succeed())
		})
	})
})
