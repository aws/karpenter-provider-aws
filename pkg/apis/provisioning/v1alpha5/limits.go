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

package v1alpha5

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
)

// Limits define bounds on the resources being provisioned by Karpenter
type Limits struct {
	// Resources contains all the allocatable resources that Karpenter supports for limiting.
	Resources v1.ResourceList `json:"resources,omitempty"`
}

func (p *Provisioner) HasExceededResources() error {
	var currentResource = p.Status.Resources
	var currentLimits = p.Spec.Limits.Resources

	for resourceName, usage := range currentResource {
		fmt.Printf("resourceName %v resourceUsage %v\n", resourceName, usage)
		if limit, ok := currentLimits[resourceName]; ok {
			fmt.Printf("limitName %v limitUsage %v\n", resourceName, limit)
			if usage.Cmp(limit) >= 0 {
				return fmt.Errorf("%v limits exceeded", resourceName)
			}
		}
	}
	return nil
}
