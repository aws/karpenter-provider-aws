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

package counter

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	nodepoolutils "sigs.k8s.io/karpenter/pkg/utils/nodepool"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

// Controller for the resource
type Controller struct {
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
	cluster       *state.Cluster
}

var ResourceNode = corev1.ResourceName("nodes")

var BaseResources = corev1.ResourceList{
	corev1.ResourceCPU:              resource.MustParse("0"),
	corev1.ResourceMemory:           resource.MustParse("0"),
	corev1.ResourcePods:             resource.MustParse("0"),
	corev1.ResourceEphemeralStorage: resource.MustParse("0"),
	ResourceNode:                    resource.MustParse("0"),
}

// NewController is a constructor
func NewController(kubeClient client.Client, cloudProvider cloudprovider.CloudProvider, cluster *state.Cluster) *Controller {
	return &Controller{
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
		cluster:       cluster,
	}
}

// Reconcile a control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, nodePool *v1.NodePool) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "nodepool.counter")
	if !nodepoolutils.IsManaged(nodePool, c.cloudProvider) {
		return reconcile.Result{}, nil
	}

	// We need to ensure that our internal cluster state mechanism is synced before we proceed
	// Otherwise, we have the potential to patch over the status with a lower value for the nodepool resource
	// counts on startup
	if !c.cluster.Synced(ctx) {
		return reconcile.Result{RequeueAfter: time.Second}, nil
	}
	stored := nodePool.DeepCopy()
	// Determine resource usage and update nodepool.status.resources
	nodePool.Status.Resources = c.resourceCountsFor(v1.NodePoolLabelKey, nodePool.Name)
	if !equality.Semantic.DeepEqual(stored, nodePool) {
		if err := c.kubeClient.Status().Patch(ctx, nodePool, client.MergeFrom(stored)); err != nil {
			return reconcile.Result{}, client.IgnoreNotFound(err)
		}
	}
	return reconcile.Result{}, nil
}

func (c *Controller) resourceCountsFor(ownerLabel string, ownerName string) corev1.ResourceList {
	res := BaseResources.DeepCopy()
	nodeCount := 0
	// Record all resources provisioned by the nodepools, we look at the cluster state nodes as their capacity
	// is accurately reported even for nodes that haven't fully started yet. This allows us to update our nodepool
	// status immediately upon node creation instead of waiting for the node to become ready.
	c.cluster.ForEachNode(func(n *state.StateNode) bool {
		// Don't count nodes that we are planning to delete. This is to ensure that we are consistent throughout
		// our provisioning and deprovisioning loops
		if n.MarkedForDeletion() {
			return true
		}
		if n.Labels()[ownerLabel] == ownerName {
			res = resources.MergeInto(res, n.Capacity())
			nodeCount += 1
		}
		return true
	})
	res[ResourceNode] = resource.MustParse(fmt.Sprintf("%d", nodeCount))
	return res
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("nodepool.counter").
		For(&v1.NodePool{}, builder.WithPredicates(nodepoolutils.IsManagedPredicateFuncs(c.cloudProvider))).
		Watches(&v1.NodeClaim{}, nodepoolutils.NodeClaimEventHandler(), builder.WithPredicates(predicate.NewPredicateFuncs(func(o client.Object) bool {
			// Add a predicate here to filter out NodeClaims triggering reconciliation if they haven't resolved their
			// providerID (and therefore, their resources)
			nc := o.(*v1.NodeClaim)
			return nc.Status.ProviderID != ""
		}))).
		Watches(&corev1.Node{}, nodepoolutils.NodeEventHandler()).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(reconcile.AsReconciler(m.GetClient(), c))

}
