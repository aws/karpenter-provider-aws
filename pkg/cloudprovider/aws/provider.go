package aws

import (
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
)

type AutoScalingGroupProvider struct {
}

func (a *AutoScalingGroupProvider) NewNodeGroup(id cloudprovider.NodeGroupIdentifier) (cloudprovider.NodeGroup, error) {
	return NewDefaultAutoScalingGroup(id.GroupName())
}

type ManagedNodeGroupProvider struct {
}

func (m *ManagedNodeGroupProvider) NewNodeGroup(id cloudprovider.NodeGroupIdentifier) (cloudprovider.NodeGroup, error) {
	return NewDefaultAutoScalingGroup(id.GroupName())
}
