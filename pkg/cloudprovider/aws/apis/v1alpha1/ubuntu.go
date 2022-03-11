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

type UbuntuOptions struct {
	SimplifiedLaunchTemplateInput
	// The Ubuntu version, if empty the most recent available Ubuntu LTS version will be used (e.g. "20.04", ...).
	Version *string `json:"version,omitempty"`
}

func (b *UbuntuOptions) Validate(config *Constraints) (errs *apis.FieldError) {
	if b != nil {
		errs = errs.Also(b.SimplifiedLaunchTemplateInput.Validate())
	}
	return errs
}
