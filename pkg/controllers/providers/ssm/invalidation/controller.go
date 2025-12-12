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

package invalidation

import (
	"context"
	"time"

	"github.com/awslabs/operatorpkg/reconciler"
	"github.com/awslabs/operatorpkg/singleton"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/karpenter/pkg/operator/injection"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

// The SSM Invalidation controller is responsible for invalidating "latest" SSM parameters when they point to deprecated
// AMIs. This can occur when an EKS-optimized AMI with a regression is released, and the AMI team chooses to deprecate
// the AMI. Normally, SSM parameter cache entries expire after 24 hours to prevent a thundering herd upon a new AMI
// release, however Karpenter should react faster when an AMI is deprecated. This controller will ensure Karpenter
// reacts to AMI deprecations within it's polling period (30m).
type Controller struct {
	cache       *cache.Cache
	amiProvider amifamily.Provider
}

func NewController(ssmCache *cache.Cache, amiProvider amifamily.Provider) *Controller {
	return &Controller{
		cache:       ssmCache,
		amiProvider: amiProvider,
	}
}

func (c *Controller) Name() string {
	return "providers.ssm.invalidation"
}

func (c *Controller) Reconcile(ctx context.Context) (reconciler.Result, error) {
	ctx = injection.WithControllerName(ctx, c.Name())

	amiIDsToParameters := map[string]ssm.Parameter{}
	for _, item := range c.cache.Items() {
		entry := item.Object.(ssm.CacheEntry)
		if !entry.Parameter.IsMutable {
			continue
		}
		amiIDsToParameters[entry.Value] = entry.Parameter
	}
	amis := []amifamily.AMI{}
	for _, nodeClass := range lo.Map(lo.Keys(amiIDsToParameters), func(amiID string, _ int) *v1.EC2NodeClass {
		return &v1.EC2NodeClass{
			ObjectMeta: metav1.ObjectMeta{
				UID: uuid.NewUUID(), // ensures that this doesn't hit the AMI cache.
			},
			Spec: v1.EC2NodeClassSpec{
				AMISelectorTerms: []v1.AMISelectorTerm{{ID: amiID}},
			},
		}
	}) {
		resolvedAMIs, err := c.amiProvider.List(ctx, nodeClass)
		if err != nil {
			return reconciler.Result{}, err
		}
		amis = append(amis, resolvedAMIs...)
	}
	for _, ami := range amis {
		if !ami.Deprecated {
			continue
		}
		parameter := amiIDsToParameters[ami.AmiID]
		c.cache.Delete(parameter.CacheKey())
	}
	return reconciler.Result{RequeueAfter: 30 * time.Minute}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named(c.Name()).
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}
