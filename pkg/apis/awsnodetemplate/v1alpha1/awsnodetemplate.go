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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AWSNodeTemplateSpec is the top level specification for the AWS Karpenter Provider.
// This will contain configuration necessary to launch instances in AWS.
type AWSNodeTemplateSpec struct {
	// UserData to be applied to the provisioned nodes.
	// It must be in the appropriate format based on the AMIFamily in use. Karpenter will merge certain fields into
	// this UserData to ensure nodes are being provisioned with the correct configuration.
	// +optional
	UserData *string `json:"userData,omitempty"`
	AWS      `json:",inline"`
	// AMISelector discovers AMIs to be used by Amazon EC2 tags.
	// +optional
	AMISelector map[string]string `json:"amiSelector,omitempty"`
}

// AWSNodeTemplate is the Schema for the AWSNodeTemplate API
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=awsnodetemplates,scope=Cluster,categories=karpenter
// +kubebuilder:subresource:status
type AWSNodeTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AWSNodeTemplateSpec `json:"spec,omitempty"`
}

// AWSNodeTemplateList contains a list of AWSNodeTemplate
// +kubebuilder:object:root=true
type AWSNodeTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AWSNodeTemplate `json:"items"`
}
