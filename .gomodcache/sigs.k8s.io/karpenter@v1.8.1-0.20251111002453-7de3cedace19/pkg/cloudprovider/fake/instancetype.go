/*
Copyright The Kubernetes Authors.

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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
)

const (
	LabelInstanceSize                           = "size"
	ExoticInstanceLabelKey                      = "special"
	IntegerInstanceLabelKey                     = "integer"
	ResourceGPUVendorA      corev1.ResourceName = "fake.com/vendor-a"
	ResourceGPUVendorB      corev1.ResourceName = "fake.com/vendor-b"
)

func init() {
	v1.WellKnownLabels.Insert(
		LabelInstanceSize,
		ExoticInstanceLabelKey,
		IntegerInstanceLabelKey,
	)
}

func NewInstanceType(options InstanceTypeOptions) *cloudprovider.InstanceType {
	return NewInstanceTypeWithCustomRequirement(options, nil)
}

func NewInstanceTypeWithCustomRequirement(options InstanceTypeOptions, customReq *scheduling.Requirement) *cloudprovider.InstanceType {
	if options.Resources == nil {
		options.Resources = map[corev1.ResourceName]resource.Quantity{}
	}
	if r := options.Resources[corev1.ResourceCPU]; r.IsZero() {
		options.Resources[corev1.ResourceCPU] = resource.MustParse("4")
	}
	if r := options.Resources[corev1.ResourceMemory]; r.IsZero() {
		options.Resources[corev1.ResourceMemory] = resource.MustParse("4Gi")
	}
	if r := options.Resources[corev1.ResourcePods]; r.IsZero() {
		options.Resources[corev1.ResourcePods] = resource.MustParse("5")
	}
	if len(options.Offerings) == 0 {
		options.Offerings = []*cloudprovider.Offering{
			{
				Available: true,
				Requirements: scheduling.NewLabelRequirements(map[string]string{
					v1.CapacityTypeLabelKey:  "spot",
					corev1.LabelTopologyZone: "test-zone-1",
				}),
				Price: PriceFromResources(options.Resources),
			},
			{
				Available: true,
				Requirements: scheduling.NewLabelRequirements(map[string]string{
					v1.CapacityTypeLabelKey:  "spot",
					corev1.LabelTopologyZone: "test-zone-2",
				}),
				Price: PriceFromResources(options.Resources),
			},
			{
				Available: true,
				Requirements: scheduling.NewLabelRequirements(map[string]string{
					v1.CapacityTypeLabelKey:  "on-demand",
					corev1.LabelTopologyZone: "test-zone-1",
				}),
				Price: PriceFromResources(options.Resources),
			},
			{
				Available: true,
				Requirements: scheduling.NewLabelRequirements(map[string]string{
					v1.CapacityTypeLabelKey:  "on-demand",
					corev1.LabelTopologyZone: "test-zone-2",
				}),
				Price: PriceFromResources(options.Resources),
			},
			{
				Available: true,
				Requirements: scheduling.NewLabelRequirements(map[string]string{
					v1.CapacityTypeLabelKey:  "on-demand",
					corev1.LabelTopologyZone: "test-zone-3",
				}),
				Price: PriceFromResources(options.Resources),
			},
		}
	}
	if len(options.Architecture) == 0 {
		options.Architecture = "amd64"
	}
	if options.OperatingSystems.Len() == 0 {
		options.OperatingSystems = sets.New(string(corev1.Linux), string(corev1.Windows), "darwin")
	}
	requirements := scheduling.NewRequirements(
		scheduling.NewRequirement(corev1.LabelInstanceTypeStable, corev1.NodeSelectorOpIn, options.Name),
		scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, options.Architecture),
		scheduling.NewRequirement(corev1.LabelOSStable, corev1.NodeSelectorOpIn, sets.List(options.OperatingSystems)...),
		scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, lo.Map(options.Offerings.Available(), func(o *cloudprovider.Offering, _ int) string {
			return o.Requirements.Get(corev1.LabelTopologyZone).Any()
		})...),
		scheduling.NewRequirement(v1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, lo.Map(options.Offerings.Available(), func(o *cloudprovider.Offering, _ int) string {
			return o.Requirements.Get(v1.CapacityTypeLabelKey).Any()
		})...),
		scheduling.NewRequirement(LabelInstanceSize, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(ExoticInstanceLabelKey, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(IntegerInstanceLabelKey, corev1.NodeSelectorOpIn, fmt.Sprint(options.Resources.Cpu().Value())),
	)
	if customReq != nil {
		requirements.Add(customReq)
	}
	if options.Resources.Cpu().Cmp(resource.MustParse("4")) > 0 &&
		options.Resources.Memory().Cmp(resource.MustParse("8Gi")) > 0 {
		requirements.Get(LabelInstanceSize).Insert("large")
		requirements.Get(ExoticInstanceLabelKey).Insert("optional")
	} else {
		requirements.Get(LabelInstanceSize).Insert("small")
	}

	return &cloudprovider.InstanceType{
		Name:         options.Name,
		Requirements: requirements,
		Offerings:    options.Offerings,
		Capacity:     options.Resources,
		Overhead: &cloudprovider.InstanceTypeOverhead{
			KubeReserved: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("10Mi"),
			},
		},
	}
}

// InstanceTypesAssorted create many unique instance types with varying CPU/memory/architecture/OS/zone/capacity type.
func InstanceTypesAssorted() []*cloudprovider.InstanceType {
	var instanceTypes []*cloudprovider.InstanceType
	for _, cpu := range []int{1, 2, 4, 8, 16, 32, 64} {
		for _, mem := range []int{1, 2, 4, 8, 16, 32, 64, 128} {
			for _, zone := range []string{"test-zone-1", "test-zone-2", "test-zone-3"} {
				for _, ct := range []string{v1.CapacityTypeSpot, v1.CapacityTypeOnDemand} {
					for _, os := range []sets.Set[string]{sets.New(string(corev1.Linux)), sets.New(string(corev1.Windows))} {
						for _, arch := range []string{v1.ArchitectureAmd64, v1.ArchitectureArm64} {
							opts := InstanceTypeOptions{
								Name:             fmt.Sprintf("%d-cpu-%d-mem-%s-%s-%s-%s", cpu, mem, arch, strings.Join(sets.List(os), ","), zone, ct),
								Architecture:     arch,
								OperatingSystems: os,
								Resources: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", cpu)),
									corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", mem)),
								},
							}
							price := PriceFromResources(opts.Resources)
							opts.Offerings = []*cloudprovider.Offering{
								{
									Available: true,
									Requirements: scheduling.NewLabelRequirements(map[string]string{
										v1.CapacityTypeLabelKey:  ct,
										corev1.LabelTopologyZone: zone,
									}),
									Price: price,
								},
							}
							instanceTypes = append(instanceTypes, NewInstanceType(opts))
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
//
//	2vcpu, 4Gi mem, 20 pods
//	3vcpu, 6Gi mem, 30 pods
func InstanceTypes(total int) []*cloudprovider.InstanceType {
	instanceTypes := []*cloudprovider.InstanceType{}
	for i := 0; i < total; i++ {
		instanceTypes = append(instanceTypes, NewInstanceType(InstanceTypeOptions{
			Name: fmt.Sprintf("fake-it-%d", i),
			Resources: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%d", i+1)),
				corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", (i+1)*2)),
				corev1.ResourcePods:   resource.MustParse(fmt.Sprintf("%d", (i+1)*10)),
			},
		}))
	}
	return instanceTypes
}

type InstanceTypeOptions struct {
	Name             string
	Offerings        cloudprovider.Offerings
	Architecture     string
	OperatingSystems sets.Set[string]
	Resources        corev1.ResourceList
}

func PriceFromResources(resources corev1.ResourceList) float64 {
	price := 0.0
	for k, v := range resources {
		switch k {
		case corev1.ResourceCPU:
			price += 0.1 * v.AsApproximateFloat64()
		case corev1.ResourceMemory:
			price += 0.1 * v.AsApproximateFloat64() / (1e9)
		case ResourceGPUVendorA, ResourceGPUVendorB:
			price += 1.0
		}
	}
	return price
}
