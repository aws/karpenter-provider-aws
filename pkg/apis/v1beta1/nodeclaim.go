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

package v1beta1

import (
	"context"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"

	"github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter-core/pkg/scheduling"
)

// NodeClaim is an alias type for additional validation
// +kubebuilder:object:root=true
type NodeClaim v1beta1.NodeClaim

// NodeClaimSpec is an alias type for additional validation
type NodeClaimSpec v1beta1.NodeClaimSpec

func (n *NodeClaim) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{
		admissionregistrationv1.Create,
		admissionregistrationv1.Update,
	}
}

func (n *NodeClaim) Validate(_ context.Context) (errs *apis.FieldError) { return nil }

func (n *NodeClaim) SetDefaults(ctx context.Context) {
	spec := NodeClaimSpec(n.Spec)
	spec.SetDefaults(ctx)
	n.Spec = v1beta1.NodeClaimSpec(spec)
}

func (n *NodeClaimSpec) SetDefaults(_ context.Context) {
	requirements := scheduling.NewNodeSelectorRequirements(n.Requirements...)

	// default to linux OS
	if !requirements.Has(v1.LabelOSStable) {
		n.Requirements = append(n.Requirements, v1.NodeSelectorRequirement{
			Key: v1.LabelOSStable, Operator: v1.NodeSelectorOpIn, Values: []string{string(v1.Linux)},
		})
	}

	// default to amd64
	if !requirements.Has(v1.LabelArchStable) {
		n.Requirements = append(n.Requirements, v1.NodeSelectorRequirement{
			Key: v1.LabelArchStable, Operator: v1.NodeSelectorOpIn, Values: []string{v1beta1.ArchitectureAmd64},
		})
	}

	// default to on-demand
	if !requirements.Has(v1beta1.CapacityTypeLabelKey) {
		n.Requirements = append(n.Requirements, v1.NodeSelectorRequirement{
			Key: v1beta1.CapacityTypeLabelKey, Operator: v1.NodeSelectorOpIn, Values: []string{v1beta1.CapacityTypeOnDemand},
		})
	}

	// default to C, M, R categories if no instance type constraints are specified
	if !requirements.Has(v1.LabelInstanceTypeStable) &&
		!requirements.Has(LabelInstanceFamily) &&
		!requirements.Has(LabelInstanceCategory) &&
		!requirements.Has(LabelInstanceGeneration) {
		n.Requirements = append(n.Requirements, []v1.NodeSelectorRequirement{
			{Key: LabelInstanceCategory, Operator: v1.NodeSelectorOpIn, Values: []string{"c", "m", "r"}},
			{Key: LabelInstanceGeneration, Operator: v1.NodeSelectorOpGt, Values: []string{"2"}},
		}...)
	}
}
