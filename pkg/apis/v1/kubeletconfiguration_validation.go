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
	"slices"
	"strconv"
	"strings"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"

	// sigs.k8s.io/json reports unknown fields as errors rather than ignoring them, which
	// encoding/json can't do.
	sigsjson "sigs.k8s.io/json"
)

// evictionSignals are the kubelet eviction signals that may key an eviction map. The kubelet
// ignores any other key, so accepting one would silently drop the threshold the user set.
var evictionSignals = []string{
	"memory.available",
	"nodefs.available",
	"nodefs.inodesFree",
	"imagefs.available",
	"imagefs.inodesFree",
	"pid.available",
}

// reservedResources are the resource names the kubelet can reserve. As with eviction signals,
// anything else is ignored rather than rejected by the kubelet itself.
var reservedResources = []string{"cpu", "memory", "ephemeral-storage", "pid"}

// karpenterOwnedFields are the kubelet config fields Karpenter sets itself, mapped to the reason it
// has to. Bootstrap overwrites these unconditionally, so a user value would be discarded on the way
// to the node with no error anywhere -- rejecting it here is what makes that visible. Fields
// Karpenter only defaults, like clusterDNS and maxPods, aren't listed: those keep the value the user
// set, so there's nothing to warn about.
var karpenterOwnedFields = map[string]string{
	"registerWithTaints": "Karpenter sets it from the NodeClaim's taints so the node registers unschedulable until it's ready",
}

// validateEvictionThreshold checks that an eviction threshold value is one of the two forms the
// kubelet accepts: a percentage of the relevant resource ("5%") or a non-negative resource.Quantity
// ("500Mi"). Constraining these to a Quantity alone would reject the percentage form the kubelet
// honors. A "%" suffix selects the percentage branch; everything else is parsed as a Quantity.
func validateEvictionThreshold(field, key, value string) error {
	if pct, ok := strings.CutSuffix(value, "%"); ok {
		// The kubelet reads a percentage as a fraction of the resource and rejects anything outside
		// [0,100]. resource.ParseQuantity would accept the bare number too, so it's the range check
		// that gives the percentage form meaning rather than making it a synonym for a quantity.
		if v, err := strconv.ParseFloat(pct, 64); err != nil || v < 0 || v > 100 {
			return fmt.Errorf("spec.kubelet.%s[%s]: %q must be a percentage or a resource quantity", field, key, value)
		}
		return nil
	}
	q, err := resource.ParseQuantity(value)
	if err != nil {
		return fmt.Errorf("spec.kubelet.%s[%s]: %q must be a percentage or a resource quantity", field, key, value)
	}
	if q.Sign() < 0 {
		return fmt.Errorf("spec.kubelet.%s[%s]: %q can't be a negative resource quantity", field, key, value)
	}
	return nil
}

// validateEvictionGracePeriod checks that an evictionSoftGracePeriod value is a non-negative Go
// duration ("30s", "2m"). The upstream type is a plain map[string]string with no duration
// constraint, and the kubelet parses these with time.ParseDuration at runtime, so a malformed
// value would otherwise pass validation here and only fail once it reached a node.
func validateEvictionGracePeriod(field, key, value string) error {
	d, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("spec.kubelet.%s[%s]: %q must be a duration", field, key, value)
	}
	if d < 0 {
		return fmt.Errorf("spec.kubelet.%s[%s]: %q can't be a negative duration", field, key, value)
	}
	return nil
}

// ValidateKubeletConfig validates spec.kubelet against the upstream kubelet configuration type
// that Karpenter compiles against, plus the semantic rules the upstream Go types can't express.
//
// spec.kubelet is an open map the API server does no validation of, so every rule has to live
// here. Errors surface on the EC2NodeClass as ValidationSucceeded=False, which blocks node
// launch, so invalid configuration never reaches a node. Bumping k8s.io/kubelet in go.mod is
// what makes newly released kubelet fields available.
func ValidateKubeletConfig(kc KubeletConfiguration) []error {
	if len(kc) == 0 {
		return nil
	}
	var errs []error
	errs = append(errs, validateAgainstUpstreamType(kc)...)
	errs = append(errs, validateKubeletSemantics(kc)...)
	return errs
}

// validateAgainstUpstreamType decodes the config against the upstream struct, which is what
// catches unknown fields and wrong types without Karpenter maintaining a field list of its own.
func validateAgainstUpstreamType(kc KubeletConfiguration) []error {
	// Fields that may hold a CEL expression are checked with a placeholder standing in for the
	// expression. Upstream types maxPods as int32, so an expression string would fail to decode
	// and mask every other field in the same document. Only the shape matters here; the
	// expression itself is validated where it's evaluated.
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
		// A type error fails the whole decode, so this is reported alone rather than alongside
		// the per-field errors below.
		return []error{fmt.Errorf("spec.kubelet: %w", err)}
	}
	errs := make([]error, 0, len(strictErrs))
	for _, strictErr := range strictErrs {
		errs = append(errs, fmt.Errorf("spec.kubelet: %w", strictErr))
	}
	return errs
}

// validateKubeletSemantics enforces the constraints that aren't expressible in the upstream Go
// types: which map keys are meaningful, what ranges values may take, and how fields relate.
//
//nolint:gocyclo
func validateKubeletSemantics(kc KubeletConfiguration) []error {
	var errs []error

	// A field Karpenter owns is rejected rather than accepted and then overwritten at bootstrap,
	// which would drop the user's value with no error anywhere. See karpenterOwnedFields.
	for _, field := range slices.Sorted(maps.Keys(karpenterOwnedFields)) {
		if _, ok := kc[field]; ok {
			errs = append(errs, fmt.Errorf("spec.kubelet.%s: can't be set, %s", field, karpenterOwnedFields[field]))
		}
	}

	// Eviction maps are keyed by eviction signal. All four are map[string]string upstream with no
	// value constraint, so their values are checked here: evictionHard, evictionSoft, and
	// evictionMinimumReclaim hold a percentage or a quantity, while evictionSoftGracePeriod holds a
	// duration.
	for _, field := range []string{"evictionHard", "evictionSoft", "evictionSoftGracePeriod", "evictionMinimumReclaim"} {
		signals, ok := decodeStringMap(kc, field)
		if !ok {
			continue
		}
		for _, key := range slices.Sorted(maps.Keys(signals)) {
			if !slices.Contains(evictionSignals, key) {
				errs = append(errs, fmt.Errorf("spec.kubelet.%s: %q isn't a valid eviction signal, must be one of %s",
					field, key, strings.Join(evictionSignals, ", ")))
			}
			if field == "evictionSoftGracePeriod" {
				if err := validateEvictionGracePeriod(field, key, signals[key]); err != nil {
					errs = append(errs, err)
				}
				continue
			}
			if err := validateEvictionThreshold(field, key, signals[key]); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// A soft eviction threshold with no grace period is ignored by the kubelet, and a grace
	// period for a signal it isn't evicting on is rejected, so the two maps must have matching
	// keys. An absent map counts as empty rather than as an excuse to skip the check: setting
	// only one of the pair is exactly the mistake this catches.
	soft, _ := decodeStringMap(kc, "evictionSoft")
	grace, _ := decodeStringMap(kc, "evictionSoftGracePeriod")
	for _, signal := range slices.Sorted(maps.Keys(soft)) {
		if _, ok := grace[signal]; !ok {
			errs = append(errs, fmt.Errorf("spec.kubelet.evictionSoft[%s]: has no matching evictionSoftGracePeriod", signal))
		}
	}
	for _, signal := range slices.Sorted(maps.Keys(grace)) {
		if _, ok := soft[signal]; !ok {
			errs = append(errs, fmt.Errorf("spec.kubelet.evictionSoftGracePeriod[%s]: has no matching evictionSoft", signal))
		}
	}

	// kubeReserved and systemReserved accept only the resources the kubelet reserves, and a
	// reservation can't be negative. Values aren't required to parse as a quantity: one may be a
	// CEL expression evaluated per instance type (e.g. "vcpus * 10"), resolved before it reaches
	// the kubelet.
	for _, field := range []string{"kubeReserved", "systemReserved"} {
		reserved, ok := decodeStringMap(kc, field)
		if !ok {
			continue
		}
		for _, key := range slices.Sorted(maps.Keys(reserved)) {
			if !slices.Contains(reservedResources, key) {
				errs = append(errs, fmt.Errorf("spec.kubelet.%s: %q isn't a reservable resource, must be one of %s",
					field, key, strings.Join(reservedResources, ", ")))
			}
			if strings.HasPrefix(reserved[key], "-") {
				errs = append(errs, fmt.Errorf("spec.kubelet.%s[%s]: %q can't be a negative resource quantity",
					field, key, reserved[key]))
			}
		}
	}

	// Percentages are 0-100, and counts and grace periods can't be negative. maxPods is excluded
	// because it may hold a CEL expression; its integer form is checked below.
	for _, field := range []string{"imageGCHighThresholdPercent", "imageGCLowThresholdPercent"} {
		if value, ok := decodeInt(kc, field); ok && (value < 0 || value > 100) {
			errs = append(errs, fmt.Errorf("spec.kubelet.%s: %d must be between 0 and 100", field, value))
		}
	}
	for _, field := range []string{"podsPerCore", "evictionMaxPodGracePeriod"} {
		if value, ok := decodeInt(kc, field); ok && value < 0 {
			errs = append(errs, fmt.Errorf("spec.kubelet.%s: %d can't be negative", field, value))
		}
	}
	// An expression is bounds-checked once it's been evaluated against an instance type, so only
	// the literal integer form is constrained here.
	if maxPods, ok := kc["maxPods"]; ok && !isJSONString(maxPods) {
		if value, ok := decodeInt(kc, "maxPods"); ok && value < 0 {
			errs = append(errs, fmt.Errorf("spec.kubelet.maxPods: %d can't be negative", value))
		}
	}

	// The kubelet always runs image garbage collection above the high threshold and never below
	// the low one, so an inverted pair would never collect.
	high, highOK := decodeInt(kc, "imageGCHighThresholdPercent")
	low, lowOK := decodeInt(kc, "imageGCLowThresholdPercent")
	if highOK && lowOK && high <= low {
		errs = append(errs, fmt.Errorf("spec.kubelet: imageGCHighThresholdPercent (%d) must be greater than imageGCLowThresholdPercent (%d)", high, low))
	}

	return errs
}

// decodeStringMap reads a map[string]string field, reporting false if it's absent or isn't one.
// A value of the wrong type is treated as absent rather than reported, since
// validateAgainstUpstreamType already describes the mismatch against the upstream type.
func decodeStringMap(kc KubeletConfiguration, field string) (map[string]string, bool) {
	raw, ok := kc[field]
	if !ok {
		return nil, false
	}
	var out map[string]string
	if err := json.Unmarshal(raw.Raw, &out); err != nil {
		return nil, false
	}
	return out, true
}

// decodeInt reads an integer field, reporting false if it's absent or isn't an integer. As above,
// a wrong type is left to validateAgainstUpstreamType to report.
func decodeInt(kc KubeletConfiguration, field string) (int64, bool) {
	raw, ok := kc[field]
	if !ok {
		return 0, false
	}
	var out int64
	if err := json.Unmarshal(raw.Raw, &out); err != nil {
		return 0, false
	}
	return out, true
}

// isJSONString reports whether a raw JSON value is a string.
func isJSONString(v apiextensionsv1.JSON) bool {
	var s string
	return json.Unmarshal(v.Raw, &s) == nil
}
