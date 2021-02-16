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
	"time"

	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/awslabs/karpenter/pkg/apis/autoscaling/v1alpha1"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller is an interface implemented by Karpenter custom resources.
type Controller interface {
	// Reconcile hands a hydrated kubernetes resource to the controller for
	// reconciliation. Any changes made to the resource's status are persisted
	// after Reconcile returns, even if it returns an error.
	Reconcile(Object) error
	// Interval returns an interval that the controller should wait before
	// executing another reconciliation loop. If set to zero, will only execute
	// on watch events or the global resync interval.
	Interval() time.Duration
	// For returns a default instantiation of the resource and is injected by
	// data from the API Server at the start of the reconciliation loop.
	For() Object
	// Owns returns a slice of resources that are watched by this resources.
	// Watch events are triggered if owner references are set on the resource.
	Owns() []Object
}

// NamedController allows controllers to optionally implement a Name() function which will be used instead of the
// reconciled resource's name. This is useful when writing multiple controllers for a single resource type.
type NamedController interface {
	Controller
	// Name returns the name of the controller
	Name() string
}

// Object provides an abstraction over a kubernetes custom resource with
// methods necessary to standardize reconciliation behavior in Karpenter.
type Object interface {
	client.Object
	webhook.Validator
	webhook.Defaulter
	StatusConditions() apis.ConditionManager
}

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
	// 3. Validate
	if err := c.For().ValidateCreate(); err != nil {
		resource.StatusConditions().MarkFalse(v1alpha1.Active, "could not validate kind %s, %s",
			resource.GetObjectKind().GroupVersionKind().Kind, err.Error())
		zap.S().Errorf("Controller failed to validate kind %s, %s",
			resource.GetObjectKind().GroupVersionKind().Kind, err.Error())
		// 4. Reconcile
	} else if err := c.Controller.Reconcile(resource); err != nil {
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
