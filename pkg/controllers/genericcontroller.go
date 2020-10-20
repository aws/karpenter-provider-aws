package controllers

import (
	"context"
	"fmt"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

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
		if apierrors.IsNotFound(err) {
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
