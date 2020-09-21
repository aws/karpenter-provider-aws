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

// +kubebuilder:webhook:path=/mutate-autoscaling-karpenter-sh-v1alpha1-horizontalautoscaler,mutating=true,sideEffects=None,failurePolicy=fail,groups=autoscaling.karpenter.sh,resources=horizontalautoscalers,verbs=create;update,versions=v1alpha1,name=mhorizontalautoscaler.kb.io

package v1alpha1

import (
	v2beta2 "k8s.io/api/autoscaling/v2beta2"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var _ webhook.Defaulter = &HorizontalAutoscaler{}

var (
	DefaultScaleDownStabilizationWindowSeconds int32 = 300
	DefaultScaleUpStabilizationWindowSeconds   int32 = 3
	DefaultSelectPolicy                              = v2beta2.MaxPolicySelect
)

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *HorizontalAutoscaler) Default() {
	// TODO(user): fill in your defaulting logic.
}

// RuntimeDefault is used to set defaults for the resource in memory, but will not be persisted to the API Server.
// This is useful when the codebase has a default behavior that isn't surfaced to the user via the API.
func (r *HorizontalAutoscaler) RuntimeDefault() {
	if r.Spec.Behavior.ScaleUp == nil {
		r.Spec.Behavior.ScaleUp = &ScalingRules{}
	}
	if r.Spec.Behavior.ScaleDown == nil {
		r.Spec.Behavior.ScaleDown = &ScalingRules{}
	}
	if r.Spec.Behavior.ScaleUp.StabilizationWindowSeconds == nil {
		r.Spec.Behavior.ScaleUp.StabilizationWindowSeconds = &DefaultScaleUpStabilizationWindowSeconds
	}
	if r.Spec.Behavior.ScaleUp.SelectPolicy == nil {
		r.Spec.Behavior.ScaleUp.SelectPolicy = &DefaultSelectPolicy
	}
	if r.Spec.Behavior.ScaleDown.StabilizationWindowSeconds == nil {
		r.Spec.Behavior.ScaleUp.StabilizationWindowSeconds = &DefaultScaleDownStabilizationWindowSeconds
	}
	if r.Spec.Behavior.ScaleDown.SelectPolicy == nil {
		r.Spec.Behavior.ScaleDown.SelectPolicy = &DefaultSelectPolicy
	}
}
