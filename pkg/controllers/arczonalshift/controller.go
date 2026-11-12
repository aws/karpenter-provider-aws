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

package arczonalshift

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/operatorpkg/reconciler"
	"github.com/awslabs/operatorpkg/singleton"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	cloudproviderevents "github.com/aws/karpenter-provider-aws/pkg/cloudprovider/events"
	"github.com/aws/karpenter-provider-aws/pkg/providers/arczonalshift"
)

type Controller struct {
	kubeClient            client.Client
	recorder              events.Recorder
	arczonalshiftProvider arczonalshift.Provider

	previousShiftedZones sets.Set[string] // zone-id
}

func NewController(
	kubeClient client.Client,
	recorder events.Recorder,
	arczonalshiftProvider arczonalshift.Provider,
) *Controller {
	return &Controller{
		kubeClient:            kubeClient,
		recorder:              recorder,
		arczonalshiftProvider: arczonalshiftProvider,
		previousShiftedZones:  sets.New[string](),
	}
}

func (c *Controller) Reconcile(ctx context.Context) (reconciler.Result, error) {
	ctx = injection.WithControllerName(ctx, "zonalshift")
	err := c.arczonalshiftProvider.UpdateZonalShifts(ctx)
	if err != nil {
		return reconciler.Result{}, fmt.Errorf("updating zonal shifts: %w", err)
	}

	currentShiftedZones := c.arczonalshiftProvider.ShiftedZones()
	if !currentShiftedZones.Equal(c.previousShiftedZones) {
		zoneInfosByNodePool, err := c.zoneInfosByNodePool(ctx)
		if err != nil {
			return reconciler.Result{}, fmt.Errorf("publishing zonal shift events: %w", err)
		}
		PublishZonalShiftEvents(c.recorder, c.previousShiftedZones, currentShiftedZones, zoneInfosByNodePool)
	}
	// Advance only after a successful publish, so a failed publish retries the same transitions next reconcile.
	c.previousShiftedZones = currentShiftedZones

	return reconciler.Result{RequeueAfter: 30 * time.Second}, nil
}

func (c *Controller) zoneInfosByNodePool(ctx context.Context) (map[*karpv1.NodePool][]v1.ZoneInfo, error) {
	nodePools := &karpv1.NodePoolList{}
	if err := c.kubeClient.List(ctx, nodePools); err != nil {
		return nil, fmt.Errorf("listing nodepools, %w", err)
	}
	zoneInfosByNodePool := map[*karpv1.NodePool][]v1.ZoneInfo{}
	for i := range nodePools.Items {
		nodePool := &nodePools.Items[i]
		if nodePool.Spec.Template.Spec.NodeClassRef == nil {
			continue
		}
		nodeClass := &v1.EC2NodeClass{}
		if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: nodePool.Spec.Template.Spec.NodeClassRef.Name}, nodeClass); err != nil {
			return nil, fmt.Errorf("getting nodeclass %q for nodepool %q, %w", nodePool.Spec.Template.Spec.NodeClassRef.Name, nodePool.Name, err)
		}
		zoneInfosByNodePool[nodePool] = nodeClass.ZoneInfo()
	}
	return zoneInfosByNodePool, nil
}

// PublishZonalShiftEvents takes zoneInfosByNodePool as input so it stays decoupled from the concrete
// NodeClass type a caller resolves it from.
func PublishZonalShiftEvents(recorder events.Recorder, previous, current sets.Set[string], zoneInfosByNodePool map[*karpv1.NodePool][]v1.ZoneInfo) {
	shifted := current.Difference(previous)
	cleared := previous.Difference(current)
	for nodePool, zoneInfos := range zoneInfosByNodePool {
		requirements := scheduling.NewNodeSelectorRequirementsWithMinValues(nodePool.Spec.Template.Spec.Requirements...)
		for zoneID := range shifted {
			if zoneName, affected := zoneAffected(requirements, zoneInfos, zoneID); affected {
				recorder.Publish(cloudproviderevents.NodePoolZonalShiftDetected(nodePool, zoneName, zoneID))
			}
		}
		for zoneID := range cleared {
			if zoneName, affected := zoneAffected(requirements, zoneInfos, zoneID); affected {
				recorder.Publish(cloudproviderevents.NodePoolZonalShiftCleared(nodePool, zoneName, zoneID))
			}
		}
	}
}

// zoneAffected returns the shifted zone's name and whether a NodePool with the given requirements can provision
// into it. Shifts are keyed by zone ID while NodePool requirements are typically expressed by zone name, so
// zoneInfos (from the NodeClass subnets) bridges the two. Empty requirements match any reachable zone.
func zoneAffected(requirements scheduling.Requirements, zoneInfos []v1.ZoneInfo, shiftedZoneID string) (string, bool) {
	zoneInfo, found := lo.Find(zoneInfos, func(zi v1.ZoneInfo) bool {
		return zi.ZoneID == shiftedZoneID
	})
	if !found {
		return "", false
	}
	if !requirements.Get(corev1.LabelTopologyZone).Has(zoneInfo.Zone) {
		return "", false
	}
	if !requirements.Get(v1.LabelTopologyZoneID).Has(shiftedZoneID) {
		return "", false
	}
	return zoneInfo.Zone, true
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named("zonalshift").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}
