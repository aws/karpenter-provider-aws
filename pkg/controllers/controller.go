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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
	if err := c.Controller.Reconcile(ctx, resource); err != nil {
		resource.StatusConditions().MarkFalse(v1alpha1.Active, "", err.Error())
		zap.S().Errorf("Controller failed to reconcile kind %s, %s",
			resource.GetObjectKind().GroupVersionKind().Kind, err.Error())
	} else {
		resource.StatusConditions().MarkTrue(v1alpha1.Active)
	}
	// 5. Update Status using a merge patch
	if err := c.Status().Patch(ctx, resource, client.MergeFrom(persisted)); err != nil {
		return reconcile.Result{}, fmt.Errorf("Failed to persist changes to %s, %w", req.NamespacedName, err)
	}
	return reconcile.Result{RequeueAfter: c.Interval()}, nil
}
