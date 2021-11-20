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

package binpacking

import (
	"context"
	"fmt"

	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/controllers/provisioning/scheduling"
	"github.com/awslabs/karpenter/pkg/utils/resources"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
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

// PackablesFor creates viable packables for the provided schedule, excluding
// those that can't fit resources or violate schedule.
func PackablesFor(ctx context.Context, instanceTypes []cloudprovider.InstanceType, schedule *scheduling.Schedule) []*Packable {
	packables := []*Packable{}
	for _, instanceType := range instanceTypes {
		packable := PackableFor(instanceType)
		// First pass at filtering down to viable instance types;
		// additional filtering will be done by later steps (such as
		// removing instance types that obviously lack resources, such
		// as GPUs, for the workload being presented).
		if err := multierr.Combine(
			packable.validateZones(schedule),
			packable.validateInstanceType(schedule),
			packable.validateArchitecture(schedule),
			packable.validateCapacityTypes(schedule),
			// Although this will remove instances that have GPUs when
			// not required, removal of instance types that *lack*
			// GPUs will be done later.
			packable.validateNvidiaGpus(schedule),
			packable.validateAMDGpus(schedule),
			packable.validateAWSNeurons(schedule),
		); err != nil {
			continue
		}
		// Calculate Kubelet Overhead
		if ok := packable.reserve(instanceType.Overhead()); !ok {
			logging.FromContext(ctx).Debugf("Excluding instance type %s because there are not enough resources for kubelet and system overhead", packable.Name())
			continue
		}
		// Calculate Daemonset Overhead
		if len(packable.Pack(schedule.Daemons).unpacked) > 0 {
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

// Pack attempts to pack the pods, keeping track of previously packed
// ones. Any pods that cannot fit, including because of missing
// resources on the packable, will be left unpacked.
func (p *Packable) Pack(pods []*v1.Pod) *Result {
	result := &Result{}
	for i, pod := range pods {
		if ok := p.reservePod(pod); ok {
			result.packed = append(result.packed, pod)
			continue
		}
		if p.fits(pods[len(pods)-1]) {
			result.unpacked = append(result.unpacked, pods[i:]...)
			return result
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

// fits checks if adding the pod would overflow the total resources
// available. It also ensures that instance types that could not
// possibly satisfy the pod at all (for example if the pod needs
// NvidiaGPUs and the instance type doesn't have any) will be
// eliminated from consideration.
func (p *Packable) fits(pod *v1.Pod) bool {
	minResourceList := resources.RequestsForPods(pod)
	for resourceName, totalQuantity := range p.total {
		reservedQuantity := p.reserved[resourceName].DeepCopy()
		reservedQuantity.Add(minResourceList[resourceName])
		if !totalQuantity.IsZero() && reservedQuantity.Cmp(totalQuantity) >= 0 {
			return true
		}
	}
	return false
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

func (p *Packable) validateInstanceType(schedule *scheduling.Schedule) error {
	if !schedule.Requirements.InstanceTypes().Has(p.Name()) {
		return fmt.Errorf("instance type %s is not in %v", p.Name(), schedule.Requirements.InstanceTypes().List())
	}
	return nil
}

func (p *Packable) validateArchitecture(schedule *scheduling.Schedule) error {
	if !schedule.Requirements.Architectures().Has(p.Architecture()) {
		return fmt.Errorf("architecture %s is not in %v", p.Architecture(), schedule.Requirements.Architectures().List())
	}
	return nil
}

func (p *Packable) validateZones(schedule *scheduling.Schedule) error {
	zones := sets.String{}
	for _, offering := range p.Offerings() {
		zones.Insert(offering.Zone)
	}
	if schedule.Requirements.Zones().Intersection(zones).Len() == 0 {
		return fmt.Errorf("zones %v are not in %v", zones, schedule.Requirements.Zones().List())
	}
	return nil
}

func (p *Packable) validateCapacityTypes(schedule *scheduling.Schedule) error {
	capacityTypes := sets.String{}
	for _, offering := range p.Offerings() {
		capacityTypes.Insert(offering.CapacityType)
	}
	if schedule.Requirements.CapacityTypes().Intersection(capacityTypes).Len() == 0 {
		return fmt.Errorf("capacity types %v are not in %v", capacityTypes, schedule.Requirements.CapacityTypes().List())
	}
	return nil
}

func (p *Packable) validateNvidiaGpus(schedule *scheduling.Schedule) error {
	if p.InstanceType.NvidiaGPUs().IsZero() {
		return nil
	}
	for _, pod := range schedule.Pods {
		for _, container := range pod.Spec.Containers {
			if _, ok := container.Resources.Requests[resources.NvidiaGPU]; ok {
				return nil
			}
		}
	}
	return fmt.Errorf("nvidia gpu is not required")
}

func (p *Packable) validateAMDGpus(schedule *scheduling.Schedule) error {
	if p.InstanceType.AMDGPUs().IsZero() {
		return nil
	}
	for _, pod := range schedule.Pods {
		for _, container := range pod.Spec.Containers {
			if _, ok := container.Resources.Requests[resources.AMDGPU]; ok {
				return nil
			}
		}
	}
	return fmt.Errorf("amd gpu is not required")
}

func (p *Packable) validateAWSNeurons(schedule *scheduling.Schedule) error {
	if p.InstanceType.AWSNeurons().IsZero() {
		return nil
	}
	for _, pod := range schedule.Pods {
		for _, container := range pod.Spec.Containers {
			if _, ok := container.Resources.Requests[resources.AWSNeuron]; ok {
				return nil
			}
		}
	}
	return fmt.Errorf("aws neuron is not required")
}

func packableNames(instanceTypes []*Packable) []string {
	names := []string{}
	for _, instanceType := range instanceTypes {
		names = append(names, instanceType.Name())
	}
	return names
}
