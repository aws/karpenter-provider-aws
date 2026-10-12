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

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
)

func validateCapacityReservationGroupRequirements(requirements []karpv1.NodeSelectorRequirementWithMinValues, labels map[string]string) error {
	reqs := scheduling.NewNodeSelectorRequirementsWithMinValues(requirements...)
	reqs.Add(scheduling.NewLabelRequirements(labels).Values()...)
	capacityType := reqs.Get(karpv1.CapacityTypeLabelKey)
	if !capacityType.Has(karpv1.CapacityTypeOnDemand) || !capacityType.Has(karpv1.CapacityTypeReserved) {
		return fmt.Errorf("capacityReservationGroupARN requires both on-demand and reserved capacity types to be allowed")
	}
	for label := range cloudprovider.ReservedCapacityLabels {
		if reqs.Has(label) {
			return fmt.Errorf("capacityReservationGroupARN is incompatible with requirements on %s", label)
		}
	}
	return nil
}
