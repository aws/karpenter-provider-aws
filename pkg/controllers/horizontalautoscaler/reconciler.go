package horizontalautoscaler

import (
	"context"
	"time"

	v1alpha1 "github.com/ellistarn/karpenter/pkg/apis/horizontalautoscaler/v1alpha1"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler reconciles a HorizontalAutoscaler object
type Reconciler struct {
	client.Client
	Autoscalers map[AutoscalerKey]Autoscaler
}

// AutoscalerKey is a unique key for an Autoscaler
type AutoscalerKey struct {
	NodeGroup               string
	HorizontalPodAutoscaler types.NamespacedName
}

// Reconcile executes a control loop for the HorizontalAutoscaler resource
// +kubebuilder:rbac:groups=karpenter.sh,resources=horizontalautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=karpenter.sh,resources=horizontalautoscalers/status,verbs=get;update;patch
func (r *Reconciler) Reconcile(req controllerruntime.Request) (controllerruntime.Result, error) {
	// For now, assume a singleton architecture where all definitions are handled in a single shard.
	// In the future, we may wish to do some sort of sharded assignment to spread definitions across many controller instances.
	ha := &v1alpha1.HorizontalAutoscaler{}
	if err := r.Get(context.Background(), req.NamespacedName, ha); err != nil {
		if errors.IsNotFound(err) {
			zap.S().Infof("Removing definition for %s.", req.NamespacedName)
			delete(r.Autoscalers, AutoscalerKey{
				// TODO: include NodeGroup
				HorizontalPodAutoscaler: req.NamespacedName,
			})
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	zap.S().Infof("Updating definition for %s.", req.NamespacedName)
	r.Autoscalers[AutoscalerKey{
		HorizontalPodAutoscaler: req.NamespacedName,
	}] = Autoscaler{
		// TODO: include NodeGroup
		HorizontalAutoscaler: ha,
	}

	return controllerruntime.Result{}, nil
}

// Start initializes the analysis loop for known Autoscalers
func (r *Reconciler) Start() {
	zap.S().Infof("Starting analysis loop")
	for {
		// TODO: Use goroutines or something smarter.
		for _, a := range r.Autoscalers {
			if err := a.Reconcile(); err != nil {
				zap.S().Warnf("Continuing after failing to reconcile autoscaler, %v")
			}
		}
		time.Sleep(10 * time.Second)
	}
}
