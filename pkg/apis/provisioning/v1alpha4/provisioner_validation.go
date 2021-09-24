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

package v1alpha4

import (
	"context"
	"fmt"
	"strings"

	"github.com/awslabs/karpenter/pkg/utils/functional"
	"github.com/awslabs/karpenter/pkg/utils/ptr"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
)

func (p *Provisioner) Validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		apis.ValidateObjectMetadata(p).ViaField("metadata"),
		p.Spec.validate(ctx).ViaField("spec"),
	)
}

func (s *ProvisionerSpec) validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		s.validateTTLSecondsUntilExpired(),
		s.validateTTLSecondsAfterEmpty(),
		// This validation is on the ProvisionerSpec despite the fact that
		// labels are a property of Constraints. This is necessary because
		// validation is applied to constraints that include pod overrides.
		// These labels are restricted when creating provisioners, but are not
		// restricted for pods since they're necessary to override constraints.
		s.validateRestrictedLabels(),
		s.Constraints.Validate(ctx),
	)
}

func (s *ProvisionerSpec) validateTTLSecondsUntilExpired() (errs *apis.FieldError) {
	if ptr.Int64Value(s.TTLSecondsUntilExpired) < 0 {
		return errs.Also(apis.ErrInvalidValue("cannot be negative", "ttlSecondsUntilExpired"))
	}
	return errs
}
func (s *ProvisionerSpec) validateTTLSecondsAfterEmpty() (errs *apis.FieldError) {
	if ptr.Int64Value(s.TTLSecondsAfterEmpty) < 0 {
		return errs.Also(apis.ErrInvalidValue("cannot be negative", "ttlSecondsAfterEmpty"))
	}
	return errs
}

func (s *ProvisionerSpec) validateRestrictedLabels() (errs *apis.FieldError) {
	for key := range s.Labels {
		for _, restricted := range RestrictedLabels {
			if strings.HasPrefix(key, restricted) {
				errs = errs.Also(apis.ErrInvalidKeyName(key, "labels"))
			}
		}
	}
	return errs
}

// Validate constraints subresource. This validation logic is used both upon
// creation of a provisioner as well as when a pod is attempting to be
// provisioned. If a provisioner fails validation, it will be rejected by the
// API Server. If validation fails at provisioning time, the pod is ignored
func (c *Constraints) Validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		c.validateLabels(),
		c.validateTaints(),
		ValidateWellKnown(v1.LabelTopologyZone, c.Zones, "zones"),
		ValidateWellKnown(v1.LabelInstanceTypeStable, c.InstanceTypes, "instanceTypes"),
		ValidateWellKnown(v1.LabelArchStable, c.Architectures, "architectures"),
		ValidateWellKnown(v1.LabelOSStable, c.OperatingSystems, "operatingSystems"),
		ValidateHook(ctx, c),
	)
}

func (c *Constraints) validateLabels() (errs *apis.FieldError) {
	for key, value := range c.Labels {
		for _, err := range validation.IsQualifiedName(key) {
			errs = errs.Also(apis.ErrInvalidKeyName(key, "labels", err))
		}
		for _, err := range validation.IsValidLabelValue(value) {
			errs = errs.Also(apis.ErrInvalidValue(value+", "+err, "labels"))
		}
	}
	return errs
}

func (c *Constraints) validateTaints() (errs *apis.FieldError) {
	for i, taint := range c.Taints {
		// Validate Key
		if len(taint.Key) == 0 {
			errs = errs.Also(apis.ErrInvalidArrayValue(errs, "taints", i))
		}
		for _, err := range validation.IsQualifiedName(taint.Key) {
			errs = errs.Also(apis.ErrInvalidArrayValue(err, "taints", i))
		}
		// Validate Value
		if len(taint.Value) != 0 {
			for _, err := range validation.IsQualifiedName(taint.Value) {
				errs = errs.Also(apis.ErrInvalidArrayValue(err, "taints", i))
			}
		}
		// Validate effect
		switch taint.Effect {
		case v1.TaintEffectNoSchedule, v1.TaintEffectPreferNoSchedule, v1.TaintEffectNoExecute, "":
		default:
			errs = errs.Also(apis.ErrInvalidArrayValue(taint.Effect, "effect", i))
		}
	}
	return errs
}

func ValidateWellKnown(key string, values []string, fieldName string) (errs *apis.FieldError) {
	if values != nil && len(values) == 0 {
		errs = errs.Also(apis.ErrMissingField(fieldName))
	}
	for i, value := range values {
		if known := WellKnownLabels[key]; !functional.ContainsString(known, value) {
			errs = errs.Also(apis.ErrInvalidArrayValue(fmt.Sprintf("%s not in %v", value, known), fieldName, i))
		}
	}
	return errs
}
