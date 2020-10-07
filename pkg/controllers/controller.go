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
	"time"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller is an interface implemented by Karpenter custom resources.
type Controller interface {
	// Reconcile hands a hydrated kubernetes object to the controller for
	// reconciliation. Any changes made to the resource's status are persisted
	// after Reconcile returns, even if it returns an error.
	Reconcile(Object) error
	// Interval returns an interval that the controller should wait before
	// executing another reconciliation loop. If set to zero, will only execute
	// on watch events or the global resync interval.
	Interval() time.Duration
	// For returns a default instantiation of the object and is inject by data
	// from the API Server at the start of the reconcilation loop.
	For() Object
	// Owns returns a slice of objects that are watched by this resources. Watch
	// events are triggered if owner references are set for the owned resource.
	Owns() []Object
}

// Object provides an abstraction over a kubernetes custom resource with
// methods necessary to standardize reconciliation behavior in Karpenter.
type Object interface {
	runtime.Object
	metav1.Object
	StatusConditions() apis.ConditionManager
}

// GenericController implements controllerruntime.Reconciler and runs a
// standardized reconcilation workflow against incoming object watch events.
type GenericController struct {
	Controller
	client.Client
}

// Reconcile executes a control loop for the resource
func (c *GenericController) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	// 1. Read Spec
	resource := c.For()
	if err := c.Get(context.Background(), req.NamespacedName, resource); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	// 2. Reconcile
	if err := c.Controller.Reconcile(resource); err != nil {
		resource.StatusConditions().MarkFalse(v1alpha1.Active, "", err.Error())
	} else {
		resource.StatusConditions().MarkTrue(v1alpha1.Active)
	}
	// 3. Update Status
	if err := c.Status().Update(context.Background(), resource); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "Failed to persist changes to %s", req.NamespacedName)
	}

	return reconcile.Result{RequeueAfter: c.Interval()}, nil
}

// Register registers the controller to the provided controllerruntime.Manager.
func Register(manager controllerruntime.Manager, controller Controller) error {
	var builder = controllerruntime.NewControllerManagedBy(manager).For(controller.For())
	for _, resource := range controller.Owns() {
		builder = builder.Owns(resource)
	}
	if err := builder.Complete(&GenericController{Controller: controller, Client: manager.GetClient()}); err != nil {
		return errors.Wrapf(err, "registering controller to manager for resource %v", controller.For())
	}
	if err := controllerruntime.NewWebhookManagedBy(manager).For(controller.For()).Complete(); err != nil {
		return errors.Wrapf(err, "registering webhook to manager for resource %v", controller.For())
	}
	return nil
}
