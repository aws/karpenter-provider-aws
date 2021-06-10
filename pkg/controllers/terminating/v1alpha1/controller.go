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

package v1alpha1

import (
	"context"
	"fmt"
	"time"

	provisioning "github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/functional"

	v1 "k8s.io/api/core/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Controller for the resource
type Controller struct {
	terminator    *Terminator
	cloudProvider cloudprovider.CloudProvider
}

// For returns the resource this controller is for.
func (c *Controller) For() client.Object {
	return &v1.Node{}
}

// Owns returns the resources owned by this controller's resource.
func (c *Controller) Owns() []client.Object {
	return nil
}

func (c *Controller) Interval() time.Duration {
	return 5 * time.Second
}

func (c *Controller) Name() string {
	return "terminator"
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, coreV1Client corev1.CoreV1Interface, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		terminator:    &Terminator{kubeClient: kubeClient, cloudProvider: cloudProvider, coreV1Client: coreV1Client},
		cloudProvider: cloudProvider,
	}
}

// Reconcile executes a termination control loop for the resource
<<<<<<< HEAD
func (c *Controller) Reconcile(ctx context.Context, object client.Object) (reconcile.Result, error) {
	node := object.(*v1.Node)
	// 1. Check if node is terminable
	if node.DeletionTimestamp == nil || !functional.ContainsString(node.Finalizers, provisioning.KarpenterFinalizer) {
		return reconcile.Result{}, nil
	}
	// 2. Cordon node
	if err := c.terminator.cordonNode(ctx, node); err != nil {
		return reconcile.Result{}, fmt.Errorf("cordoning node %s, %w", node.Name, err)
	}
	// 3. Drain node
	drained, err := c.terminator.drainNode(ctx, node)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("draining node %s, %w", node.Name, err)
	}
	// 4. If fully drained, terminate the node
	if drained {
		if err := c.terminator.terminateNode(ctx, node); err != nil {
			return reconcile.Result{}, fmt.Errorf("terminating nodes, %w", err)
		}
	}
	return reconcile.Result{Requeue: !drained}, nil
}

func (c *Controller) Watches() (source.Source, handler.EventHandler, builder.WatchesOption) {
	return &source.Kind{Type: &v1.Node{}},
		&handler.EnqueueRequestForObject{},
		builder.WithPredicates(
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool { return false }, //no-op
				DeleteFunc: func(e event.DeleteEvent) bool { return false }, //no-op
				UpdateFunc: func(e event.UpdateEvent) bool {
					node := e.ObjectNew.(*v1.Node)
					labels := node.GetLabels()
					// Don't watch nodes not created by provisioners
					if _, ok := labels[provisioning.ProvisionerNameLabelKey]; !ok {
						return false
					}
					if _, ok := labels[provisioning.ProvisionerNamespaceLabelKey]; !ok {
						return false
					}
					if phase, ok := labels[provisioning.ProvisionerPhaseLabel]; ok {
						return phase != provisioning.ProvisionerUnderutilizedPhase &&
							e.ObjectOld.GetResourceVersion() != e.ObjectNew.GetResourceVersion()
					}
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool { return false }, //no-op
			},
		)
}
