package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
)

// AutoScalingGroup implements the NodeGroup CloudProvider for AWS EC2 AutoScalingGroups
type AutoScalingGroup struct {
	Name   string
	Client autoscalingiface.AutoScalingAPI
}

type AutoScalingGroupIdentifier string

func (a AutoScalingGroupIdentifier) GroupName() string {
	return string(a)
}

func (a AutoScalingGroupIdentifier) ClusterName() *string {
	return nil
}

func NewDefaultAutoScalingGroup(name string) (asg *AutoScalingGroup, err error) {
	return &AutoScalingGroup{
		Name:   name,
		Client: autoscaling.New(session.Must(session.NewSession())),
	}, nil
}

// Name returns the name of the node group
func (asg *AutoScalingGroup) Id() cloudprovider.NodeGroupIdentifier {
	return AutoScalingGroupIdentifier(asg.Name)
}

// SetReplicas sets the NodeGroups's replica count
func (asg *AutoScalingGroup) SetReplicas(value int) error {
	newSize := aws.Int64(int64(value))
	_, err := asg.Client.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asg.Name),
		MaxSize:              newSize,
		MinSize:              newSize,
		DesiredCapacity:      newSize,
	})
	return err
}
