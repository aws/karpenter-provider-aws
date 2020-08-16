package aws

import (
	//"context"

	"github.com/aws/aws-sdk-go/aws"
	//	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
)

// AutoScalingGroup implements the NodeGroup CloudProvider for AWS EC2 AutoScalingGroups
type AutoScalingGroup struct {
	name   string
	client autoscalingiface.AutoScalingAPI
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
func (asg *AutoScalingGroup) Name() string {
	return asg.name
}

// SetReplicas sets the NodeGroups's replica count
func (asg *AutoScalingGroup) SetReplicas(value int) error {
	newSize := aws.Int64(int64(value))
	_, err := asg.client.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
		MaxSize:         newSize,
		MinSize:         newSize,
		DesiredCapacity: newSize,
	})
	return err
}
