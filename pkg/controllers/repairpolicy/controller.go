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

package repairpolicy

import (
	"context"
	"os"

	"github.com/awslabs/operatorpkg/reasonable"
	corev1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/karpenter/pkg/operator/injection"

	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
)

// Controller watches the node repair policy ConfigMap and pushes parsed policies
// into the CloudProvider so RepairPolicies() reflects changes without a restart.
type Controller struct {
	kubeClient    client.Client
	cloudProvider *cloudprovider.CloudProvider
	configMapName string
}

func NewController(kubeClient client.Client, cp *cloudprovider.CloudProvider, configMapName string) *Controller {
	return &Controller{
		kubeClient:    kubeClient,
		cloudProvider: cp,
		configMapName: configMapName,
	}
}

func (c *Controller) Name() string { return "repairpolicy" }

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named(c.Name()).
		For(&corev1.ConfigMap{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(o client.Object) bool {
			return o.GetNamespace() == os.Getenv("SYSTEM_NAMESPACE") &&
				o.GetName() == c.configMapName
		}))).
		WithOptions(controller.Options{
			RateLimiter:             reasonable.RateLimiter(),
			MaxConcurrentReconciles: 1,
		}).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}

func (c *Controller) Reconcile(ctx context.Context, cm *corev1.ConfigMap) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, c.Name())
	policies, err := cloudprovider.ParseRepairPolicies(cm)
	if err != nil {
		log.FromContext(ctx).Error(err, "invalid repair policy configmap, retaining previous policies")
		return reconcile.Result{}, nil
	}
	c.cloudProvider.SetRepairPolicies(policies)
	log.FromContext(ctx).Info("reloaded repair policies", "count", len(policies))
	return reconcile.Result{}, nil
}
