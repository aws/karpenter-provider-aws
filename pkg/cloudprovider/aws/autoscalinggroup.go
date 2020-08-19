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
	name   string
	client autoscalingiface.AutoScalingAPI
}

type AutoScalingGroupIdentifier string

func (a AutoScalingGroupIdentifier) GroupName() string {
	return string(a)
}

func (a AutoScalingGroupIdentifier) ClusterName() *string {
	return nil
}

func NewDefaultAutoScalingGroup(name string) (asg *AutoScalingGroup, err error) {
	sess := session.Must(session.NewSession())
	svc := autoscaling.New(sess)
	return NewAutoScalingGroup(svc, name), nil
}

func NewAutoScalingGroup(svc autoscalingiface.AutoScalingAPI, name string) *AutoScalingGroup {
	return &AutoScalingGroup{name: name,
		client: svc,
	}
}

// Name returns the name of the node group
func (asg *AutoScalingGroup) Id() cloudprovider.NodeGroupIdentifier {
	return AutoScalingGroupIdentifier(asg.name)
}

// SetReplicas sets the NodeGroups's replica count
func (asg *AutoScalingGroup) SetReplicas(value int) error {
	newSize := aws.Int64(int64(value))
	_, err := asg.client.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asg.name),
		MaxSize:              newSize,
		MinSize:              newSize,
		DesiredCapacity:      newSize,
	})
	return err
}
