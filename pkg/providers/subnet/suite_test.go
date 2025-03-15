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
	"sort"
	"sync"
	"testing"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
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
				},
				{
					SubnetId:                lo.ToPtr("subnet-test2"),
					AvailabilityZone:        lo.ToPtr("test-zone-1b"),
					AvailabilityZoneId:      lo.ToPtr("tstz1-1b"),
					AvailableIpAddressCount: lo.ToPtr[int32](100),
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
				},
				{
					SubnetId:                lo.ToPtr("subnet-test2"),
					AvailabilityZone:        lo.ToPtr("test-zone-1b"),
					AvailabilityZoneId:      lo.ToPtr("tstz1-1b"),
					AvailableIpAddressCount: lo.ToPtr[int32](100),
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
				},
			}, subnets)
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
				},
				{
					SubnetId:                lo.ToPtr("subnet-test2"),
					AvailabilityZone:        lo.ToPtr("test-zone-1b"),
					AvailabilityZoneId:      lo.ToPtr("tstz1-1b"),
					AvailableIpAddressCount: lo.ToPtr[int32](100),
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
				},
			}, subnets)
		})
	})
	Context("Provider Cache", func() {
		It("should resolve subnets from cache that are filtered by id", func() {
			expectedSubnets := awsEnv.EC2API.DescribeSubnetsOutput.Clone().Subnets
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

			for _, cachedObject := range awsEnv.SubnetCache.Items() {
				cachedSubnet := cachedObject.Object.([]ec2types.Subnet)
				Expect(cachedSubnet).To(HaveLen(1))
				lo.Contains(lo.ToSlicePtr(expectedSubnets), lo.ToPtr(cachedSubnet[0]))
			}
		})
		It("should resolve subnets from cache that are filtered by tags", func() {
			expectedSubnets := awsEnv.EC2API.DescribeSubnetsOutput.Clone().Subnets
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

			for _, cachedObject := range awsEnv.SubnetCache.Items() {
				cachedSubnet := cachedObject.Object.([]ec2types.Subnet)
				Expect(cachedSubnet).To(HaveLen(1))
				lo.Contains(lo.ToSlicePtr(expectedSubnets), lo.ToPtr(cachedSubnet[0]))
			}
		})
	})
	It("should not cause data races when calling List() simultaneously", func() {
		wg := sync.WaitGroup{}
		for i := 0; i < 10000; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
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
					},
				}))
			}()
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
