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

package scheduling

import (
	"context"
	"regexp"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
)

// instanceTypeFilter is a compiled instance type filter based on the v1alpha5.InstanceTypeFilter from the provisioner
// spec
type instanceTypeFilter struct {
	minCPU               *resource.Quantity
	maxCPU               *resource.Quantity
	minMemory            *resource.Quantity
	maxMemory            *resource.Quantity
	minMemoryMiBPerCPU   *int64
	maxMemoryMiBPerCPU   *int64
	nameMatchExpressions []*regexp.Regexp
}

// newInstanceTypeFilter constructs a new compiled instance type filter.  It has cyclomatic complexity of 12, just
// over the limit of 11. I think breaking this function apart hurts readability, so there's a nolint directive.
// nolint: gocyclo
func newInstanceTypeFilter(ctx context.Context, filter *v1alpha5.InstanceTypeFilter) *instanceTypeFilter {
	f := &instanceTypeFilter{}
	if filter.CPUCount != nil {
		if filter.CPUCount.Min != nil {
			f.minCPU = resource.NewQuantity(*filter.CPUCount.Min, resource.DecimalExponent)
		}
		if filter.CPUCount.Max != nil {
			f.maxCPU = resource.NewQuantity(*filter.CPUCount.Max, resource.DecimalExponent)
		}
	}
	if filter.MemoryMiB != nil {
		if filter.MemoryMiB.Min != nil {
			f.minMemory = resource.NewQuantity(*filter.MemoryMiB.Min*1024*1024, resource.DecimalExponent)
		}
		if filter.MemoryMiB.Max != nil {
			f.maxMemory = resource.NewQuantity(*filter.MemoryMiB.Max*1024*1024, resource.DecimalExponent)
		}
	}
	if filter.MemoryMiBPerCPU != nil {
		if filter.MemoryMiBPerCPU.Max != nil {
			f.maxMemoryMiBPerCPU = filter.MemoryMiBPerCPU.Max
		}
		if filter.MemoryMiBPerCPU.Min != nil {
			f.minMemoryMiBPerCPU = filter.MemoryMiBPerCPU.Min
		}
	}
	for i, expression := range filter.NameMatchExpressions {
		re, err := regexp.Compile(expression)
		if err != nil {
			logging.FromContext(ctx).Errorf("unable to parse NameMatchExpressions[%d]=%q, it will be ignored", i, expression)
			continue
		}
		f.nameMatchExpressions = append(f.nameMatchExpressions, re)
	}
	return f
}

func (f *instanceTypeFilter) Accepts(it cloudprovider.InstanceType) bool {
	cpu := it.Resources()[v1.ResourceCPU]
	mem := it.Resources()[v1.ResourceMemory]
	if !f.passesCPUFilter(cpu) {
		return false
	}
	if !f.passesMemoryFilter(mem) {
		return false
	}
	if !f.passesMemoryRatioFilter(cpu, mem) {
		return false
	}
	if !f.passesNameMatchExpressions(it.Name()) {
		return false
	}
	return true
}

func (f *instanceTypeFilter) passesMemoryFilter(mem resource.Quantity) bool {
	if f.minMemory != nil && mem.Cmp(*f.minMemory) < 0 {
		return false
	}
	if f.maxMemory != nil && mem.Cmp(*f.maxMemory) > 0 {
		return false
	}
	return true
}

func (f *instanceTypeFilter) passesCPUFilter(cpu resource.Quantity) bool {
	if f.minCPU != nil && cpu.Cmp(*f.minCPU) < 0 {
		return false
	}
	if f.maxCPU != nil && cpu.Cmp(*f.maxCPU) > 0 {
		return false
	}
	return true
}

func (f *instanceTypeFilter) passesMemoryRatioFilter(cpu resource.Quantity, mem resource.Quantity) bool {
	ratio := mem.AsApproximateFloat64() / (1024 * 1024) / cpu.AsApproximateFloat64()
	if f.minMemoryMiBPerCPU != nil {
		if ratio < float64(*f.minMemoryMiBPerCPU) {
			return false
		}
	}
	if f.maxMemoryMiBPerCPU != nil {
		if ratio > float64(*f.maxMemoryMiBPerCPU) {
			return false
		}
	}
	return true
}

func (f *instanceTypeFilter) passesNameMatchExpressions(name string) bool {
	// no expressions, so all instance types match
	if len(f.nameMatchExpressions) == 0 {
		return true
	}
	// expression are OR'd so the name must match at least one
	for _, expr := range f.nameMatchExpressions {
		if expr.MatchString(name) {
			return true
		}
	}
	// or it fails
	return false
}
