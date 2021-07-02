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
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/cloudprovider"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

type InstanceProvider struct {
	ec2api ec2iface.EC2API
}

// Create an instance given the constraints.
// instanceTypeOptions should be sorted by priority for spot capacity type.
// If spot is not used, the instanceTypeOptions are not required to be sorted
// because we are using ec2 fleet's lowest-price OD allocation strategy
func (p *InstanceProvider) Create(ctx context.Context,
	launchTemplate *LaunchTemplate,
	instanceTypeOptions []cloudprovider.InstanceType,
	subnets []*ec2.Subnet,
	capacityType string,
) (*string, error) {
	// 1. Construct override options.
	var overrides []*ec2.FleetLaunchTemplateOverridesRequest
	for i, instanceType := range instanceTypeOptions {
		for _, zone := range instanceType.Zones() {
			for _, subnet := range subnets {
				if aws.StringValue(subnet.AvailabilityZone) == zone {
					override := &ec2.FleetLaunchTemplateOverridesRequest{
						InstanceType: aws.String(instanceType.Name()),
						SubnetId:     subnet.SubnetId,
					}
					// Add a priority for spot requests since we are using the capacity-optimized-prioritized spot allocation strategy
					// to reduce the likelihood of getting an excessively large instance type.
					// instanceTypeOptions are sorted by vcpus and memory so this prioritizes smaller instance types.
					if capacityType == CapacityTypeSpot {
						override.Priority = aws.Float64(float64(i))
					}
					overrides = append(overrides, override)
					// FleetAPI cannot span subnets from the same AZ, so break after the first one.
					break
				}
			}
		}
	}
	if len(overrides) == 0 {
		return nil, fmt.Errorf("no viable {subnet, instanceType} combination")
	}

	// 2. Create fleet
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
				LaunchTemplateId: aws.String(launchTemplate.Id),
				Version:          aws.String(launchTemplate.Version),
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
	if count := len(createFleetOutput.Errors); count > 0 {
		zap.S().Debugf("CreateFleet encountered %d errors, but still launched instances, %v", count, createFleetOutput.Errors)
	}
	return createFleetOutput.Instances[0].InstanceIds[0], nil
}

func (p *InstanceProvider) Terminate(ctx context.Context, node *v1.Node) error {
	id, err := p.getInstanceID(node)
	if err != nil {
		return fmt.Errorf("getting instance ID for node %s, %w", node.Name, err)
	}
	if _, err = p.ec2api.TerminateInstancesWithContext(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []*string{id},
	}); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("terminating instance %s, %w", node.Name, err)
	}
	return nil
}

func (p *InstanceProvider) getInstanceID(node *v1.Node) (*string, error) {
	id := strings.Split(node.Spec.ProviderID, "/")
	if len(id) < 5 {
		return nil, fmt.Errorf("parsing instance id %s", node.Spec.ProviderID)
	}
	return aws.String(id[4]), nil
}
