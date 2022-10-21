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
