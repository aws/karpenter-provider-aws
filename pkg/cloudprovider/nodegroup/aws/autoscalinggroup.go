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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
)

// AutoScalingGroup implements the NodeGroup CloudProvider for AWS EC2 AutoScalingGroups
type AutoScalingGroup struct {
	ID     string
	Client autoscalingiface.AutoScalingAPI
}

func NewAutoScalingGroup(id string) *AutoScalingGroup {
	return &AutoScalingGroup{
		ID:     id,
		Client: autoscaling.New(session.Must(session.NewSession())),
	}
}

// GetReplicas returns replica count for an EC2 auto scaling group
func (asg *AutoScalingGroup) GetReplicas() (int32, error) {
	out, err := asg.Client.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{aws.String(asg.ID)},
		MaxRecords:            aws.Int64(1),
	})
	if err != nil {
		return 0, NewAWSTransientError(err)
	}
	if len(out.AutoScalingGroups) != 1 {
		return 0, fmt.Errorf("autoscaling group has no instances: %s", asg.ID)
	}
	return int32(len(out.AutoScalingGroups[0].Instances)), nil
}

func (asg *AutoScalingGroup) SetReplicas(count int32) error {
	_, err := asg.Client.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asg.ID),
		DesiredCapacity:      aws.Int64(int64(count)),
	})
	return NewAWSTransientError(err)
}
