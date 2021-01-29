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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Capacity struct {
	Client         client.Client
	EC2API         ec2iface.EC2API
	LaunchTemplate *ec2.LaunchTemplate
	ZonalSubnets   map[string]*ec2.Subnet
}

// NewCapacity constructs a Capacity client for AWS
func NewCapacity(EC2API ec2iface.EC2API, EKSAPI eksiface.EKSAPI, IAMAPI iamiface.IAMAPI, client client.Client) *Capacity {
	initialization := NewInitialization(EC2API, EKSAPI, IAMAPI, client)
	return &Capacity{
		EC2API:         EC2API,
		LaunchTemplate: initialization.LaunchTemplate,
		ZonalSubnets:   initialization.ZonalSubnets,
		Client:         client,
	}
}

// Create a set of nodes given the constraints
func (c *Capacity) Create(ctx context.Context, constraints *cloudprovider.CapacityConstraints) ([]*v1.Node, error) {
	// TODO, select a zone more intelligently
	var zone string
	for zone = range c.ZonalSubnets {
	}

	createFleetOutput, err := c.EC2API.CreateFleetWithContext(context.TODO(), &ec2.CreateFleetInput{
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
	})
	if err != nil {
		return nil, fmt.Errorf("creating fleet, %w", err)
	}

	var nodes []*v1.Node
	var instanceIds []*string
	for _, instance := range createFleetOutput.Instances {
		instanceIds = append(instanceIds, instance.InstanceIds...)
	}

	// TODO, add retries to describe instances, since create fleet is eventually consistent.
	describeInstancesOutput, err := c.EC2API.DescribeInstances(&ec2.DescribeInstancesInput{InstanceIds: instanceIds})
	if err != nil {
		return nil, fmt.Errorf("describing instances %v, %w", instanceIds, err)
	}

	for _, reservation := range describeInstancesOutput.Reservations {
		for _, instance := range reservation.Instances {
			nodes = append(nodes, nodeFrom(instance))
		}
	}

	return nodes, nil
}

func nodeFrom(instance *ec2.Instance) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: *instance.PrivateDnsName,
		},
		Spec: v1.NodeSpec{
			ProviderID: fmt.Sprintf("aws:///%s/%s", *instance.Placement.AvailabilityZone, *instance.InstanceId),
		},
		Status: v1.NodeStatus{
			Allocatable: v1.ResourceList{
				// TODO, This value is necessary to avoid OutOfPods failure state. Find a way to set this (and cpu/mem) correctly
				v1.ResourcePods: resource.MustParse("100"),
			},
		},
	}
}
