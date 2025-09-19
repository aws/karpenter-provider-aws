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

package version

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/operatorpkg/singleton"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/karpenter/pkg/operator/injection"

	"github.com/aws/karpenter-provider-aws/pkg/providers/version"
)

type UpdateVersion func(context.Context) error

type Controller struct {
	versionProvider *version.DefaultProvider
	updateVersion   UpdateVersion
}

func NewController(versionProvider *version.DefaultProvider, updateVersion UpdateVersion) *Controller {
	return &Controller{
		versionProvider: versionProvider,
		updateVersion:   updateVersion,
	}
}

func (c *Controller) Reconcile(ctx context.Context) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "providers.version")

	if err := c.updateVersion(ctx); err != nil {
		return reconcile.Result{}, fmt.Errorf("updating version, %w", err)
	}
	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	// Includes a default exponential failure rate limiter of base: time.Millisecond, and max: 1000*time.Second
	return controllerruntime.NewControllerManagedBy(m).
		Named("providers.version").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}
