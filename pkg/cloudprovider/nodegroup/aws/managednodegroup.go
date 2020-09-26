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
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func validate(sng *v1alpha1.ScalableNodeGroupSpec) (err error) {
	_, err = parseClusterId(sng.ID)
	return
}

func init() {
	v1alpha1.RegisterScalableNodeGroupValidator(v1alpha1.AWSEKSNodeGroup, validate)
}

// ManagedNodeGroup implements the NodeGroup CloudProvider for AWS EKS Managed Node Groups
type ManagedNodeGroup struct {
	*v1alpha1.ScalableNodeGroup
	Client    eksiface.EKSAPI
	Cluster   string
	Nodegroup string
}

func NewNodeGroup(sng *v1alpha1.ScalableNodeGroup) *ManagedNodeGroup {
	mngId, err := parseClusterId(sng.Spec.ID)
	if err != nil {
		zap.S().Fatalf("failed to instantiate ManagedNodeGroup: invalid arn %s", sng.Spec.ID)
	}
	return &ManagedNodeGroup{ScalableNodeGroup: sng,
		Cluster:   mngId.cluster,
		Nodegroup: mngId.nodegroup,
		Client:    eks.New(session.Must(session.NewSession()))}
}

type managedNodeGroupId struct {
	cluster   string
	nodegroup string
}

// parseClusterId extracts the cluster and nodegroup names from an
// ARN. This is needed for Managed Node Group APIs that don't take an
// ARN directly.
func parseClusterId(fromArn string) (*managedNodeGroupId, error) {
	nodegroupArn, err := arn.Parse(fromArn)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse GroupName %s as ARN", fromArn)
	}
	// Example node group ARN:
	// arn:aws:eks:us-west-2:741206201142:nodegroup/ridiculous-sculpture-1594766004/ng-0b663e8a/aeb9a7fe-69d6-21f0-cb41-fb9b03d3aaa9
	//                                              ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ ^^^^^^^^^^^
	//                                              |                               |
	//                                              cluster name                    nodegroup name
	components := strings.Split(nodegroupArn.Resource, "/")
	if len(components) < 3 {
		return nil, errors.Errorf("ARN resource missing components: %s", nodegroupArn.Resource)
	}
	return &managedNodeGroupId{
		cluster:   components[1],
		nodegroup: components[2],
	}, nil
}

// SetReplicas TODO(jacob@)
func (mng *ManagedNodeGroup) SetReplicas(value int) error {
	if _, err := mng.Client.UpdateNodegroupConfig(&eks.UpdateNodegroupConfigInput{
		ClusterName:   &mng.Cluster,
		NodegroupName: &mng.Nodegroup,
		ScalingConfig: &eks.NodegroupScalingConfig{
			DesiredSize: aws.Int64(int64(value)),
		},
	}); err != nil {
		return err
	}
	mng.ScalableNodeGroup.Status.Replicas = int32(value)
	return nil
}

func (mng *ManagedNodeGroup) Name() string {
	return mng.ScalableNodeGroup.Spec.ID
}
