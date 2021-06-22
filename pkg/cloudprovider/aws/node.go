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
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NodeFactory struct {
	ec2api ec2iface.EC2API
}

// For a given set of instanceIDs return a map of instanceID to Kubernetes node object.
func (n *NodeFactory) For(ctx context.Context, instanceIDs []*string) (map[string]*v1.Node, error) {
	// EC2 will return all instances if unspecified, so we must short circuit
	if len(instanceIDs) == 0 {
		return nil, nil
	}
	describeInstancesOutput, err := n.ec2api.DescribeInstancesWithContext(ctx, &ec2.DescribeInstancesInput{InstanceIds: instanceIDs})
	if err == nil {
		return n.nodesFrom(describeInstancesOutput.Reservations), nil
	}
	if aerr, ok := err.(awserr.Error); ok {
		return nil, aerr
	}
	return nil, fmt.Errorf("failed to describe ec2 instances, %w", err)
}

func (n *NodeFactory) nodesFrom(reservations []*ec2.Reservation) map[string]*v1.Node {
	nodes := map[string]*v1.Node{}
	for _, reservation := range reservations {
		for _, instance := range reservation.Instances {
			nodes[*instance.InstanceId] = n.nodeFrom(instance)
		}
	}
	return nodes
}

func (n *NodeFactory) nodeFrom(instance *ec2.Instance) *v1.Node {
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
				v1.ResourcePods:   resource.MustParse("1000"),
				v1.ResourceCPU:    resource.MustParse("96"),
				v1.ResourceMemory: resource.MustParse("384Gi"),
			},
			NodeInfo: v1.NodeSystemInfo{
				Architecture:    aws.StringValue(instance.Architecture),
				OperatingSystem: v1alpha1.OperatingSystemLinux,
			},
		},
	}
}
