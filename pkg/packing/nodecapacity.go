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
	"fmt"

	"github.com/awslabs/karpenter/pkg/utils/resources"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type nodeCapacity struct {
	instanceType *Instance
	reserved     v1.ResourceList
	total        v1.ResourceList
}

func nodeCapacityFrom(instanceType *Instance) *nodeCapacity {
	// The number of pods per node is calculated using the formula:
	// max number of ENIs * (IPv4 Addresses per ENI -1) + 2
	// https://github.com/awslabs/amazon-eks-ami/blob/master/files/eni-max-pods.txt#L20
	podResources := *instanceType.NetworkInfo.MaximumNetworkInterfaces*(*instanceType.NetworkInfo.Ipv4AddressesPerInterface-1) + 2
	return &nodeCapacity{
		instanceType: instanceType,
		total: v1.ResourceList{
			v1.ResourceCPU:      resource.MustParse(fmt.Sprint(*instanceType.VCpuInfo.DefaultVCpus)),
			v1.ResourceMemory:   resource.MustParse(fmt.Sprintf("%dMi", *instanceType.MemoryInfo.SizeInMiB)),
			resources.NvidiaGPU: resource.MustParse(fmt.Sprint(countNvidiaGPUs(instanceType))),
			resources.AWSNeuron: resource.MustParse(fmt.Sprint(countAWSNeurons(instanceType))),
			v1.ResourcePods:     resource.MustParse(fmt.Sprint(podResources)),
		},
	}
}

func (nc *nodeCapacity) Copy() *nodeCapacity {
	return &nodeCapacity{nc.instanceType, nc.reserved.DeepCopy(), nc.total.DeepCopy()}
}

func (nc *nodeCapacity) reserve(requests v1.ResourceList) bool {
	candidate := resources.Merge(nc.reserved, requests)
	// If any candidate resource exceeds total, fail to reserve
	for resourceName, quantity := range candidate {
		if quantity.Cmp(nc.total[resourceName]) > 0 {
			return false
		}
	}
	nc.reserved = candidate
	return true
}

func (nc *nodeCapacity) reserveForPod(pod *v1.Pod) bool {
	requests := resources.RequestsForPods(pod)
	requests[v1.ResourcePods] = *resource.NewQuantity(1, resource.BinarySI)
	return nc.reserve(requests)
}
