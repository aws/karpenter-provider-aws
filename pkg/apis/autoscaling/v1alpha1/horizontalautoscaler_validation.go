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

// +kubebuilder:webhook:verbs=create;update;delete,path=/validate-autoscaling-karpenter-sh-v1alpha1-horizontalautoscaler,mutating=false,sideEffects=None,failurePolicy=fail,groups=autoscaling.karpenter.sh,resources=horizontalautoscalers,versions=v1alpha1,name=vhorizontalautoscaler.kb.io

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var _ webhook.Validator = &HorizontalAutoscaler{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *HorizontalAutoscaler) ValidateCreate() error {

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *HorizontalAutoscaler) ValidateUpdate(old runtime.Object) error {

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *HorizontalAutoscaler) ValidateDelete() error {

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
