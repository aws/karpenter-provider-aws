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

type nullProvider struct{}

var (
	nullError = fmt.Errorf("unknown nodegroup provider")
)

func (*nullProvider) SetReplicas(count int) error { return nullError }
func (*nullProvider) GetReplicas() (int, error)   { return 0, nullError }

func (f *Factory) For(sng *v1alpha1.ScalableNodeGroup) cloudprovider.NodeGroup {
	var nodegroup cloudprovider.NodeGroup
	switch sng.Spec.Type {
	case v1alpha1.AWSEC2AutoScalingGroup:
		nodegroup = aws.NewAutoScalingGroup(sng.Spec.ID)
	case v1alpha1.AWSEKSNodeGroup:
		nodegroup = aws.NewManagedNodeGroup(sng.Spec.ID)
	default:
		log.InvariantViolated(fmt.Sprintf("Failed to instantiate node group of type %s", spec.Type))
		nodegroup = &nullProvider{}
	}
	return nodegroup
}
