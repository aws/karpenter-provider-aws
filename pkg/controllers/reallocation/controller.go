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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha2"
	"github.com/awslabs/karpenter/pkg/cloudprovider"

	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller for the resource
type Controller struct {
	utilization   *Utilization
	cloudProvider cloudprovider.CloudProvider
}

// For returns the resource this controller is for.
func (c *Controller) For() client.Object {
	return &v1alpha2.Provisioner{}
}

func (c *Controller) Interval() time.Duration {
	return 5 * time.Second
}

func (c *Controller) Name() string {
	return "provisioner/reallocator"
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, coreV1Client corev1.CoreV1Interface, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		utilization:   &Utilization{kubeClient: kubeClient},
		cloudProvider: cloudProvider,
	}
}

// Reconcile executes a reallocation control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, object client.Object) (reconcile.Result, error) {
	provisioner := object.(*v1alpha2.Provisioner)
	// Skip reconciliation if utilization ttl is not defined.
	if provisioner.Spec.TTLSecondsAfterEmpty == nil {
		return reconcile.Result{}, nil
	}
	// 1. Set TTL on TTLable Nodes
	if err := c.utilization.markUnderutilized(ctx, provisioner); err != nil {
		return reconcile.Result{}, fmt.Errorf("adding ttl and underutilized label, %w", err)
	}

	// 2. Remove TTL from Utilized Nodes
	if err := c.utilization.clearUnderutilized(ctx, provisioner); err != nil {
		return reconcile.Result{}, fmt.Errorf("removing ttl from node, %w", err)
	}

	// 3. Delete any node past its TTL
	if err := c.utilization.terminateExpired(ctx, provisioner); err != nil {
		return reconcile.Result{}, fmt.Errorf("marking nodes terminable, %w", err)
	}
	return reconcile.Result{RequeueAfter: c.Interval()}, nil
}
