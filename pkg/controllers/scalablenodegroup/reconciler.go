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
