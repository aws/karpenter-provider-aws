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

package garbagecollection

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/operatorpkg/singleton"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	nodeclaimutils "sigs.k8s.io/karpenter/pkg/utils/nodeclaim"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

type Controller struct {
	kubeClient      client.Client
	cloudProvider   cloudprovider.CloudProvider
	successfulCount uint64 // keeps track of successful reconciles for more aggressive requeueing near the start of the controller
}

func NewController(kubeClient client.Client, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		kubeClient:      kubeClient,
		cloudProvider:   cloudProvider,
		successfulCount: 0,
	}
}

func (c *Controller) Reconcile(ctx context.Context) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "instance.garbagecollection")

	// We LIST NodeClaims on the CloudProvider BEFORE we grab NodeClaims/Nodes on the cluster so that we make sure that, if
	// LISTing cloudNodeClaims takes a long time, our information is more updated by the time we get to Node and NodeClaim LIST
	// This works since our CloudProvider cloudNodeClaims are deleted based on whether the Machine exists or not, not vise-versa
	cloudNodeClaims, err := c.cloudProvider.List(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("listing cloudprovider nodeclaims, %w", err)
	}
	// Filter out any cloudprovider NodeClaim which is already terminating
	cloudNodeClaims = lo.Filter(cloudNodeClaims, func(nc *karpv1.NodeClaim, _ int) bool {
		return nc.DeletionTimestamp.IsZero()
	})
	clusterNodeClaims, err := nodeclaimutils.List(ctx, c.kubeClient, nodeclaimutils.WithManagedFilter(c.cloudProvider))
	if err != nil {
		return reconcile.Result{}, err
	}
	clusterProviderIDs := sets.New(lo.FilterMap(clusterNodeClaims, func(nc *karpv1.NodeClaim, _ int) (string, bool) {
		return nc.Status.ProviderID, nc.Status.ProviderID != ""
	})...)
	nodeList := &corev1.NodeList{}
	if err = c.kubeClient.List(ctx, nodeList); err != nil {
		return reconcile.Result{}, err
	}
	errs := make([]error, len(cloudNodeClaims))
	workqueue.ParallelizeUntil(ctx, 100, len(cloudNodeClaims), func(i int) {
		if nc := cloudNodeClaims[i]; !clusterProviderIDs.Has(nc.Status.ProviderID) && time.Since(nc.CreationTimestamp.Time) > time.Second*30 {
			errs[i] = c.garbageCollect(ctx, cloudNodeClaims[i], nodeList)
		}
	})
	if err = multierr.Combine(errs...); err != nil {
		return reconcile.Result{}, err
	}
	c.successfulCount++
	return reconcile.Result{RequeueAfter: lo.Ternary(c.successfulCount <= 20, time.Second*10, time.Minute*2)}, nil
}

func (c *Controller) garbageCollect(ctx context.Context, nodeClaim *karpv1.NodeClaim, nodeList *corev1.NodeList) error {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("provider-id", nodeClaim.Status.ProviderID))
	if err := c.cloudProvider.Delete(ctx, nodeClaim); err != nil {
		return cloudprovider.IgnoreNodeClaimNotFoundError(err)
	}
	log.FromContext(ctx).V(1).Info("garbage collected cloudprovider instance")

	// Go ahead and cleanup the node if we know that it exists to make scheduling go quicker
	if node, ok := lo.Find(nodeList.Items, func(n corev1.Node) bool {
		return n.Spec.ProviderID == nodeClaim.Status.ProviderID
	}); ok {
		if err := c.kubeClient.Delete(ctx, &node); err != nil {
			return client.IgnoreNotFound(err)
		}
		log.FromContext(ctx).WithValues("Node", klog.KRef("", node.Name)).V(1).Info("garbage collected node")
	}
	return nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("instance.garbagecollection").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}
