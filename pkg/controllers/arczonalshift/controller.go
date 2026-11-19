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
	"errors"
	"fmt"
	"time"

	"github.com/awslabs/operatorpkg/reconciler"
	"github.com/awslabs/operatorpkg/singleton"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	cloudproviderevents "github.com/aws/karpenter-provider-aws/pkg/cloudprovider/events"
	"github.com/aws/karpenter-provider-aws/pkg/providers/arczonalshift"
)

const (
	doNotRepairValue = "true"
	// PreShiftUnset is the sentinel stored in PreShiftDoNotRepairAnnotation when the
	// node had no do-not-repair annotation before the controller took it over, so
	// weigh-back knows to remove the veto rather than restore a value.
	PreShiftUnset = "<unset>"
)

// PreShiftDoNotRepairAnnotation is a controller-managed marker recording the node's
// do-not-repair value from before this controller applied the zonal-shift veto (or
// PreShiftUnset if it had none). Its presence is the controller's ownership signal:
// on weigh-back the controller restores this value (or removes the veto) and deletes
// the marker. It is managed solely by this controller — editing or removing it
// independently is unsupported and can strand the veto.
var PreShiftDoNotRepairAnnotation = apis.Group + "/pre-shift-do-not-repair"

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

	// Apply the do-not-repair veto first. It is the primary protective action and does
	// not depend on NodeClass/event resolution, so it must not be gated behind the
	// informational event-publish block below — that block returns early when a
	// NodePool references a missing NodeClass, and since previousShiftedZones is not
	// advanced on that error the failure would otherwise persist and suppress the veto
	// for the whole shift. Runs every reconcile so newly-registered nodes in a shifted
	// zone are covered.
	if err := c.reconcileDoNotRepair(ctx, currentShiftedZones); err != nil {
		return reconciler.Result{}, fmt.Errorf("reconciling do-not-repair veto: %w", err)
	}

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

// reconcileDoNotRepair converges the do-not-repair veto on Karpenter-managed nodes:
// nodes in a shifted zone get the veto (recording the pre-shift value first), and
// nodes no longer in a shifted zone have a controller-applied veto reverted. A node the
// controller never took over is left untouched; but while a node is under a shift the
// controller owns its do-not-repair annotation — the value restored on weigh-back is the
// one captured at first takeover, so an edit made to it mid-shift is not preserved.
// Shifts are keyed by zone ID, matched against the node's topology.k8s.aws/zone-id label.
//
// Repair is a voluntary-disruption method that only acts on registered nodes and reads
// the veto from the Node's annotations, so stamping Nodes covers it; instances that
// never register are reaped by NodeClaim registration liveness, which this veto does
// not gate. Nodes are re-swept every reconcile so a node registering into a shifted
// zone is covered well within repair's multi-minute condition tolerations.
func (c *Controller) reconcileDoNotRepair(ctx context.Context, shiftedZones sets.Set[string]) error {
	nodes := &corev1.NodeList{}
	if err := c.kubeClient.List(ctx, nodes, client.HasLabels{karpv1.NodePoolLabelKey}); err != nil {
		return fmt.Errorf("listing nodes: %w", err)
	}

	var errs []error
	for i := range nodes.Items {
		node := &nodes.Items[i]

		// Guard against an empty zone-id: node.Labels[...] is "" when the label is
		// absent, so without the zoneID != "" check a stray empty entry in the shifted
		// set would match every unlabeled node and veto repair on all of them.
		zoneID := node.Labels[v1.LabelTopologyZoneID]
		shifted := zoneID != "" && shiftedZones.Has(zoneID)
		_, owned := node.Annotations[PreShiftDoNotRepairAnnotation]
		// Nothing to do for a node that is neither in a shifted zone nor already
		// carrying the controller's marker (the overwhelmingly common case) — skip it
		// with cheap map lookups before the per-node DeepCopy.
		if !shifted && !owned {
			continue
		}

		stored := node.DeepCopy()
		if shifted {
			setDoNotRepairVeto(node)
		} else {
			restoreDoNotRepairVeto(node)
		}

		if equality.Semantic.DeepEqual(stored.Annotations, node.Annotations) {
			continue
		}
		if err := c.kubeClient.Patch(ctx, node, client.MergeFrom(stored)); err != nil {
			errs = append(errs, fmt.Errorf("patching node %q: %w", node.Name, err))
		}
	}
	return errors.Join(errs...)
}

// setDoNotRepairVeto forces do-not-repair on a node in a shifted zone, recording its
// pre-shift do-not-repair value once (on first takeover) so restoreDoNotRepairVeto can
// put it back on weigh-back. The prior value is captured only when the marker is absent,
// so repeated reconciles never overwrite it with the controller's own value.
func setDoNotRepairVeto(node *corev1.Node) {
	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}
	if _, owned := node.Annotations[PreShiftDoNotRepairAnnotation]; !owned {
		prior, ok := node.Annotations[karpv1.DoNotRepairAnnotationKey]
		node.Annotations[PreShiftDoNotRepairAnnotation] = lo.Ternary(ok, prior, PreShiftUnset)
	}
	node.Annotations[karpv1.DoNotRepairAnnotationKey] = doNotRepairValue
}

// restoreDoNotRepairVeto reverts a controller-applied veto to the node's pre-shift
// state on weigh-back. It is a no-op on a node the controller never took over (marker
// absent). For a node it does own, it restores the value captured at first takeover, so
// a change made to do-not-repair while the shift was active is not preserved.
func restoreDoNotRepairVeto(node *corev1.Node) {
	prior, owned := node.Annotations[PreShiftDoNotRepairAnnotation]
	if !owned {
		return
	}
	if prior == PreShiftUnset {
		delete(node.Annotations, karpv1.DoNotRepairAnnotationKey)
	} else {
		node.Annotations[karpv1.DoNotRepairAnnotationKey] = prior
	}
	delete(node.Annotations, PreShiftDoNotRepairAnnotation)
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
