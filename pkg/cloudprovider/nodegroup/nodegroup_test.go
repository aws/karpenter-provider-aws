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

package nodegroup

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/cloudprovider/nodegroup/aws"
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

type mockedUpdateManagedNodeGroup struct {
	eksiface.EKSAPI
	UpdateOutput   eks.UpdateNodegroupConfigOutput
	DescribeOutput eks.DescribeNodegroupOutput
	Error          error
}

func (m mockedUpdateManagedNodeGroup) UpdateNodegroupConfig(*eks.UpdateNodegroupConfigInput) (*eks.UpdateNodegroupConfigOutput, error) {
	return &m.UpdateOutput, m.Error
}

func (m mockedUpdateManagedNodeGroup) DescribeNodegroup(*eks.DescribeNodegroupInput) (*eks.DescribeNodegroupOutput, error) {
	return &m.DescribeOutput, m.Error
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
	asg := &aws.AutoScalingGroup{
		Client: client,
		ID:     "spatula",
	}

	ng := &DefaultNodeGroup{
		ScalableNodeGroup: &v1alpha1.ScalableNodeGroup{
			Spec: v1alpha1.ScalableNodeGroupSpec{
				Replicas: ptr.Int32(23),
			},
		},
		Provider: asg,
	}

	err := ng.Reconcile()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if *ng.Status.Replicas != 3 {
		t.Errorf("Status.Replicas = %d; want 3", *ng.Status.Replicas)
	}

}

func TestUpdateManagedNodeGroupSuccess(t *testing.T) {
	mng := &aws.ManagedNodeGroup{
		EKSAPI: mockedUpdateManagedNodeGroup{
			UpdateOutput: eks.UpdateNodegroupConfigOutput{},
			DescribeOutput: eks.DescribeNodegroupOutput{
				Nodegroup: &eks.Nodegroup{
					Resources: &eks.NodegroupResources{
						AutoScalingGroups: []*eks.AutoScalingGroup{
							{Name: ptr.String("asg1")},
							{Name: ptr.String("asg2")},
						},
					},
				},
			},
		},
		AutoScalingAPI: &mockedUpdateAutoScalingGroup{
			DescribeResp: autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: []*autoscaling.Group{
					{
						Instances: []*autoscaling.Instance{nil, nil, nil},
					},
					{
						Instances: []*autoscaling.Instance{nil, nil, nil},
					},
				},
			},
		},
		Cluster:   "ridiculous-sculpture-1594766004",
		NodeGroup: "ng-0b663e8a",
	}

	ng := &DefaultNodeGroup{
		ScalableNodeGroup: &v1alpha1.ScalableNodeGroup{
			Spec: v1alpha1.ScalableNodeGroupSpec{
				Replicas: ptr.Int32(23),
			},
		},
		Provider: mng,
	}

	got := ng.Reconcile()
	if got != nil {
		t.Errorf("Reconcile() = %v; want nil", got)
	}

	if *ng.Status.Replicas != 6 {
		t.Errorf("Status.Replicas = %d; want 6", *ng.Status.Replicas)
	}
}
