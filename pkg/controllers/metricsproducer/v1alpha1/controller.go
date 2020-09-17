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

// +kubebuilder:rbac:groups=autoscaling.karpenter.sh,resources=metricsproducers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling.karpenter.sh,resources=metricsproducers/status,verbs=get;update;patch

package v1alpha1

import (
	"context"
	"time"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	DefaultMetricAnalysisPeriodSeconds = 10
)

// Controller for the resource
type Controller struct {
	client.Client
}

// For returns the resource this controller is for.
func (c *Controller) For() runtime.Object {
	return &v1alpha1.MetricsProducer{}
}

// Owns returns the resources owned by this controller's resource.
func (c *Controller) Owns() []runtime.Object {
	return []runtime.Object{}
}

// Reconcile executes a control loop for the resource
func (c *Controller) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	zap.S().Infof("Reconciling %s.", req.String())
	// 1. Retrieve resource from API Server
	resource := &v1alpha1.MetricsProducer{}
	if err := c.Get(context.Background(), req.NamespacedName, resource); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// 2. Calculate metrics

	// 3. Apply changes to API Server
	if err := c.Update(context.Background(), resource); err != nil {
		return reconcile.Result{}, errors.Cause(errors.Wrapf(err, "Failed to persist changes to %s", req.NamespacedName))
	}

	return reconcile.Result{
		RequeueAfter: time.Second * DefaultMetricAnalysisPeriodSeconds,
	}, nil
}
