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

package instancetype

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/operatorpkg/reconciler"
	"github.com/awslabs/operatorpkg/singleton"
	lop "github.com/samber/lo/parallel"
	"go.uber.org/multierr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/karpenter/pkg/operator/injection"

	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
)

type Controller struct {
	instanceTypeProvider *instancetype.DefaultProvider
}

func NewController(instanceTypeProvider *instancetype.DefaultProvider) *Controller {
	return &Controller{
		instanceTypeProvider: instanceTypeProvider,
	}
}

func (c *Controller) Reconcile(ctx context.Context) (reconciler.Result, error) {
	ctx = injection.WithControllerName(ctx, "providers.instancetype")

	work := []func(ctx context.Context) error{
		c.instanceTypeProvider.UpdateInstanceTypes,
		c.instanceTypeProvider.UpdateInstanceTypeOfferings,
	}
	errs := make([]error, len(work))
	lop.ForEach(work, func(f func(ctx context.Context) error, i int) {
		if err := f(ctx); err != nil {
			errs[i] = err
		}
	})
	if err := multierr.Combine(errs...); err != nil {
		return reconciler.Result{}, fmt.Errorf("updating instancetype, %w", err)
	}
	return reconciler.Result{RequeueAfter: 12 * time.Hour}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	// Includes a default exponential failure rate limiter of base: time.Millisecond, and max: 1000*time.Second
	return controllerruntime.NewControllerManagedBy(m).
		Named("providers.instancetype").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}
