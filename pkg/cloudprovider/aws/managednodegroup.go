package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
)

type ManagedNodeGroupProvider struct {
	ClusterName string
}

func (m *ManagedNodeGroupProvider) NewNodeGroup(name string) cloudprovider.NodeGroup {
	return NewDefaultManagedNodeGroup(name, m.ClusterName)
}

// ManagedNodeGroup implements the NodeGroup CloudProvider for AWS EKS Managed Node Groups
type ManagedNodeGroup struct {
	Client      eksiface.EKSAPI
	GroupName   string
	ClusterName string
}

func NewDefaultManagedNodeGroup(name string, clusterName string) *ManagedNodeGroup {
	return &ManagedNodeGroup{
		Client:      eks.New(session.Must(session.NewSession())),
		GroupName:   name,
		ClusterName: clusterName,
	}
}

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

func (mng *ManagedNodeGroup) Name() string {
	return mng.GroupName
}
