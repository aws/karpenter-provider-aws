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

package state

import (
	"context"
	"sync"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ProvisionerController struct {
	kubeClient client.Client
	labelMap   sync.Map
}

func NewProvisionerController(kubeClient client.Client) *ProvisionerController {
	return &ProvisionerController{
		kubeClient: kubeClient,
	}
}

// Reconcile executes a termination control loop for the resource
func (c *ProvisionerController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("provisionermetrics").With("provisioner", req.Name))

	// Remove the previous gauge after provisioner labels are updated
	c.cleanup(req.NamespacedName)

	// Retrieve provisioner from reconcile request
	provisioner := &v1alpha5.Provisioner{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, provisioner); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	if err := c.record(ctx, provisioner); err != nil {
		logging.FromContext(ctx).Errorf("Failed to update gauges: %s", err)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (c *ProvisionerController) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named("provisionermetrics").
		For(&v1alpha5.Provisioner{}).
		Complete(c)
}
