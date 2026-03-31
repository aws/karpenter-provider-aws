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

package placementgroup_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/placementgroup"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var env *coretest.Environment
var awsEnv *test.Environment

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "PlacementGroup")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(
		coretest.WithCRDs(test.DisableCapacityReservationIDValidation(test.RemoveNodeClassTagValidation(apis.CRDs))...),
		coretest.WithCRDs(v1alpha1.CRDs...),
	)
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	awsEnv = test.NewEnvironment(ctx, env)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	awsEnv.Reset()
})

func nodeClassWithSelector(selector *v1.PlacementGroupSelectorTerm) *v1.EC2NodeClass {
	return &v1.EC2NodeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-nodeclass",
		},
		Spec: v1.EC2NodeClassSpec{
			PlacementGroupSelector: selector,
		},
	}
}

var _ = Describe("Query", func() {
	It("should use GroupNames when query specifies Name", func() {
		q := &placementgroup.Query{Name: "my-placement-group"}
		input := q.DescribePlacementGroupsInput()
		Expect(input.GroupNames).To(ConsistOf("my-placement-group"))
		Expect(input.GroupIds).To(BeEmpty())
		Expect(input.Filters).To(HaveLen(1))
		Expect(aws.ToString(input.Filters[0].Name)).To(Equal("state"))
		Expect(input.Filters[0].Values).To(ConsistOf(string(ec2types.PlacementGroupStateAvailable)))
	})
	It("should use GroupIds when query specifies ID", func() {
		q := &placementgroup.Query{ID: "pg-0123456789abcdef0"}
		input := q.DescribePlacementGroupsInput()
		Expect(input.GroupIds).To(ConsistOf("pg-0123456789abcdef0"))
		Expect(input.GroupNames).To(BeEmpty())
		Expect(input.Filters).To(HaveLen(1))
		Expect(aws.ToString(input.Filters[0].Name)).To(Equal("state"))
		Expect(input.Filters[0].Values).To(ConsistOf(string(ec2types.PlacementGroupStateAvailable)))
	})
	It("should use GroupNames even when name has pg- prefix", func() {
		q := &placementgroup.Query{Name: "pg-mygroup"}
		input := q.DescribePlacementGroupsInput()
		Expect(input.GroupNames).To(ConsistOf("pg-mygroup"))
		Expect(input.GroupIds).To(BeEmpty())
	})
	It("should produce consistent cache keys for the same query", func() {
		q1 := &placementgroup.Query{Name: "my-pg"}
		q2 := &placementgroup.Query{Name: "my-pg"}
		Expect(q1.CacheKey()).To(Equal(q2.CacheKey()))
	})
	It("should produce different cache keys for different queries", func() {
		q1 := &placementgroup.Query{Name: "my-pg"}
		q2 := &placementgroup.Query{ID: "pg-123"}
		Expect(q1.CacheKey()).ToNot(Equal(q2.CacheKey()))
	})
})

var _ = Describe("Placement Group Provider", func() {
	var clusterPG ec2types.PlacementGroup

	BeforeEach(func() {
		clusterPG = ec2types.PlacementGroup{
			GroupId:   lo.ToPtr("pg-cluster123"),
			GroupName: lo.ToPtr("my-cluster-pg"),
			State:     ec2types.PlacementGroupStateAvailable,
			Strategy:  ec2types.PlacementStrategyCluster,
		}
		awsEnv.EC2API.DescribePlacementGroupsOutput.Set(&ec2.DescribePlacementGroupsOutput{
			PlacementGroups: []ec2types.PlacementGroup{clusterPG},
		})
	})

	It("should return a placement group by name from the EC2 API", func() {
		nc := nodeClassWithSelector(&v1.PlacementGroupSelectorTerm{Name: "my-cluster-pg"})
		pg, err := awsEnv.PlacementGroupProvider.Get(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pg).ToNot(BeNil())
		Expect(pg.ID).To(Equal("pg-cluster123"))
		Expect(pg.Name).To(Equal("my-cluster-pg"))
		Expect(pg.Strategy).To(Equal(placementgroup.StrategyCluster))
	})
	It("should return a placement group by ID from the EC2 API", func() {
		nc := nodeClassWithSelector(&v1.PlacementGroupSelectorTerm{ID: "pg-cluster123"})
		pg, err := awsEnv.PlacementGroupProvider.Get(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pg).ToNot(BeNil())
		Expect(pg.ID).To(Equal("pg-cluster123"))
	})
	It("should return nil when placement group is not found", func() {
		awsEnv.EC2API.DescribePlacementGroupsOutput.Set(&ec2.DescribePlacementGroupsOutput{
			PlacementGroups: []ec2types.PlacementGroup{},
		})
		nc := nodeClassWithSelector(&v1.PlacementGroupSelectorTerm{Name: "nonexistent"})
		pg, err := awsEnv.PlacementGroupProvider.Get(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pg).To(BeNil())
	})
	It("should return nil when no matching placement group is found by name", func() {
		nc := nodeClassWithSelector(&v1.PlacementGroupSelectorTerm{Name: "does-not-exist"})
		pg, err := awsEnv.PlacementGroupProvider.Get(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pg).To(BeNil())
	})
	It("should return an error when EC2 API returns a non-not-found error", func() {
		awsEnv.EC2API.NextError.Set(fmt.Errorf("InternalError: something went wrong"))
		nc := nodeClassWithSelector(&v1.PlacementGroupSelectorTerm{Name: "my-cluster-pg"})
		pg, err := awsEnv.PlacementGroupProvider.Get(ctx, nc)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("describing placement groups"))
		Expect(pg).To(BeNil())
	})
	It("should cache results and return from cache on subsequent calls", func() {
		nc := nodeClassWithSelector(&v1.PlacementGroupSelectorTerm{Name: "my-cluster-pg"})
		pg1, err := awsEnv.PlacementGroupProvider.Get(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pg1).ToNot(BeNil())

		awsEnv.EC2API.DescribePlacementGroupsOutput.Set(&ec2.DescribePlacementGroupsOutput{
			PlacementGroups: []ec2types.PlacementGroup{},
		})

		pg2, err := awsEnv.PlacementGroupProvider.Get(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pg2).ToNot(BeNil())
		Expect(pg2.ID).To(Equal("pg-cluster123"))
	})
	It("should not return a cached entry for a different selector", func() {
		nc1 := nodeClassWithSelector(&v1.PlacementGroupSelectorTerm{Name: "my-cluster-pg"})
		pg1, err := awsEnv.PlacementGroupProvider.Get(ctx, nc1)
		Expect(err).ToNot(HaveOccurred())
		Expect(pg1).ToNot(BeNil())

		nc2 := nodeClassWithSelector(&v1.PlacementGroupSelectorTerm{Name: "other-pg"})
		pg2, err := awsEnv.PlacementGroupProvider.Get(ctx, nc2)
		Expect(err).ToNot(HaveOccurred())
		Expect(pg2).To(BeNil())
	})
	It("should return nil when the output has no placement groups", func() {
		awsEnv.EC2API.DescribePlacementGroupsOutput.Set(&ec2.DescribePlacementGroupsOutput{
			PlacementGroups: []ec2types.PlacementGroup{},
		})
		nc := nodeClassWithSelector(&v1.PlacementGroupSelectorTerm{Name: "empty"})
		pg, err := awsEnv.PlacementGroupProvider.Get(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pg).To(BeNil())
	})
	It("should return nil when no placement group selector is set", func() {
		nc := nodeClassWithSelector(nil)
		pg, err := awsEnv.PlacementGroupProvider.Get(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pg).To(BeNil())
	})
	It("should return the first placement group when multiple are returned", func() {
		awsEnv.EC2API.DescribePlacementGroupsOutput.Set(&ec2.DescribePlacementGroupsOutput{
			PlacementGroups: []ec2types.PlacementGroup{
				{
					GroupId:   lo.ToPtr("pg-first"),
					GroupName: lo.ToPtr("first-pg"),
					State:     ec2types.PlacementGroupStateAvailable,
					Strategy:  ec2types.PlacementStrategyCluster,
				},
				{
					GroupId:   lo.ToPtr("pg-second"),
					GroupName: lo.ToPtr("second-pg"),
					State:     ec2types.PlacementGroupStateAvailable,
					Strategy:  ec2types.PlacementStrategySpread,
				},
			},
		})
		nc := nodeClassWithSelector(&v1.PlacementGroupSelectorTerm{Name: "first-pg"})
		pg, err := awsEnv.PlacementGroupProvider.Get(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pg).ToNot(BeNil())
		Expect(pg.ID).To(Equal("pg-first"))
	})
	It("should store resolved placement group for GetForNodeClass", func() {
		nc := nodeClassWithSelector(&v1.PlacementGroupSelectorTerm{Name: "my-cluster-pg"})
		pg, err := awsEnv.PlacementGroupProvider.Get(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pg).ToNot(BeNil())

		resolved := awsEnv.PlacementGroupProvider.GetForNodeClass(nc)
		Expect(resolved).ToNot(BeNil())
		Expect(resolved.ID).To(Equal("pg-cluster123"))
	})
	It("should clear resolved placement group on Clear", func() {
		nc := nodeClassWithSelector(&v1.PlacementGroupSelectorTerm{Name: "my-cluster-pg"})
		_, err := awsEnv.PlacementGroupProvider.Get(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(awsEnv.PlacementGroupProvider.GetForNodeClass(nc)).ToNot(BeNil())

		awsEnv.PlacementGroupProvider.Clear(nc)
		Expect(awsEnv.PlacementGroupProvider.GetForNodeClass(nc)).To(BeNil())
	})
})
