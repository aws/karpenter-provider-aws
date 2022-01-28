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
	"sort"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/utils/resources"
	"go.uber.org/multierr"
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
func PackablesFor(ctx context.Context, instanceTypes []cloudprovider.InstanceType, constraints *v1alpha5.Constraints, pods []*v1.Pod, daemons []*v1.Pod) []*Packable {
	packables := []*Packable{}
	for _, instanceType := range instanceTypes {
		packable := PackableFor(instanceType)
		// First pass at filtering down to viable instance types;
		// additional filtering will be done by later steps (such as
		// removing instance types that obviously lack resources, such
		// as GPUs, for the workload being presented).
		if err := multierr.Combine(
			packable.validateOfferings(constraints),
			packable.validateInstanceType(constraints),
			packable.validateArchitecture(constraints),
			packable.validateOperatingSystems(constraints),
			packable.validateAWSPodENI(pods),
			packable.validateGPUs(pods),
		); err != nil {
			continue
		}
		// Calculate Kubelet Overhead
		if ok := packable.reserve(instanceType.Overhead()); !ok {
			logging.FromContext(ctx).Debugf("Excluding instance type %s because there are not enough resources for kubelet and system overhead", packable.Name())
			continue
		}
		// Calculate Daemonset Overhead
		if len(packable.Pack(daemons).unpacked) > 0 {
			logging.FromContext(ctx).Debugf("Excluding instance type %s because there are not enough resources for daemons", packable.Name())
			continue
		}
		packables = append(packables, packable)
	}
	// Sort in ascending order so that the packer can short circuit bin-packing for larger instance types
	sort.Slice(packables, func(i, j int) bool {
		// Check GPU equality assuming GPU classes are mutually exclusive
		if packables[i].AMDGPUs().Equal(*packables[j].AMDGPUs()) ||
			packables[i].NvidiaGPUs().Equal(*packables[j].NvidiaGPUs()) ||
			packables[i].AWSNeurons().Equal(*packables[j].AWSNeurons()) {
			if packables[i].CPU().Equal(*packables[j].CPU()) {
				// check for memory
				return packables[i].Memory().Cmp(*packables[j].Memory()) == -1
			}
			return packables[i].CPU().Cmp(*packables[j].CPU()) == -1
		}
		return packables[i].AMDGPUs().Cmp(*packables[j].AMDGPUs()) == -1 ||
			packables[i].NvidiaGPUs().Cmp(*packables[j].NvidiaGPUs()) == -1 ||
			packables[i].AWSNeurons().Cmp(*packables[j].AWSNeurons()) == -1
	})
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
			resources.AWSPodENI: *i.AWSPodENI(),
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

func (p *Packable) DeepCopy() *Packable {
	return &Packable{
		InstanceType: p.InstanceType,
		reserved:     p.reserved.DeepCopy(),
		total:        p.total.DeepCopy(),
	}
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

func (p *Packable) validateInstanceType(constraints *v1alpha5.Constraints) error {
	if !constraints.Requirements.InstanceTypes().Has(p.Name()) {
		return fmt.Errorf("instance type %s not in %s", p.Name(), constraints.Requirements.InstanceTypes())
	}
	return nil
}

func (p *Packable) validateArchitecture(constraints *v1alpha5.Constraints) error {
	if !constraints.Requirements.Architectures().Has(p.Architecture()) {
		return fmt.Errorf("architecture %s not in %s", p.Name(), constraints.Requirements.Architectures())
	}
	return nil
}

func (p *Packable) validateOperatingSystems(constraints *v1alpha5.Constraints) error {
	if constraints.Requirements.OperatingSystems().Intersection(p.OperatingSystems()).Len() == 0 {
		return fmt.Errorf("operating systems %s not in %s", p.Name(), constraints.Requirements.OperatingSystems())
	}
	return nil
}

func (p *Packable) validateOfferings(constraints *v1alpha5.Constraints) error {
	for _, offering := range p.Offerings() {
		if constraints.Requirements.CapacityTypes().Has(offering.CapacityType) && constraints.Requirements.Zones().Has(offering.Zone) {
			return nil
		}
	}
	return fmt.Errorf("offerings %v are not available for capacity types %s and zones %s", p.Offerings(), constraints.Requirements.CapacityTypes(), constraints.Requirements.Zones())
}

func (p *Packable) validateGPUs(pods []*v1.Pod) error {
	gpuResources := map[v1.ResourceName]*resource.Quantity{
		resources.NvidiaGPU: p.InstanceType.NvidiaGPUs(),
		resources.AMDGPU:    p.InstanceType.AMDGPUs(),
		resources.AWSNeuron: p.InstanceType.AWSNeurons(),
	}
	for resourceName, instanceTypeResourceQuantity := range gpuResources {
		if p.requiresResource(pods, resourceName) && instanceTypeResourceQuantity.IsZero() {
			return fmt.Errorf("%s is required", resourceName)
		} else if !p.requiresResource(pods, resourceName) && !instanceTypeResourceQuantity.IsZero() {
			return fmt.Errorf("%s is not required", resourceName)
		}
	}
	return nil
}

func (p *Packable) requiresResource(pods []*v1.Pod, resource v1.ResourceName) bool {
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			if _, ok := container.Resources.Requests[resource]; ok {
				return true
			}
			if _, ok := container.Resources.Limits[resource]; ok {
				return true
			}
		}
	}
	return false
}

func (p *Packable) validateAWSPodENI(pods []*v1.Pod) error {
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			if _, ok := container.Resources.Requests[resources.AWSPodENI]; ok {
				if p.InstanceType.AWSPodENI().IsZero() {
					return fmt.Errorf("aws pod eni is required")
				}
				return nil
			}
		}
	}
	return nil
}

func packableNames(instanceTypes []*Packable) []string {
	names := []string{}
	for _, instanceType := range instanceTypes {
		names = append(names, instanceType.Name())
	}
	return names
}
