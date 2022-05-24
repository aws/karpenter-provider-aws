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
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

const (
	MachineFinalizer    = "machines.karpenter.sh"
	MachineNameLabelKey = "machines.karpenter.sh/name"
)

type MachineSpec struct {
	// Labels are layered with Requirements and applied to every node.
	//+optional
	Labels map[string]string `json:"labels,omitempty"`
	// Taints will be applied to every node launched by the Machine. If
	// specified, the provisioner will not provision nodes for pods that do not
	// have matching tolerations. Additional taints will be created that match
	// pod tolerations on a per-node basis.
	// +optional
	Taints []v1.Taint `json:"taints,omitempty"`
	// StartupTaints are taints that are applied to nodes upon startup which are
	// expected to be removed automatically within a short period of time,
	// typically by a DaemonSet that tolerates the taint. These are commonly
	// used by daemonsets to allow initialization and enforce startup ordering.
	// StartupTaints are ignored for provisioning purposes in that pods are not
	// required to tolerate a StartupTaint in order to have nodes provisioned
	// for them.
	// +optional
	StartupTaints []v1.Taint `json:"startupTaints,omitempty"`
	// Requirements are layered with Labels and applied to every node.
	Requirements []v1.NodeSelectorRequirement `json:"requirements,omitempty"`
	// KubeletConfiguration are options passed to the kubelet when provisioning nodes
	// +optional
	KubeletConfiguration *KubeletConfiguration `json:"kubeletConfiguration,omitempty"`
	// Provider contains fields specific to your cloudprovider.
	// +kubebuilder:pruning:PreserveUnknownFields
	Provider *Provider `json:"provider,omitempty"`
}

// Machine is the Schema for the Machines API
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=machines,scope=Cluster,categories=karpenter
// +kubebuilder:subresource:status
type Machine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MachineSpec   `json:"spec,omitempty"`
	Status MachineStatus `json:"status,omitempty"`
}

// MachineList contains a list of Machine
// +kubebuilder:object:root=true
type MachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Machine `json:"items"`
}

// Validate the object
func (m *Machine) Validate(ctx context.Context) (errs *apis.FieldError) {
	return nil
}

// SetDefaults for the object
func (m *Machine) SetDefaults(ctx context.Context) {}
