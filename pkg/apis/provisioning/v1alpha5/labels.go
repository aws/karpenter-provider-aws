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
	stringsets "k8s.io/apimachinery/pkg/util/sets"
)

var (
	ArchitectureAmd64    = "amd64"
	ArchitectureArm64    = "arm64"
	OperatingSystemLinux = "linux"

	// ValidTopologyKeys are the topology keys that Karpenter allows for topology spread and pod affinity/anti-affinity
	ValidTopologyKeys = stringsets.NewString(v1.LabelHostname, v1.LabelTopologyZone, LabelCapacityType)

	// Karpenter specific domains and labels
	KarpenterLabelDomain = "karpenter.sh"
	LabelCapacityType    = KarpenterLabelDomain + "/capacity-type"

	// AnnotationExtendedResources is used to record the expected extended resources on a node that will be created when
	// device plugins have finished initializing
	AnnotationExtendedResources = KarpenterLabelDomain + "/extended-resources"

	// RestrictedLabelDomains are either prohibited by the kubelet or reserved by karpenter
	RestrictedLabelDomains = stringsets.NewString(
		"kubernetes.io",
		"k8s.io",
		KarpenterLabelDomain,
	)

	// LabelDomainException are sub-domains of the RestrictedLabelDomains but allowed because
	// they are not used in a context where they may be passed as argument to kubelet.
	LabelDomainExceptions = stringsets.NewString(
		"kops.k8s.io",
	)

	// WellKnownLabels are labels that belong to the RestrictedLabelDomains but allowed.
	// Karpenter is aware of these labels, and they can be used to further narrow down
	// the range of the corresponding values by either provisioner or pods.
	WellKnownLabels = stringsets.NewString(
		v1.LabelTopologyZone,
		v1.LabelInstanceTypeStable,
		v1.LabelArchStable,
		v1.LabelOSStable,
		LabelCapacityType,
	)

	// RestrictedLabels are labels that should not be used
	// because they may interfer the internall provisioning logic.
	RestrictedLabels = stringsets.NewString(
		// Used internally by provisioning logic
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
	// IgnoredLables are not considered in scheduling decisions
	// and prevent validation errors when specified
	IgnoredLabels = stringsets.NewString(
		v1.LabelTopologyRegion,
	)
)

// IsRestrictedLabel returns an error if the label is restricted.
func IsRestrictedLabel(key string) error {
	if WellKnownLabels.Has(key) {
		return nil
	}
	if RestrictedLabels.Has(key) {
		return fmt.Errorf("label is restricted, %s", key)
	}
	labelDomain := getLabelDomain(key)
	if LabelDomainExceptions.Has(labelDomain) {
		return nil
	}
	for restrictedLabelDomain := range RestrictedLabelDomains {
		if strings.HasSuffix(labelDomain, restrictedLabelDomain) {
			return fmt.Errorf("label domain not allowed, %s", getLabelDomain(key))
		}
	}
	return nil
}

// IsRestrictedNodeLabel returns true if a node label should not be injected by Karpenter.
func IsRestrictedNodeLabel(key string) bool {
	return RestrictedLabels.Has(key)
}

func getLabelDomain(key string) string {
	if parts := strings.SplitN(key, "/", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}
