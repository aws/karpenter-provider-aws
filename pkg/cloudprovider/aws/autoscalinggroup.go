package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
)

type AutoScalingGroupProvider struct {
}

func (a *AutoScalingGroupProvider) NewNodeGroup(name string) (cloudprovider.NodeGroup, error) {
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

func NewDefaultAutoScalingGroup(name string) (asg *AutoScalingGroup, err error) {
	return &AutoScalingGroup{
		GroupName: name,
		Client:    autoscaling.New(session.Must(session.NewSession())),
	}, nil
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
