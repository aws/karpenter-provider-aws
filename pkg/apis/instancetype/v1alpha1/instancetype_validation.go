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

package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/validation"
	"knative.dev/pkg/apis"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/resources"
)

func (in *InstanceType) Validate(_ context.Context) (errs *apis.FieldError) {
	return errs.Also(
		in.validateMetadata().ViaField("metadata"),
		in.Spec.validate().ViaField("spec"),
	)
}

func (in *InstanceType) validateMetadata() *apis.FieldError {
	msgs := validation.NameIsDNSSubdomain(in.Name, false)
	if len(msgs) > 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf("not a DNS subdomain name: %v", msgs),
			Paths:   []string{"name"},
		}
	}
	return nil
}

func (in *InstanceTypeSpec) validate() (errs *apis.FieldError) {
	return errs.Also(
		in.validateResources(),
	)
}

// validateResources should reject the InstanceType if any of the key values
// are part of the well-known requirements set
func (in *InstanceTypeSpec) validateResources() (errs *apis.FieldError) {
	fmt.Println(v1alpha5.WellKnownLabels)
	for k := range in.Resources {
		if v1alpha5.WellKnownLabels.Has(k.String()) {
			errs = errs.Also(apis.ErrInvalidValue("cannot be from the set of well-known requirements", fmt.Sprintf("resources[%s]", k)))
		} else if resources.WellKnownResourceNames.Has(k.String()) {
			errs = errs.Also(apis.ErrInvalidValue("cannot be from the set of well-known resource names", fmt.Sprintf("resources[%s]", k)))
		} else if strings.HasPrefix(k.String(), v1.ResourceHugePagesPrefix) || strings.HasPrefix(k.String(), v1.ResourceAttachableVolumesPrefix) {
			errs = errs.Also(apis.ErrInvalidValue("cannot use a resource name from a set of well-known prefixes", fmt.Sprintf("resources[%s]", k)))
		}
	}
	return errs
}
