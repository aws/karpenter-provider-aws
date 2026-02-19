// +kubebuilder:object:generate=true
// +groupName=node.eks.aws
// +kubebuilder:validation:Optional
package v1alpha1

import (
	"github.com/awslabs/amazon-eks-ami/nodeadm/api"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	GroupVersion  = schema.GroupVersion{Group: api.GroupName, Version: "v1alpha1"}
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}
	AddToScheme   = SchemeBuilder.AddToScheme
)
