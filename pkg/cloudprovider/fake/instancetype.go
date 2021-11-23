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

package fake

import (
	"github.com/awslabs/karpenter/pkg/cloudprovider"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func NewInstanceType(options InstanceTypeOptions) *InstanceType {
	if len(options.offerings) == 0 {
		options.offerings = []cloudprovider.Offering{
			{CapacityType: "spot", Zone: "test-zone-1"},
			{CapacityType: "spot", Zone: "test-zone-2"},
			{CapacityType: "on-demand", Zone: "test-zone-1"},
			{CapacityType: "on-demand", Zone: "test-zone-2"},
			{CapacityType: "on-demand", Zone: "test-zone-3"}}
	}
	if len(options.architecture) == 0 {
		options.architecture = "amd64"
	}
	if options.cpu.IsZero() {
		options.cpu = resource.MustParse("4")
	}
	if options.memory.IsZero() {
		options.memory = resource.MustParse("4Gi")
	}
	if options.pods.IsZero() {
		options.pods = resource.MustParse("5")
	}
	return &InstanceType{
		InstanceTypeOptions: InstanceTypeOptions{
			name:         options.name,
			offerings:    options.offerings,
			architecture: options.architecture,
			cpu:          options.cpu,
			memory:       options.memory,
			pods:         options.pods,
			nvidiaGPUs:   options.nvidiaGPUs,
			amdGPUs:      options.amdGPUs,
			awsNeurons:   options.awsNeurons,
		},
	}
}

type InstanceTypeOptions struct {
	name         string
	offerings    []cloudprovider.Offering
	architecture string
	cpu          resource.Quantity
	memory       resource.Quantity
	pods         resource.Quantity
	nvidiaGPUs   resource.Quantity
	amdGPUs      resource.Quantity
	awsNeurons   resource.Quantity
}

type InstanceType struct {
	InstanceTypeOptions
}

func (i *InstanceType) Name() string {
	return i.name
}

func (i *InstanceType) Offerings() []cloudprovider.Offering {
	return i.offerings
}

func (i *InstanceType) Architecture() string {
	return i.architecture
}

func (i *InstanceType) CPU() *resource.Quantity {
	return &i.cpu
}

func (i *InstanceType) Memory() *resource.Quantity {
	return &i.memory
}

func (i *InstanceType) Pods() *resource.Quantity {
	return &i.pods
}

func (i *InstanceType) NvidiaGPUs() *resource.Quantity {
	return &i.nvidiaGPUs
}

func (i *InstanceType) AMDGPUs() *resource.Quantity {
	return &i.amdGPUs
}

func (i *InstanceType) AWSNeurons() *resource.Quantity {
	return &i.awsNeurons
}

func (i *InstanceType) Overhead() v1.ResourceList {
	return v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse("100m"),
		v1.ResourceMemory: resource.MustParse("10Mi"),
	}
}
