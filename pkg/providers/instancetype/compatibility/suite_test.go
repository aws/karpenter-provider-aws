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
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype/compatibility"
	"github.com/aws/karpenter-provider-aws/pkg/providers/placementgroup"
)

func TestCompatibility(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Compatibility")
}

var _ = Describe("CompatibilityTest", func() {
	Context("AMIFamilyCompatibility", func() {
		DescribeTable("should handle various instance types across different AMI families",
			func(instanceType string, amiFamily string, expected bool) {
				info := makeInstanceTypeInfo(instanceType, nil)
				nc := newMockNodeClass(amiFamily, nil)
				result := compatibility.IsCompatibleWithNodeClass(info, nc, nil)
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
	Context("NetworkInterfaceCompatibility", func() {
		DescribeTable("should validate network interface compatibility with instance types",
			func(networkInterfaces []*v1.NetworkInterface, networkInfo *ec2types.NetworkInfo, expected bool) {
				info := makeInstanceTypeInfo("", networkInfo)
				nc := newMockNodeClass(v1.AMIFamilyAL2023, networkInterfaces)
				result := compatibility.IsCompatibleWithNodeClass(info, nc, nil)
				Expect(result).To(Equal(expected))
			},
			Entry("compatible instance with EFA support and single EFA interface",
				[]*v1.NetworkInterface{
					{NetworkCardIndex: 0, DeviceIndex: 0, InterfaceType: v1.InterfaceTypeInterface},
					{NetworkCardIndex: 0, DeviceIndex: 1, InterfaceType: v1.InterfaceTypeEFAOnly},
				},
				makeNetworkInfo(1, 2, aws.Int32(1)),
				true,
			),
			Entry("compatible instance with EFA support and multiple EFA interfaces",
				[]*v1.NetworkInterface{
					{NetworkCardIndex: 0, DeviceIndex: 0, InterfaceType: v1.InterfaceTypeInterface},
					{NetworkCardIndex: 0, DeviceIndex: 1, InterfaceType: v1.InterfaceTypeEFAOnly},
					{NetworkCardIndex: 1, DeviceIndex: 0, InterfaceType: v1.InterfaceTypeEFAOnly},
				},
				makeNetworkInfo(2, 2, aws.Int32(2)),
				true,
			),
			Entry("EFA interfaces exceed maximum supported",
				[]*v1.NetworkInterface{
					{NetworkCardIndex: 0, DeviceIndex: 0, InterfaceType: v1.InterfaceTypeInterface},
					{NetworkCardIndex: 0, DeviceIndex: 1, InterfaceType: v1.InterfaceTypeEFAOnly},
					{NetworkCardIndex: 1, DeviceIndex: 0, InterfaceType: v1.InterfaceTypeEFAOnly},
				},
				makeNetworkInfo(2, 2, aws.Int32(1)),
				false,
			),
			Entry("instance does not support with EFA",
				[]*v1.NetworkInterface{
					{NetworkCardIndex: 0, DeviceIndex: 0, InterfaceType: v1.InterfaceTypeInterface},
					{NetworkCardIndex: 0, DeviceIndex: 1, InterfaceType: v1.InterfaceTypeEFAOnly},
				},
				makeNetworkInfo(1, 4, nil),
				false,
			),
			Entry("instance does not support with ENA",
				[]*v1.NetworkInterface{{NetworkCardIndex: 0, DeviceIndex: 0, InterfaceType: v1.InterfaceTypeInterface}},
				&ec2types.NetworkInfo{
					EnaSupport:   ec2types.EnaSupportUnsupported,
					NetworkCards: []ec2types.NetworkCardInfo{{NetworkCardIndex: aws.Int32(0), MaximumNetworkInterfaces: aws.Int32(4)}},
				},
				false,
			),
			Entry("network card index exceeds available cards",
				[]*v1.NetworkInterface{{NetworkCardIndex: 1, DeviceIndex: 0, InterfaceType: v1.InterfaceTypeInterface}},
				makeNetworkInfo(1, 4, nil),
				false,
			),
			Entry("device index exceeds maximum ENIs",
				[]*v1.NetworkInterface{{NetworkCardIndex: 0, DeviceIndex: 5, InterfaceType: v1.InterfaceTypeInterface}},
				makeNetworkInfo(1, 4, nil),
				false,
			),
		)
	})
	Context("PlacementGroupCompatibility", func() {
		DescribeTable("should handle placement group strategy compatibility",
			func(instanceType string, supportedStrategies []ec2types.PlacementGroupStrategy, pg *placementgroup.PlacementGroup, expected bool) {
				info := makeInstanceTypeInfo(instanceType, nil)
				info.PlacementGroupInfo = &ec2types.PlacementGroupInfo{
					SupportedStrategies: supportedStrategies,
				}
				nc := newMockNodeClass(v1.AMIFamilyAL2023, nil)
				result := compatibility.IsCompatibleWithNodeClass(info, nc, pg)
				Expect(result).To(Equal(expected))
			},
			Entry("nil placement group", "m5.large",
				[]ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategyCluster},
				(*placementgroup.PlacementGroup)(nil), true),
			Entry("cluster strategy supported", "m5.large",
				[]ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategyCluster, ec2types.PlacementGroupStrategyPartition, ec2types.PlacementGroupStrategySpread},
				&placementgroup.PlacementGroup{Strategy: placementgroup.StrategyCluster}, true),
			Entry("cluster strategy not supported", "t3.medium",
				[]ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategyPartition, ec2types.PlacementGroupStrategySpread},
				&placementgroup.PlacementGroup{Strategy: placementgroup.StrategyCluster}, false),
			Entry("partition strategy supported", "m5.large",
				[]ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategyCluster, ec2types.PlacementGroupStrategyPartition, ec2types.PlacementGroupStrategySpread},
				&placementgroup.PlacementGroup{Strategy: placementgroup.StrategyPartition}, true),
			Entry("partition strategy not supported", "t3.medium",
				[]ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategySpread},
				&placementgroup.PlacementGroup{Strategy: placementgroup.StrategyPartition}, false),
			Entry("spread strategy supported", "m5.large",
				[]ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategySpread},
				&placementgroup.PlacementGroup{Strategy: placementgroup.StrategySpread}, true),
			Entry("spread strategy not supported", "t3.medium",
				[]ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategyCluster},
				&placementgroup.PlacementGroup{Strategy: placementgroup.StrategySpread}, false),
		)
	})
	Context("NestedVirtualizationCompatibility", func() {
		DescribeTable("should gate instance types on ProcessorInfo.SupportedFeatures when nestedVirtualization is enabled",
			func(nestedVirt *string, supportedFeatures []ec2types.SupportedAdditionalProcessorFeature, expected bool) {
				info := makeInstanceTypeInfo("", nil)
				info.ProcessorInfo.SupportedFeatures = supportedFeatures
				nc := newMockNodeClass(v1.AMIFamilyAL2023, nil)
				if nestedVirt != nil {
					nc.cpuOptions = &v1.CPUOptions{NestedVirtualization: nestedVirt}
				}
				result := compatibility.IsCompatibleWithNodeClass(info, nc, nil)
				Expect(result).To(Equal(expected))
			},
			Entry("nil CPUOptions passes regardless of features", nil, nil, true),
			Entry("nestedVirtualization=enabled + feature present", lo.ToPtr("enabled"), []ec2types.SupportedAdditionalProcessorFeature{ec2types.SupportedAdditionalProcessorFeatureNestedVirtualization}, true),
			Entry("nestedVirtualization=enabled + feature absent", lo.ToPtr("enabled"), nil, false),
			Entry("nestedVirtualization=disabled passes regardless", lo.ToPtr("disabled"), nil, true),
		)
	})
})

func newMockNodeClass(amiFamily string, networkInterfaces []*v1.NetworkInterface) *mockNodeClass {
	return &mockNodeClass{
		amiFamily:         amiFamily,
		networkInterfaces: networkInterfaces,
	}
}

type mockNodeClass struct {
	amiFamily         string
	networkInterfaces []*v1.NetworkInterface
	cpuOptions        *v1.CPUOptions
}

func (m mockNodeClass) AMIFamily() string {
	return m.amiFamily
}

func (m *mockNodeClass) NetworkInterfaces() []*v1.NetworkInterface {
	return m.networkInterfaces
}

func (m *mockNodeClass) CPUOptions() *v1.CPUOptions {
	return m.cpuOptions
}

func makeInstanceTypeInfo(instanceType string, networkInfo *ec2types.NetworkInfo) ec2types.InstanceTypeInfo {
	return ec2types.InstanceTypeInfo{
		InstanceType: lo.Ternary(instanceType == "", ec2types.InstanceTypeM5Large, ec2types.InstanceType(instanceType)),
		ProcessorInfo: &ec2types.ProcessorInfo{
			SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeX8664},
		},
		VCpuInfo: &ec2types.VCpuInfo{
			DefaultVCpus: aws.Int32(2),
		},
		MemoryInfo: &ec2types.MemoryInfo{
			SizeInMiB: aws.Int64(8192),
		},
		NetworkInfo: networkInfo,
	}
}

func makeNetworkInfo(numCards int, deviceIndeces int32, maxEfaInterfaces *int32) *ec2types.NetworkInfo {
	networkCards := make([]ec2types.NetworkCardInfo, numCards)
	for i := 0; i < numCards; i++ {
		networkCards[i] = ec2types.NetworkCardInfo{
			NetworkCardIndex:         aws.Int32(int32(i)),
			MaximumNetworkInterfaces: aws.Int32(deviceIndeces),
		}
	}
	networkInfo := &ec2types.NetworkInfo{
		EnaSupport:   ec2types.EnaSupportSupported,
		NetworkCards: networkCards,
	}
	if maxEfaInterfaces != nil {
		networkInfo.EfaInfo = &ec2types.EfaInfo{
			MaximumEfaInterfaces: maxEfaInterfaces,
		}
	}
	return networkInfo
}
