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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	cloudprovideraws "github.com/awslabs/karpenter/pkg/cloudprovider/aws"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/fake"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	instanceTypeMocks = map[string]*ec2.InstanceTypeInfo{
		"m5.large": {
			InstanceType:                  aws.String("m5.large"),
			SupportedUsageClasses:         []*string{aws.String("on-demand")},
			BurstablePerformanceSupported: aws.Bool(false),
			BareMetal:                     aws.Bool(false),
			ProcessorInfo: &ec2.ProcessorInfo{
				SupportedArchitectures: aws.StringSlice([]string{"x86_64"}),
			},
		},
		"m6g.large": {
			InstanceType:                  aws.String("m6g.large"),
			SupportedUsageClasses:         []*string{aws.String("on-demand")},
			BurstablePerformanceSupported: aws.Bool(false),
			BareMetal:                     aws.Bool(false),
			ProcessorInfo: &ec2.ProcessorInfo{
				SupportedArchitectures: aws.StringSlice([]string{"arm64"}),
			},
		},
	}
	defaultArch = "amd64"
	testZone    = "test-zone"
)

var _ = Describe("InstanceTypes", func() {

	Describe("Getting Instance Types", func() {
		Context("With amd64 architecture", func() {
			ec2api := getInstanceTypeProviderMocks([]string{testZone}, []string{"m5.large"})
			instanceTypeProvider := cloudprovideraws.NewInstanceTypeProvider(ec2api)
			zonalSubnetOptions := map[string][]*ec2.Subnet{testZone: nil}
			constraints := cloudprovideraws.Constraints(cloudprovider.Constraints{})
			constraints.Architecture = &v1alpha1.ArchitectureAmd64
			instanceTypes, err := instanceTypeProvider.Get(context.Background(), zonalSubnetOptions, constraints)

			It("should return one m5.large supported in test-zone", func() {
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(instanceTypes)).Should(Equal(1))
				instanceType := instanceTypes[0]
				Expect(*instanceType.InstanceType).Should(Equal("m5.large"))
				Expect(instanceType.Zones[0]).Should(Equal(testZone))
			})
		})

		Context("With arm64 architecture", func() {
			ec2api := getInstanceTypeProviderMocks([]string{testZone}, []string{"m6g.large"})
			instanceTypeProvider := cloudprovideraws.NewInstanceTypeProvider(ec2api)
			zonalSubnetOptions := map[string][]*ec2.Subnet{testZone: nil}
			constraints := cloudprovideraws.Constraints(cloudprovider.Constraints{})
			constraints.Architecture = &v1alpha1.ArchitectureArm64
			instanceTypes, err := instanceTypeProvider.Get(context.Background(), zonalSubnetOptions, constraints)

			It("should return one m6g.large supported in test-zone", func() {
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(instanceTypes)).Should(Equal(1))
				instanceType := instanceTypes[0]
				Expect(*instanceType.InstanceType).Should(Equal("m6g.large"))
				Expect(instanceType.Zones[0]).Should(Equal(testZone))
			})
		})

		Context("With arm64 architecture but no arm64 instance types supported", func() {
			ec2api := getInstanceTypeProviderMocks([]string{testZone}, []string{"m5.large"})
			instanceTypeProvider := cloudprovideraws.NewInstanceTypeProvider(ec2api)
			zonalSubnetOptions := map[string][]*ec2.Subnet{testZone: nil}
			constraints := cloudprovideraws.Constraints(cloudprovider.Constraints{})
			constraints.Architecture = &v1alpha1.ArchitectureArm64
			instanceTypes, err := instanceTypeProvider.Get(context.Background(), zonalSubnetOptions, constraints)

			It("should not return any instance types", func() {
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(instanceTypes)).Should(Equal(0))
			})
		})

		Context("With allowed instance types constraint", func() {
			ec2api := getInstanceTypeProviderMocks([]string{testZone}, []string{"m5.large"})
			instanceTypeProvider := cloudprovideraws.NewInstanceTypeProvider(ec2api)
			zonalSubnetOptions := map[string][]*ec2.Subnet{testZone: nil}
			constraints := cloudprovideraws.Constraints(cloudprovider.Constraints{})
			constraints.Architecture = &defaultArch
			constraints.InstanceTypes = append(constraints.InstanceTypes, "m5.large")
			instanceTypes, err := instanceTypeProvider.Get(context.Background(), zonalSubnetOptions, constraints)

			It("should return one m5.large supported in test-zone", func() {
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(instanceTypes)).Should(Equal(1))
				instanceType := instanceTypes[0]
				Expect(*instanceType.InstanceType).Should(Equal("m5.large"))
				Expect(instanceType.Zones[0]).Should(Equal(testZone))
			})
		})

	})

})

// Test Helpers

func getInstanceTypeProviderMocks(zones []string, instanceTypes []string) ec2iface.EC2API {
	ec2api := &fake.EC2API{
		EC2Behavior: fake.EC2Behavior{
			DescribeInstanceTypesOutput:         &ec2.DescribeInstanceTypesOutput{},
			DescribeInstanceTypeOfferingsOutput: &ec2.DescribeInstanceTypeOfferingsOutput{},
		},
	}

	for _, instanceType := range instanceTypes {
		ec2api.DescribeInstanceTypesOutput.InstanceTypes = append(ec2api.DescribeInstanceTypesOutput.InstanceTypes, instanceTypeMocks[instanceType])
	}
	for _, zone := range zones {
		for _, instanceType := range instanceTypes {
			offering := &ec2.InstanceTypeOffering{
				InstanceType: &instanceType,
				Location:     &zone,
			}
			ec2api.DescribeInstanceTypeOfferingsOutput.InstanceTypeOfferings = append(ec2api.DescribeInstanceTypeOfferingsOutput.InstanceTypeOfferings, offering)
		}
	}
	return ec2api
}
