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
	"fmt"
	"math"

	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/scheduling"

	"github.com/samber/lo"

	"github.com/aws/karpenter/pkg/cloudprovider"
)

// AvailableOfferings filters the offerings on the passed instance type
// and returns the offerings marked as available
func AvailableOfferings(it cloudprovider.InstanceType) []cloudprovider.Offering {
	return lo.Filter(it.Offerings(), func(o cloudprovider.Offering, _ int) bool {
		return o.Available
	})
}

// GetOffering gets the offering from passed offerings that matches the
// passed zone and capacity type
func GetOffering(ofs []cloudprovider.Offering, ct, zone string) (cloudprovider.Offering, error) {
	for _, of := range ofs {
		if of.CapacityType == ct && of.Zone == zone {
			return of, nil
		}
	}
	return cloudprovider.Offering{}, fmt.Errorf("cloudprovider not found")
}

// HasZone checks whether any of the passed offerings have a zone matching
// the passed zone
func HasZone(ofs []cloudprovider.Offering, zone string) bool {
	for _, elem := range ofs {
		if elem.Zone == zone {
			return true
		}
	}
	return false
}

// CheapestOffering grabs the cheapest offering from the passed offerings
func CheapestOffering(ofs []cloudprovider.Offering) cloudprovider.Offering {
	return CheapestOfferingWithReqs(ofs, scheduling.NewRequirements())
}

// CheapestOfferingWithReqs grabs the cheapest offering from the passed offerings
// based on the requirements placed on the node
func CheapestOfferingWithReqs(ofs []cloudprovider.Offering, requirements scheduling.Requirements) cloudprovider.Offering {
	offering := cloudprovider.Offering{Price: math.MaxFloat64}
	for _, of := range ofs {
		if requirements.Get(v1alpha5.LabelCapacityType).Has(of.CapacityType) && requirements.Get(v1.LabelTopologyZone).Has(of.Zone) {
			if of.Price < offering.Price {
				offering = of
			}
		}
	}
	return offering
}
