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

package fake

import (
	"context"
	"fmt"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha2"
)

// EC2Behavior must be reset between tests otherwise tests will
// pollute each other.
type EC2Behavior struct {
	DescribeInstancesOutput             *ec2.DescribeInstancesOutput
	DescribeLaunchTemplatesOutput       *ec2.DescribeLaunchTemplatesOutput
	DescribeSubnetsOutput               *ec2.DescribeSubnetsOutput
	DescribeSecurityGroupsOutput        *ec2.DescribeSecurityGroupsOutput
	DescribeInstanceTypesOutput         *ec2.DescribeInstanceTypesOutput
	DescribeInstanceTypeOfferingsOutput *ec2.DescribeInstanceTypeOfferingsOutput
	DescribeAvailabilityZonesOutput     *ec2.DescribeAvailabilityZonesOutput
	CalledWithCreateFleetInput          []*ec2.CreateFleetInput
	CalledWithCreateLaunchTemplateInput []*ec2.CreateLaunchTemplateInput
	Instances                           []*ec2.Instance
	LaunchTemplates                     []*ec2.LaunchTemplate
}

type EC2API struct {
	ec2iface.EC2API
	EC2Behavior
}

// Reset must be called between tests otherwise tests will pollute
// each other.
func (e *EC2API) Reset() {
	e.EC2Behavior = EC2Behavior{}
}

func (e *EC2API) CreateFleetWithContext(ctx context.Context, input *ec2.CreateFleetInput, options ...request.Option) (*ec2.CreateFleetOutput, error) {
	e.CalledWithCreateFleetInput = append(e.CalledWithCreateFleetInput, input)
	if input.LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateId == nil &&
		input.LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateName == nil {
		return nil, fmt.Errorf("missing launch template id or name")
	}
	instance := &ec2.Instance{
		InstanceId:     aws.String(randomdata.SillyName()),
		Placement:      &ec2.Placement{AvailabilityZone: aws.String("test-zone-1a")},
		PrivateDnsName: aws.String(fmt.Sprintf("test-instance-%d.example.com", len(e.Instances))),
		InstanceType:   input.LaunchTemplateConfigs[0].Overrides[0].InstanceType,
	}
	e.Instances = append(e.Instances, instance)
	return &ec2.CreateFleetOutput{Instances: []*ec2.CreateFleetInstance{{InstanceIds: []*string{instance.InstanceId}}}}, nil
}

func (e *EC2API) CreateLaunchTemplateWithContext(ctx context.Context, input *ec2.CreateLaunchTemplateInput, options ...request.Option) (*ec2.CreateLaunchTemplateOutput, error) {
	e.CalledWithCreateLaunchTemplateInput = append(e.CalledWithCreateLaunchTemplateInput, input)
	launchTemplate := &ec2.LaunchTemplate{LaunchTemplateName: input.LaunchTemplateName, LaunchTemplateId: aws.String("test-launch-template-id")}
	e.LaunchTemplates = append(e.LaunchTemplates, launchTemplate)
	return &ec2.CreateLaunchTemplateOutput{LaunchTemplate: launchTemplate}, nil
}

func (e *EC2API) DescribeInstancesWithContext(context.Context, *ec2.DescribeInstancesInput, ...request.Option) (*ec2.DescribeInstancesOutput, error) {
	if e.DescribeInstancesOutput != nil {
		return e.DescribeInstancesOutput, nil
	}
	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{{Instances: e.Instances}},
	}, nil
}

func (e *EC2API) DescribeLaunchTemplatesWithContext(ctx context.Context, input *ec2.DescribeLaunchTemplatesInput, options ...request.Option) (*ec2.DescribeLaunchTemplatesOutput, error) {
	if e.DescribeLaunchTemplatesOutput != nil {
		return e.DescribeLaunchTemplatesOutput, nil
	}
	output := &ec2.DescribeLaunchTemplatesOutput{}
	for _, wanted := range input.LaunchTemplateNames {
		for _, launchTemplate := range e.LaunchTemplates {
			if launchTemplate.LaunchTemplateName == wanted {
				output.LaunchTemplates = append(output.LaunchTemplates, launchTemplate)
			}
		}
	}
	if len(output.LaunchTemplates) == 0 {
		return nil, awserr.New("InvalidLaunchTemplateName.NotFoundException", "not found", nil)
	}
	return output, nil
}

func (e *EC2API) DescribeSubnetsWithContext(context.Context, *ec2.DescribeSubnetsInput, ...request.Option) (*ec2.DescribeSubnetsOutput, error) {
	if e.DescribeSubnetsOutput != nil {
		return e.DescribeSubnetsOutput, nil
	}
	return &ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
		{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"),
			Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}}},
		{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1b"),
			Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}}},
		{SubnetId: aws.String("test-subnet-3"), AvailabilityZone: aws.String("test-zone-1c"),
			Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-3")}, {Key: aws.String("TestTag")}}},
	}}, nil
}

func (e *EC2API) DescribeSecurityGroupsWithContext(context.Context, *ec2.DescribeSecurityGroupsInput, ...request.Option) (*ec2.DescribeSecurityGroupsOutput, error) {
	if e.DescribeSecurityGroupsOutput != nil {
		return e.DescribeSecurityGroupsOutput, nil
	}
	return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: []*ec2.SecurityGroup{
		{GroupId: aws.String("test-security-group-1"), Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-security-group-1")}}},
		{GroupId: aws.String("test-security-group-2"), Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-security-group-2")}}},
		{GroupId: aws.String("test-security-group-3"), Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-security-group-3")}, {Key: aws.String("TestTag")}}},
	}}, nil
}

func (e *EC2API) DescribeAvailabilityZonesWithContext(context.Context, *ec2.DescribeAvailabilityZonesInput, ...request.Option) (*ec2.DescribeAvailabilityZonesOutput, error) {
	if e.DescribeAvailabilityZonesOutput != nil {
		return e.DescribeAvailabilityZonesOutput, nil
	}
	return &ec2.DescribeAvailabilityZonesOutput{AvailabilityZones: []*ec2.AvailabilityZone{
		{ZoneName: aws.String("test-zone-1a"), ZoneId: aws.String("testzone1a")},
		{ZoneName: aws.String("test-zone-1b"), ZoneId: aws.String("testzone1b")},
		{ZoneName: aws.String("test-zone-1c"), ZoneId: aws.String("testzone1c")},
	}}, nil
}

func (e *EC2API) DescribeInstanceTypesPagesWithContext(ctx context.Context, input *ec2.DescribeInstanceTypesInput, fn func(*ec2.DescribeInstanceTypesOutput, bool) bool, opts ...request.Option) error {
	if e.DescribeInstanceTypesOutput != nil {
		fn(e.DescribeInstanceTypesOutput, false)
		return nil
	}
	fn(&ec2.DescribeInstanceTypesOutput{
		InstanceTypes: []*ec2.InstanceTypeInfo{
			{
				InstanceType:                  aws.String("m5.large"),
				SupportedUsageClasses:         []*string{aws.String("on-demand")},
				SupportedVirtualizationTypes:  []*string{aws.String("hvm")},
				BurstablePerformanceSupported: aws.Bool(false),
				BareMetal:                     aws.Bool(false),
				ProcessorInfo: &ec2.ProcessorInfo{
					SupportedArchitectures: aws.StringSlice([]string{"x86_64"}),
				},
				VCpuInfo: &ec2.VCpuInfo{
					DefaultVCpus: aws.Int64(2),
				},
				MemoryInfo: &ec2.MemoryInfo{
					SizeInMiB: aws.Int64(8 * 1024),
				},
				NetworkInfo: &ec2.NetworkInfo{
					MaximumNetworkInterfaces:  aws.Int64(3),
					Ipv4AddressesPerInterface: aws.Int64(30),
				},
			},
			{
				InstanceType:                  aws.String("m5.xlarge"),
				SupportedUsageClasses:         []*string{aws.String("on-demand")},
				SupportedVirtualizationTypes:  []*string{aws.String("hvm")},
				BurstablePerformanceSupported: aws.Bool(false),
				BareMetal:                     aws.Bool(false),
				ProcessorInfo: &ec2.ProcessorInfo{
					SupportedArchitectures: aws.StringSlice([]string{"x86_64"}),
				},
				VCpuInfo: &ec2.VCpuInfo{
					DefaultVCpus: aws.Int64(4),
				},
				MemoryInfo: &ec2.MemoryInfo{
					SizeInMiB: aws.Int64(16 * 1024),
				},
				NetworkInfo: &ec2.NetworkInfo{
					MaximumNetworkInterfaces:  aws.Int64(4),
					Ipv4AddressesPerInterface: aws.Int64(60),
				},
			},
			{
				InstanceType:                  aws.String("p3.8xlarge"),
				SupportedUsageClasses:         []*string{aws.String("on-demand")},
				SupportedVirtualizationTypes:  []*string{aws.String("hvm")},
				BurstablePerformanceSupported: aws.Bool(false),
				BareMetal:                     aws.Bool(false),
				ProcessorInfo: &ec2.ProcessorInfo{
					SupportedArchitectures: aws.StringSlice([]string{"x86_64"}),
				},
				VCpuInfo: &ec2.VCpuInfo{
					DefaultVCpus: aws.Int64(32),
				},
				MemoryInfo: &ec2.MemoryInfo{
					SizeInMiB: aws.Int64(249856),
				},
				GpuInfo: &ec2.GpuInfo{
					Gpus: []*ec2.GpuDeviceInfo{{
						Manufacturer: aws.String("NVIDIA"),
						Count:        aws.Int64(4),
					}},
				},
				NetworkInfo: &ec2.NetworkInfo{
					MaximumNetworkInterfaces:  aws.Int64(4),
					Ipv4AddressesPerInterface: aws.Int64(60),
				},
			},
			{
				InstanceType:                  aws.String("c6g.large"),
				SupportedUsageClasses:         []*string{aws.String("on-demand")},
				SupportedVirtualizationTypes:  []*string{aws.String("hvm")},
				BurstablePerformanceSupported: aws.Bool(false),
				BareMetal:                     aws.Bool(false),
				ProcessorInfo: &ec2.ProcessorInfo{
					SupportedArchitectures: aws.StringSlice([]string{v1alpha2.ArchitectureArm64}),
				},
				VCpuInfo: &ec2.VCpuInfo{
					DefaultVCpus: aws.Int64(2),
				},
				MemoryInfo: &ec2.MemoryInfo{
					SizeInMiB: aws.Int64(2 * 1024),
				},
				NetworkInfo: &ec2.NetworkInfo{
					MaximumNetworkInterfaces:  aws.Int64(4),
					Ipv4AddressesPerInterface: aws.Int64(60),
				},
			},
			{
				InstanceType:                  aws.String("inf1.6xlarge"),
				SupportedUsageClasses:         []*string{aws.String("on-demand")},
				SupportedVirtualizationTypes:  []*string{aws.String("hvm")},
				BurstablePerformanceSupported: aws.Bool(false),
				BareMetal:                     aws.Bool(false),
				ProcessorInfo: &ec2.ProcessorInfo{
					SupportedArchitectures: aws.StringSlice([]string{"x86_64"}),
				},
				VCpuInfo: &ec2.VCpuInfo{
					DefaultVCpus: aws.Int64(24),
				},
				MemoryInfo: &ec2.MemoryInfo{
					SizeInMiB: aws.Int64(49152),
				},
				InferenceAcceleratorInfo: &ec2.InferenceAcceleratorInfo{
					Accelerators: []*ec2.InferenceDeviceInfo{{
						Manufacturer: aws.String("AWS"),
						Count:        aws.Int64(4),
					}}},
				NetworkInfo: &ec2.NetworkInfo{
					MaximumNetworkInterfaces:  aws.Int64(4),
					Ipv4AddressesPerInterface: aws.Int64(60),
				},
			},
		},
	}, false)
	return nil
}

func (e *EC2API) DescribeInstanceTypeOfferingsPagesWithContext(ctx context.Context, input *ec2.DescribeInstanceTypeOfferingsInput, fn func(*ec2.DescribeInstanceTypeOfferingsOutput, bool) bool, opts ...request.Option) error {
	if e.DescribeInstanceTypeOfferingsOutput != nil {
		fn(e.DescribeInstanceTypeOfferingsOutput, false)
		return nil
	}
	fn(&ec2.DescribeInstanceTypeOfferingsOutput{
		InstanceTypeOfferings: []*ec2.InstanceTypeOffering{
			{
				InstanceType: aws.String("m5.large"),
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: aws.String("m5.large"),
				Location:     aws.String("test-zone-1b"),
			},
			{
				InstanceType: aws.String("m5.large"),
				Location:     aws.String("test-zone-1c"),
			},
			{
				InstanceType: aws.String("m5.xlarge"),
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: aws.String("m5.2xlarge"),
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: aws.String("m5.4xlarge"),
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: aws.String("m5.8xlarge"),
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: aws.String("p3.8xlarge"),
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: aws.String("inf1.6xlarge"),
				Location:     aws.String("test-zone-1a"),
			},
		},
	}, false)
	return nil
}
