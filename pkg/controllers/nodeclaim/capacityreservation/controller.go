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

package capacityreservation

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/operatorpkg/singleton"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	nodeclaimutils "sigs.k8s.io/karpenter/pkg/utils/nodeclaim"
)

type Controller struct {
	cp         cloudprovider.CloudProvider
	kubeClient client.Client
}

func NewController(kubeClient client.Client, cp cloudprovider.CloudProvider) *Controller {
	return &Controller{
		cp:         cp,
		kubeClient: kubeClient,
	}
}

func (*Controller) Name() string {
	return "nodeclaim.capacityreservation"
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named(c.Name()).
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}

func (c *Controller) Reconcile(ctx context.Context) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, c.Name())
	cpNodeClaims, err := c.cp.List(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("listing instance types, %w", err)
	}
	providerIDsToCPNodeClaims := lo.SliceToMap(cpNodeClaims, func(nc *karpv1.NodeClaim) (string, *karpv1.NodeClaim) {
		return nc.Status.ProviderID, nc
	})
	ncs := &karpv1.NodeClaimList{}
	if err := c.kubeClient.List(ctx, ncs); err != nil {
		return reconcile.Result{}, fmt.Errorf("listing nodeclaims, %w", err)
	}
	updatedNodeClaims := sets.New[string]()
	var errs []error
	for i := range ncs.Items {
		cpNC, ok := providerIDsToCPNodeClaims[ncs.Items[i].Status.ProviderID]
		if !ok {
			continue
		}
		updated, err := c.syncCapacityType(ctx, cpNC.Labels[karpv1.CapacityTypeLabelKey], &ncs.Items[i])
		if err != nil {
			errs = append(errs, err)
		}
		if updated {
			updatedNodeClaims.Insert(ncs.Items[i].Name)
		}
	}
	if len(updatedNodeClaims) != 0 {
		log.FromContext(ctx).WithValues("NodeClaims", lo.Map(updatedNodeClaims.UnsortedList(), func(name string, _ int) klog.ObjectRef {
			return klog.KRef("", name)
		})).V(1).Info("updated capacity type for nodeclaims")
	}
	if len(errs) != 0 {
		if lo.EveryBy(errs, func(err error) bool { return errors.IsConflict(err) }) {
			return reconcile.Result{Requeue: true}, nil
		}
		return reconcile.Result{}, multierr.Combine(errs...)
	}
	return reconcile.Result{RequeueAfter: time.Minute}, nil
}

// syncCapacityType will update the capacity type for the given NodeClaim. This accounts for the fact that capacity
// reservations will expire, demoting NodeClaims with capacity type "reserved" to "on-demand".
func (c *Controller) syncCapacityType(ctx context.Context, capacityType string, nc *karpv1.NodeClaim) (bool, error) {
	// We won't be able to sync deleting NodeClaims, and there's no real need to either as they're already draining.
	if !nc.DeletionTimestamp.IsZero() {
		return false, nil
	}

	// For now we only account for the case where a reserved NodeClaim becomes an on-demand NodeClaim. This does not
	// account for on-demand NodeClaims being promoted to reserved since that is not natively supported by Karpenter.
	if capacityType != karpv1.CapacityTypeOnDemand {
		return false, nil
	}
	if nc.Labels[karpv1.CapacityTypeLabelKey] == karpv1.CapacityTypeReserved {
		stored := nc.DeepCopy()
		nc.Labels[karpv1.CapacityTypeLabelKey] = karpv1.CapacityTypeOnDemand
		delete(nc.Labels, cloudprovider.ReservationIDLabel)
		if err := c.kubeClient.Patch(ctx, nc, client.MergeFrom(stored)); client.IgnoreNotFound(err) != nil {
			return false, fmt.Errorf("patching nodeclaim %q, %w", nc.Name, err)
		}
	}

	// If the reservation expired before the NodeClaim became registered, there may not be a Node on the cluster. Note
	// that there should never be duplicate Nodes for a given NodeClaim, but handling this user-induced error is more
	// straightforward than handling the duplicate error.
	nodes, err := nodeclaimutils.AllNodesForNodeClaim(ctx, c.kubeClient, nc)
	if err != nil {
		return false, fmt.Errorf("listing nodes for nodeclaim %q, %w", nc.Name, err)
	}
	updated := false
	for _, n := range nodes {
		if !n.DeletionTimestamp.IsZero() {
			continue
		}
		// Skip Nodes which haven't been registered since we still may not have synced labels. We'll get it on the next
		// iteration.
		if n.Labels[karpv1.NodeRegisteredLabelKey] != "true" {
			continue
		}
		if n.Labels[karpv1.CapacityTypeLabelKey] != karpv1.CapacityTypeReserved {
			continue
		}
		stored := n.DeepCopy()
		n.Labels[karpv1.CapacityTypeLabelKey] = karpv1.CapacityTypeOnDemand
		delete(n.Labels, cloudprovider.ReservationIDLabel)
		if err := c.kubeClient.Patch(ctx, n, client.MergeFrom(stored)); client.IgnoreNotFound(err) != nil {
			return false, fmt.Errorf("patching node %q, %w", n.Name, err)
		}
		updated = true
	}
	return updated, nil
}
