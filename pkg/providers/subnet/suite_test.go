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
	"sort"
	"sync"
	"testing"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var nodeClass *v1.EC2NodeClass

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "SubnetProvider")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	nodeClass = test.EC2NodeClass(v1.EC2NodeClass{
		Spec: v1.EC2NodeClassSpec{
			AMISelectorTerms: []v1.AMISelectorTerm{{
				Alias: "al2@latest",
			}},
			SubnetSelectorTerms: []v1.SubnetSelectorTerm{
				{
					Tags: map[string]string{
						"*": "*",
					},
				},
			},
			SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{
						"*": "*",
					},
				},
			},
		},
	})
	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("SubnetProvider", func() {
	Context("List", func() {
		It("should discover subnet by ID", func() {
			nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					ID: "subnet-test1",
				},
			}
			subnets, err := awsEnv.SubnetProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			ExpectConsistsOfSubnets([]ec2types.Subnet{
				{
					SubnetId:                lo.ToPtr("subnet-test1"),
					AvailabilityZone:        lo.ToPtr("test-zone-1a"),
					AvailabilityZoneId:      lo.ToPtr("tstz1-1a"),
					AvailableIpAddressCount: lo.ToPtr[int32](100),
					VpcId:                   lo.ToPtr("vpc-test1"),
				},
			}, subnets)
		})
		It("should discover subnets by IDs", func() {
			nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					ID: "subnet-test1",
				},
				{
					ID: "subnet-test2",
				},
			}
			subnets, err := awsEnv.SubnetProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			ExpectConsistsOfSubnets([]ec2types.Subnet{
				{
					SubnetId:                lo.ToPtr("subnet-test1"),
					AvailabilityZone:        lo.ToPtr("test-zone-1a"),
					AvailabilityZoneId:      lo.ToPtr("tstz1-1a"),
					AvailableIpAddressCount: lo.ToPtr[int32](100),
					VpcId:                   lo.ToPtr("vpc-test1"),
				},
				{
					SubnetId:                lo.ToPtr("subnet-test2"),
					AvailabilityZone:        lo.ToPtr("test-zone-1b"),
					AvailabilityZoneId:      lo.ToPtr("tstz1-1b"),
					AvailableIpAddressCount: lo.ToPtr[int32](100),
					VpcId:                   lo.ToPtr("vpc-test1"),
				},
			}, subnets)
		})
		It("should discover subnets by IDs and tags", func() {
			nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					ID:   "subnet-test1",
					Tags: map[string]string{"foo": "bar"},
				},
				{
					ID:   "subnet-test2",
					Tags: map[string]string{"foo": "bar"},
				},
			}
			subnets, err := awsEnv.SubnetProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			ExpectConsistsOfSubnets([]ec2types.Subnet{
				{
					SubnetId:                lo.ToPtr("subnet-test1"),
					AvailabilityZone:        lo.ToPtr("test-zone-1a"),
					AvailabilityZoneId:      lo.ToPtr("tstz1-1a"),
					AvailableIpAddressCount: lo.ToPtr[int32](100),
					VpcId:                   lo.ToPtr("vpc-test1"),
				},
				{
					SubnetId:                lo.ToPtr("subnet-test2"),
					AvailabilityZone:        lo.ToPtr("test-zone-1b"),
					AvailabilityZoneId:      lo.ToPtr("tstz1-1b"),
					AvailableIpAddressCount: lo.ToPtr[int32](100),
					VpcId:                   lo.ToPtr("vpc-test1"),
				},
			}, subnets)
		})
		It("should discover subnets by a single tag", func() {
			nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					Tags: map[string]string{"Name": "test-subnet-1"},
				},
			}
			subnets, err := awsEnv.SubnetProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			ExpectConsistsOfSubnets([]ec2types.Subnet{
				{
					SubnetId:                lo.ToPtr("subnet-test1"),
					AvailabilityZone:        lo.ToPtr("test-zone-1a"),
					AvailabilityZoneId:      lo.ToPtr("tstz1-1a"),
					AvailableIpAddressCount: lo.ToPtr[int32](100),
					VpcId:                   lo.ToPtr("vpc-test1"),
				},
			}, subnets)
		})
		It("should not query subnets for a term without filters", func() {
			nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					Tags: map[string]string{},
				},
			}
			subnets, err := awsEnv.SubnetProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			Expect(subnets).To(BeEmpty())
			Expect(awsEnv.EC2API.DescribeSubnetsBehavior.Calls()).To(BeZero())
		})
		It("should discover subnets by multiple tag values", func() {
			nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					Tags: map[string]string{"Name": "test-subnet-1"},
				},
				{
					Tags: map[string]string{"Name": "test-subnet-2"},
				},
			}
			subnets, err := awsEnv.SubnetProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			ExpectConsistsOfSubnets([]ec2types.Subnet{
				{
					SubnetId:                lo.ToPtr("subnet-test1"),
					AvailabilityZone:        lo.ToPtr("test-zone-1a"),
					AvailabilityZoneId:      lo.ToPtr("tstz1-1a"),
					AvailableIpAddressCount: lo.ToPtr[int32](100),
					VpcId:                   lo.ToPtr("vpc-test1"),
				},
				{
					SubnetId:                lo.ToPtr("subnet-test2"),
					AvailabilityZone:        lo.ToPtr("test-zone-1b"),
					AvailabilityZoneId:      lo.ToPtr("tstz1-1b"),
					AvailableIpAddressCount: lo.ToPtr[int32](100),
					VpcId:                   lo.ToPtr("vpc-test1"),
				},
			}, subnets)
		})
		It("should discover subnets by IDs intersected with tags", func() {
			nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					ID:   "subnet-test2",
					Tags: map[string]string{"foo": "bar"},
				},
			}
			subnets, err := awsEnv.SubnetProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			ExpectConsistsOfSubnets([]ec2types.Subnet{
				{
					SubnetId:                lo.ToPtr("subnet-test2"),
					AvailabilityZone:        lo.ToPtr("test-zone-1b"),
					AvailabilityZoneId:      lo.ToPtr("tstz1-1b"),
					AvailableIpAddressCount: lo.ToPtr[int32](100),
					VpcId:                   lo.ToPtr("vpc-test1"),
				},
			}, subnets)
		})
		It("should handle empty pages when describing subnets", func() {
			awsEnv.EC2API.DescribeSubnetsBehavior.OutputPages.Add(&ec2.DescribeSubnetsOutput{Subnets: []ec2types.Subnet{}})
			awsEnv.EC2API.DescribeSubnetsBehavior.OutputPages.Add(
				&ec2.DescribeSubnetsOutput{
					Subnets: []ec2types.Subnet{
						{SubnetId: aws.String("test-subnet-1000"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int32(10),
							Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1000")}},
						},
					},
				},
			)
			subnets, err := awsEnv.SubnetProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			ExpectConsistsOfSubnets([]ec2types.Subnet{
				{
					SubnetId:                lo.ToPtr("test-subnet-1000"),
					AvailabilityZone:        lo.ToPtr("test-zone-1a"),
					AvailabilityZoneId:      lo.ToPtr("tstz1-1a"),
					AvailableIpAddressCount: lo.ToPtr[int32](10),
					VpcId:                   lo.ToPtr("vpc-test-1"),
				},
			}, subnets)
			Expect(awsEnv.EC2API.DescribeSubnetsBehavior.Calls()).To(Equal(2))
		})
		It("should not overwrite found values when handling multiple pages of subnets", func() {
			awsEnv.EC2API.DescribeSubnetsBehavior.OutputPages.Add(&ec2.DescribeSubnetsOutput{Subnets: []ec2types.Subnet{
				{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int32(10),
					Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}},
				},
			}})
			awsEnv.EC2API.DescribeSubnetsBehavior.OutputPages.Add(
				&ec2.DescribeSubnetsOutput{
					Subnets: []ec2types.Subnet{
						{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int32(10),
							Tags:  []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}},
							VpcId: lo.ToPtr("vpc-test1"),
						},
					},
				},
			)
			awsEnv.EC2API.DescribeSubnetsBehavior.OutputPages.Add(
				&ec2.DescribeSubnetsOutput{
					Subnets: []ec2types.Subnet{
						{SubnetId: aws.String("test-subnet-3"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int32(10),
							Tags:  []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-3")}},
							VpcId: lo.ToPtr("vpc-test1"),
						},
					},
				},
			)
			subnets, err := awsEnv.SubnetProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			ExpectConsistsOfSubnets([]ec2types.Subnet{
				{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int32(10),
					Tags:  []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}},
					VpcId: lo.ToPtr("vpc-test1"),
				},
				{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int32(10),
					Tags:  []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}},
					VpcId: lo.ToPtr("vpc-test1"),
				},
				{SubnetId: aws.String("test-subnet-3"), AvailabilityZone: aws.String("test-zone-1a"), AvailabilityZoneId: aws.String("tstz1-1a"), AvailableIpAddressCount: aws.Int32(10),
					Tags:  []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-3")}},
					VpcId: lo.ToPtr("vpc-test1"),
				},
			}, subnets)
			Expect(awsEnv.EC2API.DescribeSubnetsBehavior.Calls()).To(Equal(3))
		})
	})
	Context("Provider Cache", func() {
		It("should resolve subnets from cache that are filtered by id", func() {
			expectedSubnets := []ec2types.Subnet{
				{
					SubnetId: aws.String("test-subnet-id-1"), SubnetArn: aws.String("test-subnet-arn-1"),
					Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}},
				},
			}
			awsEnv.EC2API.DescribeSubnetsBehavior.Output.Set(&ec2.DescribeSubnetsOutput{Subnets: expectedSubnets})
			for _, subnet := range expectedSubnets {
				nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
					{
						ID: *subnet.SubnetId,
					},
				}
				// Call list to request from aws and store in the cache
				_, err := awsEnv.SubnetProvider.List(ctx, nodeClass)
				Expect(err).To(BeNil())
			}

			Expect(awsEnv.SubnetCache.Items()).To(HaveLen(1))
			for _, cachedObject := range awsEnv.SubnetCache.Items() {
				cachedSubnet := cachedObject.Object.([]ec2types.Subnet)
				Expect(cachedSubnet).To(HaveLen(1))
				lo.Contains(lo.ToSlicePtr(expectedSubnets), lo.ToPtr(cachedSubnet[0]))
			}
		})
		It("should resolve subnets from cache that are filtered by tags", func() {
			expectedSubnets := []ec2types.Subnet{
				{
					SubnetId: aws.String("test-subnet-id-1"), SubnetArn: aws.String("test-subnet-arn-1"),
					Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}},
				},
			}
			awsEnv.EC2API.DescribeSubnetsBehavior.Output.Set(&ec2.DescribeSubnetsOutput{Subnets: expectedSubnets})
			tagSet := lo.Map(expectedSubnets, func(subnet ec2types.Subnet, _ int) map[string]string {
				tag, _ := lo.Find(subnet.Tags, func(tag ec2types.Tag) bool {
					return lo.FromPtr(tag.Key) == "Name"
				})
				return map[string]string{"Name": lo.FromPtr(tag.Value)}
			})
			for _, tag := range tagSet {
				nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
					{
						Tags: tag,
					},
				}
				// Call list to request from aws and store in the cache
				_, err := awsEnv.SubnetProvider.List(ctx, nodeClass)
				Expect(err).To(BeNil())
			}

			Expect(awsEnv.SubnetCache.Items()).To(HaveLen(1))
			for _, cachedObject := range awsEnv.SubnetCache.Items() {
				cachedSubnet := cachedObject.Object.([]ec2types.Subnet)
				Expect(cachedSubnet).To(HaveLen(1))
				lo.Contains(lo.ToSlicePtr(expectedSubnets), lo.ToPtr(cachedSubnet[0]))
			}
		})
		It("should correctly disambiguate AND vs OR semantics for tags", func() {
			// AND semantics
			awsEnv.EC2API.DescribeSubnetsBehavior.MultiOut.Add(&ec2.DescribeSubnetsOutput{Subnets: []ec2types.Subnet{
				{
					SubnetId: aws.String("test-subnet-id-3"), SubnetArn: aws.String("test-subnet-arn-3"),
					Tags: []ec2types.Tag{{Key: aws.String("tag-key-1"), Value: aws.String("tag-value-1")}, {Key: aws.String("tag-key-2"), Value: aws.String("tag-value-2")}},
				},
			}})
			nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					Tags: map[string]string{"tag-key-1": "tag-value-1", "tag-key-2": "tag-value-2"},
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			subnets, err := awsEnv.SubnetProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			ExpectConsistsOfSubnets([]ec2types.Subnet{
				{
					SubnetId:  aws.String("test-subnet-id-3"),
					SubnetArn: aws.String("test-subnet-arn-3"),
				},
			}, subnets)

			// OR semantics
			awsEnv.EC2API.DescribeSubnetsBehavior.MultiOut.Add(&ec2.DescribeSubnetsOutput{Subnets: []ec2types.Subnet{
				{SubnetId: aws.String("test-subnet-id-2"), SubnetArn: aws.String("test-subnet-arn-2"), Tags: []ec2types.Tag{{Key: aws.String("tag-key-2"), Value: aws.String("tag-value-2")}}},
			}})
			awsEnv.EC2API.DescribeSubnetsBehavior.MultiOut.Add(&ec2.DescribeSubnetsOutput{Subnets: []ec2types.Subnet{
				{SubnetId: aws.String("test-subnet-id-1"), SubnetArn: aws.String("test-subnet-arn-1"), Tags: []ec2types.Tag{{Key: aws.String("tag-key-1"), Value: aws.String("tag-value-1")}}},
			}})
			nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					Tags: map[string]string{"tag-key-1": "tag-value-1"},
				},
				{
					Tags: map[string]string{"tag-key-2": "tag-value-2"},
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			subnets, err = awsEnv.SubnetProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			ExpectConsistsOfSubnets([]ec2types.Subnet{
				{
					SubnetId:  aws.String("test-subnet-id-1"),
					SubnetArn: aws.String("test-subnet-arn-1"),
				},
				{
					SubnetId:  aws.String("test-subnet-id-2"),
					SubnetArn: aws.String("test-subnet-arn-2"),
				},
			}, subnets)

			cacheItems := awsEnv.SubnetCache.Items()
			// Caching is per filter set: one entry for the AND set, one for each OR set
			Expect(cacheItems).To(HaveLen(3))
			// Extract cached subnet arrays for comparison
			cachedSubnets := make([][]ec2types.Subnet, 0, len(cacheItems))
			for _, item := range cacheItems {
				cachedSubnets = append(cachedSubnets, item.Object.([]ec2types.Subnet))
			}
			Expect(cachedSubnets).To(ContainElement(
				[]ec2types.Subnet{
					{
						SubnetId:  aws.String("test-subnet-id-1"),
						SubnetArn: aws.String("test-subnet-arn-1"),
						Tags:      []ec2types.Tag{{Key: aws.String("tag-key-1"), Value: aws.String("tag-value-1")}},
					},
				},
			))
			Expect(cachedSubnets).To(ContainElement(
				[]ec2types.Subnet{
					{
						SubnetId:  aws.String("test-subnet-id-2"),
						SubnetArn: aws.String("test-subnet-arn-2"),
						Tags:      []ec2types.Tag{{Key: aws.String("tag-key-2"), Value: aws.String("tag-value-2")}},
					},
				},
			))
			Expect(cachedSubnets).To(ContainElement(
				[]ec2types.Subnet{
					{
						SubnetId:  aws.String("test-subnet-id-3"),
						SubnetArn: aws.String("test-subnet-arn-3"),
						Tags:      []ec2types.Tag{{Key: aws.String("tag-key-1"), Value: aws.String("tag-value-1")}, {Key: aws.String("tag-key-2"), Value: aws.String("tag-value-2")}},
					},
				},
			))
		})
		Context("Cross-NodeClass sharing", func() {
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
				// Filter sets are described in term order: the first succeeds, the second fails
				awsEnv.EC2API.DescribeSubnetsBehavior.Error.Set(fmt.Errorf("throttled"), fake.SkipCalls(1))
				nc := test.EC2NodeClass()
				nc.Spec.SubnetSelectorTerms = terms("test-subnet-1", "test-subnet-2")
				_, err := awsEnv.SubnetProvider.List(ctx, nc)
				Expect(err).To(HaveOccurred())
				Expect(awsEnv.SubnetCache.Items()).To(HaveLen(1))
				Expect(awsEnv.EC2API.DescribeSubnetsBehavior.SuccessfulCalls()).To(Equal(1))
				Expect(awsEnv.EC2API.DescribeSubnetsBehavior.FailedCalls()).To(Equal(1))

				// The filter set resolved before the failure is served from the cache
				other := test.EC2NodeClass()
				other.Spec.SubnetSelectorTerms = terms("test-subnet-1")
				subnets, err := awsEnv.SubnetProvider.List(ctx, other)
				Expect(err).ToNot(HaveOccurred())
				Expect(ids(subnets)).To(ConsistOf("subnet-test1"))
				Expect(awsEnv.EC2API.DescribeSubnetsBehavior.SuccessfulCalls()).To(Equal(1))
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
		Context("Inflight IPs", func() {
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
			// A second NodeClass with the same selector term calls List after the launches above
			DescribeTable("inflight IP tracking after another List",
				func(setup func(), wantCalls int, want string) {
					setup()
					nodeClass2 := test.EC2NodeClass()
					nodeClass2.UID = "nodeclass-2-uid"
					nodeClass2.Spec.SubnetSelectorTerms = nodeClass.Spec.SubnetSelectorTerms
					_, err := awsEnv.SubnetProvider.List(ctx, nodeClass2)
					Expect(err).To(BeNil())
					Expect(awsEnv.EC2API.DescribeSubnetsBehavior.Calls()).To(Equal(wantCalls))
					Expect(pick()).To(Equal(want))
				},
				// The refresh replaces the inflight estimates with the observed counts
				Entry("should reset inflight IP tracking when List refreshes subnets from EC2",
					func() { awsEnv.SubnetCache.Flush() }, 2, "subnet-a"),
				// No EC2 call, so the inflight estimates still apply
				Entry("should keep inflight IP tracking when List is served from the subnet cache",
					func() {}, 1, "subnet-b"),
				// subnet-a is marked as out of capacity, so subnet-b (40 inflight) must be picked
				Entry("should not let a cache hit overwrite an ICE'd subnet's available IP count with the cached count",
					func() { awsEnv.SubnetProvider.UpdateICEdSubnet("subnet-a") }, 1, "subnet-b"),
			)
		})
	})
	It("should not cause data races when calling List() simultaneously", func() {
		wg := sync.WaitGroup{}
		for range 10000 {
			wg.Go(func() {
				defer GinkgoRecover()
				subnets, err := awsEnv.SubnetProvider.List(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())

				Expect(subnets).To(HaveLen(4))
				// Sort everything in parallel and ensure that we don't get data races
				sort.Slice(subnets, func(i, j int) bool {
					if int(*subnets[i].AvailableIpAddressCount) != int(*subnets[j].AvailableIpAddressCount) {
						return int(*subnets[i].AvailableIpAddressCount) > int(*subnets[j].AvailableIpAddressCount)
					}
					return *subnets[i].SubnetId < *subnets[j].SubnetId
				})
				Expect(subnets).To(BeEquivalentTo([]ec2types.Subnet{
					{
						AvailabilityZone:        lo.ToPtr("test-zone-1a"),
						AvailabilityZoneId:      lo.ToPtr("tstz1-1a"),
						AvailableIpAddressCount: lo.ToPtr[int32](100),
						SubnetId:                lo.ToPtr("subnet-test1"),
						MapPublicIpOnLaunch:     lo.ToPtr(false),
						Tags: []ec2types.Tag{
							{
								Key:   lo.ToPtr("Name"),
								Value: lo.ToPtr("test-subnet-1"),
							},
							{
								Key:   lo.ToPtr("foo"),
								Value: lo.ToPtr("bar"),
							},
						},
						VpcId: lo.ToPtr("vpc-test1"),
					},
					{
						AvailabilityZone:        lo.ToPtr("test-zone-1b"),
						AvailabilityZoneId:      lo.ToPtr("tstz1-1b"),
						AvailableIpAddressCount: lo.ToPtr[int32](100),
						MapPublicIpOnLaunch:     lo.ToPtr(true),
						SubnetId:                lo.ToPtr("subnet-test2"),

						Tags: []ec2types.Tag{
							{
								Key:   lo.ToPtr("Name"),
								Value: lo.ToPtr("test-subnet-2"),
							},
							{
								Key:   lo.ToPtr("foo"),
								Value: lo.ToPtr("bar"),
							},
						},
						VpcId: lo.ToPtr("vpc-test1"),
					},
					{
						AvailabilityZone:        lo.ToPtr("test-zone-1c"),
						AvailabilityZoneId:      lo.ToPtr("tstz1-1c"),
						AvailableIpAddressCount: lo.ToPtr[int32](100),
						SubnetId:                lo.ToPtr("subnet-test3"),
						Tags: []ec2types.Tag{
							{
								Key:   lo.ToPtr("Name"),
								Value: lo.ToPtr("test-subnet-3"),
							},
							{
								Key: lo.ToPtr("TestTag"),
							},
							{
								Key:   lo.ToPtr("foo"),
								Value: lo.ToPtr("bar"),
							},
						},
						VpcId: lo.ToPtr("vpc-test1"),
					},
					{
						AvailabilityZone:        lo.ToPtr("test-zone-1a-local"),
						AvailabilityZoneId:      lo.ToPtr("tstz1-1alocal"),
						AvailableIpAddressCount: lo.ToPtr[int32](100),
						SubnetId:                lo.ToPtr("subnet-test4"),
						MapPublicIpOnLaunch:     lo.ToPtr(true),
						Tags: []ec2types.Tag{
							{
								Key:   lo.ToPtr("Name"),
								Value: lo.ToPtr("test-subnet-4"),
							},
						},
						VpcId: lo.ToPtr("vpc-test1"),
					},
				}))
			})
		}
		wg.Wait()
	})
})

func ExpectConsistsOfSubnets(expected, actual []ec2types.Subnet) {
	GinkgoHelper()
	Expect(actual).To(HaveLen(len(expected)))
	for _, elem := range expected {
		_, ok := lo.Find(actual, func(s ec2types.Subnet) bool {
			return lo.FromPtr(s.SubnetId) == lo.FromPtr(elem.SubnetId) &&
				lo.FromPtr(s.AvailabilityZoneId) == lo.FromPtr(elem.AvailabilityZoneId) &&
				lo.FromPtr(s.AvailabilityZone) == lo.FromPtr(elem.AvailabilityZone) &&
				lo.FromPtr(s.AvailableIpAddressCount) == lo.FromPtr(elem.AvailableIpAddressCount)
		})
		Expect(ok).To(BeTrue(), `Expected subnet with {"SubnetId": %q, "AvailabilityZone": %q, "AvailableIpAddressCount": %q} to exist`, lo.FromPtr(elem.SubnetId), lo.FromPtr(elem.AvailabilityZone), lo.FromPtr(elem.AvailableIpAddressCount))
	}
}
