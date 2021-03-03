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
	nodePools = []*nodeCapacity{
		{
			instanceType: "m5.8xlarge",
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
			instanceType: "m5.2xlarge",
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
			instanceType: "m5.xlarge",
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
			instanceType: "m5.large",
			totalCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("2000m"),
				v1.ResourceMemory: resource.MustParse("8Gi"),
			},
			utilizedCapacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("0"),
				v1.ResourceMemory: resource.MustParse("0"),
			},
		},
	}
)

var ErrInsufficientCapacity = errors.New("insufficient capacity")

func (p *packingEstimator) getInstanceTypes(filter string) []*nodeCapacity {
	// sort by increasing capacity of the instance
	sort.Sort(byCapacity{nodePools})
	return nodePools
}

type nodeCapacity struct {
	instanceType     string
	utilizedCapacity v1.ResourceList
	totalCapacity    v1.ResourceList
}

func (nc *nodeCapacity) isAllocatable(cpu, memory resource.Quantity) bool {
	// TODO check pods count
	return nc.totalCapacity.Cpu().Cmp(cpu) >= 0 &&
		nc.totalCapacity.Memory().Cmp(memory) >= 0
}

func (nc *nodeCapacity) reserveCapacity(cpu, memory resource.Quantity) error {

	// TODO reserve pods count
	targetCPU := nc.utilizedCapacity.Cpu()
	targetCPU.Add(cpu)
	targetMemory := nc.utilizedCapacity.Memory()
	targetMemory.Add(memory)
	if !nc.isAllocatable(*targetCPU, *targetMemory) {
		return ErrInsufficientCapacity
	}
	nc.utilizedCapacity[v1.ResourceCPU] = *targetCPU
	nc.utilizedCapacity[v1.ResourceMemory] = *targetMemory
	return nil
}

type nodeCapacities []*nodeCapacity

func (t nodeCapacities) Len() int {
	return len(t)
}

func (t nodeCapacities) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

type byCapacity struct{ nodeCapacities }

func (c byCapacity) Less(i, j int) bool {
	if c.nodeCapacities[i].totalCapacity.Cpu().MilliValue() ==
		c.nodeCapacities[j].totalCapacity.Cpu().MilliValue() {
		return c.nodeCapacities[i].totalCapacity.Memory().MilliValue() <
			c.nodeCapacities[j].totalCapacity.Memory().MilliValue()
	}
	return c.nodeCapacities[i].totalCapacity.Cpu().MilliValue() <
		c.nodeCapacities[j].totalCapacity.Cpu().MilliValue()
}
