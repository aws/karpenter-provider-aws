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
	"text/template"

	"knative.dev/pkg/apis"
)

type GenericOptions struct {
	GenericLaunchTemplateInput
	// VerbatimUserData set to `true` to use userData verbatim, not interpreting it as golang template.
	// +optional
	VerbatimUserData bool `json:"verbatimUserData,omitempty"`
}

func (b *GenericOptions) Validate(constraints *Constraints) (errs *apis.FieldError) {
	if b != nil {
		errs = errs.Also(
			b.GenericLaunchTemplateInput.Validate(),
			b.validateUserData(),
		)
	} else {
		errs = errs.Also(
			apis.ErrMissingField(imageIDPath),
		)
	}
	return
}

func (b *GenericOptions) validateUserData() (errs *apis.FieldError) {
	_, err := b.ParseTemplate()
	if err != nil {
		errs = errs.Also(apis.ErrInvalidValue(*b.UserData, "userData", fmt.Sprintf("%s", err)))
	}
	return errs
}

func (b *GenericOptions) ParseTemplate() (*template.Template, error) {
	templateInput := b.GenericLaunchTemplateInput.UserData
	if b.VerbatimUserData || templateInput == nil {
		return nil, nil
	}
	t, err := template.New("userData").Parse(*templateInput)
	if err != nil {
		return nil, fmt.Errorf("parsing userData template, %w", err)
	}
	return t, nil
}
