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

package v1alpha1

import (
	v2beta2 "k8s.io/api/autoscaling/v2beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ScalePolicySpec is modeled after https://godoc.org/k8s.io/api/autoscaling/v2beta2#HorizontalPodAutoscalerSpec
// This enables parity of functionality between Pod and Node autoscaling, with a few minor differences.
// 1. ObjectSelector is replaced by NodeSelector
// 2. Metrics.PodsMetricSelector is replaced by NodeMetricsSelector
type ScalePolicySpec struct {
	// NodeLabelSelector identifies Nodes, which in turn identify NodeGroups controlled by this scale policy.
	// NodeGroup and Provider are identified from node.providerId and node.metadata.labels["NGName"].
	NodeLabelSelector map[string]string `json:"selector"`
	// MinReplicas is the lower limit for the number of replicas to which the autoscaler
	// can scale down.  It defaults to 1 pod.  minReplicas is allowed to be 0 if the
	// alpha feature gate HPAScaleToZero is enabled and at least one Object or External
	// metric is configured.  Scaling is active as long as at least one metric value is
	// available.
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	// MaxReplicas is the upper limit for the number of replicas to which the autoscaler can scale up.
	// It cannot be less that minReplicas.
	MaxReplicas int32 `json:"maxReplicas"`
	// Metrics contains the specifications for which to use to calculate the
	// desired replica count (the maximum replica count across all metrics will
	// be used).  The desired replica count is calculated multiplying the
	// ratio between the target value and the current value by the current
	// number of pods.  Ergo, metrics used must decrease as the pod count is
	// increased, and vice-versa.  See the individual metric source types for
	// more information about how each type of metric must respond.
	// If not set, the default metric will be set to 80% average CPU utilization.
	// +optional
	Metrics []Metrics `json:"metrics,omitempty"`
	// Behavior configures the scaling behavior of the target
	// in both Up and Down directions (scaleUp and scaleDown fields respectively).
	// If not set, the default HPAScalingRules for scale up and scale down are used.
	// +optional
	Behavior v2beta2.HorizontalPodAutoscalerBehavior `json:"behavior,omitempty"`
}

// Metrics is modeled after https://godoc.org/k8s.io/api/autoscaling/v2beta2#MetricSpec
// +optional
type Metrics struct {
	// type is the type of metric source.  It should be one of "Object",
	// "Nodes" or "Resource", each mapping to a matching field in the object.
	Type MetricSourceType `json:"type" protobuf:"bytes,1,name=type"`

	// Nodes refers to a metric describing each pod in the current scale target
	// (for example, transactions-processed-per-second).  The values will be
	// averaged together before being compared to the target value.
	// +optional
	Nodes *NodesMetricSource `json:"nodes,omitempty"`

	// resource refers to a resource metric (such as those specified in
	// requests and limits) known to Kubernetes describing each pod in the
	// current scale target (e.g. CPU or memory). Such metrics are built in to
	// Kubernetes, and have special scaling options on top of those available
	// to normal per-pod metrics using the "pods" source.
	// +optional
	Resource *v2beta2.ResourceMetricSource `json:"resource,omitempty"`

	// Object refers to a metric describing a single kubernetes object
	// (for example, hits-per-second on an Ingress object).
	// +optional
	Object *v2beta2.ObjectMetricSource `json:"object,omitempty"`

	// External refers to a global metric that is not associated
	// with any Kubernetes object. It allows autoscaling based on information
	// coming from components running outside of cluster
	// (for example length of queue in cloud messaging service, or
	// QPS from loadbalancer running outside of cluster).
	// +optional
	External *v2beta2.ExternalMetricSource `json:"external,omitempty"`

	// Utilization       *Utilization      `json:"utilization,omitempty"`
	// QueueMetricsSource *QueueMetricsSource `json:"queue,omitempty"`
}

// MetricSourceType indicates the type of metric.
type MetricSourceType string

const (
	// ObjectMetricSourceType is a metric describing a kubernetes object
	// (for example, hits-per-second on an Ingress object).
	ObjectMetricSourceType MetricSourceType = "Object"
	// NodesMetricSourceType is a metric describing each node in the current scale
	// target (for example, transactions-processed-per-second).  The values
	// will be averaged together before being compared to the target value.
	NodesMetricSourceType MetricSourceType = "Nodes"
	// ResourceMetricSourceType is a resource metric known to Kubernetes, as
	// specified in requests and limits, describing each pod in the current
	// scale target (e.g. CPU or memory).  Such metrics are built in to
	// Kubernetes, and have special scaling options on top of those available
	// to normal per-pod metrics (the "pods" source).
	ResourceMetricSourceType MetricSourceType = "Resource"
	// ExternalMetricSourceType is a global metric that is not associated
	// with any Kubernetes object. It allows autoscaling based on information
	// coming from components running outside of cluster
	// (for example length of queue in cloud messaging service, or
	// QPS from loadbalancer running outside of cluster).
	ExternalMetricSourceType MetricSourceType = "External"
)

//
type NodesMetricSource struct {
	// Metric identifies the target metric by name and selector
	Metric v2beta2.MetricIdentifier `json:"metric"`
	// Target specifies the target value for the given metric
	Target v2beta2.MetricTarget `json:"target"`
}

// // Utilization defines a thermostat-like scaler for CPU, Memory, or Custom Resource types (i.e. GPU)
// type Utilization map[corev1.ResourceName]string

// // Queue defines a scale policy that reacts to the length of a queue and preemptively scales out
// type Queue struct {
// 	// MessagesPerNode determines how quickly to scale out for a queue.
// 	MessagesPerNode int32 `json:"messagesPerNode"`
// }

// ScalePolicy is the Schema for the scalepolicies API
// +kubebuilder:object:root=true
type ScalePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScalePolicySpec   `json:"spec,omitempty"`
	Status ScalePolicyStatus `json:"status,omitempty"`
}

// ScalePolicyList contains a list of ScalePolicy
// +kubebuilder:object:root=true
type ScalePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScalePolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ScalePolicy{}, &ScalePolicyList{})
}

// log is for logging in this package.
var scalepolicylog = logf.Log.WithName("scalepolicy-resource")

func (r *ScalePolicy) SetupWebhookWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}
