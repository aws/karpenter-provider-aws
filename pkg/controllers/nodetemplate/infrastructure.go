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

package nodetemplate

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	awssettings "github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/controllers/providers"
)

type Infrastructure struct {
	kubeClient client.Client
	provider   *providers.Infrastructure

	lastInfrastructureReconcile time.Time
}

// Reconcile reconciles the infrastructure based on whether interruption handling is enabled and deletes
// the infrastructure by ref-counting when the last AWSNodeTemplate is removed
func (i *Infrastructure) Reconcile(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) (reconcile.Result, error) {
	if awssettings.FromContext(ctx).EnableInterruptionHandling {
		list := &v1alpha1.AWSNodeTemplateList{}
		if err := i.kubeClient.List(ctx, list); err != nil {
			return reconcile.Result{}, err
		}
		if !nodeTemplate.DeletionTimestamp.IsZero() && len(list.Items) == 1 {
			if err := i.provider.Delete(ctx); err != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		} else if len(list.Items) >= 1 {
			infrastructureActive.Set(1)
			if i.lastInfrastructureReconcile.Add(time.Hour).Before(time.Now()) {
				if err := i.provider.Create(ctx); err != nil {
					infrastructureHealthy.Set(0)
					return reconcile.Result{}, err
				}
				i.lastInfrastructureReconcile = time.Now()
				infrastructureHealthy.Set(1)
			}
			// TODO: Implement an alerting mechanism for settings updates; until then, just poll
			return reconcile.Result{RequeueAfter: time.Second * 10}, nil
		}
	}
	infrastructureActive.Set(0)
	infrastructureHealthy.Set(0)
	return reconcile.Result{}, nil
}
