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

	"github.com/awslabs/karpenter/pkg/cloudprovider"

	v1 "k8s.io/api/core/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller for the resource
type Controller struct {
	terminator    *Terminator
	cloudProvider cloudprovider.Factory
}

// For returns the resource this controller is for.
func (c *Controller) For() client.Object {
	return &v1.Node{}
}

// Owns returns the resources owned by this controller's resource.
func (c *Controller) Owns() []client.Object {
	return []client.Object{}
}

func (c *Controller) Interval() time.Duration {
	return 5 * time.Second
}

func (c *Controller) Name() string {
	return "terminator"
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, coreV1Client corev1.CoreV1Interface, cloudProvider cloudprovider.Factory) *Controller {
	return &Controller{
		terminator:    &Terminator{kubeClient: kubeClient, cloudprovider: cloudProvider, coreV1Client: coreV1Client},
		cloudProvider: cloudProvider,
	}
}

// Reconcile executes a reallocation control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, object client.Object) (reconcile.Result, error) {
	// 1. Cordon terminable nodes
	if err := c.terminator.cordonNodes(ctx); err != nil {
		return reconcile.Result{}, fmt.Errorf("cordoning terminable nodes, %w", err)
	}

	// 2. Drain and delete nodes
	if err := c.terminator.terminateNodes(ctx); err != nil {
		return reconcile.Result{}, fmt.Errorf("terminating nodes, %w", err)
	}
	return reconcile.Result{}, nil
}
