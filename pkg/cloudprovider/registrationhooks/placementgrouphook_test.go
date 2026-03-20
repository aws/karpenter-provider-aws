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

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"

	opstatus "github.com/awslabs/operatorpkg/status"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider/registrationhooks"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("PlacementGroupRegistrationHook", func() {
	var hook *registrationhooks.PlacementGroupRegistrationHook
	var nodeClass *v1.EC2NodeClass
	var nc *karpv1.NodeClaim

	BeforeEach(func() {
		hook = registrationhooks.NewPlacementGroupRegistrationHook(env.Client, awsEnv.InstanceProvider)
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
		nodeClass.Status.PlacementGroups = nil
		ExpectApplied(ctx, env.Client, nodeClass, nc)

		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
	})

	It("should pass through immediately when EC2NodeClass has a cluster placement group", func() {
		nodeClass.Status.PlacementGroups = []v1.PlacementGroup{
			{
				ID:       "pg-cluster123",
				Name:     "cluster-pg",
				Strategy: v1.PlacementGroupStrategyCluster,
				State:    v1.PlacementGroupStateAvailable,
			},
		}
		ExpectApplied(ctx, env.Client, nodeClass, nc)

		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
	})

	It("should pass through immediately when EC2NodeClass has a spread placement group", func() {
		nodeClass.Status.PlacementGroups = []v1.PlacementGroup{
			{
				ID:          "pg-spread123",
				Name:        "spread-pg",
				Strategy:    v1.PlacementGroupStrategySpread,
				SpreadLevel: v1.PlacementGroupSpreadLevelRack,
				State:       v1.PlacementGroupStateAvailable,
			},
		}
		ExpectApplied(ctx, env.Client, nodeClass, nc)

		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
	})

	It("should pass through when partition label is already set", func() {
		nodeClass.Status.PlacementGroups = []v1.PlacementGroup{
			{
				ID:             "pg-partition123",
				Name:           "partition-pg",
				PartitionCount: 7,
				Strategy:       v1.PlacementGroupStrategyPartition,
				State:          v1.PlacementGroupStateAvailable,
			},
		}
		nc.Labels[v1.LabelPlacementGroupPartition] = "3"
		ExpectApplied(ctx, env.Client, nodeClass, nc)

		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
	})

	It("should block registration when providerID is empty for partition placement group", func() {
		nodeClass.Status.PlacementGroups = []v1.PlacementGroup{
			{
				ID:             "pg-partition123",
				Name:           "partition-pg",
				PartitionCount: 7,
				Strategy:       v1.PlacementGroupStrategyPartition,
				State:          v1.PlacementGroupStateAvailable,
			},
		}
		nc.Status.ProviderID = ""
		ExpectApplied(ctx, env.Client, nodeClass, nc)

		result, err := hook.Registered(ctx, nc)
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeTrue())
	})

	It("should set partition label and proceed when DescribeInstances returns partition number", func() {
		instanceID := fake.InstanceID()
		nodeClass.Status.PlacementGroups = []v1.PlacementGroup{
			{
				ID:             "pg-partition123",
				Name:           "partition-pg",
				PartitionCount: 7,
				Strategy:       v1.PlacementGroupStrategyPartition,
				State:          v1.PlacementGroupStateAvailable,
			},
		}
		nc.Status.ProviderID = fmt.Sprintf("aws:///test-zone-1a/%s", instanceID)
		ExpectApplied(ctx, env.Client, nodeClass, nc)
		nc.Labels = map[string]string{}

		// Configure the EC2 API to return an instance with a partition number
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

	It("should block registration when instance has no partition number yet", func() {
		instanceID := fake.InstanceID()
		nodeClass.Status.PlacementGroups = []v1.PlacementGroup{
			{
				ID:             "pg-partition123",
				Name:           "partition-pg",
				PartitionCount: 7,
				Strategy:       v1.PlacementGroupStrategyPartition,
				State:          v1.PlacementGroupStateAvailable,
			},
		}
		nc.Status.ProviderID = fmt.Sprintf("aws:///test-zone-1a/%s", instanceID)
		ExpectApplied(ctx, env.Client, nodeClass, nc)

		// Configure the EC2 API to return an instance without a partition number
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
		Expect(result.Requeue).To(BeTrue())
	})

	It("should return error when instance is not found", func() {
		instanceID := fake.InstanceID()
		nodeClass.Status.PlacementGroups = []v1.PlacementGroup{
			{
				ID:             "pg-partition123",
				Name:           "partition-pg",
				PartitionCount: 7,
				Strategy:       v1.PlacementGroupStrategyPartition,
				State:          v1.PlacementGroupStateAvailable,
			},
		}
		nc.Status.ProviderID = fmt.Sprintf("aws:///test-zone-1a/%s", instanceID)
		ExpectApplied(ctx, env.Client, nodeClass, nc)

		// Don't store any instance — the instance won't be found

		_, err := hook.Registered(ctx, nc)
		Expect(err).To(HaveOccurred())
	})

	It("should handle multiple partition numbers correctly", func() {
		for _, partitionNum := range []int32{1, 4, 7} {
			instanceID := fake.InstanceID()
			nodeClass.Status.PlacementGroups = []v1.PlacementGroup{
				{
					ID:             "pg-partition123",
					Name:           "partition-pg",
					PartitionCount: 7,
					Strategy:       v1.PlacementGroupStrategyPartition,
					State:          v1.PlacementGroupStateAvailable,
				},
			}

			testNC := coretest.NodeClaim(karpv1.NodeClaim{
				Spec: karpv1.NodeClaimSpec{
					NodeClassRef: &karpv1.NodeClassReference{
						Group: "karpenter.k8s.aws",
						Kind:  "EC2NodeClass",
						Name:  nodeClass.Name,
					},
				},
			})
			testNC.Labels = map[string]string{}
			testNC.Status.ProviderID = fmt.Sprintf("aws:///test-zone-1a/%s", instanceID)
			ExpectApplied(ctx, env.Client, nodeClass, testNC)
			testNC.Labels = map[string]string{}

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
