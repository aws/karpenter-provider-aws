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
	"regexp"

	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/ptr"
)

var (
	SupportedNodeSelectorOps = sets.NewString(string(v1.NodeSelectorOpIn), string(v1.NodeSelectorOpNotIn), string(v1.NodeSelectorOpExists), string(v1.NodeSelectorOpDoesNotExist))
	SupportedProvisionerOps  = sets.NewString(string(v1.NodeSelectorOpIn), string(v1.NodeSelectorOpNotIn), string(v1.NodeSelectorOpExists))
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
		s.validateInstanceTypeFilter().ViaField("instanceTypeFilter"),
		s.Validate(ctx),
	)
}
func (s *ProvisionerSpec) validateInstanceTypeFilter() *apis.FieldError {
	if s.InstanceTypeFilter == nil {
		return nil
	}
	for key, minQuantity := range s.InstanceTypeFilter.MinResources {
		if minQuantity.AsApproximateFloat64() < 0 {
			return apis.ErrInvalidValue(minQuantity.String(), apis.CurrentField, "cannot be negative").ViaKey(string(key)).ViaField("minResources")
		}
		if maxQuantity, ok := s.InstanceTypeFilter.MaxResources[key]; ok {
			if minQuantity.Cmp(maxQuantity) > 0 {
				return apis.ErrGeneric("min must be <= max", apis.CurrentField).ViaKey(string(key)).ViaField("maxResources")
			}
		}
	}
	for key, quantity := range s.InstanceTypeFilter.MaxResources {
		if quantity.AsApproximateFloat64() < 0 {
			return apis.ErrInvalidValue(quantity.String(), apis.CurrentField, "cannot be negative").ViaKey(string(key)).ViaField("maxResources")
		}
	}
	if err := s.validateMinMax(s.InstanceTypeFilter.MemoryPerCPU); err != nil {
		return err.ViaField("memoryPerCPU")
	}
	if err := s.validateRegularExpressions("nameIncludeExpressions", s.InstanceTypeFilter.NameIncludeExpressions); err != nil {
		return err
	}
	if err := s.validateRegularExpressions("nameExcludeExpressions", s.InstanceTypeFilter.NameExcludeExpressions); err != nil {
		return err
	}
	return nil
}

func (s *ProvisionerSpec) validateRegularExpressions(fieldName string, expressions []string) *apis.FieldError {
	for i, expr := range expressions {
		_, err := regexp.Compile(expr)
		if err != nil {
			// we need the field name here to get the error message to construct correctly
			// e.g. filter.nameIncludeExpressions[0] vs filter[0].nameIncludeExpressions
			return apis.ErrInvalidValue(expr, fieldName, err.Error()).ViaIndex(i)
		}
	}
	return nil
}

func (s *ProvisionerSpec) validateMinMax(minMax *MinMax) *apis.FieldError {
	if minMax == nil {
		return nil
	}
	if minMax.Min != nil && minMax.Min.AsApproximateFloat64() < 0 {
		return apis.ErrInvalidValue(minMax.Min.String(), "min", "cannot be negative")
	}
	if minMax.Max != nil && minMax.Max.AsApproximateFloat64() < 0 {
		return apis.ErrInvalidValue(minMax.Max.String(), "max", "cannot be negative")
	}
	if minMax.Min != nil && minMax.Max != nil && minMax.Min.AsApproximateFloat64() > minMax.Max.AsApproximateFloat64() {
		return apis.ErrGeneric("min must be <= max", "min", "max")
	}
	return nil
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

func (s *ProvisionerSpec) validateTaintsField(taints Taints, existing map[taintKeyEffect]struct{}, fieldName string) *apis.FieldError {
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
func (s *ProvisionerSpec) validateRequirements() (errs *apis.FieldError) {
	var err error
	for _, requirement := range s.Requirements.Requirements {
		// Ensure requirements operator is allowed
		if !SupportedProvisionerOps.Has(string(requirement.Operator)) {
			err = multierr.Append(err, fmt.Errorf("key %s has an unsupported operator %s, provisioner only supports %s", requirement.Key, requirement.Operator, SupportedProvisionerOps.UnsortedList()))
		}
		if e := IsRestrictedLabel(requirement.Key); e != nil {
			err = multierr.Append(err, e)
		}
		// We don't support a 'NotExists' operator, but this turns into an empty set of values by re-building node selector requirements
		if requirement.Operator == v1.NodeSelectorOpIn && len(requirement.Values) == 0 {
			err = multierr.Append(err, fmt.Errorf("key %s is unsatisfiable due to unsupported operator or no values being provided", requirement.Key))
		}
	}
	err = multierr.Append(err, s.Requirements.Validate())
	if err != nil {
		errs = errs.Also(apis.ErrInvalidValue(err, "requirements"))
	}
	return errs
}
