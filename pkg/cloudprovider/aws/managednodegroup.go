package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
)

// ManagedNodeGroup implements the NodeGroup CloudProvider for AWS EKS Managed Node Groups
type ManagedNodeGroup struct {
	client eksiface.EKSAPI
	id     ManagedNodeGroupIdentifier
}

type ManagedNodeGroupIdentifier struct {
	name    string
	cluster string
}

func (m ManagedNodeGroupIdentifier) GroupName() string {
	return m.name
}

func (m ManagedNodeGroupIdentifier) ClusterName() *string {
	return &m.cluster
}

func NewManagedNodeGroup(svc eksiface.EKSAPI, name string, cluster string) *ManagedNodeGroup {
	return &ManagedNodeGroup{
		client: svc,
		id: ManagedNodeGroupIdentifier{
			name:    name,
			cluster: cluster,
		},
	}
}

func NewDefaultManagedNodeGroup(name string, cluster string) (mng *ManagedNodeGroup, err error) {
	sess := session.Must(session.NewSession())
	svc := eks.New(sess)
	return &ManagedNodeGroup{client: svc, id: ManagedNodeGroupIdentifier{name: name, cluster: cluster}}, nil
}

func (mng *ManagedNodeGroup) SetReplicas(value int) error {
	newSize := aws.Int64(int64(value))
	_, err := mng.client.UpdateNodegroupConfig(&eks.UpdateNodegroupConfigInput{
		ClusterName:   aws.String(mng.id.cluster),
		NodegroupName: aws.String(mng.id.name),
		ScalingConfig: &eks.NodegroupScalingConfig{
			DesiredSize: newSize,
			MinSize:     newSize,
			MaxSize:     newSize,
		},
	})
	return err
}

func (mng *ManagedNodeGroup) Id() cloudprovider.NodeGroupIdentifier {
	return mng.Id()
}
