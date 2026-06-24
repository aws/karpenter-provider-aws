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

package offering

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/placementgroup"
)

// PlacementGroupResolver expands each offering into N offerings (one per partition) for partition placement groups.
// This enables the scheduler to use TopologySpreadConstraints with the partition topology key.
type PlacementGroupResolver struct{}

func (r *PlacementGroupResolver) ResolveOfferings(
	_ context.Context,
	_ *cloudprovider.InstanceType,
	offerings cloudprovider.Offerings,
	resolverCtx *OfferingResolverContext,
) cloudprovider.Offerings {
	if resolverCtx.PlacementGroup == nil || resolverCtx.PlacementGroup.Strategy != placementgroup.StrategyPartition {
		return offerings
	}
	partitionCount := int(resolverCtx.PlacementGroup.PartitionCount)
	if partitionCount <= 0 {
		return offerings
	}
	expanded := make([]*cloudprovider.Offering, 0, len(offerings)*partitionCount)
	for _, offering := range offerings {
		for partition := 1; partition <= partitionCount; partition++ {
			reqs := scheduling.NewRequirements(offering.Requirements.Values()...)
			reqs.Add(scheduling.NewRequirement(v1.LabelPlacementGroupPartition, corev1.NodeSelectorOpIn, fmt.Sprintf("%d", partition)))
			expanded = append(expanded, &cloudprovider.Offering{
				Requirements:        reqs,
				Price:               offering.Price,
				Available:           offering.Available,
				ReservationCapacity: offering.ReservationCapacity,
				CapacityOverride:    offering.CapacityOverride,
				OverheadOverride:    offering.OverheadOverride,
			})
		}
	}
	return expanded
}
