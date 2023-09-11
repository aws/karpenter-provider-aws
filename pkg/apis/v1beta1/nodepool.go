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
	"knative.dev/pkg/apis"

	"github.com/aws/karpenter-core/pkg/apis/v1beta1"
)

// NodePool is an alias type for additional validation
// +kubebuilder:object:root=true
type NodePool v1beta1.NodePool

func (n *NodePool) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{
		admissionregistrationv1.Create,
		admissionregistrationv1.Update,
	}
}

func (n *NodePool) Validate(_ context.Context) (errs *apis.FieldError) { return nil }

func (n *NodePool) SetDefaults(ctx context.Context) {
	spec := NodeClaimSpec(n.Spec.Template.Spec)
	spec.SetDefaults(ctx)
	n.Spec.Template.Spec = v1beta1.NodeClaimSpec(spec)
}
