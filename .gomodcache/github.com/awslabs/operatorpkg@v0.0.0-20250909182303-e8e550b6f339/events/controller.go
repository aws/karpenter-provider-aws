package events

import (
	"context"
	"fmt"
	"strings"
	"time"

	pmetrics "github.com/awslabs/operatorpkg/metrics"
	"github.com/awslabs/operatorpkg/object"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/clock"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Controller[T client.Object] struct {
	gvk        schema.GroupVersionKind
	startTime  time.Time
	kubeClient client.Client
	EventCount pmetrics.CounterMetric
}

func NewController[T client.Object](client client.Client, clock clock.Clock) *Controller[T] {
	gvk := object.GVK(object.New[T]())
	return &Controller[T]{
		gvk:        gvk,
		startTime:  clock.Now(),
		kubeClient: client,
		EventCount: eventTotalMetric(strings.ToLower(gvk.Kind)),
	}
}

func (c *Controller[T]) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		For(&v1.Event{}, builder.WithPredicates(predicate.NewTypedPredicateFuncs(func(o client.Object) bool {
			// Only reconcile on the object kind we care about
			event := o.(*v1.Event)
			return event.InvolvedObject.Kind == c.gvk.Kind && event.InvolvedObject.APIVersion == c.gvk.GroupVersion().String()
		}))).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Named(fmt.Sprintf("operatorpkg.%s.events", strings.ToLower(c.gvk.Kind))).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}

func (c *Controller[T]) Reconcile(ctx context.Context, event *v1.Event) (reconcile.Result, error) {
	// We check if the event was created in the lifetime of this controller
	// since we don't duplicate metrics on controller restart or lease handover
	if c.startTime.Before(event.LastTimestamp.Time) {
		c.EventCount.Inc(map[string]string{
			pmetrics.LabelType:   event.Type,
			pmetrics.LabelReason: event.Reason,
		})
	}

	return reconcile.Result{}, nil
}
