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
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

type EC2API struct {
	ec2iface.EC2API
	CreateFleetOutput                   *ec2.CreateFleetOutput
	DescribeInstancesOutput             *ec2.DescribeInstancesOutput
	DescribeLaunchTemplatesOutput       *ec2.DescribeLaunchTemplatesOutput
	DescribeSubnetsOutput               *ec2.DescribeSubnetsOutput
	DescribeSecurityGroupsOutput        *ec2.DescribeSecurityGroupsOutput
	DescribeInstanceTypesOutput         *ec2.DescribeInstanceTypesOutput
	DescribeInstanceTypeOfferingsOutput *ec2.DescribeInstanceTypeOfferingsOutput
	DescribeAvailabilityZonesOutput     *ec2.DescribeAvailabilityZonesOutput
	WantErr                             error

	CalledWithCreateFleetInput []ec2.CreateFleetInput
	Instances                  []*ec2.Instance
}

func (a *EC2API) Reset() {
	a.CalledWithCreateFleetInput = nil
	a.Instances = nil
}

func (a *EC2API) CreateFleetWithContext(ctx context.Context, input *ec2.CreateFleetInput, options ...request.Option) (*ec2.CreateFleetOutput, error) {
	a.CalledWithCreateFleetInput = append(a.CalledWithCreateFleetInput, *input)
	if a.WantErr != nil {
		return nil, a.WantErr
	}
	if a.CreateFleetOutput != nil {
		return a.CreateFleetOutput, nil
	}
	instance := &ec2.Instance{
		InstanceId:     aws.String(randomdata.SillyName()),
		Placement:      &ec2.Placement{AvailabilityZone: aws.String("test-zone")},
		PrivateDnsName: aws.String(fmt.Sprintf("test-instance-%d.example.com", len(a.Instances))),
	}
	a.Instances = append(a.Instances, instance)
	return &ec2.CreateFleetOutput{Instances: []*ec2.CreateFleetInstance{{InstanceIds: []*string{instance.InstanceId}}}}, nil
}

func (a *EC2API) DescribeInstancesWithContext(context.Context, *ec2.DescribeInstancesInput, ...request.Option) (*ec2.DescribeInstancesOutput, error) {
	if a.WantErr != nil {
		return nil, a.WantErr
	}
	if a.DescribeInstancesOutput != nil {
		return a.DescribeInstancesOutput, nil
	}
	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{{Instances: a.Instances}},
	}, nil
}

func (a *EC2API) DescribeLaunchTemplatesWithContext(context.Context, *ec2.DescribeLaunchTemplatesInput, ...request.Option) (*ec2.DescribeLaunchTemplatesOutput, error) {
	if a.WantErr != nil {
		return nil, a.WantErr
	}
	if a.DescribeLaunchTemplatesOutput != nil {
		return a.DescribeLaunchTemplatesOutput, nil
	}
	return &ec2.DescribeLaunchTemplatesOutput{LaunchTemplates: []*ec2.LaunchTemplate{{
		LaunchTemplateName: aws.String("test-launch-template"),
	}}}, nil
}

func (a *EC2API) DescribeSubnetsWithContext(context.Context, *ec2.DescribeSubnetsInput, ...request.Option) (*ec2.DescribeSubnetsOutput, error) {
	if a.WantErr != nil {
		return nil, a.WantErr
	}
	if a.DescribeSubnetsOutput != nil {
		return a.DescribeSubnetsOutput, nil
	}
	return &ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{{SubnetId: aws.String("test-subnet"), AvailabilityZone: aws.String("test-zone")}}}, nil
}

func (a *EC2API) DescribeSecurityGroupsWithContext(context.Context, *ec2.DescribeSecurityGroupsInput, ...request.Option) (*ec2.DescribeSecurityGroupsOutput, error) {
	if a.WantErr != nil {
		return nil, a.WantErr
	}
	if a.DescribeSecurityGroupsOutput != nil {
		return a.DescribeSecurityGroupsOutput, nil
	}
	return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: []*ec2.SecurityGroup{{GroupId: aws.String("test-group")}}}, nil
}

func (a *EC2API) DescribeAvailabilityZonesWithContext(context.Context, *ec2.DescribeAvailabilityZonesInput, ...request.Option) (*ec2.DescribeAvailabilityZonesOutput, error) {
	if a.WantErr != nil {
		return nil, a.WantErr
	}
	if a.DescribeAvailabilityZonesOutput != nil {
		return a.DescribeAvailabilityZonesOutput, nil
	}
	return &ec2.DescribeAvailabilityZonesOutput{AvailabilityZones: []*ec2.AvailabilityZone{
		{ZoneName: aws.String("test-zone-1a"), ZoneId: aws.String("testzone1a")},
		{ZoneName: aws.String("test-zone-1b"), ZoneId: aws.String("testzone1b")},
		{ZoneName: aws.String("test-zone-1c"), ZoneId: aws.String("testzone1c")},
	}}, nil
}

func (a *EC2API) DescribeInstanceTypesPagesWithContext(ctx context.Context, input *ec2.DescribeInstanceTypesInput, fn func(*ec2.DescribeInstanceTypesOutput, bool) bool, opts ...request.Option) error {
	if a.WantErr != nil {
		return a.WantErr
	}
	if a.DescribeInstanceTypesOutput != nil {
		fn(a.DescribeInstanceTypesOutput, false)
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
					SizeInMiB: aws.Int64(8),
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
					SizeInMiB: aws.Int64(16),
				},
				NetworkInfo: &ec2.NetworkInfo{
					MaximumNetworkInterfaces:  aws.Int64(4),
					Ipv4AddressesPerInterface: aws.Int64(60),
				},
			},
		},
	}, false)
	return nil
}

func (a *EC2API) DescribeInstanceTypeOfferingsPagesWithContext(ctx context.Context, input *ec2.DescribeInstanceTypeOfferingsInput, fn func(*ec2.DescribeInstanceTypeOfferingsOutput, bool) bool, opts ...request.Option) error {
	if a.WantErr != nil {
		return a.WantErr
	}
	if a.DescribeInstanceTypeOfferingsOutput != nil {
		fn(a.DescribeInstanceTypeOfferingsOutput, false)
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
		},
	}, false)
	return nil
}
