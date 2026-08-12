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
	awstest "github.com/aws/karpenter-provider-aws/pkg/test"

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
		It("should fail to switch between an unmanaged and managed instance profile", func() {
			nodeClass.Spec.Role = ""
			nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())

			nodeClass.Spec.Role = "test-role"
			nodeClass.Spec.InstanceProfile = nil
			Expect(env.Client.Update(env.Context, nodeClass)).ToNot(Succeed())
		})
		It("should fail to switch between a managed and unmanaged instance profile", func() {
			// Skipping this test for private cluster because there is no VPC private endpoint for the IAM API. As a result,
			// you cannot use the default spec.role field in your EC2NodeClass. Instead, you need to provision and manage an
			// instance profile manually and then specify Karpenter to use this instance profile through the spec.instanceProfile field.
			if env.PrivateCluster {
				Skip("skipping Unmanaged instance profile test for private cluster")
			}
			nodeClass.Spec.Role = "test-role"
			nodeClass.Spec.InstanceProfile = nil
			Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())

			nodeClass.Spec.Role = ""
			nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(env.Client.Update(env.Context, nodeClass)).ToNot(Succeed())
		})
		// spec.kubelet is an open map the API server can't validate, so an invalid kubelet
		// configuration is accepted on apply and surfaced by the nodeclass controller as
		// ValidationSucceeded=False rather than being rejected at admission time.
		It("should mark ValidationSucceeded=False if imageGCHighThresholdPercent is less than imageGCLowThresholdPercent", func() {
			nodeClass.Spec.Kubelet = awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"imageGCHighThresholdPercent": int32(10),
				"imageGCLowThresholdPercent":  int32(60),
			})
			Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())
			ExpectKubeletValidationFailed(nodeClass)
		})
		It("should mark ValidationSucceeded=False if imageGCHighThresholdPercent is negative", func() {
			nodeClass.Spec.Kubelet = awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"imageGCHighThresholdPercent": int32(-10),
			})
			Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())
			ExpectKubeletValidationFailed(nodeClass)
		})
		It("should mark ValidationSucceeded=False if imageGCLowThresholdPercent is negative", func() {
			nodeClass.Spec.Kubelet = awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"imageGCLowThresholdPercent": int32(-10),
			})
			Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())
			ExpectKubeletValidationFailed(nodeClass)
		})
	})
})

// ExpectKubeletValidationFailed waits for the nodeclass controller to reject the applied
// EC2NodeClass's kubelet configuration by reporting ValidationSucceeded=False. spec.kubelet is an
// open map the API server can't validate, so an invalid config is accepted on apply and surfaced
// at reconcile time rather than being rejected at admission. See v1.ValidateKubeletConfig.
func ExpectKubeletValidationFailed(nodeClass *v1.EC2NodeClass) {
	GinkgoHelper()
	By("waiting for the EC2NodeClass to report ValidationSucceeded=False")
	generation := nodeClass.Generation
	Eventually(func(g Gomega) {
		g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)).To(Succeed())
		cond := nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded)
		g.Expect(cond.IsFalse()).To(BeTrue())
		g.Expect(cond.ObservedGeneration).To(Equal(generation))
		g.Expect(cond.Reason).To(Equal("InvalidKubeletConfiguration"))
	}).Should(Succeed())
}
