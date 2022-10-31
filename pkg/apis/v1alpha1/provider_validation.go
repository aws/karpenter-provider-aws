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
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/apis"

	"github.com/aws/karpenter-core/pkg/utils/functional"
)

const (
	launchTemplatePath          = "launchTemplate"
	securityGroupSelectorPath   = "securityGroupSelector"
	fieldPathSubnetSelectorPath = "subnetSelector"
	amiFamilyPath               = "amiFamily"
	metadataOptionsPath         = "metadataOptions"
	instanceProfilePath         = "instanceProfile"
	blockDeviceMappingsPath     = "blockDeviceMappings"
)

var (
	minVolumeSize      = *resource.NewScaledQuantity(1, resource.Giga)
	maxVolumeSize      = *resource.NewScaledQuantity(64, resource.Tera)
	subnetRegex        = regexp.MustCompile("subnet-[0-9a-z]+")
	securityGroupRegex = regexp.MustCompile("sg-[0-9a-z]+")
)

func (a *AWS) Validate() (errs *apis.FieldError) {
	return errs.Also(
		a.validate().ViaField("provider"),
	)
}

func (a *AWS) validate() (errs *apis.FieldError) {
	return errs.Also(
		a.validateLaunchTemplate(),
		a.validateSubnets(),
		a.validateSecurityGroups(),
		a.validateTags(),
		a.validateMetadataOptions(),
		a.validateAMIFamily(),
		a.validateBlockDeviceMappings(),
	)
}

func (a *AWS) validateLaunchTemplate() (errs *apis.FieldError) {
	if a.LaunchTemplateName == nil {
		return nil
	}
	if a.SecurityGroupSelector != nil {
		errs = errs.Also(apis.ErrMultipleOneOf(launchTemplatePath, securityGroupSelectorPath))
	}
	if a.MetadataOptions != nil {
		errs = errs.Also(apis.ErrMultipleOneOf(launchTemplatePath, metadataOptionsPath))
	}
	if a.AMIFamily != nil {
		errs = errs.Also(apis.ErrMultipleOneOf(launchTemplatePath, amiFamilyPath))
	}
	if a.InstanceProfile != nil {
		errs = errs.Also(apis.ErrMultipleOneOf(launchTemplatePath, instanceProfilePath))
	}
	if len(a.BlockDeviceMappings) != 0 {
		errs = errs.Also(apis.ErrMultipleOneOf(launchTemplatePath, blockDeviceMappingsPath))
	}
	return errs
}

func (a *AWS) validateSubnets() (errs *apis.FieldError) {
	if a.SubnetSelector == nil {
		errs = errs.Also(apis.ErrMissingField(fieldPathSubnetSelectorPath))
	}
	for key, value := range a.SubnetSelector {
		if key == "" || value == "" {
			errs = errs.Also(apis.ErrInvalidValue("\"\"", fmt.Sprintf("%s['%s']", fieldPathSubnetSelectorPath, key)))
		}
		if key == "aws-ids" {
			for _, subnetID := range functional.SplitCommaSeparatedString(value) {
				if !subnetRegex.MatchString(subnetID) {
					fieldValue := fmt.Sprintf("\"%s\"", subnetID)
					message := fmt.Sprintf("%s['%s'] must be a valid subnet-id (regex: %s)", fieldPathSubnetSelectorPath, key, subnetRegex.String())
					errs = errs.Also(apis.ErrInvalidValue(fieldValue, message))
				}
			}
		}
	}
	return errs
}

func (a *AWS) validateSecurityGroups() (errs *apis.FieldError) {
	if a.LaunchTemplateName != nil {
		return nil
	}
	if a.SecurityGroupSelector == nil {
		errs = errs.Also(apis.ErrMissingField(securityGroupSelectorPath))
	}
	for key, value := range a.SecurityGroupSelector {
		if key == "" || value == "" {
			errs = errs.Also(apis.ErrInvalidValue("\"\"", fmt.Sprintf("%s['%s']", securityGroupSelectorPath, key)))
		}
		if key == "aws-ids" {
			for _, securityGroupID := range functional.SplitCommaSeparatedString(value) {
				if !securityGroupRegex.MatchString(securityGroupID) {
					fieldValue := fmt.Sprintf("\"%s\"", securityGroupID)
					message := fmt.Sprintf("%s['%s'] must be a valid group-id (regex: %s)", securityGroupSelectorPath, key, securityGroupRegex.String())
					errs = errs.Also(apis.ErrInvalidValue(fieldValue, message))
				}
			}
		}
	}
	return errs
}

func (a *AWS) validateTags() (errs *apis.FieldError) {
	// Avoiding a check on number of tags (hard limit of 50) since that limit is shared by user
	// defined and Karpenter tags, and the latter could change over time.
	for tagKey, tagValue := range a.Tags {
		if tagKey == "" {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf(
				"the tag with key : '' and value : '%s' is invalid because empty tag keys aren't supported", tagValue), "tags"))
		}
	}
	return errs
}

func (a *AWS) validateMetadataOptions() (errs *apis.FieldError) {
	if a.MetadataOptions == nil {
		return nil
	}
	return errs.Also(
		a.validateHTTPEndpoint(),
		a.validateHTTPProtocolIpv6(),
		a.validateHTTPPutResponseHopLimit(),
		a.validateHTTPTokens(),
	).ViaField(metadataOptionsPath)
}

func (a *AWS) validateHTTPEndpoint() *apis.FieldError {
	if a.MetadataOptions.HTTPEndpoint == nil {
		return nil
	}
	return a.validateStringEnum(*a.MetadataOptions.HTTPEndpoint, "httpEndpoint", ec2.LaunchTemplateInstanceMetadataEndpointState_Values())
}

func (a *AWS) validateHTTPProtocolIpv6() *apis.FieldError {
	if a.MetadataOptions.HTTPProtocolIPv6 == nil {
		return nil
	}
	return a.validateStringEnum(*a.MetadataOptions.HTTPProtocolIPv6, "httpProtocolIPv6", ec2.LaunchTemplateInstanceMetadataProtocolIpv6_Values())
}

func (a *AWS) validateHTTPPutResponseHopLimit() *apis.FieldError {
	if a.MetadataOptions.HTTPPutResponseHopLimit == nil {
		return nil
	}
	limit := *a.MetadataOptions.HTTPPutResponseHopLimit
	if limit < 1 || limit > 64 {
		return apis.ErrOutOfBoundsValue(limit, 1, 64, "httpPutResponseHopLimit")
	}
	return nil
}

func (a *AWS) validateHTTPTokens() *apis.FieldError {
	if a.MetadataOptions.HTTPTokens == nil {
		return nil
	}
	return a.validateStringEnum(*a.MetadataOptions.HTTPTokens, "httpTokens", ec2.LaunchTemplateHttpTokensState_Values())
}

func (a *AWS) validateAMIFamily() *apis.FieldError {
	if a.AMIFamily == nil {
		return nil
	}
	return a.validateStringEnum(*a.AMIFamily, amiFamilyPath, SupportedAMIFamilies)
}

func (a *AWS) validateStringEnum(value, field string, validValues []string) *apis.FieldError {
	for _, validValue := range validValues {
		if value == validValue {
			return nil
		}
	}
	return apis.ErrInvalidValue(fmt.Sprintf("%s not in %v", value, strings.Join(validValues, ", ")), field)
}

func (a *AWS) validateBlockDeviceMappings() (errs *apis.FieldError) {
	for i, blockDeviceMapping := range a.BlockDeviceMappings {
		if err := a.validateBlockDeviceMapping(blockDeviceMapping); err != nil {
			errs = errs.Also(err.ViaFieldIndex(blockDeviceMappingsPath, i))
		}
	}
	return errs
}

func (a *AWS) validateBlockDeviceMapping(blockDeviceMapping *BlockDeviceMapping) (errs *apis.FieldError) {
	return errs.Also(a.validateDeviceName(blockDeviceMapping), a.validateEBS(blockDeviceMapping))
}

func (a *AWS) validateDeviceName(blockDeviceMapping *BlockDeviceMapping) *apis.FieldError {
	if blockDeviceMapping.DeviceName == nil {
		return apis.ErrMissingField("deviceName")
	}
	return nil
}

func (a *AWS) validateEBS(blockDeviceMapping *BlockDeviceMapping) (errs *apis.FieldError) {
	if blockDeviceMapping.EBS == nil {
		return apis.ErrMissingField("ebs")
	}
	for _, err := range []*apis.FieldError{
		a.validateVolumeType(blockDeviceMapping),
		a.validateVolumeSize(blockDeviceMapping),
	} {
		if err != nil {
			errs = errs.Also(err.ViaField("ebs"))
		}
	}
	return errs
}

func (a *AWS) validateVolumeType(blockDeviceMapping *BlockDeviceMapping) *apis.FieldError {
	if blockDeviceMapping.EBS.VolumeType != nil {
		return a.validateStringEnum(*blockDeviceMapping.EBS.VolumeType, "volumeType", ec2.VolumeType_Values())
	}
	return nil
}

func (a *AWS) validateVolumeSize(blockDeviceMapping *BlockDeviceMapping) *apis.FieldError {
	// if an EBS mapping is present, one of volumeSize or snapshotID must be present
	if blockDeviceMapping.EBS.SnapshotID != nil && blockDeviceMapping.EBS.VolumeSize == nil {
		return nil
	} else if blockDeviceMapping.EBS.VolumeSize == nil {
		return apis.ErrMissingField("volumeSize")
	} else if blockDeviceMapping.EBS.VolumeSize.Cmp(minVolumeSize) == -1 || blockDeviceMapping.EBS.VolumeSize.Cmp(maxVolumeSize) == 1 {
		return apis.ErrOutOfBoundsValue(blockDeviceMapping.EBS.VolumeSize.String(), minVolumeSize.String(), maxVolumeSize.String(), "volumeSize")
	}
	return nil
}
