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
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/binpacking"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
)

type podPacker struct {
	// TODO use this ec2 API to get the instance types
	ec2         ec2iface.EC2API
	reserveOnce sync.Once
}

// Packer helps pack the pods and calculates efficient placement on the instances.
type Packer interface {
	// TODO use ctx when calling ec2 API
	Pack(ctx context.Context, constraints *cloudprovider.Constraints) ([]*Packing, error)
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
func (p *podPacker) Pack(ctx context.Context, constraints *cloudprovider.Constraints) ([]*Packing, error) {
	// TODO use ctx when calling ec2 API
	// Sort pods in decreasing order by the amount of CPU requested, if
	// CPU requested is equal compare memory requested.
	sort.Sort(sort.Reverse(binpacking.ByResourcesRequested{SortablePods: constraints.Pods}))
	packings := []*Packing{}
	var packing *Packing
	remainingPods := constraints.Pods
	for len(remainingPods) > 0 {
		packing, remainingPods = p.packWithLargestPod(remainingPods, constraints)
		// checked all instance type and found no packing option
		if len(packing.Pods) == 0 {
			zap.S().Warnf("Failed to find instance type for pod %s/%s ", remainingPods[0].Namespace, remainingPods[0].Name)
			remainingPods = remainingPods[1:]
			continue
		}
		packings = append(packings, packing)
		zap.S().Debugf("For %d pod(s) instance types selected are %v", len(packing.Pods), packing.InstanceTypes)
	}
	return packings, nil
}

// TODO filter instance types based on node contraints like availability zones etc.
func (p *podPacker) getNodeCapacities(constraints *cloudprovider.Constraints) []*nodeCapacity {
	p.reserveOnce.Do(func() {
		for _, nc := range nodeCapacities {
			kubeletOverhead := binpacking.CalculateKubeletOverhead(nc.total)
			if ok := nc.reserve(binpacking.MergeResources(constraints.Overhead, kubeletOverhead)); !ok {
				zap.S().Errorf("Failed to reserve kubelet overhead for node capacity type %v", nc.instanceType)
			}
		}
	})
	nodeCapacitiesCopy := make([]*nodeCapacity, 0)
	for _, nc := range nodeCapacities {
		nodeCapacitiesCopy = append(nodeCapacitiesCopy, &nodeCapacity{nc.instanceType, nc.reserved.DeepCopy(), nc.total.DeepCopy()})
	}
	return nodeCapacitiesCopy
}

type packingsPerCapacity struct {
	packed   []*v1.Pod
	unpacked []*v1.Pod
}

// packWithLargestPod will try to pack max number of pods with largest pod in
// pods across all available node capacities. It returns Packing: max pod count
// that fit; with their node capacities and list of leftover pods
func (p *podPacker) packWithLargestPod(unpackedPods []*v1.Pod, constraints *cloudprovider.Constraints) (*Packing, []*v1.Pod) {
	bestPackedPods := []*v1.Pod{}
	bestCapacities := []*nodeCapacity{}
	remainingPods := unpackedPods
	for _, nc := range p.getNodeCapacities(constraints) {
		// check how many pods we can fit with the available capacity
		result := p.packPodsForCapacity(nc, unpackedPods)
		if len(result.packed) == 0 {
			continue
		}
		// If the pods packed are the same as before, this instance type can be
		// considered as a backup option in case we get ICE
		if podsMatch(bestPackedPods, result.packed) {
			bestCapacities = append(bestCapacities, nc)
		} else if len(result.packed) > len(bestPackedPods) {
			// If pods packed are more than compared to what we got in last
			// iteration, consider using this instance type
			bestPackedPods = result.packed
			remainingPods = result.unpacked
			bestCapacities = []*nodeCapacity{nc}
		}
	}
	capacityNames := []string{}
	for _, capacity := range bestCapacities {
		capacityNames = append(capacityNames, capacity.instanceType)
	}
	return &Packing{Pods: bestPackedPods, InstanceTypes: capacityNames}, remainingPods
}

func (p *podPacker) packPodsForCapacity(capacity *nodeCapacity, pods []*v1.Pod) *packingsPerCapacity {
	// start with the largest pod based on resources requested
	result := &packingsPerCapacity{packed: []*v1.Pod{}, unpacked: []*v1.Pod{}}
	for _, pod := range pods {
		if ok := capacity.reserveForPod(&pod.Spec); ok {
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
