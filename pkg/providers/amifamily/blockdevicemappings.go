/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package amifamily

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	kubeletcel "github.com/aws/karpenter-provider-aws/pkg/cel"
	karpopts "github.com/aws/karpenter-provider-aws/pkg/operator/options"
)

// ResolveBlockDeviceMappings returns blockDeviceMappings with every volumeSizeExpression replaced by the
// volumeSize it evaluates to for one instance type. It is the single resolution path shared by the scheduler
// (ephemeral-storage capacity) and the launch template resolver (the volume that's created), so the two
// can't diverge. The input is never mutated; it's returned as-is when there's nothing to evaluate.
//
// varsFn is only called when an expression is present, so callers can defer building the variables.
// With the NodeClassCEL gate off, expressions aren't honored: the entry keeps no volume size, falling back
// to the AMI family default, rather than sizing a volume from an expression the user hasn't opted into.
// The validation controller rejects such a NodeClass separately; this guards the resolution path itself.
func ResolveBlockDeviceMappings(ctx context.Context, celEnv *kubeletcel.CELEnvironment, blockDeviceMappings []*v1.BlockDeviceMapping, varsFn func() (kubeletcel.InstanceTypeVars, error)) ([]*v1.BlockDeviceMapping, error) {
	if !lo.ContainsBy(blockDeviceMappings, hasVolumeSizeExpression) {
		return blockDeviceMappings, nil
	}
	gateEnabled := karpopts.FromContext(ctx).FeatureGates.NodeClassCEL
	var vars kubeletcel.InstanceTypeVars
	if gateEnabled {
		var err error
		if vars, err = varsFn(); err != nil {
			return nil, err
		}
	}
	resolved := make([]*v1.BlockDeviceMapping, 0, len(blockDeviceMappings))
	for _, bdm := range blockDeviceMappings {
		if !hasVolumeSizeExpression(bdm) {
			resolved = append(resolved, bdm)
			continue
		}
		bdm = bdm.DeepCopy()
		expression := lo.FromPtr(bdm.EBS.VolumeSizeExpression)
		bdm.EBS.VolumeSizeExpression = nil
		if gateEnabled {
			size, err := celEnv.ResolveVolumeSize(expression, vars)
			if err != nil {
				return nil, fmt.Errorf("resolving volumeSizeExpression for device %q, %w", lo.FromPtr(bdm.DeviceName), err)
			}
			bdm.EBS.VolumeSize = &size
		}
		resolved = append(resolved, bdm)
	}
	return resolved, nil
}

func hasVolumeSizeExpression(bdm *v1.BlockDeviceMapping) bool {
	return bdm != nil && bdm.EBS != nil && bdm.EBS.VolumeSizeExpression != nil
}

// serializeVolumeSizes encodes the volume size of each mapping as a comparable string, for use in the
// launch template grouping key so that instance types resolving to different volume sizes get different
// launch templates, while those resolving to the same sizes still share one.
func serializeVolumeSizes(blockDeviceMappings []*v1.BlockDeviceMapping) string {
	parts := lo.FilterMap(blockDeviceMappings, func(bdm *v1.BlockDeviceMapping, _ int) (string, bool) {
		if bdm == nil || bdm.EBS == nil || bdm.EBS.VolumeSize == nil {
			return "", false
		}
		return lo.FromPtr(bdm.DeviceName) + "=" + bdm.EBS.VolumeSize.String(), true
	})
	sort.Strings(parts)
	return strings.Join(parts, ",")
}
