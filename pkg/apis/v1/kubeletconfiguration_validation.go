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

package v1

import (
	"encoding/json"
	"fmt"
	"maps"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"

	// sigs.k8s.io/json reports unknown fields as errors rather than ignoring them, which
	// encoding/json can't do.
	sigsjson "sigs.k8s.io/json"
)

// ValidateKubeletConfig catches invalid kubelet configuration that the CRD schema structurally
// cannot reject.
//
// The EC2NodeClass CRD carries a typed schema for spec.kubelet generated from the k8s.io/kubelet
// version in go.mod (see hack/code/kubeletschema_gen), so the API server enforces types and the
// CEL rules for every client. Unknown fields are handled differently depending on where they sit
// and how the request was made:
//
//   - A top-level unknown field is rejected at apply time when the client requests Strict field
//     validation (kubectl, Helm, and Argo all do), and silently pruned when it doesn't. Pruning
//     happens before persistence, so by the time this function reads spec.kubelet back from the
//     cluster the field is already gone -- this is NOT the case being backstopped, because there
//     is nothing left to detect.
//
//   - A field nested inside a subtree the generated schema marks
//     x-kubernetes-preserve-unknown-fields (`logging` is the one such subtree today, because its
//     upstream type is opaque) is accepted and persisted no matter what the client asks for:
//     preserve-unknown-fields suppresses both pruning and the Strict unknown-field check.
//
// The second case is what this exists for. Decoding the persisted config against the upstream
// struct surfaces those fields as a status condition instead of leaving a typo the user believes
// is in effect but that the kubelet will never honor.
func ValidateKubeletConfig(kc KubeletConfiguration) []error {
	if len(kc) == 0 {
		return nil
	}
	// Fields that may hold a CEL expression are checked against the upstream type with a
	// placeholder standing in for the expression. Upstream types maxPods as int32, so an
	// expression string would fail to decode and mask every other field in the same document.
	// Only the shape matters here; the expression itself is validated where it's evaluated.
	if maxPods, ok := kc["maxPods"]; ok && isJSONString(maxPods) {
		kc = maps.Clone(kc)
		kc["maxPods"] = JSONValue(0)
	}
	data, err := json.Marshal(kc)
	if err != nil {
		return []error{fmt.Errorf("spec.kubelet: %w", err)}
	}
	strictErrs, err := sigsjson.UnmarshalStrict(data, &kubeletconfigv1beta1.KubeletConfiguration{})
	if err != nil {
		return []error{fmt.Errorf("spec.kubelet: %w", err)}
	}
	errs := make([]error, 0, len(strictErrs))
	for _, strictErr := range strictErrs {
		errs = append(errs, fmt.Errorf("spec.kubelet: %w", strictErr))
	}
	return errs
}

// isJSONString reports whether a raw JSON value is a string.
func isJSONString(v apiextensionsv1.JSON) bool {
	var s string
	return json.Unmarshal(v.Raw, &s) == nil
}
