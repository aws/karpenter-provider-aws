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
	"knative.dev/pkg/apis"
)

func (a *AWS) Validate() (errs *apis.FieldError) {
	return a.validate().ViaField("provider")
}

func (a *AWS) validate() (errs *apis.FieldError) {
	return errs.Also(
		a.validateLaunchTemplate(),
		a.validateSubnets(),
		a.validateSecurityGroups(),
		a.validateTags(),
		a.validateMetadataOptions(),
	)
}

func (a *AWS) validateLaunchTemplate() (errs *apis.FieldError) {
	// nothing to validate at the moment
	return errs
}

func (a *AWS) validateSubnets() (errs *apis.FieldError) {
	if a.SubnetSelector == nil {
		errs = errs.Also(apis.ErrMissingField("subnetSelector"))
	}
	for key, value := range a.SubnetSelector {
		if key == "" || value == "" {
			errs = errs.Also(apis.ErrInvalidValue("\"\"", fmt.Sprintf("subnetSelector['%s']", key)))
		}
	}
	return errs
}

func (a *AWS) validateSecurityGroups() (errs *apis.FieldError) {
	if a.SecurityGroupSelector == nil {
		errs = errs.Also(apis.ErrMissingField("securityGroupSelector"))
	}
	for key, value := range a.SecurityGroupSelector {
		if key == "" || value == "" {
			errs = errs.Also(apis.ErrInvalidValue("\"\"", fmt.Sprintf("securityGroupSelector['%s']", key)))
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
	).ViaField("metadataOptions")
}

func (a *AWS) validateHTTPEndpoint() (errs *apis.FieldError) {
	if a.MetadataOptions.HTTPEndpoint == nil {
		return nil
	}
	return a.validateStringEnum(*a.MetadataOptions.HTTPEndpoint, "httpEndpoint", ec2.LaunchTemplateInstanceMetadataEndpointState_Values())
}

func (a *AWS) validateHTTPProtocolIpv6() (errs *apis.FieldError) {
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

func (a *AWS) validateStringEnum(value, field string, validValues []string) *apis.FieldError {
	for _, validValue := range validValues {
		if value == validValue {
			return nil
		}
	}
	return apis.ErrInvalidValue(fmt.Sprintf("valid values: %s", strings.Join(validValues, ", ")), field)
}
