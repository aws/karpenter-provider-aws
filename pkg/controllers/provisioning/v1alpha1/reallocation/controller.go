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
	"github.com/awslabs/karpenter/pkg/controllers"

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
	terminator    *Terminator
	utilization   *Utilization
	cloudProvider cloudprovider.Factory
}

// For returns the resource this controller is for.
func (c *Controller) For() controllers.Object {
	return &v1alpha1.Provisioner{}
}

// Owns returns the resources owned by this controller's resource.
func (c *Controller) Owns() []controllers.Object {
	return []controllers.Object{}
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
		terminator:    &Terminator{kubeClient: kubeClient, cloudprovider: cloudProvider, coreV1Client: coreV1Client},
		cloudProvider: cloudProvider,
	}
}

// Reconcile executes a reallocation control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, object controllers.Object) (reconcile.Result, error) {
	provisioner := object.(*v1alpha1.Provisioner)
	if err := c.utilization.Reconcile(ctx, provisioner); err != nil {
		return reconcile.Result{}, fmt.Errorf("reconciling utilization sub-controller, %w", err)
	}
	if err := c.terminator.Reconcile(ctx, provisioner); err != nil {
		return reconcile.Result{}, fmt.Errorf("reconciling termination sub-controller, %w", err)
	}
	return reconcile.Result{}, nil
}

func (c *Controller) Watches(context.Context) (source.Source, handler.EventHandler, builder.WatchesOption) {
	return &source.Kind{Type: &v1.Pod{}},
		&handler.EnqueueRequestForObject{},
		builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool { return false }))
}
