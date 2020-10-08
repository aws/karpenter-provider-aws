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
	"testing"

	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"knative.dev/pkg/ptr"
)

type mockedUpdateAutoScalingGroup struct {
	autoscalingiface.AutoScalingAPI
	UpdateResp   autoscaling.UpdateAutoScalingGroupOutput
	DescribeResp autoscaling.DescribeAutoScalingGroupsOutput
	Error        error
}

func (m mockedUpdateAutoScalingGroup) UpdateAutoScalingGroup(*autoscaling.UpdateAutoScalingGroupInput) (*autoscaling.UpdateAutoScalingGroupOutput, error) {
	return &m.UpdateResp, m.Error
}

func (m mockedUpdateAutoScalingGroup) DescribeAutoScalingGroupsPages(input *autoscaling.DescribeAutoScalingGroupsInput, fn func(*autoscaling.DescribeAutoScalingGroupsOutput, bool) bool) error {
	fn(&m.DescribeResp, true)
	return m.Error
}

func (m mockedUpdateAutoScalingGroup) DescribeAutoScalingGroups(*autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return &m.DescribeResp, m.Error
}

func TestUpdateAutoScalingGroupSuccess(t *testing.T) {
	client := mockedUpdateAutoScalingGroup{
		UpdateResp: autoscaling.UpdateAutoScalingGroupOutput{},
		DescribeResp: autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []*autoscaling.Group{
				{
					Instances: []*autoscaling.Instance{nil, nil, nil},
				},
			},
		},
		Error: nil,
	}
	asg := &AutoScalingGroup{
		Client: client,
		ScalableNodeGroup: &v1alpha1.ScalableNodeGroup{
			Spec: v1alpha1.ScalableNodeGroupSpec{
				ID:       "spatula",
				Replicas: ptr.Int32(23),
			},
		},
	}
	err := asg.Reconcile()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
