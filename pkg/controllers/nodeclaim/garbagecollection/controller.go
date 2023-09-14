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

	"github.com/samber/lo"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/apis/v1beta1"
	corecloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/operator/controller"
	nodeclaimutil "github.com/aws/karpenter-core/pkg/utils/nodeclaim"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/controllers/nodeclaim/link"
)

type Controller struct {
	kubeClient      client.Client
	cloudProvider   *cloudprovider.CloudProvider
	successfulCount uint64           // keeps track of successful reconciles for more aggressive requeueing near the start of the controller
	linkController  *link.Controller // get machines recently linked by this controller

}

func NewController(kubeClient client.Client, cloudProvider *cloudprovider.CloudProvider, linkController *link.Controller) *Controller {
	return &Controller{
		kubeClient:      kubeClient,
		cloudProvider:   cloudProvider,
		successfulCount: 0,
		linkController:  linkController,
	}
}

func (c *Controller) Name() string {
	return "machine.garbagecollection"
}

func (c *Controller) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	// We LIST machines on the CloudProvider BEFORE we grab Machines/Nodes on the cluster so that we make sure that, if
	// LISTing instances takes a long time, our information is more updated by the time we get to Machine and Node LIST
	// This works since our CloudProvider instances are deleted based on whether the Machine exists or not, not vise-versa
	retrieved, err := c.cloudProvider.List(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("listing cloudprovider machines, %w", err)
	}
	managedRetrieved := lo.Filter(retrieved, func(nc *v1beta1.NodeClaim, _ int) bool {
		return nc.Annotations[v1beta1.ManagedByAnnotationKey] != "" && nc.DeletionTimestamp.IsZero()
	})
	nodeClaimList, err := nodeclaimutil.List(ctx, c.kubeClient)
	if err != nil {
		return reconcile.Result{}, err
	}
	nodeList := &v1.NodeList{}
	if err := c.kubeClient.List(ctx, nodeList); err != nil {
		return reconcile.Result{}, err
	}
	resolvedNodeClaims := lo.Filter(nodeClaimList.Items, func(n v1beta1.NodeClaim, _ int) bool {
		return n.Status.ProviderID != "" || n.Annotations[v1alpha5.MachineLinkedAnnotationKey] != ""
	})
	resolvedProviderIDs := sets.New[string](lo.Map(resolvedNodeClaims, func(n v1beta1.NodeClaim, _ int) string {
		if n.Status.ProviderID != "" {
			return n.Status.ProviderID
		}
		return n.Annotations[v1alpha5.MachineLinkedAnnotationKey]
	})...)
	errs := make([]error, len(retrieved))
	workqueue.ParallelizeUntil(ctx, 100, len(managedRetrieved), func(i int) {
		_, recentlyLinked := c.linkController.Cache.Get(managedRetrieved[i].Status.ProviderID)

		if !recentlyLinked &&
			!resolvedProviderIDs.Has(managedRetrieved[i].Status.ProviderID) &&
			time.Since(managedRetrieved[i].CreationTimestamp.Time) > time.Second*30 {
			errs[i] = c.garbageCollect(ctx, managedRetrieved[i], nodeList)
		}
	})
	c.successfulCount++
	return reconcile.Result{RequeueAfter: lo.Ternary(c.successfulCount <= 20, time.Second*10, time.Minute*2)}, multierr.Combine(errs...)
}

func (c *Controller) garbageCollect(ctx context.Context, nodeClaim *v1beta1.NodeClaim, nodeList *v1.NodeList) error {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("provider-id", nodeClaim.Status.ProviderID))
	if err := c.cloudProvider.Delete(ctx, nodeClaim); err != nil {
		return corecloudprovider.IgnoreNodeClaimNotFoundError(err)
	}
	logging.FromContext(ctx).Debugf("garbage collected cloudprovider instance")

	// Go ahead and cleanup the node if we know that it exists to make scheduling go quicker
	if node, ok := lo.Find(nodeList.Items, func(n v1.Node) bool {
		return n.Spec.ProviderID == nodeClaim.Status.ProviderID
	}); ok {
		if err := c.kubeClient.Delete(ctx, &node); err != nil {
			return client.IgnoreNotFound(err)
		}
		logging.FromContext(ctx).With("node", node.Name).Debugf("garbage collected node")
	}
	return nil
}

func (c *Controller) Builder(_ context.Context, m manager.Manager) controller.Builder {
	return controller.NewSingletonManagedBy(m)
}
