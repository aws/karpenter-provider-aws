/*
Copyright 2018 The Kubernetes Authors.

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

package pkg

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChaosPodSpec defines the desired state of ChaosPod
type ChaosPodSpec struct {
	Template corev1.PodTemplateSpec `json:"template"`
	// +optional
	NextStop metav1.Time `json:"nextStop,omitempty"`
}

// ChaosPodStatus defines the observed state of ChaosPod.
// It should always be reconstructable from the state of the cluster and/or outside world.
type ChaosPodStatus struct {
	LastRun metav1.Time `json:"lastRun,omitempty"`
}

// +kubebuilder:object:root=true

// ChaosPod is the Schema for the randomjobs API
// +kubebuilder:printcolumn:name="next stop",type="string",JSONPath=".spec.nextStop",format="date"
// +kubebuilder:printcolumn:name="last run",type="string",JSONPath=".status.lastRun",format="date"
type ChaosPod struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChaosPodSpec   `json:"spec,omitempty"`
	Status ChaosPodStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ChaosPodList contains a list of ChaosPod
type ChaosPodList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChaosPod `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ChaosPod{}, &ChaosPodList{})
}
