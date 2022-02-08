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
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultMetadataOptionsHTTPEndpoint            = ec2.LaunchTemplateInstanceMetadataEndpointStateEnabled
	DefaultMetadataOptionsHTTPProtocolIPv6        = ec2.LaunchTemplateInstanceMetadataProtocolIpv6Disabled
	DefaultMetadataOptionsHTTPPutResponseHopLimit = 2
	DefaultMetadataOptionsHTTPTokens              = ec2.LaunchTemplateHttpTokensStateRequired
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
	// AMIFamily is the AMI family that instances use.
	// +optional
	AMIFamily *string `json:"amiFamily,omitempty"`
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
	// MetadataOptions for the generated launch template of provisioned nodes.
	//
	// This specifies the exposure of the Instance Metadata Service to
	// provisioned EC2 nodes. For more information,
	// see Instance Metadata and User Data
	// (https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html)
	// in the Amazon Elastic Compute Cloud User Guide.
	//
	// Refer to recommended, security best practices
	// (https://aws.github.io/aws-eks-best-practices/security/docs/iam/#restrict-access-to-the-instance-profile-assigned-to-the-worker-node)
	// for limiting exposure of Instance Metadata and User Data to pods.
	// If omitted, defaults to httpEndpoint enabled, with httpProtocolIPv6
	// disabled, with httpPutResponseLimit of 2, and with httpTokens
	// required.
	// +optional
	MetadataOptions *MetadataOptions `json:"metadataOptions,omitempty"`
}

// MetadataOptions contains parameters for specifying the exposure of the
// Instance Metadata Service to provisioned EC2 nodes.
type MetadataOptions struct {
	// HTTPEndpoint enables or disables the HTTP metadata endpoint on provisioned
	// nodes. If metadata options is non-nil, but this parameter is not specified,
	// the default state is "enabled".
	//
	// If you specify a value of "disabled", instance metadata will not be accessible
	// on the node.
	// +optional
	HTTPEndpoint *string `json:"httpEndpoint,omitempty"`

	// HTTPProtocolIPv6 enables or disables the IPv6 endpoint for the instance metadata
	// service on provisioned nodes. If metadata options is non-nil, but this parameter
	// is not specified, the default state is "disabled".
	// +optional
	HTTPProtocolIPv6 *string `json:"httpProtocolIPv6,omitempty"`

	// HTTPPutResponseHopLimit is the desired HTTP PUT response hop limit for
	// instance metadata requests. The larger the number, the further instance
	// metadata requests can travel. Possible values are integers from 1 to 64.
	// If metadata options is non-nil, but this parameter is not specified, the
	// default value is 1.
	// +optional
	HTTPPutResponseHopLimit *int64 `json:"httpPutResponseHopLimit,omitempty"`

	// HTTPTokens determines the state of token usage for instance metadata
	// requests. If metadata options is non-nil, but this parameter is not
	// specified, the default state is "optional".
	//
	// If the state is optional, one can choose to retrieve instance metadata with
	// or without a signed token header on the request. If one retrieves the IAM
	// role credentials without a token, the version 1.0 role credentials are
	// returned. If one retrieves the IAM role credentials using a valid signed
	// token, the version 2.0 role credentials are returned.
	//
	// If the state is "required", one must send a signed token header with any
	// instance metadata retrieval requests. In this state, retrieving the IAM
	// role credentials always returns the version 2.0 credentials; the version
	// 1.0 credentials are not available.
	// +optional
	HTTPTokens *string `json:"httpTokens,omitempty"`
}

func Deserialize(constraints *v1alpha5.Constraints) (*Constraints, error) {
	if constraints.Provider == nil {
		return nil, fmt.Errorf("invariant violated: spec.provider is not defined. Is the defaulting webhook installed?")
	}
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
	if constraints.Provider == nil {
		return fmt.Errorf("invariant violated: spec.provider is not defined. Is the defaulting webhook installed?")
	}
	bytes, err := json.Marshal(a)
	if err != nil {
		return err
	}
	constraints.Provider.Raw = bytes
	return nil
}

func (a *AWS) GetMetadataOptions() *MetadataOptions {
	if a.MetadataOptions == nil {
		return &MetadataOptions{
			HTTPEndpoint:            aws.String(DefaultMetadataOptionsHTTPEndpoint),
			HTTPProtocolIPv6:        aws.String(DefaultMetadataOptionsHTTPProtocolIPv6),
			HTTPPutResponseHopLimit: aws.Int64(DefaultMetadataOptionsHTTPPutResponseHopLimit),
			HTTPTokens:              aws.String(DefaultMetadataOptionsHTTPTokens),
		}
	}
	return a.MetadataOptions
}
