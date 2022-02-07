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
		ValidateHook(ctx, p),
	)
}

func (s *ProvisionerSpec) validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		s.validateTTLSecondsUntilExpired(),
		s.validateTTLSecondsAfterEmpty(),
		s.MaintenanceWindows.Validate(),
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
func (s *ProvisionerSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		s.validateLabels(),
		s.validateTaints(),
		s.validateRequirements(),
	)
}

func (s *ProvisionerSpec) validateLabels() (errs *apis.FieldError) {
	for key, value := range s.Labels {
		for _, err := range validation.IsQualifiedName(key) {
			errs = errs.Also(apis.ErrInvalidKeyName(key, "labels", err))
		}
		for _, err := range validation.IsValidLabelValue(value) {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s, %s", value, err), fmt.Sprintf("labels[%s]", key)))
		}
		if err := IsRestrictedLabel(key); err != nil {
			errs = errs.Also(apis.ErrInvalidKeyName(key, "labels", err.Error()))
		}
	}
	return errs
}

type taintKeyEffect struct {
	Key    string
	Effect v1.TaintEffect
}

func (s *ProvisionerSpec) validateTaints() (errs *apis.FieldError) {
	existing := map[taintKeyEffect]struct{}{}
	errs = errs.Also(s.validateTaintsField(s.Taints, existing, "taints"))
	errs = errs.Also(s.validateTaintsField(s.StartupTaints, existing, "startupTaints"))
	return errs
}

func (s *ProvisionerSpec) validateTaintsField(taints []v1.Taint, existing map[taintKeyEffect]struct{}, fieldName string) *apis.FieldError {
	var errs *apis.FieldError
	for i, taint := range taints {
		// Validate Key
		if len(taint.Key) == 0 {
			errs = errs.Also(apis.ErrInvalidArrayValue(errs, fieldName, i))
		}
		for _, err := range validation.IsQualifiedName(taint.Key) {
			errs = errs.Also(apis.ErrInvalidArrayValue(err, fieldName, i))
		}
		// Validate Value
		if len(taint.Value) != 0 {
			for _, err := range validation.IsQualifiedName(taint.Value) {
				errs = errs.Also(apis.ErrInvalidArrayValue(err, fieldName, i))
			}
		}
		// Validate effect
		switch taint.Effect {
		case v1.TaintEffectNoSchedule, v1.TaintEffectPreferNoSchedule, v1.TaintEffectNoExecute, "":
		default:
			errs = errs.Also(apis.ErrInvalidArrayValue(taint.Effect, "effect", i))
		}

		// Check for duplicate Key/Effect pairs
		key := taintKeyEffect{Key: taint.Key, Effect: taint.Effect}
		if _, ok := existing[key]; ok {
			errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("duplicate taint Key/Effect pair %s=%s", taint.Key, taint.Effect), apis.CurrentField).
				ViaFieldIndex("taints", i))
		}
		existing[key] = struct{}{}
	}
	return errs
}

// This function is used by the provisioner validation webhook to verify the provisioner requirements.
// When this function is called, the provisioner's requirments do not include the requirements from labels.
// Provisioner requirements only support well known labels.
func (s *ProvisionerSpec) validateRequirements() *apis.FieldError {
	var errs error
	for _, requirement := range s.Requirements {
		// Ensure requirements operator is allowed
		if !SupportedProvisionerOps.Has(string(requirement.Operator)) {
			errs = multierr.Append(errs, fmt.Errorf("key %s has an unsupported operator %s, provisioner only supports %s", requirement.Key, requirement.Operator, SupportedProvisionerOps.UnsortedList()))
		}
		if e := IsRestrictedLabel(requirement.Key); e != nil {
			errs = multierr.Append(errs, e)
		}
		// We don't support a 'NotExists' operator, but this turns into an empty set of values by re-building node selector requirements
		if requirement.Operator == v1.NodeSelectorOpIn && len(requirement.Values) == 0 {
			errs = multierr.Append(errs, fmt.Errorf("key %s is unsatisfiable due to unsupported operator or no values being provided", requirement.Key))
		}
		for _, err := range validation.IsQualifiedName(requirement.Key) {
			errs = multierr.Append(errs, fmt.Errorf("key %s is not a qualified name, %s", requirement.Key, err))
		}
		for _, value := range requirement.Values {
			for _, err := range validation.IsValidLabelValue(value) {
				errs = multierr.Append(errs, fmt.Errorf("invalid value %s for key %s, %s", value, requirement.Key, err))
			}
		}
	}
	var result *apis.FieldError
	if errs != nil {
		result = result.Also(apis.ErrInvalidValue(errs, "requirements"))
	}
	return result
}

func (m *MaintenanceWindows) Validate() (errs *apis.FieldError) {
	for i, maintenanceWindow := range *m {
		if err := maintenanceWindow.Validate(); err != nil {
			errs = errs.Also(apis.ErrInvalidArrayValue(err, "maintenanceWindow", i))
		}
	}
	return errs
}
