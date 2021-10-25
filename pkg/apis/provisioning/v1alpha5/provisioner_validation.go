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

package v1alpha5

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"

	"github.com/awslabs/karpenter/pkg/utils/functional"
	"github.com/awslabs/karpenter/pkg/utils/ptr"
)

var (
	SupportedNodeSelectorOps = []string{string(v1.NodeSelectorOpIn), string(v1.NodeSelectorOpNotIn)}
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

// Validate the constraints
func (c *Constraints) Validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		c.validateLabels(),
		c.validateTaints(),
		c.validateRequirements(),
		ValidateHook(ctx, c),
	)
}

func (c *Constraints) validateLabels() (errs *apis.FieldError) {
	for key, value := range c.Labels {
		for _, err := range validation.IsQualifiedName(key) {
			errs = errs.Also(apis.ErrInvalidKeyName(key, "labels", err))
		}
		for _, err := range validation.IsValidLabelValue(value) {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s, %s", value, err), fmt.Sprintf("labels[%s]", key)))
		}
		if _, ok := WellKnownLabels[key]; !ok && IsRestrictedLabelDomain(key) {
			errs = errs.Also(apis.ErrInvalidKeyName(key, "labels", "label domain not allowed"))
		}
	}
	return errs
}

func IsRestrictedLabelDomain(key string) bool {
	labelDomain := getLabelDomain(key)
	for _, restrictedLabelDomain := range RestrictedLabelDomains {
		if strings.HasSuffix(labelDomain, restrictedLabelDomain) {
			return true
		}
	}
	return false
}

func getLabelDomain(key string) string {
	if parts := strings.SplitN(key, "/", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
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

func (c *Constraints) validateRequirements() (errs *apis.FieldError) {
	for i, requirement := range c.Requirements {
		if err := validateRequirement(requirement); err != nil {
			errs = errs.Also(apis.ErrInvalidArrayValue(err, "requirements", i))
		}
	}
	return errs
}

func (r Requirements) Validate() (errs *apis.FieldError) {
	for _, label := range r.GetLabels() {
		if len(r.GetLabelValues(label)) == 0 {
			errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("%s is too constrained", label)))
		}
	}
	return errs
}

func validateRequirement(requirement v1.NodeSelectorRequirement) (errs *apis.FieldError) {
	for _, err := range validation.IsQualifiedName(requirement.Key) {
		errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s, %s", requirement.Key, err), "key"))
	}
	for i, value := range requirement.Values {
		for _, err := range validation.IsValidLabelValue(value) {
			errs = errs.Also(apis.ErrInvalidArrayValue(fmt.Sprintf("%s, %s", value, err), "values", i))
		}
	}
	if !functional.ContainsString(SupportedNodeSelectorOps, string(requirement.Operator)) {
		errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s not in %s", requirement.Operator, SupportedNodeSelectorOps), "operator"))
	}
	return errs
}
