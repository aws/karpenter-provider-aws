/*
Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// HorizontalAutoscalerStatus defines the observed state of HorizontalAutoscaler
type HorizontalAutoscalerStatus struct {
	// LastScaleTime is the last time the HorizontalAutoscaler scaled the number
	// of pods, used by the autoscaler to control how often the number of pods
	// is changed. +optional
	LastScaleTime *metav1.Time `json:"lastScaleTime,omitempty"`
	// CurrentReplicas is current number of replicas of pods managed by this
	// autoscaler, as last seen by the autoscaler.
	CurrentReplicas int32 `json:"currentReplicas"`
	// DesiredReplicas is the desired number of replicas of pods managed by this
	// autoscaler, as last calculated by the autoscaler.
	DesiredReplicas int32 `json:"desiredReplicas"`
	// CurrentMetrics is the last read state of the metrics used by this
	// autoscaler. +optional
	CurrentMetrics []MetricStatus `json:"currentMetrics,omitempty"`
	// Conditions is the set of conditions required for this autoscaler to scale
	// its target, and indicates whether or not those conditions are met.
	Conditions apis.Conditions `json:"conditions,omitempty"`
}

const (
	// ScalingActive indicates that the controller is able to scale if
	// necessary: it's correctly configured, can fetch the desired metrics, and
	// isn't disabled.
	ScalingActive apis.ConditionType = "ScalingActive"
	// AbleToScale indicates a lack of transient issues which prevent scaling
	// from occurring, such as being in a backoff window, or being unable to
	// access/update the target scale.
	AbleToScale apis.ConditionType = "AbleToScale"
	// ScalingUnlimited indicates that the calculated scale based on metrics would
	// be above or below the range for the HA, and has thus been capped.
	ScalingUnlimited apis.ConditionType = "ScalingUnlimited"
)

// MetricStatus contains status information for the configured metrics source.
// This status has a one-of semantic and will only ever contain one value.
type MetricStatus struct {
	// +optional
	Object *PrometheusMetricStatus `json:"prometheus,omitempty"`
}

type PrometheusMetricStatus struct {
	// Query of the metric
	Query string `json:"query"`
	// Current contains the current value for the given metric
	Current MetricValueStatus `json:"current"`
}

type MetricValueStatus struct {
	// Value is the current value of the metric (as a quantity).
	// +optional
	Value *resource.Quantity `json:"value,omitempty"`
	// AverageValue is the current value of the average of the metric across all
	// relevant pods (as a quantity)
	// +optional
	AverageValue *resource.Quantity `json:"averageValue,omitempty"`
	// currentAverageUtilization is the current value of the average of the
	// resource metric across all relevant pods, represented as a percentage of
	// the requested value of the resource for the pods.
	// +optional
	AverageUtilization *int32 `json:"averageUtilization,omitempty"`
}

func (s *HorizontalAutoscalerStatus) MarkScalingActive() {
	s.SetCondition(ScalingActive, &apis.Condition{
		Type:     ScalingActive,
		Status:   v1.ConditionTrue,
		Severity: apis.ConditionSeverityInfo,
	})
}

func (s *HorizontalAutoscalerStatus) MarkNotScalingActive(message string) {
	s.SetCondition(ScalingActive, &apis.Condition{
		Type:     ScalingActive,
		Status:   v1.ConditionFalse,
		Severity: apis.ConditionSeverityError,
		Message:  message,
	})
}

func (s *HorizontalAutoscalerStatus) MarkAbleToScale() {
	s.SetCondition(AbleToScale, &apis.Condition{
		Type:     AbleToScale,
		Status:   v1.ConditionTrue,
		Severity: apis.ConditionSeverityInfo,
	})
}

func (s *HorizontalAutoscalerStatus) MarkNotAbleToScale(message string) {
	s.SetCondition(AbleToScale, &apis.Condition{
		Type:     AbleToScale,
		Status:   v1.ConditionFalse,
		Severity: apis.ConditionSeverityWarning,
		Message:  message,
	})
}

func (s *HorizontalAutoscalerStatus) MarkScalingUnlimited() {
	s.SetCondition(ScalingUnlimited, &apis.Condition{
		Type:     ScalingUnlimited,
		Status:   v1.ConditionTrue,
		Severity: apis.ConditionSeverityInfo,
	})
}

func (s *HorizontalAutoscalerStatus) MarkScalingNotUnlimited(message string) {
	s.SetCondition(ScalingUnlimited, &apis.Condition{
		Type:     ScalingUnlimited,
		Status:   v1.ConditionFalse,
		Severity: apis.ConditionSeverityInfo,
		Message:  message,
	})
}

// We use knative's libraries for ConditionSets to manage status conditions.
// Conditions are all of "true-happy" polarity. If any condition is false, the resource's "happiness" is false.
// https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-conditions
// https://github.com/knative/serving/blob/f1582404be275d6eaaf89ccd908fb44aef9e48b5/vendor/knative.dev/pkg/apis/condition_set.go
var conditionSet = apis.NewLivingConditionSet(
	ScalingActive,
	AbleToScale,
	ScalingUnlimited,
)

func (s *HorizontalAutoscalerStatus) GetConditions() apis.Conditions {
	return s.Conditions
}

func (s *HorizontalAutoscalerStatus) SetConditions(conditions apis.Conditions) {
	s.Conditions = conditions
}

func (s *HorizontalAutoscalerStatus) SetCondition(conditionType apis.ConditionType, condition *apis.Condition) {
	switch {
	case condition == nil:
	case condition.Status == v1.ConditionUnknown:
		conditionSet.Manage(s).MarkUnknown(conditionType, condition.Reason, condition.Message)
	case condition.Status == v1.ConditionTrue:
		conditionSet.Manage(s).MarkTrue(conditionType)
	case condition.Status == v1.ConditionFalse:
		conditionSet.Manage(s).MarkFalse(conditionType, condition.Reason, condition.Message)
	}
}
