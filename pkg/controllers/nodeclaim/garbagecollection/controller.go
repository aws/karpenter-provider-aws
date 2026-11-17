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

	"github.com/awslabs/operatorpkg/reconciler"
	"github.com/awslabs/operatorpkg/singleton"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	nodeclaimutils "sigs.k8s.io/karpenter/pkg/utils/nodeclaim"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

// minInstanceAge is how long an instance must have existed before it can be garbage collected. It covers the window
// between launch and the NodeClaim status write that records the instance's providerID.
const minInstanceAge = 5 * time.Minute

type Controller struct {
	kubeClient client.Client
	// apiReader bypasses the informer cache, which can lag by tens of seconds during large scale-ups
	apiReader       client.Reader
	cloudProvider   cloudprovider.CloudProvider
	successfulCount uint64 // keeps track of successful reconciles for more aggressive requeueing near the start of the controller
}

func NewController(kubeClient client.Client, apiReader client.Reader, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		kubeClient:      kubeClient,
		apiReader:       apiReader,
		cloudProvider:   cloudProvider,
		successfulCount: 0,
	}
}

func (c *Controller) Reconcile(ctx context.Context) (reconciler.Result, error) {
	ctx = injection.WithControllerName(ctx, "instance.garbagecollection")

	// We LIST NodeClaims on the CloudProvider BEFORE we grab NodeClaims/Nodes on the cluster so that we make sure that, if
	// LISTing cloudNodeClaims takes a long time, our information is more updated by the time we get to Node and NodeClaim LIST
	// This works since our CloudProvider cloudNodeClaims are deleted based on whether the Machine exists or not, not vise-versa
	cloudNodeClaims, err := c.cloudProvider.List(ctx)
	if err != nil {
		return reconciler.Result{}, fmt.Errorf("listing cloudprovider nodeclaims, %w", err)
	}
	// Filter out any cloudprovider NodeClaim which is already terminating
	cloudNodeClaims = lo.Filter(cloudNodeClaims, func(nc *karpv1.NodeClaim, _ int) bool {
		return nc.DeletionTimestamp.IsZero()
	})
	clusterNodeClaims, err := nodeclaimutils.ListManaged(ctx, c.kubeClient, c.cloudProvider)
	if err != nil {
		return reconciler.Result{}, err
	}
	candidates := orphaned(cloudNodeClaims, clusterNodeClaims)
	if len(candidates) > 0 {
		// The cached list may predate the NodeClaim status writes for recently launched instances
		nodeClaimList := &karpv1.NodeClaimList{}
		if err = c.apiReader.List(ctx, nodeClaimList); err != nil {
			return reconciler.Result{}, fmt.Errorf("listing nodeclaims from the api server, %w", err)
		}
		candidates = orphaned(candidates, lo.ToSlicePtr(nodeClaimList.Items))
	}
	if len(candidates) > 0 {
		nodeList := &corev1.NodeList{}
		if err = c.kubeClient.List(ctx, nodeList); err != nil {
			return reconciler.Result{}, err
		}
		errs := make([]error, len(candidates))
		workqueue.ParallelizeUntil(ctx, 100, len(candidates), func(i int) {
			errs[i] = c.garbageCollect(ctx, candidates[i], nodeList)
		})
		if err = multierr.Combine(errs...); err != nil {
			return reconciler.Result{}, err
		}
	}
	c.successfulCount++
	return reconciler.Result{RequeueAfter: lo.Ternary(c.successfulCount <= 20, time.Second*10, time.Minute*2)}, nil
}

// orphaned returns the cloudprovider NodeClaims old enough to collect whose providerID no cluster NodeClaim claims
func orphaned(cloudNodeClaims, clusterNodeClaims []*karpv1.NodeClaim) []*karpv1.NodeClaim {
	clusterProviderIDs := sets.New(lo.FilterMap(clusterNodeClaims, func(nc *karpv1.NodeClaim, _ int) (string, bool) {
		return nc.Status.ProviderID, nc.Status.ProviderID != ""
	})...)
	return lo.Filter(cloudNodeClaims, func(nc *karpv1.NodeClaim, _ int) bool {
		return !clusterProviderIDs.Has(nc.Status.ProviderID) && time.Since(nc.CreationTimestamp.Time) > minInstanceAge
	})
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
		// The Node comes from the cache. The UID precondition stops us from deleting a newer Node that reused its name.
		if err := c.kubeClient.Delete(ctx, &node, client.Preconditions{UID: &node.UID}); err != nil {
			if apierrors.IsConflict(err) {
				return nil
			}
			return client.IgnoreNotFound(err)
		}
		log.FromContext(ctx).WithValues("Node", klog.KObj(&node)).V(1).Info("garbage collected node")
	}
	return nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("instance.garbagecollection").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}
