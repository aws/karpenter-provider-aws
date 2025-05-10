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
	"fmt"
	"strings"
	"time"

	"github.com/awslabs/operatorpkg/singleton"
	lop "github.com/samber/lo/parallel"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/karpenter/pkg/operator/injection"

	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
)

type Controller struct {
	kubeClient      client.Client
	pricingProvider pricing.Provider
}

func NewController(kubeClient client.Client, pricingProvider pricing.Provider) *Controller {
	return &Controller{
		kubeClient:      kubeClient,
		pricingProvider: pricingProvider,
	}
}

func (c *Controller) Reconcile(ctx context.Context) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "providers.pricing")

	work := []func(ctx context.Context) error{
		c.pricingProvider.UpdateSpotPricing,
		c.pricingProvider.UpdateOnDemandPricing,
	}
	errs := make([]error, len(work))
	lop.ForEach(work, func(f func(ctx context.Context) error, i int) {
		if err := f(ctx); err != nil {
			errs[i] = err
		}
	})
	if err := multierr.Combine(errs...); err != nil {
		return reconcile.Result{}, fmt.Errorf("updating pricing, %w", err)
	}
	return reconcile.Result{RequeueAfter: 12 * time.Hour}, nil
}

func (c *Controller) ReconcileSpotPriceMaxConfigMap(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "providers.pricing.configmap")
	
	// Get the ConfigMap
	configMap := &corev1.ConfigMap{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, configMap); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	
	// Update max spot prices with data from ConfigMap
	c.pricingProvider.UpdateMaxSpotPrices(ctx, configMap.Data)
	
	return reconcile.Result{}, nil
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	// Register the regular pricing controller
	if err := controllerruntime.NewControllerManagedBy(m).
		Named("providers.pricing").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c)); err != nil {
		return err
	}
	
	// Register the ConfigMap controller if a ConfigMap is specified
	opts := options.FromContext(ctx)
	if opts != nil && opts.SpotPriceMaxConfigMap != "" {
		// Parse namespace/name format
		parts := strings.SplitN(opts.SpotPriceMaxConfigMap, "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid format for spot-price-max-configmap, expected namespace/name, got %s", opts.SpotPriceMaxConfigMap)
		}
		
		namespace, name := parts[0], parts[1]
		namespacedName := types.NamespacedName{Namespace: namespace, Name: name}
		
		// Create a configmap controller that watches for changes to the specific ConfigMap
		return controllerruntime.NewControllerManagedBy(m).
			Named("providers.pricing.configmap").
			For(&corev1.ConfigMap{}).
			WithEventFilter(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return e.Object.GetNamespace() == namespace && e.Object.GetName() == name
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					return e.ObjectNew.GetNamespace() == namespace && e.ObjectNew.GetName() == name
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return e.Object.GetNamespace() == namespace && e.Object.GetName() == name
				},
			}).
			Watches(
				&source.Kind{Type: &corev1.ConfigMap{}},
				&handler.EnqueueRequestForObject{},
				builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
					return obj.GetNamespace() == namespace && obj.GetName() == name
				})),
			).
			Complete(reconcile.Func(c.ReconcileSpotPriceMaxConfigMap))
	}
	
	return nil
}
