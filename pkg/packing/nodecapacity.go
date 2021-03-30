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

	resourcesUtil "github.com/awslabs/karpenter/pkg/utils/resources"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type nodeCapacity struct {
	instanceType *Instance
	reserved     v1.ResourceList
	total        v1.ResourceList
}

func nodeCapacityFrom(instanceType *Instance) *nodeCapacity {
	return &nodeCapacity{
		instanceType: instanceType,
		total: v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse(fmt.Sprint(*instanceType.VCpuInfo.DefaultVCpus)),
			v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", *instanceType.MemoryInfo.SizeInMiB)),
			// The number of pods per node is calculated using the formula:
			// max number of ENIs * (IPv4 Addresses per ENI -1) + 2
			// https://github.com/awslabs/amazon-eks-ami/blob/master/files/eni-max-pods.txt#L20
			v1.ResourcePods: resource.MustParse(fmt.Sprint(
				*instanceType.NetworkInfo.MaximumNetworkInterfaces*(*instanceType.NetworkInfo.Ipv4AddressesPerInterface-1) + 2)),
		},
	}
}

func (nc *nodeCapacity) Copy() *nodeCapacity {
	return &nodeCapacity{nc.instanceType, nc.reserved.DeepCopy(), nc.total.DeepCopy()}
}

func (nc *nodeCapacity) reserve(resources v1.ResourceList) bool {
	targetUtilization := resourcesUtil.Merge(nc.reserved, resources)
	// If pod fits reserve the capacity
	if nc.total.Cpu().Cmp(*targetUtilization.Cpu()) >= 0 &&
		nc.total.Memory().Cmp(*targetUtilization.Memory()) >= 0 &&
		nc.total.Pods().Cmp(*targetUtilization.Pods()) >= 0 {
		nc.reserved = targetUtilization
		return true
	}
	return false
}

func (nc *nodeCapacity) reserveForPod(podSpec *v1.PodSpec) bool {
	resources := resourcesUtil.ForPods(podSpec)
	resources[v1.ResourcePods] = *resource.NewQuantity(1, resource.BinarySI)
	return nc.reserve(resources)
}
