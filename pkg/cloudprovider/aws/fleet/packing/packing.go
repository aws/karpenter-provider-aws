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
	"k8s.io/apimachinery/pkg/api/resource"
)

type podPacker struct {
	ec2 ec2iface.EC2API
}

// Packer helps pack the pods and calculates efficient placement on the instances.
type Packer interface {
	Pack(ctx context.Context, pods []*v1.Pod) ([]*Packing, error)
}

// Packing contains a list of pods that can be placed on any of Instance type
// in the InstanceTypes
type Packing struct {
	Pods          []*v1.Pod
	InstanceTypes []string
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
func (p *podPacker) Pack(ctx context.Context, pods []*v1.Pod) ([]*Packing, error) {
	// 1. Arrange pods in decreasing order by the amount of CPU requested, if
	// CPU requested is equal compare memory requested.
	return p.packPods(pods)
}

// takes a list of pods, sorts them based on their resource requirements compared by CPU and memory.
func (p *podPacker) packPods(pods []*v1.Pod) ([]*Packing, error) {
	sort.Sort(sort.Reverse(byResourceRequested{pods}))
	estimator := newPackingEstimator()
	packings := []*Packing{}
	// start with the biggest pod check max how many pods can we fit with this pod
	for start, pod := range pods {
		if estimator.isPodPacked(pod) {
			continue
		}
		packing := estimator.calculatePackingForPod(start, pods)
		// checked all instance type and found no packing option
		if len(packing.Pods) == 0 || len(packing.InstanceTypes) == 0 {
			return nil, fmt.Errorf("no instance type found for packing %d pods", len(pods))
		}
		// keep a track of pods we have packed
		estimator.setPodsPacked(packing.Pods...)
		packings = append(packings, packing)
		zap.S().Debugf("For %d pod(s) instance types selected are %v", len(packing.Pods), packing.InstanceTypes)
	}
	return packings, nil
}

type packingEstimator struct {
	podsPacked map[*v1.Pod]bool
}

func newPackingEstimator() *packingEstimator {
	return &packingEstimator{podsPacked: map[*v1.Pod]bool{}}
}

func (p *packingEstimator) isPodPacked(pod *v1.Pod) bool {
	return p.podsPacked[pod]
}

func (p *packingEstimator) setPodsPacked(pods ...*v1.Pod) {
	for _, pod := range pods {
		p.podsPacked[pod] = true
	}
}

// TODO filter instance types based on node contraints like availability zones etc.
func (p *packingEstimator) getNodeCapacities(filter ...string) []*nodeCapacity {
	return nodePools
}

// calculatePackingForPod will calculate the packing for pods starting from
// start index, it returns the max pods that can be packed on a set of
// instance types with this pod.
func (p *packingEstimator) calculatePackingForPod(start int, pods []*v1.Pod) *Packing {
	podsPacked := []*v1.Pod{}
	instanceTypesSelected := []*nodeCapacity{}
	// Get all available instance types reverse sorted by their capacity, for every instance
	// try to fit as many pods as possible. return the list of instances with
	// highest pods packed and the pods.

	// TODO add filters
	// TODO reserve (Kubelet+ daemon sets) overhead for instance types
	// TODO count number of pods created on an instance type
	for _, instanceOption := range p.getNodeCapacities("") {
		// check how many pods we can fit on this instance type
		packings, err := p.packingForInstance(instanceOption, start, pods)
		if err != nil {
			zap.S().Errorf("Failed to calculate packing for instance type %s, err, %w", instanceOption.instanceType, err)
			continue
		}
		if len(packings) == 0 {
			continue
		}
		// If the pods packed are the same as before, this instance type can be
		// considered as a backup option in case we get ICE
		if podsMatch(podsPacked, packings) {
			instanceTypesSelected = append(instanceTypesSelected, instanceOption)
		} else if len(packings) > len(podsPacked) {
			// If pods packed are more than compared to what we got in last
			// iteration, consider using this instance type
			podsPacked = packings
			instanceTypesSelected = []*nodeCapacity{instanceOption}
		}
	}
	instanceTypeNames := []string{}
	for _, instance := range instanceTypesSelected {
		instanceTypeNames = append(instanceTypeNames, instance.instanceType)
	}
	return &Packing{Pods: podsPacked, InstanceTypes: instanceTypeNames}
}

func (p *packingEstimator) packingForInstance(instance *nodeCapacity, start int, pods []*v1.Pod) ([]*v1.Pod, error) {
	packing := []*v1.Pod{}
	// start with the largest pod based on resources requested
	// pods before the start index have been packed
	for i := start; i < len(pods); i++ {
		pod := pods[i]
		if p.isPodPacked(pod) {
			continue
		}
		cpu := cpuFor(pod)
		memory := memoryFor(pod)
		if ok := instance.reserveCapacity(*cpu, *memory); !ok {
			// First we need to find the instance on which we can fit the
			// largest pods, it can't fit here, return and try with a next
			// bigger instance type
			if len(packing) == 0 {
				return packing, nil
			}
			// We have already packed a bigger pod on this instance type, if
			// we can't pack this pod, check the next pod it might be
			// smaller in size and help fill the gaps on the instance
			continue
		}
		packing = append(packing, pod)
	}
	instance.utilizedCapacity = v1.ResourceList{
		v1.ResourceCPU:    resource.Quantity{},
		v1.ResourceMemory: resource.Quantity{},
	}
	return packing, nil
}

func podsMatch(first, second []*v1.Pod) bool {
	if len(first) != len(second) {
		return false
	}
	podkey := func(pod *v1.Pod) string {
		return pod.Namespace + "/" + pod.Name
	}
	podSeen := map[string]int{}
	for _, pod := range first {
		podSeen[podkey(pod)]++
	}
	for _, pod := range second {
		podSeen[podkey(pod)]--
	}
	for _, value := range podSeen {
		if value != 0 {
			return false
		}
	}
	return true
}
