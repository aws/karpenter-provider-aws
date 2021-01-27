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
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Capacity struct {
	ec2Iface ec2iface.EC2API
}

// NewCapacity constructs a Capacity client for AWS
func NewCapacity(client ec2iface.EC2API) *Capacity {
	return &Capacity{ec2Iface: client}
}

// Create a set of nodes given the constraints
func (cp *Capacity) Create(ctx context.Context, constraints *cloudprovider.CapacityConstraints) error {
	// Convert contraints to the Node types and select the launch template
	// TODO

	// Create the desired number of instances based on desired capacity
	// TODO remove hard coded template ID
	config := defaultInstanceConfig("lt-02f427483e1be00f5", "$Latest", cp.ec2Iface)

	if err := cp.updateConfigWithConstraints(config, constraints); err != nil {
		return fmt.Errorf("config failed to update %w", err)
	}
	// create instances using EC2 fleet API
	if err := config.validateAndCreate(ctx); err != nil {
		return err
	}
	return nil
}

// Set AvailabilityZone, subnet, capacity, on-demand or spot
func (cp *Capacity) updateConfigWithConstraints(config *instanceConfig, constraints *cloudprovider.CapacityConstraints) error {

	instanceType, count := cp.calculateInstanceTypeAndCount(constraints)
	subnetID := cp.selectSubnetID(constraints)

	config.templateConfig.Overrides[0].InstanceType = aws.String(instanceType)
	config.capacitySpec.OnDemandTargetCapacity = aws.Int64(count)
	config.capacitySpec.TotalTargetCapacity = aws.Int64(count)
	config.templateConfig.Overrides[0].SubnetId = aws.String(subnetID)
	config.templateConfig.Overrides[0].AvailabilityZone = aws.String("us-east-2a")

	return nil
}

// TODO
func (cp *Capacity) calculateInstanceTypeAndCount(constraints *cloudprovider.CapacityConstraints) (string, int64) {
	return "m5.large", 1
}

// TODO
func (cp *Capacity) selectSubnetID(constraints *cloudprovider.CapacityConstraints) string {
	return "subnet-03216d5a693377033"
}

type instanceConfig struct {
	ec2Iface       ec2iface.EC2API
	templateConfig *ec2.FleetLaunchTemplateConfigRequest
	capacitySpec   *ec2.TargetCapacitySpecificationRequest
	instanceID     string
}

func defaultInstanceConfig(templateID, templateVersion string, client ec2iface.EC2API) *instanceConfig {
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
		return fmt.Errorf("failed to create fleet %w", err)
	}
	// TODO Get instanceID from the output
	_ = output
	_ = cfg.instanceID
	return nil
}

// calculateResourceListOrDie queries EC2 API and gets the CPU & Mem for a list of instance types
func calculateResourceListOrDie(client ec2iface.EC2API, instanceType []*string) map[string]v1.ResourceList {
	output, err := client.DescribeInstanceTypes(
		&ec2.DescribeInstanceTypesInput{
			InstanceTypes: instanceType,
		},
	)
	if err != nil {
		log.PanicIfError(err, "Describe instance type request failed")
	}
	var instanceTypes = map[string]v1.ResourceList{}
	for _, instance := range output.InstanceTypes {
		resourceList := v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse(strconv.FormatInt(*instance.VCpuInfo.DefaultVCpus, 10)),
			v1.ResourceMemory: resource.MustParse(strconv.FormatInt(*instance.MemoryInfo.SizeInMiB, 10)),
		}
		instanceTypes[*instance.InstanceType] = resourceList
	}
	return instanceTypes
}
