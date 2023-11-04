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

// EC2NodeClassSpec is the top level specification for the AWS Karpenter Provider.
// This will contain configuration necessary to launch instances in AWS.
type EC2NodeClassSpec struct {
	// SubnetSelectorTerms is a list of or subnet selector terms. The terms are ORed.
	// +kubebuilder:validation:XValidation:message="subnetSelectorTerms cannot be empty",rule="self.size() != 0"
	// +kubebuilder:validation:XValidation:message="expected at least one, got none, ['tags', 'id']",rule="self.all(x, has(x.tags) || has(x.id))"
	// +kubebuilder:validation:XValidation:message="'id' is mutually exclusive, cannot be set with a combination of other fields in subnetSelectorTerms",rule="!self.all(x, has(x.id) && has(x.tags))"
	// +kubebuilder:validation:MaxItems:=30
	// +required
	SubnetSelectorTerms []SubnetSelectorTerm `json:"subnetSelectorTerms" hash:"ignore"`
	// SecurityGroupSelectorTerms is a list of or security group selector terms. The terms are ORed.
	// +kubebuilder:validation:XValidation:message="securityGroupSelectorTerms cannot be empty",rule="self.size() != 0"
	// +kubebuilder:validation:XValidation:message="expected at least one, got none, ['tags', 'id', 'name']",rule="self.all(x, has(x.tags) || has(x.id) || has(x.name))"
	// +kubebuilder:validation:XValidation:message="'id' is mutually exclusive, cannot be set with a combination of other fields in securityGroupSelectorTerms",rule="!self.all(x, has(x.id) && (has(x.tags) || has(x.name)))"
	// +kubebuilder:validation:XValidation:message="'name' is mutually exclusive, cannot be set with a combination of other fields in securityGroupSelectorTerms",rule="!self.all(x, has(x.name) && (has(x.tags) || has(x.id)))"
	// +kubebuilder:validation:MaxItems:=30
	// +required
	SecurityGroupSelectorTerms []SecurityGroupSelectorTerm `json:"securityGroupSelectorTerms" hash:"ignore"`
	// AMISelectorTerms is a list of or ami selector terms. The terms are ORed.
	// +kubebuilder:validation:XValidation:message="expected at least one, got none, ['tags', 'id', 'name']",rule="self.all(x, has(x.tags) || has(x.id) || has(x.name))"
	// +kubebuilder:validation:XValidation:message="'id' is mutually exclusive, cannot be set with a combination of other fields in amiSelectorTerms",rule="!self.all(x, has(x.id) && (has(x.tags) || has(x.name) || has(x.owner)))"
	// +kubebuilder:validation:MaxItems:=30
	// +optional
	AMISelectorTerms []AMISelectorTerm `json:"amiSelectorTerms,omitempty" hash:"ignore"`
	// AMIFamily is the AMI family that instances use.
	// +kubebuilder:validation:Enum:={AL2,Bottlerocket,Ubuntu,Custom,Windows2019,Windows2022}
	// +required
	AMIFamily *string `json:"amiFamily"`
	// UserData to be applied to the provisioned nodes.
	// It must be in the appropriate format based on the AMIFamily in use. Karpenter will merge certain fields into
	// this UserData to ensure nodes are being provisioned with the correct configuration.
	// +optional
	UserData *string `json:"userData,omitempty"`
	// Role is the AWS identity that nodes use. This field is immutable.
	// Marking this field as immutable avoids concerns around terminating managed instance profiles from running instances.
	// This field may be made mutable in the future, assuming the correct garbage collection and drift handling is implemented
	// for the old instance profiles on an update.
	// +kubebuilder:validation:XValidation:rule="self != ''",message="role cannot be empty"
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="immutable field changed"
	// +required
	Role string `json:"role"`
	// Tags to be applied on ec2 resources like instances and launch templates.
	// +kubebuilder:validation:XValidation:message="empty tag keys aren't supported",rule="self.all(k, k != '')"
	// +kubebuilder:validation:XValidation:message="tag contains a restricted tag matching kubernetes.io/cluster/",rule="self.all(k, !k.startsWith('kubernetes.io/cluster') )"
	// +kubebuilder:validation:XValidation:message="tag contains a restricted tag matching karpenter.sh/provisioner-name",rule="self.all(k, k != 'karpenter.sh/provisioner-name')"
	// +kubebuilder:validation:XValidation:message="tag contains a restricted tag matching karpenter.sh/nodepool",rule="self.all(k, k != 'karpenter.sh/nodepool')"
	// +kubebuilder:validation:XValidation:message="tag contains a restricted tag matching karpenter.sh/managed-by",rule="self.all(k, k !='karpenter.sh/managed-by')"
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
	// BlockDeviceMappings to be applied to provisioned nodes.
	// +kubebuilder:validation:XValidation:message="must have only one blockDeviceMappings with rootVolume",rule="self.filter(x, has(x.rootVolume)?x.rootVolume==true:false).size() <= 1"
	// +kubebuilder:validation:MaxItems:=50
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
	// +kubebuilder:default={"httpEndpoint":"enabled","httpProtocolIPv6":"disabled","httpPutResponseHopLimit":2,"httpTokens":"required"}
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
	// +kubebuilder:validation:XValidation:message="empty tag keys or values aren't supported",rule="self.all(k, k != '' && self[k] != '')"
	// +kubebuilder:validation:MaxProperties:=20
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
	// +kubebuilder:validation:XValidation:message="empty tag keys or values aren't supported",rule="self.all(k, k != '' && self[k] != '')"
	// +kubebuilder:validation:MaxProperties:=20
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
	// +kubebuilder:validation:XValidation:message="empty tag keys or values aren't supported",rule="self.all(k, k != '' && self[k] != '')"
	// +kubebuilder:validation:MaxProperties:=20
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
	// +kubebuilder:default=enabled
	// +kubebuilder:validation:Enum:={enabled,disabled}
	// +optional
	HTTPEndpoint *string `json:"httpEndpoint,omitempty"`
	// HTTPProtocolIPv6 enables or disables the IPv6 endpoint for the instance metadata
	// service on provisioned nodes. If metadata options is non-nil, but this parameter
	// is not specified, the default state is "disabled".
	// +kubebuilder:default=disabled
	// +kubebuilder:validation:Enum:={enabled,disabled}
	// +optional
	HTTPProtocolIPv6 *string `json:"httpProtocolIPv6,omitempty"`
	// HTTPPutResponseHopLimit is the desired HTTP PUT response hop limit for
	// instance metadata requests. The larger the number, the further instance
	// metadata requests can travel. Possible values are integers from 1 to 64.
	// If metadata options is non-nil, but this parameter is not specified, the
	// default value is 2.
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=64
	// +optional
	HTTPPutResponseHopLimit *int64 `json:"httpPutResponseHopLimit,omitempty"`
	// HTTPTokens determines the state of token usage for instance metadata
	// requests. If metadata options is non-nil, but this parameter is not
	// specified, the default state is "required".
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
	// +kubebuilder:default=required
	// +kubebuilder:validation:Enum:={required,optional}
	// +optional
	HTTPTokens *string `json:"httpTokens,omitempty"`
}

type BlockDeviceMapping struct {
	// The device name (for example, /dev/sdh or xvdh).
	// +required
	DeviceName *string `json:"deviceName,omitempty"`
	// EBS contains parameters used to automatically set up EBS volumes when an instance is launched.
	// +kubebuilder:validation:XValidation:message="snapshotID or volumeSize must be defined",rule="has(self.snapshotID) || has(self.volumeSize)"
	// +required
	EBS *BlockDevice `json:"ebs,omitempty"`
	// RootVolume is a flag indicating if this device is mounted as kubelet root dir. You can
	// configure at most one root volume in BlockDeviceMappings.
	RootVolume bool `json:"rootVolume,omitempty"`
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
	// VolumeSize in `Gi`, `G`, `Ti`, or `T`. You must specify either a snapshot ID or
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
	// + TODO: Add the CEL resources.quantity type after k8s 1.29
	// + https://github.com/kubernetes/apiserver/commit/b137c256373aec1c5d5810afbabb8932a19ecd2a#diff-838176caa5882465c9d6061febd456397a3e2b40fb423ed36f0cabb1847ecb4dR190
	// +kubebuilder:validation:Pattern:="^((?:[1-9][0-9]{0,3}|[1-4][0-9]{4}|[5][0-8][0-9]{3}|59000)Gi|(?:[1-9][0-9]{0,3}|[1-5][0-9]{4}|[6][0-3][0-9]{3}|64000)G|([1-9]||[1-5][0-7]|58)Ti|([1-9]||[1-5][0-9]|6[0-3]|64)T)$"
	// +kubebuilder:validation:XIntOrString
	// +optional
	VolumeSize *resource.Quantity `json:"volumeSize,omitempty"`
	// VolumeType of the block device.
	// For more information, see Amazon EBS volume types (https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html)
	// in the Amazon Elastic Compute Cloud User Guide.
	// +kubebuilder:validation:Enum:={standard,io1,io2,gp2,sc1,st1,gp3}
	// +optional
	VolumeType *string `json:"volumeType,omitempty"`
}

// EC2NodeClass is the Schema for the EC2NodeClass API
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=ec2nodeclasses,scope=Cluster,categories=karpenter,shortName={ec2nc,ec2ncs}
// +kubebuilder:subresource:status
type EC2NodeClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:XValidation:message="amiSelectorTerms is required when amiFamily == 'Custom'",rule="self.amiFamily == 'Custom' ? self.amiSelectorTerms.size() != 0 : true"
	Spec   EC2NodeClassSpec   `json:"spec,omitempty"`
	Status EC2NodeClassStatus `json:"status,omitempty"`

	// IsNodeTemplate tells Karpenter whether the in-memory representation of this object
	// is actually referring to a AWSNodeTemplate object. This value is not actually part of the v1beta1 public-facing API
	// TODO @joinnis: Remove this field when v1alpha5 is unsupported in a future version of Karpenter
	IsNodeTemplate bool `json:"-" hash:"ignore"`
}

func (in *EC2NodeClass) Hash() string {
	return fmt.Sprint(lo.Must(hashstructure.Hash(in.Spec, hashstructure.FormatV2, &hashstructure.HashOptions{
		SlicesAsSets:    true,
		IgnoreZeroValue: true,
		ZeroNil:         true,
	})))
}

// EC2NodeClassList contains a list of EC2NodeClass
// +kubebuilder:object:root=true
type EC2NodeClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EC2NodeClass `json:"items"`
}
