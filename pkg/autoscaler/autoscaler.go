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
	"go.uber.org/zap"
	v1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/scale"
	"knative.dev/pkg/apis"
)

const (
	ScaleBoundedFormat       = "recommendation %d limited by bounds [%d, %d]"
	ScaleStabilizationFormat = "within stabilization window %d/%d seconds"
)

func NewFactoryOrDie(metricsclientfactory *clients.Factory, mapper meta.RESTMapper, config *rest.Config) *Factory {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	log.PanicIfError(err, "Failed to create discovery client")
	scalesgetter, err := scale.NewForConfig(config, mapper, dynamic.LegacyAPIPathResolverFunc, scale.NewDiscoveryScaleKindResolver(discoveryClient))
	log.PanicIfError(err, "Failed to create scale client")
	return &Factory{
		MetricsClientFactory: metricsclientfactory,
		Mapper:               mapper,
		ScalesGetter:         scalesgetter,
	}
}

// Factory instantiates autoscalers
type Factory struct {
	MetricsClientFactory *clients.Factory
	Mapper               meta.RESTMapper
	ScalesGetter         scale.ScalesGetter
}

// For returns an autoscaler for the resource
func (f *Factory) For(resource *v1alpha1.HorizontalAutoscaler) Autoscaler {
	return Autoscaler{
		HorizontalAutoscaler: resource,
		algorithm:            algorithms.For(resource.Spec),
		metricsClientFactory: f.MetricsClientFactory,
		mapper:               f.Mapper,
		scalesGetter:         f.ScalesGetter,
	}
}

// Autoscaler calculates desired replicas using the provided algorithm.
type Autoscaler struct {
	*v1alpha1.HorizontalAutoscaler
	metricsClientFactory *clients.Factory
	algorithm            algorithms.Algorithm
	mapper               meta.RESTMapper
	scalesGetter         scale.ScalesGetter
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
	desiredReplicas := a.getDesiredReplicas(metrics, scaleTarget.Spec.Replicas)
	if desiredReplicas == scaleTarget.Spec.Replicas {
		return nil
	}

	// 4. Persist updated scale to server
	scaleTarget.Spec.Replicas = desiredReplicas
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
			return nil, fmt.Errorf("failed retrieving metric, %w", err)
		}
		metrics = append(metrics, algorithms.Metric{
			Metric:      observed,
			TargetType:  metric.GetTarget().Type,
			TargetValue: float64(metric.GetTarget().Value.Value()),
		})
	}
	return metrics, nil
}

/* getDesiredReplicas returns the desired scale value and sets limit conditions.

Status conditions are always set, regardless of the outcome of the policy
decisions. The conditions will only be set if the autoscaler is attempting to
scale and prevented by the limits. e.g. if at max but not recommended to scale
up, the ScalingUnbounded condition will continue to be true.

They are also orthogonal, such that {ScalingUnbounded, AbleToScale} can be
{true, true}: no limits, desired replicas is set directly to the recommendation,
{true, false}: outside of stabilization window or policy but limited by min/max,
{false, true}: limited by min/max but not stabilization window or policy,
{false, false}: limited stabilization window or policy and also by min/max.
*/
func (a *Autoscaler) getDesiredReplicas(metrics []algorithms.Metric, replicas int32) int32 {
	var recommendations []int32
	for _, metric := range metrics {
		recommendations = append(recommendations, a.algorithm.GetDesiredReplicas(metric, replicas))
	}

	recommended := a.Spec.Behavior.ApplySelectPolicy(recommendations, replicas)
	limited := a.applyTransientLimits(recommended, replicas)
	bounded := a.applyBoundedLimits(limited)

	return bounded
}

func (a *Autoscaler) applyBoundedLimits(desiredReplicas int32) int32 {
	boundedReplicas := desiredReplicas
	boundedReplicas = f.MinInt32([]int32{boundedReplicas, a.Spec.MaxReplicas})
	boundedReplicas = f.MaxInt32([]int32{boundedReplicas, a.Spec.MinReplicas})
	if boundedReplicas != desiredReplicas {
		a.StatusConditions().MarkFalse(v1alpha1.ScalingUnbounded, "", fmt.Sprintf(ScaleBoundedFormat, desiredReplicas, a.Spec.MinReplicas, a.Spec.MaxReplicas))
	} else {
		a.StatusConditions().MarkTrue(v1alpha1.ScalingUnbounded)
	}
	return boundedReplicas
}

func (a *Autoscaler) applyTransientLimits(recommendation int32, replicas int32) int32 {
	rules := a.Spec.Behavior.GetScalingRules(recommendation, replicas)
	// 1. Don't scale if within stabilization window. Check after determining
	// scale up vs down, as scale up window doesn't prevent scale down.
	if a.Status.LastScaleTime != nil {
		if elapsed, window := time.Now().Second()-a.Status.LastScaleTime.Inner.Second(), int(*rules.StabilizationWindowSeconds); elapsed < window {
			a.StatusConditions().MarkFalse(v1alpha1.AbleToScale, "", fmt.Sprintf(ScaleStabilizationFormat, elapsed, window))
			return recommendation
		}
	}

	// 2. TODO Check if limited by Policies
	for _, policy := range rules.Policies {
		zap.S().Info("TODO: check policy %s", policy)
	}

	// 3. If not limited, use raw recommended value
	a.StatusConditions().MarkTrue(v1alpha1.AbleToScale)
	return recommendation
}

func (a *Autoscaler) getScaleTarget() (*v1.Scale, error) {
	groupResource, err := a.parseGroupResource(a.Spec.ScaleTargetRef)
	if err != nil {
		return nil, fmt.Errorf("parsing group resource for %v, %w", a.Spec.ScaleTargetRef, err)
	}
	scaleTarget, err := a.scalesGetter.
		Scales(a.ObjectMeta.Namespace).
		Get(context.TODO(), groupResource, a.Spec.ScaleTargetRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting scale target for %v, %w", a.Spec.ScaleTargetRef, err)
	}
	return scaleTarget, nil
}

func (a *Autoscaler) updateScaleTarget(scaleTarget *v1.Scale) error {
	groupResource, err := a.parseGroupResource(a.Spec.ScaleTargetRef)
	if err != nil {
		return fmt.Errorf("parsing group resource for %v, %w", a.Spec.ScaleTargetRef, err)
	}
	if _, err := a.scalesGetter.
		Scales(a.ObjectMeta.Namespace).
		Update(context.TODO(), groupResource, scaleTarget, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating %v, %w", scaleTarget.ObjectMeta.SelfLink, err)
	}
	return nil
}

func (a *Autoscaler) parseGroupResource(scaleTargetRef v1alpha1.CrossVersionObjectReference) (schema.GroupResource, error) {
	groupVersion, err := schema.ParseGroupVersion(scaleTargetRef.APIVersion)
	if err != nil {
		return schema.GroupResource{}, fmt.Errorf("parsing groupversion from APIVersion %s, %w", scaleTargetRef.APIVersion, err)
	}
	groupKind := schema.GroupKind{
		Group: groupVersion.Group,
		Kind:  scaleTargetRef.Kind,
	}
	mapping, err := a.mapper.RESTMapping(groupKind, groupVersion.Version)
	if err != nil {
		return schema.GroupResource{}, fmt.Errorf("getting RESTMapping for %v %v, %w", groupKind, groupVersion.Version, err)
	}
	return mapping.Resource.GroupResource(), nil
}
