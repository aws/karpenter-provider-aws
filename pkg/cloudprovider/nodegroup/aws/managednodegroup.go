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
)

func validate(sng *v1alpha1.ScalableNodeGroup) error {
	if sng.Spec.Type != v1alpha1.AWSEKSNodeGroup {
		return nil
	} else {
		_, err := parseClusterId(sng.Spec.ID)
		return err
	}
}

func init() {
	v1alpha1.RegisterScalableNodeGroupValidator(validate)
}

// ManagedNodeGroup implements the NodeGroup CloudProvider for AWS EKS Managed Node Groups
type ManagedNodeGroup struct {
	ARN    string
	Client eksiface.EKSAPI
}

func NewNodeGroup(arn string) *ManagedNodeGroup {
	return &ManagedNodeGroup{
		Client: eks.New(session.Must(session.NewSession())),
		ARN:    arn,
	}
}

type clusterId struct {
	clusterName   string
	nodegroupName string
}

// parseClusterId extracts the cluster and nodegroup names from an
// ARN. This is needed for Managed Node Group APIs that don't take an
// ARN directly.
func parseClusterId(fromArn string) (*clusterId, error) {
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
	return &clusterId{
		clusterName:   components[1],
		nodegroupName: components[2],
	}, nil
}

// SetReplicas TODO(jacob@)
func (mng *ManagedNodeGroup) SetReplicas(value int) error {
	id, err := parseClusterId(mng.ARN)
	if err != nil {
		return errors.Wrapf(err, "unable to parse ARN %s", mng.ARN)
	}
	_, err = mng.Client.UpdateNodegroupConfig(&eks.UpdateNodegroupConfigInput{
		ClusterName:   &id.clusterName,
		NodegroupName: &id.nodegroupName,
		ScalingConfig: &eks.NodegroupScalingConfig{
			DesiredSize: aws.Int64(int64(value)),
		},
	})
	return err
}

// Name TODO(jacob@)
func (mng *ManagedNodeGroup) Name() string {
	return mng.ARN
}
