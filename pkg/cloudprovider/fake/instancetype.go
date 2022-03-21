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

	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/aws/karpenter/pkg/cloudprovider"
)

func NewInstanceType(options InstanceTypeOptions) *InstanceType {
	if options.Resources == nil {
		options.Resources = map[v1.ResourceName]resource.Quantity{}
	}
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
	if r := options.Resources[v1.ResourceCPU]; r.IsZero() {
		options.Resources[v1.ResourceCPU] = resource.MustParse("4")
	}
	if r := options.Resources[v1.ResourceMemory]; r.IsZero() {
		options.Resources[v1.ResourceMemory] = resource.MustParse("4Gi")
	}
	if r := options.Resources[v1.ResourcePods]; r.IsZero() {
		options.Resources[v1.ResourcePods] = resource.MustParse("5")
	}

	return &InstanceType{
		options: InstanceTypeOptions{
			Name:             options.Name,
			Offerings:        options.Offerings,
			Architecture:     options.Architecture,
			OperatingSystems: options.OperatingSystems,
			Resources:        options.Resources},
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
			Name: fmt.Sprintf("fake-it-%d", i),
			Resources: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", i+1)),
				v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", (i+1)*2)),
				v1.ResourcePods:   resource.MustParse(fmt.Sprintf("%d", (i+1)*10)),
			},
		}))
	}
	return instanceTypes
}

type InstanceTypeOptions struct {
	Name             string
	Offerings        []cloudprovider.Offering
	Architecture     string
	OperatingSystems sets.String
	Resources        v1.ResourceList
}

type InstanceType struct {
	options InstanceTypeOptions
}

func (i *InstanceType) Price() float64 {
	price := 0.0
	for k, v := range i.Resources() {
		switch k {
		case v1.ResourceCPU:
			price += 0.1 * v.AsApproximateFloat64()
		case v1.ResourceMemory:
			price += 0.1 * v.AsApproximateFloat64() / (1e9)
		case v1alpha1.ResourceNVIDIAGPU, v1alpha1.ResourceAMDGPU:
			price += 1.0
		}
	}
	return price
}

func (i *InstanceType) Resources() v1.ResourceList {
	return i.options.Resources
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

func (i *InstanceType) Overhead() v1.ResourceList {
	return v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse("100m"),
		v1.ResourceMemory: resource.MustParse("10Mi"),
	}
}
