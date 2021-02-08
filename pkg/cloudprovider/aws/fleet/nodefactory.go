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
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"go.uber.org/zap"
	"gopkg.in/retry.v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NodeFactory struct {
	ec2 ec2iface.EC2API
}

// For a given set of instanceIds return a map of instanceID to Kubernetes node object.
func (n *NodeFactory) For(ctx context.Context, instanceIds []*string) (map[string]*v1.Node, error) {
	// Backoff retry is necessary here because EC2's APIs are eventually
	// consistent. In most cases, this call will only be made once.
	for attempt := retry.Start(retry.Exponential{
		Initial:  1 * time.Second,
		MaxDelay: 10 * time.Second,
		Factor:   2, Jitter: true,
	}, nil); attempt.Next(); {
		describeInstancesOutput, err := n.ec2.DescribeInstances(&ec2.DescribeInstancesInput{InstanceIds: instanceIds})
		if err == nil {
			return n.nodesFrom(describeInstancesOutput.Reservations), nil
		}
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() != "InvalidInstanceID.NotFound" {
			return nil, aerr
		}
		zap.S().Infof("Retrying DescribeInstances due to eventual consistency: fleet created, but instances not yet found.")
	}
	return nil, fmt.Errorf("failed to describe ec2 instances")
}

func (n *NodeFactory) nodesFrom(reservations []*ec2.Reservation) map[string]*v1.Node {
	var nodes map[string]*v1.Node
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
				v1.ResourcePods:   resource.MustParse("100"),
				v1.ResourceCPU:    resource.MustParse("2"),
				v1.ResourceMemory: resource.MustParse("2Gi"),
			},
		},
	}
}
