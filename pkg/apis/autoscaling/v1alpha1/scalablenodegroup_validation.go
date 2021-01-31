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

// +kubebuilder:webhook:verbs=create;update,path=/validate-autoscaling-karpenter-sh-v1alpha1-scalablenodegroup,mutating=false,sideEffects=None,failurePolicy=fail,groups=autoscaling.karpenter.sh,resources=scalablenodegroups,versions=v1alpha1,name=scalablenodegroup.kb.io
package v1alpha1

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var _ webhook.Validator = &ScalableNodeGroup{}

func (sng *ScalableNodeGroup) ValidateCreate() error {
	return nil
}

func (sng *ScalableNodeGroup) ValidateUpdate(old runtime.Object) error {
	return nil
}

func (sng *ScalableNodeGroup) ValidateDelete() error {
	return nil
}

// +kubebuilder:object:generate=false
type ScalableNodeGroupValidator func(*ScalableNodeGroupSpec) error

var scalableNodeGroupValidators = map[NodeGroupType]ScalableNodeGroupValidator{}

func RegisterScalableNodeGroupValidator(nodeGroupType NodeGroupType, validator ScalableNodeGroupValidator) {
	scalableNodeGroupValidators[nodeGroupType] = validator
}

func (sng *ScalableNodeGroup) Validate() error {
	validator, ok := scalableNodeGroupValidators[sng.Spec.Type]
	if !ok {
		return fmt.Errorf("Unexpected type %v", sng.Spec.Type)
	}
	if err := validator(&sng.Spec); err != nil {
		return fmt.Errorf("Invalid ScalableNodeGroup, %w", err)
	}
	return nil
}
