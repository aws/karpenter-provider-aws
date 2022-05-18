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
	nameMatchExpressions []*regexp.Regexp
	minResources         v1.ResourceList
	maxResources         v1.ResourceList
	memoryPerCPU         *v1alpha5.MinMax
}

// newInstanceTypeFilter constructs a new compiled instance type filter.
func newInstanceTypeFilter(ctx context.Context, filter *v1alpha5.InstanceTypeFilter) *instanceTypeFilter {
	f := &instanceTypeFilter{
		minResources: filter.MinResources,
		maxResources: filter.MaxResources,
		memoryPerCPU: filter.MemoryPerCPU,
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
	if !f.passesMinResourcesFilter(it.Resources()) {
		return false
	}
	if !f.passesMaxResourcesFilter(it.Resources()) {
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

func (f *instanceTypeFilter) passesMemoryRatioFilter(cpu, mem resource.Quantity) bool {
	if f.memoryPerCPU == nil {
		return true
	}

	memPerCPU := mem.AsApproximateFloat64() / cpu.AsApproximateFloat64()
	if f.memoryPerCPU.Min != nil {
		minMemPerCPU := f.memoryPerCPU.Min.AsApproximateFloat64()
		if memPerCPU < minMemPerCPU {
			return false
		}
	}
	if f.memoryPerCPU.Max != nil {
		maxMemPerCPU := f.memoryPerCPU.Max.AsApproximateFloat64()
		if memPerCPU > maxMemPerCPU {
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

func (f *instanceTypeFilter) passesMinResourcesFilter(resources v1.ResourceList) bool {
	for k, required := range f.minResources {
		available, ok := resources[k]
		if !ok {
			return false
		}
		if required.Cmp(available) > 0 {
			return false
		}
	}
	return true
}

func (f *instanceTypeFilter) passesMaxResourcesFilter(resources v1.ResourceList) bool {
	for k, required := range f.maxResources {
		available, ok := resources[k]
		if !ok {
			// it's ok if we don't have the resource as zero is less than the max
			continue
		}
		if required.Cmp(available) < 0 {
			return false
		}
	}
	return true
}
