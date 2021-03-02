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

package packing

import (
	"errors"
	"sort"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// TODO get this information from node-selector
var (
	instanceTypesAvailable = []*instanceType{
		{
			name: "m5.8xlarge",
			totalCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("32000m"),
				v1.ResourceMemory: resource.MustParse("128Gi"),
			},
			utilizedCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("0"),
				v1.ResourceMemory: resource.MustParse("0"),
			},
		},
		{
			name: "m5.2xlarge",
			totalCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("8000m"),
				v1.ResourceMemory: resource.MustParse("32Gi"),
			},
			utilizedCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("0"),
				v1.ResourceMemory: resource.MustParse("0"),
			},
		},
		{
			name: "m5.xlarge",
			totalCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("4000m"),
				v1.ResourceMemory: resource.MustParse("16Gi"),
			},
			utilizedCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("0"),
				v1.ResourceMemory: resource.MustParse("0"),
			},
		},
		{
			name: "m5.large",
			totalCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("2"),
				v1.ResourceMemory: resource.MustParse("8Gi"),
			},
			utilizedCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("0"),
				v1.ResourceMemory: resource.MustParse("0"),
			},
		},
	}
)

var InsufficentCapacityErr = errors.New("insufficient capacity")

type instanceType struct {
	name             string
	utilizedCapacity v1.ResourceList
	totalCapacity    v1.ResourceList
}

func (p *PodPacker) getInstanceTypes(filter string) []*instanceType {
	instanceTypeOptions := instanceTypesAvailable
	// sort by increasing capacity of the instance
	sort.Sort(byCapacity{instanceTypeOptions})
	return instanceTypeOptions
}

func (it *instanceType) isAllocatable(cpu, memory resource.Quantity) bool {
	// TODO check pods count
	return it.totalCapacity.Cpu().Cmp(cpu) >= 0 &&
		it.totalCapacity.Memory().Cmp(memory) >= 0
}

func (it *instanceType) reserveCapacity(cpu, memory resource.Quantity) error {

	// TODO reserve pods count
	targetCPU := it.utilizedCapacity.Cpu()
	targetCPU.Add(cpu)
	targetMemory := it.utilizedCapacity.Memory()
	targetMemory.Add(memory)
	if !it.isAllocatable(*targetCPU, *targetMemory) {
		return InsufficentCapacityErr
	}
	it.utilizedCapacity[v1.ResourceCPU] = *targetCPU
	it.utilizedCapacity[v1.ResourceMemory] = *targetMemory
	return nil
}

type instanceTypes []*instanceType

func (t instanceTypes) Len() int {
	return len(t)
}

func (t instanceTypes) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

type byCapacity struct{ instanceTypes }

func (c byCapacity) Less(i, j int) bool {
	if c.instanceTypes[i].totalCapacity.Cpu().MilliValue() ==
		c.instanceTypes[j].totalCapacity.Cpu().MilliValue() {
		return c.instanceTypes[i].totalCapacity.Memory().MilliValue() <
			c.instanceTypes[j].totalCapacity.Memory().MilliValue()
	}
	return c.instanceTypes[i].totalCapacity.Cpu().MilliValue() <
		c.instanceTypes[j].totalCapacity.Cpu().MilliValue()
}
