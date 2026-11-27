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

var _ = Describe("SecurityGroupProvider cache sharing", func() {
	ids := func(sgs []ec2types.SecurityGroup) []string {
		return lo.Map(sgs, func(s ec2types.SecurityGroup, _ int) string { return lo.FromPtr(s.GroupId) })
	}
	terms := func(names ...string) []v1.SecurityGroupSelectorTerm {
		return lo.Map(names, func(n string, _ int) v1.SecurityGroupSelectorTerm {
			return v1.SecurityGroupSelectorTerm{Tags: map[string]string{"Name": n}}
		})
	}
	It("should share a cache entry across NodeClasses with the same selector term", func() {
		// Distinct UIDs and Generations, so a per-NodeClass key would not share
		nc1 := test.EC2NodeClass()
		nc1.UID, nc1.Generation = "uid-1", 1
		nc1.Spec.SecurityGroupSelectorTerms = terms("test-security-group-1")
		nc2 := test.EC2NodeClass()
		nc2.UID, nc2.Generation = "uid-2", 7
		nc2.Spec.SecurityGroupSelectorTerms = terms("test-security-group-1")
		_, err := awsEnv.SecurityGroupProvider.List(ctx, nc1)
		Expect(err).ToNot(HaveOccurred())
		sgs, err := awsEnv.SecurityGroupProvider.List(ctx, nc2)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(sgs)).To(ConsistOf("sg-test1"))
		Expect(awsEnv.SecurityGroupCache.Items()).To(HaveLen(1))
		Expect(awsEnv.EC2API.DescribeSecurityGroupsBehavior.Calls()).To(Equal(1))
	})
	It("should only describe new filter sets for partially overlapping NodeClasses and return each its own union", func() {
		nc1 := test.EC2NodeClass()
		nc1.Spec.SecurityGroupSelectorTerms = terms("test-security-group-1")
		sgs, err := awsEnv.SecurityGroupProvider.List(ctx, nc1)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(sgs)).To(ConsistOf("sg-test1"))

		nc2 := test.EC2NodeClass()
		nc2.Spec.SecurityGroupSelectorTerms = terms("test-security-group-1", "test-security-group-2")
		sgs, err = awsEnv.SecurityGroupProvider.List(ctx, nc2)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(sgs)).To(ConsistOf("sg-test1", "sg-test2"))
		// Only the new filter set triggers another call
		Expect(awsEnv.SecurityGroupCache.Items()).To(HaveLen(2))
		Expect(awsEnv.EC2API.DescribeSecurityGroupsBehavior.Calls()).To(Equal(2))

		// nc1 is not polluted by nc2's extra term
		sgs, err = awsEnv.SecurityGroupProvider.List(ctx, nc1)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(sgs)).To(ConsistOf("sg-test1"))
		Expect(awsEnv.EC2API.DescribeSecurityGroupsBehavior.Calls()).To(Equal(2))
	})
	It("should dedupe security groups matched by overlapping terms on both the uncached and cached paths", func() {
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{Tags: map[string]string{"foo": "bar"}},
			{ID: "sg-test1"},
			{Tags: map[string]string{"Name": "test-security-group-1"}},
		}
		for range 2 {
			sgs, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(ids(sgs)).To(ConsistOf("sg-test1", "sg-test2", "sg-test3"))
		}
		Expect(awsEnv.EC2API.DescribeSecurityGroupsBehavior.Calls()).To(Equal(3))
	})
	It("should cache filter sets resolved before a failure in the same List, and not the failed one", func() {
		sgCache := cache.New(time.Hour, time.Hour)
		provider := securitygroup.NewDefaultProvider(&failingTagEC2API{EC2API: awsEnv.EC2API, failValue: "fail-me"}, sgCache)
		nc := test.EC2NodeClass()
		nc.Spec.SecurityGroupSelectorTerms = terms("test-security-group-1", "fail-me")
		_, err := provider.List(ctx, nc)
		Expect(err).To(HaveOccurred())
		Expect(sgCache.Items()).To(HaveLen(1))
		Expect(awsEnv.EC2API.DescribeSecurityGroupsBehavior.Calls()).To(Equal(1))

		// The filter set resolved before the failure is served from the cache
		other := test.EC2NodeClass()
		other.Spec.SecurityGroupSelectorTerms = terms("test-security-group-1")
		sgs, err := provider.List(ctx, other)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(sgs)).To(ConsistOf("sg-test1"))
		Expect(awsEnv.EC2API.DescribeSecurityGroupsBehavior.Calls()).To(Equal(1))
	})
	It("should return the new selector's security groups after a NodeClass changes its selector", func() {
		nc := test.EC2NodeClass()
		nc.UID, nc.Generation = "uid-1", 1
		nc.Spec.SecurityGroupSelectorTerms = terms("test-security-group-1")
		sgs, err := awsEnv.SecurityGroupProvider.List(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(sgs)).To(ConsistOf("sg-test1"))
		nc.Spec.SecurityGroupSelectorTerms = terms("test-security-group-2")
		sgs, err = awsEnv.SecurityGroupProvider.List(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(sgs)).To(ConsistOf("sg-test2"))
	})
	It("should not let callers mutate the cached slice", func() {
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{{Tags: map[string]string{"foo": "bar"}}}
		sgs, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		sgs[0] = ec2types.SecurityGroup{GroupId: aws.String("mutated")}
		sgs, err = awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(sgs)).To(ConsistOf("sg-test1", "sg-test2", "sg-test3"))
	})
	It("should keep distinct security groups that share a GroupName (e.g. in different VPCs)", func() {
		awsEnv.EC2API.DescribeSecurityGroupsBehavior.Output.Set(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []ec2types.SecurityGroup{
			{GroupId: aws.String("sg-a"), GroupName: aws.String("shared")},
			{GroupId: aws.String("sg-b"), GroupName: aws.String("shared")},
		}})
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{{Tags: map[string]string{"foo": "bar"}}}
		for range 2 {
			sgs, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(ids(sgs)).To(ConsistOf("sg-a", "sg-b"))
		}
	})
})
