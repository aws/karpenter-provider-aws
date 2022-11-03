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

package nodetemplate

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
)

type FinalizerReconciler struct{}

func NewFinalizerReconciler() *FinalizerReconciler {
	return &FinalizerReconciler{}
}

// Reconcile adds the finalizer if the nodeTemplate doesn't have it or removes the finalizer
// if the nodeTemplate is being deleted
func (r *FinalizerReconciler) Reconcile(_ context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) (reconcile.Result, error) {
	if !nodeTemplate.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(nodeTemplate, v1alpha1.InterruptionInfrastructureFinalizer)
		return reconcile.Result{}, nil
	}
	controllerutil.AddFinalizer(nodeTemplate, v1alpha1.InterruptionInfrastructureFinalizer)
	return reconcile.Result{}, nil
}
