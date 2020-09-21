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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
)

// ManagedNodeGroupProvider TODO(jacob@)
type ManagedNodeGroupProvider struct {
	ClusterName string
}

// NewNodeGroup TODO(jacob@)
func (m *ManagedNodeGroupProvider) NewNodeGroup(name string) cloudprovider.NodeGroup {
	return NewDefaultManagedNodeGroup(name, m.ClusterName)
}

// ManagedNodeGroup implements the NodeGroup CloudProvider for AWS EKS Managed Node Groups
type ManagedNodeGroup struct {
	Client      eksiface.EKSAPI
	GroupName   string
	ClusterName string
}

// NewDefaultManagedNodeGroup TODO(jacob@)
func NewDefaultManagedNodeGroup(name string, clusterName string) *ManagedNodeGroup {
	return &ManagedNodeGroup{
		Client:      eks.New(session.Must(session.NewSession())),
		GroupName:   name,
		ClusterName: clusterName,
	}
}

// SetReplicas TODO(jacob@)
func (mng *ManagedNodeGroup) SetReplicas(value int) error {
	_, err := mng.Client.UpdateNodegroupConfig(&eks.UpdateNodegroupConfigInput{
		ClusterName:   aws.String(mng.ClusterName),
		NodegroupName: aws.String(mng.GroupName),
		ScalingConfig: &eks.NodegroupScalingConfig{
			DesiredSize: aws.Int64(int64(value)),
		},
	})
	return err
}

// Name TODO(jacbo@)
func (mng *ManagedNodeGroup) Name() string {
	return mng.GroupName
}
