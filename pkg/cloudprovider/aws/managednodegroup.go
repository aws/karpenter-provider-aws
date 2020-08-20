package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
)

type ManagedNodeGroupProvider struct {
}

func (m *ManagedNodeGroupProvider) NewNodeGroup(id cloudprovider.NodeGroupIdentifier) (cloudprovider.NodeGroup, error) {
	return NewDefaultAutoScalingGroup(id.GroupName())
}

// ManagedNodeGroup implements the NodeGroup CloudProvider for AWS EKS Managed Node Groups
type ManagedNodeGroup struct {
	Client eksiface.EKSAPI
	Ident  ManagedNodeGroupIdentifier
}

type ManagedNodeGroupIdentifier struct {
	Name    string
	Cluster string
}

func (m ManagedNodeGroupIdentifier) GroupName() string {
	return m.Name
}

func (m ManagedNodeGroupIdentifier) ClusterName() *string {
	return &m.Cluster
}

func NewDefaultManagedNodeGroup(name string, cluster string) (mng *ManagedNodeGroup, err error) {
	return &ManagedNodeGroup{
		Client: eks.New(session.Must(session.NewSession())),
		Ident: ManagedNodeGroupIdentifier{
			Name:    name,
			Cluster: cluster,
		},
	}, nil
}

func (mng *ManagedNodeGroup) SetReplicas(value int) error {
	_, err := mng.Client.UpdateNodegroupConfig(&eks.UpdateNodegroupConfigInput{
		ClusterName:   aws.String(mng.Ident.Cluster),
		NodegroupName: aws.String(mng.Ident.Name),
		ScalingConfig: &eks.NodegroupScalingConfig{
			DesiredSize: aws.Int64(int64(value)),
		},
	})
	return err
}

func (mng *ManagedNodeGroup) Id() cloudprovider.NodeGroupIdentifier {
	return mng.Id()
}
