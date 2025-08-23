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

package nodeclass_test

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/awslabs/operatorpkg/status"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("NodeClass Launch Template CIDR Resolution Controller", func() {
	BeforeEach(func() {
		awsEnv.EC2API.Reset()
		nodeClass = test.EC2NodeClass(v1.EC2NodeClass{
			Spec: v1.EC2NodeClassSpec{
				SubnetSelectorTerms: []v1.SubnetSelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
				SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
				AMIFamily: lo.ToPtr(v1.AMIFamilyCustom),
				AMISelectorTerms: []v1.AMISelectorTerm{
					{
						Tags: map[string]string{"*": "*"},
					},
				},
			},
		})
		// Cluster CIDR will only be resolved once per lifetime of the launch template provider, reset to nil between tests
		awsEnv.LaunchTemplateProvider.ClusterCIDR.Store(nil)

		// Mock EC2 Describe Launch Template API calls to ensure CreateLaunchTemplate validation passes
		awsEnv.EC2API.DescribeLaunchTemplatesOutput.Set(&ec2.DescribeLaunchTemplatesOutput{
			LaunchTemplates: []ec2types.LaunchTemplate{
				{
					LaunchTemplateName: aws.String("test-lt"),
					LaunchTemplateId:   aws.String("lt-12345"),
				},
			},
		})
	})
	DescribeTable(
		"shouldn't resolve cluster CIDR for non-AL2023 NodeClasses",
		func(family string, terms []v1.AMISelectorTerm) {
			nodeClass.Spec.AMIFamily = lo.ToPtr(family)
			nodeClass.Spec.AMISelectorTerms = terms
			ExpectApplied(ctx, env.Client, nodeClass)
			ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
			Expect(awsEnv.LaunchTemplateProvider.ClusterCIDR.Load()).To(BeNil())
		},
		Entry(v1.AMIFamilyAL2, v1.AMIFamilyAL2, []v1.AMISelectorTerm{{Alias: "al2@latest"}}),
		Entry(v1.AMIFamilyBottlerocket, v1.AMIFamilyBottlerocket, []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}),
		Entry(v1.AMIFamilyWindows2019, v1.AMIFamilyWindows2019, []v1.AMISelectorTerm{{Alias: "windows2019@latest"}}),
		Entry(v1.AMIFamilyWindows2022, v1.AMIFamilyWindows2022, []v1.AMISelectorTerm{{Alias: "windows2022@latest"}}),
		Entry(v1.AMIFamilyCustom, v1.AMIFamilyCustom, []v1.AMISelectorTerm{{ID: "ami-12345"}}),
	)
	It("should resolve cluster CIDR for IPv4 clusters", func() {
		nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2023@latest"}}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(lo.FromPtr(awsEnv.LaunchTemplateProvider.ClusterCIDR.Load())).To(Equal("10.100.0.0/16"))
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().IsTrue(status.ConditionReady)).To(BeTrue())
	})
	It("should resolve cluster CIDR for IPv6 clusters", func() {
		awsEnv.EKSAPI.DescribeClusterBehavior.Output.Set(&eks.DescribeClusterOutput{
			Cluster: &ekstypes.Cluster{
				KubernetesNetworkConfig: &ekstypes.KubernetesNetworkConfigResponse{
					ServiceIpv6Cidr: lo.ToPtr("2001:db8::/64"),
				},
				Version: lo.ToPtr("1.30"),
			},
		})
		nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2023@latest"}}
		ExpectApplied(ctx, env.Client, nodeClass)
		ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)
		Expect(lo.FromPtr(awsEnv.LaunchTemplateProvider.ClusterCIDR.Load())).To(Equal("2001:db8::/64"))
		nodeClass = ExpectExists(ctx, env.Client, nodeClass)
		Expect(nodeClass.StatusConditions().IsTrue(status.ConditionReady)).To(BeTrue())
	})
})
