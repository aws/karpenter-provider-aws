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
	"github.com/pkg/errors"
)

// +kubebuilder:object:generate=false
type ScalableNodeGroupValidator func(*ScalableNodeGroup) error

var scalableNodeGroupValidators []ScalableNodeGroupValidator

func RegisterScalableNodeGroupValidator(validator ScalableNodeGroupValidator) {
	scalableNodeGroupValidators = append(scalableNodeGroupValidators, validator)
}

func (sng *ScalableNodeGroup) Validate() error {
	for _, validator := range scalableNodeGroupValidators {
		if err := validator(sng); err != nil {
			// TODO(jacob): make this error more informative
			return errors.Wrap(err, "invalid ScalableNodeGroup")
		}
	}
	return nil
}

// TODO(jacob) put cloudprovider-accessible validation hook here?
