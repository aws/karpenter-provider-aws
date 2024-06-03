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

package status_test

import (
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodeClass Launch Template CIDR Resolution Controller", func() {
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass(v1beta1.EC2NodeClass{
			Spec: v1beta1.EC2NodeClassSpec{
				SubnetSelectorTerms: []v1beta1.SubnetSelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
				SecurityGroupSelectorTerms: []v1beta1.SecurityGroupSelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
				AMISelectorTerms: []v1beta1.AMISelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
			},
		})
		// Cluster CIDR will only be resolved once per lifetime of the launch template provider, reset to nil between tests
		awsEnv.LaunchTemplateProvider.ClusterCIDR.Store(nil)
	})
	It("shouldn't resolve cluster CIDR for non-AL2023 NodeClasses", func() {
		for _, family := range []string{
			v1beta1.AMIFamilyAL2,
			v1beta1.AMIFamilyBottlerocket,
			v1beta1.AMIFamilyUbuntu,
			v1beta1.AMIFamilyWindows2019,
			v1beta1.AMIFamilyWindows2022,
			v1beta1.AMIFamilyCustom,
		} {
			nodeClass.Spec.AMIFamily = lo.ToPtr(family)
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
			Expect(awsEnv.LaunchTemplateProvider.ClusterCIDR.Load()).To(BeNil())
		}
	})
	It("should resolve cluster CIDR for IPv4 clusters", func() {
		nodeClass.Spec.AMIFamily = lo.ToPtr(v1beta1.AMIFamilyAL2023)
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
		Expect(lo.FromPtr(awsEnv.LaunchTemplateProvider.ClusterCIDR.Load())).To(Equal("10.100.0.0/16"))
	})
	It("should resolve cluster CIDR for IPv6 clusters", func() {
		awsEnv.EKSAPI.DescribeClusterBehavior.Output.Set(&eks.DescribeClusterOutput{
			Cluster: &eks.Cluster{
				KubernetesNetworkConfig: &eks.KubernetesNetworkConfigResponse{
					ServiceIpv6Cidr: lo.ToPtr("2001:db8::/64"),
				},
			},
		})
		nodeClass.Spec.AMIFamily = lo.ToPtr(v1beta1.AMIFamilyAL2023)
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, statusController, nodeClass)
		Expect(lo.FromPtr(awsEnv.LaunchTemplateProvider.ClusterCIDR.Load())).To(Equal("2001:db8::/64"))
	})
})
