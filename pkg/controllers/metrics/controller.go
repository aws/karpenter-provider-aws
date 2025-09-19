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
	"time"

	"github.com/awslabs/operatorpkg/singleton"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
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
func (c *Controller) Reconcile(ctx context.Context) (reconcile.Result, error) {
	nodePools := &karpv1.NodePoolList{}
	if err := c.kubeClient.List(ctx, nodePools); err != nil {
		return reconcile.Result{}, err
	}
	availability := map[metricDimensions]bool{}
	price := map[metricDimensions]float64{}
	for _, nodePool := range nodePools.Items {
		instanceTypes, err := c.cloudProvider.GetInstanceTypes(ctx, &nodePool)
		if err != nil {
			return reconcile.Result{}, err
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
	return reconcile.Result{RequeueAfter: time.Minute}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("cloudprovider.metrics").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}
