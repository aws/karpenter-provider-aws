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

package scheduling

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/rand"
)

// Constraints is a copy of the API Constraints type with more methods to be used during scheduling.
// +k8s:deepcopy-gen=true
type Constraints struct {
	Labels               map[string]string
	Requirements         Requirements
	Taints               v1alpha5.Taints                `json:"taints,omitempty"`
	KubeletConfiguration *v1alpha5.KubeletConfiguration `json:"kubeletConfiguration,omitempty"`
	Provider             *v1alpha5.Provider             `json:"provider,omitempty"`
}

func NewConstraints(constraints v1alpha5.Constraints) *Constraints {
	return &Constraints{
		Requirements:         NewRequirements(constraints.Requirements...),
		Taints:               constraints.Taints.DeepCopy(),
		Labels:               constraints.Labels,
		KubeletConfiguration: constraints.KubeletConfiguration,
		Provider:             constraints.Provider,
	}
}

// ValidatePod returns an error if the pod's requirements are not met by the constraints
func (c *Constraints) ValidatePod(pod *v1.Pod) error {
	// Tolerate Taints
	if err := c.Taints.Tolerates(pod); err != nil {
		return err
	}
	requirements := NewPodRequirements(pod)
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

func (c *Constraints) ToNode() *v1.Node {
	labels := map[string]string{}
	for key, value := range c.Labels {
		labels[key] = value
	}
	for key := range c.Requirements.Keys() {
		if !v1alpha5.IsRestrictedNodeLabel(key) {
			switch c.Requirements.Get(key).Type() {
			case v1.NodeSelectorOpIn:
				labels[key] = c.Requirements.Get(key).Values().UnsortedList()[0]
			case v1.NodeSelectorOpExists:
				labels[key] = rand.String(10)
			}
		}
	}
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Labels:     labels,
			Finalizers: []string{v1alpha5.TerminationFinalizer},
		},
		Spec: v1.NodeSpec{
			// Taint karpenter.sh/not-ready=NoSchedule to prevent the kube scheduler
			// from scheduling pods before we're able to bind them ourselves. The kube
			// scheduler has an eventually consistent cache of nodes and pods, so it's
			// possible for it to see a provisioned node before it sees the pods bound
			// to it. This creates an edge case where other pending pods may be bound to
			// the node by the kube scheduler, causing OutOfCPU errors when the
			// binpacked pods race to bind to the same node. The system eventually
			// heals, but causes delays from additional provisioning (thrash). This
			// taint will be removed by the node controller when a node is marked ready.
			Taints: append(c.Taints, v1.Taint{
				Key:    v1alpha5.NotReadyTaintKey,
				Effect: v1.TaintEffectNoSchedule,
			}),
		},
	}
}
