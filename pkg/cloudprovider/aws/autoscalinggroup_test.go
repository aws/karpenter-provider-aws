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
	return &m.Resp, m.Error
}

func TestUpdateAutoScalingGroupSuccess(t *testing.T) {
	client := mockedUpdateAutoScalingGroup{
		Resp:  autoscaling.UpdateAutoScalingGroupOutput{},
		Error: nil,
	}
	asg := &AutoScalingGroup{
		Client:    client,
		GroupName: "spatula",
	}
	err := asg.SetReplicas(23)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
