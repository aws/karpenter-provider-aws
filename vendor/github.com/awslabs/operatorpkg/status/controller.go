package status

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	pmetrics "github.com/awslabs/operatorpkg/metrics"
	"github.com/awslabs/operatorpkg/object"
	"github.com/awslabs/operatorpkg/option"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Controller[T Object] struct {
	gvk                           schema.GroupVersionKind
	additionalMetricLabels        []string
	kubeClient                    client.Client
	eventRecorder                 record.EventRecorder
	observedConditions            sync.Map // map[reconcile.Request]ConditionSet
	observedFinalizers            sync.Map // map[reconcile.Request]Finalizer
	terminatingObjects            sync.Map // map[reconcile.Request]Object
	emitDeprecatedMetrics         bool
	ConditionDuration             pmetrics.ObservationMetric
	ConditionCount                pmetrics.GaugeMetric
	ConditionCurrentStatusSeconds pmetrics.GaugeMetric
	ConditionTransitionsTotal     pmetrics.CounterMetric
	TerminationCurrentTimeSeconds pmetrics.GaugeMetric
	TerminationDuration           pmetrics.ObservationMetric
}

type Option struct {
	// Current list of deprecated metrics
	// - operator_status_condition_transitions_total
	// - operator_status_condition_transition_seconds
	// - operator_status_condition_current_status_seconds
	// - operator_status_condition_count
	// - operator_termination_current_time_seconds
	// - operator_termination_duration_seconds
	EmitDeprecatedMetrics bool
	MetricLabels          []string
}

func EmitDeprecatedMetrics(o *Option) {
	o.EmitDeprecatedMetrics = true
}

func WithLabels(labels ...string) func(*Option) {
	return func(o *Option) {
		o.MetricLabels = append(o.MetricLabels, labels...)
	}
}

func NewController[T Object](client client.Client, eventRecorder record.EventRecorder, opts ...option.Function[Option]) *Controller[T] {
	options := option.Resolve(opts...)
	obj := reflect.New(reflect.TypeOf(*new(T)).Elem()).Interface().(runtime.Object)
	obj.GetObjectKind().SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
	gvk := object.GVK(obj)

	return &Controller[T]{
		gvk:                           gvk,
		additionalMetricLabels:        options.MetricLabels,
		kubeClient:                    client,
		eventRecorder:                 eventRecorder,
		emitDeprecatedMetrics:         options.EmitDeprecatedMetrics,
		ConditionDuration:             conditionDurationMetric(strings.ToLower(gvk.Kind), lo.Map(options.MetricLabels, func(k string, _ int) string { return toPrometheusLabel(k) })...),
		ConditionCount:                conditionCountMetric(strings.ToLower(gvk.Kind), lo.Map(options.MetricLabels, func(k string, _ int) string { return toPrometheusLabel(k) })...),
		ConditionCurrentStatusSeconds: conditionCurrentStatusSecondsMetric(strings.ToLower(gvk.Kind), lo.Map(options.MetricLabels, func(k string, _ int) string { return toPrometheusLabel(k) })...),
		ConditionTransitionsTotal:     conditionTransitionsTotalMetric(strings.ToLower(gvk.Kind), lo.Map(options.MetricLabels, func(k string, _ int) string { return toPrometheusLabel(k) })...),
		TerminationCurrentTimeSeconds: terminationCurrentTimeSecondsMetric(strings.ToLower(gvk.Kind), lo.Map(options.MetricLabels, func(k string, _ int) string { return toPrometheusLabel(k) })...),
		TerminationDuration:           terminationDurationMetric(strings.ToLower(gvk.Kind), lo.Map(options.MetricLabels, func(k string, _ int) string { return toPrometheusLabel(k) })...),
	}
}

func (c *Controller[T]) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		For(object.New[T]()).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Named(fmt.Sprintf("operatorpkg.%s.status", strings.ToLower(c.gvk.Kind))).
		Complete(c)
}

func (c *Controller[T]) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return c.reconcile(ctx, req, object.New[T]())
}

type GenericObjectController[T client.Object] struct {
	*Controller[*unstructuredAdapter[T]]
}

func NewGenericObjectController[T client.Object](client client.Client, eventRecorder record.EventRecorder, opts ...option.Function[Option]) *GenericObjectController[T] {
	return &GenericObjectController[T]{
		Controller: NewController[*unstructuredAdapter[T]](client, eventRecorder, opts...),
	}
}

func (c *GenericObjectController[T]) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		For(object.New[T]()).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Named(fmt.Sprintf("operatorpkg.%s.status", strings.ToLower(reflect.TypeOf(object.New[T]()).Elem().Name()))).
		Complete(c)
}

func (c *GenericObjectController[T]) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return c.reconcile(ctx, req, NewUnstructuredAdapter[T](object.New[T]()))
}

func (c *Controller[T]) toAdditionalMetricLabels(obj Object) map[string]string {
	return lo.SliceToMap(c.additionalMetricLabels, func(label string) (string, string) { return toPrometheusLabel(label), obj.GetLabels()[label] })
}

func toPrometheusLabel(k string) string {
	unsupportedChars := []string{"/", "."}
	for _, char := range unsupportedChars {
		k = strings.ReplaceAll(k, char, "_")
	}
	return k
}

func (c *Controller[T]) reconcile(ctx context.Context, req reconcile.Request, o Object) (reconcile.Result, error) {
	if err := c.kubeClient.Get(ctx, req.NamespacedName, o); err != nil {
		if errors.IsNotFound(err) {
			c.deletePartialMatchGaugeMetric(c.ConditionCount, ConditionCount, map[string]string{
				MetricLabelNamespace: req.Namespace,
				MetricLabelName:      req.Name,
			})
			c.deletePartialMatchGaugeMetric(c.ConditionCurrentStatusSeconds, ConditionCurrentStatusSeconds, map[string]string{
				MetricLabelNamespace: req.Namespace,
				MetricLabelName:      req.Name,
			})
			c.deletePartialMatchGaugeMetric(c.TerminationCurrentTimeSeconds, TerminationCurrentTimeSeconds, map[string]string{
				MetricLabelNamespace: req.Namespace,
				MetricLabelName:      req.Name,
			})
			if obj, ok := c.terminatingObjects.LoadAndDelete(req); ok {
				c.observeHistogram(c.TerminationDuration, TerminationDuration, time.Since(obj.(Object).GetDeletionTimestamp().Time).Seconds(), map[string]string{}, c.toAdditionalMetricLabels(obj.(Object)))
			}
			if finalizers, ok := c.observedFinalizers.LoadAndDelete(req); ok {
				for _, finalizer := range finalizers.([]string) {
					c.eventRecorder.Event(o, v1.EventTypeNormal, "Finalized", fmt.Sprintf("Finalized %s", finalizer))
				}
			}
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("getting object, %w", err)
	}

	// Detect and record terminations
	observedFinalizers, _ := c.observedFinalizers.Swap(req, o.GetFinalizers())
	if observedFinalizers != nil {
		for _, finalizer := range lo.Without(observedFinalizers.([]string), o.GetFinalizers()...) {
			c.eventRecorder.Event(o, v1.EventTypeNormal, "Finalized", fmt.Sprintf("Finalized %s", finalizer))
		}
	}

	if o.GetDeletionTimestamp() != nil {
		c.setGaugeMetric(c.TerminationCurrentTimeSeconds, TerminationCurrentTimeSeconds, time.Since(o.GetDeletionTimestamp().Time).Seconds(), map[string]string{
			MetricLabelNamespace: req.Namespace,
			MetricLabelName:      req.Name,
		}, c.toAdditionalMetricLabels(o))
		c.terminatingObjects.Store(req, o)
	}

	// Detect and record condition counts
	currentConditions := o.StatusConditions()
	observedConditions := ConditionSet{}
	if v, ok := c.observedConditions.Load(req); ok {
		observedConditions = v.(ConditionSet)
	}
	c.observedConditions.Store(req, currentConditions)

	for _, condition := range o.GetConditions() {
		c.setGaugeMetric(c.ConditionCount, ConditionCount, 1, map[string]string{
			MetricLabelNamespace:       req.Namespace,
			MetricLabelName:            req.Name,
			pmetrics.LabelType:         condition.Type,
			MetricLabelConditionStatus: string(condition.Status),
			pmetrics.LabelReason:       condition.Reason,
		}, c.toAdditionalMetricLabels(o))
		c.setGaugeMetric(c.ConditionCurrentStatusSeconds, ConditionCurrentStatusSeconds, time.Since(condition.LastTransitionTime.Time).Seconds(), map[string]string{
			MetricLabelNamespace:       req.Namespace,
			MetricLabelName:            req.Name,
			pmetrics.LabelType:         condition.Type,
			MetricLabelConditionStatus: string(condition.Status),
			pmetrics.LabelReason:       condition.Reason,
		}, c.toAdditionalMetricLabels(o))
	}

	for _, observedCondition := range observedConditions.List() {
		if currentCondition := currentConditions.Get(observedCondition.Type); currentCondition == nil || currentCondition.Status != observedCondition.Status || currentCondition.Reason != observedCondition.Reason {
			c.deletePartialMatchGaugeMetric(c.ConditionCount, ConditionCount, map[string]string{
				MetricLabelNamespace:       req.Namespace,
				MetricLabelName:            req.Name,
				pmetrics.LabelType:         observedCondition.Type,
				MetricLabelConditionStatus: string(observedCondition.Status),
				pmetrics.LabelReason:       observedCondition.Reason,
			})
			c.deletePartialMatchGaugeMetric(c.ConditionCurrentStatusSeconds, ConditionCurrentStatusSeconds, map[string]string{
				MetricLabelNamespace:       req.Namespace,
				MetricLabelName:            req.Name,
				pmetrics.LabelType:         observedCondition.Type,
				MetricLabelConditionStatus: string(observedCondition.Status),
				pmetrics.LabelReason:       observedCondition.Reason,
			})
		}
	}

	// Detect and record status transitions. This approach is best effort,
	// since we may batch multiple writes within a single reconcile loop.
	// It's exceedingly difficult to atomically track all changes to an
	// object, since the Kubernetes is evenutally consistent by design.
	// Despite this, we can catch the majority of transition by remembering
	// what we saw last, and reporting observed changes.
	//
	// We rejected the alternative of tracking these changes within the
	// condition library itself, since you cannot guarantee that a
	// transition made in memory was successfully persisted.
	//
	// Automatic monitoring systems must assume that these observations are
	// lossy, specifically for when a condition transition rapidly. However,
	// for the common case, we want to alert when a transition took a long
	// time, and our likelyhood of observing this is much higher.
	for _, condition := range currentConditions.List() {
		observedCondition := observedConditions.Get(condition.Type)
		if observedCondition.GetStatus() == condition.GetStatus() {
			continue
		}
		// A condition transitions if it either didn't exist before or it has changed
		c.incCounterMetric(c.ConditionTransitionsTotal, ConditionTransitionsTotal, map[string]string{
			pmetrics.LabelType:         condition.Type,
			MetricLabelConditionStatus: string(condition.Status),
			pmetrics.LabelReason:       condition.Reason,
		}, c.toAdditionalMetricLabels(o))
		if observedCondition == nil {
			continue
		}
		duration := condition.LastTransitionTime.Time.Sub(observedCondition.LastTransitionTime.Time).Seconds()
		c.observeHistogram(c.ConditionDuration, ConditionDuration, duration, map[string]string{
			pmetrics.LabelType:         observedCondition.Type,
			MetricLabelConditionStatus: string(observedCondition.Status),
		}, c.toAdditionalMetricLabels(o))
		c.eventRecorder.Event(o, v1.EventTypeNormal, condition.Type, fmt.Sprintf("Status condition transitioned, Type: %s, Status: %s -> %s, Reason: %s%s",
			condition.Type,
			observedCondition.Status,
			condition.Status,
			condition.Reason,
			lo.Ternary(condition.Message != "", fmt.Sprintf(", Message: %s", condition.Message), ""),
		))
	}
	return reconcile.Result{RequeueAfter: time.Second * 10}, nil
}

func (c *Controller[T]) incCounterMetric(current pmetrics.CounterMetric, deprecated pmetrics.CounterMetric, labels, additionalLabels map[string]string) {
	current.Inc(lo.Assign(labels, additionalLabels))
	if c.emitDeprecatedMetrics {
		labels[pmetrics.LabelKind] = c.gvk.Kind
		labels[pmetrics.LabelGroup] = c.gvk.Group
		deprecated.Inc(labels)
	}
}

func (c *Controller[T]) setGaugeMetric(current pmetrics.GaugeMetric, deprecated pmetrics.GaugeMetric, value float64, labels, additionalLabels map[string]string) {
	current.Set(value, lo.Assign(labels, additionalLabels))
	if c.emitDeprecatedMetrics {
		labels[pmetrics.LabelKind] = c.gvk.Kind
		labels[pmetrics.LabelGroup] = c.gvk.Group
		deprecated.Set(value, labels)
	}
}

func (c *Controller[T]) deletePartialMatchGaugeMetric(current pmetrics.GaugeMetric, deprecated pmetrics.GaugeMetric, labels map[string]string) {
	current.DeletePartialMatch(labels)
	if c.emitDeprecatedMetrics {
		labels[pmetrics.LabelKind] = c.gvk.Kind
		labels[pmetrics.LabelGroup] = c.gvk.Group
		deprecated.DeletePartialMatch(labels)
	}
}

func (c *Controller[T]) observeHistogram(current pmetrics.ObservationMetric, deprecated pmetrics.ObservationMetric, value float64, labels, additionalLabels map[string]string) {
	current.Observe(value, lo.Assign(labels, additionalLabels))
	if c.emitDeprecatedMetrics {
		labels[pmetrics.LabelKind] = c.gvk.Kind
		labels[pmetrics.LabelGroup] = c.gvk.Group
		deprecated.Observe(value, labels)
	}
}
