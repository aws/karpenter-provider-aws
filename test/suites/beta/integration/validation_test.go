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
	"time"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
)

var _ = Describe("Validation", func() {
	Context("NodePool", func() {
		It("should error when a restricted label is used in labels (karpenter.sh/nodepool)", func() {
			nodePool.Spec.Template.Labels = map[string]string{
				corev1beta1.NodePoolLabelKey: "my-custom-provisioner",
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
		It("should error when a requirement references a restricted label (karpenter.sh/provisioner-name)", func() {
			nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, []v1.NodeSelectorRequirement{
				{
					Key:      corev1beta1.NodePoolLabelKey,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"default"},
				},
			}...)
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
		})
		It("should error when a requirement uses In but has no values", func() {
			nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, []v1.NodeSelectorRequirement{
				{
					Key:      v1.LabelInstanceTypeStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{},
				},
			}...)
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
		})
		It("should error when a requirement uses an unknown operator", func() {
			nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha5.LabelCapacityType,
					Operator: "within",
					Values:   []string{v1alpha5.CapacityTypeSpot},
				},
			}...)
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
		})
		It("should error when Gt is used with multiple integer values", func() {
			nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceMemory,
					Operator: v1.NodeSelectorOpGt,
					Values:   []string{"1000000", "2000000"},
				},
			}...)
			Expect(env.Client.Create(env.Context, nodePool)).ToNot(Succeed())
		})
		It("should error when Lt is used with multiple integer values", func() {
			nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceMemory,
					Operator: v1.NodeSelectorOpLt,
					Values:   []string{"1000000", "2000000"},
				},
			}...)
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
	})
	//Context("EC2NodeClass", func() {
	//	It("should error when amiSelectorTerms are not defined for amiFamily Custom", func() {
	//		nodeClass.Spec.AMIFamily = &v1alpha1.AMIFamilyCustom
	//		Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
	//	})
	//	It("should fail for poorly formatted AMI ids", func() {
	//		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
	//			{
	//				ID: "must-start-with-ami",
	//			},
	//		}
	//		Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
	//	})
	//	It("should succeed when tags don't contain restricted keys", func() {
	//		nodeClass.Spec.Tags = map[string]string{"karpenter.sh/custom-key": "custom-value", "kubernetes.io/role/key": "custom-value"}
	//		Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())
	//	})
	//	DescribeTable("should error when tags contains a restricted key",
	//		func(tags map[string]string) {
	//			nodeClass.Spec.Tags = tags
	//			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
	//		},
	//		Entry("karpenter.sh/provisioner-name", map[string]string{"karpenter.sh/provisioner-name": "custom-value"}),
	//		Entry("karpenter.sh/managed-by", map[string]string{"karpenter.sh/managed-by": env.ClusterName}),
	//		Entry("kubernetes.io/cluster/*", map[string]string{fmt.Sprintf("kubernetes.io/cluster/%s", env.ClusterName): "owned"}),
	//	)
	//	It("should fail when securityGroupSelector has id and other filters", func() {
	//		nodeClass.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
	//			{
	//				Tags: map[string]string{"karpenter.sh/discovery": env.ClusterName},
	//				ID:   "sg-12345",
	//			},
	//		}
	//		Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
	//	})
	//	It("should fail when subnetSelector has id and other filters", func() {
	//		nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
	//			{
	//				Tags: map[string]string{"karpenter.sh/discovery": env.ClusterName},
	//				ID:   "subnet-12345",
	//			},
	//		}
	//		Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
	//	})
	//	It("should fail when amiSelector has id and other filters", func() {
	//		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
	//			{
	//				Tags: map[string]string{"karpenter.sh/discovery": env.ClusterName},
	//				ID:   "ami-12345",
	//			},
	//		}
	//		Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
	//	})
	//})
})
