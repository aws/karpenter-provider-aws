package v1alpha1

import (
	"k8s.io/api/autoscaling/v2beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MetricsProducerSpec defines an object that outputs metrics.
type MetricsProducerSpec struct {
	// +optional
	ScheduledCapacity *ScheduledCapacitySpec `json:"scheduledCapacity,omitempty"`
	// +optional
	PendingPods *PendingPodsSpec `json:"pendingPods,omitempty"`
	// +optional
	Queue *QueueSpec `json:"queue,omitempty"`
}

// ScheduledCapacitySpec outputs metrics on a schedule configured by a list of crontabs
type ScheduledCapacitySpec struct {
	// NodeGroup points to a resource that manages a group of nodes.
	NodeGroup v2beta2.CrossVersionObjectReference `json:"nodeGroup"`
	// Behaviors may be layered to achieve complex scheduling autoscaling logic
	Behaviors []ScheduledBehavior `json:"behaviors"`
}

// ScheduledBehavior defines a crontab which sets the metric to a specific replica value on a schedule.
type ScheduledBehavior struct {
	Crontab  string `json:"crontab"`
	Replicas int32  `json:"replicas"`
}

// PendingPodsSpec outputs a metric that identifies scheduling opportunities for pending pods in specified node groups.
// If multiple pending pods metrics producers exist, the algorithm will ensure that only a single node group scales up.
type PendingPodsSpec struct {
	// NodeGroup points to a resource that manages a group of nodes.
	NodeGroup v2beta2.CrossVersionObjectReference `json:"nodeGroup"`
}

// QueueSpec outputs metrics for a queue.
type QueueSpec struct {
	Type QueueProviderType `json:"type"`
	ID   string            `json:"id"`
}

// QueueProviderType corresponds to an implementation of a queue
type QueueProviderType string

// QueueProvider enum
const (
	AWSSQSQueueProvider QueueProviderType = "AWSSQSQueueProvider"
)

// MetricsProducer is the Schema for the MetricsProducers API
// +kubebuilder:object:root=true
type MetricsProducer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MetricsProducerSpec   `json:"spec,omitempty"`
	Status MetricsProducerStatus `json:"status,omitempty"`
}

// MetricsProducerList contains a list of MetricsProducer
// +kubebuilder:object:root=true
type MetricsProducerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetricsProducer `json:"items"`
}
