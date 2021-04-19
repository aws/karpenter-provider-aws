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
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/binpacking"
	"github.com/awslabs/karpenter/pkg/utils/resources"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

func PackablesFrom(instanceTypes []cloudprovider.InstanceType, overhead v1.ResourceList) []*Packable {
	packables := []*Packable{}
	for _, instanceType := range instanceTypes {
		packable := PackableFrom(instanceType)
		if ok := packable.reserve(resources.Merge(overhead, binpacking.CalculateKubeletOverhead(packable.total))); !ok {
			zap.S().Debugf("Excluding instance type %s because there are not enough resources for the kubelet overhead", packable.Name())
			continue
		}
		packables = append(packables, packable)
	}
	return packables
}

func PackableFrom(instance cloudprovider.InstanceType) *Packable {
	return &Packable{
		InstanceType: instance,
		total: v1.ResourceList{
			v1.ResourceCPU:      *instance.CPU(),
			v1.ResourceMemory:   *instance.Memory(),
			resources.NvidiaGPU: *instance.NvidiaGPUs(),
			resources.AWSNeuron: *instance.AWSNeurons(),
			v1.ResourcePods:     *instance.Pods(),
		},
	}
}

func (p *Packable) Pack(pods []*v1.Pod) *Result {
	result := &Result{}
	for _, pod := range pods {
		if ok := p.reserveForPod(pod); ok {
			result.packed = append(result.packed, pod)
			continue
		}
		// if largest pod can't be packed try next node capacity
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

func (p *Packable) reserveForPod(pod *v1.Pod) bool {
	requests := resources.RequestsForPods(pod)
	requests[v1.ResourcePods] = *resource.NewQuantity(1, resource.BinarySI)
	return p.reserve(requests)
}
