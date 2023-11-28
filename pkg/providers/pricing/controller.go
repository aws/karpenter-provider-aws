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

package pricing

import (
	"context"
	"time"

	lop "github.com/samber/lo/parallel"
	"go.uber.org/multierr"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corecontroller "sigs.k8s.io/karpenter/pkg/operator/controller"
)

type Controller struct {
	pricingProvider *Provider
}

func NewController(pricingProvider *Provider) *Controller {
	return &Controller{
		pricingProvider: pricingProvider,
	}
}

func (c *Controller) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: 12 * time.Hour}, c.updatePricing(ctx)
}

func (c *Controller) Name() string {
	return "pricing"
}

func (c *Controller) Builder(_ context.Context, m manager.Manager) corecontroller.Builder {
	return corecontroller.NewSingletonManagedBy(m)
}

func (c *Controller) updatePricing(ctx context.Context) error {
	work := []func(ctx context.Context) error{
		c.pricingProvider.UpdateOnDemandPricing,
		c.pricingProvider.UpdateSpotPricing,
	}
	errs := make([]error, len(work))
	lop.ForEach(work, func(f func(ctx context.Context) error, i int) {
		if err := f(ctx); err != nil {
			errs[i] = err
		}
	})

	return multierr.Combine(errs...)
}
