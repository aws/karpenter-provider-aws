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

package compatibility_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype/compatibility"
)

func TestCompatibility(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Compatibility")
}

var _ = Describe("CompatibilityTest", func() {
	Context("AMIFamilyCompatibility", func() {
		DescribeTable("should handle various instance types across different AMI families",
			func(instanceType string, amiFamily string, expected bool) {
				info := makeInstanceTypeInfo(instanceType)
				nc := newMockNodeClass(amiFamily, nil)
				result := compatibility.IsCompatibleWithNodeClass(info, nc)
				Expect(result).To(Equal(expected))
			},
			Entry("a1.medium w/ Custom AMI", "a1.medium", v1.AMIFamilyCustom, true),
			Entry("a1.large w/ Custom AMI", "a1.large", v1.AMIFamilyCustom, true),
			Entry("t3.medium w/ Custom AMI", "t3.medium", v1.AMIFamilyCustom, true),
			Entry("a1.medium w/ AL2023 AMI", "a1.medium", v1.AMIFamilyAL2023, false),
			Entry("t3.medium w/ AL2023 AMI", "t3.medium", v1.AMIFamilyAL2023, true),
			Entry("a1.large w/ Bottlerocket", "a1.large", v1.AMIFamilyBottlerocket, true),
		)
	})
	Context("PlacementGroupCompatibility", func() {
		DescribeTable("should handle placement group strategy compatibility",
			func(instanceType string, supportedStrategies []ec2types.PlacementGroupStrategy, placementGroups []v1.PlacementGroup, expected bool) {
				info := makeInstanceTypeInfo(instanceType)
				info.PlacementGroupInfo = &ec2types.PlacementGroupInfo{
					SupportedStrategies: supportedStrategies,
				}
				nc := newMockNodeClass(v1.AMIFamilyAL2023, placementGroups)
				result := compatibility.IsCompatibleWithNodeClass(info, nc)
				Expect(result).To(Equal(expected))
			},
			Entry("nil placement groups", "m5.large",
				[]ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategyCluster},
				nil, true),
			Entry("empty placement groups", "m5.large",
				[]ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategyCluster},
				[]v1.PlacementGroup{}, true),
			Entry("cluster strategy supported", "m5.large",
				[]ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategyCluster, ec2types.PlacementGroupStrategyPartition, ec2types.PlacementGroupStrategySpread},
				[]v1.PlacementGroup{{Strategy: v1.PlacementGroupStrategyCluster}}, true),
			Entry("cluster strategy not supported", "t3.medium",
				[]ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategyPartition, ec2types.PlacementGroupStrategySpread},
				[]v1.PlacementGroup{{Strategy: v1.PlacementGroupStrategyCluster}}, false),
			Entry("partition strategy supported", "m5.large",
				[]ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategyCluster, ec2types.PlacementGroupStrategyPartition, ec2types.PlacementGroupStrategySpread},
				[]v1.PlacementGroup{{Strategy: v1.PlacementGroupStrategyPartition}}, true),
			Entry("partition strategy not supported", "t3.medium",
				[]ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategySpread},
				[]v1.PlacementGroup{{Strategy: v1.PlacementGroupStrategyPartition}}, false),
			Entry("spread strategy supported", "m5.large",
				[]ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategySpread},
				[]v1.PlacementGroup{{Strategy: v1.PlacementGroupStrategySpread}}, true),
			Entry("spread strategy not supported", "t3.medium",
				[]ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategyCluster},
				[]v1.PlacementGroup{{Strategy: v1.PlacementGroupStrategySpread}}, false),
		)
	})
})

func newMockNodeClass(amiFamily string, placementGroups []v1.PlacementGroup) *mockNodeClass {
	return &mockNodeClass{
		amiFamily:       amiFamily,
		placementGroups: placementGroups,
	}
}

type mockNodeClass struct {
	amiFamily       string
	placementGroups []v1.PlacementGroup
}

func (m mockNodeClass) AMIFamily() string {
	return m.amiFamily
}

func (m mockNodeClass) PlacementGroups() []v1.PlacementGroup {
	return m.placementGroups
}

func makeInstanceTypeInfo(instanceType string) ec2types.InstanceTypeInfo {
	return ec2types.InstanceTypeInfo{
		InstanceType: ec2types.InstanceType(instanceType),
		ProcessorInfo: &ec2types.ProcessorInfo{
			SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeArm64},
		},
		VCpuInfo: &ec2types.VCpuInfo{
			DefaultVCpus: aws.Int32(2),
		},
		MemoryInfo: &ec2types.MemoryInfo{
			SizeInMiB: aws.Int64(4096),
		},
	}
}
