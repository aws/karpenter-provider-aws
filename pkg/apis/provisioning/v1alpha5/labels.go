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

package v1alpha5

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	ArchitectureAmd64    = "amd64"
	ArchitectureArm64    = "arm64"
	OperatingSystemLinux = "linux"

	// Karpenter specific domains and labels
	KarpenterLabelDomain = "karpenter.sh"

	ProvisionerNameLabelKey           = Group + "/provisioner-name"
	DoNotEvictPodAnnotationKey        = Group + "/do-not-evict"
	DoNotConsolidateNodeAnnotationKey = KarpenterLabelDomain + "/do-not-consolidate"
	EmptinessTimestampAnnotationKey   = Group + "/emptiness-timestamp"
	TerminationFinalizer              = Group + "/termination"

	LabelCapacityType    = KarpenterLabelDomain + "/capacity-type"
	LabelNodeInitialized = KarpenterLabelDomain + "/initialized"

	// RestrictedLabelDomains are either prohibited by the kubelet or reserved by karpenter
	RestrictedLabelDomains = sets.NewString(
		"kubernetes.io",
		"k8s.io",
		KarpenterLabelDomain,
	)

	// LabelDomainException are sub-domains of the RestrictedLabelDomains but allowed because
	// they are not used in a context where they may be passed as argument to kubelet.
	LabelDomainExceptions = sets.NewString(
		"kops.k8s.io",
		v1.LabelNamespaceSuffixNode,
	)

	// WellKnownLabels are labels that belong to the RestrictedLabelDomains but allowed.
	// Karpenter is aware of these labels, and they can be used to further narrow down
	// the range of the corresponding values by either provisioner or pods.
	WellKnownLabels = sets.NewString(
		ProvisionerNameLabelKey,
		v1.LabelTopologyZone,
		v1.LabelTopologyRegion,
		v1.LabelInstanceTypeStable,
		v1.LabelArchStable,
		v1.LabelOSStable,
		LabelCapacityType,
	)

	// RestrictedLabels are labels that should not be used
	// because they may interfere with the internal provisioning logic.
	RestrictedLabels = sets.NewString(
		EmptinessTimestampAnnotationKey,
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
		return fmt.Errorf("label %s is restricted; specify a well known label: %v, or a custom label that does not use a restricted domain: %v", key, WellKnownLabels.List(), RestrictedLabelDomains.List())
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
	labelDomain := getLabelDomain(key)
	if LabelDomainExceptions.Has(labelDomain) {
		return false
	}
	for restrictedLabelDomain := range RestrictedLabelDomains {
		if strings.HasSuffix(labelDomain, restrictedLabelDomain) {
			return true
		}
	}
	return RestrictedLabels.Has(key)
}

func getLabelDomain(key string) string {
	if parts := strings.SplitN(key, "/", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}
