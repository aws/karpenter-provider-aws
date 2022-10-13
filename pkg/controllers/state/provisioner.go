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

	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
)

const provisionerControllerName = "provisioner-state"

// ProvisionerController reconciles provisioners to re-trigger consolidation on change.
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
		WithEventFilter(predicate.Funcs{DeleteFunc: func(event event.DeleteEvent) bool { return false }}).
		For(&v1alpha5.Provisioner{}).
		Complete(c)
}
