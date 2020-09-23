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

package autoscaler

import (
	"context"
	"fmt"
	"time"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/autoscaler/algorithms"
	"github.com/ellistarn/karpenter/pkg/metrics/clients"
	f "github.com/ellistarn/karpenter/pkg/utils/functional"
	"github.com/ellistarn/karpenter/pkg/utils/log"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	v1 "k8s.io/api/autoscaling/v1"
	"k8s.io/api/autoscaling/v2beta2"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/scale"
	"knative.dev/pkg/apis"
)

// Factory instantiates autoscalers
type Factory struct {
	MetricsClientFactory clients.Factory
	Mapper               meta.RESTMapper
	ScaleNamespacer      scale.ScalesGetter
}

// For returns an autoscaler for the resource
func (af *Factory) For(resource *v1alpha1.HorizontalAutoscaler) Autoscaler {
	return Autoscaler{
		HorizontalAutoscaler: resource,
		algorithm:            algorithms.For(resource.Spec),
		metricsClientFactory: af.MetricsClientFactory,
		mapper:               af.Mapper,
		scaleNamespacer:      af.ScaleNamespacer,
	}
}

// Autoscaler calculates desired replicas using the provided algorithm.
type Autoscaler struct {
	*v1alpha1.HorizontalAutoscaler
	metricsClientFactory clients.Factory
	algorithm            algorithms.Algorithm
	mapper               meta.RESTMapper
	scaleNamespacer      scale.ScalesGetter
}

// Reconcile executes an autoscaling loop
func (a *Autoscaler) Reconcile() error {
	// 1. Retrieve current metrics for the autoscaler
	metrics, err := a.getMetrics()
	if err != nil {
		return err
	}

	// 2. Retrieve current number of replicas
	scaleTarget, err := a.getScaleTarget()
	if err != nil {
		return err
	}
	a.Status.CurrentReplicas = scaleTarget.Status.Replicas

	// 3. Calculate desired replicas using metrics and current desired replicas
	recommended := a.getRecommendation(metrics, scaleTarget.Spec.Replicas)
	if recommended == scaleTarget.Spec.Replicas {
		return nil
	}

	// 4. Persist updated scale to server
	scaleTarget.Spec.Replicas = recommended
	if err := a.updateScaleTarget(scaleTarget); err != nil {
		return err
	}

	a.Status.DesiredReplicas = scaleTarget.Spec.Replicas
	a.Status.LastScaleTime = &apis.VolatileTime{Inner: metav1.Now()}
	return nil
}

func (a *Autoscaler) getMetrics() ([]algorithms.Metric, error) {
	metrics := []algorithms.Metric{}
	for _, metric := range a.Spec.Metrics {
		observed, err := a.metricsClientFactory.For(metric).GetCurrentValue(metric)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, algorithms.Metric{
			Metric:      observed,
			TargetType:  metric.GetTarget().Type,
			TargetValue: float64(metric.GetTarget().Value.Value()),
		})
	}
	return metrics, nil
}

/* getRecommendation returns the desired scale value and sets limit conditions.

Status conditions are always set, regardless of the outcome of the policy
decisions. The conditions will only be set if the autoscaling is attempting to
scale and prevented by the limits. e.g. if at max but not recommended to scale
up, the ScalingUnbounded condition will continue to be true.

They are also orthogonal, such that {ScalingUnbounded, AbleToScale} can be
{true, true}: no limits, desired replicas is set directly to the recommendation,
{true, false}: outside of stabilization window or policy but limited by min/max,
{false, true}: limited by min/max but not stabilization window or policy,
{false, false}: limited stabilization window or policy and also by min/max.
*/
func (a *Autoscaler) getRecommendation(metrics []algorithms.Metric, replicas int32) int32 {
	var recommendations []int32
	var reason string

	// 1. Get recommendations
	for _, metric := range metrics {
		recommendations = append(recommendations, a.algorithm.GetDesiredReplicas(metric, replicas))
	}

	// 2. Check transient limits.
	if replicas, reason = a.getTransientlyLimitedValue(recommendations, replicas); reason == "" {
		a.MarkAbleToScale()
	} else {
		a.MarkNotAbleToScale(reason)
	}

	// 3. Check bounded limits.
	if replicas, reason = a.getBoundedValue(replicas); reason == "" {
		a.MarkScalingUnbounded()
	} else {
		a.MarkNotScalingUnbounded(reason)
	}

	// 4. Return recommendation
	return replicas

}

func (a *Autoscaler) getTransientlyLimitedValue(recommendations []int32, replicas int32) (int32, string) {
	var rules v1alpha1.ScalingRules
	// 1. Prioritize scale up over scale down. If no recommendations are
	// available, return an unchanged value is without any limiting reason.
	if gt := f.GreaterThanInt32(recommendations, replicas); len(gt) > 0 {
		recommendations, rules = gt, *a.Spec.Behavior.ScaleUp
	} else if lt := f.LessThanInt32(recommendations, replicas); len(lt) > 0 {
		recommendations, rules = lt, *a.Spec.Behavior.ScaleDown
	} else {
		return replicas, ""
	}

	// 2. Get the recommendation scale up/down configured above
	replicas = a.getSelectionFromRecommendations(recommendations, replicas, *rules.SelectPolicy)

	// 3. Don't scale if within stabilization window
	if a.Status.LastScaleTime != nil {
		if elapsed, window := time.Now().Second()-a.Status.LastScaleTime.Inner.Second(), int(*rules.StabilizationWindowSeconds); elapsed < window {
			return replicas, fmt.Sprintf("within stabilization window %d/%d seconds", elapsed, window)
		}
	}

	// 4. TODO Check if limited by Policies
	for _, policy := range rules.Policies {
		zap.S().Info("TODO: check policy %s", policy)
	}

	return replicas, ""
}

func (a *Autoscaler) getSelectionFromRecommendations(recommendations []int32, replicas int32, policy v2beta2.ScalingPolicySelect) int32 {
	switch policy {
	case v2beta2.MaxPolicySelect:
		return f.MaxInt32(recommendations)
	case v2beta2.MinPolicySelect:
		return f.MinInt32(recommendations)
	case v2beta2.DisabledPolicySelect:
		return replicas
	default:
		log.FatalInvariantViolated(fmt.Sprintf("unknown select policy: %s", policy))
		return replicas
	}
}

func (a *Autoscaler) getBoundedValue(recommendation int32) (int32, string) {
	if recommendation > a.Spec.MaxReplicas {
		return a.Spec.MaxReplicas, fmt.Sprintf("limited by maximum %d/%d replicas", recommendation, a.Spec.MaxReplicas)
	}
	if recommendation < a.Spec.MinReplicas {
		return a.Spec.MinReplicas, fmt.Sprintf("limited by minimum %d/%d replicas", recommendation, a.Spec.MinReplicas)
	}
	return recommendation, ""
}

func (a *Autoscaler) getScaleTarget() (*v1.Scale, error) {
	groupResource, err := a.parseGroupResource(a.Spec.ScaleTargetRef)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing group resource for %v", a.Spec.ScaleTargetRef)
	}
	scaleTarget, err := a.scaleNamespacer.
		Scales(a.ObjectMeta.Namespace).
		Get(context.TODO(), groupResource, a.Spec.ScaleTargetRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "getting scale target for %v", a.Spec.ScaleTargetRef)
	}
	return scaleTarget, nil
}

func (a *Autoscaler) updateScaleTarget(scaleTarget *v1.Scale) error {
	groupResource, err := a.parseGroupResource(a.Spec.ScaleTargetRef)
	if err != nil {
		return errors.Wrapf(err, "parsing group resource for %v", a.Spec.ScaleTargetRef)
	}
	if _, err := a.scaleNamespacer.
		Scales(a.ObjectMeta.Namespace).
		Update(context.TODO(), groupResource, scaleTarget, metav1.UpdateOptions{}); err != nil {
		return errors.Wrapf(err, "updating %v", scaleTarget.ObjectMeta.SelfLink)
	}
	return nil
}

func (a *Autoscaler) parseGroupResource(scaleTargetRef v1alpha1.CrossVersionObjectReference) (schema.GroupResource, error) {
	groupVersion, err := schema.ParseGroupVersion(scaleTargetRef.APIVersion)
	if err != nil {
		return schema.GroupResource{}, errors.Wrapf(err, "parsing groupversion from APIVersion %s", scaleTargetRef.APIVersion)
	}
	groupKind := schema.GroupKind{
		Group: groupVersion.Group,
		Kind:  scaleTargetRef.Kind,
	}
	mapping, err := a.mapper.RESTMapping(groupKind, groupVersion.Version)
	if err != nil {
		return schema.GroupResource{}, errors.Wrapf(err, "getting RESTMapping for %v %v", groupKind, groupVersion.Version)
	}
	return mapping.Resource.GroupResource(), nil
}
