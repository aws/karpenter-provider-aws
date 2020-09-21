package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MetricsProducerSpec defines an object that outputs metrics.
type MetricsProducerSpec struct {
	// PendingCapacity produces a metric that recommends increases or decreases
	// to the sizes of a set of node groups based on pending pods.
	// +optional
	PendingCapacity *PendingCapacitySpec `json:"pendingCapacity,omitempty"`
	// Queue produces metrics about a specified queue, such as length and age of oldest message,
	// +optional
	Queue *QueueSpec `json:"queue,omitempty"`
	// ReservedCapacity produces a metric corresponding to the ratio of committed resources
	// to availabile resources for the nodes of a specified node group.
	// +optional
	ReservedCapacity *ReservedCapacitySpec `json:"reservedCapacity,omitempty"`
	// ScheduledCapacity produces a metric according to a specified schedule.
	// +optional
	ScheduledCapacity *ScheduledCapacitySpec `json:"scheduledCapacity,omitempty"`
}

type ReservedCapacitySpec struct {
	// NodeSelectors specifies a list of node groups. Each selector must uniquely identify a set of nodes.
	NodeSelectors []v1.NodeSelector `json:"nodeSelectors"`
}

type PendingCapacitySpec struct {
	// NodeSelectors specifies a list of node groups. Each selector must uniquely identify a set of nodes.
	NodeSelectors []v1.NodeSelector `json:"nodeSelectors"`
}

type ScheduledCapacitySpec struct {
	// NodeSelectors specifies a list of node groups. Each selector must uniquely identify a set of nodes.
	NodeSelectors []v1.NodeSelector `json:"nodeSelectors"`
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
	// NodeSelectors specifies a list of node groups. Each selector must uniquely identify a set of nodes.
	NodeSelectors []v1.NodeSelector `json:"nodeSelectors"`
}

// QueueSpec outputs metrics for a queue.
type QueueSpec struct {
	Type QueueType `json:"type"`
	ID   string    `json:"id"`
}

// QueueType corresponds to an implementation of a queue
type QueueType string

// QueueType enum
const (
	AWSSQSQueueType QueueType = "AWSSQSQueue"
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
