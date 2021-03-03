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
	"errors"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type podPacker struct {
	ec2 ec2iface.EC2API
}

// Packer helps pack the pods and calculates efficient placement on the instances.
type Packer interface {
	Pack(ctx context.Context, pods []*v1.Pod) ([]*Packings, error)
}

// Packings contains a list of pods that can be placed on any of Instance type
// in the InstanceTypeOptions
type Packings struct {
	Pods                []*v1.Pod
	InstanceTypeOptions []string
}

// NewPacker returns a Packer implementation
func NewPacker(ec2 ec2iface.EC2API) Packer {
	return &podPacker{ec2: ec2}
}

// Pack returns the packings for the provided pods. Computes a set of viable
// instance types for each packing of pods. Instance variety enables EC2 to make
// better cost and availability decisions. Pods provided are all schedulable in
// the same zone as tightly as possible. It follows the First Fit Decreasing bin
// packing technique, reference-
// https://en.wikipedia.org/wiki/Bin_packing_problem#First_Fit_Decreasing_(FFD)
func (p *podPacker) Pack(ctx context.Context, pods []*v1.Pod) ([]*Packings, error) {
	// 1. Arrange pods in decreasing order by the amount of CPU requested, if
	// CPU requested is equal compare memory requested.
	sort.Sort(sort.Reverse(byResourceRequested{pods}))
	return p.packSortedPods(pods)
}

// takes a list of pods sorted based on their resource requirements compared by CPU and memory.
func (p *podPacker) packSortedPods(pods []*v1.Pod) ([]*Packings, error) {
	// start with the smallest instance type and the biggest pod check how many
	// pods can we fit, go to the next bigger type and check if we can fit more
	// pods. Compare pods packed on all types and select the instance type with
	// highest pod count.
	estimator := &packingEstimator{isPodPacked: map[*v1.Pod]bool{}}
	packings := []*Packings{}
	for _, pod := range pods {
		if estimator.isPodPacked[pod] {
			continue
		}
		podsPacked, instances := estimator.calculatePackingForAllTypes(pods)
		// checked all instance type and found no packing option
		if len(podsPacked) == 0 || len(instances) == 0 {
			return nil, fmt.Errorf("no capacity type found for packing %d pods", len(pods))
		}
		// keep a track of pods we have packed
		for _, pod := range podsPacked {
			estimator.isPodPacked[pod] = true
		}
		zap.S().Debugf("For %d pod(s) instance types selected are %v", len(podsPacked), instances)
		packings = append(packings, &Packings{podsPacked, instances})
	}
	return packings, nil
}

type packingEstimator struct {
	isPodPacked map[*v1.Pod]bool
}

func (p *packingEstimator) calculatePackingForAllTypes(pods []*v1.Pod) ([]*v1.Pod, []string) {
	previousPacking := []*v1.Pod{}
	instanceTypesSelected := []*nodeCapacity{}
	// Get all available instance types reverse sorted by their capacity, for every instance
	// try to fit as many pods as possible. return the list of instances with
	// highest pods packed and the pods.

	// TODO add filters
	// TODO reserve (Kubelet+ daemon sets) overhead for instance types
	// TODO count number of pods created on an instance type
	for _, instanceOption := range p.getInstanceTypes("") {
		// check how many pods we can fit on this instance type
		packings, err := p.calculatePackingForInstance(instanceOption, pods)
		if err != nil {
			zap.S().Errorf("Failed to calculate packing for instance type %s, err, %w", instanceOption.instanceType, err)
			continue
		}
		if len(packings) == 0 {
			continue
		}
		// If the pods packed are the same as before, this instance type can be
		// considered as a backup option in case we get ICE
		if podsMatch(previousPacking, packings) {
			instanceTypesSelected = append(instanceTypesSelected, instanceOption)
		} else if len(packings) > len(previousPacking) {
			// If pods packed are more than compared to what we got in last
			// iteration, consider using this instance type
			previousPacking = packings
			instanceTypesSelected = []*nodeCapacity{instanceOption}
		}
	}
	instanceTypeNames := []string{}
	for _, instance := range instanceTypesSelected {
		instanceTypeNames = append(instanceTypeNames, instance.instanceType)
	}
	return previousPacking, instanceTypeNames
}

func (p *packingEstimator) calculatePackingForInstance(instance *nodeCapacity, podList []*v1.Pod) ([]*v1.Pod, error) {
	packing := []*v1.Pod{}
	// start with the smallest pod based on resources requested
	for _, pod := range podList {
		if p.isPodPacked[pod] {
			continue
		}
		cpu := calculateCPURequested(pod)
		memory := calculateMemoryRequested(pod)
		if err := instance.reserveCapacity(cpu, memory); err != nil {
			if errors.Is(err, ErrInsufficientCapacity) {
				// If we can't pack this pod, we can't do any better return the
				// packings we have already calculated
				break
			}
			return nil, fmt.Errorf("reserve capacity failed %w", err)
		}
		packing = append(packing, pod)
	}
	instance.utilizedCapacity = v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse("0"),
		v1.ResourceMemory: resource.MustParse("0"),
	}
	return packing, nil
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
		if _, ok := podSeen[pod]; !ok {
			return false
		}
	}
	return true
}
