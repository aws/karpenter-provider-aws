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

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	// 2. Copy object for merge patch base
	persisted := resource.DeepCopyObject()
	// 3. Reconcile
	result, err := c.Controller.Reconcile(ctx, resource)
	if err != nil {
		zap.S().Errorf("Controller failed to reconcile kind %s, %s", resource.GetObjectKind().GroupVersionKind().Kind, err.Error())
	}
	// 5. Update Status using a merge patch
	// If the controller is reconciling nodes, don't patch
	if _, ok := resource.(*v1.Node); !ok {
		if err := c.Status().Patch(ctx, resource, client.MergeFrom(persisted)); err != nil {
			return result, fmt.Errorf("Failed to persist changes to %s, %w", req.NamespacedName, err)
		}
	}
	return result, err
}

// Watches returns the necessary information to create a watch
//   a. source: the resource that is being watched
//   b. eventHandler: which controller objects to be reconciled
//   c. predicates: which events can be filtered out before processed
func (c *GenericController) Watches(ctx context.Context) (source.Source, handler.EventHandler, builder.WatchesOption) {
	return c.Controller.Watches(ctx)
}

// ConcurrentReconciles controls the number of concurrent reconciles that can occur
func (c *GenericController) ConcurrentReconciles() int {
	return c.Controller.ConcurrentReconciles()
}
