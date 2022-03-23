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
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

func Requirements(instanceTypes []InstanceType) v1alpha5.Requirements {
	supported := map[string]sets.String{
		v1.LabelInstanceTypeStable: sets.NewString(),
		v1.LabelTopologyZone:       sets.NewString(),
		v1.LabelArchStable:         sets.NewString(),
		v1.LabelOSStable:           sets.NewString(),
		v1alpha5.LabelCapacityType: sets.NewString(),
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
	requirements := v1alpha5.NewRequirements()
	for key, values := range supported {
		requirements = requirements.Add(v1.NodeSelectorRequirement{Key: key, Operator: v1.NodeSelectorOpIn, Values: values.UnsortedList()})
	}
	return requirements
}

func Compatible(it InstanceType, requirements v1alpha5.Requirements) bool {
	if !requirements.Get(v1.LabelInstanceTypeStable).Has(it.Name()) {
		return false
	}
	if !requirements.Get(v1.LabelArchStable).Has(it.Architecture()) {
		return false
	}
	if !requirements.Get(v1.LabelOSStable).HasAny(it.OperatingSystems().List()...) {
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
