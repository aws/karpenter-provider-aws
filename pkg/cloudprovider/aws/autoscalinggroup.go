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
	"fmt"
	"strings"

	"github.com/awslabs/karpenter/pkg/apis/autoscaling/v1alpha1"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
)

// AutoScalingGroup implements the NodeGroup CloudProvider for AWS EC2 AutoScalingGroups
type AutoScalingGroup struct {
	ID     string
	Client autoscalingiface.AutoScalingAPI
}

func NewAutoScalingGroup(id string, client autoscalingiface.AutoScalingAPI) *AutoScalingGroup {
	normalized, _ := normalizeID(id)
	return &AutoScalingGroup{
		ID:     normalized,
		Client: client,
	}
}

func init() {
	v1alpha1.RegisterScalableNodeGroupValidator(v1alpha1.AWSEKSNodeGroup, func(sng *v1alpha1.ScalableNodeGroupSpec) error {
		_, err := normalizeID(sng.ID)
		return err
	})
}

// normalizeID extracts the name of the ASG from an ARN; ASG API calls
// need the name and do not work on an ARN, but users will often want
// to specify an ARN in their YAML. Returns fromArn unchanged if it
// does not appear to be a valid ASG ARN, in which case, it is either
// just a flat-out invalid ARN that won't work anyway, or else (more
// likely) already a valid name.
func normalizeID(id string) (string, error) {
	asgArn, err := arn.Parse(id)
	if err != nil {
		return id, nil
	}

	// ARN: arn:aws:autoscaling:region:123456789012:autoScalingGroup:uuid:autoScalingGroupName/asg-name
	// Resource: autoScalingGroup:uuid:autoScalingGroupName/asg-name
	resource := strings.Split(asgArn.Resource, ":")
	if len(resource) < 3 || resource[0] != "autoScalingGroup" {
		return id, fmt.Errorf("%s: is not an autoScalingGroup ARN", id)
	}

	nameSpecifier := strings.Split(resource[2], "/")
	if len(nameSpecifier) != 2 || nameSpecifier[0] != "autoScalingGroupName" {
		return id, fmt.Errorf("%s: does not contain autoScalingGroupName", id)
	}

	return nameSpecifier[1], nil

}

// GetReplicas returns replica count for an EC2 auto scaling group
func (a *AutoScalingGroup) GetReplicas() (int32, error) {
	out, err := a.Client.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{aws.String(a.ID)},
		MaxRecords:            aws.Int64(1),
	})
	if err != nil {
		return 0, TransientError(err)
	}
	if len(out.AutoScalingGroups) != 1 {
		return 0, fmt.Errorf("autoscaling group has no instances: %s", a.ID)
	}

	var readyReplicas int32 = 0
	for _, instance := range out.AutoScalingGroups[0].Instances {
		if instance.HealthStatus != nil && *instance.HealthStatus == "Healthy" &&
			instance.LifecycleState != nil && *instance.LifecycleState == autoscaling.LifecycleStateInService {
			readyReplicas++
		}
	}

	return int32(readyReplicas), nil
}

func (a *AutoScalingGroup) SetReplicas(count int32) error {
	_, err := a.Client.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(a.ID),
		DesiredCapacity:      aws.Int64(int64(count)),
	})
	return TransientError(err)
}

func (a *AutoScalingGroup) Stabilized() (bool, string, error) {
	return true, "", nil // TODO
}
