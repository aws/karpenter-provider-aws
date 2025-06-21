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

package securitygroup_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SecurityGroupProvider with Status", func() {
	It("should use security groups from EC2NodeClass.status when available", func() {
		// Create a nodeClass with security groups in status
		nodeClassWithStatus := test.EC2NodeClass(v1.EC2NodeClass{
			Spec: v1.EC2NodeClassSpec{
				SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{
					{
						Tags: map[string]string{
							"*": "*",
						},
					},
				},
			},
			Status: v1.EC2NodeClassStatus{
				SecurityGroups: []v1.SecurityGroup{
					{
						ID:   "sg-status-1",
						Name: "securityGroup-status-1",
					},
					{
						ID:   "sg-status-2",
						Name: "securityGroup-status-2",
					},
				},
			},
		})

		// Create a provider with a mock EC2API
		provider := securitygroup.NewDefaultProvider(awsEnv.EC2API, cache.New(cache.NoExpiration, cache.NoExpiration))

		// Call List and verify it returns security groups from status
		securityGroups, err := provider.List(context.Background(), nodeClassWithStatus)
		Expect(err).To(BeNil())

		// Verify the returned security groups match those in status
		Expect(securityGroups).To(HaveLen(2))
		Expect(securityGroups).To(ContainElement(ec2types.SecurityGroup{
			GroupId:   aws.String("sg-status-1"),
			GroupName: aws.String("securityGroup-status-1"),
		}))
		Expect(securityGroups).To(ContainElement(ec2types.SecurityGroup{
			GroupId:   aws.String("sg-status-2"),
			GroupName: aws.String("securityGroup-status-2"),
		}))

		// Verify that no AWS API calls were made
		Expect(awsEnv.EC2API.DescribeSecurityGroupsBehavior.Calls()).To(Equal(0))
	})

	It("should fall back to AWS API when security groups are not in status", func() {
		// Create a nodeClass without security groups in status
		nodeClassWithoutStatus := test.EC2NodeClass(v1.EC2NodeClass{
			Spec: v1.EC2NodeClassSpec{
				SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{
					{
						Tags: map[string]string{
							"*": "*",
						},
					},
				},
			},
		})

		// Set up expected AWS API response
		awsEnv.EC2API.DescribeSecurityGroupsBehavior.Output.Set(&ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []ec2types.SecurityGroup{
				{
					GroupId:   lo.ToPtr("sg-api-1"),
					GroupName: lo.ToPtr("securityGroup-api-1"),
				},
				{
					GroupId:   lo.ToPtr("sg-api-2"),
					GroupName: lo.ToPtr("securityGroup-api-2"),
				},
			},
		})

		// Create a provider with a mock EC2API
		provider := securitygroup.NewDefaultProvider(awsEnv.EC2API, cache.New(cache.NoExpiration, cache.NoExpiration))

		// Call List and verify it falls back to AWS API
		securityGroups, err := provider.List(context.Background(), nodeClassWithoutStatus)
		Expect(err).To(BeNil())

		// Verify the returned security groups match those from the API
		Expect(securityGroups).To(HaveLen(2))
		Expect(securityGroups).To(ContainElement(ec2types.SecurityGroup{
			GroupId:   aws.String("sg-api-1"),
			GroupName: aws.String("securityGroup-api-1"),
		}))
		Expect(securityGroups).To(ContainElement(ec2types.SecurityGroup{
			GroupId:   aws.String("sg-api-2"),
			GroupName: aws.String("securityGroup-api-2"),
		}))

		// Verify that AWS API was called
		Expect(awsEnv.EC2API.DescribeSecurityGroupsBehavior.Calls()).To(Equal(1))
	})
})
