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

// Package v1alpha3 contains API Schema definitions for the v1alpha3 API group
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=package,register
// +k8s:defaulter-gen=TypeMeta
// +groupName=karpenter.k8s.aws
package v1alpha1

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewConstraints(constraints *v1alpha3.Constraints) (*Constraints, error) {
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
	*v1alpha3.Constraints
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
	// InstanceRole is the AWS identity that instances use.
	// +required
	InstanceRole string `json:"instanceRole"`
	// Subnets specify the names of the subnets.
	// +optional
	Subnets []string `json:"subnets,omitempty"`
	// SubnetSelector discovers subnets by tags. A value of "" is a wildcard.
	// +optional
	SubnetSelector map[string]string `json:"subnetTags,omitempty"`
	// SecurityGroups specify the names of the security groups.
	// +optional
	SecurityGroups []string `json:"securityGroups,omitempty"`
	// SecurityGroupSelector discovers security groups by tags. A value of "" is a wildcard.
	// +optional
	SecurityGroupsSelector map[string]string `json:"securityGroupSelector,omitempty"`
	// LaunchTemplate for the node. If not specified, a launch template will be generated.
	// +optional
	LaunchTemplate *string `json:"launchTemplate,omitempty"`
	// CapacityType for the node. If not specified, defaults to on-demand
	// +optional
	CapacityType *string `json:"capacityType,omitempty"`
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

func (c *Constraints) GetCapacityType() string {
	capacityType, ok := c.Labels[CapacityTypeLabel]
	if !ok {
		capacityType = CapacityTypeOnDemand
	}
	return capacityType
}

type LaunchTemplate struct {
	Name    string
	Version string
}

func (c *Constraints) GetLaunchTemplate() *LaunchTemplate {
	name, ok := c.Labels[LaunchTemplateNameLabel]
	if !ok {
		return nil
	}
	return &LaunchTemplate{
		Name:    name,
		Version: DefaultLaunchTemplateVersion,
	}
}

func (c *Constraints) GetSubnetName() *string {
	name, ok := c.Labels[SubnetNameLabel]
	if !ok {
		return nil
	}
	return aws.String(name)
}

func (c *Constraints) GetSubnetTagKey() *string {
	tag, ok := c.Labels[SubnetTagKeyLabel]
	if !ok {
		return nil
	}
	return aws.String(tag)
}

func (c *Constraints) GetSecurityGroupName() *string {
	name, ok := c.Labels[SecurityGroupNameLabel]
	if !ok {
		return nil
	}
	return aws.String(name)
}

func (c *Constraints) GetSecurityGroupTagKey() *string {
	tag, ok := c.Labels[SecurityGroupTagKeyLabel]
	if !ok {
		return nil
	}
	return aws.String(tag)
}
