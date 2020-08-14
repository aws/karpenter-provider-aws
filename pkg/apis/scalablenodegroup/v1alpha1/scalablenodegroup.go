// Package v1alpha1 holds definitions for ScalableNodeGroup
// +kubebuilder:object:generate=true
// +groupName=karpenter.sh
package v1alpha1

import (
	"github.com/ellistarn/karpenter/pkg/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ScalableNodeGroupSpec is an abstract representation for a Cloud Provider's Node Group. It implements
// https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#scale-subresource
// which enables it to be targeted by Horizontal Pod Autoscalers.
type ScalableNodeGroupSpec struct {
	// Replicas is the desired number of replicas for the targeted Node Group
	Replicas *int32 `json:"replicas,omitempty"`
	// MinReplicas limits the minimum size of the ScalableNodeGroup
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	// MaxReplicas limits the maximum size of the ScalableNodeGroup
	MaxReplicas int32 `json:"maxReplicas,omitempty"`
	// Type for the resource of name ScalableNodeGroup.ObjectMeta.Name
	Type ProviderType `json:"type"`
}

// ScalableNodeGroupStatus holds status information for the ScalableNodeGroup
type ScalableNodeGroupStatus struct {
	// Replicas displays the current size of the ScalableNodeGroup
	Replicas int32 `json:"replicas,omitempty"`
}

// ProviderType refers to the implementation of the ScalableNodeGroup
type ProviderType string

// Supported provider implementations
const (
	AWSEC2AutoScalingGroup ProviderType = "AWSEC2AutoScalingGroup"
	AWSEKSManagedNodeGroup ProviderType = "AWSEKSManagedNodeGroup"
)

// ScalableNodeGroup is the Schema for the ScalableNodeGroups API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.selector
type ScalableNodeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScalableNodeGroupSpec   `json:"spec,omitempty"`
	Status ScalableNodeGroupStatus `json:"status,omitempty"`
}

// ScalableNodeGroupList contains a list of ScalableNodeGroup
// +kubebuilder:object:root=true
type ScalableNodeGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScalableNodeGroup `json:"items"`
}

func init() {
	apis.SchemeBuilder.Register(&ScalableNodeGroup{}, &ScalableNodeGroupList{})
}
