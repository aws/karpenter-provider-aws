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
	"github.com/samber/lo"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeTemplate encapsulates the fields required to create a node and mirrors
// the fields in Provisioner. These structs are maintained separately in order
// for fields like Requirements to be able to be stored more efficiently.
type NodeTemplate struct {
	ProvisionerName      string
	Provider             *v1alpha5.Provider
	ProviderRef          *v1alpha5.ProviderRef
	Labels               map[string]string
	Taints               Taints
	StartupTaints        Taints
	Requirements         Requirements
	KubeletConfiguration *v1alpha5.KubeletConfiguration
}

func NewNodeTemplate(provisioner *v1alpha5.Provisioner) *NodeTemplate {
	labels := lo.Assign(provisioner.Spec.Labels, map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner.Name})
	requirements := NewRequirements()
	requirements.Add(NewNodeSelectorRequirements(provisioner.Spec.Requirements...).Values()...)
	requirements.Add(NewLabelRequirements(labels).Values()...)
	return &NodeTemplate{
		ProvisionerName:      provisioner.Name,
		Provider:             provisioner.Spec.Provider,
		ProviderRef:          provisioner.Spec.ProviderRef,
		KubeletConfiguration: provisioner.Spec.KubeletConfiguration,
		Labels:               labels,
		Taints:               provisioner.Spec.Taints,
		StartupTaints:        provisioner.Spec.StartupTaints,
		Requirements:         requirements,
	}
}

func (n *NodeTemplate) ToNode() *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Labels:     lo.Assign(n.Labels, n.Requirements.Labels()),
			Finalizers: []string{v1alpha5.TerminationFinalizer},
		},
		Spec: v1.NodeSpec{
			Taints: append(n.Taints, n.StartupTaints...),
		},
	}
}
