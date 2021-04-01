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

// +kubebuilder:webhook:path=/mutate-provisioning-karpenter-sh-v1alpha1-provisioner,mutating=true,sideEffects=None,failurePolicy=fail,groups=provisioning.karpenter.sh,resources=provisioners,verbs=create;update,versions=v1alpha1,name=mutation.provisioning.karpenter.sh

package v1alpha1

import (
	"context"
	"encoding/json"
	"net/http"

	provisioning "github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Defaulter defaults Provisioners
type Defaulter struct {
	decoder *admission.Decoder
}

// Path of the webhook handler
func (v *Defaulter) Path() string {
	return "/mutate-provisioning-karpenter-sh-v1alpha1-provisioner"
}

// InjectDecoder injects the decoder.
func (v *Defaulter) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}

// Handle a defaulting request for a Provisioner
func (v *Defaulter) Handle(ctx context.Context, req admission.Request) admission.Response {
	provisioner := &provisioning.Provisioner{}
	err := v.decoder.Decode(req, provisioner)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	v.applyDefaults(&provisioner.Spec)
	marshaled, err := json.Marshal(provisioner)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaled)
}

func (v *Defaulter) applyDefaults(spec *provisioning.ProvisionerSpec) {
	v.defaultTTL(spec)
}

func (v *Defaulter) defaultTTL(spec *provisioning.ProvisionerSpec) {
	if spec.TTLSeconds == nil {
		spec.TTLSeconds = ptr.Int32(300)
	}
}
