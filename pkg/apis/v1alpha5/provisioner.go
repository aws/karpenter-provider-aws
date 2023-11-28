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

	"github.com/aws/aws-sdk-go/service/ec2"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"

	"sigs.k8s.io/karpenter/pkg/apis/v1alpha5"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
)

// Provisioner is an alias type for additional validation
// +kubebuilder:object:root=true
type Provisioner v1alpha5.Provisioner

func (p *Provisioner) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{
		admissionregistrationv1.Create,
		admissionregistrationv1.Update,
	}
}

func (p *Provisioner) Validate(_ context.Context) (errs *apis.FieldError) {
	if p.Spec.Provider == nil {
		return nil
	}
	provider, err := v1alpha1.DeserializeProvider(p.Spec.Provider.Raw)
	if err != nil {
		return apis.ErrGeneric(err.Error())
	}
	return provider.Validate()
}

func (p *Provisioner) SetDefaults(_ context.Context) {
	requirements := scheduling.NewNodeSelectorRequirements(p.Spec.Requirements...)

	// default to linux OS
	if !requirements.Has(v1.LabelOSStable) {
		p.Spec.Requirements = append(p.Spec.Requirements, v1.NodeSelectorRequirement{
			Key: v1.LabelOSStable, Operator: v1.NodeSelectorOpIn, Values: []string{string(v1.Linux)},
		})
	}

	// default to amd64
	if !requirements.Has(v1.LabelArchStable) {
		p.Spec.Requirements = append(p.Spec.Requirements, v1.NodeSelectorRequirement{
			Key: v1.LabelArchStable, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha5.ArchitectureAmd64},
		})
	}

	// default to on-demand
	if !requirements.Has(v1alpha5.LabelCapacityType) {
		p.Spec.Requirements = append(p.Spec.Requirements, v1.NodeSelectorRequirement{
			Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{ec2.DefaultTargetCapacityTypeOnDemand},
		})
	}

	// default to C, M, R categories if no instance type constraints are specified
	if !requirements.Has(v1.LabelInstanceTypeStable) &&
		!requirements.Has(v1alpha1.LabelInstanceFamily) &&
		!requirements.Has(v1alpha1.LabelInstanceCategory) &&
		!requirements.Has(v1alpha1.LabelInstanceGeneration) {
		p.Spec.Requirements = append(p.Spec.Requirements, []v1.NodeSelectorRequirement{
			{Key: v1alpha1.LabelInstanceCategory, Operator: v1.NodeSelectorOpIn, Values: []string{"c", "m", "r"}},
			{Key: v1alpha1.LabelInstanceGeneration, Operator: v1.NodeSelectorOpGt, Values: []string{"2"}},
		}...)
	}
}
