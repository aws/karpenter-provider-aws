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

package instance

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/operatorpkg/reconciler"
	"github.com/awslabs/operatorpkg/singleton"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/karpenter/pkg/operator/injection"

	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
)

// cacheRefreshInterval is how often the instance cache is refreshed from EC2. It is the single fleet-wide
// DescribeInstances cadence shared by all cache consumers (garbage collection, capacity-type reconciliation, and
// cache-first Get lookups). It is set to match the tightest consumer freshness need (capacity-type reconciliation
// previously listed every minute) while collapsing what were previously independent per-controller List sweeps into
// one.
const cacheRefreshInterval = time.Minute

// Controller owns refreshing the instance provider's cache. Separating the "populate the cache" responsibility into a
// dedicated controller lets instance.Provider.List/Get become pure cache reads, so consumers no longer each issue
// their own fleet-wide DescribeInstances.
type Controller struct {
	instanceProvider instance.Provider
}

func NewController(instanceProvider instance.Provider) *Controller {
	return &Controller{
		instanceProvider: instanceProvider,
	}
}

func (*Controller) Name() string {
	return "providers.instance.cache"
}

func (c *Controller) Reconcile(ctx context.Context) (reconciler.Result, error) {
	ctx = injection.WithControllerName(ctx, c.Name())
	if err := c.instanceProvider.SyncCache(ctx); err != nil {
		return reconciler.Result{}, fmt.Errorf("syncing instance cache, %w", err)
	}
	// Record the successful sync time so cache staleness (exposed as seconds_since_last_sync) is observable. If this
	// controller stalls, that gauge keeps climbing while List/Get serve an increasingly stale cache.
	recordSync(time.Now())
	return reconciler.Result{RequeueAfter: cacheRefreshInterval}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named(c.Name()).
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}
