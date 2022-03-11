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
	"knative.dev/pkg/apis"
)

type LauchtemplateReference struct {
	// LaunchTemplate name or ID of a predefined (already existing) EC2 LaunchTemplate.
	LaunchTemplateID *string `json:"launchTemplateId,omitempty"`
	// LaunchTemplate name or ID of a predefined (already existing) EC2 LaunchTemplate.
	LaunchTemplateName *string `json:"launchTemplateName,omitempty"`
	// LaunchTemplate version.
	//+optional
	Version *string `json:"version,omitempty"`
}

func (l *LauchtemplateReference) GetVersion() string {
	if l.Version != nil {
		return *l.Version
	}
	return "$Latest"
}

type PredefinedOptions struct {
	LauchtemplateReference
}

func (b *PredefinedOptions) Validate(constraints *Constraints) (errs *apis.FieldError) {
	if b != nil {
		if b.LaunchTemplateID != nil && b.LaunchTemplateName != nil {
			errs = errs.Also(
				apis.ErrMultipleOneOf("launchTemplateId", "launchTemplateName"),
			)
		}
		if b.LaunchTemplateID == nil && b.LaunchTemplateName == nil {
			errs = errs.Also(
				apis.ErrMissingOneOf("launchTemplateId", "launchTemplateName"),
			)
		}
	}
	return errs
}
