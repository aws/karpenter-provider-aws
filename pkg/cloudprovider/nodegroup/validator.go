package nodegroup

import (
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/pkg/errors"
)

var NodeGroupValidators map[v1alpha1.NodeGroupType]func(v1alpha1.ScalableNodeGroup) error

func Validate(sng v1alpha1.ScalableNodeGroup) error {
	nodeGroupType := sng.Spec.Type
	if validator, ok := NodeGroupValidators[nodeGroupType]; !ok {
		return errors.Errorf("Unknown type %s", string(nodeGroupType))
	} else {
		return validator(sng)
	}
}
