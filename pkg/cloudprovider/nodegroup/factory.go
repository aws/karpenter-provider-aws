package nodegroup

import (
	"fmt"

	"github.com/cloudevents/sdk-go/pkg/binding/spec"
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
	"github.com/ellistarn/karpenter/pkg/cloudprovider/nodegroup/aws"
	"github.com/ellistarn/karpenter/pkg/utils/log"
	"github.com/pkg/errors"
	"knative.dev/pkg/ptr"
)

type Factory struct {
	// TODO dependencies
}

type DefaultNodeGroup struct {
	*v1alpha1.ScalableNodeGroup
	Provider cloudprovider.ProviderNodeGroup
}

type nullProvider struct{}

var (
	nullError = fmt.Errorf("unknown nodegroup provider")
)

func (*nullProvider) SetReplicas(count int) error { return nullError }
func (*nullProvider) GetReplicas() (int, error)   { return 0, nullError }

// Reconcile is intended to be used by a controller. It tries to
// update the replica count in the status. If that works, it then
// tries to set the desired replica count if it diverges from actual.
// This is the default behavior that should work for all known cases,
// but could be overrideen for a specific cloud provider if need be.
func (dng *DefaultNodeGroup) Reconcile() error {
	replicas, err := dng.Provider.GetReplicas()
	if err != nil {
		return errors.Wrapf(err, "unable to get replica count for node group: %v", dng.Spec.ID)
	}
	dng.Status.Replicas = ptr.Int32(int32(replicas))
	if dng.Spec.Replicas == nil || *dng.Spec.Replicas == int32(replicas) {
		return nil
	}
	if err := dng.Provider.SetReplicas(replicas); err != nil {
		return errors.Wrapf(err, "unable to set replicas for node group: %v", dng.Spec.ID)
	}
	return nil
}

func (f *Factory) For(sng *v1alpha1.ScalableNodeGroup) cloudprovider.NodeGroup {
	var provider cloudprovider.ProviderNodeGroup
	switch sng.Spec.Type {
	case v1alpha1.AWSEC2AutoScalingGroup:
		provider = aws.NewAutoScalingGroup(sng.Spec.ID)
	case v1alpha1.AWSEKSNodeGroup:
		provider = aws.NewManagedNodeGroup(sng.Spec.ID)
	default:
		log.InvariantViolated(fmt.Sprintf("Unknown node group type %s", spec.Type))
	}
	if provider == nil {
		log.InvariantViolated(fmt.Sprintf("Failed to instantiate node group of type %s", spec.Type))
		provider = &nullProvider{}

	}
	return &DefaultNodeGroup{
		ScalableNodeGroup: sng,
		Provider:          provider,
	}
}
