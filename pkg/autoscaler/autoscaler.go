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

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/autoscaler/algorithms"
	"github.com/ellistarn/karpenter/pkg/metrics/clients"
	"github.com/pkg/errors"
	v1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/scale"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Factory instantiates autoscalers
type Factory struct {
	MetricsClientFactory clients.Factory
	KubernetesClient     client.Client
	Mapper               meta.RESTMapper
	ScaleNamespacer      scale.ScalesGetter
}

// For returns an autoscaler for the resource
func (af *Factory) For(resource *v1alpha1.HorizontalAutoscaler) Autoscaler {
	return Autoscaler{
		HorizontalAutoscaler: resource,
		algorithm:            algorithms.For(resource.Spec),
		metricsClientFactory: af.MetricsClientFactory,
		kubernetesClient:     af.KubernetesClient,
		mapper:               af.Mapper,
		scaleNamespacer:      af.ScaleNamespacer,
	}
}

// Autoscaler calculates desired replicas using the provided algorithm.
type Autoscaler struct {
	*v1alpha1.HorizontalAutoscaler
	metricsClientFactory clients.Factory
	kubernetesClient     client.Client
	algorithm            algorithms.Algorithm
	mapper               meta.RESTMapper
	scaleNamespacer      scale.ScalesGetter
}

// Reconcile executes an autoscaling loop
func (a *Autoscaler) Reconcile() error {
	// 1. Retrieve current metrics for the autoscaler
	metrics, err := a.getMetrics()
	if err != nil {
		return errors.Wrap(err, "getting metrics")
	}

	// 2. Retrieve current number of replicas
	scaleTarget, err := a.getScaleTarget()
	if err != nil {
		return errors.Wrap(err, "getting scale target")
	}
	a.Status.CurrentReplicas = scaleTarget.Spec.Replicas

	// 3. Calculate desired replicas
	scaleTarget.Spec.Replicas = a.getDesiredReplicas(metrics, scaleTarget.Spec.Replicas)

	// 4. Persist updated scale to server
	if err := a.updateScaleTarget(scaleTarget); err != nil {
		return errors.Wrap(err, "setting replicas")
	}

	a.Status.DesiredReplicas = scaleTarget.Spec.Replicas
	return nil
}

func (a *Autoscaler) getMetrics() ([]algorithms.Metric, error) {
	metrics := []algorithms.Metric{}
	for _, metric := range a.Spec.Metrics {
		observed, err := a.metricsClientFactory.For(metric).GetCurrentValue(metric)
		if err != nil {
			return nil, errors.Wrapf(err, "reading metric %v", metric)
		}
		metrics = append(metrics, algorithms.Metric{
			Metric:      observed,
			TargetType:  metric.GetTarget().Type,
			TargetValue: float64(metric.GetTarget().Value.Value()),
		})
	}
	return metrics, nil
}

func (a *Autoscaler) getDesiredReplicas(metrics []algorithms.Metric, replicas int32) int32 {
	recommendations := []int32{}
	for _, metric := range metrics {
		recommendations = append(recommendations, a.algorithm.GetDesiredReplicas(metric, replicas))
	}
	// TODO apply Spec.Behaviors to this policy
	return recommendations[0]
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
