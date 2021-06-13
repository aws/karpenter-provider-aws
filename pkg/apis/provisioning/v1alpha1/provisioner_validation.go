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

	"github.com/awslabs/karpenter/pkg/utils/functional"
	"knative.dev/pkg/apis"
)

var (
	// RestrictedLabels prevent usage of specific labels. Instead, use top level provisioner fields (e.g. zone)
	RestrictedLabels = []string{
		ArchitectureLabelKey,
		OperatingSystemLabelKey,
		ProvisionerNameLabelKey,
		ProvisionerNamespaceLabelKey,
		ProvisionerPhaseLabel,
		ProvisionerTTLKey,
		ZoneLabelKey,
		InstanceTypeLabelKey,
	}

	// The following fields are injected by Cloud Providers
	SupportedArchitectures    = []string{}
	SupportedOperatingSystems = []string{}
	SupportedZones            = []string{}
	SupportedInstanceTypes    = []string{}
	ValidationHook            func(ctx context.Context, spec *ProvisionerSpec) *apis.FieldError
)

func (p *Provisioner) Validate(ctx context.Context) (errs *apis.FieldError) {
	return errs.Also(
		apis.ValidateObjectMetadata(p),
		p.Spec.Validate(ctx),
	)
}

func (s *ProvisionerSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	errs = errs.Also(
		s.validateClusterSpec(ctx),
		s.validateLabels(ctx),
		s.validateZones(ctx),
		s.validateInstanceTypes(ctx),
		s.validateArchitecture(ctx),
		s.validateOperatingSystem(ctx),
	)
	if ValidationHook != nil {
		errs = errs.Also(ValidationHook(ctx, s))
	}
	return errs
}

func (s *ProvisionerSpec) validateClusterSpec(ctx context.Context) (errs *apis.FieldError) {
	if s.Cluster == nil {
		return errs.Also(apis.ErrMissingField("spec.cluster"))
	}
	if len(s.Cluster.Name) == 0 {
		errs = errs.Also(apis.ErrMissingField("spec.cluster.name"))
	}
	if len(s.Cluster.Endpoint) == 0 {
		errs = errs.Also(apis.ErrMissingField("spec.cluster.endpoint"))
	}
	if len(s.Cluster.CABundle) == 0 {
		errs = errs.Also(apis.ErrMissingField("spec.cluster.caBundle"))
	}
	return errs
}

func (s *ProvisionerSpec) validateLabels(ctx context.Context) (errs *apis.FieldError) {
	for _, restricted := range RestrictedLabels {
		if _, ok := s.Labels[restricted]; ok {
			errs = errs.Also(apis.ErrInvalidKeyName(restricted, "spec.labels"))
		}
	}
	return errs
}

func (s *ProvisionerSpec) validateArchitecture(ctx context.Context) (errs *apis.FieldError) {
	if s.Architecture == nil {
		return nil
	}
	if !functional.ContainsString(SupportedArchitectures, *s.Architecture) {
		errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s not in %v", *s.Architecture, SupportedArchitectures), "spec.architecture"))
	}
	return errs
}

func (s *ProvisionerSpec) validateOperatingSystem(ctx context.Context) (errs *apis.FieldError) {
	if s.OperatingSystem == nil {
		return nil
	}
	if !functional.ContainsString(SupportedOperatingSystems, *s.OperatingSystem) {
		errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s not in %v", *s.OperatingSystem, SupportedOperatingSystems), "spec.operatingSystem"))
	}
	return errs
}

func (s *ProvisionerSpec) validateZones(ctx context.Context) (errs *apis.FieldError) {
	for i, zone := range s.Zones {
		if !functional.ContainsString(SupportedZones, zone) {
			errs = errs.Also(apis.ErrInvalidArrayValue(fmt.Sprintf("%s not in %v", zone, SupportedZones), "spec.zones", i))
		}
	}
	return errs
}

func (s *ProvisionerSpec) validateInstanceTypes(ctx context.Context) (errs *apis.FieldError) {
	for i, instanceType := range s.InstanceTypes {
		if !functional.ContainsString(SupportedInstanceTypes, instanceType) {
			errs = errs.Also(apis.ErrInvalidArrayValue(fmt.Sprintf("%s not in %v", instanceType, SupportedInstanceTypes), "spec.instanceTypes", i))
		}
	}
	return errs
}
