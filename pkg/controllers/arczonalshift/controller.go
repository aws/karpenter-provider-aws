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

package arczonalshift

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/operatorpkg/reconciler"
	"github.com/awslabs/operatorpkg/singleton"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/karpenter/pkg/operator/injection"

	"github.com/aws/karpenter-provider-aws/pkg/providers/arczonalshift"
)

type Controller struct {
	arczonalshiftProvider arczonalshift.Provider
}

func NewController(
	arczonalshiftProvider arczonalshift.Provider,
) *Controller {
	return &Controller{
		arczonalshiftProvider: arczonalshiftProvider,
	}
}

func (c *Controller) Reconcile(ctx context.Context) (reconciler.Result, error) {
	ctx = injection.WithControllerName(ctx, "zonalshift")
	err := c.arczonalshiftProvider.UpdateZonalShifts(ctx)
	if err != nil {
		return reconciler.Result{}, fmt.Errorf("updating zonal shifts: %w", err)
	}

	return reconciler.Result{RequeueAfter: 30 * time.Second}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).Named("zonalshift").WatchesRawSource(singleton.Source()).Complete(singleton.AsReconciler(c))
}
