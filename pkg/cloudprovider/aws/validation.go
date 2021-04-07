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
	return c.validateLabels()
}

func (c *Capacity) validateLabels() error {
	hasLaunchTemplateId := false
	hasLaunchTemplateVersion := false
	for key, value := range c.spec.Labels {
		if strings.HasPrefix(key, nodeLabelPrefix) {
			switch key {
			case capacityTypeLabel:
				capacityTypes := []string{capacityTypeSpot, capacityTypeOnDemand}
				if !functional.ContainsString(capacityTypes, value) {
					return fmt.Errorf("%s must be one of %v", key, capacityTypes)
				}
			case launchTemplateIdLabel:
				hasLaunchTemplateId = true
			case launchTemplateVersionLabel:
				hasLaunchTemplateVersion = true
			default:
				return fmt.Errorf("%s is reserved for AWS cloud provider use", key)
			}
		}
	}
	if hasLaunchTemplateVersion && !hasLaunchTemplateId {
		return fmt.Errorf("%s can only be specified with %s", launchTemplateVersionLabel, launchTemplateIdLabel)
	}
	return nil
}
