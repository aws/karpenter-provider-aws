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
	"fmt"
	"sync"

	"github.com/awslabs/operatorpkg/serrors"
	"go.uber.org/multierr"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
)

var _ cloudprovider.NodeClaimStateProvider = (*CloudProvider)(nil)

// RefreshNodeClaimStates observes all supplied group consumers, across NodePools,
// using bounded concurrent reads coalesced by the instance provider's EC2 batcher.
// Per-instance failures omit only that observation; callers must treat it as unknown.
func (c *CloudProvider) RefreshNodeClaimStates(ctx context.Context, claims []*karpv1.NodeClaim) (map[string]cloudprovider.NodeClaimState, error) {
	result := map[string]cloudprovider.NodeClaimState{}
	// Preserve the shared-capacity scope on failed observations so core can defer
	// affected peers without blocking unrelated groups.
	for _, nc := range claims {
		if scope := nc.Annotations[v1.AnnotationCapacityReservationGroupARN]; scope != "" {
			result[nc.Status.ProviderID] = cloudprovider.NodeClaimState{Scope: scope}
		}
	}
	errs := make([]error, len(claims))
	var mu sync.Mutex
	workqueue.ParallelizeUntil(ctx, 16, len(claims), func(index int) {
		nc := claims[index]
		if nc.Annotations[v1.AnnotationCapacityReservationGroupARN] == "" {
			return
		}
		if !coreoptions.FromContext(ctx).FeatureGates.ReservedCapacity {
			errs[index] = serrors.Wrap(fmt.Errorf("reserved capacity feature gate is disabled"), "NodeClaim", klog.KObj(nc))
			return
		}
		observed, err := instance.ObserveCapacityReservationGroup(ctx, c.instanceProvider, nc)
		if err != nil {
			errs[index] = serrors.Wrap(err, "NodeClaim", klog.KObj(nc))
			return
		}
		price, found := c.instanceTypeProvider.OnDemandPrice(observed.Type)
		if !found {
			errs[index] = serrors.Wrap(fmt.Errorf("on-demand running-node price unavailable"), "NodeClaim", klog.KObj(nc))
			return
		}
		if observed.CapacityType == karpv1.CapacityTypeReserved {
			// Match the existing reserved-offering marginal-cost model, keeping
			// relative instance-type ordering without advertising a reserved slot.
			price /= 10_000_000.0
		}
		state := cloudprovider.NodeClaimState{
			ObservedAt: observed.ObservedAt,
			Price:      price,
			Scope:      observed.CapacityReservationGroupARN,
			Labels:     observed.CapacityReservationLabels(),
		}
		mu.Lock()
		result[nc.Status.ProviderID] = state
		mu.Unlock()
	})
	return result, multierr.Combine(errs...)
}
