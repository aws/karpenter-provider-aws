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

package instance

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/operatorpkg/serrors"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

// ObserveCapacityReservationGroup requires a complete fresh EC2 association for
// the persisted launch target, even if the NodeClass has changed or disappeared.
func ObserveCapacityReservationGroup(ctx context.Context, provider Provider, nc *karpv1.NodeClaim) (*Instance, error) {
	id, err := utils.ParseInstanceID(nc.Status.ProviderID)
	if err != nil {
		return nil, err
	}
	i, err := provider.Get(ctx, id, RequireFresh)
	if err != nil {
		return nil, err
	}
	if !i.HasFreshCapacityReservationGroupObservation(nc.Annotations[v1.AnnotationCapacityReservationGroupARN]) {
		return nil, serrors.Wrap(fmt.Errorf("incomplete capacity reservation group observation"), "id", id)
	}
	return i, nil
}

// HasFreshCapacityReservationGroupObservation is shared by registration and cost
// refreshes so neither treats missing target data as confirmed On-Demand fallback.
func (i *Instance) HasFreshCapacityReservationGroupObservation(groupARN string) bool {
	age := time.Since(i.ObservedAt)
	if groupARN == "" || i.CapacityReservationGroupARN != groupARN || i.ObservedAt.IsZero() || age < 0 || age > time.Minute {
		return false
	}
	return i.CapacityType == karpv1.CapacityTypeOnDemand || i.hasOrdinaryReservationCoverage()
}

func (i *Instance) hasOrdinaryReservationCoverage() bool {
	return i.CapacityType == karpv1.CapacityTypeReserved && i.CapacityReservationDetails != nil &&
		i.CapacityReservationDetails.ID != "" && i.CapacityReservationDetails.Type == v1.CapacityReservationTypeDefault && !i.CapacityReservationDetails.Interruptible
}
