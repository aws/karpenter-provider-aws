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

package v1

import (
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EC2NodeClassSpec is the top level specification for the AWS Karpenter Provider.
// This will contain configuration necessary to launch instances in AWS.
type EC2NodeClassSpec struct {
	// SubnetSelectorTerms is a list of subnet selector terms. The terms are ORed.
	// +kubebuilder:validation:XValidation:message="subnetSelectorTerms cannot be empty",rule="self.size() != 0"
	// +kubebuilder:validation:XValidation:message="expected at least one, got none, ['tags', 'id', 'cidr']",rule="self.all(x, has(x.tags) || has(x.id) || has(x.cidr))"
	// +kubebuilder:validation:XValidation:message="'id' is mutually exclusive, cannot be set with a combination of other fields in a subnet selector term",rule="!self.all(x, has(x.id) && has(x.tags))"
	// +kubebuilder:validation:MaxItems:=30
	// +required
	SubnetSelectorTerms []SubnetSelectorTerm `json:"subnetSelectorTerms" hash:"ignore"`
	// SecurityGroupSelectorTerms is a list of security group selector terms. The terms are ORed.
	// +kubebuilder:validation:XValidation:message="securityGroupSelectorTerms cannot be empty",rule="self.size() != 0"
	// +kubebuilder:validation:XValidation:message="expected at least one, got none, ['tags', 'id', 'name']",rule="self.all(x, has(x.tags) || has(x.id) || has(x.name))"
	// +kubebuilder:validation:XValidation:message="'id' is mutually exclusive, cannot be set with a combination of other fields in a security group selector term",rule="!self.all(x, has(x.id) && (has(x.tags) || has(x.name)))"
	// +kubebuilder:validation:XValidation:message="'name' is mutually exclusive, cannot be set with a combination of other fields in a security group selector term",rule="!self.all(x, has(x.name) && (has(x.tags) || has(x.id)))"
	// +kubebuilder:validation:MaxItems:=30
	// +required
	SecurityGroupSelectorTerms []SecurityGroupSelectorTerm `json:"securityGroupSelectorTerms" hash:"ignore"`
	// CapacityReservationSelectorTerms is a list of capacity reservation selector terms. Each term is ORed together to
	// determine the set of eligible capacity reservations.
	// +kubebuilder:validation:XValidation:message="expected at least one, got none, ['tags', 'id', 'instanceMatchCriteria']",rule="self.all(x, has(x.tags) || has(x.id) || has(x.instanceMatchCriteria))"
	// +kubebuilder:validation:XValidation:message="'id' is mutually exclusive, cannot be set along with other fields in a capacity reservation selector term",rule="!self.all(x, has(x.id) && (has(x.tags) || has(x.ownerID) || has(x.instanceMatchCriteria)))"
	// +kubebuilder:validation:MaxItems:=30
	// +optional
	CapacityReservationSelectorTerms []CapacityReservationSelectorTerm `json:"capacityReservationSelectorTerms" hash:"ignore"`
	// AssociatePublicIPAddress controls if public IP addresses are assigned to instances that are launched with the nodeclass.
	// +optional
	AssociatePublicIPAddress *bool `json:"associatePublicIPAddress,omitempty"`
	// IPPrefixCount sets the number of IPv4 prefixes to be automatically assigned to the network interface.
	// +kubebuilder:validation:Minimum:=0
	// +optional
	IPPrefixCount *int32 `json:"ipPrefixCount,omitempty" hash:"ignore"`
	// AMISelectorTerms is a list of or ami selector terms. The terms are ORed.
	// +kubebuilder:validation:XValidation:message="expected at least one, got none, ['tags', 'id', 'name', 'alias', 'ssmParameter']",rule="self.all(x, has(x.tags) || has(x.id) || has(x.name) || has(x.alias) || has(x.ssmParameter))"
	// +kubebuilder:validation:XValidation:message="'id' is mutually exclusive, cannot be set with a combination of other fields in amiSelectorTerms",rule="!self.exists(x, has(x.id) && (has(x.alias) || has(x.tags) || has(x.name) || has(x.owner)))"
	// +kubebuilder:validation:XValidation:message="'alias' is mutually exclusive, cannot be set with a combination of other fields in amiSelectorTerms",rule="!self.exists(x, has(x.alias) && (has(x.id) || has(x.tags) || has(x.name) || has(x.owner)))"
	// +kubebuilder:validation:XValidation:message="'alias' is mutually exclusive, cannot be set with a combination of other amiSelectorTerms",rule="!(self.exists(x, has(x.alias)) && self.size() != 1)"
	// +kubebuilder:validation:MinItems:=1
	// +kubebuilder:validation:MaxItems:=30
	// +required
	AMISelectorTerms []AMISelectorTerm `json:"amiSelectorTerms" hash:"ignore"`
	// AMIFamily dictates the UserData format and default BlockDeviceMappings used when generating launch templates.
	// This field is optional when using an alias amiSelectorTerm, and the value will be inferred from the alias'
	// family. When an alias is specified, this field may only be set to its corresponding family or 'Custom'. If no
	// alias is specified, this field is required.
	// NOTE: We ignore the AMIFamily for hashing here because we hash the AMIFamily dynamically by using the alias using
	// the AMIFamily() helper function
	// +kubebuilder:validation:Enum:={AL2,AL2023,Bottlerocket,Custom,Windows2019,Windows2022}
	// +optional
	AMIFamily *string `json:"amiFamily,omitempty" hash:"ignore"`
	// UserData to be applied to the provisioned nodes.
	// It must be in the appropriate format based on the AMIFamily in use. Karpenter will merge certain fields into
	// this UserData to ensure nodes are being provisioned with the correct configuration.
	// +optional
	UserData *string `json:"userData,omitempty"`
	// Role is the AWS identity that nodes use.
	// This field is mutually exclusive from instanceProfile.
	// +kubebuilder:validation:XValidation:rule="self != ''",message="role cannot be empty"
	// +optional
	Role string `json:"role,omitempty"`
	// InstanceProfile is the AWS entity that instances use.
	// This field is mutually exclusive from role.
	// The instance profile should already have a role assigned to it that Karpenter
	//  has PassRole permission on for instance launch using this instanceProfile to succeed.
	// +kubebuilder:validation:XValidation:rule="self != ''",message="instanceProfile cannot be empty"
	// +optional
	InstanceProfile *string `json:"instanceProfile,omitempty"`
	// Tags to be applied on ec2 resources like instances and launch templates.
	// +kubebuilder:validation:XValidation:message="empty tag keys aren't supported",rule="self.all(k, k != '')"
	// +kubebuilder:validation:XValidation:message="tag contains a restricted tag matching eks:eks-cluster-name",rule="self.all(k, k !='eks:eks-cluster-name')"
	// +kubebuilder:validation:XValidation:message="tag contains a restricted tag matching kubernetes.io/cluster/",rule="self.all(k, !k.startsWith('kubernetes.io/cluster') )"
	// +kubebuilder:validation:XValidation:message="tag contains a restricted tag matching karpenter.sh/nodepool",rule="self.all(k, k != 'karpenter.sh/nodepool')"
	// +kubebuilder:validation:XValidation:message="tag contains a restricted tag matching karpenter.sh/nodeclaim",rule="self.all(k, k !='karpenter.sh/nodeclaim')"
	// +kubebuilder:validation:XValidation:message="tag contains a restricted tag matching karpenter.k8s.aws/ec2nodeclass",rule="self.all(k, k !='karpenter.k8s.aws/ec2nodeclass')"
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
	// Kubelet defines args to be used when configuring kubelet on provisioned nodes.
	// They are a subset of the upstream types, recognizing not all options may be supported.
	// Wherever possible, the types and names should reflect the upstream kubelet types.
	// +kubebuilder:validation:XValidation:message="imageGCHighThresholdPercent must be greater than imageGCLowThresholdPercent",rule="has(self.imageGCHighThresholdPercent) && has(self.imageGCLowThresholdPercent) ?  self.imageGCHighThresholdPercent > self.imageGCLowThresholdPercent  : true"
	// +kubebuilder:validation:XValidation:message="evictionSoft OwnerKey does not have a matching evictionSoftGracePeriod",rule="has(self.evictionSoft) ? self.evictionSoft.all(e, (e in self.evictionSoftGracePeriod)):true"
	// +kubebuilder:validation:XValidation:message="evictionSoftGracePeriod OwnerKey does not have a matching evictionSoft",rule="has(self.evictionSoftGracePeriod) ? self.evictionSoftGracePeriod.all(e, (e in self.evictionSoft)):true"
	// +optional
	Kubelet *KubeletConfiguration `json:"kubelet,omitempty"`
	// BlockDeviceMappings to be applied to provisioned nodes.
	// +kubebuilder:validation:XValidation:message="must have only one blockDeviceMappings with rootVolume",rule="self.filter(x, has(x.rootVolume)?x.rootVolume==true:false).size() <= 1"
	// +kubebuilder:validation:MaxItems:=50
	// +optional
	BlockDeviceMappings []*BlockDeviceMapping `json:"blockDeviceMappings,omitempty"`
	// InstanceStorePolicy specifies how to handle instance-store disks.
	// +optional
	InstanceStorePolicy *InstanceStorePolicy `json:"instanceStorePolicy,omitempty"`
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
	// disabled, with httpPutResponseLimit of 1, and with httpTokens
	// required.
	// +kubebuilder:default={"httpEndpoint":"enabled","httpProtocolIPv6":"disabled","httpPutResponseHopLimit":1,"httpTokens":"required"}
	// +optional
	MetadataOptions *MetadataOptions `json:"metadataOptions,omitempty"`
	// Context is a Reserved field in EC2 APIs
	// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateFleet.html
	// +optional
	Context *string `json:"context,omitempty"`
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
	// CIDR is the CIDR of the subnet
	// +optional
	CIDR string `json:"cidr,omitempty"`
}

// SecurityGroupSelectorTerm defines selection logic for a security group used by Karpenter to launch nodes.
// If multiple fields are used for selection, the requirements are ANDed.
type SecurityGroupSelectorTerm struct {
	// Tags is a map of key/value tags used to select security groups.
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

type CapacityReservationSelectorTerm struct {
	// Tags is a map of key/value tags used to select capacity reservations.
	// Specifying '*' for a value selects all values for a given tag key.
	// +kubebuilder:validation:XValidation:message="empty tag keys or values aren't supported",rule="self.all(k, k != '' && self[k] != '')"
	// +kubebuilder:validation:MaxProperties:=20
	// +optional
	Tags map[string]string `json:"tags,omitempty"`
	// ID is the capacity reservation id in EC2
	// +kubebuilder:validation:Pattern:="^cr-[0-9a-z]+$"
	// +optional
	ID string `json:"id,omitempty"`
	// Owner is the owner id for the ami.
	// +kubebuilder:validation:Pattern:="^[0-9]{12}$"
	// +optional
	OwnerID string `json:"ownerID,omitempty"`
	// InstanceMatchCriteria specifies how instances are matched to capacity reservations.
	// +kubebuilder:validation:Enum:={open,targeted}
	// +optional
	InstanceMatchCriteria string `json:"instanceMatchCriteria,omitempty"`
}

// AMISelectorTerm defines selection logic for an ami used by Karpenter to launch nodes.
// If multiple fields are used for selection, the requirements are ANDed.
type AMISelectorTerm struct {
	// Alias specifies which EKS optimized AMI to select.
	// Each alias consists of a family and an AMI version, specified as "family@version".
	// Valid families include: al2, al2023, bottlerocket, windows2019, and windows2022.
	// The version can either be pinned to a specific AMI release, with that AMIs version format (ex: "al2023@v20240625" or "bottlerocket@v1.10.0").
	// The version can also be set to "latest" for any family. Setting the version to latest will result in drift when a new AMI is released. This is **not** recommended for production environments.
	// Note: The Windows families do **not** support version pinning, and only latest may be used.
	// +kubebuilder:validation:XValidation:message="'alias' is improperly formatted, must match the format 'family@version'",rule="self.matches('^[a-zA-Z0-9]+@.+$')"
	// +kubebuilder:validation:XValidation:message="family is not supported, must be one of the following: 'al2', 'al2023', 'bottlerocket', 'windows2019', 'windows2022'",rule="self.split('@')[0] in ['al2','al2023','bottlerocket','windows2019','windows2022']"
	// +kubebuilder:validation:XValidation:message="windows families may only specify version 'latest'",rule="self.split('@')[0] in ['windows2019','windows2022'] ? self.split('@')[1] == 'latest' : true"
	// +kubebuilder:validation:MaxLength=30
	// +optional
	Alias string `json:"alias,omitempty"`
	// Tags is a map of key/value tags used to select amis.
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
	//SSMParameter is the name (or ARN) of the SSM parameter containing the Image ID.
	// +optional
	SSMParameter string `json:"ssmParameter,omitempty"`
}

// KubeletConfiguration defines args to be used when configuring kubelet on provisioned nodes.
// They are a subset of the upstream types, recognizing not all options may be supported.
// Wherever possible, the types and names should reflect the upstream kubelet types.
// https://pkg.go.dev/k8s.io/kubelet/config/v1beta1#KubeletConfiguration
// https://github.com/kubernetes/kubernetes/blob/9f82d81e55cafdedab619ea25cabf5d42736dacf/cmd/kubelet/app/options/options.go#L53
type KubeletConfiguration struct {
	// clusterDNS is a list of IP addresses for the cluster DNS server.
	// Note that not all providers may use all addresses.
	//+optional
	ClusterDNS []string `json:"clusterDNS,omitempty"`
	// MaxPods is an override for the maximum number of pods that can run on
	// a worker node instance.
	// +kubebuilder:validation:Minimum:=0
	// +optional
	MaxPods *int32 `json:"maxPods,omitempty"`
	// PodsPerCore is an override for the number of pods that can run on a worker node
	// instance based on the number of cpu cores. This value cannot exceed MaxPods, so, if
	// MaxPods is a lower value, that value will be used.
	// +kubebuilder:validation:Minimum:=0
	// +optional
	PodsPerCore *int32 `json:"podsPerCore,omitempty"`
	// SystemReserved contains resources reserved for OS system daemons and kernel memory.
	// +kubebuilder:validation:XValidation:message="valid keys for systemReserved are ['cpu','memory','ephemeral-storage','pid']",rule="self.all(x, x=='cpu' || x=='memory' || x=='ephemeral-storage' || x=='pid')"
	// +kubebuilder:validation:XValidation:message="systemReserved value cannot be a negative resource quantity",rule="self.all(x, !self[x].startsWith('-'))"
	// +optional
	SystemReserved map[string]string `json:"systemReserved,omitempty"`
	// KubeReserved contains resources reserved for Kubernetes system components.
	// +kubebuilder:validation:XValidation:message="valid keys for kubeReserved are ['cpu','memory','ephemeral-storage','pid']",rule="self.all(x, x=='cpu' || x=='memory' || x=='ephemeral-storage' || x=='pid')"
	// +kubebuilder:validation:XValidation:message="kubeReserved value cannot be a negative resource quantity",rule="self.all(x, !self[x].startsWith('-'))"
	// +optional
	KubeReserved map[string]string `json:"kubeReserved,omitempty"`
	// EvictionHard is the map of signal names to quantities that define hard eviction thresholds
	// +kubebuilder:validation:XValidation:message="valid keys for evictionHard are ['memory.available','nodefs.available','nodefs.inodesFree','imagefs.available','imagefs.inodesFree','pid.available']",rule="self.all(x, x in ['memory.available','nodefs.available','nodefs.inodesFree','imagefs.available','imagefs.inodesFree','pid.available'])"
	// +optional
	EvictionHard map[string]string `json:"evictionHard,omitempty"`
	// EvictionSoft is the map of signal names to quantities that define soft eviction thresholds
	// +kubebuilder:validation:XValidation:message="valid keys for evictionSoft are ['memory.available','nodefs.available','nodefs.inodesFree','imagefs.available','imagefs.inodesFree','pid.available']",rule="self.all(x, x in ['memory.available','nodefs.available','nodefs.inodesFree','imagefs.available','imagefs.inodesFree','pid.available'])"
	// +optional
	EvictionSoft map[string]string `json:"evictionSoft,omitempty"`
	// EvictionSoftGracePeriod is the map of signal names to quantities that define grace periods for each eviction signal
	// +kubebuilder:validation:XValidation:message="valid keys for evictionSoftGracePeriod are ['memory.available','nodefs.available','nodefs.inodesFree','imagefs.available','imagefs.inodesFree','pid.available']",rule="self.all(x, x in ['memory.available','nodefs.available','nodefs.inodesFree','imagefs.available','imagefs.inodesFree','pid.available'])"
	// +optional
	EvictionSoftGracePeriod map[string]metav1.Duration `json:"evictionSoftGracePeriod,omitempty"`
	// EvictionMaxPodGracePeriod is the maximum allowed grace period (in seconds) to use when terminating pods in
	// response to soft eviction thresholds being met.
	// +optional
	EvictionMaxPodGracePeriod *int32 `json:"evictionMaxPodGracePeriod,omitempty"`
	// ImageGCHighThresholdPercent is the percent of disk usage after which image
	// garbage collection is always run. The percent is calculated by dividing this
	// field value by 100, so this field must be between 0 and 100, inclusive.
	// When specified, the value must be greater than ImageGCLowThresholdPercent.
	// +kubebuilder:validation:Minimum:=0
	// +kubebuilder:validation:Maximum:=100
	// +optional
	ImageGCHighThresholdPercent *int32 `json:"imageGCHighThresholdPercent,omitempty"`
	// ImageGCLowThresholdPercent is the percent of disk usage before which image
	// garbage collection is never run. Lowest disk usage to garbage collect to.
	// The percent is calculated by dividing this field value by 100,
	// so the field value must be between 0 and 100, inclusive.
	// When specified, the value must be less than imageGCHighThresholdPercent
	// +kubebuilder:validation:Minimum:=0
	// +kubebuilder:validation:Maximum:=100
	// +optional
	ImageGCLowThresholdPercent *int32 `json:"imageGCLowThresholdPercent,omitempty"`
	// CPUCFSQuota enables CPU CFS quota enforcement for containers that specify CPU limits.
	// +optional
	CPUCFSQuota *bool `json:"cpuCFSQuota,omitempty"`
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
	// default value is 1.
	// +kubebuilder:default=1
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
	// +optional
	DeviceName *string `json:"deviceName,omitempty"`
	// EBS contains parameters used to automatically set up EBS volumes when an instance is launched.
	// +kubebuilder:validation:XValidation:message="snapshotID or volumeSize must be defined",rule="has(self.snapshotID) || has(self.volumeSize)"
	// +kubebuilder:validation:XValidation:message="snapshotID must be set when volumeInitializationRate is set",rule="!has(self.volumeInitializationRate) || (has(self.snapshotID) && self.snapshotID != '')"
	// +optional
	EBS *BlockDevice `json:"ebs,omitempty"`
	// RootVolume is a flag indicating if this device is mounted as kubelet root dir. You can
	// configure at most one root volume in BlockDeviceMappings.
	// +optional
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
	// Identifier (key ID, key alias, key ARN, or alias ARN) of the customer managed KMS key to use for EBS encryption.
	// +optional
	KMSKeyID *string `json:"kmsKeyID,omitempty"`
	// SnapshotID is the ID of an EBS snapshot
	// +optional
	SnapshotID *string `json:"snapshotID,omitempty"`
	// Throughput to provision for a gp3 volume, with a maximum of 1,000 MiB/s.
	// Valid Range: Minimum value of 125. Maximum value of 1000.
	// +optional
	Throughput *int64 `json:"throughput,omitempty"`
	// VolumeInitializationRate specifies the Amazon EBS Provisioned Rate for Volume Initialization,
	// in MiB/s, at which to download the snapshot blocks from Amazon S3 to the volume. This is also known as volume
	// initialization. Specifying a volume initialization rate ensures that the volume is initialized at a
	// predictable and consistent rate after creation. Only allowed if SnapshotID is set.
	// Valid Range: Minimum value of 100. Maximum value of 300.
	// +kubebuilder:validation:Minimum:=100
	// +kubebuilder:validation:Maximum:=300
	// +optional
	VolumeInitializationRate *int32 `json:"volumeInitializationRate,omitempty"`
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
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type:=string
	// +optional
	VolumeSize *resource.Quantity `json:"volumeSize,omitempty" hash:"string"`
	// VolumeType of the block device.
	// For more information, see Amazon EBS volume types (https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html)
	// in the Amazon Elastic Compute Cloud User Guide.
	// +kubebuilder:validation:Enum:={standard,io1,io2,gp2,sc1,st1,gp3}
	// +optional
	VolumeType *string `json:"volumeType,omitempty"`
}

// InstanceStorePolicy enumerates options for configuring instance store disks.
// +kubebuilder:validation:Enum={RAID0}
type InstanceStorePolicy string

const (
	// InstanceStorePolicyRAID0 configures a RAID-0 array that includes all ephemeral NVMe instance storage disks.
	// The containerd and kubelet state directories (`/var/lib/containerd` and `/var/lib/kubelet`) will then use the
	// ephemeral storage for more and faster node ephemeral-storage. The node's ephemeral storage can be shared among
	// pods that request ephemeral storage and container images that are downloaded to the node.
	InstanceStorePolicyRAID0 InstanceStorePolicy = "RAID0"
)

// EC2NodeClass is the Schema for the EC2NodeClass API
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Role",type="string",JSONPath=".spec.role",priority=1,description=""
// +kubebuilder:resource:path=ec2nodeclasses,scope=Cluster,categories=karpenter,shortName={ec2nc,ec2ncs}
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
type EC2NodeClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:XValidation:message="must specify exactly one of ['role', 'instanceProfile']",rule="(has(self.role) && !has(self.instanceProfile)) || (!has(self.role) && has(self.instanceProfile))"
	// +kubebuilder:validation:XValidation:message="if set, amiFamily must be 'AL2' or 'Custom' when using an AL2 alias",rule="!has(self.amiFamily) || (self.amiSelectorTerms.exists(x, has(x.alias) && x.alias.find('^[^@]+') == 'al2') ? (self.amiFamily == 'Custom' || self.amiFamily == 'AL2') : true)"
	// +kubebuilder:validation:XValidation:message="if set, amiFamily must be 'AL2023' or 'Custom' when using an AL2023 alias",rule="!has(self.amiFamily) || (self.amiSelectorTerms.exists(x, has(x.alias) && x.alias.find('^[^@]+') == 'al2023') ? (self.amiFamily == 'Custom' || self.amiFamily == 'AL2023') : true)"
	// +kubebuilder:validation:XValidation:message="if set, amiFamily must be 'Bottlerocket' or 'Custom' when using a Bottlerocket alias",rule="!has(self.amiFamily) || (self.amiSelectorTerms.exists(x, has(x.alias) && x.alias.find('^[^@]+') == 'bottlerocket') ? (self.amiFamily == 'Custom' || self.amiFamily == 'Bottlerocket') : true)"
	// +kubebuilder:validation:XValidation:message="if set, amiFamily must be 'Windows2019' or 'Custom' when using a Windows2019 alias",rule="!has(self.amiFamily) || (self.amiSelectorTerms.exists(x, has(x.alias) && x.alias.find('^[^@]+') == 'windows2019') ? (self.amiFamily == 'Custom' || self.amiFamily == 'Windows2019') : true)"
	// +kubebuilder:validation:XValidation:message="if set, amiFamily must be 'Windows2022' or 'Custom' when using a Windows2022 alias",rule="!has(self.amiFamily) || (self.amiSelectorTerms.exists(x, has(x.alias) && x.alias.find('^[^@]+') == 'windows2022') ? (self.amiFamily == 'Custom' || self.amiFamily == 'Windows2022') : true)"
	// +kubebuilder:validation:XValidation:message="must specify amiFamily if amiSelectorTerms does not contain an alias",rule="self.amiSelectorTerms.exists(x, has(x.alias)) ? true : has(self.amiFamily)"
	Spec   EC2NodeClassSpec   `json:"spec,omitempty"`
	Status EC2NodeClassStatus `json:"status,omitempty"`
}

// We need to bump the EC2NodeClassHashVersion when we make an update to the EC2NodeClass CRD under these conditions:
// 1. A field changes its default value for an existing field that is already hashed
// 2. A field is added to the hash calculation with an already-set value
// 3. A field is removed from the hash calculations
const EC2NodeClassHashVersion = "v4"

func (in *EC2NodeClass) Hash() string {
	return fmt.Sprint(lo.Must(hashstructure.Hash([]interface{}{
		in.Spec,
		// AMIFamily should be hashed using the dynamically resolved value rather than the literal value of the field.
		// This ensures that scenarios such as changing the field from nil to AL2023 with the alias "al2023@latest"
		// doesn't trigger drift.
		in.AMIFamily(),
	}, hashstructure.FormatV2, &hashstructure.HashOptions{
		SlicesAsSets:    true,
		IgnoreZeroValue: true,
		ZeroNil:         true,
	})))
}

func (in *EC2NodeClass) LegacyInstanceProfileName(clusterName, region string) string {
	return fmt.Sprintf("%s_%d", clusterName, lo.Must(hashstructure.Hash(fmt.Sprintf("%s%s", region, in.Name), hashstructure.FormatV2, nil)))
}

func (in *EC2NodeClass) InstanceProfileName(clusterName, region string) string {
	return fmt.Sprintf("%s_%d", clusterName, lo.Must(hashstructure.Hash(fmt.Sprintf("%s%s%s", region, in.Name, uuid.New().String()), hashstructure.FormatV2, nil)))
}

func (in *EC2NodeClass) InstanceProfileRole() string {
	return in.Spec.Role
}

func (in *EC2NodeClass) InstanceProfileTags(clusterName string, region string) map[string]string {
	return lo.Assign(in.Spec.Tags, map[string]string{
		fmt.Sprintf("kubernetes.io/cluster/%s", clusterName): "owned",
		EKSClusterNameTagKey:   clusterName,
		LabelNodeClass:         in.Name,
		v1.LabelTopologyRegion: region,
	})
}

func (in *EC2NodeClass) BlockDeviceMappings() []*BlockDeviceMapping {
	return in.Spec.BlockDeviceMappings
}

func (in *EC2NodeClass) InstanceStorePolicy() *InstanceStorePolicy {
	return in.Spec.InstanceStorePolicy
}

func (in *EC2NodeClass) KubeletConfiguration() *KubeletConfiguration {
	return in.Spec.Kubelet
}

// AMIFamily returns the family for a NodePool based on the following items, in order of precdence:
//   - ec2nodeclass.spec.amiFamily
//   - ec2nodeclass.spec.amiSelectorTerms[].alias
//
// If an alias is specified, ec2nodeclass.spec.amiFamily must match that alias, or be 'Custom' (enforced via validation).
func (in *EC2NodeClass) AMIFamily() string {
	if in.Spec.AMIFamily != nil {
		return *in.Spec.AMIFamily
	}
	if alias := in.Alias(); alias != nil {
		return alias.Family
	}
	// Unreachable: validation enforces that one of the above conditions must be met
	return AMIFamilyCustom
}

type Alias struct {
	Family  string
	Version string
}

const (
	AliasVersionLatest = "latest"
)

func (a *Alias) String() string {
	return fmt.Sprintf("%s@%s", a.Family, a.Version)
}

func (in *EC2NodeClass) Alias() *Alias {
	term, ok := lo.Find(in.Spec.AMISelectorTerms, func(term AMISelectorTerm) bool {
		return term.Alias != ""
	})
	if !ok {
		return nil
	}
	return &Alias{
		Family:  amiFamilyFromAlias(term.Alias),
		Version: amiVersionFromAlias(term.Alias),
	}
}

func amiFamilyFromAlias(alias string) string {
	components := strings.Split(alias, "@")
	if len(components) != 2 {
		log.Fatalf("failed to parse AMI alias %q, invalid format", alias)
	}
	family, ok := lo.Find([]string{
		AMIFamilyAL2,
		AMIFamilyAL2023,
		AMIFamilyBottlerocket,
		AMIFamilyWindows2019,
		AMIFamilyWindows2022,
	}, func(family string) bool {
		return strings.ToLower(family) == components[0]
	})
	if !ok {
		log.Fatalf("%q is an invalid alias family", components[0])
	}
	return family
}

func amiVersionFromAlias(alias string) string {
	components := strings.Split(alias, "@")
	if len(components) != 2 {
		log.Fatalf("failed to parse AMI alias %q, invalid format", alias)
	}
	return components[1]
}

// EC2NodeClassList contains a list of EC2NodeClass
// +kubebuilder:object:root=true
type EC2NodeClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EC2NodeClass `json:"items"`
}
