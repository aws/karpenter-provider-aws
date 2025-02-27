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
	_ "embed"

	"github.com/awslabs/operatorpkg/object"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:generate controller-gen crd object:headerFile="../../../hack/boilerplate.go.txt" paths="./..." output:crd:artifacts:config=crds
var (
	//go:embed crds/karpenter.test.sh_testnodeclasses.yaml
	TestNodeClassCRD []byte
	CRDs             = []*v1.CustomResourceDefinition{
		object.Unmarshal[v1.CustomResourceDefinition](TestNodeClassCRD),
	}
)

// TestNodeClass is the Schema for the TestNodeClass API
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=testnodeclasses,scope=Cluster
// +kubebuilder:subresource:status
type TestNodeClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Status            TestNodeClassStatus `json:"status,omitempty"`
}

// TestNodeClassList contains a list of TestNodeClass
// +kubebuilder:object:root=true
type TestNodeClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TestNodeClass `json:"items"`
}
