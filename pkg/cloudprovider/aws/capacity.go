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

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"k8s.io/client-go/rest"
)

type Capacity struct {
	EC2API         ec2iface.EC2API
	LaunchTemplate *ec2.LaunchTemplate
	ZonalSubnets   map[string]*ec2.Subnet
}

// NewCapacity constructs a Capacity client for AWS
func NewCapacity(EC2API ec2iface.EC2API, EKSAPI eksiface.EKSAPI, IAMAPI iamiface.IAMAPI, config *rest.Config) *Capacity {
	initialization := NewInitialization(EC2API, EKSAPI, IAMAPI, config)
	return &Capacity{
		EC2API:         EC2API,
		LaunchTemplate: initialization.LaunchTemplate,
		ZonalSubnets:   initialization.ZonalSubnets,
	}
}

// Create a set of nodes given the constraints
func (c *Capacity) Create(ctx context.Context, constraints *cloudprovider.CapacityConstraints) error {
	// TODO, select a zone more intelligently
	var zone string
	for zone = range c.ZonalSubnets {
	}

	if _, err := c.EC2API.CreateFleetWithContext(context.TODO(), &ec2.CreateFleetInput{
		Type: aws.String(ec2.FleetTypeInstant),
		TargetCapacitySpecification: &ec2.TargetCapacitySpecificationRequest{
			DefaultTargetCapacityType: aws.String(ec2.DefaultTargetCapacityTypeOnDemand),
			TotalTargetCapacity:       aws.Int64(1),
		},
		LaunchTemplateConfigs: []*ec2.FleetLaunchTemplateConfigRequest{{
			LaunchTemplateSpecification: &ec2.FleetLaunchTemplateSpecificationRequest{
				LaunchTemplateName: c.LaunchTemplate.LaunchTemplateName,
				Version:            aws.String("$Default"),
			},
			Overrides: []*ec2.FleetLaunchTemplateOverridesRequest{{
				AvailabilityZone: aws.String(zone),
				InstanceType:     aws.String("m5.large"),
				SubnetId:         c.ZonalSubnets[zone].SubnetId,
			}},
		}},
	}); err != nil {
		return fmt.Errorf("creating fleet, %w", err)
	}
	return nil
}
