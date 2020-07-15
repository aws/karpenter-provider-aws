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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ScalePolicySpec defines the desired state of ScalePolicy
type ScalePolicySpec struct {
	UtilizationMetric *UtilizationMetric `json:"utilizationPolicy,omitempty"`
	SQSQueueMetric    *SQSQueueMetric    `json:"sqsQueuePolicy,omitempty"`

	NodeLabelSelector map[string]string `json:"selector,omitempty"`
	MinReplicas       int32
	MaxReplicas       int32
}

// UtilizationMetric defines a thermostat-like scaler for CPU, Memory, or Custom Resource types (i.e. GPU)
type UtilizationMetric struct {
	// TODO, make this a map of resource type -> threshold a.la resource requests
	CPUThreshhold    int32 `json:"cpuThreshhold"`
	MemoryThreshhold int32 `json:"memoryThreshhold"`
	PodThreshhold    int32 `json:"podThreshhold"`
}

// SQSQueueMetric defines a scale policy that reacts to the length of a queue and preemptively scales out
type SQSQueueMetric struct {
	MessagesPerNode int32 `json:"messagesPerNode"`
}

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

func (r *ScalePolicy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}
