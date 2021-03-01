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
	"fmt"
	"sort"
	"sync"

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
			availableCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("32000m"),
				v1.ResourceMemory: resource.MustParse("128Gi"),
			},
			pricePerHour: 1.536,
		},
		// {
		// 	name: "m5.4xlarge	",
		// 	totalCapacity: v1.ResourceList{
		// 		v1.ResourceCPU:    resource.MustParse("16000m"),
		// 		v1.ResourceMemory: resource.MustParse("64Gi"),
		// 	},
		// 	availableCapacity: v1.ResourceList{
		// 		v1.ResourceCPU:    resource.MustParse("16000m"),
		// 		v1.ResourceMemory: resource.MustParse("64Gi"),
		// 	},
		// 	pricePerHour: 0.768,
		// },
		{
			name: "m5.2xlarge",
			totalCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("8000m"),
				v1.ResourceMemory: resource.MustParse("32Gi"),
			},
			availableCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("8000m"),
				v1.ResourceMemory: resource.MustParse("32Gi"),
			},
			pricePerHour: 0.384,
		},
		{
			name: "m5.xlarge",
			totalCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("4000m"),
				v1.ResourceMemory: resource.MustParse("16Gi"),
			},
			availableCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("4000m"),
				v1.ResourceMemory: resource.MustParse("16Gi"),
			},
			pricePerHour: 0.192,
		},
		{
			name: "m5.large",
			totalCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("2"),
				v1.ResourceMemory: resource.MustParse("8Gi"),
			},
			availableCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("2"),
				v1.ResourceMemory: resource.MustParse("8Gi"),
			},
			pricePerHour: 0.096,
		},
	}
)

type instanceType struct {
	name string
	sync.RWMutex
	availableCapacity v1.ResourceList
	totalCapacity     v1.ResourceList
	pricePerHour      float64
}

func (p *PodPacker) getInstanceTypes(filter string) []*instanceType {
	instanceTypeOptions := instanceTypesAvailable
	// sort by increasing capacity of the instance
	sort.Sort(byCapacity{instanceTypeOptions})
	return instanceTypeOptions
}

func (r *instanceType) isAllocatable(cpu, memory resource.Quantity) bool {
	r.Lock()
	defer r.Unlock()
	return (r.availableCapacity.Cpu().Cmp(cpu) >= 0) &&
		(r.availableCapacity.Memory().Cmp(memory) >= 0)
}

func (r *instanceType) reserveCapacity(cpu, memory resource.Quantity) error {
	r.Lock()
	defer r.Unlock()
	if r.availableCapacity.Cpu().IsZero() {
		return fmt.Errorf("insufficient CPU capacity")
	}
	if r.availableCapacity.Memory().IsZero() {
		return fmt.Errorf("insufficient Memory capacity")
	}
	r.availableCapacity.Cpu().Sub(cpu)
	r.availableCapacity.Memory().Sub(memory)
	return nil
}

func filterInstancesBasedOnCost(instanceTypeOptions []*instanceType) []*instanceType {
	sort.Sort(byCost{instanceTypeOptions})
	return instanceTypeOptions
}

type instanceTypes []*instanceType

func (c instanceTypes) Len() int {
	return len(c)
}

func (c instanceTypes) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
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

type byCost struct{ instanceTypes }

func (c byCost) Less(i, j int) bool {
	return c.instanceTypes[i].pricePerHour < c.instanceTypes[j].pricePerHour
}
