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

	"knative.dev/pkg/apis"
)

const (
	securityGroupSelectorPath   = "securityGroupSelector"
	fieldPathSubnetSelectorPath = "subnetSelector"
)

func (a *AWS) Validate(constraints *Constraints) (errs *apis.FieldError) {
	if a != nil {
		return a.validate(constraints).ViaField("provider")
	}
	return
}

func (a *AWS) validate(constraints *Constraints) (errs *apis.FieldError) {
	return errs.Also(
		a.validateOSProvider(constraints),
		a.validateSubnets(),
		a.validateTags(),
	)
}

func (a *AWS) validateTags() (errs *apis.FieldError) {
	for tagKey, tagValue := range a.Tags {
		if tagKey == "" {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf(
				"the tag with key : '' and value : '%s' is invalid because empty tag keys aren't supported", tagValue), "tags"))
		}
	}
	return errs
}

func (a *AWS) validateOSProvider(constraints *Constraints) (errs *apis.FieldError) {
	c := 0
	if a.Amazonlinux != nil {
		errs = errs.Also(a.Amazonlinux.Validate(constraints).ViaField(AMIFamilyAmazonlinux))
		c++
	}
	if a.Bottlerocket != nil {
		errs = errs.Also(a.Bottlerocket.Validate(constraints).ViaField(AMIFamilyBottlerocket))
		c++
	}
	if a.Ubuntu != nil {
		errs = errs.Also(a.Ubuntu.Validate(constraints).ViaField(AMIFamilyUbuntu))
		c++
	}
	if a.Generic != nil {
		errs = errs.Also(a.Generic.Validate(constraints).ViaField(AMIFamilyGeneric))
		c++
	}
	if a.Predefined != nil {
		errs = errs.Also(a.Predefined.Validate(constraints).ViaField(AMIFamilyPredefined))
		c++
	}
	if c > 1 {
		errs = errs.Also(apis.ErrMultipleOneOf(AMIFamilyAmazonlinux, AMIFamilyBottlerocket, AMIFamilyUbuntu, AMIFamilyGeneric, AMIFamilyPredefined))
	}
	return
}

func (a *AWS) validateSubnets() (errs *apis.FieldError) {
	if a.SubnetSelector == nil {
		errs = errs.Also(apis.ErrMissingField(fieldPathSubnetSelectorPath))
	}
	for key, value := range a.SubnetSelector {
		if key == "" || value == "" {
			errs = errs.Also(apis.ErrInvalidValue("\"\"", fmt.Sprintf("%s['%s']", fieldPathSubnetSelectorPath, key)))
		}
	}
	return errs
}
