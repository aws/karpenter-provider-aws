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
	"math/rand"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/packing"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
)

const (
	// maxInstanceTypes defines the number of instance type options to pass to fleet
	maxInstanceTypes = 20
)

type InstanceProvider struct {
	ec2api ec2iface.EC2API
	vpc    *VPCProvider
}

// Create an instance given the constraints.
// instanceTypeOptions should be sorted by priority for spot capacity type.
// If spot is not used, the instanceTypeOptions are not required to be sorted
// because we are using ec2 fleet's lowest-price OD allocation strategy
func (p *InstanceProvider) Create(ctx context.Context,
	launchTemplate *ec2.LaunchTemplate,
	instanceTypeOptions []*packing.Instance,
	zonalSubnetOptions map[string][]*ec2.Subnet,
	capacityType string,
) (*string, error) {
	// 1. Trim the instanceTypeOptions so that the fleet request doesn't get too large
	// If ~130 instance types are passed into fleet, the request can exceed the EC2 request size limit (145kb)
	// due to the overrides expansion for subnetId (depends on number of AZs), Instance Type, and Priority.
	// For spot capacity-optimized-prioritized, the request should be smaller to prevent using
	// excessively large instance types that are more plentiful in capacity which the algorithm will bias towards.
	// packing.InstanceTypes is sorted by vcpus and memory ascending so it's safe to trim the end of the list
	// to remove excessively large instance types
	if len(instanceTypeOptions) > maxInstanceTypes {
		instanceTypeOptions = instanceTypeOptions[:maxInstanceTypes]
	}
	// 2. Construct override options.
	var overrides []*ec2.FleetLaunchTemplateOverridesRequest
	for i, instanceType := range instanceTypeOptions {
		for _, zone := range instanceType.Zones {
			subnets := zonalSubnetOptions[zone]
			if len(subnets) == 0 {
				continue
			}
			override := &ec2.FleetLaunchTemplateOverridesRequest{
				InstanceType: aws.String(*instanceType.InstanceType),
				// FleetAPI cannot span subnets from the same AZ, so randomize.
				SubnetId: aws.String(*subnets[rand.Intn(len(subnets))].SubnetId),
			}
			// Add a priority for spot requests since we are using the capacity-optimized-prioritized spot allocation strategy
			// to reduce the likelihood of getting an excessively large instance type.
			// instanceTypeOptions are sorted by vcpus and memory so this prioritizes smaller instance types.
			if capacityType == capacityTypeSpot {
				override.Priority = aws.Float64(float64(i))
			}
			overrides = append(overrides, override)
		}
	}
	// 3. Create fleet
	createFleetOutput, err := p.ec2api.CreateFleetWithContext(ctx, &ec2.CreateFleetInput{
		Type: aws.String(ec2.FleetTypeInstant),
		TargetCapacitySpecification: &ec2.TargetCapacitySpecificationRequest{
			DefaultTargetCapacityType: aws.String(capacityType),
			TotalTargetCapacity:       aws.Int64(1),
		},
		// OnDemandOptions are allowed to be specified even when requesting spot
		OnDemandOptions: &ec2.OnDemandOptionsRequest{
			AllocationStrategy: aws.String(ec2.FleetOnDemandAllocationStrategyLowestPrice),
		},
		// SpotOptions are allowed to be specified even when requesting on-demand
		SpotOptions: &ec2.SpotOptionsRequest{
			AllocationStrategy: aws.String(ec2.SpotAllocationStrategyCapacityOptimizedPrioritized),
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
	if count := len(createFleetOutput.Instances); count != 1 {
		return nil, fmt.Errorf("expected 1 instance, but got %d due to errors %v", count, createFleetOutput.Errors)
	}
	if count := len(createFleetOutput.Instances[0].InstanceIds); count != 1 {
		return nil, fmt.Errorf("expected 1 instance ids, but got %d due to errors %v", count, createFleetOutput.Errors)
	}
	// TODO aggregate errors
	if count := len(createFleetOutput.Errors); count > 0 {
		zap.S().Warnf("CreateFleet encountered %d errors, but still launched instances, %v", count, createFleetOutput.Errors)
	}
	return createFleetOutput.Instances[0].InstanceIds[0], nil
}

func (p *InstanceProvider) Terminate(ctx context.Context, nodes []*v1.Node) error {
	if len(nodes) == 0 {
		return nil
	}
	ids := p.getInstanceIDs(nodes)

	_, err := p.ec2api.TerminateInstancesWithContext(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: ids,
	})
	if err != nil {
		return fmt.Errorf("terminating %d instances, %w", len(ids), err)
	}

	return nil
}

func (p *InstanceProvider) getInstanceIDs(nodes []*v1.Node) []*string {
	ids := []*string{}
	for _, node := range nodes {
		id := strings.Split(node.Spec.ProviderID, "/")
		if len(id) < 5 {
			zap.S().Debugf("Continuing after failure to parse instance id, %s has invalid format", node.Name)
			continue
		}
		ids = append(ids, aws.String(id[4]))
	}
	return ids
}
