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
	"encoding/json"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Constraints wraps generic constraints with AWS specific parameters
type Constraints struct {
	*v1alpha5.Constraints
	*AWS
}

// AWS contains parameters specific to this cloud provider
// +kubebuilder:object:root=true
type AWS struct {
	// TypeMeta includes version and kind of the extensions, inferred if not provided.
	// +optional
	metav1.TypeMeta `json:",inline"`
	// InstanceProfile is the AWS identity that instances use.
	// +required
	InstanceProfile string `json:"instanceProfile"`
	// LaunchTemplate for the node. If not specified, a launch template will be generated.
	// +optional
	LaunchTemplate *string `json:"launchTemplate,omitempty"`
	// SubnetSelector discovers subnets by tags. A value of "" is a wildcard.
	// +optional
	SubnetSelector map[string]string `json:"subnetSelector,omitempty"`
	// SecurityGroups specify the names of the security groups.
	// +optional
	SecurityGroupSelector map[string]string `json:"securityGroupSelector,omitempty"`
	// Tags to be applied on ec2 resources like instances and launch templates.
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
}

func Deserialize(constraints *v1alpha5.Constraints) (*Constraints, error) {
	aws := &AWS{}
	_, gvk, err := Codec.UniversalDeserializer().Decode(constraints.Provider.Raw, nil, aws)
	if err != nil {
		return nil, err
	}
	if gvk != nil {
		aws.SetGroupVersionKind(*gvk)
	}
	return &Constraints{constraints, aws}, nil
}

func (a *AWS) Serialize(constraints *v1alpha5.Constraints) error {
	bytes, err := json.Marshal(a)
	if err != nil {
		return err
	}
	constraints.Provider.Raw = bytes
	return nil
}
