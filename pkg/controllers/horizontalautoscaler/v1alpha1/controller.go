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
package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/ellistarn/karpenter/pkg/apis/horizontalautoscaler/v1alpha1"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller reconciles a HorizontalAutoscaler object
type Controller struct {
	client.Client
	Autoscalers map[AutoscalerKey]Autoscaler
}

// AutoscalerKey is a unique key for an Autoscaler
type AutoscalerKey struct {
	NodeGroup               string
	HorizontalPodAutoscaler types.NamespacedName
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
// +kubebuilder:rbac:groups=karpenter.sh,resources=horizontalautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=karpenter.sh,resources=horizontalautoscalers/status,verbs=get;update;patch
func (c *Controller) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	// For now, assume a singleton architecture where all definitions are handled in a single shard.
	// In the future, we may wish to do some sort of sharded assignment to spread definitions across many controller instances.
	ha := &v1alpha1.HorizontalAutoscaler{}
	if err := c.Get(context.Background(), req.NamespacedName, ha); err != nil {
		if errors.IsNotFound(err) {
			zap.S().Infof("Removing definition for %s.", req.NamespacedName)
			delete(c.Autoscalers, AutoscalerKey{
				// TODO: include NodeGroup
				HorizontalPodAutoscaler: req.NamespacedName,
			})
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	zap.S().Infof("Updating definition for %s.", req.NamespacedName)
	c.Autoscalers[AutoscalerKey{
		HorizontalPodAutoscaler: req.NamespacedName,
	}] = Autoscaler{
		// TODO: include NodeGroup
		HorizontalAutoscaler: ha,
	}

	return controllerruntime.Result{}, nil
}

// Start initializes the analysis loop for known Autoscalers
func (c *Controller) Start() {
	zap.S().Infof("Starting analysis loop")
	for {
		// TODO: Use goroutines or something smarter.
		for _, a := range c.Autoscalers {
			if err := a.Reconcile(); err != nil {
				zap.S().Warnf("Continuing after failing to reconcile autoscaler, %v")
			}
		}
		time.Sleep(10 * time.Second)
	}
}
