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
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/awslabs/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/awslabs/karpenter/pkg/utils/node"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	v1alpha1.RegisterScalableNodeGroupValidator(v1alpha1.AWSEKSNodeGroup, func(sng *v1alpha1.ScalableNodeGroupSpec) error {
		_, _, err := parseId(sng.ID)
		return err
	})
}

const (
	NodeGroupLabel = "eks.amazonaws.com/nodegroup"
)

// ManagedNodeGroup implements the NodeGroup CloudProvider for AWS EKS Managed Node Groups
type ManagedNodeGroup struct {
	Cluster           string
	NodeGroup         string
	EKSClient         eksiface.EKSAPI
	AutoscalingClient autoscalingiface.AutoScalingAPI
	Client            client.Client
}

func NewManagedNodeGroup(id string, eksClient eksiface.EKSAPI, autoscalingClient autoscalingiface.AutoScalingAPI, client client.Client) *ManagedNodeGroup {
	// Ignore error; it could only actually happen if webhook didn't
	// catch invalid ARN. In that case user will see errors from
	// reconciliation, which they can fix.
	cluster, nodeGroup, _ := parseId(id)
	return &ManagedNodeGroup{
		Cluster:           cluster,
		NodeGroup:         nodeGroup,
		EKSClient:         eksClient,
		AutoscalingClient: autoscalingClient,
		Client:            client,
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
	nodes := &v1.NodeList{}
	if err := mng.Client.List(context.Background(), nodes, client.MatchingLabels(map[string]string{NodeGroupLabel: mng.NodeGroup})); err != nil {
		return 0, fmt.Errorf("failed to list nodes for %s, %w", mng.NodeGroup, err)
	}
	var readyNodes int32 = 0
	for _, n := range nodes.Items {
		if node.IsReadyAndSchedulable(n) {
			readyNodes++
		}
	}
	return readyNodes, nil
}

func (mng *ManagedNodeGroup) SetReplicas(count int32) error {
	// https://docs.aws.amazon.com/eks/latest/APIReference/API_UpdateNodegroupConfig.html
	_, err := mng.EKSClient.UpdateNodegroupConfig(&eks.UpdateNodegroupConfigInput{
		ClusterName:   &mng.Cluster,
		NodegroupName: &mng.NodeGroup,
		ScalingConfig: &eks.NodegroupScalingConfig{
			DesiredSize: aws.Int64(int64(count)),
		},
	})
	return TransientError(err)
}

func (mng *ManagedNodeGroup) Stabilized() (bool, string, error) {
	return true, "", nil // TODO
}
