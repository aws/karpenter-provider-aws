package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// +kubebuilder:webhook:verbs=create;update;delete,path=/validate-karpenter.sh-v1alpha1-scalepolicy,mutating=false,failurePolicy=fail,groups=karpenter.sh,resources=scalepolicies,versions=v1alpha1,name=vscalepolicy.kb.io
var _ webhook.Validator = &ScalePolicy{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ScalePolicy) ValidateCreate() error {
	scalepolicylog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ScalePolicy) ValidateUpdate(old runtime.Object) error {
	scalepolicylog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ScalePolicy) ValidateDelete() error {
	scalepolicylog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
