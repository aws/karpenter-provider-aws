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
// Package v1alpha1 holds definitions for HorizontalAutoscaler
// +kubebuilder:object:generate=true
// +groupName=karpenter.sh
package v1alpha1

import (
	"github.com/ellistarn/karpenter/pkg/apis"
	v2beta2 "k8s.io/api/autoscaling/v2beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HorizontalAutoscalerSpec is modeled after https://godoc.org/k8s.io/api/autoscaling/v2beta2#HorizontalPodAutoscalerSpec
// This enables parity of functionality between Pod and Node autoscaling, with a few minor differences.
// 1. ObjectSelector is replaced by NodeSelector.
// 2. Metrics.PodsMetricSelector is replaced by the more generic Metrics.ReplicaMetricSelector.
type HorizontalAutoscalerSpec struct {
	// ScaleTargetRef points to the target resource to scale.
	ScaleTargetRef v2beta2.CrossVersionObjectReference `json:"scaleTargetRef"`

	// MinReplicas is the lower limit for the number of replicas to which the autoscaler
	// can scale down. MinReplicas is allowed to be 0 if the
	MinReplicas int32 `json:"minReplicas"`
	// MaxReplicas is the upper limit for the number of replicas to which the autoscaler can scale up.
	// It cannot be less that minReplicas.
	MaxReplicas int32 `json:"maxReplicas"`
	// Metrics contains the specifications for which to use to calculate the
	// desired replica count (the maximum replica count across all metrics will
	// be used).  The desired replica count is calculated multiplying the
	// ratio between the target value and the current value by the current
	// number of replicas.  Ergo, metrics used must decrease as the replica count is
	// increased, and vice-versa.  See the individual metric source types for
	// more information about how each type of metric must respond.
	// If not set, the default metric will be set to 80% average CPU utilization.
	// +optional
	Metrics []Metrics `json:"metrics,omitempty"`
	// Behavior configures the scaling behavior of the target
	// in both Up and Down directions (scaleUp and scaleDown fields respectively).
	// If not set, the default ScalingRules for scale up and scale down are used.
	// +optional
	Behavior HorizontalAutoscalerBehavior `json:"behavior,omitempty"`
}

// HorizontalAutoscalerBehavior configures the scaling behavior of the target
// in both Up and Down directions (scaleUp and scaleDown fields respectively).
type HorizontalAutoscalerBehavior struct {
	// ScaleUp is scaling policy for scaling Up.
	// If not set, the default value is the higher of:
	//   * increase no more than 4 replicas per 60 seconds
	//   * double the number of replicas per 60 seconds
	// No stabilization is used.
	// +optional
	ScaleUp *ScalingRules `json:"scaleUp,omitempty"`
	// ScaleDown is scaling policy for scaling Down.
	// If not set, the default value is to allow to scale down to minReplicas, with a
	// 300 second stabilization window (i.e., the highest recommendation for
	// the last 300sec is used).
	// +optional
	ScaleDown *ScalingRules `json:"scaleDown,omitempty"`
}

// ScalingRules configures the scaling behavior for one direction.
// These Rules are applied after calculating DesiredReplicas from metrics for the HPA.
// They can limit the scaling velocity by specifying scaling policies.
// They can prevent flapping by specifying the stabilization window, so that the
// number of replicas is not set instantly, instead, the safest value from the stabilization
// window is chosen.
type ScalingRules struct {
	// StabilizationWindowSeconds is the number of seconds for which past recommendations should be
	// considered while scaling up or scaling down.
	// StabilizationWindowSeconds must be greater than or equal to zero and less than or equal to 3600 (one hour).
	// If not set, use the default values:
	// - For scale up: 0 (i.e. no stabilization is done).
	// - For scale down: 300 (i.e. the stabilization window is 300 seconds long).
	// +optional
	StabilizationWindowSeconds *int32 `json:"stabilizationWindowSeconds"`
	// selectPolicy is used to specify which policy should be used.
	// If not set, the default value MaxPolicySelect is used.
	// +optional
	SelectPolicy *v2beta2.ScalingPolicySelect `json:"selectPolicy,omitempty"`
	// policies is a list of potential scaling polices which can be used during scaling.
	// At least one policy must be specified, otherwise the ScalingRules will be discarded as invalid
	// +optional
	Policies []ScalingPolicy `json:"policies,omitempty"`
}

// ScalingPolicyType is the type of the policy which could be used while making scaling decisions.
type ScalingPolicyType string

const (
	// CountScalingPolicy is a policy used to specify a change in absolute number of replicas.
	CountScalingPolicy ScalingPolicyType = "Count"
	// PercentScalingPolicy is a policy used to specify a relative amount of change with respect to
	// the current number of replicas.
	PercentScalingPolicy ScalingPolicyType = "Percent"
)

// ScalingPolicy is a single policy which must hold true for a specified past interval.
type ScalingPolicy struct {
	// Type is used to specify the scaling policy.
	Type ScalingPolicyType `json:"type"`
	// Value contains the amount of change which is permitted by the policy.
	// It must be greater than zero
	Value int32 `json:"value"`
	// PeriodSeconds specifies the window of time for which the policy should hold true.
	// PeriodSeconds must be greater than zero and less than or equal to 1800 (30 min).
	PeriodSeconds int32 `json:"periodSeconds"`
}

// Metrics is modeled after https://godoc.org/k8s.io/api/autoscaling/v2beta2#MetricSpec
// +optional
type Metrics struct {
	// type is the type of metric source.  It should be one of "Object",
	// "Replicas" or "Resource", each mapping to a matching field in the object.
	Type MetricSourceType `json:"type"`
	// +optional
	Prometheus *PrometheusMetricSource `json:"prometheus,omitempty"`
}

// PrometheusMetricSource defines a metric in Prometheus
type PrometheusMetricSource struct {
	Query  string               `json:"query"`
	Target v2beta2.MetricTarget `json:"target"`
}

// MetricSourceType indicates the type of metric.
type MetricSourceType string

// MetricSourceType enum definition
const (
	PrometheusMetricSourceType MetricSourceType = "PrometheusMetricSourceType"
)

// HorizontalAutoscaler is the Schema for the horizontalautoscalers API
// +kubebuilder:object:root=true
type HorizontalAutoscaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HorizontalAutoscalerSpec   `json:"spec,omitempty"`
	Status HorizontalAutoscalerStatus `json:"status,omitempty"`
}

// HorizontalAutoscalerList contains a list of HorizontalAutoscaler
// +kubebuilder:object:root=true
type HorizontalAutoscalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HorizontalAutoscaler `json:"items"`
}

func init() {
	apis.SchemeBuilder.Register(&HorizontalAutoscaler{}, &HorizontalAutoscalerList{})
}
