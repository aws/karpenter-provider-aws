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
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Constraints are applied to all nodes created by the provisioner.
type Constraints struct {
	// Labels are layered with Requirements and applied to every node.
	//+optional
	Labels map[string]string `json:"labels,omitempty"`
	// Taints will be applied to every node launched by the Provisioner. If
	// specified, the provisioner will not provision nodes for pods that do not
	// have matching tolerations. Additional taints will be created that match
	// pod tolerations on a per-node basis.
	// +optional
	Taints Taints `json:"taints,omitempty"`
	// Requirements are layered with Labels and applied to every node.
	Requirements Requirements `json:"requirements,inline,omitempty"`
	// KubeletConfiguration are options passed to the kubelet when provisioning nodes
	//+optional
	KubeletConfiguration KubeletConfiguration `json:"kubeletConfiguration,omitempty"`
	// Provider contains fields specific to your cloudprovider.
	// +kubebuilder:pruning:PreserveUnknownFields
	Provider *Provider `json:"provider,omitempty"`
}

// +kubebuilder:object:generate=false
type Provider = runtime.RawExtension

// ValidatePod returns an error if the pod's requirements are not met by the constraints
func (c *Constraints) ValidatePod(pod *v1.Pod) error {
	// Tolerate Taints
	if err := c.Taints.Tolerates(pod); err != nil {
		return err
	}
	// Test if pod requirements are valid
	requirements := NewPodRequirements(pod)
	if errs := requirements.Validate(); errs != nil {
		return fmt.Errorf("pod requirements not feasible, %v", errs)
	}
	// Test if pod requirements are compatible
	if errs := c.Requirements.Compatible(requirements); errs != nil {
		return fmt.Errorf("incompatible requirements, %w", errs)
	}
	return nil
}

func (c *Constraints) Tighten(pod *v1.Pod) *Constraints {
	return &Constraints{
		Labels:               c.Labels,
		Requirements:         c.Requirements.Add(NewPodRequirements(pod).Requirements...).WellKnown(),
		Taints:               c.Taints,
		Provider:             c.Provider,
		KubeletConfiguration: c.KubeletConfiguration,
	}
}
