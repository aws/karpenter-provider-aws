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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/karpenter/pkg/utils/resources"
)

// InFlightNodeSpec is the Schema for an inflight node, which is a node Karpenter has launched but which hasn't become
// ready yet.
// +kubebuilder:printcolumn:JSONPath=".spec.providerID",name=ProviderID,type=string
type InFlightNodeSpec struct {
	ProviderID  string            `json:"providerID"`
	Provisioner string            `json:"provisioner"`
	Labels      map[string]string `json:"labels,omitempty"`
	Taints      []v1.Taint        `json:"taints,omitempty"`
	Overhead    v1.ResourceList   `json:"overhead,omitempty"`
	Capacity    v1.ResourceList   `json:"capacity,omitempty"`
}

// InFlightNode is the Schema for the InFlightNode API
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=inflightnodes,scope=Cluster,categories=karpenter,shortName=inflight
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID"
type InFlightNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              InFlightNodeSpec `json:"spec,omitempty"`
}

func (in *InFlightNode) ToNode() *v1.Node {
	n := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              in.Name,
			CreationTimestamp: in.CreationTimestamp,
			DeletionTimestamp: in.DeletionTimestamp,
			Labels: map[string]string{
				ProvisionerNameLabelKey: in.Spec.Provisioner,
			},
		},
		Spec: v1.NodeSpec{
			ProviderID: in.Spec.ProviderID,
			Taints:     in.Spec.Taints,
		},
		Status: v1.NodeStatus{
			Capacity:    in.Spec.Capacity,
			Allocatable: resources.Subtract(in.Spec.Capacity, in.Spec.Overhead),
			Phase:       v1.NodePending,
		},
	}
	for k, v := range in.Spec.Labels {
		n.Labels[k] = v
	}
	return n
}

// InFlightNodeList contains a list of InFlightNode
// +kubebuilder:object:root=true
type InFlightNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InFlightNode `json:"items"`
}
