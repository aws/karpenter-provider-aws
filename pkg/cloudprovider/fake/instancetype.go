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
	"fmt"

	"github.com/aws/karpenter/pkg/cloudprovider"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
)

func NewInstanceType(options InstanceTypeOptions) *InstanceType {
	if len(options.Offerings) == 0 {
		options.Offerings = []cloudprovider.Offering{
			{CapacityType: "spot", Zone: "test-zone-1"},
			{CapacityType: "spot", Zone: "test-zone-2"},
			{CapacityType: "on-demand", Zone: "test-zone-1"},
			{CapacityType: "on-demand", Zone: "test-zone-2"},
			{CapacityType: "on-demand", Zone: "test-zone-3"}}
	}
	if len(options.Architecture) == 0 {
		options.Architecture = "amd64"
	}
	if options.OperatingSystems.Len() == 0 {
		options.OperatingSystems = sets.NewString("linux", "windows", "darwin")
	}
	if options.CPU.IsZero() {
		options.CPU = resource.MustParse("4")
	}
	if options.Memory.IsZero() {
		options.Memory = resource.MustParse("4Gi")
	}
	if options.Pods.IsZero() {
		options.Pods = resource.MustParse("5")
	}
	return &InstanceType{
		options: InstanceTypeOptions{
			Name:             options.Name,
			Offerings:        options.Offerings,
			Architecture:     options.Architecture,
			OperatingSystems: options.OperatingSystems,
			CPU:              options.CPU,
			Memory:           options.Memory,
			Pods:             options.Pods,
			NvidiaGPUs:       options.NvidiaGPUs,
			AMDGPUs:          options.AMDGPUs,
			AWSNeurons:       options.AWSNeurons,
			AWSPodENI:        options.AWSPodENI,
		},
	}
}

// InstanceTypes creates instance types with incrementing resources
// 2Gi of RAM and 10 pods for every 1vcpu
// i.e. 1vcpu, 2Gi mem, 10 pods
//      2vcpu, 4Gi mem, 20 pods
//      3vcpu, 6Gi mem, 30 pods
func InstanceTypes(total int) []cloudprovider.InstanceType {
	instanceTypes := []cloudprovider.InstanceType{}
	for i := 0; i < total; i++ {
		instanceTypes = append(instanceTypes, NewInstanceType(InstanceTypeOptions{
			Name:   fmt.Sprintf("fake-it-%d", i),
			CPU:    resource.MustParse(fmt.Sprintf("%d", i+1)),
			Memory: resource.MustParse(fmt.Sprintf("%dGi", (i+1)*2)),
			Pods:   resource.MustParse(fmt.Sprintf("%d", (i+1)*10)),
		}))
	}
	return instanceTypes
}

type InstanceTypeOptions struct {
	Name             string
	Offerings        []cloudprovider.Offering
	Architecture     string
	OperatingSystems sets.String
	CPU              resource.Quantity
	Memory           resource.Quantity
	Pods             resource.Quantity
	NvidiaGPUs       resource.Quantity
	AMDGPUs          resource.Quantity
	AWSNeurons       resource.Quantity
	AWSPodENI        resource.Quantity
}

type InstanceType struct {
	options InstanceTypeOptions
}

func (i *InstanceType) Name() string {
	return i.options.Name
}

func (i *InstanceType) Offerings() []cloudprovider.Offering {
	return i.options.Offerings
}

func (i *InstanceType) Architecture() string {
	return i.options.Architecture
}

func (i *InstanceType) OperatingSystems() sets.String {
	return i.options.OperatingSystems
}

func (i *InstanceType) CPU() *resource.Quantity {
	return &i.options.CPU
}

func (i *InstanceType) Memory() *resource.Quantity {
	return &i.options.Memory
}

func (i *InstanceType) Pods() *resource.Quantity {
	return &i.options.Pods
}

func (i *InstanceType) NvidiaGPUs() *resource.Quantity {
	return &i.options.NvidiaGPUs
}

func (i *InstanceType) AMDGPUs() *resource.Quantity {
	return &i.options.AMDGPUs
}

func (i *InstanceType) AWSNeurons() *resource.Quantity {
	return &i.options.AWSNeurons
}

func (i *InstanceType) AWSPodENI() *resource.Quantity {
	return &i.options.AWSPodENI
}

func (i *InstanceType) Overhead() v1.ResourceList {
	return v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse("100m"),
		v1.ResourceMemory: resource.MustParse("10Mi"),
	}
}
