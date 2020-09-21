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

// +kubebuilder:rbac:groups=autoscaling.karpenter.sh,resources=horizontalautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling.karpenter.sh,resources=horizontalautoscalers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=autoscaling.karpenter.sh,resources=scalablenodegroups/scale,verbs=get;update;patch

package v1alpha1

import (
	"context"
	"fmt"
	"time"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/autoscaler"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TODO, make these configmappable or wired through the API.
const (
	DefaultAutoscalingPeriod = 10 * time.Second
)

// Controller reconciles a HorizontalAutoscaler object
type Controller struct {
	client.Client
	AutoscalerFactory autoscaler.Factory
}

// For returns the resource this controller is for.
func (c *Controller) For() runtime.Object {
	return &v1alpha1.HorizontalAutoscaler{}
}

// Owns returns the resources owned by this controller's resource.
func (c *Controller) Owns() []runtime.Object {
	return []runtime.Object{}
}

// Reconcile executes a control loop for the HorizontalAutoscaler resource
// For now, assume a singleton architecture where all definitions are handled in a single shard.
// In the future, we may wish to do some sort of sharded assignment to spread definitions across many controller instances.
func (c *Controller) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	// 1. Retrieve resource spec from API Server
	resource := &v1alpha1.HorizontalAutoscaler{}
	if err := c.Get(context.Background(), req.NamespacedName, resource); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	resource.RuntimeDefault()

	// 2. Execute autoscaling logic, do not exit on error
	autoscaler := c.AutoscalerFactory.For(resource)
	if err := autoscaler.Reconcile(); err != nil {
		resource.Status.MarkNotAbleToScale(err.Error())
	} else {
		resource.Status.MarkAbleToScale()
	}

	// 3. Apply status to API Server
	if err := c.Status().Update(context.Background(), resource); err != nil {
		return reconcile.Result{}, fmt.Errorf("Failed to persist changes: %s", err.Error())
	}

	return controllerruntime.Result{RequeueAfter: DefaultAutoscalingPeriod}, nil
}
