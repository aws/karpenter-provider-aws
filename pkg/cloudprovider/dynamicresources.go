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

package cloudprovider

import (
	"context"

	karpoptions "sigs.k8s.io/karpenter/pkg/operator/options"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/nvidiadra"
)

func (c *CloudProvider) populateDynamicResources(ctx context.Context, nodeClass *v1.EC2NodeClass, instanceTypes []*cloudprovider.InstanceType) {
	gates := options.FromContext(ctx).FeatureGates
	if (!gates.DRANVIDIAGPU && !gates.DRAEFA) || karpoptions.FromContext(ctx).IgnoreDRARequests {
		return
	}

	// Each driver is gated separately. A disabled driver contributes nothing: a nil map reads as the
	// zero DynamicResources for every instance type.
	var nvidiaResources, efaResources map[string]cloudprovider.DynamicResources
	if gates.DRANVIDIAGPU {
		consumableCapacity, _ := nvidiadra.ParseConsumableCapacity(nodeClass.Annotations[v1.AnnotationNVIDIAConsumableCapacity])
		// NVIDIA GPU driver: DRA metadata keyed by instance type name.
		nvidiaResources = c.nvidiaDRAProvider.ResolveDynamicResources(ctx, instanceTypes, consumableCapacity)
	}
	if gates.DRAEFA {
		// dranet EFA driver: DRA metadata keyed by instance type name.
		efaResources = c.efaDRAProvider.ResolveDynamicResources(ctx, instanceTypes)
	}

	for _, it := range instanceTypes {
		// Additional DRA drivers would append their contribution here.
		contributions := []cloudprovider.DynamicResources{nvidiaResources[it.Name], efaResources[it.Name]}

		var resources cloudprovider.DynamicResources
		for _, contribution := range contributions {
			resources.ResourceSliceTemplates = append(resources.ResourceSliceTemplates, contribution.ResourceSliceTemplates...)
			resources.AttributeBindings = append(resources.AttributeBindings, contribution.AttributeBindings...)
		}
		if len(resources.ResourceSliceTemplates) > 0 {
			it.DynamicResources = resources
		}
	}
}
