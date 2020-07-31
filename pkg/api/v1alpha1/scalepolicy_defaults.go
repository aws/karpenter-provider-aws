package v1alpha1

import "sigs.k8s.io/controller-runtime/pkg/webhook"

// +kubebuilder:webhook:path=/mutate-karpenter-my-domain-v1alpha1-scalepolicy,mutating=true,failurePolicy=fail,groups=karpenter.my.domain,resources=scalepolicies,verbs=create;update,versions=v1alpha1,name=mscalepolicy.kb.io

var _ webhook.Defaulter = &ScalePolicy{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ScalePolicy) Default() {
	scalepolicylog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}
