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
	corev1 "k8s.io/api/core/v1"
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
			ExpectKubeletValidationFailed(nodeClass, "InvalidKubeletConfiguration")
		})
		It("should mark ValidationSucceeded=False if imageGCHighThresholdPercent is negative", func() {
			nodeClass.Spec.Kubelet = awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"imageGCHighThresholdPercent": int32(-10),
			})
			Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())
			ExpectKubeletValidationFailed(nodeClass, "InvalidKubeletConfiguration")
		})
		It("should mark ValidationSucceeded=False if imageGCLowThresholdPercent is negative", func() {
			nodeClass.Spec.Kubelet = awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"imageGCLowThresholdPercent": int32(-10),
			})
			Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())
			ExpectKubeletValidationFailed(nodeClass, "InvalidKubeletConfiguration")
		})
		// The NodeClassCEL gate is enabled by the e2e install (test/hack/e2e_scripts/install_karpenter.sh),
		// so these expressions are accepted on apply and the failure is surfaced by the nodeclass controller.
		// An expression that can't compile can never resolve, so it fails the compile-only gate with
		// KubeletExpressionInvalid before any instance-type evaluation is attempted.
		DescribeTable("should mark ValidationSucceeded=False when a kubelet expression can't compile",
			func(kc v1.KubeletConfiguration) {
				nodeClass.Spec.Kubelet = kc
				Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())
				ExpectKubeletValidationFailed(nodeClass, "KubeletExpressionInvalid")
			},
			Entry("maxPods with a syntax error", awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"maxPods": "min(110,",
			})),
			Entry("maxPods referencing an undefined variable", awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"maxPods": "undefined_var + 1",
			})),
			Entry("kubeReserved with a syntax error", awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"kubeReserved": map[string]string{"cpu": "vcpus *"},
			})),
			Entry("systemReserved referencing an undefined variable", awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"systemReserved": map[string]string{"memory": "bogus_var * 1048576"},
			})),
		)
		// These expressions compile but produce an invalid result for at least one candidate instance type
		// (a negative value, or a division by zero), which the per-instance-type evaluation rejects with
		// KubeletExpressionEvaluationFailed.
		DescribeTable("should mark ValidationSucceeded=False when a kubelet expression compiles but fails evaluation",
			func(kc v1.KubeletConfiguration) {
				nodeClass.Spec.Kubelet = kc
				Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())
				ExpectKubeletValidationFailed(nodeClass, "KubeletExpressionEvaluationFailed")
			},
			Entry("maxPods that evaluates to a negative value", awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"maxPods": "0 - 1",
			})),
			Entry("kubeReserved that evaluates to a negative value", awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"kubeReserved": map[string]string{"cpu": "0 - 1"},
			})),
			Entry("systemReserved that divides by zero", awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"systemReserved": map[string]string{"memory": "1048576 / (vcpus - vcpus)"},
			})),
		)
		// The e2e install enables the NodeClassCEL gate, so the enabled path is the default. The disabled
		// path flips AWS_FEATURE_GATES on the karpenter deployment (which restarts the controller); the
		// suite's AfterEach restores the original settings via ExpectSettingsReplaced.
		It("should accept a CEL expression when the NodeClassCEL gate is enabled", func() {
			nodeClass.Spec.Kubelet = awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"maxPods": "min(110, vcpus * 20)",
			})
			Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())
			ExpectKubeletValidationSucceeded(nodeClass)
		})
		DescribeTable("should mark ValidationSucceeded=False for a CEL expression when the NodeClassCEL gate is disabled",
			func(kc v1.KubeletConfiguration) {
				env.ExpectSettingsOverridden(corev1.EnvVar{Name: "AWS_FEATURE_GATES", Value: "NodeClassCEL=false"})
				nodeClass.Spec.Kubelet = kc
				Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())
				ExpectKubeletValidationFailed(nodeClass, "KubeletExpressionsDisabled")
			},
			Entry("maxPods", awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"maxPods": "min(110, vcpus * 20)",
			})),
			Entry("kubeReserved", awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"kubeReserved": map[string]string{"cpu": "max(60, vcpus * 30)"},
			})),
			Entry("systemReserved", awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"systemReserved": map[string]string{"memory": "max_pods * 11"},
			})),
		)
		It("should still accept static kubelet values when the NodeClassCEL gate is disabled", func() {
			// The gate only guards expressions -- integer maxPods and quantity-literal reservations must keep
			// working for every user who never opts in.
			env.ExpectSettingsOverridden(corev1.EnvVar{Name: "AWS_FEATURE_GATES", Value: "NodeClassCEL=false"})
			nodeClass.Spec.Kubelet = awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"maxPods":        int32(110),
				"kubeReserved":   map[string]string{"cpu": "100m"},
				"systemReserved": map[string]string{"memory": "100Mi"},
			})
			Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())
			ExpectKubeletValidationSucceeded(nodeClass)
		})
		// Only AL2023 renders the raw kubelet config through to the node. Other families apply just the subset
		// Karpenter maps to bootstrap and would silently drop the rest, so an unmappable field is rejected with
		// UnsupportedKubeletConfiguration rather than dropped. registryPullQPS is a valid upstream kubelet field
		// Karpenter doesn't map (a passthrough field); podsPerCore is a field Karpenter maps but Bottlerocket has
		// no setting to render it into.
		DescribeTable("should mark ValidationSucceeded=False for a kubelet field the AMI family won't apply",
			func(alias string, kc v1.KubeletConfiguration) {
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: alias}}
				nodeClass.Spec.Kubelet = kc
				Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())
				ExpectKubeletValidationFailed(nodeClass, "UnsupportedKubeletConfiguration")
			},
			Entry("passthrough field on Bottlerocket", "bottlerocket@latest", awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"registryPullQPS": int32(10),
			})),
			Entry("passthrough field on Windows", "windows2022@latest", awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"registryPullQPS": int32(10),
			})),
			Entry("podsPerCore on Bottlerocket", "bottlerocket@latest", awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"podsPerCore": int32(10),
			})),
		)
		It("should accept a passthrough field on AL2023, which renders the raw config through", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2023@latest"}}
			nodeClass.Spec.Kubelet = awstest.MustMakeKubeletConfiguration(map[string]interface{}{
				"registryPullQPS": int32(10),
			})
			Expect(env.Client.Create(env.Context, nodeClass)).To(Succeed())
			ExpectKubeletValidationSucceeded(nodeClass)
		})
	})
})

// ExpectKubeletValidationFailed waits for the nodeclass controller to reject the applied
// EC2NodeClass's kubelet configuration by reporting ValidationSucceeded=False with the given reason.
// spec.kubelet is an open map the API server can't validate, so an invalid config is accepted on
// apply and surfaced at reconcile time rather than being rejected at admission. See
// v1.ValidateKubeletConfig and the nodeclass validation controller.
func ExpectKubeletValidationFailed(nodeClass *v1.EC2NodeClass, reason string) {
	GinkgoHelper()
	By(fmt.Sprintf("waiting for the EC2NodeClass to report ValidationSucceeded=False with reason %s", reason))
	generation := nodeClass.Generation
	Eventually(func(g Gomega) {
		g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)).To(Succeed())
		cond := nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded)
		g.Expect(cond.IsFalse()).To(BeTrue())
		g.Expect(cond.ObservedGeneration).To(Equal(generation))
		g.Expect(cond.Reason).To(Equal(reason))
	}).Should(Succeed())
}

// ExpectKubeletValidationSucceeded waits for the nodeclass controller to accept the applied
// EC2NodeClass's kubelet configuration by reporting ValidationSucceeded=True for the current generation.
func ExpectKubeletValidationSucceeded(nodeClass *v1.EC2NodeClass) {
	GinkgoHelper()
	By("waiting for the EC2NodeClass to report ValidationSucceeded=True")
	generation := nodeClass.Generation
	Eventually(func(g Gomega) {
		g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(nodeClass), nodeClass)).To(Succeed())
		cond := nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded)
		g.Expect(cond.IsTrue()).To(BeTrue())
		g.Expect(cond.ObservedGeneration).To(Equal(generation))
	}).Should(Succeed())
}
