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

package v1beta1

import (
	"fmt"

	"github.com/mitchellh/hashstructure/v2"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeClassSpec is the top level specification for the AWS Karpenter Provider.
// This will contain configuration necessary to launch instances in AWS.
type NodeClassSpec struct {
	// SubnetSelectorTerms is a list of or subnet selector terms. The terms are ORed.
	// +optional
	SubnetSelectorTerms []SubnetSelectorTerm `json:"subnetSelectorTerms" hash:"ignore"`
	// SecurityGroupSelectorTerms is a list of or security group selector terms. The terms are ORed.
	// +optional
	SecurityGroupSelectorTerms []SecurityGroupSelectorTerm `json:"securityGroupSelectorTerms" hash:"ignore"`
	// AMISelectorTerms is a list of or ami selector terms. The terms are ORed.
	// +optional
	AMISelectorTerms []AMISelectorTerm `json:"amiSelectorTerms,omitempty" hash:"ignore"`
	// AMIFamily is the AMI family that instances use.
	// +optional
	AMIFamily *string `json:"amiFamily,omitempty"`
	// UserData to be applied to the provisioned nodes.
	// It must be in the appropriate format based on the AMIFamily in use. Karpenter will merge certain fields into
	// this UserData to ensure nodes are being provisioned with the correct configuration.
	// +optional
	UserData *string `json:"userData,omitempty"`
	// Role is the AWS identity that nodes use.
	// +optional
	Role *string `json:"role,omitempty"`
	// Tags to be applied on ec2 resources like instances and launch templates.
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
	// BlockDeviceMappings to be applied to provisioned nodes.
	// +optional
	BlockDeviceMappings []*BlockDeviceMapping `json:"blockDeviceMappings,omitempty"`
	// DetailedMonitoring controls if detailed monitoring is enabled for instances that are launched
	// +optional
	DetailedMonitoring *bool `json:"detailedMonitoring,omitempty"`
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
	// Context is a Reserved field in EC2 APIs
	// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateFleet.html
	// +optional
	Context *string `json:"context,omitempty"`
	// TODO @joinnis: Remove this field when v1alpha5 is unsupported in a future version of Karpenter
	// LaunchTemplateName for the node. If not specified, a launch template will be generated.
	// NOTE: This field is for specifying a custom launch template and is exposed in the Spec
	// as `launchTemplate` for backwards compatibility.
	// +optional
	LaunchTemplateName *string `json:"-" hash:"ignore"`
	// TODO @joinnis: Remove this field when v1alpha5 is unsupported in a future version of Karpenter
	// InstanceProfile is the AWS identity that instances use.
	// +optional
	InstanceProfile *string `json:"-" hash:"ignore"`
	// TODO @joinnis: Remove this field when v1alpha5 is unsupported in a future version of Karpenter
	// OriginalSubnetSelector is the original subnet selector that was used by the v1alpha5 representation of this API.
	// DO NOT USE THIS VALUE when performing business logic in code
	// +optional
	OriginalSubnetSelector map[string]string `json:"-" hash:"ignore"`
	// TODO @joinnis: Remove this field when v1alpha5 is unsupported in a future version of Karpenter
	// OriginalSecurityGroupSelector is the original security group selector that was used by the v1alpha5 representation of this API.
	// DO NOT USE THIS VALUE when performing business logic in code
	// +optional
	OriginalSecurityGroupSelector map[string]string `json:"-" hash:"ignore"`
	// TODO @joinnis: Remove this field when v1alpha5 is unsupported in a future version of Karpenter
	// OriginalAMISelector is the original ami selector that was used by the v1alpha5 representation of this API.
	// DO NOT USE THIS VALUE when performing business logic in code
	// +optional
	OriginalAMISelector map[string]string `json:"-" hash:"ignore"`
}

// SubnetSelectorTerm defines selection logic for a subnet used by Karpenter to launch nodes.
// If multiple fields are used for selection, the requirements are ANDed.
type SubnetSelectorTerm struct {
	// Tags is a map of key/value tags used to select subnets
	// Specifying '*' for a value selects all values for a given tag key.
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
	// ID is the subnet id in EC2
	// +kubebuilder:validation:Pattern="subnet-[0-9a-z]+"
	// +optional
	ID string `json:"id,omitempty"`
}

// SecurityGroupSelectorTerm defines selection logic for a security group used by Karpenter to launch nodes.
// If multiple fields are used for selection, the requirements are ANDed.
type SecurityGroupSelectorTerm struct {
	// Tags is a map of key/value tags used to select subnets
	// Specifying '*' for a value selects all values for a given tag key.
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
	// ID is the security group id in EC2
	// +kubebuilder:validation:Pattern:="sg-[0-9a-z]+"
	// +optional
	ID string `json:"id,omitempty"`
	// Name is the security group name in EC2.
	// This value is the name field, which is different from the name tag.
	Name string `json:"name,omitempty"`
}

// AMISelectorTerm defines selection logic for an ami used by Karpenter to launch nodes.
// If multiple fields are used for selection, the requirements are ANDed.
type AMISelectorTerm struct {
	// Tags is a map of key/value tags used to select subnets
	// Specifying '*' for a value selects all values for a given tag key.
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
	// ID is the ami id in EC2
	// +kubebuilder:validation:Pattern:="ami-[0-9a-z]+"
	// +optional
	ID string `json:"id,omitempty"`
	// Name is the ami name in EC2.
	// This value is the name field, which is different from the name tag.
	// +optional
	Name string `json:"name,omitempty"`
	// Owner is the owner for the ami.
	// You can specify a combination of AWS account IDs, "self", "amazon", and "aws-marketplace"
	// +optional
	Owner string `json:"owner,omitempty"`
	// SSM is the ssm alias for an ami.
	// +optional
	SSM string `json:"ssm,omitempty"`
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

type BlockDeviceMapping struct {
	// The device name (for example, /dev/sdh or xvdh).
	// +optional
	DeviceName *string `json:"deviceName,omitempty"`
	// EBS contains parameters used to automatically set up EBS volumes when an instance is launched.
	// +optional
	EBS *BlockDevice `json:"ebs,omitempty"`
}

type BlockDevice struct {
	// DeleteOnTermination indicates whether the EBS volume is deleted on instance termination.
	// +optional
	DeleteOnTermination *bool `json:"deleteOnTermination,omitempty"`
	// Encrypted indicates whether the EBS volume is encrypted. Encrypted volumes can only
	// be attached to instances that support Amazon EBS encryption. If you are creating
	// a volume from a snapshot, you can't specify an encryption value.
	// +optional
	Encrypted *bool `json:"encrypted,omitempty"`
	// IOPS is the number of I/O operations per second (IOPS). For gp3, io1, and io2 volumes,
	// this represents the number of IOPS that are provisioned for the volume. For
	// gp2 volumes, this represents the baseline performance of the volume and the
	// rate at which the volume accumulates I/O credits for bursting.
	//
	// The following are the supported values for each volume type:
	//
	//    * gp3: 3,000-16,000 IOPS
	//
	//    * io1: 100-64,000 IOPS
	//
	//    * io2: 100-64,000 IOPS
	//
	// For io1 and io2 volumes, we guarantee 64,000 IOPS only for Instances built
	// on the Nitro System (https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-types.html#ec2-nitro-instances).
	// Other instance families guarantee performance up to 32,000 IOPS.
	//
	// This parameter is supported for io1, io2, and gp3 volumes only. This parameter
	// is not supported for gp2, st1, sc1, or standard volumes.
	// +optional
	IOPS *int64 `json:"iops,omitempty"`
	// KMSKeyID (ARN) of the symmetric Key Management Service (KMS) CMK used for encryption.
	// +optional
	KMSKeyID *string `json:"kmsKeyID,omitempty"`
	// SnapshotID is the ID of an EBS snapshot
	// +optional
	SnapshotID *string `json:"snapshotID,omitempty"`
	// Throughput to provision for a gp3 volume, with a maximum of 1,000 MiB/s.
	// Valid Range: Minimum value of 125. Maximum value of 1000.
	// +optional
	Throughput *int64 `json:"throughput,omitempty"`
	// VolumeSize in GiBs. You must specify either a snapshot ID or
	// a volume size. The following are the supported volumes sizes for each volume
	// type:
	//
	//    * gp2 and gp3: 1-16,384
	//
	//    * io1 and io2: 4-16,384
	//
	//    * st1 and sc1: 125-16,384
	//
	//    * standard: 1-1,024
	// +optional
	VolumeSize *resource.Quantity `json:"volumeSize,omitempty" hash:"string"`
	// VolumeType of the block device.
	// For more information, see Amazon EBS volume types (https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html)
	// in the Amazon Elastic Compute Cloud User Guide.
	// +optional
	VolumeType *string `json:"volumeType,omitempty"`
}

// NodeClass is the Schema for the NodeClass API
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=nodeclasses,scope=Cluster,categories=karpenter
// +kubebuilder:subresource:status
type NodeClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeClassSpec   `json:"spec,omitempty"`
	Status NodeClassStatus `json:"status,omitempty"`

	// IsNodeTemplate tells Karpenter whether the in-memory representation of this object
	// is actually referring to a AWSNodeTemplate object. This value is not actually part of the v1beta1 public-facing API
	// TODO @joinnis: Remove this field when v1alpha5 is unsupported in a future version of Karpenter
	IsNodeTemplate bool `json:"-" hash:"ignore"`
}

func (a *NodeClass) Hash() string {
	return fmt.Sprint(lo.Must(hashstructure.Hash(a.Spec, hashstructure.FormatV2, &hashstructure.HashOptions{
		SlicesAsSets:    true,
		IgnoreZeroValue: true,
		ZeroNil:         true,
	})))
}

// NodeClassList contains a list of NodeClass
// +kubebuilder:object:root=true
type NodeClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeClass `json:"items"`
}
