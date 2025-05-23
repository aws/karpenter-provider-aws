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

package expiration

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/operatorpkg/serrors"
	"github.com/awslabs/operatorpkg/singleton"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	"sigs.k8s.io/karpenter/pkg/utils/nodeclaim"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/capacityreservation"
)

// The capacityreservation.expiration controller handles pre-emptive termination for nodes in expiring capacity-block
// reservations. These nodes should be terminated 40 minutes before the capacity-block's end time, 10 minutes before EC2
// begins reclaiming the instances. This is inline with the interruption notification emitted by EC2.
type Controller struct {
	clk           clock.Clock
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
	crProvider    capacityreservation.Provider
}

func NewController(
	clk clock.Clock,
	kubeClient client.Client,
	cloudProvider cloudprovider.CloudProvider,
	capacityReservationProvider capacityreservation.Provider,
) *Controller {
	return &Controller{
		clk:           clk,
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
		crProvider:    capacityReservationProvider,
	}
}

func (c *Controller) Name() string {
	return "capacityreservation.expiration"
}

func (c *Controller) Reconcile(ctx context.Context) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, c.Name())
	ncs, err := nodeclaim.ListManaged(ctx, c.kubeClient, c.cloudProvider)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("listing instance types, %w", err)
	}
	ec2CRs, err := c.crProvider.List(ctx, lo.FilterMap(ncs, func(nc *karpv1.NodeClaim, _ int) (v1.CapacityReservationSelectorTerm, bool) {
		id, ok := nc.Labels[cloudprovider.ReservationIDLabel]
		return v1.CapacityReservationSelectorTerm{ID: id}, ok
	})...)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("getting capacity reservations, %w", err)
	}
	expiringCRs := sets.New(lo.FilterMap(ec2CRs, func(ec2CR *ec2types.CapacityReservation, _ int) (string, bool) {
		cr, err := v1.CapacityReservationFromEC2(c.clk, ec2CR)
		if err != nil {
			log.FromContext(ctx).WithValues("capacity-reservation-id", *ec2CR.CapacityReservationId).Error(err, "failed to parse capacity reservation")
			return "", false
		}
		return cr.ID, cr.State == v1.CapacityReservationStateExpiring
	})...)
	toDelete := lo.Filter(ncs, func(nc *karpv1.NodeClaim, _ int) bool {
		id, ok := nc.Labels[cloudprovider.ReservationIDLabel]
		if !ok {
			return false
		}
		return expiringCRs.Has(id)
	})
	errs := map[*karpv1.NodeClaim]error{}
	workqueue.ParallelizeUntil(ctx, 10, len(toDelete), func(i int) {
		if err := c.kubeClient.Delete(ctx, toDelete[i]); err != nil {
			if apierrors.IsNotFound(err) {
				return
			}
			errs[ncs[i]] = err
			return
		}
		log.FromContext(ctx).
			WithValues("NodeClaim", klog.KObj(ncs[i]), "capacity-reservation-id", toDelete[i].Labels[v1.LabelCapacityReservationID]).
			Info("initiating delete for capacity block expiration")
	})
	if len(errs) != 0 {
		return reconcile.Result{}, serrors.Wrap(
			fmt.Errorf("deleting nodeclaims, %w", multierr.Combine(lo.Values(errs)...)),
			"NodeClaims", lo.Map(lo.Keys(errs), func(nc *karpv1.NodeClaim, _ int) klog.ObjectRef {
				return klog.KObj(nc)
			}),
		)
	}
	return reconcile.Result{RequeueAfter: time.Minute}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named(c.Name()).
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}
