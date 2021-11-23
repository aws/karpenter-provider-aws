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
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	// CPU limit, in cores. (500m = .5 cores)
	ResourceLimitsCPU v1.ResourceName = "cpu"
	// Memory limit, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
	ResourceLimitsMemory v1.ResourceName = "memory"
)

var DefaultCPULimits *resource.Quantity = resource.NewScaledQuantity(100, 0)
var DefaultMemoryLimits *resource.Quantity = resource.NewScaledQuantity(400, resource.Giga)

// Limits define bounds on the resources being provisioned by Karpenter
type Limits struct {
	// Resources contains all the allocatable resources that Karpenter supports for limiting.
	Resources v1.ResourceList `json:"resources,omitempty"`
}

func (l *Limits) Default(ctx context.Context) {
	if l.Resources == nil {
		l.Resources = v1.ResourceList{
			ResourceLimitsCPU:    *DefaultCPULimits,
			ResourceLimitsMemory: *DefaultMemoryLimits,
		}
		return
	}
	if _, ok := l.Resources[ResourceLimitsCPU]; !ok {
		l.Resources[ResourceLimitsCPU] = *DefaultCPULimits
	}
	if _, ok := l.Resources[ResourceLimitsMemory]; !ok {
		l.Resources[ResourceLimitsMemory] = *DefaultMemoryLimits
	}
}

func (p *Provisioner) HasExceededResources() (bool, error) {
	var currentResource = p.Status.Resources
	var currentLimits = p.Spec.Limits.Resources

	var CPUUsage = currentResource[ResourceLimitsCPU]
	if CPUUsage.Cmp(currentLimits[ResourceLimitsCPU]) >= 0 {
		return true, fmt.Errorf("cpu limits exceeded")
	}

	var MemoryUsage = currentResource[ResourceLimitsCPU]
	if MemoryUsage.Cmp(currentLimits[ResourceLimitsCPU]) >= 0 {
		return true, fmt.Errorf("memory limits exceeded")
	}
	return false, nil
}
