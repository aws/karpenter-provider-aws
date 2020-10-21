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

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller is an interface implemented by Karpenter custom resources.
type Controller interface {
	// Reconcile hands a hydrated kubernetes resource to the controller for
	// reconciliation. Any changes made to the resource's status are persisted
	// after Reconcile returns, even if it returns an error.
	Reconcile(Resource) error
	// Interval returns an interval that the controller should wait before
	// executing another reconciliation loop. If set to zero, will only execute
	// on watch events or the global resync interval.
	Interval() time.Duration
	// For returns a default instantiation of the resource and is injected by
	// data from the API Server at the start of the reconcilation loop.
	For() Resource
	// Owns returns a slice of resources that are watched by this resources.
	// Watch events are triggered if owner references are set on the resource.
	Owns() []Resource
}

// Resource provides an abstraction over a kubernetes custom resource with
// methods necessary to standardize reconciliation behavior in Karpenter.
type Resource interface {
	runtime.Object
	metav1.Object
	StatusConditions() apis.ConditionManager
}

// GenericController implements controllerruntime.Reconciler and runs a
// standardized reconcilation workflow against incoming resource watch events.
type GenericController struct {
	Controller
	client.Client
}

// Reconcile executes a control loop for the resource
func (c *GenericController) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	// 1. Read Spec
	resource := c.For()
	if err := c.Get(context.Background(), req.NamespacedName, resource); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	// 2. Copy object for merge patch base
	persisted := resource.DeepCopyObject()
	// 3. Reconcile
	if err := c.Controller.Reconcile(resource); err != nil {
		resource.StatusConditions().MarkFalse(v1alpha1.Active, "", err.Error())
	} else {
		resource.StatusConditions().MarkTrue(v1alpha1.Active)
	}
	// 4. Update Status using a merge patch
	if err := c.Status().Patch(context.Background(), resource, client.MergeFrom(persisted)); err != nil {
		return reconcile.Result{}, fmt.Errorf("Failed to persist changes to %s, %w", req.NamespacedName, err)
	}
	return reconcile.Result{RequeueAfter: c.Interval()}, nil
}
