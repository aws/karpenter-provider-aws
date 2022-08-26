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
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

// ProvisionerStatus defines the observed state of Provisioner
type ProvisionerStatus struct {
	// LastScaleTime is the last time the Provisioner scaled the number
	// of nodes
	// +optional
	// +kubebuilder:validation:Format="date-time"
	LastScaleTime *apis.VolatileTime `json:"lastScaleTime,omitempty"`

	// Conditions is the set of conditions required for this provisioner to scale
	// its target, and indicates whether or not those conditions are met.
	// +optional
	Conditions apis.Conditions `json:"conditions,omitempty"`

	// Resources is the list of resources that have been provisioned.
	Resources v1.ResourceList `json:"resources,omitempty"`
}

func (p *Provisioner) StatusConditions() apis.ConditionManager {
	return apis.NewLivingConditionSet(
		Active,
	).Manage(p)
}

func (p *Provisioner) GetConditions() apis.Conditions {
	return p.Status.Conditions
}

func (p *Provisioner) SetConditions(conditions apis.Conditions) {
	p.Status.Conditions = conditions
}
