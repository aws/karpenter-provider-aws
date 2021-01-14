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
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

// MetricsProducerStatus defines the observed state of the resource.
// +kubebuilder:subresource:status
type MetricsProducerStatus struct {
	// +optional
	PendingCapacity *PendingCapacityStatus `json:"pendingCapacity,omitempty"`
	// +optional
	Queue *QueueStatus `json:"queue,omitempty"`
	// +optional
	ReservedCapacity map[v1.ResourceName]string `json:"reservedCapacity,omitempty"`
	// +optional
	ScheduledCapacity *ScheduledCapacityStatus `json:"scheduledCapacity,omitempty"`
	// Conditions is the set of conditions required for the metrics producer to
	// successfully publish metrics to the metrics server
	// +optional
	Conditions apis.Conditions `json:"conditions,omitempty"`
}

type PendingCapacityStatus struct {
}

type QueueStatus struct {
	// Length of the Queue
	Length int64 `json:"length"`

	// The age of the oldest message in the queue in seconds
	OldestMessageAgeSeconds int64 `json:"oldestMessageAgeSeconds,omitempty"`
}

type ScheduledCapacityStatus struct {
	// The current recommendation - the metric the MetricsProducer is emitting
	CurrentValue *int32 `json:"currentValue,omitempty"`

	// The time in the future where CurrentValue will switch to NextValue
	NextValueTime *apis.VolatileTime `json:"nextValueTime,omitempty"`

	// The next recommendation for the metric
	NextValue *int32 `json:"nextValue,omitempty"`
}

// We use knative's libraries for ConditionSets to manage status conditions.
// Conditions are all of "true-happy" polarity. If any condition is false, the resource's "happiness" is false.
// https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-conditions
// https://github.com/knative/serving/blob/f1582404be275d6eaaf89ccd908fb44aef9e48b5/vendor/knative.dev/pkg/apis/condition_set.go
func (m *MetricsProducer) StatusConditions() apis.ConditionManager {
	return apis.NewLivingConditionSet(
		Active,
	).Manage(m)
}

func (m *MetricsProducer) GetConditions() apis.Conditions {
	return m.Status.Conditions
}

func (m *MetricsProducer) SetConditions(conditions apis.Conditions) {
	m.Status.Conditions = conditions
}
