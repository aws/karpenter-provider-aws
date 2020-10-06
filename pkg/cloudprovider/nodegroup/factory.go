package nodegroup

import (
	"fmt"

	"github.com/cloudevents/sdk-go/pkg/binding/spec"
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
	"github.com/ellistarn/karpenter/pkg/cloudprovider/nodegroup/aws"
	"github.com/ellistarn/karpenter/pkg/utils/log"
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
	log.InvariantViolated(fmt.Sprintf("Failed to instantiate node group: unexpected type  %s", spec.Type))
	return nil
}
