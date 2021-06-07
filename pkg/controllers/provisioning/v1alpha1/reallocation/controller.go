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

package reallocation

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"

	v1 "k8s.io/api/core/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Controller for the resource
type Controller struct {
	utilization   *Utilization
	cloudProvider cloudprovider.Factory
}

// For returns the resource this controller is for.
func (c *Controller) For() client.Object {
	return &v1alpha1.Provisioner{}
}

// Owns returns the resources owned by this controller's resource.
func (c *Controller) Owns() []client.Object {
	return nil
}

func (c *Controller) Interval() time.Duration {
	return 5 * time.Second
}

func (c *Controller) Name() string {
	return "provisioner/reallocator"
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, coreV1Client corev1.CoreV1Interface, cloudProvider cloudprovider.Factory) *Controller {
	return &Controller{
		utilization:   &Utilization{kubeClient: kubeClient},
		cloudProvider: cloudProvider,
	}
}

// Reconcile executes a reallocation control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, object client.Object) (reconcile.Result, error) {
	provisioner := object.(*v1alpha1.Provisioner)

	// 1. Set TTL on TTLable Nodes
	if err := c.utilization.markUnderutilized(ctx, provisioner); err != nil {
		return reconcile.Result{}, fmt.Errorf("adding ttl and underutilized label, %w", err)
	}

	// 2. Remove TTL from Utilized Nodes
	if err := c.utilization.clearUnderutilized(ctx, provisioner); err != nil {
		return reconcile.Result{}, fmt.Errorf("removing ttl from node, %w", err)
	}

	// 3. Mark any Node past TTL as expired
	if err := c.utilization.markTerminable(ctx, provisioner); err != nil {
		return reconcile.Result{}, fmt.Errorf("marking nodes terminable, %w", err)
	}
	return reconcile.Result{}, nil
}

func (c *Controller) Watches(context.Context) (source.Source, handler.EventHandler, builder.WatchesOption) {
	return &source.Kind{Type: &v1.Pod{}},
		&handler.EnqueueRequestForObject{},
		builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool { return false }))
}
