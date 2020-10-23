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
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	//	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
)

func Validate(sng *v1alpha1.ScalableNodeGroupSpec) (err error) {
	_, _, err = parseId(sng.ID)
	return
}

func init() {
	v1alpha1.RegisterScalableNodeGroupValidator(v1alpha1.AWSEKSNodeGroup, Validate)
}

// ManagedNodeGroup implements the NodeGroup CloudProvider for AWS EKS Managed Node Groups
type ManagedNodeGroup struct {
	EKSAPI         eksiface.EKSAPI
	AutoScalingAPI autoscalingiface.AutoScalingAPI
	Cluster        string
	NodeGroup      string
}

func NewManagedNodeGroup(id string) *ManagedNodeGroup {
	// Ignore error; it could only actually happen if webhook didn't
	// catch invalid ARN. In that case user will see errors from
	// reconciliation, which they can fix.
	cluster, nodeGroup, _ := parseId(id)
	session := session.Must(session.NewSession())
	return &ManagedNodeGroup{
		Cluster:        cluster,
		NodeGroup:      nodeGroup,
		EKSAPI:         eks.New(session),
		AutoScalingAPI: autoscaling.New(session),
	}
}

// parseId extracts the cluster and nodegroup from an ARN. This is
// needed for Managed Node Group APIs that don't take an ARN directly.
func parseId(fromArn string) (cluster string, nodegroup string, err error) {
	nodeGroupArn, err := arn.Parse(fromArn)
	if err != nil {
		return "", "", fmt.Errorf("invalid managed node group id %s, %w", fromArn, err)
	}
	// Example node group ARN:
	// arn:aws:eks:us-west-2:741206201142:nodegroup/ridiculous-sculpture-1594766004/ng-0b663e8a/aeb9a7fe-69d6-21f0-cb41-fb9b03d3aaa9
	//                                              ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ ^^^^^^^^^^^
	//                                              |                               |
	//                                              cluster name                    nodegroup name
	components := strings.Split(nodeGroupArn.Resource, "/")
	if len(components) < 3 {
		return "", "", fmt.Errorf("invalid managed node group id %s", fromArn)
	}
	return components[1], components[2], nil
}

func (mng *ManagedNodeGroup) GetReplicas() (int32, error) {
	nodegroupOutput, err := mng.EKSAPI.DescribeNodegroup(&eks.DescribeNodegroupInput{
		ClusterName:   &mng.Cluster,
		NodegroupName: &mng.NodeGroup,
	})
	if err != nil {
		return 0, Transientify(err)
	}

	var autoscalingGroupNames = []*string{}
	for _, group := range nodegroupOutput.Nodegroup.Resources.AutoScalingGroups {
		autoscalingGroupNames = append(autoscalingGroupNames, group.Name)
	}

	var replicas = 0
	err = mng.AutoScalingAPI.DescribeAutoScalingGroupsPages(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: autoscalingGroupNames,
	}, func(page *autoscaling.DescribeAutoScalingGroupsOutput, _ bool) bool {
		for _, group := range page.AutoScalingGroups {
			replicas += len(group.Instances)
		}
		return true
	})
	return int32(replicas), Transientify(err)
}

func (mng *ManagedNodeGroup) SetReplicas(count int32) error {
	// https://docs.aws.amazon.com/eks/latest/APIReference/API_UpdateNodegroupConfig.html
	_, err := mng.EKSAPI.UpdateNodegroupConfig(&eks.UpdateNodegroupConfigInput{
		ClusterName:   &mng.Cluster,
		NodegroupName: &mng.NodeGroup,
		ScalingConfig: &eks.NodegroupScalingConfig{
			DesiredSize: aws.Int64(int64(count)),
		},
	})

	return Transientify(err)
}
