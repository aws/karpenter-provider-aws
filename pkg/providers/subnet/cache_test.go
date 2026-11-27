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

package subnet_test

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
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

// failingTagEC2API fails DescribeSubnets for one tag value and delegates every other call.
type failingTagEC2API struct {
	*fake.EC2API
	failValue string
}

func (f *failingTagEC2API) DescribeSubnets(ctx context.Context, in *ec2.DescribeSubnetsInput, opts ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	for _, flt := range in.Filters {
		if lo.Contains(flt.Values, f.failValue) {
			return nil, fmt.Errorf("throttled")
		}
	}
	return f.EC2API.DescribeSubnets(ctx, in, opts...)
}

var _ = Describe("SubnetProvider cache", func() {
	ids := func(subnets []ec2types.Subnet) []string {
		return lo.Map(subnets, func(s ec2types.Subnet, _ int) string { return aws.ToString(s.SubnetId) })
	}
	It("should serve overlapping terms from the cache on the second List and still dedupe", func() {
		nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
			{Tags: map[string]string{"Name": "test-subnet-1"}},
			{ID: "subnet-test1"},
			{ID: "subnet-test2"},
			{Tags: map[string]string{"Name": "test-subnet-2"}},
		}
		subnets, err := awsEnv.SubnetProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(subnets)).To(ConsistOf("subnet-test1", "subnet-test2"))
		calls := awsEnv.EC2API.DescribeSubnetsBehavior.Calls()
		subnets, err = awsEnv.SubnetProvider.List(ctx, nodeClass)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(subnets)).To(ConsistOf("subnet-test1", "subnet-test2"))
		Expect(awsEnv.EC2API.DescribeSubnetsBehavior.Calls()).To(Equal(calls))
	})
	It("should cache a filter set that succeeded earlier in the same List that later failed", func() {
		subnetCache := cache.New(time.Hour, time.Hour)
		provider := subnet.NewDefaultProvider(&failingTagEC2API{EC2API: awsEnv.EC2API, failValue: "fail-me"}, subnetCache, cache.New(time.Hour, time.Hour))
		nc := test.EC2NodeClass()
		nc.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
			{Tags: map[string]string{"Name": "test-subnet-1"}},
			{Tags: map[string]string{"Name": "fail-me"}},
		}
		_, err := provider.List(ctx, nc)
		Expect(err).To(HaveOccurred())
		Expect(subnetCache.Items()).To(HaveLen(1))
		Expect(awsEnv.EC2API.DescribeSubnetsBehavior.Calls()).To(Equal(1))

		other := test.EC2NodeClass()
		other.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{{Tags: map[string]string{"Name": "test-subnet-1"}}}
		subnets, err := provider.List(ctx, other)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(subnets)).To(ConsistOf("subnet-test1"))
		Expect(awsEnv.EC2API.DescribeSubnetsBehavior.Calls()).To(Equal(1))
	})
	It("should return the new selector's subnets after a NodeClass changes its selector", func() {
		nc := test.EC2NodeClass()
		nc.UID, nc.Generation = "uid-1", 1
		nc.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{{Tags: map[string]string{"Name": "test-subnet-1"}}}
		subnets, err := awsEnv.SubnetProvider.List(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(subnets)).To(ConsistOf("subnet-test1"))
		nc.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{{Tags: map[string]string{"Name": "test-subnet-2"}}}
		subnets, err = awsEnv.SubnetProvider.List(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(subnets)).To(ConsistOf("subnet-test2"))
	})
})
