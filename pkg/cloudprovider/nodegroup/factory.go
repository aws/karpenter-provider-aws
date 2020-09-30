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

func (f *Factory) For(sng *v1alpha1.ScalableNodeGroup) cloudprovider.NodeGroup {
	switch sng.Spec.Type {
	case v1alpha1.AWSEC2AutoScalingGroup:
		return aws.NewDefaultAutoScalingGroup(sng.Spec.ID)
	case v1alpha1.AWSEKSNodeGroup:
		return aws.NewNodeGroup(sng)
	}
	zap.S().Fatalf("Failed to instantiate node group: unexpected type  %s", sng.Spec.Type)
	return nil
}
