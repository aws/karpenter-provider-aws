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
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

var _ = Describe("SubnetProvider cache sharing", func() {
	ids := func(subnets []ec2types.Subnet) []string {
		return lo.Map(subnets, func(s ec2types.Subnet, _ int) string { return lo.FromPtr(s.SubnetId) })
	}
	terms := func(names ...string) []v1.SubnetSelectorTerm {
		return lo.Map(names, func(n string, _ int) v1.SubnetSelectorTerm {
			return v1.SubnetSelectorTerm{Tags: map[string]string{"Name": n}}
		})
	}
	It("should share a cache entry across NodeClasses with the same selector term", func() {
		// Distinct UIDs and Generations, so a per-NodeClass key would not share
		nc1 := test.EC2NodeClass()
		nc1.UID, nc1.Generation = "uid-1", 1
		nc1.Spec.SubnetSelectorTerms = terms("test-subnet-1")
		nc2 := test.EC2NodeClass()
		nc2.UID, nc2.Generation = "uid-2", 7
		nc2.Spec.SubnetSelectorTerms = terms("test-subnet-1")
		_, err := awsEnv.SubnetProvider.List(ctx, nc1)
		Expect(err).ToNot(HaveOccurred())
		subnets, err := awsEnv.SubnetProvider.List(ctx, nc2)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(subnets)).To(ConsistOf("subnet-test1"))
		Expect(awsEnv.SubnetCache.Items()).To(HaveLen(1))
		Expect(awsEnv.EC2API.DescribeSubnetsBehavior.Calls()).To(Equal(1))
	})
	It("should only describe new filter sets for partially overlapping NodeClasses and return each its own union", func() {
		nc1 := test.EC2NodeClass()
		nc1.Spec.SubnetSelectorTerms = terms("test-subnet-1")
		subnets, err := awsEnv.SubnetProvider.List(ctx, nc1)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(subnets)).To(ConsistOf("subnet-test1"))

		nc2 := test.EC2NodeClass()
		nc2.Spec.SubnetSelectorTerms = terms("test-subnet-1", "test-subnet-2")
		subnets, err = awsEnv.SubnetProvider.List(ctx, nc2)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(subnets)).To(ConsistOf("subnet-test1", "subnet-test2"))
		// Only the new filter set triggers another call
		Expect(awsEnv.SubnetCache.Items()).To(HaveLen(2))
		Expect(awsEnv.EC2API.DescribeSubnetsBehavior.Calls()).To(Equal(2))

		// nc1 is not polluted by nc2's extra term
		subnets, err = awsEnv.SubnetProvider.List(ctx, nc1)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(subnets)).To(ConsistOf("subnet-test1"))
		Expect(awsEnv.EC2API.DescribeSubnetsBehavior.Calls()).To(Equal(2))
	})
	It("should dedupe subnets matched by overlapping terms on both the uncached and cached paths", func() {
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
	It("should cache filter sets resolved before a failure in the same List, and not the failed one", func() {
		subnetCache := cache.New(time.Hour, time.Hour)
		provider := subnet.NewDefaultProvider(&failingTagEC2API{EC2API: awsEnv.EC2API, failValue: "fail-me"}, subnetCache, cache.New(time.Hour, time.Hour))
		nc := test.EC2NodeClass()
		nc.Spec.SubnetSelectorTerms = terms("test-subnet-1", "fail-me")
		_, err := provider.List(ctx, nc)
		Expect(err).To(HaveOccurred())
		Expect(subnetCache.Items()).To(HaveLen(1))
		Expect(awsEnv.EC2API.DescribeSubnetsBehavior.Calls()).To(Equal(1))

		// The filter set resolved before the failure is served from the cache
		other := test.EC2NodeClass()
		other.Spec.SubnetSelectorTerms = terms("test-subnet-1")
		subnets, err := provider.List(ctx, other)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(subnets)).To(ConsistOf("subnet-test1"))
		Expect(awsEnv.EC2API.DescribeSubnetsBehavior.Calls()).To(Equal(1))
	})
	It("should return the new selector's subnets after a NodeClass changes its selector", func() {
		nc := test.EC2NodeClass()
		nc.UID, nc.Generation = "uid-1", 1
		nc.Spec.SubnetSelectorTerms = terms("test-subnet-1")
		subnets, err := awsEnv.SubnetProvider.List(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(subnets)).To(ConsistOf("subnet-test1"))
		nc.Spec.SubnetSelectorTerms = terms("test-subnet-2")
		subnets, err = awsEnv.SubnetProvider.List(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(ids(subnets)).To(ConsistOf("subnet-test2"))
	})
})

var _ = Describe("SubnetProvider inflight IPs", func() {
	var pick func() string
	BeforeEach(func() {
		awsEnv.EC2API.DescribeSubnetsBehavior.Output.Set(&ec2.DescribeSubnetsOutput{Subnets: []ec2types.Subnet{
			{SubnetId: aws.String("subnet-a"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int32(100)},
			{SubnetId: aws.String("subnet-b"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int32(90)},
		}})
		nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{{Tags: map[string]string{"foo": "bar"}}}
		nodeClass.Status.Subnets = []v1.Subnet{
			{ID: "subnet-a", Zone: "test-zone-1a", ZoneID: "tstz1-1a"},
			{ID: "subnet-b", Zone: "test-zone-1a", ZoneID: "tstz1-1a"},
		}
		instanceTypes := []*cloudprovider.InstanceType{{
			Name:     "fake-it",
			Capacity: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("50")},
			Offerings: cloudprovider.Offerings{{
				Requirements: scheduling.NewLabelRequirements(map[string]string{
					karpv1.CapacityTypeLabelKey: karpv1.CapacityTypeOnDemand,
					corev1.LabelTopologyZone:    "test-zone-1a",
				}),
				Available: true,
			}},
		}}
		pick = func() string {
			zonal, err := awsEnv.SubnetProvider.ZonalSubnetsForLaunch(ctx, nodeClass, instanceTypes, karpv1.CapacityTypeOnDemand)
			Expect(err).To(BeNil())
			return zonal["test-zone-1a"].ID
		}
		_, err := awsEnv.SubnetProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		// Each launch deducts 50 inflight IPs: a(100) -> b(90) -> a(50), leaving a=0 and b=40 tracked
		Expect([]string{pick(), pick(), pick()}).To(Equal([]string{"subnet-a", "subnet-b", "subnet-a"}))
	})
	It("should reset inflight IP tracking when List refreshes subnets from EC2", func() {
		awsEnv.SubnetCache.Flush()
		_, err := awsEnv.SubnetProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		// The refresh replaces the inflight estimates with the observed counts
		Expect(pick()).To(Equal("subnet-a"))
	})
	It("should keep inflight IP tracking when List is served from the subnet cache", func() {
		// e.g. another NodeClass sharing the selector term: no EC2 call, so the inflight estimates still apply
		nodeClass2 := test.EC2NodeClass()
		nodeClass2.UID = "nodeclass-2-uid"
		nodeClass2.Spec.SubnetSelectorTerms = nodeClass.Spec.SubnetSelectorTerms
		_, err := awsEnv.SubnetProvider.List(ctx, nodeClass2)
		Expect(err).To(BeNil())
		Expect(awsEnv.EC2API.DescribeSubnetsBehavior.Calls()).To(Equal(1))
		Expect(pick()).To(Equal("subnet-b"))
	})
	It("should not let a cache hit overwrite an ICE'd subnet's available IP count with the cached count", func() {
		// subnet-a is marked as out of capacity, so subnet-b (40 inflight) must be picked
		awsEnv.SubnetProvider.UpdateICEdSubnet("subnet-a")
		nodeClass2 := test.EC2NodeClass()
		nodeClass2.UID = "nodeclass-2-uid"
		nodeClass2.Spec.SubnetSelectorTerms = nodeClass.Spec.SubnetSelectorTerms
		_, err := awsEnv.SubnetProvider.List(ctx, nodeClass2)
		Expect(err).To(BeNil())
		Expect(awsEnv.EC2API.DescribeSubnetsBehavior.Calls()).To(Equal(1))
		Expect(pick()).To(Equal("subnet-b"))
	})
})
