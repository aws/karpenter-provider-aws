package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// +kubebuilder:webhook:verbs=create;update;delete,path=/validate-karpenter.sh-v1alpha1-horizontalautoscaler,mutating=false,failurePolicy=fail,groups=karpenter.sh,resources=horizontalautoscalers,versions=v1alpha1,name=vhorizontalautoscaler.kb.io
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
