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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

type instanceConfig struct {
	ec2Iface       ec2iface.EC2API
	templateConfig *ec2.FleetLaunchTemplateConfigRequest
	capacitySpec   *ec2.TargetCapacitySpecificationRequest
	instanceID     string
}

func NewFleetRequest(templateID, templateVersion string, client ec2iface.EC2API) *instanceConfig {

	return &instanceConfig{
		ec2Iface: client,
		templateConfig: &ec2.FleetLaunchTemplateConfigRequest{
			LaunchTemplateSpecification: &ec2.FleetLaunchTemplateSpecificationRequest{
				LaunchTemplateId: aws.String(templateID),
				Version:          aws.String(templateVersion),
			},
			Overrides: []*ec2.FleetLaunchTemplateOverridesRequest{
				&ec2.FleetLaunchTemplateOverridesRequest{},
			},
		},
		capacitySpec: &ec2.TargetCapacitySpecificationRequest{
			DefaultTargetCapacityType: aws.String(ec2.DefaultTargetCapacityTypeOnDemand),
		},
	}
}

func (cfg *instanceConfig) SetAvailabilityZone(zone string) {
	cfg.templateConfig.Overrides[0].AvailabilityZone = aws.String(zone)
}

func (cfg *instanceConfig) SetSubnet(subnetID string) {
	cfg.templateConfig.Overrides[0].SubnetId = aws.String(subnetID)
}

func (cfg *instanceConfig) SetOnDemandCapacity(targetCap, totalCap int64) {
	cfg.capacitySpec.OnDemandTargetCapacity = aws.Int64(targetCap)
	cfg.capacitySpec.TotalTargetCapacity = aws.Int64(totalCap)
}

func (cfg *instanceConfig) SetInstanceType(instanceType string) {
	cfg.capacitySpec.DefaultTargetCapacityType = aws.String(instanceType)
}

func (cfg *instanceConfig) Create(ctx context.Context) error {
	return cfg.validateAndCreate(ctx)
}

func (cfg *instanceConfig) validateAndCreate(ctx context.Context) error {
	input := &ec2.CreateFleetInput{
		LaunchTemplateConfigs:       []*ec2.FleetLaunchTemplateConfigRequest{cfg.templateConfig},
		TargetCapacitySpecification: cfg.capacitySpec,
		Type:                        aws.String(ec2.FleetTypeInstant),
	}
	if err := input.Validate(); err != nil {
		return err
	}
	output, err := cfg.ec2Iface.CreateFleetWithContext(ctx, input)
	if err != nil {
		return err
	}
	// TODO Get instanceID from the output
	_ = output
	return nil
}
