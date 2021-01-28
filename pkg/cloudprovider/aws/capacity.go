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
	"go.uber.org/zap"
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
	// TODO Convert contraints to the Node types and select the launch template

	// Create the desired number of instances based on constraints
	// create instances using EC2 fleet API
	// TODO remove hard coded values
	output, err := cp.ec2Iface.CreateFleetWithContext(ctx, &ec2.CreateFleetInput{
		LaunchTemplateConfigs: []*ec2.FleetLaunchTemplateConfigRequest{
			{
				LaunchTemplateSpecification: &ec2.FleetLaunchTemplateSpecificationRequest{
					LaunchTemplateId: aws.String("lt-02f427483e1be00f5"),
					Version:          aws.String("$Latest"),
				},
				Overrides: []*ec2.FleetLaunchTemplateOverridesRequest{
					{
						InstanceType:     aws.String("m5.large"),
						SubnetId:         aws.String("subnet-03216d5a693377033"),
						AvailabilityZone: aws.String("us-east-2a"),
					},
				},
			},
		},
		TargetCapacitySpecification: &ec2.TargetCapacitySpecificationRequest{
			DefaultTargetCapacityType: aws.String(ec2.DefaultTargetCapacityTypeOnDemand),
			OnDemandTargetCapacity:    aws.Int64(1),
			TotalTargetCapacity:       aws.Int64(1),
		},
		Type: aws.String(ec2.FleetTypeInstant),
	})
	if err != nil {
		return fmt.Errorf("failed to create fleet %w", err)
	}
	// TODO Get instanceID from the output
	_ = output
	// _ = cfg.instanceID
	zap.S().Infof("Successfully created a node in zone %v", constraints.Zone)
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
