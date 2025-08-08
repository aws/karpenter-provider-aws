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

	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Validation", func() {
	Context("EC2NodeClass", func() {
		It("should error when amiSelectorTerms are not defined", func() {
			nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
		})
		It("should fail for poorly formatted AMI ids", func() {
			nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
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

			nodeClass.Spec.Tags = map[string]string{v1.EKSClusterNameTagKey: env.ClusterName}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())

			nodeClass.Spec.Tags = map[string]string{fmt.Sprintf("kubernetes.io/cluster/%s", env.ClusterName): "owned"}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())

			nodeClass.Spec.Tags = map[string]string{"karpenter.sh/nodeclaim": "custom-value"}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())

			nodeClass.Spec.Tags = map[string]string{"karpenter.k8s.aws/ec2nodeclass": "custom-value"}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
		})
		It("should fail when securityGroupSelectorTerms has id and other filters", func() {
			nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{"karpenter.sh/discovery": env.ClusterName},
					ID:   "sg-12345",
				},
			}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
		})
		It("should fail when subnetSelectorTerms has id and other filters", func() {
			nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					Tags: map[string]string{"karpenter.sh/discovery": env.ClusterName},
					ID:   "subnet-12345",
				},
			}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
		})
		It("should fail when amiSelectorTerms has id and other filters", func() {
			nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
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
		It("should succeed to switch between an unmanaged and managed instance profile", func() {
			nodeClass.Spec.Role = ""
			nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())

			updatedNodeClass := &v1.EC2NodeClass{}
			Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), updatedNodeClass)).To(Succeed())
			updatedNodeClass.Spec.Role = "test-role"
			updatedNodeClass.Spec.InstanceProfile = nil
			Expect(env.Client.Update(env.Context, updatedNodeClass)).To(Succeed())
		})
		It("should succeed to switch between a managed and unmanaged instance profile", func() {
			// Skipping this test for private cluster because there is no VPC private endpoint for the IAM API. As a result,
			// you cannot use the default spec.role field in your EC2NodeClass. Instead, you need to provision and manage an
			// instance profile manually and then specify Karpenter to use this instance profile through the spec.instanceProfile field.
			if env.PrivateCluster {
				Skip("skipping Unmanaged instance profile test for private cluster")
			}
			nodeClass.Spec.Role = "test-role"
			nodeClass.Spec.InstanceProfile = nil
			Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())

			updatedNodeClass := &v1.EC2NodeClass{}
			Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), updatedNodeClass)).To(Succeed())
			updatedNodeClass.Spec.Role = ""
			updatedNodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(env.Client.Update(env.Context, updatedNodeClass)).To(Succeed())
		})
		It("should error if imageGCHighThresholdPercent is less than imageGCLowThresholdPercent", func() {
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				ImageGCHighThresholdPercent: lo.ToPtr(int32(10)),
				ImageGCLowThresholdPercent:  lo.ToPtr(int32(60)),
			}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
		})
		It("should error if imageGCHighThresholdPercent or imageGCLowThresholdPercent is negative", func() {
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				ImageGCHighThresholdPercent: lo.ToPtr(int32(-10)),
			}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				ImageGCLowThresholdPercent: lo.ToPtr(int32(-10)),
			}
			Expect(env.Client.Create(env.Context, nodeClass)).ToNot(Succeed())
		})
	})
})
