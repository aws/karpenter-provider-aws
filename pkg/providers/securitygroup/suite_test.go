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
	"sort"
	"sync"
	"testing"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	gocache "github.com/patrickmn/go-cache"
	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
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
			expectedSecurityGroups := []ec2types.SecurityGroup{
				{
					GroupId: aws.String("test-sg-id-1"), GroupName: aws.String("test-sg-name-1"),
					Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-sg-1")}},
				},
			}
			awsEnv.EC2API.DescribeSecurityGroupsBehavior.Output.Set(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: expectedSecurityGroups})
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

			Expect(awsEnv.SecurityGroupCache.Items()).To(HaveLen(1))
			for _, cachedObject := range awsEnv.SecurityGroupCache.Items() {
				cachedSecurityGroup := cachedObject.Object.([]ec2types.SecurityGroup)
				Expect(cachedSecurityGroup).To(HaveLen(1))
				lo.Contains(lo.ToSlicePtr(expectedSecurityGroups), lo.ToPtr(cachedSecurityGroup[0]))
			}
		})
		It("should resolve security groups from cache that are filtered by Name", func() {
			expectedSecurityGroups := []ec2types.SecurityGroup{
				{
					GroupId: aws.String("test-sg-id-1"), GroupName: aws.String("test-sg-name-1"),
					Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-sg-1")}},
				},
			}
			awsEnv.EC2API.DescribeSecurityGroupsBehavior.Output.Set(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: expectedSecurityGroups})
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

			Expect(awsEnv.SecurityGroupCache.Items()).To(HaveLen(1))
			for _, cachedObject := range awsEnv.SecurityGroupCache.Items() {
				cachedSecurityGroup := cachedObject.Object.([]ec2types.SecurityGroup)
				Expect(cachedSecurityGroup).To(HaveLen(1))
				lo.Contains(lo.ToSlicePtr(expectedSecurityGroups), lo.ToPtr(cachedSecurityGroup[0]))
			}
		})
		It("should resolve security groups from cache that are filtered by tags", func() {
			expectedSecurityGroups := []ec2types.SecurityGroup{
				{
					GroupId: aws.String("test-sg-id-1"), GroupName: aws.String("test-sg-name-1"),
					Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("test-sg-1")}},
				},
			}
			awsEnv.EC2API.DescribeSecurityGroupsBehavior.Output.Set(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: expectedSecurityGroups})
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

			Expect(awsEnv.SecurityGroupCache.Items()).To(HaveLen(1))
			for _, cachedObject := range awsEnv.SecurityGroupCache.Items() {
				cachedSecurityGroup := cachedObject.Object.([]ec2types.SecurityGroup)
				Expect(cachedSecurityGroup).To(HaveLen(1))
				lo.Contains(lo.ToSlicePtr(expectedSecurityGroups), lo.ToPtr(cachedSecurityGroup[0]))
			}
		})
		It("should correctly disambiguate AND vs OR semantics for tags", func() {
			// AND semantics
			awsEnv.EC2API.DescribeSecurityGroupsBehavior.MultiOut.Add(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []ec2types.SecurityGroup{
				{GroupName: aws.String("test-sgName-3"), GroupId: aws.String("test-sg-3"), Tags: []ec2types.Tag{{Key: aws.String("tag-key-1"), Value: aws.String("tag-value-1")}, {Key: aws.String("tag-key-2"), Value: aws.String("tag-value-2")}}},
			}})
			nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{"tag-key-1": "tag-value-1", "tag-key-2": "tag-value-2"},
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			securityGroups, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
			Expect(err).To(BeNil())
			ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{
				{
					GroupId:   aws.String("test-sg-3"),
					GroupName: aws.String("test-sgName-3"),
				},
			}, securityGroups)

			// OR semantics
			awsEnv.EC2API.DescribeSecurityGroupsBehavior.MultiOut.Add(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []ec2types.SecurityGroup{
				{GroupName: aws.String("test-sgName-2"), GroupId: aws.String("test-sg-2"), Tags: []ec2types.Tag{{Key: aws.String("tag-key-2"), Value: aws.String("tag-value-2")}}},
			}})
			awsEnv.EC2API.DescribeSecurityGroupsBehavior.MultiOut.Add(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []ec2types.SecurityGroup{
				{GroupName: aws.String("test-sgName-1"), GroupId: aws.String("test-sg-1"), Tags: []ec2types.Tag{{Key: aws.String("tag-key-1"), Value: aws.String("tag-value-1")}}},
			}})
			nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{"tag-key-1": "tag-value-1"},
				},
				{
					Tags: map[string]string{"tag-key-2": "tag-value-2"},
				},
			}
			ExpectApplied(ctx, env.Client, nodeClass)
			securityGroups, err = awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
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

			cacheItems := awsEnv.SecurityGroupCache.Items()
			// There should be 2 cache entries one for each semantic.
			Expect(cacheItems).To(HaveLen(2))
			// Extract cached security group arrays for comparison
			cachedSecurityGroups := make([][]ec2types.SecurityGroup, 0, len(cacheItems))
			for _, item := range cacheItems {
				cachedSecurityGroups = append(cachedSecurityGroups, item.Object.([]ec2types.SecurityGroup))
			}
			// Expect cache to contain result of both look ups.
			Expect(cachedSecurityGroups).To(ContainElement(ContainElements(
				[]ec2types.SecurityGroup{
					{
						GroupId:   aws.String("test-sg-1"),
						GroupName: aws.String("test-sgName-1"),
						Tags:      []ec2types.Tag{{Key: aws.String("tag-key-1"), Value: aws.String("tag-value-1")}},
					},
					{
						GroupId:   aws.String("test-sg-2"),
						GroupName: aws.String("test-sgName-2"),
						Tags:      []ec2types.Tag{{Key: aws.String("tag-key-2"), Value: aws.String("tag-value-2")}},
					},
				},
			)))
			Expect(cachedSecurityGroups).To(ContainElement(
				[]ec2types.SecurityGroup{
					{
						GroupId:   aws.String("test-sg-3"),
						GroupName: aws.String("test-sgName-3"),
						Tags:      []ec2types.Tag{{Key: aws.String("tag-key-1"), Value: aws.String("tag-value-1")}, {Key: aws.String("tag-key-2"), Value: aws.String("tag-value-2")}},
					},
				},
			))
		})
	})
	Context("Cluster VPC Scoping", func() {
		var securityGroupProvider *securitygroup.DefaultProvider
		var securityGroupCache *gocache.Cache
		clusterVPCFilter := ec2types.Filter{Name: lo.ToPtr("vpc-id"), Values: []string{"vpc-test1"}}
		// Security group names are only unique within a VPC, so both groups share a name
		clusterVPCSecurityGroup := ec2types.SecurityGroup{
			GroupId:   lo.ToPtr("sg-cluster-vpc"),
			GroupName: lo.ToPtr("karpenter-nodes"),
			VpcId:     lo.ToPtr("vpc-test1"),
			Tags:      []ec2types.Tag{{Key: lo.ToPtr("karpenter.sh/discovery"), Value: lo.ToPtr("test-cluster")}},
		}
		otherVPCSecurityGroup := ec2types.SecurityGroup{
			GroupId:   lo.ToPtr("sg-other-vpc"),
			GroupName: lo.ToPtr("karpenter-nodes"),
			VpcId:     lo.ToPtr("vpc-other"),
			Tags:      []ec2types.Tag{{Key: lo.ToPtr("karpenter.sh/discovery"), Value: lo.ToPtr("other-cluster")}},
		}
		wildcardTagFilter := ec2types.Filter{Name: lo.ToPtr("tag-key"), Values: []string{"karpenter.sh/discovery"}}

		BeforeEach(func() {
			// The resolved cluster VPC is cached on the provider and isn't cleared by awsEnv.Reset()
			securityGroupCache = gocache.New(gocache.NoExpiration, gocache.NoExpiration)
			securityGroupProvider = securitygroup.NewDefaultProvider(awsEnv.EC2API, awsEnv.EKSAPI, securityGroupCache)
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{ClusterEndpoint: lo.ToPtr("")}))
			awsEnv.EC2API.SecurityGroups.Store(lo.FromPtr(clusterVPCSecurityGroup.GroupId), clusterVPCSecurityGroup)
			awsEnv.EC2API.SecurityGroups.Store(lo.FromPtr(otherVPCSecurityGroup.GroupId), otherVPCSecurityGroup)
			nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{{Tags: map[string]string{"karpenter.sh/discovery": "*"}}}
		})
		It("should not scope discovery or call DescribeCluster when the cluster endpoint is set explicitly", func() {
			ctx = options.ToContext(ctx, test.Options())
			awsEnv.EKSAPI.DescribeClusterBehavior.Error.Set(fmt.Errorf("not an EKS cluster"))
			securityGroups, err := securityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{clusterVPCSecurityGroup, otherVPCSecurityGroup}, securityGroups)
			Expect(describeSecurityGroupsFilters()).To(ConsistOf(ConsistOf(wildcardTagFilter)))
			Expect(awsEnv.EKSAPI.DescribeClusterBehavior.Calls()).To(Equal(0))
			Expect(awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.Calls()).To(Equal(0))
		})
		It("should scope tag-based discovery to the cluster VPC when the cluster endpoint is discovered", func() {
			securityGroups, err := securityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{clusterVPCSecurityGroup}, securityGroups)
			Expect(describeSecurityGroupsFilters()).To(ConsistOf(ConsistOf(wildcardTagFilter)))
		})
		It("should scope tag-based discovery to the cluster VPC when eksControlPlane is enabled", func() {
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{EKSControlPlane: lo.ToPtr(true)}))
			securityGroups, err := securityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{clusterVPCSecurityGroup}, securityGroups)
			Expect(describeSecurityGroupsFilters()).To(ConsistOf(ConsistOf(wildcardTagFilter)))
		})
		DescribeTable("should retain security groups associated with the cluster VPC",
			func(term v1.SecurityGroupSelectorTerm) {
				awsEnv.EC2API.SecurityGroupVpcAssociations.Store("sg-other-vpc/vpc-test1", ec2types.SecurityGroupVpcAssociation{
					GroupId: lo.ToPtr("sg-other-vpc"),
					VpcId:   lo.ToPtr("vpc-test1"),
					State:   ec2types.SecurityGroupVpcAssociationStateAssociated,
				})
				nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{term}
				securityGroups, err := securityGroupProvider.List(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())
				ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{clusterVPCSecurityGroup, otherVPCSecurityGroup}, securityGroups)
				Expect(awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.Calls()).To(Equal(1))
				Expect(awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.CalledWithInput.Pop().Filters).To(ConsistOf(
					clusterVPCFilter,
					ec2types.Filter{Name: lo.ToPtr("state"), Values: []string{"associated"}},
				))
			},
			Entry("tag selector", v1.SecurityGroupSelectorTerm{Tags: map[string]string{"karpenter.sh/discovery": "*"}}),
			Entry("name selector", v1.SecurityGroupSelectorTerm{Name: "karpenter-nodes"}),
		)
		DescribeTable("should exclude groups without an active association to the cluster VPC",
			func(state ec2types.SecurityGroupVpcAssociationState, vpcID string) {
				awsEnv.EC2API.SecurityGroupVpcAssociations.Store("association", ec2types.SecurityGroupVpcAssociation{
					GroupId: lo.ToPtr("sg-other-vpc"), VpcId: lo.ToPtr(vpcID), State: state,
				})
				securityGroups, err := securityGroupProvider.List(ctx, nodeClass)
				Expect(err).ToNot(HaveOccurred())
				ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{clusterVPCSecurityGroup}, securityGroups)
			},
			Entry("associating", ec2types.SecurityGroupVpcAssociationState("associating"), "vpc-test1"),
			Entry("association failed", ec2types.SecurityGroupVpcAssociationState("association-failed"), "vpc-test1"),
			Entry("disassociating", ec2types.SecurityGroupVpcAssociationState("disassociating"), "vpc-test1"),
			Entry("disassociated", ec2types.SecurityGroupVpcAssociationState("disassociated"), "vpc-test1"),
			Entry("another VPC", ec2types.SecurityGroupVpcAssociationStateAssociated, "vpc-unrelated"),
		)
		It("should not query associations when all matching groups were created in the cluster VPC", func() {
			nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{{Tags: map[string]string{"karpenter.sh/discovery": "test-cluster"}}}
			awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.Error.Set(fmt.Errorf("AccessDenied"))
			securityGroups, err := securityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{clusterVPCSecurityGroup}, securityGroups)
			Expect(awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.Calls()).To(Equal(0))
		})
		It("should preserve explicit IDs that also match scoped terms without checking their associations", func() {
			nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{ID: "sg-other-vpc"}, {Tags: map[string]string{"karpenter.sh/discovery": "*"}},
			}
			awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.Error.Set(fmt.Errorf("AccessDenied"))
			securityGroups, err := securityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{clusterVPCSecurityGroup, otherVPCSecurityGroup}, securityGroups)
			Expect(awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.Calls()).To(Equal(0))
		})
		It("should fail the entire mixed selector and retry after an association lookup error", func() {
			nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{ID: "sg-cluster-vpc"}, {Tags: map[string]string{"karpenter.sh/discovery": "*"}},
			}
			awsEnv.EC2API.SecurityGroupVpcAssociations.Store("association", ec2types.SecurityGroupVpcAssociation{
				GroupId: lo.ToPtr("sg-other-vpc"), VpcId: lo.ToPtr("vpc-test1"), State: ec2types.SecurityGroupVpcAssociationStateAssociated,
			})
			awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.Error.Set(fmt.Errorf("AccessDenied"), fake.MaxCalls(1))
			securityGroups, err := securityGroupProvider.List(ctx, nodeClass)
			Expect(err).To(MatchError(ContainSubstring("AccessDenied")))
			Expect(securityGroups).To(BeEmpty())
			Expect(securityGroupCache.ItemCount()).To(BeZero())
			securityGroups, err = securityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{clusterVPCSecurityGroup, otherVPCSecurityGroup}, securityGroups)
			Expect(awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.Calls()).To(Equal(2))
			Expect(awsEnv.EKSAPI.DescribeClusterBehavior.Calls()).To(Equal(1))
		})
		It("should paginate associations and only retain groups matched by the selector", func() {
			awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.OutputPages.Add(
				&ec2.DescribeSecurityGroupVpcAssociationsOutput{SecurityGroupVpcAssociations: []ec2types.SecurityGroupVpcAssociation{
					{GroupId: lo.ToPtr("sg-unmatched"), VpcId: lo.ToPtr("vpc-test1"), State: ec2types.SecurityGroupVpcAssociationStateAssociated},
				}},
			)
			awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.OutputPages.Add(
				&ec2.DescribeSecurityGroupVpcAssociationsOutput{SecurityGroupVpcAssociations: []ec2types.SecurityGroupVpcAssociation{
					{GroupId: lo.ToPtr("sg-other-vpc"), VpcId: lo.ToPtr("vpc-test1"), State: ec2types.SecurityGroupVpcAssociationStateAssociated},
				}},
			)
			securityGroups, err := securityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{clusterVPCSecurityGroup, otherVPCSecurityGroup}, securityGroups)
			Expect(awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.Calls()).To(Equal(2))
		})
		It("should refresh association eligibility with the security group cache", func() {
			association := ec2types.SecurityGroupVpcAssociation{
				GroupId: lo.ToPtr("sg-other-vpc"), VpcId: lo.ToPtr("vpc-test1"), State: ec2types.SecurityGroupVpcAssociationStateAssociated,
			}
			awsEnv.EC2API.SecurityGroupVpcAssociations.Store("association", association)
			securityGroups, err := securityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{clusterVPCSecurityGroup, otherVPCSecurityGroup}, securityGroups)
			association.State = ec2types.SecurityGroupVpcAssociationState("disassociated")
			awsEnv.EC2API.SecurityGroupVpcAssociations.Store("association", association)
			securityGroups, err = securityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{clusterVPCSecurityGroup, otherVPCSecurityGroup}, securityGroups)
			Expect(awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.Calls()).To(Equal(1))
			securityGroupCache.Flush()
			securityGroups, err = securityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{clusterVPCSecurityGroup}, securityGroups)
			Expect(awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.Calls()).To(Equal(2))
			Expect(awsEnv.EKSAPI.DescribeClusterBehavior.Calls()).To(Equal(1))
		})
		It("should scope name-based discovery to the cluster VPC", func() {
			nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{{Name: "karpenter-nodes"}}
			securityGroups, err := securityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{clusterVPCSecurityGroup}, securityGroups)
			Expect(describeSecurityGroupsFilters()).To(ConsistOf(ConsistOf(ec2types.Filter{Name: lo.ToPtr("group-name"), Values: []string{"karpenter-nodes"}})))
		})
		It("should not scope discovery or call DescribeCluster for ID-only selectors", func() {
			nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{{ID: "sg-other-vpc"}}
			awsEnv.EKSAPI.DescribeClusterBehavior.Error.Set(fmt.Errorf("failed to describe cluster"))
			securityGroups, err := securityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{otherVPCSecurityGroup}, securityGroups)
			Expect(describeSecurityGroupsFilters()).To(ConsistOf(ConsistOf(ec2types.Filter{Name: lo.ToPtr("group-id"), Values: []string{"sg-other-vpc"}})))
			Expect(awsEnv.EKSAPI.DescribeClusterBehavior.Calls()).To(Equal(0))
			Expect(awsEnv.EC2API.DescribeSecurityGroupVpcAssociationsBehavior.Calls()).To(Equal(0))
		})
		It("should only scope the tag-based and name-based terms of a mixed selector and return the union", func() {
			nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{ID: "sg-other-vpc"},
				{Name: "karpenter-nodes"},
				{Tags: map[string]string{"karpenter.sh/discovery": "*"}},
			}
			securityGroups, err := securityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{clusterVPCSecurityGroup, otherVPCSecurityGroup}, securityGroups)
			Expect(describeSecurityGroupsFilters()).To(ConsistOf(
				ConsistOf(wildcardTagFilter),
				ConsistOf(ec2types.Filter{Name: lo.ToPtr("group-id"), Values: []string{"sg-other-vpc"}}),
				ConsistOf(ec2types.Filter{Name: lo.ToPtr("group-name"), Values: []string{"karpenter-nodes"}}),
			))
		})
		It("should fail a mixed selector instead of returning only the ID-based security groups when DescribeCluster fails", func() {
			nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
				{ID: "sg-other-vpc"},
				{Tags: map[string]string{"karpenter.sh/discovery": "*"}},
			}
			awsEnv.EKSAPI.DescribeClusterBehavior.Error.Set(fmt.Errorf("AccessDeniedException"))
			_, err := securityGroupProvider.List(ctx, nodeClass)
			Expect(err).To(HaveOccurred())
			Expect(awsEnv.EC2API.DescribeSecurityGroupsBehavior.Calls()).To(Equal(0))
		})
		DescribeTable("should fail without falling back to unscoped discovery when DescribeCluster returns no VPC ID",
			func(output *eks.DescribeClusterOutput) {
				awsEnv.EKSAPI.DescribeClusterBehavior.Output.Set(output)
				_, err := securityGroupProvider.List(ctx, nodeClass)
				Expect(err).To(HaveOccurred())
				Expect(awsEnv.EC2API.DescribeSecurityGroupsBehavior.Calls()).To(Equal(0))
			},
			Entry("no cluster", &eks.DescribeClusterOutput{}),
			Entry("no VPC config", &eks.DescribeClusterOutput{Cluster: &ekstypes.Cluster{}}),
			Entry("no VPC ID", &eks.DescribeClusterOutput{Cluster: &ekstypes.Cluster{ResourcesVpcConfig: &ekstypes.VpcConfigResponse{}}}),
			Entry("empty VPC ID", &eks.DescribeClusterOutput{Cluster: &ekstypes.Cluster{ResourcesVpcConfig: &ekstypes.VpcConfigResponse{VpcId: lo.ToPtr("")}}}),
		)
		It("should retry resolving the cluster VPC after a failure", func() {
			awsEnv.EKSAPI.DescribeClusterBehavior.Error.Set(fmt.Errorf("ThrottlingException"), fake.MaxCalls(1))
			_, err := securityGroupProvider.List(ctx, nodeClass)
			Expect(err).To(HaveOccurred())
			securityGroups, err := securityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{clusterVPCSecurityGroup}, securityGroups)
			Expect(awsEnv.EKSAPI.DescribeClusterBehavior.Calls()).To(Equal(2))
		})
		It("should keep the resolved cluster VPC when security groups are refreshed", func() {
			_, err := securityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			securityGroupCache.Flush()
			_, err = securityGroupProvider.List(ctx, nodeClass)
			Expect(err).ToNot(HaveOccurred())
			Expect(awsEnv.EC2API.DescribeSecurityGroupsBehavior.Calls()).To(Equal(2))
			Expect(awsEnv.EKSAPI.DescribeClusterBehavior.Calls()).To(Equal(1))
		})
	})
	It("should not cause data races when calling List() simultaneously", func() {
		wg := sync.WaitGroup{}
		for range 10000 {
			wg.Go(func() {
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
						VpcId:     lo.ToPtr("vpc-test1"),
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
						VpcId:     lo.ToPtr("vpc-test1"),
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
						VpcId:     lo.ToPtr("vpc-test1"),
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
			})
		}
		wg.Wait()
	})
	It("should handle empty pages when describing security groups", func() {
		awsEnv.EC2API.DescribeSecurityGroupsBehavior.OutputPages.Add(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []ec2types.SecurityGroup{}})
		awsEnv.EC2API.DescribeSecurityGroupsBehavior.OutputPages.Add(
			&ec2.DescribeSecurityGroupsOutput{
				SecurityGroups: []ec2types.SecurityGroup{
					{
						GroupId:   aws.String("test-sg-1000"),
						GroupName: aws.String("test-sgName-1000"),
						Tags: []ec2types.Tag{
							{Key: aws.String("Name"), Value: aws.String("test-security-group-1000")},
						},
					},
				},
			},
		)
		securityGroups, err := awsEnv.SecurityGroupProvider.List(ctx, nodeClass)
		Expect(err).To(BeNil())
		ExpectConsistsOfSecurityGroups([]ec2types.SecurityGroup{
			{
				GroupId:   aws.String("test-sg-1000"),
				GroupName: aws.String("test-sgName-1000"),
			},
		}, securityGroups)
		Expect(awsEnv.EC2API.DescribeSecurityGroupsBehavior.Calls()).To(Equal(2))
	})
	It("should not overwrite found values when handling multiple pages of security groups", func() {
		awsEnv.EC2API.DescribeSecurityGroupsBehavior.OutputPages.Add(&ec2.DescribeSecurityGroupsOutput{
			SecurityGroups: []ec2types.SecurityGroup{
				{
					GroupId:   aws.String("test-sg-1"),
					GroupName: aws.String("test-sgName-1"),
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String("test-security-group-1")},
					},
				},
			},
		})
		awsEnv.EC2API.DescribeSecurityGroupsBehavior.OutputPages.Add(
			&ec2.DescribeSecurityGroupsOutput{
				SecurityGroups: []ec2types.SecurityGroup{
					{
						GroupId:   aws.String("test-sg-2"),
						GroupName: aws.String("test-sgName-2"),
						Tags: []ec2types.Tag{
							{Key: aws.String("Name"), Value: aws.String("test-security-group-2")},
						},
					},
				},
			},
		)
		awsEnv.EC2API.DescribeSecurityGroupsBehavior.OutputPages.Add(
			&ec2.DescribeSecurityGroupsOutput{
				SecurityGroups: []ec2types.SecurityGroup{
					{
						GroupId:   aws.String("test-sg-3"),
						GroupName: aws.String("test-sgName-3"),
						Tags: []ec2types.Tag{
							{Key: aws.String("Name"), Value: aws.String("test-security-group-3")},
						},
					},
				},
			},
		)
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
			{
				GroupId:   aws.String("test-sg-3"),
				GroupName: aws.String("test-sgName-3"),
			},
		}, securityGroups)
		Expect(awsEnv.EC2API.DescribeSecurityGroupsBehavior.Calls()).To(Equal(3))
	})
})

func describeSecurityGroupsFilters() [][]ec2types.Filter {
	var filters [][]ec2types.Filter
	awsEnv.EC2API.DescribeSecurityGroupsBehavior.CalledWithInput.ForEach(func(input *ec2.DescribeSecurityGroupsInput) {
		filters = append(filters, input.Filters)
	})
	return filters
}

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
