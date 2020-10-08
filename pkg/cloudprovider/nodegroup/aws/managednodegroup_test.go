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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"knative.dev/pkg/ptr"
)

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

func TestUpdateManagedNodeGroupSuccess(t *testing.T) {
	mng := &ManagedNodeGroup{
		EKSAPI: mockedUpdateManagedNodeGroup{
			UpdateOutput: eks.UpdateNodegroupConfigOutput{},
			DescribeOutput: eks.DescribeNodegroupOutput{
				Nodegroup: &eks.Nodegroup{
					Resources: &eks.NodegroupResources{
						AutoScalingGroups: []*eks.AutoScalingGroup{
							{Name: aws.String("asg1")},
							{Name: aws.String("asg2")},
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
		ScalableNodeGroup: &v1alpha1.ScalableNodeGroup{
			Spec: v1alpha1.ScalableNodeGroupSpec{
				Replicas: ptr.Int32(23),
				ID:       "arn:aws:eks:us-west-2:741206201142:nodegroup/ridiculous-sculpture-1594766004/ng-0b663e8a/aeb9a7fe-69d6-21f0-cb41-fb9b03d3aaa9",
			},
		},
	}

	got := mng.Reconcile()
	if got != nil {
		t.Errorf("SetReplicas(23) = %v; want nil", got)
	}

	if *mng.Status.Replicas != 6 {
		t.Errorf("asg.Status.Replicas = %d; want 6", *mng.Status.Replicas)
	}
}
