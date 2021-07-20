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

	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"github.com/awslabs/karpenter/pkg/utils/resources"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/logging"
)

type Packable struct {
	cloudprovider.InstanceType
	reserved v1.ResourceList
	total    v1.ResourceList
}

type Result struct {
	packed   []*v1.Pod
	unpacked []*v1.Pod
}

// PackablesFor creates viable packables for the provided constraints, excluding
// those that can't fit resources or violate constraints.
func PackablesFor(ctx context.Context, instanceTypes []cloudprovider.InstanceType, constraints *Constraints) []*Packable {
	packables := []*Packable{}
	for _, instanceType := range instanceTypes {
		packable := PackableFor(instanceType)
		// 1. Filter viable instance types
		if err := functional.ValidateAll(
			func() error { return packable.validateZones(constraints) },
			func() error { return packable.validateInstanceType(constraints) },
			func() error { return packable.validateArchitecture(constraints) },
			func() error { return packable.validateOperatingSystem(constraints) },
			func() error { return packable.validateNvidiaGpus(constraints) },
			func() error { return packable.validateAMDGpus(constraints) },
			func() error { return packable.validateAWSNeurons(constraints) },
		); err != nil {
			continue
		}
		// 2. Calculate Kubelet Overhead
		if ok := packable.reserve(instanceType.Overhead()); !ok {
			logging.FromContext(ctx).Debugf("Excluding instance type %s because there are not enough resources for kubelet and system overhead", packable.Name())
			continue
		}
		// 3. Calculate Daemonset Overhead
		if len(packable.Pack(constraints.Daemons).unpacked) > 0 {
			logging.FromContext(ctx).Debugf("Excluding instance type %s because there are not enough resources for daemons", packable.Name())
			continue
		}
		packables = append(packables, packable)
	}
	return packables
}

func PackableFor(i cloudprovider.InstanceType) *Packable {
	return &Packable{
		InstanceType: i,
		total: v1.ResourceList{
			v1.ResourceCPU:      *i.CPU(),
			v1.ResourceMemory:   *i.Memory(),
			resources.NvidiaGPU: *i.NvidiaGPUs(),
			resources.AMDGPU:    *i.AMDGPUs(),
			resources.AWSNeuron: *i.AWSNeurons(),
			v1.ResourcePods:     *i.Pods(),
		},
	}
}

// Pack attempts to pack the pods into capacity, keeping track of previously
// packed pods. If the capacity cannot fit the pod, they are set aside.
func (p *Packable) Pack(pods []*v1.Pod) *Result {
	result := &Result{}
	for _, pod := range pods {
		if ok := p.reservePod(pod); ok {
			result.packed = append(result.packed, pod)
			continue
		}
		// if largest pod can't be packed, set it aside
		if len(result.packed) == 0 {
			result.unpacked = append(result.unpacked, pods...)
			return result
		}
		result.unpacked = append(result.unpacked, pod)
	}
	return result
}

func (p *Packable) reserve(requests v1.ResourceList) bool {
	candidate := resources.Merge(p.reserved, requests)
	// If any candidate resource exceeds total, fail to reserve
	for resourceName, quantity := range candidate {
		if quantity.Cmp(p.total[resourceName]) > 0 {
			return false
		}
	}
	p.reserved = candidate
	return true
}

func (p *Packable) reservePod(pod *v1.Pod) bool {
	requests := resources.RequestsForPods(pod)
	requests[v1.ResourcePods] = *resource.NewQuantity(1, resource.BinarySI)
	return p.reserve(requests)
}

func (p *Packable) validateInstanceType(constraints *Constraints) error {
	if len(constraints.InstanceTypes) == 0 {
		return nil
	}
	if !functional.ContainsString(constraints.InstanceTypes, p.Name()) {
		return fmt.Errorf("instance type %s is not in %v", p.Name(), constraints.InstanceTypes)
	}
	return nil
}

func (p *Packable) validateArchitecture(constraints *Constraints) error {
	if constraints.Architecture == nil {
		return nil
	}
	if !functional.ContainsString(p.Architectures(), *constraints.Architecture) {
		return fmt.Errorf("architecture %s is not in %v", *constraints.Architecture, p.Architectures())
	}
	return nil
}

func (p *Packable) validateOperatingSystem(constraints *Constraints) error {
	if constraints.OperatingSystem == nil {
		return nil
	}
	if !functional.ContainsString(p.OperatingSystems(), *constraints.OperatingSystem) {
		return fmt.Errorf("operating system %s is not in %v", *constraints.OperatingSystem, p.OperatingSystems())
	}
	return nil
}

func (p *Packable) validateZones(constraints *Constraints) error {
	if len(constraints.Zones) == 0 {
		return nil
	}
	if len(functional.IntersectStringSlice(constraints.Zones, p.Zones())) == 0 {
		return fmt.Errorf("zones %v are not in %v", constraints.Zones, p.Zones())
	}
	return nil
}

func (p *Packable) validateNvidiaGpus(constraints *Constraints) error {
	if p.InstanceType.NvidiaGPUs().IsZero() {
		return nil
	}
	for _, pod := range constraints.Pods {
		for _, container := range pod.Spec.Containers {
			if _, ok := container.Resources.Requests[resources.NvidiaGPU]; ok {
				return nil
			}
		}
	}
	return fmt.Errorf("nvidia gpu is not required")
}

func (p *Packable) validateAMDGpus(constraints *Constraints) error {
	if p.InstanceType.AMDGPUs().IsZero() {
		return nil
	}
	for _, pod := range constraints.Pods {
		for _, container := range pod.Spec.Containers {
			if _, ok := container.Resources.Requests[resources.AMDGPU]; ok {
				return nil
			}
		}
	}
	return fmt.Errorf("amd gpu is not required")
}

func (p *Packable) validateAWSNeurons(constraints *Constraints) error {
	if p.InstanceType.AWSNeurons().IsZero() {
		return nil
	}
	for _, pod := range constraints.Pods {
		for _, container := range pod.Spec.Containers {
			if _, ok := container.Resources.Requests[resources.AWSNeuron]; ok {
				return nil
			}
		}
	}
	return fmt.Errorf("aws neuron is not required")
}
