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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

type EC2API struct {
	ec2iface.EC2API
	CreateFleetOutput             *ec2.CreateFleetOutput
	DescribeInstancesOutput       *ec2.DescribeInstancesOutput
	DescribeLaunchTemplatesOutput *ec2.DescribeLaunchTemplatesOutput
	DescribeSubnetsOutput         *ec2.DescribeSubnetsOutput
	DescribeSecurityGroupsOutput  *ec2.DescribeSecurityGroupsOutput
	WantErr                       error

	CalledWithCreateFleetInput []ec2.CreateFleetInput
}

func (a *EC2API) Reset() {
	a.CalledWithCreateFleetInput = nil
}

func (a *EC2API) CreateFleetWithContext(ctx context.Context, input *ec2.CreateFleetInput, options ...request.Option) (*ec2.CreateFleetOutput, error) {
	a.CalledWithCreateFleetInput = append(a.CalledWithCreateFleetInput, *input)
	if a.WantErr != nil {
		return nil, a.WantErr
	}
	if a.CreateFleetOutput != nil {
		return a.CreateFleetOutput, nil
	}
	return &ec2.CreateFleetOutput{Instances: []*ec2.CreateFleetInstance{{InstanceIds: []*string{aws.String("test-instance")}}}}, nil
}

func (a *EC2API) DescribeInstancesWithContext(context.Context, *ec2.DescribeInstancesInput, ...request.Option) (*ec2.DescribeInstancesOutput, error) {
	if a.WantErr != nil {
		return nil, a.WantErr
	}
	if a.DescribeInstancesOutput != nil {
		return a.DescribeInstancesOutput, nil
	}
	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{{Instances: []*ec2.Instance{{
			InstanceId:     aws.String("test-instance"),
			PrivateDnsName: aws.String("test-private-dns-name"),
			Placement:      &ec2.Placement{AvailabilityZone: aws.String("test-zone")}},
		}}},
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
