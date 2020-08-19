package aws

import (
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
)

type AutoScalingGroupProvider struct {
}

func (a *AutoScalingGroupProvider) NewNodeGroup(id cloudprovider.NodeGroupIdentifier) (cloudprovider.NodeGroup, error) {
	return NewDefaultAutoScalingGroup(id.GroupName())
}
