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
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

// failingTagEC2API fails DescribeSecurityGroups for one tag value and delegates every other call.
type failingTagEC2API struct {
	*fake.EC2API
	failValue string
}

func (f *failingTagEC2API) DescribeSecurityGroups(ctx context.Context, in *ec2.DescribeSecurityGroupsInput, opts ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	for _, flt := range in.Filters {
		if lo.Contains(flt.Values, f.failValue) {
			return nil, fmt.Errorf("throttled")
		}
	}
	return f.EC2API.DescribeSecurityGroups(ctx, in, opts...)
}

var _ = Describe("SecurityGroupProvider cache", func() {
	It("should cache a filter set that succeeded earlier in the same List that later failed", func() {
		sgCache := cache.New(time.Hour, time.Hour)
		provider := securitygroup.NewDefaultProvider(&failingTagEC2API{EC2API: awsEnv.EC2API, failValue: "fail-me"}, sgCache)
		nc := test.EC2NodeClass()
		nc.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{Tags: map[string]string{"Name": "test-security-group-1"}},
			{Tags: map[string]string{"Name": "fail-me"}},
		}
		_, err := provider.List(ctx, nc)
		Expect(err).To(HaveOccurred())
		Expect(sgCache.Items()).To(HaveLen(1))
		Expect(awsEnv.EC2API.DescribeSecurityGroupsBehavior.Calls()).To(Equal(1))

		other := test.EC2NodeClass()
		other.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{{Tags: map[string]string{"Name": "test-security-group-1"}}}
		sgs, err := provider.List(ctx, other)
		Expect(err).ToNot(HaveOccurred())
		Expect(lo.Map(sgs, func(s ec2types.SecurityGroup, _ int) string { return aws.ToString(s.GroupId) })).To(ConsistOf("sg-test1"))
		Expect(awsEnv.EC2API.DescribeSecurityGroupsBehavior.Calls()).To(Equal(1))
	})
	It("should return the new selector's security groups after a NodeClass changes its selector", func() {
		nc := test.EC2NodeClass()
		nc.UID, nc.Generation = "uid-1", 1
		nc.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{{Tags: map[string]string{"Name": "test-security-group-1"}}}
		sgs, err := awsEnv.SecurityGroupProvider.List(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(lo.Map(sgs, func(s ec2types.SecurityGroup, _ int) string { return aws.ToString(s.GroupId) })).To(ConsistOf("sg-test1"))
		nc.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{{Tags: map[string]string{"Name": "test-security-group-2"}}}
		sgs, err = awsEnv.SecurityGroupProvider.List(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(lo.Map(sgs, func(s ec2types.SecurityGroup, _ int) string { return aws.ToString(s.GroupId) })).To(ConsistOf("sg-test2"))
	})
})
