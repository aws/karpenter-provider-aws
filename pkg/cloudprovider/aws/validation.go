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
	"strings"

	"github.com/awslabs/karpenter/pkg/utils/functional"
)

// Validate cloud provider specific components of the cluster spec
func (c *Capacity) Validate(ctx context.Context) error {
	return functional.ValidateAll(
		c.validateAllowedLabels,
		c.validateCapacityTypeLabel,
		c.validateLaunchTemplateLabels,
	)
}

func (c *Capacity) validateCapacityTypeLabel() error {
	value, ok := c.spec.Labels[CapacityTypeLabel]
	if !ok {
		return nil
	}
	capacityTypes := []string{capacityTypeSpot, capacityTypeOnDemand}
	if !functional.ContainsString(capacityTypes, value) {
		return fmt.Errorf("%s must be one of %v", CapacityTypeLabel, capacityTypes)
	}
	return nil
}

func (c *Capacity) validateAllowedLabels() error {
	for key := range c.spec.Labels {
		if strings.HasPrefix(key, nodeLabelPrefix) &&
			!functional.ContainsString(allowedLabels, key) {
			return fmt.Errorf("%s is reserved for AWS cloud provider use", key)
		}
	}
	return nil
}

func (c *Capacity) validateLaunchTemplateLabels() error {
	if _, versionExists := c.spec.Labels[LaunchTemplateVersionLabel]; versionExists {
		if _, bothExist := c.spec.Labels[LaunchTemplateIdLabel]; !bothExist {
			return fmt.Errorf("%s can only be specified with %s", LaunchTemplateVersionLabel, LaunchTemplateIdLabel)
		}
	}
	return nil
}
