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
	"time"

	"github.com/avast/retry-go"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

type NodeFactory struct {
	ec2api ec2iface.EC2API
}

// For a given set of instanceIDs return a map of instanceID to Kubernetes node object.
func (n *NodeFactory) For(ctx context.Context, instanceId *string) (*v1.Node, error) {
	instance := ec2.Instance{}
	// EC2 is eventually consistent, so backoff-retry until we have the data we need.
	if err := retry.Do(
		func() (err error) { return n.getInstance(ctx, instanceId, &instance) },
		retry.Delay(1 * time.Second),
		retry.Attempts(3),
	); err != nil {
		return nil, err
	}
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: aws.StringValue(instance.PrivateDnsName),
		},
		Spec: v1.NodeSpec{
			ProviderID: fmt.Sprintf("aws:///%s/%s", aws.StringValue(instance.Placement.AvailabilityZone), aws.StringValue(instance.InstanceId)),
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
				OperatingSystem: v1alpha3.OperatingSystemLinux,
			},
		},
	}, nil
}

func (n *NodeFactory) getInstance(ctx context.Context, instanceId *string, instance *ec2.Instance) error {
	describeInstancesOutput, err := n.ec2api.DescribeInstancesWithContext(ctx, &ec2.DescribeInstancesInput{InstanceIds: []*string{instanceId}})
	if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "InvalidInstanceID.NotFound" {
		return aerr
	}
	if err != nil {
		return fmt.Errorf("failed to describe ec2 instances, %w", err)
	}
	if len(describeInstancesOutput.Reservations) != 1 {
		return fmt.Errorf("expected a single instance reservation, got %d", len(describeInstancesOutput.Reservations))
	}
	if len(describeInstancesOutput.Reservations[0].Instances) != 1 {
		return fmt.Errorf("expected a single instance, got %d", len(describeInstancesOutput.Reservations[0].Instances))
	}
	*instance = *describeInstancesOutput.Reservations[0].Instances[0]
	if len(aws.StringValue(instance.PrivateDnsName)) == 0 {
		return fmt.Errorf("expected PrivateDnsName to be set")
	}
	logging.FromContext(ctx).Infof("Launched instance: %s, type: %s, zone: %s, hostname: %s",
		aws.StringValue(instance.InstanceId),
		aws.StringValue(instance.InstanceType),
		aws.StringValue(instance.Placement.AvailabilityZone),
		aws.StringValue(instance.PrivateDnsName),
	)
	return nil
}
