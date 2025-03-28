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
	"sort"
	"sync"
	"testing"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
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
	RunSpecs(t, "SecurityGroupProvider")
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

var _ = Describe("SecurityGroupProvider", func() {
	It("should default to the clusters security groups", func() {
		securityGroups, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{
			{
				GroupId:   aws.String("sg-test1"),
				GroupName: aws.String("securityGroup-test1"),
			},
			{
				GroupId:   aws.String("sg-test2"),
				GroupName: aws.String("securityGroup-test2"),
			},
			{
				GroupId:   aws.String("sg-test3"),
				GroupName: aws.String("securityGroup-test3"),
			},
		}, securityGroups)
	})
	It("should discover security groups by tag", func() {
		awsEnv.EC2API.DescribeSecurityGroupsBehavior.Output.Set(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []ec2types.SecurityGroup{
			{GroupName: aws.String("test-sgName-1"), GroupId: aws.String("test-sg-1"), Tags: []ec2types.Tag{{Key: aws.String("kubernetes.io/cluster/test-cluster"), Value: aws.String("test-sg-1")}}},
			{GroupName: aws.String("test-sgName-2"), GroupId: aws.String("test-sg-2"), Tags: []ec2types.Tag{{Key: aws.String("kubernetes.io/cluster/test-cluster"), Value: aws.String("test-sg-2")}}},
		}})
		securityGroups, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{
			{
				GroupId:   aws.String("test-sg-1"),
				GroupName: aws.String("test-sgName-1"),
			},
			{
				GroupId:   aws.String("test-sg-2"),
				GroupName: aws.String("test-sgName-2"),
			},
		}, securityGroups)
	})
	It("should discover security groups by multiple tag values", func() {
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{"Name": "test-security-group-1"},
			},
			{
				Tags: map[string]string{"Name": "test-security-group-2"},
			},
		}
		securityGroups, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{
			{
				GroupId:   aws.String("sg-test1"),
				GroupName: aws.String("securityGroup-test1"),
			},
			{
				GroupId:   aws.String("sg-test2"),
				GroupName: aws.String("securityGroup-test2"),
			},
		}, securityGroups)
	})
	It("should discover security groups by ID", func() {
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				ID: "sg-test1",
			},
		}
		securityGroups, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{
			{
				GroupId:   aws.String("sg-test1"),
				GroupName: aws.String("securityGroup-test1"),
			},
		}, securityGroups)
	})
	It("should discover security groups by IDs", func() {
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				ID: "sg-test1",
			},
			{
				ID: "sg-test2",
			},
		}
		securityGroups, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{
			{
				GroupId:   aws.String("sg-test1"),
				GroupName: aws.String("securityGroup-test1"),
			},
			{
				GroupId:   aws.String("sg-test2"),
				GroupName: aws.String("securityGroup-test2"),
			},
		}, securityGroups)
	})
	It("should discover security groups by IDs and tags", func() {
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				ID:   "sg-test1",
				Tags: map[string]string{"foo": "bar"},
			},
			{
				ID:   "sg-test2",
				Tags: map[string]string{"foo": "bar"},
			},
		}
		securityGroups, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{
			{
				GroupId:   aws.String("sg-test1"),
				GroupName: aws.String("securityGroup-test1"),
			},
			{
				GroupId:   aws.String("sg-test2"),
				GroupName: aws.String("securityGroup-test2"),
			},
		}, securityGroups)
	})
	It("should discover security groups by IDs intersected with tags", func() {
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				ID:   "sg-test2",
				Tags: map[string]string{"foo": "bar"},
			},
		}
		securityGroups, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{
			{
				GroupId:   aws.String("sg-test2"),
				GroupName: aws.String("securityGroup-test2"),
			},
		}, securityGroups)
	})
	It("should discover security groups by names", func() {
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				Name: "securityGroup-test2",
			},
			{
				Name: "securityGroup-test3",
			},
		}
		securityGroups, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{
			{
				GroupId:   aws.String("sg-test2"),
				GroupName: aws.String("securityGroup-test2"),
			},
			{
				GroupId:   aws.String("sg-test3"),
				GroupName: aws.String("securityGroup-test3"),
			},
		}, securityGroups)
	})
	It("should discover security groups by names intersected with tags", func() {
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				Name: "securityGroup-test3",
				Tags: map[string]string{"TestTag": "*"},
			},
		}
		securityGroups, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{
			{
				GroupId:   aws.String("sg-test3"),
				GroupName: aws.String("securityGroup-test3"),
			},
		}, securityGroups)
	})
	Context("Provider Cache", func() {
		It("should resolve security groups from cache that are filtered by id", func() {
			expectedSecurityGroups := awsEnv.EC2API.DescribeSecurityGroupsBehavior.Output.Clone().SecurityGroups
			for _, sg := range expectedSecurityGroups {
				nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
					{
						ID: *sg.GroupId,
					},
				}
				// Call list to request from aws and store in the cache
				_, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
				Expect(err).To(BeNil())
			}

			for _, cachedObject := range awsEnv.SecurityGroupCache.Items() {
				cachedSecurityGroup := cachedObject.Object.([]ec2types.SecurityGroup)
				Expect(cachedSecurityGroup).To(HaveLen(1))
				lo.Contains(lo.ToSlicePtr(expectedSecurityGroups), lo.ToPtr(cachedSecurityGroup[0]))
			}
		})
		It("should resolve security groups from cache that are filtered by Name", func() {
			expectedSecurityGroups := awsEnv.EC2API.DescribeSecurityGroupsBehavior.Output.Clone().SecurityGroups
			for _, sg := range expectedSecurityGroups {
				nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
					{
						Name: *sg.GroupName,
					},
				}
				// Call list to request from aws and store in the cache
				_, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
				Expect(err).To(BeNil())
			}

			for _, cachedObject := range awsEnv.SecurityGroupCache.Items() {
				cachedSecurityGroup := cachedObject.Object.([]ec2types.SecurityGroup)
				Expect(cachedSecurityGroup).To(HaveLen(1))
				lo.Contains(lo.ToSlicePtr(expectedSecurityGroups), lo.ToPtr(cachedSecurityGroup[0]))
			}
		})
		It("should resolve security groups from cache that are filtered by tags", func() {
			expectedSecurityGroups := awsEnv.EC2API.DescribeSecurityGroupsBehavior.Output.Clone().SecurityGroups
			tagSet := lo.Map(expectedSecurityGroups, func(sg ec2types.SecurityGroup, _ int) map[string]string {
				tag, _ := lo.Find(sg.Tags, func(tag ec2types.Tag) bool {
					return lo.FromPtr(tag.Key) == "Name"
				})
				return map[string]string{"Name": lo.FromPtr(tag.Value)}
			})
			for _, tag := range tagSet {
				nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
					{
						Tags: tag,
					},
				}
				// Call list to request from aws and store in the cache
				_, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
				Expect(err).To(BeNil())
			}

			for _, cachedObject := range awsEnv.SubnetCache.Items() {
				cachedSecurityGroup := cachedObject.Object.([]ec2types.SecurityGroup)
				Expect(cachedSecurityGroup).To(HaveLen(1))
				lo.Contains(lo.ToSlicePtr(expectedSecurityGroups), lo.ToPtr(cachedSecurityGroup[0]))
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
				securityGroups, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())

				Expect(securityGroups).To(HaveLen(3))
				// Sort everything in parallel and ensure that we don't get data races
				sort.Slice(securityGroups, func(i, j int) bool {
					return *securityGroups[i].GroupId < *securityGroups[j].GroupId
				})
				Expect(securityGroups).To(BeEquivalentTo([]ec2types.SecurityGroup{
					{
						GroupId:   lo.ToPtr("sg-test1"),
						GroupName: lo.ToPtr("securityGroup-test1"),
						Tags: []ec2types.Tag{
							{
								Key:   lo.ToPtr("Name"),
								Value: lo.ToPtr("test-security-group-1"),
							},
							{
								Key:   lo.ToPtr("foo"),
								Value: lo.ToPtr("bar"),
							},
						},
					},
					{
						GroupId:   lo.ToPtr("sg-test2"),
						GroupName: lo.ToPtr("securityGroup-test2"),
						Tags: []ec2types.Tag{
							{
								Key:   lo.ToPtr("Name"),
								Value: lo.ToPtr("test-security-group-2"),
							},
							{
								Key:   lo.ToPtr("foo"),
								Value: lo.ToPtr("bar"),
							},
						},
					},
					{
						GroupId:   lo.ToPtr("sg-test3"),
						GroupName: lo.ToPtr("securityGroup-test3"),
						Tags: []ec2types.Tag{
							{
								Key:   lo.ToPtr("Name"),
								Value: lo.ToPtr("test-security-group-3"),
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
				}))
			}()
		}
		wg.Wait()
	})
})

func ExpectConsistsOfSecurityGroups(expected, actual []ec2types.SecurityGroup) {
	GinkgoHelper()
	Expect(actual).To(HaveLen(len(expected)))
	for _, elem := range expected {
		_, ok := lo.Find(actual, func(s ec2types.SecurityGroup) bool {
			return lo.FromPtr(s.GroupId) == lo.FromPtr(elem.GroupId) &&
				lo.FromPtr(s.GroupName) == lo.FromPtr(elem.GroupName)
		})
		Expect(ok).To(BeTrue(), `Expected security group with {"GroupId": %q, "GroupName": %q} to exist`, lo.FromPtr(elem.GroupId), lo.FromPtr(elem.GroupName))
	}
}
