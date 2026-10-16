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

package registrationhooks

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

// CapacityReservationGroupRegistrationHook observes initial coverage before a group-targeted node registers.
// Fleet's On-Demand market does not report whether EC2 associated the instance with a reservation.
type CapacityReservationGroupRegistrationHook struct {
	instanceProvider instance.Provider
}

func NewCapacityReservationGroupRegistrationHook(provider instance.Provider) *CapacityReservationGroupRegistrationHook {
	return &CapacityReservationGroupRegistrationHook{instanceProvider: provider}
}

func (*CapacityReservationGroupRegistrationHook) Name() string {
	return "CapacityReservationGroupRegistrationHook"
}

func (h *CapacityReservationGroupRegistrationHook) Registered(ctx context.Context, nc *karpv1.NodeClaim) (cloudprovider.NodeLifecycleHookResult, error) {
	groupARN := nc.Annotations[v1.AnnotationCapacityReservationGroupARN]
	if groupARN == "" {
		return cloudprovider.NodeLifecycleHookResult{}, nil
	}
	if nc.Status.ProviderID == "" {
		return cloudprovider.NodeLifecycleHookResult{Requeue: true}, nil
	}
	id, err := utils.ParseInstanceID(nc.Status.ProviderID)
	if err != nil {
		return cloudprovider.NodeLifecycleHookResult{}, fmt.Errorf("parsing instance ID, %w", err)
	}
	i, err := h.instanceProvider.Get(ctx, id, instance.RequireFresh)
	if err != nil {
		return cloudprovider.NodeLifecycleHookResult{}, fmt.Errorf("observing capacity reservation group association, %w", err)
	}
	if !i.HasFreshCapacityReservationGroupObservation(groupARN) {
		return cloudprovider.NodeLifecycleHookResult{Requeue: true}, nil
	}
	for label := range cloudprovider.ReservedCapacityLabels {
		delete(nc.Labels, label)
	}
	nc.Labels = lo.Assign(nc.Labels, i.CapacityReservationLabels())
	return cloudprovider.NodeLifecycleHookResult{}, nil
}
