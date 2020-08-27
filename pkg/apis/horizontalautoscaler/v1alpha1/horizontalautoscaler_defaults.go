package v1alpha1

import "sigs.k8s.io/controller-runtime/pkg/webhook"

// +kubebuilder:webhook:path=/mutate-karpenter-sh-v1alpha1-horizontalautoscaler,mutating=true,failurePolicy=fail,groups=karpenter.sh,resources=horizontalautoscalers,verbs=create;update,versions=v1alpha1,name=mhorizontalautoscaler.kb.io
var _ webhook.Defaulter = &HorizontalAutoscaler{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *HorizontalAutoscaler) Default() {
	// TODO(user): fill in your defaulting logic.
}
