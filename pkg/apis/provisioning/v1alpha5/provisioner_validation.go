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

	"github.com/aws/karpenter/pkg/utils/ptr"
)

var (
	SupportedNodeSelectorOps = []string{string(v1.NodeSelectorOpIn), string(v1.NodeSelectorOpNotIn), string(v1.NodeSelectorOpExists), string(v1.NodeSelectorOpDoesNotExist)}
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
		s.Validate(ctx),
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
		if RestrictedLabels.Has(key) {
			errs = errs.Also(apis.ErrInvalidKeyName(key, "labels", "label is restricted"))
		}
		if _, ok := WellKnownLabels[key]; !ok && IsRestrictedLabelDomain(key) {
			errs = errs.Also(apis.ErrInvalidKeyName(key, "labels", "label domain not allowed"))
		}
	}
	return errs
}

func IsRestrictedLabelDomain(key string) bool {
	labelDomain := getLabelDomain(key)
	if AllowedLabelDomains.Has(labelDomain) {
		return false
	}
	for restrictedLabelDomain := range RestrictedLabelDomains {
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

// This function is used by the provisioner validation webhook to verify the provisioner requirements.
// When this function is called, the provisioner's requirments do not include the requirements from labels.
// Provisioner requirements only support well known labels.
func (c *Constraints) validateRequirements() (errs *apis.FieldError) {
	// validate if requirement keys are well known labels
	for _, key := range c.Requirements.Keys() {
		if !WellKnownLabels.Has(key) {
			errs = errs.Also(apis.ErrInvalidKeyName(fmt.Sprintf("%s not in %v", key, WellKnownLabels.UnsortedList()), "key"))
		}
	}
	if err := c.Requirements.Validate(); err != nil {
		errs = errs.Also(err)
	}
	return errs
}
