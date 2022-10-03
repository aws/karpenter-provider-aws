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
	"strconv"
	"strings"

	"github.com/samber/lo"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/ptr"
)

var (
	SupportedNodeSelectorOps = sets.NewString(
		string(v1.NodeSelectorOpIn),
		string(v1.NodeSelectorOpNotIn),
		string(v1.NodeSelectorOpGt),
		string(v1.NodeSelectorOpLt),
		string(v1.NodeSelectorOpExists),
		string(v1.NodeSelectorOpDoesNotExist),
	)

	SupportedReservedResources = sets.NewString(
		v1.ResourceCPU.String(),
		v1.ResourceMemory.String(),
		v1.ResourceEphemeralStorage.String(),
		"pid",
	)

	SupportedEvictionSignals = sets.NewString(
		"memory.available",
		"nodefs.available",
		"nodefs.inodesFree",
		"imagefs.available",
		"imagefs.inodesFree",
		"pid.available",
	)
)

const (
	providerPath    = "provider"
	providerRefPath = "providerRef"
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
	// TTLSecondsAfterEmpty and consolidation are mutually exclusive
	if s.Consolidation != nil && ptr.BoolValue(s.Consolidation.Enabled) && s.TTLSecondsAfterEmpty != nil {
		return errs.Also(apis.ErrMultipleOneOf("ttlSecondsAfterEmpty", "consolidation.enabled"))
	}
	return errs
}

// Validate the constraints
func (s *ProvisionerSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		s.validateProvider(),
		s.validateLabels(),
		s.validateTaints(),
		s.validateRequirements(),
		s.validateKubeletConfiguration().ViaField("kubeletConfiguration"),
	)
}

func (s *ProvisionerSpec) validateLabels() (errs *apis.FieldError) {
	for key, value := range s.Labels {
		if key == ProvisionerNameLabelKey {
			errs = errs.Also(apis.ErrInvalidKeyName(key, "labels", "restricted"))
		}
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
func (s *ProvisionerSpec) validateRequirements() (errs *apis.FieldError) {
	for i, requirement := range s.Requirements {
		if requirement.Key == ProvisionerNameLabelKey {
			errs = errs.Also(apis.ErrInvalidArrayValue(fmt.Sprintf("%s is restricted", requirement.Key), "requirements", i))
		}
		if err := ValidateRequirement(requirement); err != nil {
			errs = errs.Also(apis.ErrInvalidArrayValue(err, "requirements", i))
		}
	}
	return errs
}

func (s *ProvisionerSpec) validateProvider() *apis.FieldError {
	if s.Provider != nil && s.ProviderRef != nil {
		return apis.ErrMultipleOneOf(providerPath, providerRefPath)
	}
	return nil
}

func (s *ProvisionerSpec) validateKubeletConfiguration() (errs *apis.FieldError) {
	if s.KubeletConfiguration == nil {
		return
	}
	return errs.Also(
		validateEvictionThresholds(s.KubeletConfiguration.EvictionHard, "evictionHard"),
		validateEvictionThresholds(s.KubeletConfiguration.EvictionSoft, "evictionSoft"),
		validateReservedResources(s.KubeletConfiguration.KubeReserved, "kubeReserved"),
		validateReservedResources(s.KubeletConfiguration.SystemReserved, "systemReserved"),
		s.KubeletConfiguration.validateEvictionSoftGracePeriod(),
		s.KubeletConfiguration.validateEvictionSoftPairs(),
	)
}

func (kc *KubeletConfiguration) validateEvictionSoftGracePeriod() (errs *apis.FieldError) {
	for k := range kc.EvictionSoftGracePeriod {
		if !SupportedEvictionSignals.Has(k) {
			errs = errs.Also(apis.ErrInvalidKeyName(k, "evictionSoftGracePeriod"))
		}
	}
	return errs
}

func (kc *KubeletConfiguration) validateEvictionSoftPairs() (errs *apis.FieldError) {
	evictionSoftKeys := sets.NewString(lo.Keys(kc.EvictionSoft)...)
	evictionSoftGracePeriodKeys := sets.NewString(lo.Keys(kc.EvictionSoftGracePeriod)...)

	evictionSoftDiff := evictionSoftKeys.Difference(evictionSoftGracePeriodKeys)
	for k := range evictionSoftDiff {
		errs = errs.Also(apis.ErrInvalidKeyName(k, "evictionSoft", "Key does not have a matching evictionSoftGracePeriod"))
	}
	evictionSoftGracePeriodDiff := evictionSoftGracePeriodKeys.Difference(evictionSoftKeys)
	for k := range evictionSoftGracePeriodDiff {
		errs = errs.Also(apis.ErrInvalidKeyName(k, "evictionSoftGracePeriod", "Key does not have a matching evictionSoft threshold value"))
	}
	return errs
}

func validateReservedResources(m v1.ResourceList, fieldName string) (errs *apis.FieldError) {
	for k, v := range m {
		if !SupportedReservedResources.Has(k.String()) {
			errs = errs.Also(apis.ErrInvalidKeyName(k.String(), fieldName))
		}
		if v.Value() < 0 {
			errs = errs.Also(apis.ErrInvalidValue(v.String(), fmt.Sprintf(`%s["%s"]`, fieldName, k), "Value cannot be a negative resource quantity"))
		}
	}
	return errs
}

func validateEvictionThresholds(m map[string]string, fieldName string) (errs *apis.FieldError) {
	if m == nil {
		return
	}
	for k, v := range m {
		if !SupportedEvictionSignals.Has(k) {
			errs = errs.Also(apis.ErrInvalidKeyName(k, fieldName))
		}
		if strings.HasSuffix(v, "%") {
			p, err := strconv.ParseFloat(strings.Trim(v, "%"), 64)
			if err != nil {
				errs = errs.Also(apis.ErrInvalidValue(v, fmt.Sprintf(`%s["%s"]`, fieldName, k), fmt.Sprintf("Value could not be parsed as a percentage value, %v", err.Error())))
			}
			if p < 0 {
				errs = errs.Also(apis.ErrInvalidValue(v, fmt.Sprintf(`%s["%s"]`, fieldName, k), "Percentage values cannot be negative"))
			}
			if p > 100 {
				errs = errs.Also(apis.ErrInvalidValue(v, fmt.Sprintf(`%s["%s"]`, fieldName, k), "Percentage values cannot be greater than 100"))
			}
		} else {
			_, err := resource.ParseQuantity(v)
			if err != nil {
				errs = errs.Also(apis.ErrInvalidValue(v, fmt.Sprintf("%s[%s]", fieldName, k), fmt.Sprintf("Value could not be parsed as a resource quantity, %v", err.Error())))
			}
		}
	}
	return errs
}

func ValidateRequirement(requirement v1.NodeSelectorRequirement) error { //nolint:gocyclo
	var errs error
	if normalized, ok := NormalizedLabels[requirement.Key]; ok {
		requirement.Key = normalized
	}
	if !SupportedNodeSelectorOps.Has(string(requirement.Operator)) {
		errs = multierr.Append(errs, fmt.Errorf("key %s has an unsupported operator %s not in %s", requirement.Key, requirement.Operator, SupportedNodeSelectorOps.UnsortedList()))
	}
	if e := IsRestrictedLabel(requirement.Key); e != nil {
		errs = multierr.Append(errs, e)
	}
	for _, err := range validation.IsQualifiedName(requirement.Key) {
		errs = multierr.Append(errs, fmt.Errorf("key %s is not a qualified name, %s", requirement.Key, err))
	}
	for _, value := range requirement.Values {
		for _, err := range validation.IsValidLabelValue(value) {
			errs = multierr.Append(errs, fmt.Errorf("invalid value %s for key %s, %s", value, requirement.Key, err))
		}
	}
	if requirement.Operator == v1.NodeSelectorOpIn && len(requirement.Values) == 0 {
		errs = multierr.Append(errs, fmt.Errorf("key %s with operator %s must have a value defined", requirement.Key, requirement.Operator))
	}
	if requirement.Operator == v1.NodeSelectorOpGt || requirement.Operator == v1.NodeSelectorOpLt {
		if len(requirement.Values) != 1 {
			errs = multierr.Append(errs, fmt.Errorf("key %s with operator %s must have a single positive integer value", requirement.Key, requirement.Operator))
		} else {
			value, err := strconv.Atoi(requirement.Values[0])
			if err != nil || value < 0 {
				errs = multierr.Append(errs, fmt.Errorf("key %s with operator %s must have a single positive integer value", requirement.Key, requirement.Operator))
			}
		}
	}
	return errs
}
