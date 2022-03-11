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
	"strings"

	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
	"knative.dev/pkg/apis"
)

const (
	imageIDPath = "imageId"
)

// The parameters for a block device for an EBS volume.
//
// Notes:
// - Encryption is on by default and cannot be disable (thus we do not provide/expose the Encrypted field).
// - DeleteOnTermination is not exposed and is by default set to true.
type EbsVolume struct {

	// The number of I/O operations per second (IOPS). For gp3, io1, and io2 volumes,
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
	Iops *int64 `json:"iops,omitempty"`

	// The throughput to provision for a gp3 volume, with a maximum of 1,000 MiB/s.
	//
	// Valid Range: Minimum value of 125. Maximum value of 1000.
	// +optional
	Throughput *int64 `json:"throughput,omitempty"`

	// The size of the volume, in GiBs. You must specify either a snapshot ID or
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
	VolumeSize *resource.Quantity `json:"volumeSize,omitempty"`

	// The volume type. For more information, see Amazon EBS volume types (https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html)
	// in the Amazon Elastic Compute Cloud User Guide.
	// +optional
	VolumeType *string `json:"volumeType,omitempty"`

	// The ARN of the symmetric Key Management Service (KMS) CMK used for encryption.
	//
	// Encryption is enabled by default and cannot be disabled. Users with additional
	// security/compliance requirements (e.g. crypto-shredding) can provide a KMS Key.
	// +optional
	KmsKeyID *string `json:"kmsKeyId,omitempty"`
}

func ensureRange(field string, min, max int64, value *int64) (errs *apis.FieldError) {
	if value != nil && (*value < min || *value > max) {
		errs = errs.Also(apis.ErrOutOfBoundsValue(*value, min, max, field))
	}
	return
}

func (v *EbsVolume) validateVolumeType() (errs *apis.FieldError) {
	if v == nil || v.VolumeType == nil {
		return nil
	}
	errs = errs.Also(validateStringEnum(*v.VolumeType, "volumeType", ec2.VolumeType_Values()))
	return
}

func (v *EbsVolume) validateVolumeSize() (errs *apis.FieldError) {
	if v == nil || v.VolumeSize == nil {
		return nil
	}
	const field = "volumeSize"
	volumeSize := v.VolumeSize.ScaledValue(resource.Giga)
	tpe := pointer.StringPtrDerefOr(v.VolumeType, "")
	switch tpe {
	case ec2.VolumeTypeGp3:
		fallthrough
	case ec2.VolumeTypeGp2:
		errs = errs.Also(ensureRange(field, 1, 16384, &volumeSize))
	case ec2.VolumeTypeIo1:
		fallthrough
	case ec2.VolumeTypeIo2:
		errs = errs.Also(ensureRange(field, 3, 16384, &volumeSize))
	case ec2.VolumeTypeSt1:
		fallthrough
	case ec2.VolumeTypeSc1:
		errs = errs.Also(ensureRange(field, 125, 16384, &volumeSize))
	case ec2.VolumeTypeStandard:
		errs = errs.Also(ensureRange(field, 1, 1024, &volumeSize))
	}
	return
}

func (v *EbsVolume) validateIops() (errs *apis.FieldError) {
	if v == nil {
		return nil
	}
	const iops = "iops"
	tpe := pointer.StringPtrDerefOr(v.VolumeType, "")
	switch tpe {
	case "gp3":
		errs = errs.Also(ensureRange(iops, 3000, 16000, v.Iops))
	case "io1":
		errs = errs.Also(ensureRange(iops, 100, 64000, v.Iops))
	case "io2":
		errs = errs.Also(ensureRange(iops, 100, 64000, v.Iops))
	default:
		if v.Iops != nil {
			errs = errs.Also(apis.ErrGeneric("iops can only be set if volumeType is one of 'gp3', 'io1' or 'io2'", iops))
		}
	}
	return
}

func (v *EbsVolume) validateThroughput() (errs *apis.FieldError) {
	if v == nil {
		return nil
	}
	const field = "throughput"
	tpe := pointer.StringPtrDerefOr(v.VolumeType, "")
	switch tpe {
	case "gp3":
		errs = errs.Also(ensureRange(field, 125, 1000, v.Throughput))
	default:
		if v.Iops != nil {
			errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("%s can only be set if volumeType is one of 'gp3', 'io1' or 'io2'", field), field))
		}
	}
	return
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
	HTTPProtocolIpv6 *string `json:"httpProtocolIPv6,omitempty"`

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

// BlockDeviceMapping used to (optionally) map additional block devices.
type BlockDeviceMapping struct {
	// The device name (for example, /dev/sdh or xvdh).
	// +optional
	DeviceName *string `json:"deviceName,omitempty"`

	// Parameters used to automatically set up EBS volumes when the instance is
	// launched.
	// +optional
	Ebs *EbsVolume `json:"ebs,omitempty"`
}

// BasicLaunchTemplateInput common among all providers.
type BasicLaunchTemplateInput struct {

	// SecurityGroupSelector specify the names of the security groups.
	// +optional
	SecurityGroupSelector map[string]string `json:"securityGroupSelector,omitempty"`

	// InstanceProfile name or Amazon Resource Name (ARN) of an IAM instance profile.
	// +optional
	InstanceProfile *string `json:"instanceProfile,omitempty"`

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

	// Indicates whether the instance is optimized for Amazon EBS I/O. This optimization
	// provides dedicated throughput to Amazon EBS and an optimized configuration
	// stack to provide optimal Amazon EBS I/O performance. This optimization isn't
	// available with all instance types. Additional usage charges apply when using
	// an EBS-optimized instance.
	// +optional
	EbsOptimized *bool `json:"ebsOptimized,omitempty"`

	// The number of threads per CPU core. To disable multithreading for the instance,
	// specify a value of 1. Otherwise, specify the default value of 2.
	// +optional
	ThreadsPerCore *int64 `json:"threadsPerCore,omitempty"`

	// The credit option for CPU usage of a T2, T3, or T3a instance. Valid values
	// are standard and unlimited.
	// +optional
	CPUCredits *string `json:"cpuCredits,omitempty"`

	// RootVolume EBS volume configuration for the root volume, used to override
	// configuration inherited from the AMI. Can be used to adjust the capacity
	// (size, throughput and IOPS) of the root volume.
	// +optional
	RootVolume *EbsVolume `json:"rootVolume,omitempty"`

	// ExtraBlockDevices additional block device mapping configurations to apply
	// in addition to the one of the root volume.
	//
	// Users which have special needs (e.g. compliance) can add additional block
	// devices. However, depending on the underlying AMI those volumes might not
	// be mounted or formatted automatically and it is up to the AMI or userData
	// to take care of mounting and formatting them.
	// +optional
	ExtraBlockDevices []BlockDeviceMapping `json:"extraBlockDevices,omitempty"`
}

// SimplifiedLaunchTemplateInput for opinionated providers.
type SimplifiedLaunchTemplateInput struct {
	BasicLaunchTemplateInput
	// ImageID the ID of the AMI.
	// +optional
	ImageID *string `json:"imageId,omitempty"`
}

// GenericLaunchTemplateInput for fully generic providers.
type GenericLaunchTemplateInput struct {
	BasicLaunchTemplateInput
	// ImageID the ID of the AMI.
	ImageID string `json:"imageId"`

	// The user data to make available to the instance. You must provide base64-encoded
	// text. User data is limited to 16 KB. For more information, see Running Commands
	// on Your Linux Instance at Launch (https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/user-data.html)
	// (Linux) or Adding User Data (https://docs.aws.amazon.com/AWSEC2/latest/WindowsGuide/ec2-instance-metadata.html#instancedata-add-user-data)
	// (Windows).
	//
	// If you are creating the launch template for use with Batch, the user data
	// must be provided in the MIME multi-part archive format (https://cloudinit.readthedocs.io/en/latest/topics/format.html#mime-multi-part-archive).
	// For more information, see Amazon EC2 user data in launch templates (https://docs.aws.amazon.com/batch/latest/userguide/launch-templates.html)
	// in the Batch User Guide.
	// +optional
	UserData *string `json:"userData,omitempty"`
}

func (v *EbsVolume) IsEmpty() bool {
	return v == nil || (v.Iops == nil && v.KmsKeyID == nil && v.Throughput == nil && v.VolumeSize == nil && v.VolumeType == nil)
}

func (v *EbsVolume) Validate() (errs *apis.FieldError) {
	if v == nil {
		errs = errs.Also(
			v.validateIops(),
			v.validateVolumeSize(),
			v.validateVolumeType(),
			v.validateThroughput(),
		)
	}
	return errs
}

func (g *BlockDeviceMapping) Validate() (errs *apis.FieldError) {
	if g == nil {
		return errs
	}
	errs = errs.Also(g.Ebs.Validate().ViaField("ebs"))
	return errs
}

func (g *BasicLaunchTemplateInput) Validate() (errs *apis.FieldError) {
	if g == nil {
		return nil
	}
	errs = errs.Also(g.MetadataOptions.Validate().ViaField("metadataOptions"))
	errs = errs.Also(validateStringEnum(*g.CPUCredits, "cpuCredits", []string{"standard", "unlimited"}))
	errs = errs.Also(g.RootVolume.Validate())
	if g.SecurityGroupSelector == nil {
		errs = errs.Also(apis.ErrMissingField(securityGroupSelectorPath))
	}
	for key, value := range g.SecurityGroupSelector {
		if key == "" || value == "" {
			errs = errs.Also(apis.ErrInvalidValue("\"\"", fmt.Sprintf("%s['%s']", securityGroupSelectorPath, key)))
		}
	}
	if len(g.ExtraBlockDevices) > 0 {
		for idx, mapping := range g.ExtraBlockDevices {
			errs = errs.Also(mapping.Validate().ViaFieldIndex("extraBlockDevices", idx))
		}
	}
	return errs
}

func (g *SimplifiedLaunchTemplateInput) Validate() (errs *apis.FieldError) {
	errs = errs.Also(g.BasicLaunchTemplateInput.Validate())
	return errs
}

func (g *GenericLaunchTemplateInput) Validate() (errs *apis.FieldError) {
	if g == nil || len(g.ImageID) == 0 {
		errs = errs.Also(apis.ErrMissingField(imageIDPath))
	}
	return errs.Also(g.BasicLaunchTemplateInput.Validate())
}

func (m *MetadataOptions) Validate() (errs *apis.FieldError) {
	if m == nil {
		return nil
	}
	if m.HTTPEndpoint != nil {
		errs = errs.Also(
			validateStringEnum(*m.HTTPEndpoint, "httpEndpoint", ec2.LaunchTemplateInstanceMetadataEndpointState_Values()),
		)
	}
	if m.HTTPProtocolIpv6 != nil {
		errs = errs.Also(
			validateStringEnum(*m.HTTPProtocolIpv6, "httpProtocolIpv6", ec2.LaunchTemplateInstanceMetadataProtocolIpv6_Values()),
		)
	}
	if m.HTTPTokens != nil {
		errs = errs.Also(
			validateStringEnum(*m.HTTPTokens, "httpTokens", ec2.LaunchTemplateHttpTokensState_Values()),
		)
	}
	if m.HTTPPutResponseHopLimit != nil {
		if *m.HTTPPutResponseHopLimit < 1 || *m.HTTPPutResponseHopLimit > 64 {
			return apis.ErrOutOfBoundsValue(*m.HTTPPutResponseHopLimit, 1, 64, "httpPutResponseHopLimit")
		}
	}
	return errs
}

func validateStringEnum(value, field string, validValues []string) *apis.FieldError {
	for _, validValue := range validValues {
		if value == validValue {
			return nil
		}
	}
	return apis.ErrInvalidValue(fmt.Sprintf("%s not in %v", value, strings.Join(validValues, ", ")), field)
}

func (m *MetadataOptions) WithDefaults() *ec2.LaunchTemplateInstanceMetadataOptionsRequest {
	options := &ec2.LaunchTemplateInstanceMetadataOptionsRequest{
		// Instance Metadata Service (IMDS) must be enabled otherwise basic cluster services
		// and add-ons (e.g. CNI, CSI, ...) will not work.
		HttpEndpoint: pointer.String(ec2.LaunchTemplateInstanceMetadataEndpointStateEnabled),
		// Enabling IPv6 support doesn't hurt, and makes sure that IPv6 only clusters will work
		// out of the box.
		HttpProtocolIpv6: pointer.String(ec2.LaunchTemplateInstanceMetadataProtocolIpv6Enabled),
		// Setting HopLimit to 1 is a recommended security best practice to prevent (non host network)
		// Pods from accessing the Instance Metadata Server (IMDS) or assuming the nodes instance profile.
		HttpPutResponseHopLimit: pointer.Int64(1),
		// Enforcing Instance Metadata Service Version 2 (IMDSv2) is a recommended security best practice.
		// See https://docs.aws.amazon.com/securityhub/latest/userguide/securityhub-standards-fsbp-controls.html#ec2-8-remediation.
		HttpTokens: pointer.String(ec2.LaunchTemplateHttpTokensStateRequired),
		// Enabling access to EC2 instance tags via Instance Metadata Service (IMDS) reduces load on EC2
		// API and thus enables large clusters without running into API rate limits.
		InstanceMetadataTags: pointer.String(ec2.LaunchTemplateInstanceMetadataTagsStateEnabled),
	}
	if m != nil {
		if m.HTTPEndpoint != nil {
			options.HttpEndpoint = m.HTTPEndpoint
		}
		if m.HTTPProtocolIpv6 != nil {
			options.HttpProtocolIpv6 = m.HTTPProtocolIpv6
		}
		if m.HTTPPutResponseHopLimit != nil {
			options.HttpPutResponseHopLimit = m.HTTPPutResponseHopLimit
		}
		if m.HTTPTokens != nil {
			options.HttpTokens = m.HTTPTokens
		}
	}
	return options
}
