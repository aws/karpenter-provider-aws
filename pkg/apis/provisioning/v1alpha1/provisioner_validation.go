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

// +kubebuilder:webhook:path=/validate-provisioning-karpenter-sh-v1alpha1-provisioner,mutating=false,sideEffects=None,failurePolicy=fail,groups=provisioning.karpenter.sh,resources=provisioners,verbs=create;update,versions=v1alpha1,name=validation.provisioning.karpenter.sh

package v1alpha1

import (
	"fmt"

	"github.com/awslabs/karpenter/pkg/utils/functional"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var _ webhook.Validator = &Provisioner{}

func (p *Provisioner) ValidateCreate() error {
	return p.validate()
}

func (p *Provisioner) ValidateUpdate(old runtime.Object) error {
	return p.validate()
}

func (p *Provisioner) ValidateDelete() error {
	return nil
}

func (p *Provisioner) validate() error {
	return functional.AllSucceed(
		p.validateClusterSpec,
		p.validateRestrictedLabels,
		p.validateArchitecture,
		p.validateOperatingSystem,
	)
}

func (p *Provisioner) validateClusterSpec() error {
	if p.Spec.Cluster == nil {
		return fmt.Errorf("spec.cluster must be defined")
	}
	if len(p.Spec.Cluster.Name) == 0 {
		return fmt.Errorf("spec.cluster.name cannot be empty")
	}
	if len(p.Spec.Cluster.Endpoint) == 0 {
		return fmt.Errorf("spec.cluster.endpoint cannot be empty")
	}
	if len(p.Spec.Cluster.CABundle) == 0 {
		return fmt.Errorf("spec.cluster.caBundle cannot be empty")
	}
	return nil
}

func (p *Provisioner) validateRestrictedLabels() error {
	for label := range p.Spec.Labels {
		for _, restricted := range RestrictedLabels {
			if restricted == label {
				return fmt.Errorf("spec.labels contains restricted label %s", label)
			}
		}
	}
	return nil
}

func (p *Provisioner) validateArchitecture() error {
	if p.Spec.Architecture == nil {
		return nil
	}
	for _, architecture := range SupportedArchitectures {
		if architecture == Architecture(*p.Spec.Architecture) {
			return nil
		}
	}
	return fmt.Errorf("unsupported architecture %s", *p.Spec.Architecture)
}

func (p *Provisioner) validateOperatingSystem() error {
	if p.Spec.OperatingSystem == nil {
		return nil
	}
	for _, operatingSystem := range SupportedOperatingSystems {
		if operatingSystem == OperatingSystem(*p.Spec.OperatingSystem) {
			return nil
		}
	}
	return fmt.Errorf("unsupported architecture %s", *p.Spec.OperatingSystem)
}

var SupportedArchitectures []Architecture

// AddSupportedArchitectures is populated by cloud providers.
func AddSupportedArchitectures(architecture ...Architecture) {
	SupportedArchitectures = append(SupportedArchitectures, architecture...)
}

var SupportedOperatingSystems []OperatingSystem

// AddSupportedOperatingSystems is populated by cloud providers.
func AddSupportedOperatingSystems(architecture ...OperatingSystem) {
	SupportedOperatingSystems = append(SupportedOperatingSystems, architecture...)
}

var RestrictedLabels []string

// AddRestrictedLabels is populated by cloud providers.
func AddRestrictedLabels(key ...string) {
	RestrictedLabels = append(RestrictedLabels, key...)
}

func init() {
	AddRestrictedLabels(
		ArchitectureLabelKey,
		OperatingSystemLabelKey,
		ProvisionerNameLabelKey,
		ProvisionerNamespaceLabelKey,
		ProvisionerPhaseLabel,
		ProvisionerTTLKey,
		ZoneLabelKey,
		InstanceTypeLabelKey,
	)
}
