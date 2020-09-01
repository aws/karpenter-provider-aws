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
package scalablenodegroup

import (
	"context"

	"github.com/ellistarn/karpenter/pkg/apis/scalablenodegroup/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler for the resource
type Reconciler struct {
	client.Client
}

// Reconcile executes a control loop for the resource
// +kubebuilder:rbac:groups=karpenter.sh,resources=scalablenodegroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=karpenter.sh,resources=scalablenodegroups/status,verbs=get;update;patch
func (r *Reconciler) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	mp := &v1alpha1.ScalableNodeGroup{}
	if err := r.Get(context.Background(), req.NamespacedName, mp); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	return controllerruntime.Result{}, nil
}
