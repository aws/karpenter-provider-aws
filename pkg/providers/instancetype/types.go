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

package instancetype

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/ptr"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/operator/options"
	"github.com/aws/karpenter/pkg/providers/amifamily"

	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/scheduling"
	"github.com/aws/karpenter-core/pkg/utils/resources"
)

const (
	MemoryAvailable = "memory.available"
	NodeFSAvailable = "nodefs.available"
)

var (
	instanceTypeScheme = regexp.MustCompile(`(^[a-z]+)(\-[0-9]+tb)?([0-9]+).*\.`)
)

func NewInstanceType(ctx context.Context, info *ec2.InstanceTypeInfo, kc *corev1beta1.KubeletConfiguration,
	region string, nodeClass *v1beta1.EC2NodeClass, offerings cloudprovider.Offerings) *cloudprovider.InstanceType {

	amiFamily := amifamily.GetAMIFamily(nodeClass.Spec.AMIFamily, &amifamily.Options{})
	it := &cloudprovider.InstanceType{
		Name:         aws.StringValue(info.InstanceType),
		Requirements: computeRequirements(ctx, info, offerings, region, amiFamily, kc, nodeClass),
		Offerings:    offerings,
		Capacity:     computeCapacity(ctx, info, amiFamily, nodeClass.Spec.BlockDeviceMappings, kc),
		Overhead: &cloudprovider.InstanceTypeOverhead{
			KubeReserved:      kubeReservedResources(cpu(info), pods(ctx, info, amiFamily, kc), ENILimitedPods(ctx, info), amiFamily, kc),
			SystemReserved:    systemReservedResources(kc),
			EvictionThreshold: evictionThreshold(memory(ctx, info), ephemeralStorage(amiFamily, nodeClass.Spec.BlockDeviceMappings), amiFamily, kc),
		},
	}
	if it.Requirements.Compatible(scheduling.NewRequirements(scheduling.NewRequirement(v1.LabelOSStable, v1.NodeSelectorOpIn, string(v1.Windows)))) == nil {
		it.Capacity[v1beta1.ResourcePrivateIPv4Address] = *privateIPv4Address(info)
	}
	return it
}

//nolint:gocyclo
func computeRequirements(ctx context.Context, info *ec2.InstanceTypeInfo, offerings cloudprovider.Offerings, region string,
	amiFamily amifamily.AMIFamily, kc *corev1beta1.KubeletConfiguration, nodeClass *v1beta1.EC2NodeClass) scheduling.Requirements {
	requirements := scheduling.NewRequirements(
		// Well Known Upstream
		scheduling.NewRequirement(v1.LabelInstanceTypeStable, v1.NodeSelectorOpIn, aws.StringValue(info.InstanceType)),
		scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, getArchitecture(info)),
		scheduling.NewRequirement(v1.LabelOSStable, v1.NodeSelectorOpIn, getOS(info, amiFamily)...),
		scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, lo.Map(offerings.Available(), func(o cloudprovider.Offering, _ int) string { return o.Zone })...),
		scheduling.NewRequirement(v1.LabelTopologyRegion, v1.NodeSelectorOpIn, region),
		scheduling.NewRequirement(v1.LabelWindowsBuild, v1.NodeSelectorOpDoesNotExist),
		// Well Known to Karpenter
		scheduling.NewRequirement(corev1beta1.CapacityTypeLabelKey, v1.NodeSelectorOpIn, lo.Map(offerings.Available(), func(o cloudprovider.Offering, _ int) string { return o.CapacityType })...),
		// Well Known to AWS
		scheduling.NewRequirement(v1beta1.LabelInstanceCPU, v1.NodeSelectorOpIn, fmt.Sprint(aws.Int64Value(info.VCpuInfo.DefaultVCpus))),
		scheduling.NewRequirement(v1beta1.LabelInstanceMemory, v1.NodeSelectorOpIn, fmt.Sprint(aws.Int64Value(info.MemoryInfo.SizeInMiB))),
		scheduling.NewRequirement(v1beta1.LabelInstanceNetworkBandwidth, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1beta1.LabelInstanceCategory, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1beta1.LabelInstanceFamily, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1beta1.LabelInstanceGeneration, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1beta1.LabelInstanceLocalNVME, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1beta1.LabelInstanceSize, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1beta1.LabelInstanceGPUName, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1beta1.LabelInstanceGPUManufacturer, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1beta1.LabelInstanceGPUCount, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1beta1.LabelInstanceGPUMemory, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1beta1.LabelInstanceAcceleratorName, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1beta1.LabelInstanceAcceleratorManufacturer, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1beta1.LabelInstanceAcceleratorCount, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1beta1.LabelInstanceHypervisor, v1.NodeSelectorOpIn, aws.StringValue(info.Hypervisor)),
		scheduling.NewRequirement(v1beta1.LabelInstanceEncryptionInTransitSupported, v1.NodeSelectorOpIn, fmt.Sprint(aws.BoolValue(info.NetworkInfo.EncryptionInTransitSupported))),
	)
	if nodeClass.IsNodeTemplate {
		requirements.Add(scheduling.NewRequirement(v1alpha1.LabelInstancePods, v1.NodeSelectorOpIn, fmt.Sprint(pods(ctx, info, amiFamily, kc))))
	}
	// Instance Type Labels
	instanceFamilyParts := instanceTypeScheme.FindStringSubmatch(aws.StringValue(info.InstanceType))
	if len(instanceFamilyParts) == 4 {
		requirements[v1beta1.LabelInstanceCategory].Insert(instanceFamilyParts[1])
		requirements[v1beta1.LabelInstanceGeneration].Insert(instanceFamilyParts[3])
	}
	instanceTypeParts := strings.Split(aws.StringValue(info.InstanceType), ".")
	if len(instanceTypeParts) == 2 {
		requirements.Get(v1beta1.LabelInstanceFamily).Insert(instanceTypeParts[0])
		requirements.Get(v1beta1.LabelInstanceSize).Insert(instanceTypeParts[1])
	}
	if info.InstanceStorageInfo != nil && aws.StringValue(info.InstanceStorageInfo.NvmeSupport) != ec2.EphemeralNvmeSupportUnsupported {
		requirements[v1beta1.LabelInstanceLocalNVME].Insert(fmt.Sprint(aws.Int64Value(info.InstanceStorageInfo.TotalSizeInGB)))
	}
	// Network bandwidth
	if bandwidth, ok := InstanceTypeBandwidthMegabits[aws.StringValue(info.InstanceType)]; ok {
		requirements[v1beta1.LabelInstanceNetworkBandwidth].Insert(fmt.Sprint(bandwidth))
	}
	// GPU Labels
	if info.GpuInfo != nil && len(info.GpuInfo.Gpus) == 1 {
		gpu := info.GpuInfo.Gpus[0]
		requirements.Get(v1beta1.LabelInstanceGPUName).Insert(lowerKabobCase(aws.StringValue(gpu.Name)))
		requirements.Get(v1beta1.LabelInstanceGPUManufacturer).Insert(lowerKabobCase(aws.StringValue(gpu.Manufacturer)))
		requirements.Get(v1beta1.LabelInstanceGPUCount).Insert(fmt.Sprint(aws.Int64Value(gpu.Count)))
		requirements.Get(v1beta1.LabelInstanceGPUMemory).Insert(fmt.Sprint(aws.Int64Value(gpu.MemoryInfo.SizeInMiB)))
	}
	// Accelerators
	if info.InferenceAcceleratorInfo != nil && len(info.InferenceAcceleratorInfo.Accelerators) == 1 {
		accelerator := info.InferenceAcceleratorInfo.Accelerators[0]
		requirements.Get(v1beta1.LabelInstanceAcceleratorName).Insert(lowerKabobCase(aws.StringValue(accelerator.Name)))
		requirements.Get(v1beta1.LabelInstanceAcceleratorManufacturer).Insert(lowerKabobCase(aws.StringValue(accelerator.Manufacturer)))
		requirements.Get(v1beta1.LabelInstanceAcceleratorCount).Insert(fmt.Sprint(aws.Int64Value(accelerator.Count)))
	}
	// Windows Build Version Labels
	if family, ok := amiFamily.(*amifamily.Windows); ok {
		requirements.Get(v1.LabelWindowsBuild).Insert(family.Build)
	}
	// Trn1 Accelerators
	// TODO: remove function once DescribeInstanceTypes contains the accelerator data
	// Values found from: https://aws.amazon.com/ec2/instance-types/trn1/
	if strings.HasPrefix(*info.InstanceType, "trn1") {
		requirements.Get(v1beta1.LabelInstanceAcceleratorName).Insert(lowerKabobCase("Inferentia"))
		requirements.Get(v1beta1.LabelInstanceAcceleratorManufacturer).Insert(lowerKabobCase("AWS"))
		requirements.Get(v1beta1.LabelInstanceAcceleratorCount).Insert(fmt.Sprint(awsNeurons(info)))
	}
	return requirements
}

func getOS(info *ec2.InstanceTypeInfo, amiFamily amifamily.AMIFamily) []string {
	if _, ok := amiFamily.(*amifamily.Windows); ok {
		if getArchitecture(info) == corev1beta1.ArchitectureAmd64 {
			return []string{string(v1.Windows)}
		}
		return []string{}
	}
	return []string{string(v1.Linux)}
}

func getArchitecture(info *ec2.InstanceTypeInfo) string {
	for _, architecture := range info.ProcessorInfo.SupportedArchitectures {
		if value, ok := v1beta1.AWSToKubeArchitectures[aws.StringValue(architecture)]; ok {
			return value
		}
	}
	return fmt.Sprint(aws.StringValueSlice(info.ProcessorInfo.SupportedArchitectures)) // Unrecognized, but used for error printing
}

func computeCapacity(ctx context.Context, info *ec2.InstanceTypeInfo, amiFamily amifamily.AMIFamily,
	blockDeviceMappings []*v1beta1.BlockDeviceMapping, kc *corev1beta1.KubeletConfiguration) v1.ResourceList {

	resourceList := v1.ResourceList{
		v1.ResourceCPU:              *cpu(info),
		v1.ResourceMemory:           *memory(ctx, info),
		v1.ResourceEphemeralStorage: *ephemeralStorage(amiFamily, blockDeviceMappings),
		v1.ResourcePods:             *pods(ctx, info, amiFamily, kc),
		v1beta1.ResourceAWSPodENI:   *awsPodENI(aws.StringValue(info.InstanceType)),
		v1beta1.ResourceNVIDIAGPU:   *nvidiaGPUs(info),
		v1beta1.ResourceAMDGPU:      *amdGPUs(info),
		v1beta1.ResourceAWSNeuron:   *awsNeurons(info),
		v1beta1.ResourceHabanaGaudi: *habanaGaudis(info),
	}
	return resourceList
}

func cpu(info *ec2.InstanceTypeInfo) *resource.Quantity {
	return resources.Quantity(fmt.Sprint(*info.VCpuInfo.DefaultVCpus))
}

func memory(ctx context.Context, info *ec2.InstanceTypeInfo) *resource.Quantity {
	sizeInMib := *info.MemoryInfo.SizeInMiB
	// Gravitons have an extra 64 MiB of cma reserved memory that we can't use
	if len(info.ProcessorInfo.SupportedArchitectures) > 0 && *info.ProcessorInfo.SupportedArchitectures[0] == "arm64" {
		sizeInMib -= 64
	}
	mem := resources.Quantity(fmt.Sprintf("%dMi", sizeInMib))
	// Account for VM overhead in calculation
	mem.Sub(resource.MustParse(fmt.Sprintf("%dMi", int64(math.Ceil(float64(mem.Value())*options.FromContext(ctx).VMMemoryOverheadPercent/1024/1024)))))
	return mem
}

// Setting ephemeral-storage to be either the default value or what is defined in blockDeviceMappings
func ephemeralStorage(amiFamily amifamily.AMIFamily, blockDeviceMappings []*v1beta1.BlockDeviceMapping) *resource.Quantity {
	if len(blockDeviceMappings) != 0 {
		// First check if there's a root volume configured in blockDeviceMappings.
		if blockDeviceMapping, ok := lo.Find(blockDeviceMappings, func(bdm *v1beta1.BlockDeviceMapping) bool {
			return bdm.RootVolume
		}); ok && blockDeviceMapping.EBS.VolumeSize != nil {
			return blockDeviceMapping.EBS.VolumeSize
		}
		switch amiFamily.(type) {
		case *amifamily.Custom:
			// We can't know if a custom AMI is going to have a volume size.
			volumeSize := blockDeviceMappings[len(blockDeviceMappings)-1].EBS.VolumeSize
			return lo.Ternary(volumeSize != nil, volumeSize, amifamily.DefaultEBS.VolumeSize)
		default:
			// If a block device mapping exists in the provider for the root volume, use the volume size specified in the provider. If not, use the default
			if blockDeviceMapping, ok := lo.Find(blockDeviceMappings, func(bdm *v1beta1.BlockDeviceMapping) bool {
				return *bdm.DeviceName == *amiFamily.EphemeralBlockDevice()
			}); ok && blockDeviceMapping.EBS.VolumeSize != nil {
				return blockDeviceMapping.EBS.VolumeSize
			}
		}
	}
	//Return the ephemeralBlockDevice size if defined in ami
	if ephemeralBlockDevice, ok := lo.Find(amiFamily.DefaultBlockDeviceMappings(), func(item *v1beta1.BlockDeviceMapping) bool {
		return *amiFamily.EphemeralBlockDevice() == *item.DeviceName
	}); ok {
		return ephemeralBlockDevice.EBS.VolumeSize
	}
	return amifamily.DefaultEBS.VolumeSize
}

func awsPodENI(name string) *resource.Quantity {
	// https://docs.aws.amazon.com/eks/latest/userguide/security-groups-for-pods.html#supported-instance-types
	limits, ok := Limits[name]
	if ok && limits.IsTrunkingCompatible {
		return resources.Quantity(fmt.Sprint(limits.BranchInterface))
	}
	return resources.Quantity("0")
}

func nvidiaGPUs(info *ec2.InstanceTypeInfo) *resource.Quantity {
	count := int64(0)
	if info.GpuInfo != nil {
		for _, gpu := range info.GpuInfo.Gpus {
			if *gpu.Manufacturer == "NVIDIA" {
				count += *gpu.Count
			}
		}
	}
	return resources.Quantity(fmt.Sprint(count))
}

func amdGPUs(info *ec2.InstanceTypeInfo) *resource.Quantity {
	count := int64(0)
	if info.GpuInfo != nil {
		for _, gpu := range info.GpuInfo.Gpus {
			if *gpu.Manufacturer == "AMD" {
				count += *gpu.Count
			}
		}
	}
	return resources.Quantity(fmt.Sprint(count))
}

// TODO: remove trn1 hardcode values once DescribeInstanceTypes contains the accelerator data
// Values found from: https://aws.amazon.com/ec2/instance-types/trn1/
func awsNeurons(info *ec2.InstanceTypeInfo) *resource.Quantity {
	count := int64(0)
	if *info.InstanceType == "trn1.2xlarge" {
		count = int64(1)
	} else if *info.InstanceType == "trn1.32xlarge" {
		count = int64(16)
	} else if *info.InstanceType == "trn1n.32xlarge" {
		count = int64(16)
	} else if info.InferenceAcceleratorInfo != nil {
		for _, accelerator := range info.InferenceAcceleratorInfo.Accelerators {
			count += *accelerator.Count
		}
	}
	return resources.Quantity(fmt.Sprint(count))
}

func habanaGaudis(info *ec2.InstanceTypeInfo) *resource.Quantity {
	count := int64(0)
	if info.GpuInfo != nil {
		for _, gpu := range info.GpuInfo.Gpus {
			if *gpu.Manufacturer == "Habana" {
				count += *gpu.Count
			}
		}
	}
	return resources.Quantity(fmt.Sprint(count))
}

func ENILimitedPods(ctx context.Context, info *ec2.InstanceTypeInfo) *resource.Quantity {
	// The number of pods per node is calculated using the formula:
	// max number of ENIs * (IPv4 Addresses per ENI -1) + 2
	// https://github.com/awslabs/amazon-eks-ami/blob/master/files/eni-max-pods.txt#L20

	// VPC CNI only uses the default network interface
	// https://github.com/aws/amazon-vpc-cni-k8s/blob/3294231c0dce52cfe473bf6c62f47956a3b333b6/scripts/gen_vpc_ip_limits.go#L162
	networkInterfaces := *info.NetworkInfo.NetworkCards[*info.NetworkInfo.DefaultNetworkCardIndex].MaximumNetworkInterfaces
	usableNetworkInterfaces := lo.Max([]int64{(networkInterfaces - int64(options.FromContext(ctx).ReservedENIs)), 0})
	if usableNetworkInterfaces == 0 {
		return resource.NewQuantity(0, resource.DecimalSI)
	}
	addressesPerInterface := *info.NetworkInfo.Ipv4AddressesPerInterface
	return resources.Quantity(fmt.Sprint(usableNetworkInterfaces*(addressesPerInterface-1) + 2))
}

func privateIPv4Address(info *ec2.InstanceTypeInfo) *resource.Quantity {
	//https://github.com/aws/amazon-vpc-resource-controller-k8s/blob/ecbd6965a0100d9a070110233762593b16023287/pkg/provider/ip/provider.go#L297
	capacity := aws.Int64Value(info.NetworkInfo.Ipv4AddressesPerInterface) - 1
	return resources.Quantity(fmt.Sprint(capacity))
}

func systemReservedResources(kc *corev1beta1.KubeletConfiguration) v1.ResourceList {
	if kc != nil && kc.SystemReserved != nil {
		return kc.SystemReserved
	}
	return v1.ResourceList{}
}

func kubeReservedResources(cpus, pods, eniLimitedPods *resource.Quantity, amiFamily amifamily.AMIFamily, kc *corev1beta1.KubeletConfiguration) v1.ResourceList {
	if amiFamily.FeatureFlags().UsesENILimitedMemoryOverhead {
		pods = eniLimitedPods
	}
	resources := v1.ResourceList{
		v1.ResourceMemory:           resource.MustParse(fmt.Sprintf("%dMi", (11*pods.Value())+255)),
		v1.ResourceEphemeralStorage: resource.MustParse("1Gi"), // default kube-reserved ephemeral-storage
	}
	// kube-reserved Computed from
	// https://github.com/bottlerocket-os/bottlerocket/pull/1388/files#diff-bba9e4e3e46203be2b12f22e0d654ebd270f0b478dd34f40c31d7aa695620f2fR611
	for _, cpuRange := range []struct {
		start      int64
		end        int64
		percentage float64
	}{
		{start: 0, end: 1000, percentage: 0.06},
		{start: 1000, end: 2000, percentage: 0.01},
		{start: 2000, end: 4000, percentage: 0.005},
		{start: 4000, end: 1 << 31, percentage: 0.0025},
	} {
		if cpu := cpus.MilliValue(); cpu >= cpuRange.start {
			r := float64(cpuRange.end - cpuRange.start)
			if cpu < cpuRange.end {
				r = float64(cpu - cpuRange.start)
			}
			cpuOverhead := resources.Cpu()
			cpuOverhead.Add(*resource.NewMilliQuantity(int64(r*cpuRange.percentage), resource.DecimalSI))
			resources[v1.ResourceCPU] = *cpuOverhead
		}
	}
	if kc != nil && kc.KubeReserved != nil {
		return lo.Assign(resources, kc.KubeReserved)
	}
	return resources
}

func evictionThreshold(memory *resource.Quantity, storage *resource.Quantity, amiFamily amifamily.AMIFamily, kc *corev1beta1.KubeletConfiguration) v1.ResourceList {
	overhead := v1.ResourceList{
		v1.ResourceMemory:           resource.MustParse("100Mi"),
		v1.ResourceEphemeralStorage: resource.MustParse(fmt.Sprint(math.Ceil(float64(storage.Value()) / 100 * 10))),
	}
	if kc == nil {
		return overhead
	}

	override := v1.ResourceList{}
	var evictionSignals []map[string]string
	if kc.EvictionHard != nil {
		evictionSignals = append(evictionSignals, kc.EvictionHard)
	}
	if kc.EvictionSoft != nil && amiFamily.FeatureFlags().EvictionSoftEnabled {
		evictionSignals = append(evictionSignals, kc.EvictionSoft)
	}

	for _, m := range evictionSignals {
		temp := v1.ResourceList{}
		if v, ok := m[MemoryAvailable]; ok {
			temp[v1.ResourceMemory] = computeEvictionSignal(*memory, v)
		}
		if v, ok := m[NodeFSAvailable]; ok {
			temp[v1.ResourceEphemeralStorage] = computeEvictionSignal(*storage, v)
		}
		override = resources.MaxResources(override, temp)
	}
	// Assign merges maps from left to right so overrides will always be taken last
	return lo.Assign(overhead, override)
}

func pods(ctx context.Context, info *ec2.InstanceTypeInfo, amiFamily amifamily.AMIFamily, kc *corev1beta1.KubeletConfiguration) *resource.Quantity {
	var count int64
	switch {
	case kc != nil && kc.MaxPods != nil:
		count = int64(ptr.Int32Value(kc.MaxPods))
	case settings.FromContext(ctx).EnableENILimitedPodDensity && amiFamily.FeatureFlags().SupportsENILimitedPodDensity:
		count = ENILimitedPods(ctx, info).Value()
	default:
		count = 110

	}
	if kc != nil && ptr.Int32Value(kc.PodsPerCore) > 0 && amiFamily.FeatureFlags().PodsPerCoreEnabled {
		count = lo.Min([]int64{int64(ptr.Int32Value(kc.PodsPerCore)) * ptr.Int64Value(info.VCpuInfo.DefaultVCpus), count})
	}
	return resources.Quantity(fmt.Sprint(count))
}

func lowerKabobCase(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", "-"))
}

// computeEvictionSignal computes the resource quantity value for an eviction signal value, computed off the
// base capacity value if the signal value is a percentage or as a resource quantity if the signal value isn't a percentage
func computeEvictionSignal(capacity resource.Quantity, signalValue string) resource.Quantity {
	if strings.HasSuffix(signalValue, "%") {
		p := mustParsePercentage(signalValue)

		// Calculation is node.capacity * signalValue if percentage
		// From https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#eviction-signals
		return resource.MustParse(fmt.Sprint(math.Ceil(capacity.AsApproximateFloat64() / 100 * p)))
	}
	return resource.MustParse(signalValue)
}

func mustParsePercentage(v string) float64 {
	p, err := strconv.ParseFloat(strings.Trim(v, "%"), 64)
	if err != nil {
		panic(fmt.Sprintf("expected percentage value to be a float but got %s, %v", v, err))
	}
	// Setting percentage value to 100% is considered disabling the threshold according to
	// https://kubernetes.io/docs/reference/config-api/kubelet-config.v1beta1/
	if p == 100 {
		p = 0
	}
	return p
}
