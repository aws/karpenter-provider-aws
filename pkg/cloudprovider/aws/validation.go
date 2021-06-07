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

package aws

import (
	"context"
	"fmt"

	"github.com/awslabs/karpenter/pkg/utils/functional"
	"knative.dev/pkg/apis"
)

// Validate cloud provider specific components of the cluster spec
func (c *Capacity) Validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		c.validateAllowedLabels(),
		c.validateCapacityTypeLabel(),
		c.validateLaunchTemplateLabels(),
	)
}
func (c *Capacity) validateAllowedLabels() (errs *apis.FieldError) {
	for key := range c.provisioner.Spec.Labels {
		if !functional.ContainsString(AllowedLabels, key) {
			errs = errs.Also(apis.ErrInvalidKeyName(key, "spec.labels"))
		}
	}
	return errs
}

func (c *Capacity) validateCapacityTypeLabel() (errs *apis.FieldError) {
	capacityType, ok := c.provisioner.Spec.Labels[CapacityTypeLabel]
	if !ok {
		return nil
	}
	capacityTypes := []string{capacityTypeSpot, capacityTypeOnDemand}
	if !functional.ContainsString(capacityTypes, capacityType) {
		errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s not in %v", capacityType, capacityTypes), fmt.Sprintf("spec.labels[%s]", CapacityTypeLabel)))
	}
	return errs
}

func (c *Capacity) validateLaunchTemplateLabels() (errs *apis.FieldError) {
	if _, versionExists := c.provisioner.Spec.Labels[LaunchTemplateVersionLabel]; versionExists {
		if _, bothExist := c.provisioner.Spec.Labels[LaunchTemplateIdLabel]; !bothExist {
			return errs.Also(apis.ErrMissingField(fmt.Sprintf("spec.labels[%s]", LaunchTemplateIdLabel)))
		}
	}
	return errs
}
