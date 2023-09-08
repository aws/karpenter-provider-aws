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
	"fmt"

	"github.com/mitchellh/hashstructure/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Subnet contains resolved Subnet selector values utilized for node launch
type Subnet struct {
	// ID of the subnet
	// +required
	ID string `json:"id"`
	// The associated availability zone
	// +required
	Zone string `json:"zone"`
}

// SecurityGroup contains resolved SecurityGroup selector values utilized for node launch
type SecurityGroup struct {
	// ID of the security group
	// +required
	ID string `json:"id"`
	// Name of the security group
	// +optional
	Name string `json:"name,omitempty"`
}

// AMI contains resolved AMI selector values utilized for node launch
type AMI struct {
	// ID of the AMI
	// +required
	ID string `json:"id"`
	// Name of the AMI
	// +optional
	Name string `json:"name,omitempty"`
	// Requirements of the AMI to be utilized on an instance type
	// +required
	Requirements []v1.NodeSelectorRequirement `json:"requirements"`
}

// AWSNodeTemplateStatus contains the resolved state of the AWSNodeTemplate
type AWSNodeTemplateStatus struct {
	// Subnets contains the current Subnet values that are available to the
	// cluster under the subnet selectors.
	// +optional
	Subnets []Subnet `json:"subnets,omitempty"`
	// SecurityGroups contains the current Security Groups values that are available to the
	// cluster under the SecurityGroups selectors.
	// +optional
	SecurityGroups []SecurityGroup `json:"securityGroups,omitempty"`
	// AMI contains the current AMI values that are available to the
	// cluster under the AMI selectors.
	// +optional
	AMIs []AMI `json:"amis,omitempty"`
}

// AWSNodeTemplateSpec is the top level specification for the AWS Karpenter Provider.
// This will contain configuration necessary to launch instances in AWS.
type AWSNodeTemplateSpec struct {
	// UserData to be applied to the provisioned nodes.
	// It must be in the appropriate format based on the AMIFamily in use. Karpenter will merge certain fields into
	// this UserData to ensure nodes are being provisioned with the correct configuration.
	// +optional
	// +kubebuilder:validation:XValidation:message=Only one of UserData and AMISelector can be provided,rule=!has(self.AMISelector) || has(self.AMISelector) != has(self.UserData)
	UserData *string `json:"userData,omitempty"`
	AWS      `json:",inline"`
	// AMISelector discovers AMIs to be used by Amazon EC2 tags.
	// +optional
	// +kubebuilder:validation:XValidation:message=Only one of UserData and AMISelector can be provided,rule=!has(self.AMISelector) || has(self.AMISelector) != has(self.UserData)
	AMISelector map[string]string `json:"amiSelector,omitempty" hash:"ignore"`
	// DetailedMonitoring controls if detailed monitoring is enabled for instances that are launched
	// +optional
	DetailedMonitoring *bool `json:"detailedMonitoring,omitempty"`
}

// AWSNodeTemplate is the Schema for the AWSNodeTemplate API
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=awsnodetemplates,scope=Cluster,categories=karpenter
// +kubebuilder:subresource:status
type AWSNodeTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:XValidation:message=If AMIFamily is Custom then AMI Selector must be set,rule=!has(self.AMIFamily) || !self.AMIFamily == "Custom" || has(self.AMISelector)
	// +kubebuilder:validation:XValidation:message=Only one of AMISelector and LaunchTemplate can be provided,rule=!has(self.AMISelector) || has(self.AMISelector) != has(self.LaunchTemplateName)
	// +kubebuilder:validation:XValidation:message=AMI ID must be a valid ami-id,rule=!has(self.AMISelector) || !has(self.AMISelector["aws-ids"]) || self.AMISelector["aws-ids"].matches("^ami-[a-f0-9]{8,17}$")
	// +kubebuilder:validation:XValidation:message=AMI ID must be a valid ami-id,rule=!has(self.AMISelector) || !has(self.AMISelector["aws::ids"]) || self.AMISelector["aws::ids"].matches("^ami-[a-f0-9]{8,17}$")
	// +kubebuilder:validation:XValidation:message=Tags must not match kubernetes patterns,rule="!has(self.Tags) || !self.Tags.all(e, e.key.matches('^kubernetes.io/cluster/[0-9A-Za-z][A-Za-z0-9\\-_]*$'))"
	// +kubebuilder:validation:XValidation:message=Tags must not match karpenter provisioner label,rule="!has(self.Tags) || !self.Tags.all(e, e.key.matches(^karpenter.k8s.aws/provisioner-name$))"
	// +kubebuilder:validation:XValidation:message=Tags must not match karpenter managed by tag,rule="!has(self.Tags) || !self.Tags.all(e, e.key.matches(^karpenter.k8s.aws/machine-managed-by$))"
	Spec   AWSNodeTemplateSpec   `json:"spec,omitempty"`
	Status AWSNodeTemplateStatus `json:"status,omitempty"`
}

func (a *AWSNodeTemplate) Hash() string {
	hash, _ := hashstructure.Hash(a.Spec, hashstructure.FormatV2, &hashstructure.HashOptions{
		SlicesAsSets:    true,
		IgnoreZeroValue: true,
		ZeroNil:         true,
	})

	return fmt.Sprint(hash)
}

// AWSNodeTemplateList contains a list of AWSNodeTemplate
// +kubebuilder:object:root=true
type AWSNodeTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AWSNodeTemplate `json:"items"`
}
