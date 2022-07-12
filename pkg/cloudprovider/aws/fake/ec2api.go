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
	"errors"
	"fmt"
	"sync"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/pkg/utils/functional"
)

type CapacityPool struct {
	CapacityType string
	InstanceType string
	Zone         string
}

// EC2Behavior must be reset between tests otherwise tests will
// pollute each other.
type EC2Behavior struct {
	DescribeInstancesOutput             AtomicPtr[ec2.DescribeInstancesOutput]
	DescribeImagesOutput                AtomicPtr[ec2.DescribeImagesOutput]
	DescribeLaunchTemplatesOutput       AtomicPtr[ec2.DescribeLaunchTemplatesOutput]
	DescribeSubnetsOutput               AtomicPtr[ec2.DescribeSubnetsOutput]
	DescribeSecurityGroupsOutput        AtomicPtr[ec2.DescribeSecurityGroupsOutput]
	DescribeInstanceTypesOutput         AtomicPtr[ec2.DescribeInstanceTypesOutput]
	DescribeInstanceTypeOfferingsOutput AtomicPtr[ec2.DescribeInstanceTypeOfferingsOutput]
	DescribeAvailabilityZonesOutput     AtomicPtr[ec2.DescribeAvailabilityZonesOutput]
	DescribeSpotPriceHistoryOutput      AtomicPtr[ec2.DescribeSpotPriceHistoryOutput]
	CalledWithCreateFleetInput          AtomicPtrSlice[ec2.CreateFleetInput]
	CalledWithCreateLaunchTemplateInput AtomicPtrSlice[ec2.CreateLaunchTemplateInput]
	CalledWithDescribeImagesInput       AtomicPtrSlice[ec2.DescribeImagesInput]
	Instances                           sync.Map
	LaunchTemplates                     sync.Map
	InsufficientCapacityPools           AtomicSlice[CapacityPool]
	NextError                           AtomicError
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
	e.DescribeInstancesOutput.Reset()
	e.DescribeImagesOutput.Reset()
	e.DescribeLaunchTemplatesOutput.Reset()
	e.DescribeSubnetsOutput.Reset()
	e.DescribeSecurityGroupsOutput.Reset()
	e.DescribeInstanceTypesOutput.Reset()
	e.DescribeInstanceTypeOfferingsOutput.Reset()
	e.DescribeAvailabilityZonesOutput.Reset()
	e.CalledWithCreateFleetInput.Reset()
	e.CalledWithCreateLaunchTemplateInput.Reset()
	e.CalledWithDescribeImagesInput.Reset()
	e.DescribeSpotPriceHistoryOutput.Reset()
	e.Instances.Range(func(k, v any) bool {
		e.Instances.Delete(k)
		return true
	})
	e.LaunchTemplates.Range(func(k, v any) bool {
		e.LaunchTemplates.Delete(k)
		return true
	})
	e.InsufficientCapacityPools.Reset()
	e.NextError.Reset()
}

// nolint: gocyclo
func (e *EC2API) CreateFleetWithContext(_ context.Context, input *ec2.CreateFleetInput, _ ...request.Option) (*ec2.CreateFleetOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	e.CalledWithCreateFleetInput.Add(input)
	if input.LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateName == nil {
		return nil, fmt.Errorf("missing launch template name")
	}
	var instanceIds []*string
	var skippedPools []CapacityPool
	var spotInstanceRequestID *string

	if aws.StringValue(input.TargetCapacitySpecification.DefaultTargetCapacityType) == v1alpha1.CapacityTypeSpot {
		spotInstanceRequestID = aws.String(test.RandomName())
	}

	for _, ltc := range input.LaunchTemplateConfigs {
		for _, override := range ltc.Overrides {
			skipInstance := false
			e.InsufficientCapacityPools.Range(func(pool CapacityPool) bool {
				if pool.InstanceType == aws.StringValue(override.InstanceType) &&
					pool.Zone == aws.StringValue(override.AvailabilityZone) &&
					pool.CapacityType == aws.StringValue(input.TargetCapacitySpecification.DefaultTargetCapacityType) {
					skippedPools = append(skippedPools, pool)
					skipInstance = true
					return false
				}
				return true
			})
			if skipInstance {
				continue
			}
			for i := 0; i < int(*input.TargetCapacitySpecification.TotalTargetCapacity); i++ {
				instance := &ec2.Instance{
					InstanceId:            aws.String(test.RandomName()),
					Placement:             &ec2.Placement{AvailabilityZone: input.LaunchTemplateConfigs[0].Overrides[0].AvailabilityZone},
					PrivateDnsName:        aws.String(randomdata.IpV4Address()),
					InstanceType:          input.LaunchTemplateConfigs[0].Overrides[0].InstanceType,
					SpotInstanceRequestId: spotInstanceRequestID,
				}
				e.Instances.Store(*instance.InstanceId, instance)
				instanceIds = append(instanceIds, instance.InstanceId)
			}
		}
	}

	result := &ec2.CreateFleetOutput{Instances: []*ec2.CreateFleetInstance{{InstanceIds: instanceIds}}}
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
	return result, nil
}

func (e *EC2API) CreateLaunchTemplateWithContext(_ context.Context, input *ec2.CreateLaunchTemplateInput, _ ...request.Option) (*ec2.CreateLaunchTemplateOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	e.CalledWithCreateLaunchTemplateInput.Add(input)
	launchTemplate := &ec2.LaunchTemplate{LaunchTemplateName: input.LaunchTemplateName}
	e.LaunchTemplates.Store(input.LaunchTemplateName, launchTemplate)
	return &ec2.CreateLaunchTemplateOutput{LaunchTemplate: launchTemplate}, nil
}

func (e *EC2API) DescribeInstancesWithContext(_ context.Context, input *ec2.DescribeInstancesInput, _ ...request.Option) (*ec2.DescribeInstancesOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	if !e.DescribeInstancesOutput.IsNil() {
		return e.DescribeInstancesOutput.Clone(), nil
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

func (e *EC2API) DescribeImagesWithContext(_ context.Context, input *ec2.DescribeImagesInput, _ ...request.Option) (*ec2.DescribeImagesOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	e.CalledWithDescribeImagesInput.Add(input)
	if !e.DescribeImagesOutput.IsNil() {
		return e.DescribeImagesOutput.Clone(), nil
	}
	return &ec2.DescribeImagesOutput{
		Images: []*ec2.Image{
			{
				ImageId:      aws.String(test.RandomName()),
				Architecture: aws.String("x86_64"),
			},
		},
	}, nil
}

func (e *EC2API) DescribeLaunchTemplatesWithContext(_ context.Context, input *ec2.DescribeLaunchTemplatesInput, _ ...request.Option) (*ec2.DescribeLaunchTemplatesOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	if !e.DescribeLaunchTemplatesOutput.IsNil() {
		return e.DescribeLaunchTemplatesOutput.Clone(), nil
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

func (e *EC2API) DescribeSubnetsWithContext(ctx context.Context, input *ec2.DescribeSubnetsInput, opts ...request.Option) (*ec2.DescribeSubnetsOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	if !e.DescribeSubnetsOutput.IsNil() {
		return e.DescribeSubnetsOutput.Clone(), nil
	}
	subnets := []*ec2.Subnet{
		{
			SubnetId:                aws.String("subnet-test1"),
			AvailabilityZone:        aws.String("test-zone-1a"),
			AvailableIpAddressCount: aws.Int64(100),
			Tags: []*ec2.Tag{
				{Key: aws.String("Name"), Value: aws.String("test-subnet-1")},
				{Key: aws.String("foo"), Value: aws.String("bar")},
			},
		},
		{
			SubnetId:                aws.String("subnet-test2"),
			AvailabilityZone:        aws.String("test-zone-1b"),
			AvailableIpAddressCount: aws.Int64(100),
			Tags: []*ec2.Tag{
				{Key: aws.String("Name"), Value: aws.String("test-subnet-2")},
				{Key: aws.String("foo"), Value: aws.String("bar")},
			},
		},
		{
			SubnetId:                aws.String("subnet-test3"),
			AvailabilityZone:        aws.String("test-zone-1c"),
			AvailableIpAddressCount: aws.Int64(100),
			Tags: []*ec2.Tag{
				{Key: aws.String("Name"), Value: aws.String("test-subnet-3")},
				{Key: aws.String("TestTag")},
				{Key: aws.String("foo"), Value: aws.String("bar")},
			},
		},
	}

	return &ec2.DescribeSubnetsOutput{Subnets: FilterDescribeSubnets(subnets, input.Filters)}, nil
}

func (e *EC2API) DescribeSecurityGroupsWithContext(ctx context.Context, input *ec2.DescribeSecurityGroupsInput, opts ...request.Option) (*ec2.DescribeSecurityGroupsOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	if !e.DescribeSecurityGroupsOutput.IsNil() {
		return e.DescribeSecurityGroupsOutput.Clone(), nil
	}
	sgs := []*ec2.SecurityGroup{
		{
			GroupId: aws.String("sg-test1"),
			Tags: []*ec2.Tag{
				{Key: aws.String("Name"), Value: aws.String("test-security-group-1")},
				{Key: aws.String("foo"), Value: aws.String("bar")},
			},
		},
		{
			GroupId: aws.String("sg-test2"),
			Tags: []*ec2.Tag{
				{Key: aws.String("Name"), Value: aws.String("test-security-group-2")},
				{Key: aws.String("foo"), Value: aws.String("bar")},
			},
		},
		{
			GroupId: aws.String("sg-test3"),
			Tags: []*ec2.Tag{
				{Key: aws.String("Name"), Value: aws.String("test-security-group-3")},
				{Key: aws.String("TestTag")},
				{Key: aws.String("foo"), Value: aws.String("bar")},
			},
		},
	}
	return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: FilterDescribeSecurtyGroups(sgs, input.Filters)}, nil
}

func (e *EC2API) DescribeAvailabilityZonesWithContext(context.Context, *ec2.DescribeAvailabilityZonesInput, ...request.Option) (*ec2.DescribeAvailabilityZonesOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	if !e.DescribeAvailabilityZonesOutput.IsNil() {
		return e.DescribeAvailabilityZonesOutput.Clone(), nil
	}
	return &ec2.DescribeAvailabilityZonesOutput{AvailabilityZones: []*ec2.AvailabilityZone{
		{ZoneName: aws.String("test-zone-1a"), ZoneId: aws.String("testzone1a")},
		{ZoneName: aws.String("test-zone-1b"), ZoneId: aws.String("testzone1b")},
		{ZoneName: aws.String("test-zone-1c"), ZoneId: aws.String("testzone1c")},
	}}, nil
}

func (e *EC2API) DescribeInstanceTypesPagesWithContext(_ context.Context, _ *ec2.DescribeInstanceTypesInput, fn func(*ec2.DescribeInstanceTypesOutput, bool) bool, _ ...request.Option) error {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return e.NextError.Get()
	}
	if !e.DescribeInstanceTypesOutput.IsNil() {
		fn(e.DescribeInstanceTypesOutput.Clone(), false)
		return nil
	}
	fn(&ec2.DescribeInstanceTypesOutput{
		InstanceTypes: []*ec2.InstanceTypeInfo{
			{
				InstanceType:                  aws.String("t3.large"),
				SupportedUsageClasses:         DefaultSupportedUsageClasses,
				SupportedVirtualizationTypes:  []*string{aws.String("hvm")},
				BurstablePerformanceSupported: aws.Bool(true),
				BareMetal:                     aws.Bool(false),
				Hypervisor:                    aws.String("nitro"),
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
					Ipv4AddressesPerInterface: aws.Int64(12),
				},
			},
			{
				InstanceType:                  aws.String("m5.large"),
				SupportedUsageClasses:         DefaultSupportedUsageClasses,
				SupportedVirtualizationTypes:  []*string{aws.String("hvm")},
				BurstablePerformanceSupported: aws.Bool(false),
				BareMetal:                     aws.Bool(false),
				Hypervisor:                    aws.String("nitro"),
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
				Hypervisor:                    aws.String("nitro"),
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
				Hypervisor:                    aws.String("xen"),
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
						Name:         aws.String("Nvidia V100"), // In reality this value is `V100`, but this exercises lower kabob casing
						Manufacturer: aws.String("NVIDIA"),
						Count:        aws.Int64(4),
						MemoryInfo: &ec2.GpuDeviceMemoryInfo{
							SizeInMiB: aws.Int64(16384),
						},
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
				Hypervisor:                    aws.String("nitro"),
				ProcessorInfo: &ec2.ProcessorInfo{
					SupportedArchitectures: aws.StringSlice([]string{v1alpha5.ArchitectureArm64}),
				},
				VCpuInfo: &ec2.VCpuInfo{
					DefaultVCpus: aws.Int64(2),
				},
				MemoryInfo: &ec2.MemoryInfo{
					SizeInMiB: aws.Int64(4 * 1024),
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
				Hypervisor:                    aws.String("nitro"),
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
				Hypervisor:                    aws.String("nitro"),
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
			{
				InstanceType:                  aws.String("m5.metal"),
				SupportedUsageClasses:         DefaultSupportedUsageClasses,
				SupportedVirtualizationTypes:  []*string{aws.String("hvm")},
				BurstablePerformanceSupported: aws.Bool(false),
				BareMetal:                     aws.Bool(true),
				Hypervisor:                    nil,
				ProcessorInfo: &ec2.ProcessorInfo{
					SupportedArchitectures: aws.StringSlice([]string{"x86_64"}),
				},
				VCpuInfo: &ec2.VCpuInfo{
					DefaultVCpus: aws.Int64(96),
				},
				MemoryInfo: &ec2.MemoryInfo{
					SizeInMiB: aws.Int64(393216),
				},
				NetworkInfo: &ec2.NetworkInfo{
					MaximumNetworkInterfaces:  aws.Int64(15),
					Ipv4AddressesPerInterface: aws.Int64(50),
				},
			},
		},
	}, false)
	return nil
}

func (e *EC2API) DescribeInstanceTypeOfferingsPagesWithContext(_ context.Context, _ *ec2.DescribeInstanceTypeOfferingsInput, fn func(*ec2.DescribeInstanceTypeOfferingsOutput, bool) bool, _ ...request.Option) error {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return e.NextError.Get()
	}
	if !e.DescribeInstanceTypeOfferingsOutput.IsNil() {
		fn(e.DescribeInstanceTypeOfferingsOutput.Clone(), false)
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
				InstanceType: aws.String("m5.xlarge"),
				Location:     aws.String("test-zone-1b"),
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
				InstanceType: aws.String("p3.8xlarge"),
				Location:     aws.String("test-zone-1b"),
			},
			{
				InstanceType: aws.String("t3.large"),
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: aws.String("t3.large"),
				Location:     aws.String("test-zone-1b"),
			},
			{
				InstanceType: aws.String("inf1.2xlarge"),
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: aws.String("inf1.6xlarge"),
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: aws.String("c6g.large"),
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: aws.String("m5.metal"),
				Location:     aws.String("test-zone-1a"),
			},
			{
				InstanceType: aws.String("m5.metal"),
				Location:     aws.String("test-zone-1b"),
			},
			{
				InstanceType: aws.String("m5.metal"),
				Location:     aws.String("test-zone-1c"),
			},
		},
	}, false)
	return nil
}

func (e *EC2API) DescribeSpotPriceHistoryPagesWithContext(_ aws.Context, _ *ec2.DescribeSpotPriceHistoryInput, fn func(*ec2.DescribeSpotPriceHistoryOutput, bool) bool, opts ...request.Option) error {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return e.NextError.Get()
	}
	if !e.DescribeSpotPriceHistoryOutput.IsNil() {
		fn(e.DescribeSpotPriceHistoryOutput.Clone(), false)
		return nil
	}
	// fail if the test doesn't provide specific data which causes our pricing provider to use its static price list
	return errors.New("no pricing data provided")
}
