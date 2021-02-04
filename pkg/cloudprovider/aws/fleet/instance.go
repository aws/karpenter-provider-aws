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

package fleet

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
)

type InstanceProvider struct {
	ec2 ec2iface.EC2API
}

// Packing is a binpacking solution of pods and their viable instances types.
type Packing struct {
	// Pods is a set of pods that is assigned to this packing.
	Pods []*v1.Pod
	// InstanceTypes is a set of instance types that is able to fit the pods.
	InstanceTypeOptions []string
}

// GetPackings for the provided pods. Computes an ordered set of viable instance
// types for each packing of pods. Instance variety enables EC2 to make better cost and availability decisions.
func (p *InstanceProvider) GetPackings(ctx context.Context, pods []*v1.Pod, overhead v1.ResourceList) ([]*Packing, error) {
	zap.S().Infof("Successfully packed %d pods onto %d nodes", len(pods), 1)
	return []*Packing{{
		InstanceTypeOptions: []string{"m5.large"}, // TODO, prioritize possible instance types
		Pods:                pods,
	}}, nil
}

// Create an instance given the constraints.
func (p *InstanceProvider) Create(ctx context.Context,
	launchTemplate *ec2.LaunchTemplate,
	instanceTypeOptions []string,
	zonalSubnetOptions map[string][]*ec2.Subnet,
) (*string, error) {
	// 1. Construct override options.
	var overrides []*ec2.FleetLaunchTemplateOverridesRequest
	for _, instanceType := range instanceTypeOptions {
		for zone, subnets := range zonalSubnetOptions {
			overrides = append(overrides, &ec2.FleetLaunchTemplateOverridesRequest{
				AvailabilityZone: aws.String(zone),
				InstanceType:     aws.String(instanceType),
				// FleetAPI cannot span subnets from the same AZ, so randomize.
				SubnetId: aws.String(*subnets[rand.Intn(len(subnets))].SubnetId),
			})
		}
	}

	// 2. Create fleet
	createFleetOutput, err := p.ec2.CreateFleetWithContext(ctx, &ec2.CreateFleetInput{
		Type: aws.String(ec2.FleetTypeInstant),
		TargetCapacitySpecification: &ec2.TargetCapacitySpecificationRequest{
			DefaultTargetCapacityType: aws.String(ec2.DefaultTargetCapacityTypeOnDemand),
			TotalTargetCapacity:       aws.Int64(1),
		},
		LaunchTemplateConfigs: []*ec2.FleetLaunchTemplateConfigRequest{{
			LaunchTemplateSpecification: &ec2.FleetLaunchTemplateSpecificationRequest{
				LaunchTemplateName: launchTemplate.LaunchTemplateName,
				Version:            aws.String("$Default"),
			},
			Overrides: overrides,
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("creating fleet %w", err)
	}
	// TODO aggregate errors
	if count := len(createFleetOutput.Errors); count > 0 {
		return nil, fmt.Errorf("errors while creating fleet, %v", createFleetOutput.Errors)
	}
	if count := len(createFleetOutput.Instances); count != 1 {
		return nil, fmt.Errorf("expected 1 instance, but got %d", count)
	}
	if count := len(createFleetOutput.Instances[0].InstanceIds); count != 1 {
		return nil, fmt.Errorf("expected 1 instance ids, but got %d", count)
	}
	return createFleetOutput.Instances[0].InstanceIds[0], nil
}
