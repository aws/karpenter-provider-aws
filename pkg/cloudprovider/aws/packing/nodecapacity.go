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

	"github.com/aws/aws-sdk-go/service/ec2"
	resourcesUtil "github.com/awslabs/karpenter/pkg/utils/resources"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// TODO get this information from node-instance-selector
var (
	fallbackNodeCapacities = []*nodeCapacity{

		{
			instanceType: "m5.24xlarge",
			total: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("96000m"),
				v1.ResourceMemory: resource.MustParse("384Gi"),
				v1.ResourcePods:   resource.MustParse("737"),
			},
			reserved: v1.ResourceList{
				v1.ResourceCPU:    resource.Quantity{},
				v1.ResourceMemory: resource.Quantity{},
			},
		},
		{
			instanceType: "m5.8xlarge",
			total: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("32000m"),
				v1.ResourceMemory: resource.MustParse("128Gi"),
				v1.ResourcePods:   resource.MustParse("234"),
			},
			reserved: v1.ResourceList{
				v1.ResourceCPU:    resource.Quantity{},
				v1.ResourceMemory: resource.Quantity{},
			},
		},
		{
			instanceType: "m5.2xlarge",
			total: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("8000m"),
				v1.ResourceMemory: resource.MustParse("32Gi"),
				v1.ResourcePods:   resource.MustParse("58"),
			},
			reserved: v1.ResourceList{
				v1.ResourceCPU:    resource.Quantity{},
				v1.ResourceMemory: resource.Quantity{},
			},
		},
		{
			instanceType: "m5.xlarge",
			total: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("4000m"),
				v1.ResourceMemory: resource.MustParse("16Gi"),
				v1.ResourcePods:   resource.MustParse("58"),
			},
			reserved: v1.ResourceList{
				v1.ResourceCPU:    resource.Quantity{},
				v1.ResourceMemory: resource.Quantity{},
			},
		},
		{
			instanceType: "m5.large",
			total: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("2000m"),
				v1.ResourceMemory: resource.MustParse("8Gi"),
				v1.ResourcePods:   resource.MustParse("29"),
			},
			reserved: v1.ResourceList{
				v1.ResourceCPU:    resource.Quantity{},
				v1.ResourceMemory: resource.Quantity{},
			},
		},
	}
)

func instanceTypeInfoToNodeCapacity(instanceTypeInfo ec2.InstanceTypeInfo) (*nodeCapacity, error) {
	instanceTypeName := *instanceTypeInfo.InstanceType
	vcpusInMillicores := fmt.Sprintf("%dm", *instanceTypeInfo.VCpuInfo.DefaultVCpus*1000)
	vcpusResource, err := resource.ParseQuantity(vcpusInMillicores)
	if err != nil {
		return nil, fmt.Errorf("parsing %s millicores resource quantity \"%s\" from instanceTypeInfo, %w", instanceTypeName, vcpusInMillicores, err)
	}
	memory := fmt.Sprintf("%dMi", *instanceTypeInfo.MemoryInfo.SizeInMiB)
	memoryResource, err := resource.ParseQuantity(memory)
	if err != nil {
		return nil, fmt.Errorf("parsing %s memory resource quantity \"%s\", %w", instanceTypeName, memory, err)
	}
	// The number of pods per node is calculated using the formula:
	//   max number of ENIs * (IPv4 Addresses per ENI -1) + 2
	// https://github.com/awslabs/amazon-eks-ami/blob/master/files/eni-max-pods.txt#L20
	podCapacity := *instanceTypeInfo.NetworkInfo.MaximumNetworkInterfaces*(*instanceTypeInfo.NetworkInfo.Ipv4AddressesPerInterface-1) + 2
	podCapacityResource, err := resource.ParseQuantity(fmt.Sprint(podCapacity))
	if err != nil {
		return nil, fmt.Errorf("parsing %s pod capacity resource quantity \"%d\", %w", instanceTypeName, podCapacity, err)
	}

	return &nodeCapacity{
		instanceType: instanceTypeName,
		total: v1.ResourceList{
			v1.ResourceCPU:    vcpusResource,
			v1.ResourceMemory: memoryResource,
			v1.ResourcePods:   podCapacityResource,
		},
		reserved: v1.ResourceList{
			v1.ResourceCPU:    resource.Quantity{},
			v1.ResourceMemory: resource.Quantity{},
		},
	}, nil
}

type nodeCapacity struct {
	instanceType string
	reserved     v1.ResourceList
	total        v1.ResourceList
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
