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

package v1beta1

import (
	"fmt"
	"regexp"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"sigs.k8s.io/karpenter/pkg/apis/v1beta1"
)

func init() {
	v1beta1.RestrictedLabelDomains = v1beta1.RestrictedLabelDomains.Insert(RestrictedLabelDomains...)
	v1beta1.WellKnownLabels = v1beta1.WellKnownLabels.Insert(
		LabelInstanceHypervisor,
		LabelInstanceEncryptionInTransitSupported,
		LabelInstanceCategory,
		LabelInstanceFamily,
		LabelInstanceGeneration,
		LabelInstanceSize,
		LabelInstanceLocalNVME,
		LabelInstanceCPU,
		LabelInstanceCPUManufacturer,
		LabelInstanceMemory,
		LabelInstanceNetworkBandwidth,
		LabelInstanceGPUName,
		LabelInstanceGPUManufacturer,
		LabelInstanceGPUCount,
		LabelInstanceGPUMemory,
		LabelInstanceAcceleratorName,
		LabelInstanceAcceleratorManufacturer,
		LabelInstanceAcceleratorCount,
		v1.LabelWindowsBuild,
	)
}

const (
	TerminationFinalizer = Group + "/termination"
)

var (
	AWSToKubeArchitectures = map[string]string{
		"x86_64":                  v1beta1.ArchitectureAmd64,
		v1beta1.ArchitectureArm64: v1beta1.ArchitectureArm64,
	}
	WellKnownArchitectures = sets.NewString(
		v1beta1.ArchitectureAmd64,
		v1beta1.ArchitectureArm64,
	)
	RestrictedLabelDomains = []string{
		Group,
	}
	RestrictedTagPatterns = []*regexp.Regexp{
		// Adheres to cluster name pattern matching as specified in the API spec
		// https://docs.aws.amazon.com/eks/latest/APIReference/API_CreateCluster.html
		regexp.MustCompile(`^kubernetes\.io/cluster/[0-9A-Za-z][A-Za-z0-9\-_]*$`),
		regexp.MustCompile(fmt.Sprintf("^%s$", regexp.QuoteMeta(v1beta1.NodePoolLabelKey))),
		regexp.MustCompile(fmt.Sprintf("^%s$", regexp.QuoteMeta(v1beta1.ManagedByAnnotationKey))),
		regexp.MustCompile(fmt.Sprintf("^%s$", regexp.QuoteMeta(LabelNodeClass))),
		regexp.MustCompile(fmt.Sprintf("^%s$", regexp.QuoteMeta(TagNodeClaim))),
	}
	AMIFamilyBottlerocket                      = "Bottlerocket"
	AMIFamilyAL2                               = "AL2"
	AMIFamilyAL2023                            = "AL2023"
	AMIFamilyUbuntu                            = "Ubuntu"
	AMIFamilyWindows2019                       = "Windows2019"
	AMIFamilyWindows2022                       = "Windows2022"
	AMIFamilyCustom                            = "Custom"
	Windows2019                                = "2019"
	Windows2022                                = "2022"
	WindowsCore                                = "Core"
	Windows2019Build                           = "10.0.17763"
	Windows2022Build                           = "10.0.20348"
	ResourceNVIDIAGPU          v1.ResourceName = "nvidia.com/gpu"
	ResourceAMDGPU             v1.ResourceName = "amd.com/gpu"
	ResourceAWSNeuron          v1.ResourceName = "aws.amazon.com/neuron"
	ResourceHabanaGaudi        v1.ResourceName = "habana.ai/gaudi"
	ResourceAWSPodENI          v1.ResourceName = "vpc.amazonaws.com/pod-eni"
	ResourcePrivateIPv4Address v1.ResourceName = "vpc.amazonaws.com/PrivateIPv4Address"
	ResourceEFA                v1.ResourceName = "vpc.amazonaws.com/efa"

	LabelNodeClass = Group + "/ec2nodeclass"

	LabelInstanceHypervisor                   = Group + "/instance-hypervisor"
	LabelInstanceEncryptionInTransitSupported = Group + "/instance-encryption-in-transit-supported"
	LabelInstanceCategory                     = Group + "/instance-category"
	LabelInstanceFamily                       = Group + "/instance-family"
	LabelInstanceGeneration                   = Group + "/instance-generation"
	LabelInstanceLocalNVME                    = Group + "/instance-local-nvme"
	LabelInstanceSize                         = Group + "/instance-size"
	LabelInstanceCPU                          = Group + "/instance-cpu"
	LabelInstanceCPUManufacturer              = Group + "/instance-cpu-manufacturer"
	LabelInstanceMemory                       = Group + "/instance-memory"
	LabelInstanceNetworkBandwidth             = Group + "/instance-network-bandwidth"
	LabelInstanceGPUName                      = Group + "/instance-gpu-name"
	LabelInstanceGPUManufacturer              = Group + "/instance-gpu-manufacturer"
	LabelInstanceGPUCount                     = Group + "/instance-gpu-count"
	LabelInstanceGPUMemory                    = Group + "/instance-gpu-memory"
	LabelInstanceAcceleratorName              = Group + "/instance-accelerator-name"
	LabelInstanceAcceleratorManufacturer      = Group + "/instance-accelerator-manufacturer"
	LabelInstanceAcceleratorCount             = Group + "/instance-accelerator-count"
	AnnotationEC2NodeClassHash                = Group + "/ec2nodeclass-hash"
	AnnotationEC2NodeClassHashVersion         = Group + "/ec2nodeclass-hash-version"
	AnnotationInstanceTagged                  = Group + "/tagged"

	TagNodeClaim = v1beta1.Group + "/nodeclaim"
	TagName      = "Name"
)
