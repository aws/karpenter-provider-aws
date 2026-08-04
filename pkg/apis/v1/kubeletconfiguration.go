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
	"math"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ParsedKubeletConfig holds the extracted kubelet configuration fields that Karpenter
// uses for scheduling decisions and bootstrap scripting.
type ParsedKubeletConfig struct {
	ClusterDNS []string `json:"clusterDNS,omitempty"`
	// MaxPods is an integer or a CEL expression evaluated per instance type. 
	// Use MaxPodsValue for the resolved count.
	MaxPods                     *intstr.IntOrString        `json:"maxPods,omitempty"`
	PodsPerCore                 *int32                     `json:"podsPerCore,omitempty"`
	SystemReserved              map[string]string          `json:"systemReserved,omitempty"`
	KubeReserved                map[string]string          `json:"kubeReserved,omitempty"`
	EvictionHard                map[string]string          `json:"evictionHard,omitempty"`
	EvictionSoft                map[string]string          `json:"evictionSoft,omitempty"`
	EvictionSoftGracePeriod     map[string]metav1.Duration `json:"evictionSoftGracePeriod,omitempty"`
	EvictionMaxPodGracePeriod   *int32                     `json:"evictionMaxPodGracePeriod,omitempty"`
	ImageGCHighThresholdPercent *int32                     `json:"imageGCHighThresholdPercent,omitempty"`
	ImageGCLowThresholdPercent  *int32                     `json:"imageGCLowThresholdPercent,omitempty"`
	CPUCFSQuota                 *bool                      `json:"cpuCFSQuota,omitempty"`
}

// ParseKubeletConfig unmarshals the unstructured kubelet config map into a typed struct
// containing the fields Karpenter needs for scheduling and bootstrap.
func ParseKubeletConfig(kc KubeletConfiguration) (*ParsedKubeletConfig, error) {
	if len(kc) == 0 {
		return &ParsedKubeletConfig{}, nil
	}
	data, err := json.Marshal(kc)
	if err != nil {
		return nil, err
	}
	parsed := &ParsedKubeletConfig{}
	return parsed, json.Unmarshal(data, parsed)
}

// MaxPodsInt returns the maxPods value as *int32 if it's set to an integer, or nil otherwise
// (unset, or a CEL expression). It's a convenience for callers that need the static integer value
// and handle expressions separately. Parsing the open map is the single source of truth; a value
// that doesn't decode is treated as unset rather than surfacing the error here.
func (kc KubeletConfiguration) MaxPodsInt() *int32 {
	parsed, err := ParseKubeletConfig(kc)
	if err != nil {
		return nil
	}
	value, ok := parsed.MaxPodsValue()
	if !ok {
		return nil
	}
	return value
}

// HasExpressions reports whether any field holds a CEL expression rather than a static value. The
// definition of "is an expression" must match what the evaluators actually treat as one: a
// string-typed maxPods, or a kubeReserved/systemReserved value that isn't a parseable resource
// quantity. A config that fails to parse is treated as having no expressions here; validation
// surfaces the decode error separately.
func (kc KubeletConfiguration) HasExpressions() bool {
	parsed, err := ParseKubeletConfig(kc)
	if err != nil {
		return false
	}
	if parsed.MaxPods != nil && parsed.MaxPods.Type == intstr.String {
		return true
	}
	return parsed.HasResourceExpressions()
}

// HasResourceExpressions reports whether kubeReserved or systemReserved holds a CEL expression -
// that is, a value that isn't a parseable resource quantity. Callers use this to skip resolving
// maxPods and building the reserved-capacity CEL variables when neither map has anything to evaluate.
func (kc KubeletConfiguration) HasResourceExpressions() bool {
	parsed, err := ParseKubeletConfig(kc)
	if err != nil {
		return false
	}
	return parsed.HasResourceExpressions()
}

// HasResourceExpressions reports whether kubeReserved or systemReserved holds a value that isn't a
// parseable resource quantity, i.e. a CEL expression. Defined on the parsed config so callers that
// already parsed the open map can reuse it without re-parsing.
func (in *ParsedKubeletConfig) HasResourceExpressions() bool {
	if in == nil {
		return false
	}
	for _, m := range []map[string]string{in.KubeReserved, in.SystemReserved} {
		for _, v := range m {
			if _, err := resource.ParseQuantity(v); err != nil {
				return true
			}
		}
	}
	return false
}

// MaxPodsValue returns maxPods as a concrete count, reporting false if it isn't set or holds a
// CEL expression rather than an integer. An expression can only be resolved against a specific
// instance type, so callers that have one must evaluate it themselves.
func (in *ParsedKubeletConfig) MaxPodsValue() (*int32, bool) {
	if in == nil || in.MaxPods == nil || in.MaxPods.Type != intstr.Int {
		return nil, false
	}
	// IntOrString holds an int32 internally, so IntValue can't be out of range on any platform
	// where int is at least 32 bits. Checked rather than asserted so a future widening upstream
	// surfaces as a missing value instead of a silently truncated pod limit.
	value := in.MaxPods.IntValue()
	if value < math.MinInt32 || value > math.MaxInt32 {
		return nil, false
	}
	return lo.ToPtr(int32(value)), true
}

// DeepCopy returns a deep copy of ParsedKubeletConfig.
func (in *ParsedKubeletConfig) DeepCopy() *ParsedKubeletConfig {
	if in == nil {
		return nil
	}
	data, err := json.Marshal(in)
	if err != nil {
		return &ParsedKubeletConfig{}
	}
	out := &ParsedKubeletConfig{}
	if err := json.Unmarshal(data, out); err != nil {
		return &ParsedKubeletConfig{}
	}
	// json round-trip doesn't preserve nil vs empty for CPUCFSQuota
	if in.CPUCFSQuota != nil {
		out.CPUCFSQuota = lo.ToPtr(*in.CPUCFSQuota)
	}
	if in.MaxPods != nil {
		out.MaxPods = lo.ToPtr(*in.MaxPods)
	}
	return out
}
