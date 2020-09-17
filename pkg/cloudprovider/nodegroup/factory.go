package nodegroup

import (
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
	"github.com/ellistarn/karpenter/pkg/cloudprovider/nodegroup/aws"
	"go.uber.org/zap"
)

type Factory struct {
	// TODO dependencies
}

func (f *Factory) For(spec v1alpha1.ScalableNodeGroupSpec) cloudprovider.NodeGroup {
	switch spec.Type {
	case v1alpha1.AWSEC2AutoScalingGroup:
		return aws.NewDefaultAutoScalingGroup(spec.ID)
	case v1alpha1.AWSEKSNodeGroup:
		return aws.NewDefaultManagedNodeGroup(spec.ID, "")
	}
	zap.S().Fatalf("Failed to instantiate node group: unexpected type  %s", spec.Type)
	return nil
}
