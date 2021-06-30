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

// Package v1alpha2 contains API Schema definitions for the v1alpha2 API group
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=package,register
// +k8s:defaulter-gen=TypeMeta
// +groupName=provisioning.karpenter.sh
package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
)

var SchemeGroupVersion = schema.GroupVersion{Group: "provisioning.karpenter.sh", Version: "v1alpha2"}
var SchemeBuilder = runtime.NewSchemeBuilder(func(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Provisioner{},
		&ProvisionerList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
})

const (
	// Active is a condition implemented by all resources. It indicates that the
	// controller is able to take actions: it's correctly configured, can make
	// necessary API calls, and isn't disabled.
	Active apis.ConditionType = "Active"
)
