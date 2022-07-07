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

	"knative.dev/pkg/apis"
)

const (
	launchTemplatePath = "launchTemplate"
	userDataPath       = "userData"
)

func (a *AWSNodeTemplate) Validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		apis.ValidateObjectMetadata(a).ViaField("metadata"),
		a.Spec.validate(ctx).ViaField("spec"),
	)
}

func (a *AWSNodeTemplateSpec) validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		a.AWS.Validate(),
		a.validateUserData(),
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
