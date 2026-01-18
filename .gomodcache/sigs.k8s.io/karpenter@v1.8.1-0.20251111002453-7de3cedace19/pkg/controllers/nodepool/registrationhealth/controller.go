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

package registrationhealth

import (
	"context"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/karpenter/pkg/state/nodepoolhealth"

	"sigs.k8s.io/karpenter/pkg/operator/injection"

	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	utilscontroller "sigs.k8s.io/karpenter/pkg/utils/controller"
	nodepoolutils "sigs.k8s.io/karpenter/pkg/utils/nodepool"
)

// Controller for the resource
type Controller struct {
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
	npState       *nodepoolhealth.State
}

// NewController will create a controller to reset NodePool's registration health when there is an update to NodePool/NodeClass spec
func NewController(kubeClient client.Client, cloudProvider cloudprovider.CloudProvider, npState *nodepoolhealth.State) *Controller {
	return &Controller{
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
		npState:       npState,
	}
}

//nolint:gocyclo
func (c *Controller) Reconcile(ctx context.Context, nodePool *v1.NodePool) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "nodepool.registrationhealth")

	nodeClass, err := nodepoolutils.GetNodeClass(ctx, c.kubeClient, nodePool, c.cloudProvider)
	if err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	// Ignore NodePools which aren't using a supported NodeClass
	if nodeClass == nil {
		return reconcile.Result{}, nil
	}
	stored := nodePool.DeepCopy()

	// If Karpenter restarts i.e. if the buffer for the nodePool is empty and the NodeRegistrationHealthy status condition
	// is set to either true/false then we pre-hydrate the buffer with the existing state of the status condition
	if c.npState.Status(nodePool.UID) == nodepoolhealth.StatusUnknown {
		if nodePool.StatusConditions().Get(v1.ConditionTypeNodeRegistrationHealthy).IsTrue() {
			c.npState.SetStatus(nodePool.UID, nodepoolhealth.StatusHealthy)
		}
		if nodePool.StatusConditions().Get(v1.ConditionTypeNodeRegistrationHealthy).IsFalse() {
			c.npState.SetStatus(nodePool.UID, nodepoolhealth.StatusUnhealthy)
		}
	}

	// If NodeClass/NodePool have been updated then NodeRegistrationHealthy = Unknown and reset the buffer
	if nodePool.StatusConditions().Get(v1.ConditionTypeNodeRegistrationHealthy) == nil ||
		nodePool.Status.NodeClassObservedGeneration != nodeClass.GetGeneration() ||
		nodePool.Generation != nodePool.StatusConditions().Get(v1.ConditionTypeNodeRegistrationHealthy).ObservedGeneration {
		nodePool.StatusConditions().SetUnknown(v1.ConditionTypeNodeRegistrationHealthy)
		c.npState.SetStatus(nodePool.UID, nodepoolhealth.StatusUnknown)
	}

	nodePool.Status.NodeClassObservedGeneration = nodeClass.GetGeneration()
	if !equality.Semantic.DeepEqual(stored, nodePool) {
		// We use client.MergeFromWithOptimisticLock because patching a list with a JSON merge patch
		// can cause races due to the fact that it fully replaces the list on a change
		// Here, we are updating the status condition list
		if err := c.kubeClient.Status().Patch(ctx, nodePool, client.MergeFromWithOptions(stored, client.MergeFromWithOptimisticLock{})); client.IgnoreNotFound(err) != nil {
			if errors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	b := controllerruntime.NewControllerManagedBy(m).
		Named("nodepool.registrationhealth").
		For(&v1.NodePool{}, builder.WithPredicates(nodepoolutils.IsManagedPredicateFuncs(c.cloudProvider))).
		WithOptions(controller.Options{MaxConcurrentReconciles: utilscontroller.LinearScaleReconciles(utilscontroller.CPUCount(ctx), 10, 1000)})
	for _, nodeClass := range c.cloudProvider.GetSupportedNodeClasses() {
		b.Watches(nodeClass, nodepoolutils.NodeClassEventHandler(c.kubeClient))
	}
	return b.Complete(reconcile.AsReconciler(m.GetClient(), c))
}
