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
	"context"
	"fmt"
	"regexp"

	"github.com/samber/lo"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"knative.dev/pkg/apis"

	"github.com/aws/karpenter-core/pkg/utils/functional"
)

const (
	userDataPath    = "userData"
	amiSelectorPath = "amiSelector"
)

var (
	amiRegex = regexp.MustCompile("ami-[0-9a-z]+")
)

func (a *AWSNodeTemplate) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{
		admissionregistrationv1.Create,
		admissionregistrationv1.Update,
	}
}

func (a *AWSNodeTemplate) Validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		apis.ValidateObjectMetadata(a).ViaField("metadata"),
		a.Spec.validate(ctx).ViaField("spec"),
	)
}

func (a *AWSNodeTemplateSpec) validate(_ context.Context) (errs *apis.FieldError) {
	return errs.Also(
		a.AWS.Validate(),
		a.validateUserData(),
		a.validateAMISelector(),
		a.validateAMIFamily(),
		a.validateTags(),
		a.ValidateNetworkInterfaces(),
	)
}

func (a *AWSNodeTemplateSpec) validateUserData() (errs *apis.FieldError) {
	if a.UserData == nil {
		return nil
	}
	if a.LaunchTemplateName != nil {
		errs = errs.Also(apis.ErrMultipleOneOf(userDataPath, launchTemplatePath))
	}
	return errs
}

func (a *AWSNodeTemplateSpec) validateAMIFamily() (errs *apis.FieldError) {
	if a.AMIFamily == nil {
		return nil
	}
	if *a.AMIFamily == AMIFamilyCustom && a.AMISelector == nil {
		errs = errs.Also(apis.ErrMissingField(amiSelectorPath))
	}
	return errs
}

func (a *AWSNodeTemplateSpec) validateAMISelector() (errs *apis.FieldError) {
	if a.AMISelector == nil {
		return nil
	}
	if a.LaunchTemplateName != nil {
		errs = errs.Also(apis.ErrMultipleOneOf(amiSelectorPath, launchTemplatePath))
	}
	for key, value := range a.AMISelector {
		if key == "" || value == "" {
			errs = errs.Also(apis.ErrInvalidValue("\"\"", fmt.Sprintf("%s['%s']", amiSelectorPath, key)))
		}
		if key == "aws-ids" {
			for _, amiID := range functional.SplitCommaSeparatedString(value) {
				if !amiRegex.MatchString(amiID) {
					fieldValue := fmt.Sprintf("\"%s\"", amiID)
					message := fmt.Sprintf("%s['%s'] must be a valid ami-id (regex: %s)", amiSelectorPath, key, amiRegex.String())
					errs = errs.Also(apis.ErrInvalidValue(fieldValue, message))
				}
			}
		}
	}
	return errs
}

func (a *AWSNodeTemplateSpec) validateTags() (errs *apis.FieldError) {
	for k := range a.Tags {
		for _, pattern := range RestrictedTagPatterns {
			if pattern.MatchString(k) {
				errs = errs.Also(apis.ErrInvalidKeyName(k, "tags", fmt.Sprintf("tag contains a restricted tag matching %q", pattern.String())))
			}
		}
	}
	return errs
}

func (a *AWSNodeTemplateSpec) ValidateNetworkInterfaces() (errs *apis.FieldError) {
	if a.NetworkInterfaces != nil {
		if len(a.NetworkInterfaces) > 2 {
			return errs.Also(apis.ErrInvalidValue(len(a.NetworkInterfaces), "networkInterfaces.length", "maximum number of network interfaces supported is 2")) // TODO
		}
		for _, networkInterface := range a.NetworkInterfaces {
			if networkInterface != nil {
				errs = errs.Also(networkInterface.Validate())
			}
		}
	}
	return nil
}
func (n *NetworkInterface) Validate() (errs *apis.FieldError) {
	if n.InterfaceType != nil {
		_, valid := lo.Find([]string{"interface", "efa"}, func(item string) bool {
			return item == *n.InterfaceType
		})
		if !valid {
			return errs.Also(apis.ErrInvalidValue(n.InterfaceType, "interfaceType must be either interface or efa"))
		}
	}
	return nil
}
