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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/rand"
)

// NodeTemplate encapsulates the fields required to create a node and mirrors
// the fields in Provisioner. These structs are maintained separately in order
// for fields like Requirements to be able to be stored more efficiently.
type NodeTemplate struct {
	ProvisionerName      string
	Provider             *v1alpha5.Provider
	ProviderRef          *v1alpha5.ProviderRef
	ProviderRefNamespace string
	Labels               map[string]string
	Taints               Taints
	StartupTaints        Taints
	Requirements         Requirements
	KubeletConfiguration *v1alpha5.KubeletConfiguration
}

func NewNodeTemplate(provisioner *v1alpha5.Provisioner, requirements ...Requirements) *NodeTemplate {
	provisioner.Spec.Labels = functional.UnionStringMaps(provisioner.Spec.Labels, map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner.Name})
	return &NodeTemplate{
		ProvisionerName:      provisioner.Name,
		Provider:             provisioner.Spec.Provider,
		ProviderRef:          provisioner.Spec.ProviderRef,
		KubeletConfiguration: provisioner.Spec.KubeletConfiguration,
		Labels:               provisioner.Spec.Labels,
		Taints:               provisioner.Spec.Taints,
		StartupTaints:        provisioner.Spec.StartupTaints,
		Requirements: NewRequirements(append(
			requirements,
			NewNodeSelectorRequirements(provisioner.Spec.Requirements...),
			NewLabelRequirements(provisioner.Spec.Labels),
		)...),
	}
}

func (n *NodeTemplate) ToNode() *v1.Node {
	labels := map[string]string{}
	for key, value := range n.Labels {
		labels[key] = value
	}
	for key := range n.Requirements.Keys() {
		if !v1alpha5.IsRestrictedNodeLabel(key) {
			switch n.Requirements.Get(key).Type() {
			case v1.NodeSelectorOpIn:
				labels[key] = n.Requirements.Get(key).Values().UnsortedList()[0]
			case v1.NodeSelectorOpExists:
				labels[key] = rand.String(10)
			}
		}
	}
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Labels:     labels,
			Finalizers: []string{v1alpha5.TerminationFinalizer},
			Annotations: map[string]string{
				v1alpha5.NotReadyAnnotationKey: "true",
			},
		},
		Status: v1.NodeStatus{
			// We need to put a not ready condition on the node or else the node-controller will immediately remove
			// the standard v1.TaintNodeNotReady taint that we add below.
			Conditions: []v1.NodeCondition{
				{
					Type:               v1.NodeReady,
					Status:             v1.ConditionFalse,
					LastHeartbeatTime:  metav1.Now(),
					LastTransitionTime: metav1.Now(),
					Reason:             "KubeletNotReady",
				},
			},
		},
		Spec: v1.NodeSpec{
			Taints: append(append(n.Taints, n.StartupTaints...), v1.Taint{Key: v1.TaintNodeNotReady, Effect: v1.TaintEffectNoSchedule}),
		},
	}
}
