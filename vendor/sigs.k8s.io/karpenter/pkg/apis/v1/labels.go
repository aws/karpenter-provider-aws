/*
Copyright The Kubernetes Authors.

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
	"fmt"
	"strings"

	"github.com/awslabs/operatorpkg/serrors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	"sigs.k8s.io/karpenter/pkg/apis"
)

// Well known labels and resources
const (
	ArchitectureAmd64    = "amd64"
	ArchitectureArm64    = "arm64"
	CapacityTypeSpot     = "spot"
	CapacityTypeOnDemand = "on-demand"
	CapacityTypeReserved = "reserved"
)

// Karpenter specific domains and labels
const (
	NodePoolLabelKey            = apis.Group + "/nodepool"
	NodeInitializedLabelKey     = apis.Group + "/initialized"
	NodeRegisteredLabelKey      = apis.Group + "/registered"
	NodeDoNotSyncTaintsLabelKey = apis.Group + "/do-not-sync-taints"
	CapacityTypeLabelKey        = apis.Group + "/capacity-type"
)

// Karpenter specific annotations
const (
	DoNotDisruptAnnotationKey                  = apis.Group + "/do-not-disrupt"
	ProviderCompatibilityAnnotationKey         = apis.CompatibilityGroup + "/provider"
	NodePoolHashAnnotationKey                  = apis.Group + "/nodepool-hash"
	NodePoolHashVersionAnnotationKey           = apis.Group + "/nodepool-hash-version"
	NodeClaimTerminationTimestampAnnotationKey = apis.Group + "/nodeclaim-termination-timestamp"
	NodeClaimMinValuesRelaxedAnnotationKey     = apis.Group + "/nodeclaim-min-values-relaxed"
)

// Karpenter specific finalizers
const (
	TerminationFinalizer = apis.Group + "/termination"
)

var (
	// RestrictedLabelDomains are either prohibited by the kubelet or reserved by karpenter
	RestrictedLabelDomains = sets.New(
		"kubernetes.io",
		"k8s.io",
		apis.Group,
	)

	// LabelDomainExceptions are sub-domains of the RestrictedLabelDomains but allowed because
	// they are not used in a context where they may be passed as argument to kubelet.
	LabelDomainExceptions = sets.New(
		"kops.k8s.io",
		v1.LabelNamespaceSuffixNode,
		v1.LabelNamespaceNodeRestriction,
	)

	// WellKnownLabels are labels that belong to the RestrictedLabelDomains but allowed.
	// Karpenter is aware of these labels, and they can be used to further narrow down
	// the range of the corresponding values by either nodepool or pods.
	WellKnownLabels = sets.New(
		NodePoolLabelKey,
		v1.LabelTopologyZone,
		v1.LabelTopologyRegion,
		v1.LabelInstanceTypeStable,
		v1.LabelArchStable,
		v1.LabelOSStable,
		CapacityTypeLabelKey,
		v1.LabelWindowsBuild,
	)

	// WellKnownValuesForRequirements are for requirements where a known set of values
	// is expected to be used for that requirement. For example, in the AWS provider,
	// only on-demand, spot, and reserved make sense as values for the capacity type requirement
	WellKnownValuesForRequirements = map[string]sets.Set[string]{
		CapacityTypeLabelKey: sets.New(
			CapacityTypeOnDemand,
			CapacityTypeSpot,
			CapacityTypeReserved,
		),
	}

	// RestrictedLabels are labels that should not be used
	// because they may interfere with the internal provisioning logic.
	RestrictedLabels = sets.New(
		v1.LabelHostname,
	)

	// NormalizedLabels translate aliased concepts into the controller's
	// WellKnownLabels. Pod requirements are translated for compatibility.
	NormalizedLabels = map[string]string{
		v1.LabelFailureDomainBetaZone:   v1.LabelTopologyZone,
		"beta.kubernetes.io/arch":       v1.LabelArchStable,
		"beta.kubernetes.io/os":         v1.LabelOSStable,
		v1.LabelInstanceType:            v1.LabelInstanceTypeStable,
		v1.LabelFailureDomainBetaRegion: v1.LabelTopologyRegion,
	}
)

// IsRestrictedLabel returns an error if the label is restricted.
func IsRestrictedLabel(key string) error {
	if WellKnownLabels.Has(key) {
		return nil
	}
	if IsRestrictedNodeLabel(key) {
		return serrors.Wrap(fmt.Errorf("label is restricted; specify a well known label or a custom label that does not use a restricted domain"), "label", key, "well-known-labels", sets.List(WellKnownLabels), "restricted-labels", sets.List(RestrictedLabelDomains))
	}
	return nil
}

// HasKnownValues returns an error if the requirement has well known values and is only presented with unknown values.
func HasKnownValues(requirement NodeSelectorRequirementWithMinValues) error {
	if !WellKnownLabels.Has(requirement.Key) {
		return nil
	}
	if !WellKnownValuesForRequirements[requirement.Key].HasAny(requirement.Values...) {
		return fmt.Errorf("invalid values: %v for key: %s, expected one of: %v", requirement.Values, requirement.Key, WellKnownValuesForRequirements[requirement.Key].UnsortedList())
	}
	return nil
}

// IsRestrictedNodeLabel returns true if a node label should not be injected by Karpenter.
// They are either known labels that will be injected by cloud providers,
// or label domain managed by other software (e.g., kops.k8s.io managed by kOps).
func IsRestrictedNodeLabel(key string) bool {
	if WellKnownLabels.Has(key) {
		return true
	}
	labelDomain := GetLabelDomain(key)
	for exceptionLabelDomain := range LabelDomainExceptions {
		if strings.HasSuffix(labelDomain, exceptionLabelDomain) {
			return false
		}
	}
	for restrictedLabelDomain := range RestrictedLabelDomains {
		if strings.HasSuffix(labelDomain, restrictedLabelDomain) {
			return true
		}
	}
	return RestrictedLabels.Has(key)
}

func GetLabelDomain(key string) string {
	if parts := strings.SplitN(key, "/", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

func NodeClassLabelKey(gk schema.GroupKind) string {
	return fmt.Sprintf("%s/%s", gk.Group, strings.ToLower(gk.Kind))
}
