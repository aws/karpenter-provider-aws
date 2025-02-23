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

package nodeclass

import (
	"context"
	"fmt"
	"sort"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/singleton"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/capacityreservation"
)

const capacityReservationPollPeriod = time.Minute

type CapacityReservation struct {
	provider capacityreservation.Provider
	clk      clock.Clock
	cm       *pretty.ChangeMonitor
}

func NewCapacityReservationReconciler(clk clock.Clock, provider capacityreservation.Provider) *CapacityReservation {
	return &CapacityReservation{
		provider: provider,
		clk:      clk,
		cm:       pretty.NewChangeMonitor(),
	}
}

func (c *CapacityReservation) Reconcile(ctx context.Context, nc *v1.EC2NodeClass) (reconcile.Result, error) {
	reservations, err := c.provider.List(ctx, nc.Spec.CapacityReservationSelectorTerms...)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("getting capacity reservations, %w", err)
	}
	if len(reservations) == 0 {
		nc.Status.CapacityReservations = nil
		nc.StatusConditions().SetTrue(v1.ConditionTypeCapacityReservationsReady)
		return reconcile.Result{RequeueAfter: capacityReservationPollPeriod}, nil
	}

	if ids := lo.Map(reservations, func(r *ec2types.CapacityReservation, _ int) string {
		return *r.CapacityReservationId
	}); c.cm.HasChanged(nc.Name, ids) {
		log.FromContext(ctx).V(1).WithValues("ids", ids).Info("discovered capacity reservations")
	}
	sort.Slice(reservations, func(i, j int) bool {
		return *reservations[i].CapacityReservationId < *reservations[j].CapacityReservationId
	})
	errors := []error{}
	nc.Status.CapacityReservations = []v1.CapacityReservation{}
	for _, r := range reservations {
		reservation, err := capacityReservationFromEC2(r)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		nc.Status.CapacityReservations = append(nc.Status.CapacityReservations, reservation)
	}
	if len(errors) != 0 {
		log.FromContext(ctx).WithValues(
			"error-count", len(errors),
			"total-count", len(reservations),
		).Error(multierr.Combine(errors...), "failed to parse discovered capacity reservations")
	}
	nc.StatusConditions().SetTrue(v1.ConditionTypeCapacityReservationsReady)
	return reconcile.Result{RequeueAfter: c.requeueAfter(reservations...)}, nil
}

func capacityReservationFromEC2(cr *ec2types.CapacityReservation) (v1.CapacityReservation, error) {
	// Guard against new instance match criteria added in the future. See https://github.com/kubernetes-sigs/karpenter/issues/806
	// for a similar issue.
	if !lo.Contains([]ec2types.InstanceMatchCriteria{
		ec2types.InstanceMatchCriteriaOpen,
		ec2types.InstanceMatchCriteriaTargeted,
	}, cr.InstanceMatchCriteria) {
		return v1.CapacityReservation{}, fmt.Errorf("capacity reservation %s has an unsupported instance match criteria %q", *cr.CapacityReservationId, cr.InstanceMatchCriteria)
	}
	var endTime *metav1.Time
	if cr.EndDate != nil {
		endTime = lo.ToPtr(metav1.NewTime(*cr.EndDate))
	}

	return v1.CapacityReservation{
		AvailabilityZone:      *cr.AvailabilityZone,
		EndTime:               endTime,
		ID:                    *cr.CapacityReservationId,
		InstanceMatchCriteria: string(cr.InstanceMatchCriteria),
		InstanceType:          *cr.InstanceType,
		OwnerID:               *cr.OwnerId,
	}, nil
}

// requeueAfter determines the duration until the next target reconciliation time based on the provided reservations. If
// any reservations are expected to expire before we would typically requeue, the duration will be based on the
// nearest expiration time.
func (c *CapacityReservation) requeueAfter(reservations ...*ec2types.CapacityReservation) time.Duration {
	var next *time.Time
	for _, reservation := range reservations {
		if reservation.EndDate == nil {
			continue
		}
		if next == nil {
			next = reservation.EndDate
			continue
		}
		if next.After(*reservation.EndDate) {
			next = reservation.EndDate
		}
	}
	if next == nil {
		return capacityReservationPollPeriod
	}
	if d := next.Sub(c.clk.Now()); d < capacityReservationPollPeriod {
		return lo.Ternary(d < 0, singleton.RequeueImmediately, d)
	}
	return capacityReservationPollPeriod
}
