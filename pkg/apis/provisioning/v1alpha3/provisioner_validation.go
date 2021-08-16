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

package v1alpha3

import (
	"context"
	"fmt"
	"net/url"

	"github.com/awslabs/karpenter/pkg/utils/functional"
	"github.com/awslabs/karpenter/pkg/utils/ptr"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
)

var (
	// RestrictedLabels prevent usage of specific labels. Instead, use top level provisioner fields (e.g. zone)
	RestrictedLabels = []string{
		ArchitectureLabelKey,
		OperatingSystemLabelKey,
		ProvisionerNameLabelKey,
		EmptinessTimestampAnnotationKey,
		ZoneLabelKey,
		InstanceTypeLabelKey,
	}

	// The following fields are injected by Cloud Providers
	SupportedArchitectures    = []string{}
	SupportedOperatingSystems = []string{}
	SupportedZones            = []string{}
	SupportedInstanceTypes    = []string{}
	ConstraintsValidationHook func(ctx context.Context, constraints *Constraints) *apis.FieldError
	SpecValidationHook        func(ctx context.Context, constraints *ProvisionerSpec) *apis.FieldError
)

func (p *Provisioner) Validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		apis.ValidateObjectMetadata(p).ViaField("metadata"),
		p.Spec.validate(ctx).ViaField("spec"),
	)
}

func (s *ProvisionerSpec) validate(ctx context.Context) (errs *apis.FieldError) {
	errs = errs.Also(
		s.validateTTLSecondsUntilExpired(),
		s.validateTTLSecondsAfterEmpty(),
		s.Cluster.validate().ViaField("cluster"),
		// This validation is on the ProvisionerSpec despite the fact that
		// labels are a property of Constraints. This is necessary because
		// validation is applied to constraints that include pod overrides.
		// These labels are restricted when creating provisioners, but are not
		// restricted for pods since they're necessary to override constraints.
		s.validateRestrictedLabels(),
		s.Constraints.Validate(ctx),
	)
	if SpecValidationHook != nil {
		// Call cloud provider specific spec validation logic.
		errs = errs.Also(SpecValidationHook(ctx, s))
	}
	return errs
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
		if functional.ContainsString(RestrictedLabels, key) {
			errs = errs.Also(apis.ErrInvalidKeyName(key, "labels"))
		}
	}
	return errs
}

func (c *Cluster) validate() (errs *apis.FieldError) {
	if c == nil {
		return errs.Also(apis.ErrMissingField())
	}
	if len(c.Endpoint) == 0 {
		errs = errs.Also(apis.ErrMissingField("endpoint"))
	} else {
		endpoint, err := url.Parse(c.Endpoint)
		// url.Parse() will accept a lot of input without error; make
		// sure it's a real URL
		if err != nil || !endpoint.IsAbs() || endpoint.Hostname() == "" {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s not a valid URL", c.Endpoint), "endpoint"))
		}
	}
	return errs
}

// Validate constraints subresource. This validation logic is used both upon
// creation of a provisioner as well as when a pod is attempting to be
// provisioned. If a provisioner fails validation, it will be rejected by the
// API Server. If constraints.WithOverrides(pod) fails validation, the pod will
// be ignored for provisioning.
func (c *Constraints) Validate(ctx context.Context) (errs *apis.FieldError) {
	errs = errs.Also(
		c.validateLabels(),
		c.validateTaints(),
		c.validateArchitecture(),
		c.validateOperatingSystem(),
		c.validateZones(),
		c.validateInstanceTypes(),
	)
	if ConstraintsValidationHook != nil {
		errs = errs.Also(ConstraintsValidationHook(ctx, c))
	}
	return errs
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

func (c *Constraints) validateArchitecture() (errs *apis.FieldError) {
	if c.Architecture == nil {
		return nil
	}
	if !functional.ContainsString(SupportedArchitectures, *c.Architecture) {
		errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s not in %v", *c.Architecture, SupportedArchitectures), "architecture"))
	}
	return errs
}

func (c *Constraints) validateOperatingSystem() (errs *apis.FieldError) {
	if c.OperatingSystem == nil {
		return nil
	}
	if !functional.ContainsString(SupportedOperatingSystems, *c.OperatingSystem) {
		errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s not in %v", *c.OperatingSystem, SupportedOperatingSystems), "operatingSystem"))
	}
	return errs
}

func (c *Constraints) validateZones() (errs *apis.FieldError) {
	for i, zone := range c.Zones {
		if !functional.ContainsString(SupportedZones, zone) {
			errs = errs.Also(apis.ErrInvalidArrayValue(fmt.Sprintf("%s not in %v", zone, SupportedZones), "zones", i))
		}
	}
	return errs
}

func (c *Constraints) validateInstanceTypes() (errs *apis.FieldError) {
	for i, instanceType := range c.InstanceTypes {
		if !functional.ContainsString(SupportedInstanceTypes, instanceType) {
			errs = errs.Also(apis.ErrInvalidArrayValue(fmt.Sprintf("%s not in %v", instanceType, SupportedInstanceTypes), "instanceTypes", i))
		}
	}
	return errs
}
