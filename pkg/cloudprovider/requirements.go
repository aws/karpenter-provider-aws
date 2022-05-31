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

package cloudprovider

import (
	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/scheduling"
	"github.com/aws/karpenter/pkg/utils/resources"
	"github.com/aws/karpenter/pkg/utils/sets"
)

func InstanceTypeRequirements(instanceTypes []InstanceType) scheduling.Requirements {
	supported := map[string]sets.Set{
		v1.LabelInstanceTypeStable: sets.NewSet(),
		v1.LabelTopologyZone:       sets.NewSet(),
		v1.LabelArchStable:         sets.NewSet(),
		v1.LabelOSStable:           sets.NewSet(),
		v1alpha5.LabelCapacityType: sets.NewSet(),
	}
	for _, instanceType := range instanceTypes {
		for _, offering := range instanceType.Offerings() {
			supported[v1.LabelTopologyZone].Insert(offering.Zone)
			supported[v1alpha5.LabelCapacityType].Insert(offering.CapacityType)
		}
		supported[v1.LabelInstanceTypeStable].Insert(instanceType.Name())
		supported[v1.LabelArchStable].Insert(instanceType.Architecture())
		supported[v1.LabelOSStable].Insert(instanceType.OperatingSystems().List()...)
	}
	requirements := scheduling.NewRequirements(supported)
	return requirements
}

func Compatible(it InstanceType, requirements scheduling.Requirements) bool {
	if !requirements.Get(v1.LabelInstanceTypeStable).Has(it.Name()) {
		return false
	}
	if !requirements.Get(v1.LabelArchStable).Has(it.Architecture()) {
		return false
	}
	if !requirements.Get(v1.LabelOSStable).HasAny(it.OperatingSystems()) {
		return false
	}
	// acceptable if we have any offering that is valid
	for _, offering := range it.Offerings() {
		if requirements.Get(v1.LabelTopologyZone).Has(offering.Zone) && requirements.Get(v1alpha5.LabelCapacityType).Has(offering.CapacityType) {
			return true
		}
	}
	return false
}

func FilterInstanceTypes(instanceTypes []InstanceType, requirements scheduling.Requirements, requests v1.ResourceList) []InstanceType {
	var result []InstanceType
	for _, instanceType := range instanceTypes {
		if !Compatible(instanceType, requirements) {
			continue
		}
		if !resources.Fits(resources.Merge(requests, instanceType.Overhead()), instanceType.Resources()) {
			continue
		}
		result = append(result, instanceType)
	}
	return result
}
