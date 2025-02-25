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

package test

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
)

func RemoveNodeClassTagValidation(crds []*apiextensionsv1.CustomResourceDefinition) []*apiextensionsv1.CustomResourceDefinition {
	for _, crd := range apis.CRDs {
		if crd.Name != "ec2nodeclasses.karpenter.k8s.aws" {
			continue
		}
		overrideProperties := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["tags"]
		overrideProperties.XValidations = nil
		crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["tags"] = overrideProperties
	}
	return crds
}

// DisableCapacityReservationIDValidation updates the regex validation used for capacity reservation IDs to allow any
// string after the "cr-" prefix. This enables us to embed useful debugging information in the reservation ID, such as
// the instance type and zone.
func DisableCapacityReservationIDValidation(crds []*apiextensionsv1.CustomResourceDefinition) []*apiextensionsv1.CustomResourceDefinition {
	for _, crd := range crds {
		if crd.Name != "ec2nodeclasses.karpenter.k8s.aws" {
			continue
		}
		// Disable validation for the selector terms
		idProps := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["capacityReservationSelectorTerms"].Items.Schema.Properties["id"]
		idProps.Pattern = ""
		crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["capacityReservationSelectorTerms"].Items.Schema.Properties["id"] = idProps

		// Disable validation for the status
		idProps = crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["status"].Properties["capacityReservations"].Items.Schema.Properties["id"]
		idProps.Pattern = ""
		crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["status"].Properties["capacityReservations"].Items.Schema.Properties["id"] = idProps
	}
	return crds
}
