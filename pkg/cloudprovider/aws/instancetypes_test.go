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

package aws_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	cloudprovideraws "github.com/awslabs/karpenter/pkg/cloudprovider/aws"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/fake"
	. "github.com/onsi/gomega"
)

var (
	instanceTypeMocks = map[string]*ec2.InstanceTypeInfo{
		"m5.large": {
			InstanceType:                  aws.String("m5.large"),
			SupportedUsageClasses:         []*string{aws.String("on-demand")},
			BurstablePerformanceSupported: aws.Bool(false),
			ProcessorInfo: &ec2.ProcessorInfo{
				SupportedArchitectures: aws.StringSlice([]string{"x86_64"}),
			},
		},
		"m6g.large": {
			InstanceType:                  aws.String("m6g.large"),
			SupportedUsageClasses:         []*string{aws.String("on-demand")},
			BurstablePerformanceSupported: aws.Bool(false),
			ProcessorInfo: &ec2.ProcessorInfo{
				SupportedArchitectures: aws.StringSlice([]string{"arm64"}),
			},
		},
	}
	defaultArch = "amd64"
)

func TestGet_InstanceTypes(t *testing.T) {
	// Setup
	g := NewWithT(t)
	clusterName := "test-cluster"
	testZone := "test-zone"
	ec2, vpcProvider := getInstanceTypeProviderMocks([]string{testZone}, []string{"m5.large"})
	instanceTypeProvider := cloudprovideraws.NewInstanceTypeProvider(ec2, &vpcProvider)
	constraints := &cloudprovider.Constraints{}
	constraints.Architecture = &v1alpha1.ArchitectureAmd64

	// iterate twice to ensure cache miss works the same as a cache hit
	for range []int{0, 1} {
		// Test
		zonalInstanceTypes, err := instanceTypeProvider.Get(context.Background(), clusterName, constraints)

		// Assertions
		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(len(zonalInstanceTypes)).Should(Equal(1))

		instanceTypes := zonalInstanceTypes[testZone]
		g.Expect(len(instanceTypes)).Should(Equal(1))
		g.Expect(*instanceTypes[0].InstanceType).Should(Equal("m5.large"))
	}
}

func TestGet_InstanceTypesFilteredByARM64(t *testing.T) {
	// Setup
	g := NewWithT(t)
	clusterName := "test-cluster"
	testZone := "test-zone"
	ec2, vpcProvider := getInstanceTypeProviderMocks([]string{testZone}, []string{"m5.large"})
	instanceTypeProvider := cloudprovideraws.NewInstanceTypeProvider(ec2, &vpcProvider)
	constraints := &cloudprovider.Constraints{}
	constraints.Architecture = &v1alpha1.ArchitectureArm64

	// Test
	zonalInstanceTypes, err := instanceTypeProvider.Get(context.Background(), clusterName, constraints)

	// Assertions
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(len(zonalInstanceTypes)).Should(Equal(1))

	instanceTypes := zonalInstanceTypes[testZone]
	g.Expect(len(instanceTypes)).Should(Equal(0))
}

func TestGet_InstanceTypesFilteredByInstanceType(t *testing.T) {
	//Setup
	g := NewWithT(t)
	clusterName := "test-cluster"
	testZone := "test-zone"
	ec2, vpcProvider := getInstanceTypeProviderMocks([]string{testZone}, []string{"m5.large"})
	instanceTypeProvider := cloudprovideraws.NewInstanceTypeProvider(ec2, &vpcProvider)
	constraints := &cloudprovider.Constraints{}
	constraints.Architecture = &defaultArch
	constraints.InstanceTypes = append(constraints.InstanceTypes, "m5.large")

	// Test
	zonalInstanceTypes, err := instanceTypeProvider.Get(context.Background(), clusterName, constraints)

	// Assertions
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(len(zonalInstanceTypes)).Should(Equal(1))

	instanceTypes := zonalInstanceTypes[testZone]
	g.Expect(len(instanceTypes)).Should(Equal(1))
	g.Expect(*instanceTypes[0].InstanceType).Should(Equal("m5.large"))
}

func TestGet_InstanceTypesFilteredByZoneID(t *testing.T) {
	// Setup
	g := NewWithT(t)
	clusterName := "test-cluster"
	testZone := "test-zone"
	testZoneID := fmt.Sprintf("%s-id", testZone)
	ec2, vpcProvider := getInstanceTypeProviderMocks([]string{testZone}, []string{"m5.large"})
	instanceTypeProvider := cloudprovideraws.NewInstanceTypeProvider(ec2, &vpcProvider)
	constraints := &cloudprovider.Constraints{}
	constraints.Architecture = &defaultArch
	constraints.Zones = []string{testZoneID}

	// Test
	zonalInstanceTypes, err := instanceTypeProvider.Get(context.Background(), clusterName, constraints)

	// Assertions
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(len(zonalInstanceTypes)).Should(Equal(1))

	instanceTypes := zonalInstanceTypes[testZone]
	g.Expect(len(instanceTypes)).Should(Equal(1))
	g.Expect(*instanceTypes[0].InstanceType).Should(Equal("m5.large"))
}

func TestUniqueInstanceTypesFrom(t *testing.T) {
	// Setup
	g := NewWithT(t)
	ec2api, vpcProvider := getInstanceTypeProviderMocks([]string{}, []string{})
	instanceTypeProvider := cloudprovideraws.NewInstanceTypeProvider(ec2api, &vpcProvider)
	instancePools := map[string][]*ec2.InstanceTypeInfo{
		"test-zone1": {instanceTypeMocks["m5.large"], instanceTypeMocks["m5.large"]},
		"test-zone2": {instanceTypeMocks["m5.large"], instanceTypeMocks["m6g.large"]},
	}

	// Test
	uniqueInstanceTypes := instanceTypeProvider.UniqueInstanceTypesFrom(instancePools)

	// Assertions
	g.Expect(len(uniqueInstanceTypes)).Should(Equal(2))
	instanceTypes := map[string]bool{}
	for _, it := range uniqueInstanceTypes {
		instanceTypes[*it.InstanceType] = true
	}
	g.Expect(instanceTypes["m5.large"]).Should(Equal(true))
	g.Expect(instanceTypes["m6g.large"]).Should(Equal(true))
}

func TestUniqueInstanceTypesFrom_EmptyInstancePools(t *testing.T) {
	// Setup
	g := NewWithT(t)
	ec2api, vpcProvider := getInstanceTypeProviderMocks([]string{}, []string{})
	instanceTypeProvider := cloudprovideraws.NewInstanceTypeProvider(ec2api, &vpcProvider)
	instancePools := map[string][]*ec2.InstanceTypeInfo{}

	// Test
	uniqueInstanceTypes := instanceTypeProvider.UniqueInstanceTypesFrom(instancePools)

	// Asertions
	g.Expect(len(uniqueInstanceTypes)).Should(Equal(0))
}

func TestInstanceTypesPerZoneFrom(t *testing.T) {
	// Setup
	g := NewWithT(t)
	ec2api, vpcProvider := getInstanceTypeProviderMocks([]string{}, []string{})
	instanceTypeProvider := cloudprovideraws.NewInstanceTypeProvider(ec2api, &vpcProvider)
	instancePools := map[string][]*ec2.InstanceTypeInfo{
		"test-zone1": {instanceTypeMocks["m5.large"]},
		"test-zone2": {instanceTypeMocks["m5.large"], instanceTypeMocks["m6g.large"]},
	}

	// Test
	instanceTypesPerZone := instanceTypeProvider.InstanceTypesPerZoneFrom([]string{"m5.large", "m6g.large"}, instancePools)

	// Assertions
	g.Expect(len(instanceTypesPerZone)).Should(Equal(2))
	g.Expect(*instanceTypeMocks["m5.large"].InstanceType).Should(Equal(*instanceTypesPerZone["test-zone1"][0].InstanceType))
	g.Expect(*instanceTypeMocks["m5.large"].InstanceType).Should(Equal(*instanceTypesPerZone["test-zone2"][0].InstanceType))
	g.Expect(*instanceTypeMocks["m6g.large"].InstanceType).Should(Equal(*instanceTypesPerZone["test-zone2"][1].InstanceType))
}

func TestInstanceTypesPerZoneFrom_EmptyZoneMapping(t *testing.T) {
	// Setup
	g := NewWithT(t)
	ec2api, vpcProvider := getInstanceTypeProviderMocks([]string{}, []string{})
	instanceTypeProvider := cloudprovideraws.NewInstanceTypeProvider(ec2api, &vpcProvider)
	instancePools := map[string][]*ec2.InstanceTypeInfo{
		"test-zone1": {instanceTypeMocks["m5.large"]},
		"test-zone2": {instanceTypeMocks["m5.large"], instanceTypeMocks["m6g.large"]},
	}

	// Test
	instanceTypesPerZone := instanceTypeProvider.InstanceTypesPerZoneFrom([]string{}, instancePools)

	// Assertions
	g.Expect(len(instanceTypesPerZone)).Should(Equal(0))
}

func TestInstanceTypesPerZoneFrom_EmptyInstanceTypes(t *testing.T) {
	// Setup
	g := NewWithT(t)
	ec2api, vpcProvider := getInstanceTypeProviderMocks([]string{}, []string{})
	instanceTypeProvider := cloudprovideraws.NewInstanceTypeProvider(ec2api, &vpcProvider)
	instancePools := map[string][]*ec2.InstanceTypeInfo{}

	// Test
	instanceTypesPerZone := instanceTypeProvider.InstanceTypesPerZoneFrom([]string{"m5.large", "m6g.large"}, instancePools)

	// Assertions
	g.Expect(len(instanceTypesPerZone)).Should(Equal(0))
}

// Test Helpers

func getInstanceTypeProviderMocks(zones []string, instanceTypes []string) (ec2iface.EC2API, cloudprovideraws.VPCProvider) {
	testSubnet := "test-subnet"
	ec2api := &fake.EC2API{
		DescribeSubnetsOutput:               &ec2.DescribeSubnetsOutput{},
		DescribeAvailabilityZonesOutput:     &ec2.DescribeAvailabilityZonesOutput{},
		DescribeInstanceTypesOutput:         &ec2.DescribeInstanceTypesOutput{},
		DescribeInstanceTypeOfferingsOutput: &ec2.DescribeInstanceTypeOfferingsOutput{},
	}

	for _, instanceType := range instanceTypes {
		ec2api.DescribeInstanceTypesOutput.InstanceTypes = append(ec2api.DescribeInstanceTypesOutput.InstanceTypes, instanceTypeMocks[instanceType])
	}
	for _, zone := range zones {
		zoneId := fmt.Sprintf("%s-id", zone)
		for _, instanceType := range instanceTypes {
			offering := &ec2.InstanceTypeOffering{
				InstanceType: &instanceType,
				Location:     &zone,
			}
			ec2api.DescribeInstanceTypeOfferingsOutput.InstanceTypeOfferings = append(ec2api.DescribeInstanceTypeOfferingsOutput.InstanceTypeOfferings, offering)
			subnet := ec2.Subnet{
				SubnetId:         &testSubnet,
				AvailabilityZone: &zone,
			}
			ec2api.DescribeSubnetsOutput.Subnets = append(ec2api.DescribeSubnetsOutput.Subnets, &subnet)
			az := ec2.AvailabilityZone{
				ZoneName: &zone,
				ZoneId:   &zoneId,
			}
			ec2api.DescribeAvailabilityZonesOutput.AvailabilityZones = append(ec2api.DescribeAvailabilityZonesOutput.AvailabilityZones, &az)
		}
	}

	subnetProvider := cloudprovideraws.NewSubnetProvider(ec2api)
	vpcProvider := cloudprovideraws.NewVPCProvider(ec2api, subnetProvider)
	return ec2api, *vpcProvider
}
