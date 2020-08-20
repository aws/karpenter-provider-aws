package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
)

type mockedUpdateAutoScalingGroup struct {
	autoscalingiface.AutoScalingAPI
	Resp  autoscaling.UpdateAutoScalingGroupOutput
	Error error
}

func (m mockedUpdateAutoScalingGroup) UpdateAutoScalingGroup(*autoscaling.UpdateAutoScalingGroupInput) (*autoscaling.UpdateAutoScalingGroupOutput, error) {
	if m.Error != nil {
		return nil, m.Error
	} else {
		return &m.Resp, nil
	}
}

func TestUpdateAutoScalingGroupSuccess(t *testing.T) {
	client := mockedUpdateAutoScalingGroup{
		Resp:  autoscaling.UpdateAutoScalingGroupOutput{},
		Error: nil,
	}
	asg := &AutoScalingGroup{
		Client: client,
		Name:   "spatula",
	}
	err := asg.SetReplicas(23)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
