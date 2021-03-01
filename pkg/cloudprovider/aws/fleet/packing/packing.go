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
	"context"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
)

type PodPacker struct {
	ec2 ec2iface.EC2API
}

// PodPacker helps pack the pods and calculates efficient placement on the instances.
type Packer interface {
	Pack(ctx context.Context, pods []*v1.Pod) ([]*Packings, error)
}

// Packings contains a list of pods that can be placed on any of Instance type
// in the InstanceTypeOptions
type Packings struct {
	Pods                []*v1.Pod
	InstanceTypeOptions []string
}

func NewPacker(ec2 ec2iface.EC2API) *PodPacker {
	return &PodPacker{ec2: ec2}
}

// Pack returns the packings for the provided pods. Computes a set of viable
// instance types for each packing of pods. Instance variety enables EC2 to make
// better cost and availability decisions.
// Pods provided are all schedulable in the same zone as tightly as possible.
func (p *PodPacker) Pack(ctx context.Context, pods []*v1.Pod) ([]*Packings, error) {
	zap.S().Debugf("Successfully packed %d pods onto %d nodes", len(pods), 1)

	// 1. Arrange pods in decreasing order by the amount of CPU requested, if
	// CPU requested is equal compare memory requested.
	sort.Sort(sort.Reverse(byResourceRequested{pods}))

	// 2. Get all available instance types with there capacity and cost
	// TODO add filters
	instanceTypes := p.getInstanceTypes("")
	// TODO reserve (Kubelet + daemon sets) overhead for instance types
	return p.packSortedPods(pods, instanceTypes)
}

// takes a list of pods sorted based on their resource requirements compared by CPU and memory.

func (p *PodPacker) packSortedPods(pods []*v1.Pod, instanceTypes []*instanceType) ([]*Packings, error) {
	// Start with the smallest instance type and the biggest pod check how
	// many pods can we fit, go to the next bigger type and check if we can fit
	// more pods. Compare pods packed on all and select the instance type with
	// highest pod count.
	podsPacked := map[*v1.Pod]bool{}
	packings := make([]*Packings, 0)
	for start, pod := range pods {
		if podsPacked[pod] {
			continue
		}

		prevPacking := make([]*v1.Pod, 0)
		instanceTypesSelected := make([]*instanceType, 0)
		// Go to the next instance type see how many more pods can we fit?
		for _, instance := range instanceTypes {
			// from start index loop through all the pods and check how many
			// pods we can fit on each instance type
			packings, err := calculateBestPackingOption(instance, start, pods, podsPacked)
			if err != nil {
				zap.S().Errorf("Failed to calculate packing for instance type %s, err, %w", instance.name, err)
				continue
			}
			zap.S().Info("packings are ", len(packings))
			// If the pods packed are the same as before, this instance type can be
			// considered a backup option in case we get ICE
			if len(packings) == len(prevPacking) &&
				podsMatch(prevPacking, packings) {
				instanceTypesSelected = append(instanceTypesSelected, instance)
			} else if len(packings) > len(prevPacking) {
				// If pods packed are more than compared to what we did last time,
				// consider using this instance type
				prevPacking = packings
				instanceTypesSelected = []*instanceType{instance}
			}
		}
		// checked all instance type and found no packing option
		if len(prevPacking) == 0 {
			return nil, fmt.Errorf("no instance type found for packing %d pods", len(pods))
		}
		// keep a track of pods we have packed
		for _, pod := range prevPacking {
			podsPacked[pod] = true
		}
		instanceOptions := []string{}
		for _, instanceType := range filterInstancesBasedOnCost(instanceTypesSelected) {
			instanceOptions = append(instanceOptions, instanceType.name)
		}
		zap.S().Info("Instance Options are %v", instanceOptions)
		packings = append(packings, &Packings{prevPacking, instanceOptions})
	}
	return packings, nil
}

func podsMatch(first, second []*v1.Pod) bool {
	if len(first) != len(second) {
		return false
	}
	podSeen := map[*v1.Pod]struct{}{}
	for _, pod := range first {
		podSeen[pod] = struct{}{}
	}
	for _, pod := range second {
		if _, exist := podSeen[pod]; !exist {
			return false
		}
	}
	return true
}

func calculateBestPackingOption(instanceType *instanceType, start int, podList []*v1.Pod, packed map[*v1.Pod]bool) ([]*v1.Pod, error) {
	packing := make([]*v1.Pod, 0)
	for i := start; i < len(podList); i++ {
		pod := podList[i]
		if packed[pod] {
			continue
		}
		cpu := calculateCPURequested(pod)
		memory := calculateMemoryRequested(pod)
		if instanceType.isAllocatable(cpu, memory) {
			if err := instanceType.reserveCapacity(cpu, memory); err != nil {
				return nil, fmt.Errorf("reserve capacity failed %w", err)
			}
			packing = append(packing, pod)
		}
	}
	return packing, nil
}
