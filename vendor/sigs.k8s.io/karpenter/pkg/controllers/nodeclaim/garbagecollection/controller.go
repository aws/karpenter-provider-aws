/*
Copyright The Kubernetes Authors.

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

package garbagecollection

import (
	"context"
	"time"

	"github.com/awslabs/operatorpkg/singleton"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/metrics"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	nodeutils "sigs.k8s.io/karpenter/pkg/utils/node"
	nodeclaimutils "sigs.k8s.io/karpenter/pkg/utils/nodeclaim"
)

type Controller struct {
	clock         clock.Clock
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
}

func NewController(c clock.Clock, kubeClient client.Client, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		clock:         c,
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
	}
}

func (c *Controller) Reconcile(ctx context.Context) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "nodeclaim.garbagecollection")

	nodeClaims, err := nodeclaimutils.ListManaged(ctx, c.kubeClient, c.cloudProvider)
	if err != nil {
		return reconcile.Result{}, err
	}
	cloudProviderNodeClaims, err := c.cloudProvider.List(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}
	cloudProviderNodeClaims = lo.Filter(cloudProviderNodeClaims, func(nc *v1.NodeClaim, _ int) bool {
		return nc.DeletionTimestamp.IsZero()
	})
	cloudProviderProviderIDs := sets.New[string](lo.Map(cloudProviderNodeClaims, func(nc *v1.NodeClaim, _ int) string {
		return nc.Status.ProviderID
	})...)
	// Only consider NodeClaims that are Registered since we don't want to fully rely on the CloudProvider
	// API to trigger deletion of the Node. Instead, we'll wait for our registration timeout to trigger
	nodeClaims = lo.Filter(nodeClaims, func(n *v1.NodeClaim, _ int) bool {
		return n.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue() &&
			n.DeletionTimestamp.IsZero() &&
			!cloudProviderProviderIDs.Has(n.Status.ProviderID)
	})

	errs := make([]error, len(nodeClaims))
	workqueue.ParallelizeUntil(ctx, 20, len(nodeClaims), func(i int) {
		node, err := nodeclaimutils.NodeForNodeClaim(ctx, c.kubeClient, nodeClaims[i])
		// Ignore these errors since a registered NodeClaim should only have a NotFound node when
		// the Node was deleted out from under us and a Duplicate Node is an invalid state
		if nodeclaimutils.IgnoreDuplicateNodeError(nodeclaimutils.IgnoreNodeNotFoundError(err)) != nil {
			errs[i] = err
		}
		// We do a check on the Ready condition of the node since, even though the CloudProvider says the instance
		// is not around, we know that the kubelet process is still running if the Node Ready condition is true
		// Similar logic to: https://github.com/kubernetes/kubernetes/blob/3a75a8c8d9e6a1ebd98d8572132e675d4980f184/staging/src/k8s.io/cloud-provider/controllers/nodelifecycle/node_lifecycle_controller.go#L144
		if node != nil && nodeutils.GetCondition(node, corev1.NodeReady).Status == corev1.ConditionTrue {
			return
		}
		if err := c.kubeClient.Delete(ctx, nodeClaims[i]); err != nil {
			errs[i] = client.IgnoreNotFound(err)
			return
		}
		log.FromContext(ctx).WithValues(
			"NodeClaim", klog.KRef("", nodeClaims[i].Name),
			"provider-id", nodeClaims[i].Status.ProviderID,
			"nodepool", nodeClaims[i].Labels[v1.NodePoolLabelKey],
		).V(1).Info("garbage collecting nodeclaim with no cloudprovider representation")
		metrics.NodeClaimsDisruptedTotal.Inc(map[string]string{
			metrics.ReasonLabel:       "garbage_collected",
			metrics.NodePoolLabel:     nodeClaims[i].Labels[v1.NodePoolLabelKey],
			metrics.CapacityTypeLabel: nodeClaims[i].Labels[v1.CapacityTypeLabelKey],
		})
	})
	if err = multierr.Combine(errs...); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{RequeueAfter: time.Minute * 2}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("nodeclaim.garbagecollection").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}
