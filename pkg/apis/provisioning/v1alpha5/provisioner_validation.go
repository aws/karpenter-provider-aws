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

	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/ptr"
)

var (
	SupportedNodeSelectorOps sets.String = sets.NewString(string(v1.NodeSelectorOpIn), string(v1.NodeSelectorOpNotIn), string(v1.NodeSelectorOpExists), string(v1.NodeSelectorOpDoesNotExist))
	SupportedProvisionerOps  sets.String = sets.NewString(string(v1.NodeSelectorOpIn), string(v1.NodeSelectorOpNotIn), string(v1.NodeSelectorOpExists))
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
		if err := IsRestrictedLabel(key); err != nil {
			errs = errs.Also(apis.ErrInvalidKeyName(key, "labels", fmt.Sprintf("%s", err)))
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

// This function is used by the provisioner validation webhook to verify the provisioner requirements.
// When this function is called, the provisioner's requirments do not include the requirements from labels.
// Provisioner requirements only support well known labels.
func (c *Constraints) validateRequirements() (errs *apis.FieldError) {
	var err error
	for _, requirement := range c.Requirements.Requirements {
		// Ensure requirements operator is allowed
		if !SupportedProvisionerOps.Has(string(requirement.Operator)) {
			err = multierr.Append(err, fmt.Errorf("key %s has an unsupported operator %s, provisioner only supports %s", requirement.Key, requirement.Operator, SupportedProvisionerOps.UnsortedList()))
		}
		if e := IsRestrictedLabel(requirement.Key); e != nil {
			err = multierr.Append(err, e)
		}
	}
	err = multierr.Append(err, c.Requirements.Validate())
	if err != nil {
		errs = errs.Also(apis.ErrInvalidValue(err, "requirements"))
	}
	return errs
}
