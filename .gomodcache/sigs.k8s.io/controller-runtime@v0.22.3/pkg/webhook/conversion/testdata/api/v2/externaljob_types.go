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

package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ExternalJobSpec defines the desired state of ExternalJob
type ExternalJobSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	ScheduleAt string `json:"scheduleAt"`

	// PanicInConversion triggers a panic during conversion when set to true.
	PanicInConversion bool `json:"panicInConversion"`
}

// ExternalJobStatus defines the observed state of ExternalJob
type ExternalJobStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// ExternalJob is the Schema for the externaljobs API
type ExternalJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExternalJobSpec   `json:"spec,omitempty"`
	Status ExternalJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ExternalJobList contains a list of ExternalJob
type ExternalJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExternalJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExternalJob{}, &ExternalJobList{})
}

// Hub is just a marker method to indicate that v2.ExternalJob is the Hub type
// in this case.
// v2.ExternalJob is the storage version so mark this as Hub.
// Storage version doesn't need to implement any conversion methods because
// default conversionHandler implements conversion logic for storage version.
// TODO(droot): Add comment annotation here to mark it as storage version
func (ej *ExternalJob) Hub() {}
