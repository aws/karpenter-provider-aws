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

package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/operatorpkg/reconciler"
	"github.com/awslabs/operatorpkg/serrors"
	"github.com/awslabs/operatorpkg/singleton"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

type metricDimensions struct {
	instanceType string
	capacityType string
	zone         string
}

type Controller struct {
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
}

func NewController(kubeClient client.Client, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
	}
}

//nolint:gocyclo
func (c *Controller) Reconcile(ctx context.Context) (reconciler.Result, error) {
	nodePools := &karpv1.NodePoolList{}
	if err := c.kubeClient.List(ctx, nodePools); err != nil {
		return reconciler.Result{}, err
	}
	availability := map[metricDimensions]bool{}
	price := map[metricDimensions]float64{}
	var errs error
	for _, nodePool := range nodePools.Items {
		instanceTypes, err := c.cloudProvider.GetInstanceTypes(ctx, &nodePool)
		if err != nil {
			// Keep going rather than abandoning the reconcile. A NodePool whose instance types can't be
			// resolved -- an undecodable spec.kubelet, for instance -- would otherwise suppress the
			// pricing and availability metrics of every other NodePool in the cluster, for as long as
			// that one NodePool stayed broken. The error is still returned below so the failure is
			// visible and retried.
			errs = multierr.Append(errs, serrors.Wrap(fmt.Errorf("getting instance types, %w", err), "NodePool", klog.KRef("", nodePool.Name)))
			continue
		}
		for _, instanceType := range instanceTypes {
			zones := sets.New[string]()
			for _, offering := range instanceType.Offerings {
				dimensions := metricDimensions{instanceType: instanceType.Name, capacityType: offering.CapacityType(), zone: offering.Zone()}
				availability[dimensions] = availability[dimensions] || offering.Available
				price[dimensions] = offering.Price
				zones.Insert(offering.Zone())
			}
			if coreoptions.FromContext(ctx).FeatureGates.ReservedCapacity {
				for zone := range zones {
					dimensions := metricDimensions{instanceType: instanceType.Name, capacityType: karpv1.CapacityTypeReserved, zone: zone}
					if _, ok := availability[dimensions]; !ok {
						availability[dimensions] = false
						price[dimensions] = 0
					}
				}
			}
		}
	}

	for dimensions, available := range availability {
		InstanceTypeOfferingAvailable.Set(float64(lo.Ternary(available, 1, 0)), map[string]string{
			instanceTypeLabel: dimensions.instanceType,
			capacityTypeLabel: dimensions.capacityType,
			zoneLabel:         dimensions.zone,
		})
	}
	for dimensions, p := range price {
		InstanceTypeOfferingPriceEstimate.Set(p, map[string]string{
			instanceTypeLabel: dimensions.instanceType,
			capacityTypeLabel: dimensions.capacityType,
			zoneLabel:         dimensions.zone,
		})
	}
	// Metrics for the NodePools that did resolve are published above before this returns, so one broken
	// NodePool costs only its own series rather than the whole cluster's.
	if errs != nil {
		return reconciler.Result{}, errs
	}
	// The requeue is a backstop for inputs that have no watch: pricing, ICE and capacity reservation counts.
	// It also covers NodeOverlay prices, zonal shift, placement groups and the periodic instance type refresh.
	return reconciler.Result{RequeueAfter: time.Minute}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	// Every event maps to the singleton's request, so a burst of changes collapses into one reconcile.
	enqueue := handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
		return []reconcile.Request{{}}
	})
	return controllerruntime.NewControllerManagedBy(m).
		Named("cloudprovider.metrics").
		WatchesRawSource(singleton.Source()).
		Watches(&karpv1.NodePool{}, enqueue, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&v1.EC2NodeClass{}, enqueue, builder.WithPredicates(NodeClassChangedPredicate())).
		Complete(singleton.AsReconciler(c))
}

// NodeClassChangedPredicate passes EC2NodeClass spec changes and changes to the resolved status fields that feed
// instance type offerings. Condition and other status writes are dropped.
func NodeClassChangedPredicate() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			o, okOld := e.ObjectOld.(*v1.EC2NodeClass)
			n, okNew := e.ObjectNew.(*v1.EC2NodeClass)
			if !okOld || !okNew {
				return true
			}
			// The subnet reconciler sorts by free IPs, so the order changes without changing the offerings.
			// AMIs and capacity reservations are written in a stable order.
			return o.Generation != n.Generation ||
				!sets.New(o.Status.Subnets...).Equal(sets.New(n.Status.Subnets...)) ||
				!equality.Semantic.DeepEqual(o.Status.AMIs, n.Status.AMIs) ||
				!equality.Semantic.DeepEqual(o.Status.CapacityReservations, n.Status.CapacityReservations)
		},
	}
}
