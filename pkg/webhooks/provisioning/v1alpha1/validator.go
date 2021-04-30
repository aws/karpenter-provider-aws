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
	"context"
	"fmt"
	"net/http"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/utils/functional"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Validator validates provisioners
type Validator struct {
	CloudProvider cloudprovider.Factory
	decoder       *admission.Decoder
}

// Path of the webhook handler
func (v *Validator) Path() string {
	return "/validate-provisioning-karpenter-sh-v1alpha1-provisioner"
}

// InjectDecoder injects the decoder for each request.
func (v *Validator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}

// Handle a validation request for the Provisioner
func (v *Validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	provisioner := &v1alpha1.Provisioner{}
	err := v.decoder.Decode(req, provisioner)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if err := functional.ValidateAll(
		func() error { return v.validateClusterSpec(ctx, provisioner) },
		func() error { return v.validateLabels(ctx, provisioner) },
		func() error { return v.validateZones(ctx, provisioner) },
		func() error { return v.validateInstanceTypes(ctx, provisioner) },
		func() error { return v.validateArchitecture(ctx, provisioner) },
		func() error { return v.validateOperatingSystem(ctx, provisioner) },
		func() error { return v.CloudProvider.CapacityFor(provisioner).Validate(ctx) },
	); err != nil {
		return admission.Denied(fmt.Sprintf("failed to validate provisioner '%s/%s', %s", provisioner.Name, provisioner.Namespace, err.Error()))
	}
	return admission.Allowed("")
}

func (v *Validator) validateClusterSpec(ctx context.Context, provisioner *v1alpha1.Provisioner) error {
	if provisioner.Spec.Cluster == nil {
		return fmt.Errorf("spec.cluster must be defined")
	}
	if len(provisioner.Spec.Cluster.Name) == 0 {
		return fmt.Errorf("spec.cluster.name cannot be empty")
	}
	if len(provisioner.Spec.Cluster.Endpoint) == 0 {
		return fmt.Errorf("spec.cluster.endpoint cannot be empty")
	}
	if len(provisioner.Spec.Cluster.CABundle) == 0 {
		return fmt.Errorf("spec.cluster.caBundle cannot be empty")
	}
	return nil
}

func (v *Validator) validateLabels(ctx context.Context, provisioner *v1alpha1.Provisioner) error {
	for label := range provisioner.Spec.Labels {
		for _, restricted := range []string{
			v1alpha1.ArchitectureLabelKey,
			v1alpha1.OperatingSystemLabelKey,
			v1alpha1.ProvisionerNameLabelKey,
			v1alpha1.ProvisionerNamespaceLabelKey,
			v1alpha1.ProvisionerPhaseLabel,
			v1alpha1.ProvisionerTTLKey,
			v1alpha1.ZoneLabelKey,
			v1alpha1.InstanceTypeLabelKey,
		} {
			if restricted == label {
				return fmt.Errorf("spec.labels contains restricted label '%s'", label)
			}
		}
	}
	return nil
}

func (v *Validator) validateArchitecture(ctx context.Context, provisioner *v1alpha1.Provisioner) error {
	if provisioner.Spec.Architecture == nil {
		return nil
	}
	supportedArchitectures, err := v.CloudProvider.CapacityFor(provisioner).GetArchitectures(ctx)
	if err != nil {
		return fmt.Errorf("getting supported architectures, %w", err)
	}
	if !functional.ContainsString(supportedArchitectures, *provisioner.Spec.Architecture) {
		return fmt.Errorf("unsupported architecture '%s' not in %v", *provisioner.Spec.Architecture, supportedArchitectures)
	}
	return nil
}

func (v *Validator) validateOperatingSystem(ctx context.Context, provisioner *v1alpha1.Provisioner) error {
	if provisioner.Spec.OperatingSystem == nil {
		return nil
	}
	supportedOperatingSystems, err := v.CloudProvider.CapacityFor(provisioner).GetOperatingSystems(ctx)
	if err != nil {
		return fmt.Errorf("getting supported operating systems, %w", err)
	}
	if !functional.ContainsString(supportedOperatingSystems, *provisioner.Spec.OperatingSystem) {
		return fmt.Errorf("unsupported operating system '%s' not in %v", *provisioner.Spec.OperatingSystem, supportedOperatingSystems)
	}
	return nil
}

func (v *Validator) validateZones(ctx context.Context, provisioner *v1alpha1.Provisioner) error {
	if provisioner.Spec.Zones == nil {
		return nil
	}
	supportedZones, err := v.CloudProvider.CapacityFor(provisioner).GetZones(ctx)
	if err != nil {
		return fmt.Errorf("getting supported zones, %w", err)
	}
	for _, zone := range provisioner.Spec.Zones {
		if !functional.ContainsString(supportedZones, zone) {
			return fmt.Errorf("unsupported zone '%s' not in %v", zone, supportedZones)
		}
	}
	return nil
}

func (v *Validator) validateInstanceTypes(ctx context.Context, provisioner *v1alpha1.Provisioner) error {
	if provisioner.Spec.InstanceTypes == nil {
		return nil
	}
	instanceTypes, err := v.CloudProvider.CapacityFor(provisioner).GetInstanceTypes(ctx)
	if err != nil {
		return fmt.Errorf("getting supported instance types, %w", err)
	}
	instanceTypeNames := []string{}
	for _, instanceType := range instanceTypes {
		instanceTypeNames = append(instanceTypeNames, instanceType.Name())
	}
	for _, instanceType := range provisioner.Spec.InstanceTypes {
		if !functional.ContainsString(instanceTypeNames, instanceType) {
			return fmt.Errorf("unsupported instance type '%s' not in %v", instanceType, instanceTypeNames)
		}
	}
	return nil
}
