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

	"github.com/aws/karpenter/pkg/utils/pretty"
	"github.com/mitchellh/hashstructure/v2"
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
	Requirements Requirements `json:"requirements,omitempty"`
	// Provider contains fields specific to your cloudprovider.
	// +kubebuilder:pruning:PreserveUnknownFields
	Provider *runtime.RawExtension `json:"provider,omitempty"`
}

// Tighten the constraints to include the pod or an error if invalid
func (c *Constraints) Tighten(pod *v1.Pod) (*Constraints, error) {
	// Tolerate Taints
	if err := c.Taints.Tolerates(pod); err != nil {
		return nil, err
	}
	// The constraints do not support this requirement
	podRequirements := PodRequirements(pod)
	for _, key := range podRequirements.Keys() {
		if c.Requirements.Requirement(key).Len() == 0 {
			return nil, fmt.Errorf("invalid nodeSelector %q, %v not in %v", key, podRequirements.Requirement(key).UnsortedList(), c.Requirements.Requirement(key).UnsortedList())
		}
	}
	// The combined requirements are not compatible
	combined := c.Requirements.With(podRequirements).Consolidate()
	for _, key := range podRequirements.Keys() {
		if combined.Requirement(key).Len() == 0 {
			return nil, fmt.Errorf("invalid nodeSelector %q, %v not in %v", key, podRequirements.Requirement(key).UnsortedList(), c.Requirements.Requirement(key).UnsortedList())
		}
	}
	// Tightened constraints
	return &Constraints{
		Labels:       c.Labels,
		Requirements: combined.WellKnown(),
		Taints:       c.Taints,
		Provider:     c.Provider,
	}, nil
}

func (c *Constraints) MustHash() string {
	key, err := hashstructure.Hash(c, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	if err != nil {
		panic(fmt.Errorf("hashing constraints %s, %w", pretty.Concise(c), err))
	}
	return fmt.Sprint(key)
}
