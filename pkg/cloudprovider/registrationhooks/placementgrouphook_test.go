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

package registrationhooks_test

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"

	opstatus "github.com/awslabs/operatorpkg/status"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider/registrationhooks"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/providers/placementgroup"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

// setupPlacementGroupInProvider configures the EC2 mock and resolves the placement group into the provider's in-memory store.
func setupPlacementGroupInProvider(nodeClass *v1.EC2NodeClass, pgOutput *ec2.DescribePlacementGroupsOutput) {
	awsEnv.EC2API.DescribePlacementGroupsOutput.Set(pgOutput)
	_, _ = awsEnv.PlacementGroupProvider.Get(ctx, nodeClass)
}

var _ = Describe("PlacementGroupRegistrationHook", func() {
	var hook *registrationhooks.PlacementGroupRegistrationHook
	var nodeClass *v1.EC2NodeClass
	var nc *karpv1.NodeClaim

	BeforeEach(func() {
		hook = registrationhooks.NewPlacementGroupRegistrationHook(awsEnv.InstanceProvider)
		nodeClass = test.EC2NodeClass(v1.EC2NodeClass{
			Status: v1.EC2NodeClassStatus{
				InstanceProfile: "test-profile",
				SecurityGroups: []v1.SecurityGroup{
					{ID: "sg-test1", Name: "securityGroup-test1"},
				},
				Subnets: []v1.Subnet{
					{ID: "subnet-test1", Zone: "test-zone-1a", ZoneID: "tstz1-1a"},
				},
			},
		})
		nodeClass.StatusConditions().SetTrue(opstatus.ConditionReady)
		nc = coretest.NodeClaim(karpv1.NodeClaim{
			Spec: karpv1.NodeClaimSpec{
				NodeClassRef: &karpv1.NodeClassReference{
					Group: "karpenter.k8s.aws",
					Kind:  "EC2NodeClass",
					Name:  nodeClass.Name,
				},
			},
		})
		nc.Labels = map[string]string{}
	})

	It("should pass through immediately when EC2NodeClass has no placement group", func() {
		// No placement group selector set
		ExpectApplied(ctx, env.Client, nodeClass, nc)

		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
	})

	It("should pass through immediately when EC2NodeClass has a cluster placement group", func() {
		nodeClass.Spec.PlacementGroupSelector = &v1.PlacementGroupSelector{Name: "cluster-pg"}
		setupPlacementGroupInProvider(nodeClass, &ec2.DescribePlacementGroupsOutput{
			PlacementGroups: []ec2types.PlacementGroup{{
				GroupId:   lo.ToPtr("pg-cluster123"),
				GroupName: lo.ToPtr("cluster-pg"),
				State:     ec2types.PlacementGroupStateAvailable,
				Strategy:  ec2types.PlacementStrategyCluster,
			}},
		})
		ExpectApplied(ctx, env.Client, nodeClass, nc)

		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
	})

	It("should pass through immediately when EC2NodeClass has a spread placement group", func() {
		nodeClass.Spec.PlacementGroupSelector = &v1.PlacementGroupSelector{Name: "spread-pg"}
		setupPlacementGroupInProvider(nodeClass, &ec2.DescribePlacementGroupsOutput{
			PlacementGroups: []ec2types.PlacementGroup{{
				GroupId:     lo.ToPtr("pg-spread123"),
				GroupName:   lo.ToPtr("spread-pg"),
				State:       ec2types.PlacementGroupStateAvailable,
				Strategy:    ec2types.PlacementStrategySpread,
				SpreadLevel: ec2types.SpreadLevelRack,
			}},
		})
		ExpectApplied(ctx, env.Client, nodeClass, nc)

		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
	})

	It("should pass through when partition label is already set", func() {
		nc.Labels[v1.LabelPlacementGroupID] = "pg-partition123"
		nc.Labels[v1.LabelPlacementGroupPartition] = "3"
		ExpectApplied(ctx, env.Client, nodeClass, nc)

		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
	})

	It("should block registration when providerID is empty for partition placement group", func() {
		nc.Labels[v1.LabelPlacementGroupID] = "pg-partition123"
		nc.Status.ProviderID = ""
		ExpectApplied(ctx, env.Client, nodeClass, nc)

		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeTrue())
	})

	It("should set partition label and proceed when DescribeInstances returns partition number", func() {
		instanceID := fake.InstanceID()
		nc.Labels[v1.LabelPlacementGroupID] = "pg-partition123"
		nc.Status.ProviderID = fmt.Sprintf("aws:///test-zone-1a/%s", instanceID)
		ExpectApplied(ctx, env.Client, nodeClass, nc)

		awsEnv.EC2API.Instances.Store(instanceID, ec2types.Instance{
			InstanceId:   lo.ToPtr(instanceID),
			InstanceType: "m5.large",
			Placement: &ec2types.Placement{
				AvailabilityZone: lo.ToPtr("test-zone-1a"),
				PartitionNumber:  lo.ToPtr[int32](3),
			},
			State: &ec2types.InstanceState{
				Name: ec2types.InstanceStateNameRunning,
			},
		})

		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
		Expect(nc.Labels[v1.LabelPlacementGroupPartition]).To(Equal("3"))
	})

	It("should pass through when instance has no partition number (not a partition PG)", func() {
		instanceID := fake.InstanceID()
		nc.Labels[v1.LabelPlacementGroupID] = "pg-cluster123"
		nc.Status.ProviderID = fmt.Sprintf("aws:///test-zone-1a/%s", instanceID)
		ExpectApplied(ctx, env.Client, nodeClass, nc)

		awsEnv.EC2API.Instances.Store(instanceID, ec2types.Instance{
			InstanceId:   lo.ToPtr(instanceID),
			InstanceType: "m5.large",
			Placement: &ec2types.Placement{
				AvailabilityZone: lo.ToPtr("test-zone-1a"),
			},
			State: &ec2types.InstanceState{
				Name: ec2types.InstanceStateNameRunning,
			},
		})

		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
	})

	It("should return error when instance is not found", func() {
		instanceID := fake.InstanceID()
		nc.Labels[v1.LabelPlacementGroupID] = "pg-partition123"
		nc.Status.ProviderID = fmt.Sprintf("aws:///test-zone-1a/%s", instanceID)
		ExpectApplied(ctx, env.Client, nodeClass, nc)

		_, err := hook.Registered(ctx, nc)
		Expect(err).To(HaveOccurred())
	})

	It("should handle multiple partition numbers correctly", func() {
		for _, partitionNum := range []int32{1, 4, 7} {
			instanceID := fake.InstanceID()
			testNC := coretest.NodeClaim(karpv1.NodeClaim{
				Spec: karpv1.NodeClaimSpec{
					NodeClassRef: &karpv1.NodeClassReference{
						Group: "karpenter.k8s.aws",
						Kind:  "EC2NodeClass",
						Name:  nodeClass.Name,
					},
				},
			})
			testNC.Labels = map[string]string{
				v1.LabelPlacementGroupID: "pg-partition123",
			}
			testNC.Status.ProviderID = fmt.Sprintf("aws:///test-zone-1a/%s", instanceID)
			ExpectApplied(ctx, env.Client, nodeClass, testNC)

			awsEnv.EC2API.Instances.Store(instanceID, ec2types.Instance{
				InstanceId:   lo.ToPtr(instanceID),
				InstanceType: "m5.large",
				Placement: &ec2types.Placement{
					AvailabilityZone: lo.ToPtr("test-zone-1a"),
					PartitionNumber:  lo.ToPtr(partitionNum),
				},
				State: &ec2types.InstanceState{
					Name: ec2types.InstanceStateNameRunning,
				},
			})

			result, err := hook.Registered(ctx, testNC)
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(testNC.Labels[v1.LabelPlacementGroupPartition]).To(Equal(fmt.Sprintf("%d", partitionNum)))
		}
	})
})

// Ensure the provider's PlacementGroup type is used (compile-time check)
var _ = placementgroup.StrategyPartition
