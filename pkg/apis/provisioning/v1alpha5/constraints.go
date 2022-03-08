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
	"crypto/rand"
	"encoding/base32"
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
	p := pod.DeepCopy()
	// The soft preference may conflict with the requirements.
	// Remove soft constraints/ preferences
	if p.Spec.Affinity != nil && p.Spec.Affinity.NodeAffinity != nil {
		p.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = nil
	}
	// Tolerate Taints
	if err := c.Taints.Tolerates(p); err != nil {
		return err
	}
	requirements := NewPodRequirements(p)
	// Test if labels are allowed
	for key := range requirements.Keys() {
		if err := IsRestrictedLabel(key); err != nil && !PodLabelExceptions.Has(key) {
			return err
		}
	}
	// Test if pod requirements are valid
	if err := requirements.Validate(); err != nil {
		return fmt.Errorf("invalid requirements, %w", err)
	}
	// Test if pod requirements are compatible to the provisioner
	if errs := c.Requirements.Compatible(requirements); errs != nil {
		return fmt.Errorf("incompatible requirements, %w", errs)
	}
	return nil
}

func (c *Constraints) Tighten(pod *v1.Pod) *Constraints {
	requirements := c.Requirements.Add(NewPodRequirements(pod).Requirements...)
	labels := map[string]string{}
	for key, value := range c.Labels {
		labels[key] = value
	}
	for key := range requirements.Keys() {
		if !IsRestrictedNodeLabel(key) {
			values := requirements.Get(key)
			if !values.IsComplement() && !values.IsEmpty() {
				labels[key] = values.Values().UnsortedList()[0]
			}
			// NotIn, Exists and DoesNotExist will have a complement value set.
			// Only write a random value if the requirements operator is Exists.
			// NotIn operator and DoesNotExist operator will schedule without the label
			if values.IsFull() {
				label := make([]byte, 32)
				_, err := rand.Read(label)
				if err != nil {
					panic(err)
				}
				labels[key] = base32.StdEncoding.EncodeToString(label)[:10]
			}
		}
	}
	return &Constraints{
		Labels:               labels,
		Requirements:         requirements,
		Taints:               c.Taints,
		Provider:             c.Provider,
		KubeletConfiguration: c.KubeletConfiguration,
	}
}
