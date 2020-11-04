package fake

import (
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
)

type AutoScalingAPI struct {
	autoscalingiface.AutoScalingAPI
	UpdateOutput   autoscaling.UpdateAutoScalingGroupOutput
	DescribeOutput autoscaling.DescribeAutoScalingGroupsOutput
	WantErr        error
}

func (m AutoScalingAPI) UpdateAutoScalingGroup(*autoscaling.UpdateAutoScalingGroupInput) (*autoscaling.UpdateAutoScalingGroupOutput, error) {
	return &m.UpdateOutput, m.WantErr
}

func (m AutoScalingAPI) DescribeAutoScalingGroupsPages(input *autoscaling.DescribeAutoScalingGroupsInput, fn func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool) error {
	fn(&m.DescribeOutput, true)
	return m.WantErr
}

func (m AutoScalingAPI) DescribeAutoScalingGroups(*autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return &m.DescribeOutput, m.WantErr
}
