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

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"knative.dev/pkg/apis"
)

// Validate cloud provider specific components of the cluster spec
func (c *Capacity) Validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		validateAllowedLabels(c.provisioner.Spec),
		validateCapacityTypeLabel(c.provisioner.Spec),
		validateLaunchTemplateLabels(c.provisioner.Spec),
	)
}
func validateAllowedLabels(spec v1alpha1.ProvisionerSpec) (errs *apis.FieldError) {
	for key := range spec.Labels {
		if strings.HasPrefix(key, AWSLabelPrefix) && !functional.ContainsString(AllowedLabels, key) {
			errs = errs.Also(apis.ErrInvalidKeyName(key, "spec.labels"))
		}
	}
	return errs
}

func validateCapacityTypeLabel(spec v1alpha1.ProvisionerSpec) (errs *apis.FieldError) {
	capacityType, ok := spec.Labels[CapacityTypeLabel]
	if !ok {
		return nil
	}
	capacityTypes := []string{CapacityTypeSpot, CapacityTypeOnDemand}
	if !functional.ContainsString(capacityTypes, capacityType) {
		errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s not in %v", capacityType, capacityTypes), fmt.Sprintf("spec.labels[%s]", CapacityTypeLabel)))
	}
	return errs
}

func validateLaunchTemplateLabels(spec v1alpha1.ProvisionerSpec) (errs *apis.FieldError) {
	if _, versionExists := spec.Labels[LaunchTemplateVersionLabel]; versionExists {
		if _, bothExist := spec.Labels[LaunchTemplateIdLabel]; !bothExist {
			return errs.Also(apis.ErrMissingField(fmt.Sprintf("spec.labels[%s]", LaunchTemplateIdLabel)))
		}
	}
	return errs
}
