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
	"strings"

	"github.com/samber/lo"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/scheduling"
	"github.com/aws/karpenter/pkg/utils/sets"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	utilsets "k8s.io/apimachinery/pkg/util/sets"

	"github.com/aws/karpenter/pkg/cloudprovider"
)

const (
	InstanceSizeLabelKey   = "size"
	ExoticInstanceLabelKey = "special"
)

func init() {
	v1alpha5.WellKnownLabels.Insert(
		InstanceSizeLabelKey,
		ExoticInstanceLabelKey,
	)
}

func NewInstanceType(options InstanceTypeOptions) *InstanceType {
	if options.Resources == nil {
		options.Resources = map[v1.ResourceName]resource.Quantity{}
	}
	if options.Overhead == nil {
		options.Overhead = v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse("100m"),
			v1.ResourceMemory: resource.MustParse("10Mi"),
		}
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
		options.OperatingSystems = utilsets.NewString("linux", "windows", "darwin")
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
			Resources:        options.Resources,
			Overhead:         options.Overhead,
			Price:            options.Price},
	}
}

// InstanceTypesAssorted create many unique instance types with varying CPU/memory/architecture/OS/zone/capacity type.
func InstanceTypesAssorted() []cloudprovider.InstanceType {
	var instanceTypes []cloudprovider.InstanceType
	for _, cpu := range []int{1, 2, 4, 8, 16, 32, 64} {
		for _, mem := range []int{1, 2, 4, 8, 16, 32, 64, 128} {
			for _, zone := range []string{"test-zone-1", "test-zone-2", "test-zone-3"} {
				for _, ct := range []string{v1alpha1.CapacityTypeSpot, v1alpha1.CapacityTypeOnDemand} {
					for _, os := range []utilsets.String{utilsets.NewString("linux"), utilsets.NewString("windows")} {
						for _, arch := range []string{v1alpha5.ArchitectureAmd64, v1alpha5.ArchitectureArm64} {
							it := NewInstanceType(InstanceTypeOptions{
								Name:             fmt.Sprintf("%d-cpu-%d-mem-%s-%s-%s-%s", cpu, mem, arch, strings.Join(os.List(), ","), zone, ct),
								Architecture:     arch,
								OperatingSystems: os,
								Resources: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", cpu)),
									v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", mem)),
								},
								Offerings: []cloudprovider.Offering{
									{
										CapacityType: ct,
										Zone:         zone,
									},
								},
							})
							instanceTypes = append(instanceTypes, it)
						}
					}
				}
			}
		}
	}
	return instanceTypes
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
	OperatingSystems utilsets.String
	Overhead         v1.ResourceList
	Resources        v1.ResourceList
	Price            float64
}

type InstanceType struct {
	options InstanceTypeOptions
}

func (i *InstanceType) Name() string {
	return i.options.Name
}

func (i *InstanceType) Price() float64 {
	if i.options.Price != 0 {
		return i.options.Price
	}

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

func (i *InstanceType) Offerings() []cloudprovider.Offering {
	return i.options.Offerings
}

func (i *InstanceType) Overhead() v1.ResourceList {
	return i.options.Overhead
}

func (i *InstanceType) Requirements() scheduling.Requirements {
	requirements := scheduling.Requirements{
		v1.LabelInstanceTypeStable: sets.NewSet(i.options.Name),
		v1.LabelArchStable:         sets.NewSet(i.options.Architecture),
		v1.LabelOSStable:           sets.NewSet(i.options.OperatingSystems.List()...),
		v1.LabelTopologyZone:       sets.NewSet(lo.Map(i.Offerings(), func(o cloudprovider.Offering, _ int) string { return o.Zone })...),
		v1alpha5.LabelCapacityType: sets.NewSet(lo.Map(i.Offerings(), func(o cloudprovider.Offering, _ int) string { return o.CapacityType })...),
	}
	if i.options.Resources.Cpu().Cmp(resource.MustParse("4")) > 0 &&
		i.options.Resources.Memory().Cmp(resource.MustParse("8Gi")) > 0 {
		requirements.Add(scheduling.Requirements{InstanceSizeLabelKey: sets.NewSet("large")})
		requirements.Add(scheduling.Requirements{ExoticInstanceLabelKey: sets.NewSet("optional")})
	} else {
		requirements.Add(scheduling.Requirements{InstanceSizeLabelKey: sets.NewSet("small")})
	}
	return requirements
}
