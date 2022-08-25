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

	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

const provisionerControllerName = "provisioner-state"

// ProvisionerController reconciles pods for the purpose of maintaining state regarding pods that is expensive to compute.
type ProvisionerController struct {
	kubeClient client.Client
	cluster    *Cluster
}

func NewProvisionerController(kubeClient client.Client, cluster *Cluster) *ProvisionerController {
	return &ProvisionerController{
		kubeClient: kubeClient,
		cluster:    cluster,
	}
}

func (c *ProvisionerController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(provisionerControllerName).With("provisioner", req.NamespacedName))
	stored := &v1alpha5.Provisioner{}

	// If the provisioner is deleted, no reason to re-consider consolidation at this point
	if err := c.kubeClient.Get(ctx, req.NamespacedName, stored); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	// Something changed in the provisioner so we should re-consider consolidation
	c.cluster.recordConsolidationChange()
	return reconcile.Result{}, nil
}

func (c *ProvisionerController) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(provisionerControllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		For(&v1alpha5.Provisioner{}).
		Complete(c)
}
