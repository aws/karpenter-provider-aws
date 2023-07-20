package kwok

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/scheduling"
	"github.com/aws/karpenter-core/pkg/utils/resources"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/providers/instancetype"
)

var (
	instanceTypeScheme = regexp.MustCompile(`(^[a-z]+)(\-[0-9]+tb)?([0-9]+).*\.`)
)

func requirements(info *ec2.InstanceTypeInfo, offerings cloudprovider.Offerings) scheduling.Requirements {
	available := offerings.Available()
	requirements := scheduling.NewRequirements(
		scheduling.NewRequirement(v1.LabelInstanceTypeStable, v1.NodeSelectorOpIn, aws.StringValue(info.InstanceType)),
		scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, getArchitecture(info)),
		scheduling.NewRequirement(v1.LabelOSStable, v1.NodeSelectorOpIn, "linux"),
		scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, uniqueZones(available)...),
		scheduling.NewRequirement(v1.LabelTopologyRegion, v1.NodeSelectorOpIn, "kwok-region"),
		scheduling.NewRequirement(v1.LabelWindowsBuild, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha5.LabelCapacityType, v1.NodeSelectorOpIn, uniqueCapacityType(available)...),
		scheduling.NewRequirement(v1alpha1.LabelInstanceCPU, v1.NodeSelectorOpIn, strconv.FormatInt(aws.Int64Value(info.VCpuInfo.DefaultVCpus), 10)),
		scheduling.NewRequirement(v1alpha1.LabelInstanceMemory, v1.NodeSelectorOpIn, strconv.FormatInt(aws.Int64Value(info.MemoryInfo.SizeInMiB), 10)),
		scheduling.NewRequirement(v1alpha1.LabelInstanceNetworkBandwidth, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceCategory, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceFamily, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceGeneration, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceLocalNVME, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceSize, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceGPUName, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceGPUManufacturer, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceGPUCount, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceGPUMemory, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceAcceleratorName, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceAcceleratorManufacturer, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceAcceleratorCount, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceHypervisor, v1.NodeSelectorOpIn, aws.StringValue(info.Hypervisor)),
		scheduling.NewRequirement(v1alpha1.LabelInstanceEncryptionInTransitSupported, v1.NodeSelectorOpIn, strconv.FormatBool(aws.BoolValue(info.NetworkInfo.EncryptionInTransitSupported))),
	)
	// Instance Type Labels
	instanceFamilyParts := instanceTypeScheme.FindStringSubmatch(aws.StringValue(info.InstanceType))
	if len(instanceFamilyParts) == 4 {
		requirements[v1alpha1.LabelInstanceCategory].Insert(instanceFamilyParts[1])
		requirements[v1alpha1.LabelInstanceGeneration].Insert(instanceFamilyParts[3])
	}
	instanceTypeParts := strings.Split(aws.StringValue(info.InstanceType), ".")
	if len(instanceTypeParts) == 2 {
		requirements.Get(v1alpha1.LabelInstanceFamily).Insert(instanceTypeParts[0])
		requirements.Get(v1alpha1.LabelInstanceSize).Insert(instanceTypeParts[1])
	}
	if info.InstanceStorageInfo != nil && aws.StringValue(info.InstanceStorageInfo.NvmeSupport) != ec2.EphemeralNvmeSupportUnsupported {
		requirements[v1alpha1.LabelInstanceLocalNVME].Insert(fmt.Sprint(aws.Int64Value(info.InstanceStorageInfo.TotalSizeInGB)))
	}
	// Network bandwidth
	if bandwidth, ok := instancetype.InstanceTypeBandwidthMegabits[aws.StringValue(info.InstanceType)]; ok {
		requirements[v1alpha1.LabelInstanceNetworkBandwidth].Insert(fmt.Sprint(bandwidth))
	}
	// GPU Labels
	if info.GpuInfo != nil && len(info.GpuInfo.Gpus) == 1 {
		gpu := info.GpuInfo.Gpus[0]
		requirements.Get(v1alpha1.LabelInstanceGPUName).Insert(lowerKabobCase(aws.StringValue(gpu.Name)))
		requirements.Get(v1alpha1.LabelInstanceGPUManufacturer).Insert(lowerKabobCase(aws.StringValue(gpu.Manufacturer)))
		requirements.Get(v1alpha1.LabelInstanceGPUCount).Insert(fmt.Sprint(aws.Int64Value(gpu.Count)))
		requirements.Get(v1alpha1.LabelInstanceGPUMemory).Insert(fmt.Sprint(aws.Int64Value(gpu.MemoryInfo.SizeInMiB)))
	}
	// Accelerators
	if info.InferenceAcceleratorInfo != nil && len(info.InferenceAcceleratorInfo.Accelerators) == 1 {
		accelerator := info.InferenceAcceleratorInfo.Accelerators[0]
		requirements.Get(v1alpha1.LabelInstanceAcceleratorName).Insert(lowerKabobCase(aws.StringValue(accelerator.Name)))
		requirements.Get(v1alpha1.LabelInstanceAcceleratorManufacturer).Insert(lowerKabobCase(aws.StringValue(accelerator.Manufacturer)))
		requirements.Get(v1alpha1.LabelInstanceAcceleratorCount).Insert(fmt.Sprint(aws.Int64Value(accelerator.Count)))
	}

	return requirements
}

func uniqueCapacityType(available cloudprovider.Offerings) []string {
	uniq := map[string]struct{}{}
	for _, c := range available {
		uniq[c.CapacityType] = struct{}{}
	}
	keys := make([]string, 0, len(uniq))
	for k := range uniq {
		keys = append(keys, k)
	}
	return keys
}

func uniqueZones(available cloudprovider.Offerings) []string {
	uniq := map[string]struct{}{}
	for _, c := range available {
		uniq[c.Zone] = struct{}{}
	}
	keys := make([]string, 0, len(uniq))
	for k := range uniq {
		keys = append(keys, k)
	}
	return keys
}

func computeCapacity(ctx context.Context, info *ec2.InstanceTypeInfo) v1.ResourceList {
	resourceList := v1.ResourceList{
		v1.ResourceCPU:               *cpu(info),
		v1.ResourceMemory:            *memory(ctx, info),
		v1.ResourceEphemeralStorage:  resource.MustParse("20G"),
		v1.ResourcePods:              resource.MustParse("110"),
		v1alpha1.ResourceNVIDIAGPU:   *nvidiaGPUs(info),
		v1alpha1.ResourceAMDGPU:      *amdGPUs(info),
		v1alpha1.ResourceAWSNeuron:   *awsNeurons(info),
		v1alpha1.ResourceHabanaGaudi: *habanaGaudis(info),
	}
	return resourceList
}
func lowerKabobCase(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", "-"))
}
func getArchitecture(info *ec2.InstanceTypeInfo) string {
	for _, architecture := range info.ProcessorInfo.SupportedArchitectures {
		if value, ok := v1alpha1.AWSToKubeArchitectures[aws.StringValue(architecture)]; ok {
			return value
		}
	}
	return fmt.Sprint(aws.StringValueSlice(info.ProcessorInfo.SupportedArchitectures)) // Unrecognized, but used for error printing
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
	VMMemoryOverheadPercent := 0.075
	mem.Sub(resource.MustParse(fmt.Sprintf("%dMi", int64(math.Ceil(float64(mem.Value())*VMMemoryOverheadPercent/1024/1024)))))
	return mem
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
