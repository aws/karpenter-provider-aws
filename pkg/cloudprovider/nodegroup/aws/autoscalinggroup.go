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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
)

// AutoScalingGroup implements the NodeGroup CloudProvider for AWS EC2 AutoScalingGroups
type AutoScalingGroup struct {
	*v1alpha1.ScalableNodeGroup
	Client autoscalingiface.AutoScalingAPI
}

func NewDefaultAutoScalingGroup(sng *v1alpha1.ScalableNodeGroup) *AutoScalingGroup {
	return &AutoScalingGroup{
		ScalableNodeGroup: sng,
		Client:            autoscaling.New(session.Must(session.NewSession())),
	}
}

// Name returns the name of the node group
func (asg *AutoScalingGroup) Name() string {
	return asg.Spec.ID
}

// SetReplicas sets the NodeGroup's replica count
func (asg *AutoScalingGroup) SetReplicas(value int32) error {
	_, err := asg.Client.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asg.Spec.ID),
		DesiredCapacity:      aws.Int64(int64(value)),
	})
	return err
}
