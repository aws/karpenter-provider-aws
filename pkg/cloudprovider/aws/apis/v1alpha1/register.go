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

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/samber/lo"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

var (
	LabelDomain = "karpenter.k8s.aws"

	CapacityTypeSpot       = ec2.DefaultTargetCapacityTypeSpot
	CapacityTypeOnDemand   = ec2.DefaultTargetCapacityTypeOnDemand
	AWSToKubeArchitectures = map[string]string{
		"x86_64":                   v1alpha5.ArchitectureAmd64,
		v1alpha5.ArchitectureArm64: v1alpha5.ArchitectureArm64,
	}
	RestrictedLabelDomains = []string{
		LabelDomain,
	}
	AMIFamilyBottlerocket = "Bottlerocket"
	AMIFamilyAL2          = "AL2"
	AMIFamilyUbuntu       = "Ubuntu"
	AMIFamilyCustom       = "Custom"
	SupportedAMIFamilies  = []string{
		AMIFamilyBottlerocket,
		AMIFamilyAL2,
		AMIFamilyUbuntu,
		AMIFamilyCustom,
	}
	SupportedContainerRuntimesByAMIFamily = map[string]sets.String{
		AMIFamilyBottlerocket: sets.NewString("containerd"),
		AMIFamilyAL2:          sets.NewString("dockerd", "containerd"),
		AMIFamilyUbuntu:       sets.NewString("dockerd", "containerd"),
	}
	ResourceNVIDIAGPU v1.ResourceName = "nvidia.com/gpu"
	ResourceAMDGPU    v1.ResourceName = "amd.com/gpu"
	ResourceAWSNeuron v1.ResourceName = "aws.amazon.com/neuron"
	ResourceAWSPodENI v1.ResourceName = "vpc.amazonaws.com/pod-eni"

	LabelInstanceHypervisor      = LabelDomain + "/instance-hypervisor"
	LabelInstanceFamily          = LabelDomain + "/instance-family"
	LabelInstanceSize            = LabelDomain + "/instance-size"
	LabelInstanceCPU             = LabelDomain + "/instance-cpu"
	LabelInstanceMemory          = LabelDomain + "/instance-memory"
	LabelInstancePods            = LabelDomain + "/instance-pods"
	LabelInstanceGPUName         = LabelDomain + "/instance-gpu-name"
	LabelInstanceGPUManufacturer = LabelDomain + "/instance-gpu-manufacturer"
	LabelInstanceGPUCount        = LabelDomain + "/instance-gpu-count"
	LabelInstanceGPUMemory       = LabelDomain + "/instance-gpu-memory"

	// Deprecated labels. Use the - notation above instead. Very sad.
	LabelInstanceFamilyDeprecated          = LabelDomain + "/instance.family"
	LabelInstanceSizeDeprecated            = LabelDomain + "/instance.size"
	LabelInstanceCPUDeprecated             = LabelDomain + "/instance.cpu"
	LabelInstanceMemoryDeprecated          = LabelDomain + "/instance.memory"
	LabelInstanceGPUNameDeprecated         = LabelDomain + "/instance.gpu.name"
	LabelInstanceGPUManufacturerDeprecated = LabelDomain + "/instance.gpu.manufacturer"
	LabelInstanceGPUCountDeprecated        = LabelDomain + "/instance.gpu.count"
	LabelInstanceGPUMemoryDeprecated       = LabelDomain + "/instance.gpu.memory"
)

var (
	Scheme = runtime.NewScheme()
	Codec  = serializer.NewCodecFactory(Scheme, serializer.EnableStrict)
)

func init() {
	Scheme.AddKnownTypes(schema.GroupVersion{Group: v1alpha5.ExtensionsGroup, Version: "v1alpha1"}, &AWS{})
	v1alpha5.RestrictedLabelDomains = v1alpha5.RestrictedLabelDomains.Insert(RestrictedLabelDomains...)
	v1alpha5.WellKnownLabels = v1alpha5.WellKnownLabels.Insert(
		LabelInstanceHypervisor,
		LabelInstanceFamily,
		LabelInstanceSize,
		LabelInstanceCPU,
		LabelInstanceMemory,
		LabelInstancePods,
		LabelInstanceGPUName,
		LabelInstanceGPUManufacturer,
		LabelInstanceGPUCount,
		LabelInstanceGPUMemory,
	)
	v1alpha5.NormalizedLabels = lo.Assign(v1alpha5.NormalizedLabels, map[string]string{
		LabelInstanceFamilyDeprecated:          LabelInstanceFamily,
		LabelInstanceSizeDeprecated:            LabelInstanceSize,
		LabelInstanceCPUDeprecated:             LabelInstanceCPU,
		LabelInstanceMemoryDeprecated:          LabelInstanceMemory,
		LabelInstanceGPUNameDeprecated:         LabelInstanceGPUName,
		LabelInstanceGPUManufacturerDeprecated: LabelInstanceGPUManufacturer,
		LabelInstanceGPUCountDeprecated:        LabelInstanceGPUCount,
		LabelInstanceGPUMemoryDeprecated:       LabelInstanceGPUMemory,
	})
}
