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

type AmazonlinuxOptions struct {
	SimplifiedLaunchTemplateInput
	// The Amazon Linux version, if empty the most recent Amazon Linux version will be used (e.g. "2" or later also "2022", ...).
	// https://aws.amazon.com/linux
	Version *string `json:"version,omitempty"`
}

func (b *AmazonlinuxOptions) Validate(config *Constraints) (errs *apis.FieldError) {
	if b != nil {
		errs = errs.Also(b.SimplifiedLaunchTemplateInput.Validate())
	}
	return errs
}
