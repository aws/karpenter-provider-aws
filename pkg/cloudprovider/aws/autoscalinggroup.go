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
*/package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
)

type AutoScalingGroupProvider struct {
}

func (a *AutoScalingGroupProvider) NewNodeGroup(name string) cloudprovider.NodeGroup {
	return NewDefaultAutoScalingGroup(name)
}

// AutoScalingGroup implements the NodeGroup CloudProvider for AWS EC2 AutoScalingGroups
type AutoScalingGroup struct {
	GroupName string
	Client    autoscalingiface.AutoScalingAPI
}

type AutoScalingGroupIdentifier string

func (a AutoScalingGroupIdentifier) GroupName() string {
	return string(a)
}

func NewDefaultAutoScalingGroup(name string) *AutoScalingGroup {
	return &AutoScalingGroup{
		GroupName: name,
		Client:    autoscaling.New(session.Must(session.NewSession())),
	}
}

// Name returns the name of the node group
func (asg *AutoScalingGroup) Name() string {
	return asg.GroupName
}

// SetReplicas sets the NodeGroups's replica count
func (asg *AutoScalingGroup) SetReplicas(value int) error {
	_, err := asg.Client.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asg.GroupName),
		DesiredCapacity:      aws.Int64(int64(value)),
	})
	return err
}
