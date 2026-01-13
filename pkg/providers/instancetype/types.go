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

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

const (
	MemoryAvailable = "memory.available"
	NodeFSAvailable = "nodefs.available"
)

var (
	instanceTypeScheme = regexp.MustCompile(`(^[a-z]+)(\-[0-9]+tb)?([0-9]+).*\.`)
)

type ZoneData struct {
	Name      string
	ID        string
	Available bool
}

type Resolver interface {
	// CacheKey tells the InstanceType cache if something changes about the InstanceTypes or Offerings based on the NodeClass.
	CacheKey(NodeClass) string
	// Resolve generates an InstanceType based on raw InstanceTypeInfo and NodeClass setting data
	Resolve(ctx context.Context, info ec2types.InstanceTypeInfo, zones []string, nodeClass NodeClass) *cloudprovider.InstanceType
}

type DefaultResolver struct {
	region string
}

func NewDefaultResolver(region string) *DefaultResolver {
	return &DefaultResolver{
		region: region,
	}
}

func (d *DefaultResolver) CacheKey(nodeClass NodeClass) string {
	kc := &v1.KubeletConfiguration{}
	if resolved := nodeClass.KubeletConfiguration(); resolved != nil {
		kc = resolved
	}
	kcHash, _ := hashstructure.Hash(kc, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	blockDeviceMappingsHash, _ := hashstructure.Hash(nodeClass.BlockDeviceMappings(), hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	capacityReservationHash, _ := hashstructure.Hash(nodeClass.CapacityReservations(), hashstructure.FormatV2, nil)
	return fmt.Sprintf(
		"%016x-%016x-%016x-%s-%s",
		kcHash,
		blockDeviceMappingsHash,
		capacityReservationHash,
		lo.FromPtr((*string)(nodeClass.InstanceStorePolicy())),
		nodeClass.AMIFamily(),
	)
}

func (d *DefaultResolver) Resolve(ctx context.Context, info ec2types.InstanceTypeInfo, zones []string, nodeClass NodeClass) *cloudprovider.InstanceType {
	// !!! Important !!!
	// Any changes to the values passed into the NewInstanceType method will require making updates to the cache key
	// so that Karpenter is able to cache the set of InstanceTypes based on values that alter the set of instance types
	// !!! Important !!!
	kc := &v1.KubeletConfiguration{}
	if resolved := nodeClass.KubeletConfiguration(); resolved != nil {
		kc = resolved
	}
	return NewInstanceType(
		ctx,
		info,
		d.region,
		zones,
		nodeClass.ZoneInfo(),
		nodeClass.BlockDeviceMappings(),
		nodeClass.InstanceStorePolicy(),
		kc.MaxPods,
		kc.PodsPerCore,
		kc.KubeReserved,
		kc.SystemReserved,
		kc.EvictionHard,
		kc.EvictionSoft,
		nodeClass.AMIFamily(),
		lo.Filter(nodeClass.CapacityReservations(), func(cr v1.CapacityReservation, _ int) bool {
			return cr.InstanceType == string(info.InstanceType)
		}),
	)
}

func NewInstanceType(
	ctx context.Context,
	info ec2types.InstanceTypeInfo,
	region string,
	offeringZones []string,
	subnetZoneInfo []v1.ZoneInfo,
	blockDeviceMappings []*v1.BlockDeviceMapping,
	instanceStorePolicy *v1.InstanceStorePolicy,
	maxPods *int32,
	podsPerCore *int32,
	kubeReserved map[string]string,
	systemReserved map[string]string,
	evictionHard map[string]string,
	evictionSoft map[string]string,
	amiFamilyType string,
	capacityReservations []v1.CapacityReservation,
) *cloudprovider.InstanceType {
	amiFamily := amifamily.GetAMIFamily(amiFamilyType, &amifamily.Options{})
	it := &cloudprovider.InstanceType{
		Name:         string(info.InstanceType),
		Requirements: computeRequirements(info, region, offeringZones, subnetZoneInfo, amiFamily, capacityReservations),
		Capacity:     computeCapacity(ctx, info, amiFamily, blockDeviceMappings, instanceStorePolicy, maxPods, podsPerCore),
		Overhead: &cloudprovider.InstanceTypeOverhead{
			KubeReserved:      kubeReservedResources(cpu(info), lo.Ternary(amiFamily.FeatureFlags().UsesENILimitedMemoryOverhead, ENILimitedPods(ctx, info, 0), pods(ctx, info, amiFamily, maxPods, podsPerCore)), kubeReserved),
			SystemReserved:    systemReservedResources(systemReserved),
			EvictionThreshold: evictionThreshold(memory(ctx, info), ephemeralStorage(info, amiFamily, blockDeviceMappings, instanceStorePolicy), evictionHard),
		},
	}
	if it.Requirements.Compatible(scheduling.NewRequirements(scheduling.NewRequirement(corev1.LabelOSStable, corev1.NodeSelectorOpIn, string(corev1.Windows)))) == nil {
		it.Capacity[v1.ResourcePrivateIPv4Address] = *privateIPv4Address(string(info.InstanceType))
	}
	return it
}

//nolint:gocyclo
func computeRequirements(
	info ec2types.InstanceTypeInfo,
	region string,
	offeringZones []string,
	subnetZoneInfo []v1.ZoneInfo,
	amiFamily amifamily.AMIFamily,
	capacityReservations []v1.CapacityReservation,
) scheduling.Requirements {
	capacityTypes := lo.FilterMap(info.SupportedUsageClasses, func(uc ec2types.UsageClassType, _ int) (string, bool) {
		if uc != ec2types.UsageClassTypeOnDemand && uc != ec2types.UsageClassTypeSpot {
			return "", false
		}
		return string(uc), true
	})
	if len(capacityReservations) != 0 {
		capacityTypes = append(capacityTypes, karpv1.CapacityTypeReserved)
	}

	// Available zones is the set intersection between zones where the instance type is available, and zones which are
	// available via the provided EC2NodeClass.
	availableZones := sets.New(offeringZones...).Intersection(sets.New(lo.Map(subnetZoneInfo, func(info v1.ZoneInfo, _ int) string {
		return info.Zone
	})...))
	requirements := scheduling.NewRequirements(
		// Well Known Upstream
		scheduling.NewRequirement(corev1.LabelInstanceTypeStable, corev1.NodeSelectorOpIn, string(info.InstanceType)),
		scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, getArchitecture(info)),
		scheduling.NewRequirement(corev1.LabelOSStable, corev1.NodeSelectorOpIn, getOS(info, amiFamily)...),
		scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, availableZones.UnsortedList()...),
		scheduling.NewRequirement(corev1.LabelTopologyRegion, corev1.NodeSelectorOpIn, region),
		scheduling.NewRequirement(corev1.LabelWindowsBuild, corev1.NodeSelectorOpDoesNotExist),
		// Well Known to Karpenter
		scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, capacityTypes...),
		// Well Known to AWS
		scheduling.NewRequirement(v1.LabelInstanceCPU, corev1.NodeSelectorOpIn, fmt.Sprint(lo.FromPtr(info.VCpuInfo.DefaultVCpus))),
		scheduling.NewRequirement(v1.LabelInstanceCPUManufacturer, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceCPUSustainedClockSpeedMhz, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceMemory, corev1.NodeSelectorOpIn, fmt.Sprint(lo.FromPtr(info.MemoryInfo.SizeInMiB))),
		scheduling.NewRequirement(v1.LabelInstanceEBSBandwidth, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceNetworkBandwidth, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceCategory, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceCapabilityFlex, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceFamily, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceGeneration, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceLocalNVME, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceSize, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceGPUName, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceGPUManufacturer, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceGPUCount, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceGPUMemory, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceAcceleratorName, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceAcceleratorManufacturer, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceAcceleratorCount, corev1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1.LabelInstanceHypervisor, corev1.NodeSelectorOpIn, string(info.Hypervisor)),
		scheduling.NewRequirement(v1.LabelInstanceEncryptionInTransitSupported, corev1.NodeSelectorOpIn, fmt.Sprint(aws.ToBool(info.NetworkInfo.EncryptionInTransitSupported))),
		scheduling.NewRequirement(v1.LabelInstanceTenancy, corev1.NodeSelectorOpIn, string(ec2types.TenancyDefault), string(ec2types.TenancyDedicated)),
	)
	// Only add zone-id label when available in offerings. It may not be available if a user has upgraded from a
	// previous version of Karpenter w/o zone-id support and the nodeclass subnet status has not yet updated.
	if zoneIDs := lo.FilterMap(subnetZoneInfo, func(info v1.ZoneInfo, _ int) (string, bool) {
		if !availableZones.Has(info.Zone) {
			return "", false
		}
		return info.ZoneID, true
	}); len(zoneIDs) != 0 {
		requirements.Add(scheduling.NewRequirement(v1.LabelTopologyZoneID, corev1.NodeSelectorOpIn, zoneIDs...))
	}
	if len(capacityReservations) != 0 {
		requirements.Add(scheduling.NewRequirement(cloudprovider.ReservationIDLabel, corev1.NodeSelectorOpIn, lo.Map(capacityReservations, func(cr v1.CapacityReservation, _ int) string {
			return cr.ID
		})...))
		requirements.Add(scheduling.NewRequirement(v1.LabelCapacityReservationType, corev1.NodeSelectorOpIn, lo.Map(capacityReservations, func(cr v1.CapacityReservation, _ int) string {
			return string(cr.ReservationType)
		})...))
	} else {
		requirements.Add(scheduling.NewRequirement(cloudprovider.ReservationIDLabel, corev1.NodeSelectorOpDoesNotExist))
		requirements.Add(scheduling.NewRequirement(v1.LabelCapacityReservationType, corev1.NodeSelectorOpDoesNotExist))
	}
	// Instance Type Labels
	instanceFamilyParts := instanceTypeScheme.FindStringSubmatch(string(info.InstanceType))
	if len(instanceFamilyParts) == 4 {
		requirements[v1.LabelInstanceCategory].Insert(instanceFamilyParts[1])
		requirements[v1.LabelInstanceGeneration].Insert(instanceFamilyParts[3])
	}
	instanceTypeParts := strings.Split(string(info.InstanceType), ".")
	if len(instanceTypeParts) == 2 {
		requirements.Get(v1.LabelInstanceFamily).Insert(instanceTypeParts[0])
		requirements.Get(v1.LabelInstanceSize).Insert(instanceTypeParts[1])
	}
	if info.InstanceStorageInfo != nil && info.InstanceStorageInfo.NvmeSupport != ec2types.EphemeralNvmeSupportUnsupported && info.InstanceStorageInfo.TotalSizeInGB != nil {
		requirements[v1.LabelInstanceLocalNVME].Insert(fmt.Sprint(lo.FromPtr(info.InstanceStorageInfo.TotalSizeInGB)))
	}
	if strings.Contains(instanceTypeParts[0], "-flex") {
		requirements[v1.LabelInstanceCapabilityFlex].Insert("true")
	} else {
		requirements[v1.LabelInstanceCapabilityFlex].Insert("false")
	}

	// Network bandwidth
	if bandwidth, ok := InstanceTypeBandwidthMegabits[string(info.InstanceType)]; ok {
		requirements[v1.LabelInstanceNetworkBandwidth].Insert(fmt.Sprint(bandwidth))
	}
	// GPU Labels
	if info.GpuInfo != nil && len(info.GpuInfo.Gpus) == 1 {
		gpu := info.GpuInfo.Gpus[0]
		requirements.Get(v1.LabelInstanceGPUName).Insert(lowerKabobCase(aws.ToString(gpu.Name)))
		requirements.Get(v1.LabelInstanceGPUManufacturer).Insert(lowerKabobCase(aws.ToString(gpu.Manufacturer)))
		requirements.Get(v1.LabelInstanceGPUCount).Insert(fmt.Sprint(lo.FromPtr(gpu.Count)))
		requirements.Get(v1.LabelInstanceGPUMemory).Insert(fmt.Sprint(lo.FromPtr(gpu.MemoryInfo.SizeInMiB)))
	}
	// Accelerators - excluding Neuron
	if info.InferenceAcceleratorInfo != nil && len(info.InferenceAcceleratorInfo.Accelerators) == 1 && info.NeuronInfo == nil {
		accelerator := info.InferenceAcceleratorInfo.Accelerators[0]
		requirements.Get(v1.LabelInstanceAcceleratorName).Insert(lowerKabobCase(aws.ToString(accelerator.Name)))
		requirements.Get(v1.LabelInstanceAcceleratorManufacturer).Insert(lowerKabobCase(aws.ToString(accelerator.Manufacturer)))
		requirements.Get(v1.LabelInstanceAcceleratorCount).Insert(fmt.Sprint(lo.FromPtr(accelerator.Count)))
	}
	// Neuron
	if info.NeuronInfo != nil && len(info.NeuronInfo.NeuronDevices) == 1 {
		device := info.NeuronInfo.NeuronDevices[0]
		requirements.Get(v1.LabelInstanceAcceleratorName).Insert(lowerKabobCase(lo.FromPtr(device.Name)))
		requirements.Get(v1.LabelInstanceAcceleratorManufacturer).Insert(lowerKabobCase("aws"))
		requirements.Get(v1.LabelInstanceAcceleratorCount).Insert(fmt.Sprint(lo.FromPtr(device.Count)))
	}
	// Windows Build Version Labels
	if family, ok := amiFamily.(*amifamily.Windows); ok {
		requirements.Get(corev1.LabelWindowsBuild).Insert(family.Build)
	}
	// CPU Manufacturer, valid options: aws, intel, amd
	if info.ProcessorInfo != nil {
		requirements.Get(v1.LabelInstanceCPUManufacturer).Insert(lowerKabobCase(aws.ToString(info.ProcessorInfo.Manufacturer)))
	}
	// CPU Sustained Clock Speed
	if info.ProcessorInfo != nil {
		// Convert from Ghz to Mhz and round to nearest whole number - converting from float64 to int to support Gt and Lt operators
		requirements.Get(v1.LabelInstanceCPUSustainedClockSpeedMhz).Insert(fmt.Sprint(int(math.Round(aws.ToFloat64(info.ProcessorInfo.SustainedClockSpeedInGhz) * 1000))))
	}
	// EBS Max Bandwidth
	if info.EbsInfo != nil && info.EbsInfo.EbsOptimizedInfo != nil && info.EbsInfo.EbsOptimizedSupport == ec2types.EbsOptimizedSupportDefault {
		requirements.Get(v1.LabelInstanceEBSBandwidth).Insert(fmt.Sprint(lo.FromPtr(info.EbsInfo.EbsOptimizedInfo.MaximumBandwidthInMbps)))
	}
	return requirements
}

func getOS(info ec2types.InstanceTypeInfo, amiFamily amifamily.AMIFamily) []string {
	if _, ok := amiFamily.(*amifamily.Windows); ok {
		if getArchitecture(info) == karpv1.ArchitectureAmd64 {
			return []string{string(corev1.Windows)}
		}
		return []string{}
	}
	return []string{string(corev1.Linux)}
}

func getArchitecture(info ec2types.InstanceTypeInfo) string {
	for _, architecture := range info.ProcessorInfo.SupportedArchitectures {
		if value, ok := v1.AWSToKubeArchitectures[string(architecture)]; ok {
			return value
		}
	}
	return fmt.Sprint(info.ProcessorInfo.SupportedArchitectures) // Unrecognized, but used for error printing
}

func computeCapacity(ctx context.Context, info ec2types.InstanceTypeInfo, amiFamily amifamily.AMIFamily,
	blockDeviceMapping []*v1.BlockDeviceMapping, instanceStorePolicy *v1.InstanceStorePolicy,
	maxPods *int32, podsPerCore *int32) corev1.ResourceList {

	resourceList := corev1.ResourceList{
		corev1.ResourceCPU:              *cpu(info),
		corev1.ResourceMemory:           *memory(ctx, info),
		corev1.ResourceEphemeralStorage: *ephemeralStorage(info, amiFamily, blockDeviceMapping, instanceStorePolicy),
		corev1.ResourcePods:             *pods(ctx, info, amiFamily, maxPods, podsPerCore),
		v1.ResourceAWSPodENI:            *awsPodENI(string(info.InstanceType)),
		v1.ResourceNVIDIAGPU:            *nvidiaGPUs(info),
		v1.ResourceAMDGPU:               *amdGPUs(info),
		v1.ResourceAWSNeuron:            *awsNeuronDevices(info),
		v1.ResourceAWSNeuronCore:        *awsNeuronCores(info),
		v1.ResourceHabanaGaudi:          *habanaGaudis(info),
		v1.ResourceEFA:                  *efas(info),
	}
	return resourceList
}

func cpu(info ec2types.InstanceTypeInfo) *resource.Quantity {
	return resources.Quantity(fmt.Sprint(*info.VCpuInfo.DefaultVCpus))
}

func memory(ctx context.Context, info ec2types.InstanceTypeInfo) *resource.Quantity {
	sizeInMib := *info.MemoryInfo.SizeInMiB
	// Gravitons have an extra 64 MiB of cma reserved memory that we can't use
	if len(info.ProcessorInfo.SupportedArchitectures) > 0 && info.ProcessorInfo.SupportedArchitectures[0] == "arm64" {
		sizeInMib -= 64
	}
	mem := resources.Quantity(fmt.Sprintf("%dMi", sizeInMib))
	// Account for VM overhead in calculation
	mem.Sub(resource.MustParse(fmt.Sprintf("%dMi", int64(math.Ceil(float64(mem.Value())*options.FromContext(ctx).VMMemoryOverheadPercent/1024/1024)))))
	return mem
}

// Setting ephemeral-storage to be either the default value, what is defined in blockDeviceMappings, or the combined size of local store volumes.
func ephemeralStorage(info ec2types.InstanceTypeInfo, amiFamily amifamily.AMIFamily, blockDeviceMappings []*v1.BlockDeviceMapping, instanceStorePolicy *v1.InstanceStorePolicy) *resource.Quantity {
	// If local store disks have been configured for node ephemeral-storage, use the total size of the disks.
	if lo.FromPtr(instanceStorePolicy) == v1.InstanceStorePolicyRAID0 {
		if info.InstanceStorageInfo != nil && info.InstanceStorageInfo.TotalSizeInGB != nil {
			return resources.Quantity(fmt.Sprintf("%dG", *info.InstanceStorageInfo.TotalSizeInGB))
		}
	}
	if len(blockDeviceMappings) != 0 {
		// First check if there's a root volume configured in blockDeviceMappings.
		if blockDeviceMapping, ok := lo.Find(blockDeviceMappings, func(bdm *v1.BlockDeviceMapping) bool {
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
			if blockDeviceMapping, ok := lo.Find(blockDeviceMappings, func(bdm *v1.BlockDeviceMapping) bool {
				return *bdm.DeviceName == *amiFamily.EphemeralBlockDevice()
			}); ok && blockDeviceMapping.EBS.VolumeSize != nil {
				return blockDeviceMapping.EBS.VolumeSize
			}
		}
	}
	//Return the ephemeralBlockDevice size if defined in ami
	if ephemeralBlockDevice, ok := lo.Find(amiFamily.DefaultBlockDeviceMappings(), func(item *v1.BlockDeviceMapping) bool {
		return *amiFamily.EphemeralBlockDevice() == *item.DeviceName
	}); ok {
		return ephemeralBlockDevice.EBS.VolumeSize
	}
	return amifamily.DefaultEBS.VolumeSize
}

// awsPodENI relies on the VPC resource controller to populate the vpc.amazonaws.com/pod-eni resource
func awsPodENI(instanceTypeName string) *resource.Quantity {
	// https://docs.aws.amazon.com/eks/latest/userguide/security-groups-for-pods.html#supported-instance-types
	limits, ok := Limits[instanceTypeName]
	if ok && limits.IsTrunkingCompatible {
		return resources.Quantity(fmt.Sprint(limits.BranchInterface))
	}
	return resources.Quantity("0")
}

func nvidiaGPUs(info ec2types.InstanceTypeInfo) *resource.Quantity {
	count := int32(0)
	if info.GpuInfo != nil {
		for _, gpu := range info.GpuInfo.Gpus {
			if *gpu.Manufacturer == "NVIDIA" {
				count += *gpu.Count
			}
		}
	}
	return resources.Quantity(fmt.Sprint(count))
}

func amdGPUs(info ec2types.InstanceTypeInfo) *resource.Quantity {
	count := int32(0)
	if info.GpuInfo != nil {
		for _, gpu := range info.GpuInfo.Gpus {
			if *gpu.Manufacturer == "AMD" {
				count += *gpu.Count
			}
		}
	}
	return resources.Quantity(fmt.Sprint(count))
}

func awsNeuronCores(info ec2types.InstanceTypeInfo) *resource.Quantity {
	count := int32(0)
	if info.NeuronInfo != nil {
		neuronDevice := info.NeuronInfo.NeuronDevices[0]
		neuronCorePerDevice := neuronDevice.CoreInfo.Count
		count = *neuronDevice.Count * *neuronCorePerDevice
	}
	return resources.Quantity(fmt.Sprint(count))
}

func awsNeuronDevices(info ec2types.InstanceTypeInfo) *resource.Quantity {
	count := int32(0)
	if info.NeuronInfo != nil {
		for _, device := range info.NeuronInfo.NeuronDevices {
			count += *device.Count
		}
	}
	return resources.Quantity(fmt.Sprint(count))
}

func habanaGaudis(info ec2types.InstanceTypeInfo) *resource.Quantity {
	count := int32(0)
	if info.GpuInfo != nil {
		for _, gpu := range info.GpuInfo.Gpus {
			if *gpu.Manufacturer == "Habana" {
				count += *gpu.Count
			}
		}
	}
	return resources.Quantity(fmt.Sprint(count))
}

func efas(info ec2types.InstanceTypeInfo) *resource.Quantity {
	count := int32(0)
	if info.NetworkInfo != nil && info.NetworkInfo.EfaInfo != nil && info.NetworkInfo.EfaInfo.MaximumEfaInterfaces != nil {
		count = *info.NetworkInfo.EfaInfo.MaximumEfaInterfaces
	}
	return resources.Quantity(fmt.Sprint(count))
}

func ENILimitedPods(ctx context.Context, info ec2types.InstanceTypeInfo, reservedENIs int) *resource.Quantity {
	// The number of pods per node is calculated using the formula:
	// max number of ENIs * (IPv4 Addresses per ENI -1) + 2
	// https://github.com/awslabs/amazon-eks-ami/blob/main/templates/shared/runtime/eni-max-pods.txt

	// VPC CNI only uses the default network interface
	// https://github.com/aws/amazon-vpc-cni-k8s/blob/3294231c0dce52cfe473bf6c62f47956a3b333b6/scripts/gen_vpc_ip_limits.go#L162
	networkInterfaces := *info.NetworkInfo.NetworkCards[*info.NetworkInfo.DefaultNetworkCardIndex].MaximumNetworkInterfaces
	usableNetworkInterfaces := lo.Max([]int64{int64(int(networkInterfaces) - reservedENIs), 0})
	if usableNetworkInterfaces == 0 {
		return resource.NewQuantity(0, resource.DecimalSI)
	}
	addressesPerInterface := *info.NetworkInfo.Ipv4AddressesPerInterface
	return resources.Quantity(fmt.Sprint(usableNetworkInterfaces*(int64(addressesPerInterface)-1) + 2))
}

func privateIPv4Address(instanceTypeName string) *resource.Quantity {
	//https://github.com/aws/amazon-vpc-resource-controller-k8s/blob/ecbd6965a0100d9a070110233762593b16023287/pkg/provider/ip/provider.go#L297
	limits, ok := Limits[instanceTypeName]
	if !ok {
		return resources.Quantity("0")
	}
	return resources.Quantity(fmt.Sprint(limits.IPv4PerInterface - 1))
}

func systemReservedResources(systemReserved map[string]string) corev1.ResourceList {
	return lo.MapEntries(systemReserved, func(k string, v string) (corev1.ResourceName, resource.Quantity) {
		return corev1.ResourceName(k), resource.MustParse(v)
	})
}

func kubeReservedResources(cpus, pods *resource.Quantity, kubeReserved map[string]string) corev1.ResourceList {
	resources := corev1.ResourceList{
		corev1.ResourceMemory:           resource.MustParse(fmt.Sprintf("%dMi", (11*pods.Value())+255)),
		corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"), // default kube-reserved ephemeral-storage
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
			resources[corev1.ResourceCPU] = *cpuOverhead
		}
	}
	return lo.Assign(resources, lo.MapEntries(kubeReserved, func(k string, v string) (corev1.ResourceName, resource.Quantity) {
		return corev1.ResourceName(k), resource.MustParse(v)
	}))
}

func evictionThreshold(memory *resource.Quantity, storage *resource.Quantity, evictionHard map[string]string) corev1.ResourceList {
	overhead := corev1.ResourceList{
		corev1.ResourceMemory:           resource.MustParse("100Mi"),
		corev1.ResourceEphemeralStorage: resource.MustParse(fmt.Sprint(math.Ceil(float64(storage.Value()) / 100 * 10))),
	}

	override := corev1.ResourceList{}
	// Only use evictionHard for allocatable memory calculation
	// evictionSoft should not impact allocatable capacity as it's only a warning threshold
	// See: https://kubernetes.io/docs/tasks/administer-cluster/reserve-compute-resources/#eviction-thresholds
	if evictionHard != nil {
		if v, ok := evictionHard[MemoryAvailable]; ok {
			override[corev1.ResourceMemory] = computeEvictionSignal(*memory, v)
		}
		if v, ok := evictionHard[NodeFSAvailable]; ok {
			override[corev1.ResourceEphemeralStorage] = computeEvictionSignal(*storage, v)
		}
	}
	// Assign merges maps from left to right so overrides will always be taken last
	return lo.Assign(overhead, override)
}

func pods(ctx context.Context, info ec2types.InstanceTypeInfo, amiFamily amifamily.AMIFamily, maxPods *int32, podsPerCore *int32) *resource.Quantity {
	var count int64
	switch {
	case maxPods != nil:
		count = int64(lo.FromPtr(maxPods))
	case amiFamily.FeatureFlags().SupportsENILimitedPodDensity:
		count = ENILimitedPods(ctx, info, options.FromContext(ctx).ReservedENIs).Value()
	default:
		count = 110

	}
	if lo.FromPtr(podsPerCore) > 0 && amiFamily.FeatureFlags().PodsPerCoreEnabled {
		count = lo.Min([]int64{int64(lo.FromPtr(podsPerCore)) * int64(lo.FromPtr(info.VCpuInfo.DefaultVCpus)), count})
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
