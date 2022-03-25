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
	"sort"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/utils/resources"
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

// PackablesFor creates viable packables for the provided constraints, excluding those that can't fit the kubelet
// or daemonsets. This assumes that instanceTypes has already been pre-filtered for pod compatibility.
func PackablesFor(ctx context.Context, instanceTypes []cloudprovider.InstanceType, daemons []*v1.Pod) []*Packable {
	var packables []*Packable
	for _, instanceType := range instanceTypes {
		packable := PackableFor(instanceType)
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
	// Sort in ascending order by price
	sort.Slice(packables, func(i, j int) bool {
		return packables[i].Price() < packables[j].Price()
	})
	return packables
}

func PackableFor(i cloudprovider.InstanceType) *Packable {
	return &Packable{
		InstanceType: i,
		total:        i.Resources().DeepCopy(),
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

// fits checks if adding the pod would overflow the total resources available. It also ensures that instance types that
// could not possibly satisfy the pod at all will be eliminated from consideration.
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

func packableNames(instanceTypes []*Packable) []string {
	var names []string
	for _, instanceType := range instanceTypes {
		names = append(names, instanceType.Name())
	}
	return names
}
