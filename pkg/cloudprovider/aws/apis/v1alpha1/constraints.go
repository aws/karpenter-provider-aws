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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewConstraints(constraints *v1alpha4.Constraints) (*Constraints, error) {
	aws := &AWS{}
	_, gvk, err := Codec.UniversalDeserializer().Decode(constraints.Provider.Raw, nil, aws)
	if err != nil {
		return nil, fmt.Errorf("decoding provider, %w", err)
	}
	if gvk != nil {
		aws.SetGroupVersionKind(*gvk)
	}
	return &Constraints{Constraints: constraints, AWS: aws}, nil
}

// Constraints are used to specify node creation parameters. Both types are
// embedded to enforce compile time checks against field conflicts.
type Constraints struct {
	*v1alpha4.Constraints
	*AWS
}

// Extensions are parameters specific to this cloud provider
// +kubebuilder:object:root=true
type AWS struct {
	// TypeMeta includes version and kind of the extensions, inferred if not provided.
	// +optional
	metav1.TypeMeta `json:",inline"`
	// Cluster is used to connect Nodes to the Kubernetes cluster.
	// +required
	Cluster Cluster `json:"cluster"`
	// InstanceProfile is the AWS identity that instances use.
	// +required
	InstanceProfile string `json:"instanceProfile"`
	// CapacityType for the node. If not specified, defaults to on-demand.
	// May be overriden by pods.spec.nodeSelector["node.k8s.aws/capacityType"]
	// +optional
	CapacityType *string `json:"capacityType,omitempty"`
	// LaunchTemplate for the node. If not specified, a launch template will be generated.
	// +optional
	LaunchTemplate *string `json:"launchTemplate,omitempty"`
	// SubnetSelector discovers subnets by tags. A value of "" is a wildcard.
	// +optional
	SubnetSelector map[string]string `json:"subnetSelector,omitempty"`
	// SecurityGroups specify the names of the security groups.
	// +optional
	SecurityGroupsSelector map[string]string `json:"securityGroupSelector,omitempty"`
}

// Cluster configures the cluster that the provisioner operates against.
type Cluster struct {
	// Name is required to authenticate with the API Server.
	// +required
	Name string `json:"name"`
	// Endpoint is required for nodes to connect to the API Server.
	// +required
	Endpoint string `json:"endpoint"`
}
