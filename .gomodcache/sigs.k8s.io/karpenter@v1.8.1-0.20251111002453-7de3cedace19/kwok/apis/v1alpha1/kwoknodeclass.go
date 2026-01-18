/*
Copyright The Kubernetes Authors.

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
)

type KWOKNodeClassSpec struct {
	// NodeRegistrationDelay is a delay for KWOK nodes to register to the cluster.
	// This is meant to model instance startup time that can happen when hardware
	// needs to start on providers that are backed by real instances.
	// +kubebuilder:validation:Pattern=`^([0-9]+(s|m|h))+$`
	// +kubebuilder:validation:Type="string"
	// +optional
	NodeRegistrationDelay metav1.Duration `json:"nodeRegistrationDelay,omitempty"`
}

// KWOKNodeClass is the Schema for the KWOKNodeClass API
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=kwoknodeclasses,scope=Cluster,categories=karpenter,shortName={kwoknc,kwokncs}
// +kubebuilder:subresource:status
type KWOKNodeClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec KWOKNodeClassSpec `json:"spec,omitempty"`
	// +kubebuilder:default:={conditions: {{type: "Ready", status: "True", reason:"Ready", lastTransitionTime: "2024-01-01T01:01:01Z", message: ""}}}
	Status KWOKNodeClassStatus `json:"status,omitempty"`
}

// KWOKNodeClassList contains a list of KwokNodeClass
// +kubebuilder:object:root=true
type KWOKNodeClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KWOKNodeClass `json:"items"`
}
