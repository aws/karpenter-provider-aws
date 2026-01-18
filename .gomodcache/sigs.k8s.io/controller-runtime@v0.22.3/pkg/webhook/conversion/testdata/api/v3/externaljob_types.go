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

package v3

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	v2 "sigs.k8s.io/controller-runtime/pkg/webhook/conversion/testdata/api/v2"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ExternalJobSpec defines the desired state of ExternalJob
type ExternalJobSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	DeferredAt string `json:"deferredAt"`

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

// ConvertTo implements conversion logic to convert to Hub type (v2.ExternalJob
// in this case)
func (ej *ExternalJob) ConvertTo(dst conversion.Hub) error {
	if ej.Spec.PanicInConversion {
		panic("PanicInConversion field set to true")
	}
	switch t := dst.(type) {
	case *v2.ExternalJob:
		jobv2 := dst.(*v2.ExternalJob)
		jobv2.ObjectMeta = ej.ObjectMeta
		jobv2.Spec.ScheduleAt = ej.Spec.DeferredAt
		return nil
	default:
		return fmt.Errorf("unsupported type %v", t)
	}
}

// ConvertFrom implements conversion logic to convert from Hub type (v2.ExternalJob
// in this case)
func (ej *ExternalJob) ConvertFrom(src conversion.Hub) error {
	if ej.Spec.PanicInConversion {
		panic("PanicInConversion field set to true")
	}
	switch t := src.(type) {
	case *v2.ExternalJob:
		jobv2 := src.(*v2.ExternalJob)
		ej.ObjectMeta = jobv2.ObjectMeta
		ej.Spec.DeferredAt = jobv2.Spec.ScheduleAt
		return nil
	default:
		return fmt.Errorf("unsupported type %v", t)
	}
}
