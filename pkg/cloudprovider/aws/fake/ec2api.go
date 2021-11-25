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
	"sync"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/functional"
	set "github.com/deckarep/golang-set"
)

type CapacityPool struct {
	InstanceType string
	Zone         string
}

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
	CalledWithCreateFleetInput          set.Set
	CalledWithCreateLaunchTemplateInput set.Set
	Instances                           sync.Map
	LaunchTemplates                     sync.Map
	InsufficientCapacityPools           []CapacityPool
}

type EC2API struct {
	ec2iface.EC2API
	EC2Behavior
}

// DefaultSupportedUsageClasses is a var because []*string can't be a const
var DefaultSupportedUsageClasses = aws.StringSlice([]string{"on-demand", "spot"})

// Reset must be called between tests otherwise tests will pollute
// each other.
func (e *EC2API) Reset() {
	e.EC2Behavior = EC2Behavior{
		CalledWithCreateFleetInput:          set.NewSet(),
		CalledWithCreateLaunchTemplateInput: set.NewSet(),
		Instances:                           sync.Map{},
		LaunchTemplates:                     sync.Map{},
		InsufficientCapacityPools:           []CapacityPool{},
	}
}

func (e *EC2API) CreateFleetWithContext(_ context.Context, input *ec2.CreateFleetInput, _ ...request.Option) (*ec2.CreateFleetOutput, error) {
	e.CalledWithCreateFleetInput.Add(input)
	if input.LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateName == nil {
		return nil, fmt.Errorf("missing launch template name")
	}
	instances := []*ec2.Instance{}
	instanceIds := []*string{}
	skippedPools := []CapacityPool{}
	for i := 0; i < int(*input.TargetCapacitySpecification.TotalTargetCapacity); i++ {
		skipInstance := false
		for _, pool := range e.InsufficientCapacityPools {
			if pool.InstanceType == aws.StringValue(input.LaunchTemplateConfigs[0].Overrides[0].InstanceType) &&
				pool.Zone == aws.StringValue(input.LaunchTemplateConfigs[0].Overrides[0].AvailabilityZone) {
				skippedPools = append(skippedPools, pool)
				skipInstance = true
				break
			}
		}
		if skipInstance {
			continue
		}
		instances = append(instances, &ec2.Instance{
			InstanceId:     aws.String(randomdata.SillyName()),
			Placement:      &ec2.Placement{AvailabilityZone: aws.String("test-zone-1a")},
			PrivateDnsName: aws.String(randomdata.IpV4Address()),
			InstanceType:   input.LaunchTemplateConfigs[0].Overrides[0].InstanceType,
		})
		e.Instances.Store(*instances[i].InstanceId, instances[i])
		instanceIds = append(instanceIds, instances[i].InstanceId)
	}

	result := &ec2.CreateFleetOutput{
		Instances: []*ec2.CreateFleetInstance{{InstanceIds: instanceIds}}}
	if len(skippedPools) > 0 {
		for _, pool := range skippedPools {
			result.Errors = append(result.Errors, &ec2.CreateFleetError{
				ErrorCode: aws.String("InsufficientInstanceCapacity"),
				LaunchTemplateAndOverrides: &ec2.LaunchTemplateAndOverridesResponse{
					LaunchTemplateSpecification: &ec2.FleetLaunchTemplateSpecification{
						LaunchTemplateId:   input.LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateId,
						LaunchTemplateName: input.LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateName,
					},
					Overrides: &ec2.FleetLaunchTemplateOverrides{
						InstanceType:     aws.String(pool.InstanceType),
						AvailabilityZone: aws.String(pool.Zone),
					},
				},
			})
		}
	}
	return result, nil
}

func (e *EC2API) CreateLaunchTemplateWithContext(_ context.Context, input *ec2.CreateLaunchTemplateInput, _ ...request.Option) (*ec2.CreateLaunchTemplateOutput, error) {
	e.CalledWithCreateLaunchTemplateInput.Add(input)
	launchTemplate := &ec2.LaunchTemplate{LaunchTemplateName: input.LaunchTemplateName}
	e.LaunchTemplates.Store(input.LaunchTemplateName, launchTemplate)
	return &ec2.CreateLaunchTemplateOutput{LaunchTemplate: launchTemplate}, nil
}

func (e *EC2API) DescribeInstancesWithContext(_ context.Context, input *ec2.DescribeInstancesInput, _ ...request.Option) (*ec2.DescribeInstancesOutput, error) {
	if e.DescribeInstancesOutput != nil {
		return e.DescribeInstancesOutput, nil
	}
	instances := []*ec2.Instance{}
	for _, instanceID := range input.InstanceIds {
		instance, _ := e.Instances.Load(*instanceID)
		instances = append(instances, instance.(*ec2.Instance))
	}

	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{{Instances: instances}},
	}, nil
}

func (e *EC2API) DescribeLaunchTemplatesWithContext(_ context.Context, input *ec2.DescribeLaunchTemplatesInput, _ ...request.Option) (*ec2.DescribeLaunchTemplatesOutput, error) {
	if e.DescribeLaunchTemplatesOutput != nil {
		return e.DescribeLaunchTemplatesOutput, nil
	}
	output := &ec2.DescribeLaunchTemplatesOutput{}
	e.LaunchTemplates.Range(func(key, value interface{}) bool {
		launchTemplate := value.(*ec2.LaunchTemplate)
		if functional.ContainsString(aws.StringValueSlice(input.LaunchTemplateNames), aws.StringValue(launchTemplate.LaunchTemplateName)) {
			output.LaunchTemplates = append(output.LaunchTemplates, launchTemplate)
		}
		return true
	})
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

func (e *EC2API) DescribeInstanceTypesPagesWithContext(_ context.Context, _ *ec2.DescribeInstanceTypesInput, fn func(*ec2.DescribeInstanceTypesOutput, bool) bool, _ ...request.Option) error {
	if e.DescribeInstanceTypesOutput != nil {
		fn(e.DescribeInstanceTypesOutput, false)
		return nil
	}
	fn(&ec2.DescribeInstanceTypesOutput{
		InstanceTypes: []*ec2.InstanceTypeInfo{
			{
				InstanceType:                  aws.String("m5.large"),
				SupportedUsageClasses:         DefaultSupportedUsageClasses,
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
				SupportedUsageClasses:         DefaultSupportedUsageClasses,
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
				SupportedUsageClasses:         DefaultSupportedUsageClasses,
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
				SupportedUsageClasses:         DefaultSupportedUsageClasses,
				SupportedVirtualizationTypes:  []*string{aws.String("hvm")},
				BurstablePerformanceSupported: aws.Bool(false),
				BareMetal:                     aws.Bool(false),
				ProcessorInfo: &ec2.ProcessorInfo{
					SupportedArchitectures: aws.StringSlice([]string{v1alpha5.ArchitectureArm64}),
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
				InstanceType:                  aws.String("inf1.2xlarge"),
				SupportedUsageClasses:         DefaultSupportedUsageClasses,
				SupportedVirtualizationTypes:  []*string{aws.String("hvm")},
				BurstablePerformanceSupported: aws.Bool(false),
				BareMetal:                     aws.Bool(false),
				ProcessorInfo: &ec2.ProcessorInfo{
					SupportedArchitectures: aws.StringSlice([]string{"x86_64"}),
				},
				VCpuInfo: &ec2.VCpuInfo{
					DefaultVCpus: aws.Int64(8),
				},
				MemoryInfo: &ec2.MemoryInfo{
					SizeInMiB: aws.Int64(16384),
				},
				InferenceAcceleratorInfo: &ec2.InferenceAcceleratorInfo{
					Accelerators: []*ec2.InferenceDeviceInfo{{
						Manufacturer: aws.String("AWS"),
						Count:        aws.Int64(1),
					}}},
				NetworkInfo: &ec2.NetworkInfo{
					MaximumNetworkInterfaces:  aws.Int64(4),
					Ipv4AddressesPerInterface: aws.Int64(60),
				},
			},
			{
				InstanceType:                  aws.String("inf1.6xlarge"),
				SupportedUsageClasses:         DefaultSupportedUsageClasses,
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

func (e *EC2API) DescribeInstanceTypeOfferingsPagesWithContext(_ context.Context, _ *ec2.DescribeInstanceTypeOfferingsInput, fn func(*ec2.DescribeInstanceTypeOfferingsOutput, bool) bool, _ ...request.Option) error {
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
				InstanceType: aws.String("inf1.2xlarge"),
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
