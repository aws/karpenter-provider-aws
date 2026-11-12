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

package scheduling

import (
	"fmt"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

// Selectors is the shared table used by the scheduling label specs and their
// cluster-free coverage check. Each entry uses the In operator with one value;
// the e2e specs exercise it as a node selector, requirement, and preference.
// Return fresh maps so individual specs cannot mutate another spec's selectors.
func Selectors(nodePoolName, region string) map[string]map[string]string {
	return map[string]map[string]string{
		"instance-type": {
			// Well Known
			karpv1.NodePoolLabelKey:        nodePoolName,
			corev1.LabelInstanceTypeStable: "c5.large",
			// Well Known to AWS
			v1.LabelInstanceHypervisor:                "nitro",
			v1.LabelInstanceCategory:                  "c",
			v1.LabelInstanceGeneration:                "5",
			v1.LabelInstanceFamily:                    "c5",
			v1.LabelInstanceSize:                      "large",
			v1.LabelInstanceCPU:                       "2",
			v1.LabelInstanceCPUManufacturer:           "intel",
			v1.LabelInstanceCPUSustainedClockSpeedMhz: "3400",
			v1.LabelInstanceMemory:                    "4096",
			v1.LabelInstanceEBSBandwidth:              "4750",
			v1.LabelInstanceNetworkBandwidth:          "750",
		},
		"deprecated": {
			// Deprecated Labels
			corev1.LabelFailureDomainBetaRegion: region,
			corev1.LabelFailureDomainBetaZone:   fmt.Sprintf("%sa", region),
			"topology.ebs.csi.aws.com/zone":     fmt.Sprintf("%sa", region),

			"beta.kubernetes.io/arch": "amd64",
			"beta.kubernetes.io/os":   "linux",
			corev1.LabelInstanceType:  "c5.large",
		},
		"topology": {
			// Well Known
			karpv1.NodePoolLabelKey:     nodePoolName,
			corev1.LabelTopologyRegion:  region,
			corev1.LabelTopologyZone:    fmt.Sprintf("%sa", region),
			corev1.LabelOSStable:        "linux",
			corev1.LabelArchStable:      "amd64",
			karpv1.CapacityTypeLabelKey: karpv1.CapacityTypeOnDemand,
		},
		"nvidia": {
			v1.LabelInstanceGPUName:         "t4",
			v1.LabelInstanceGPUMemory:       "16384",
			v1.LabelInstanceGPUManufacturer: "nvidia",
			v1.LabelInstanceGPUCount:        "1",
			v1.LabelNvidiaGPUPresent:        "true",
		},
		"inferentia": {
			v1.LabelInstanceAcceleratorName:         "inferentia",
			v1.LabelInstanceAcceleratorManufacturer: "aws",
			v1.LabelInstanceAcceleratorCount:        "1",
		},
		"windows": {
			// Well Known
			corev1.LabelWindowsBuild: v1.Windows2022Build,
			corev1.LabelOSStable:     string(corev1.Windows), // Specify the OS to enable vpc-resource-controller to inject the PrivateIPv4Address resource
		},
		"tenancy": {
			v1.LabelInstanceTenancy: string(ec2types.CapacityReservationTenancyDedicated),
		},
	}
}

// Requirements contains selectors that need a comparison operator or a
// discovered zone ID rather than the equality selectors above.
func Requirements(zoneID string) map[string][]corev1.NodeSelectorRequirement {
	return map[string][]corev1.NodeSelectorRequirement{
		"zone-id": {
			{
				Key:      v1.LabelTopologyZoneID,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{zoneID},
			},
		},
		"local-nvme": {
			{
				Key:      v1.LabelInstanceLocalNVME,
				Operator: corev1.NodeSelectorOpGt,
				Values:   []string{"0"},
			},
		},
		"encryption-in-transit": {
			{
				Key:      v1.LabelInstanceEncryptionInTransitSupported,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"true"},
			},
		},
		"flex": {{Key: v1.LabelInstanceCapabilityFlex, Operator: corev1.NodeSelectorOpIn, Values: []string{"true"}}},
	}
}
