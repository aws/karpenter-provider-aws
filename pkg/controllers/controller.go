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

package controllers

import (
	"context"
	"fmt"

	"github.com/awslabs/karpenter/pkg/utils/conditions"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// GenericController implements controllerruntime.Reconciler and runs a
// standardized reconciliation workflow against incoming resource watch events.
type GenericController struct {
	Controller
	client.Client
}

// Reconcile executes a control loop for the resource
func (c *GenericController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	// 1. Read Spec
	resource := c.For()
	if err := c.Get(ctx, req.NamespacedName, resource); err != nil {
		if errors.IsNotFound(err) || errors.IsGone(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{Requeue: true}, err
	}
	// 2. Copy object for merge patch base
	persisted := resource.DeepCopyObject()
	// 3. Set defaults to enforce invariants on object being reconciled
	if _, ok := resource.(apis.Defaultable); ok {
		resource.(apis.Defaultable).SetDefaults(ctx)
	}
	// 4. Set to true to remove race condition where multiple controllers set the status of the same object
	// TODO: remove status conditions on provisioners
	if conditionsAccessor, ok := resource.(apis.ConditionsAccessor); ok {
		apis.NewLivingConditionSet(conditions.Active).Manage(conditionsAccessor).MarkTrue(conditions.Active)
	}

	// 4. Reconcile
	result, err := c.Controller.Reconcile(ctx, resource)
	if err != nil {
		zap.S().Errorf("Controller failed to reconcile kind %s, %s",
			resource.GetObjectKind().GroupVersionKind().Kind, err.Error())
		return result, err
	}
	// 6. Update Status using a merge patch
	if err := c.Status().Patch(ctx, resource, client.MergeFrom(persisted)); err != nil {
		return result, fmt.Errorf("Failed to persist changes to %s, %w", req.NamespacedName, err)
	}
	return result, nil
}

// WatchDescription returns the necessary information to create a watch
//   a. source: the resource that is being watched
//   b. eventHandler: which controller objects to be reconciled
//   c. predicates: which events can be filtered out before processed
func (c *GenericController) Watches() (source.Source, handler.EventHandler, builder.WatchesOption) {
	return c.Controller.Watches()
}
