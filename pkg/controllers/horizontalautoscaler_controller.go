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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	karpenterv1alpha1 "github.com/ellistarn/karpenter/pkg/api/v1alpha1"
)

// HorizontalAutoscalerReconciler reconciles a HorizontalAutoscaler object
type HorizontalAutoscalerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Reconcile executes a control loop for the HorizontalAutoscaler resource
// +kubebuilder:rbac:groups=karpenter.sh,resources=horizontalautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=karpenter.sh,resources=horizontalautoscalers/status,verbs=get;update;patch
func (r *HorizontalAutoscalerReconciler) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("horizontalautoscaler", req.NamespacedName)

	// Detect policy
	// Add Node Group to state

	// your logic here

	return controllerruntime.Result{}, nil
}

// SetupWithManager attaches the reconciler to the provided Manager.
func (r *HorizontalAutoscalerReconciler) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		For(&karpenterv1alpha1.HorizontalAutoscaler{}).
		Complete(r)
}
