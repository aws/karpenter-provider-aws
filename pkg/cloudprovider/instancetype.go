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

package cloudprovider

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

	awssettings "github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/amifamily"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/scheduling"
	"github.com/aws/karpenter-core/pkg/utils/resources"
)

const (
	memoryAvailable = "memory.available"
)

var (
	instanceTypeScheme = regexp.MustCompile(`(^[a-z]+)(\-[0-9]+tb)?([0-9]+).*\.`)
)

func NewInstanceType(ctx context.Context, info *ec2.InstanceTypeInfo, kc *v1alpha5.KubeletConfiguration,
	region string, nodeTemplate *v1alpha1.AWSNodeTemplate, offerings cloudprovider.Offerings) *cloudprovider.InstanceType {

	amiFamily := amifamily.GetAMIFamily(nodeTemplate.Spec.AMIFamily, &amifamily.Options{})
	return &cloudprovider.InstanceType{
		Name:         aws.StringValue(info.InstanceType),
		Requirements: computeRequirements(ctx, info, offerings, region, amiFamily, kc),
		Offerings:    offerings,
		Capacity:     computeCapacity(ctx, info, amiFamily, nodeTemplate.Spec.BlockDeviceMappings, kc),
		Overhead: &cloudprovider.InstanceTypeOverhead{
			KubeReserved:      kubeReservedResources(cpu(info), pods(ctx, info, amiFamily, kc), eniLimitedPods(info), amiFamily, kc),
			SystemReserved:    systemReservedResources(kc),
			EvictionThreshold: evictionThreshold(memory(ctx, info), amiFamily, kc),
		},
	}
}

func computeRequirements(ctx context.Context, info *ec2.InstanceTypeInfo, offerings cloudprovider.Offerings, region string,
	amiFamily amifamily.AMIFamily, kc *v1alpha5.KubeletConfiguration) scheduling.Requirements {

	// TODO joinnis: Switch the AvailableOfferings call back to the cloudprovider.AvailableOfferings call
	requirements := scheduling.NewRequirements(
		// Well Known Upstream
		scheduling.NewRequirement(v1.LabelInstanceTypeStable, v1.NodeSelectorOpIn, aws.StringValue(info.InstanceType)),
		scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, getArchitecture(info)),
		scheduling.NewRequirement(v1.LabelOSStable, v1.NodeSelectorOpIn, string(v1.Linux)),
		scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, lo.Map(offerings.Available(), func(o cloudprovider.Offering, _ int) string { return o.Zone })...),
		scheduling.NewRequirement(v1.LabelTopologyRegion, v1.NodeSelectorOpIn, region),
		// Well Known to Karpenter
		scheduling.NewRequirement(v1alpha5.LabelCapacityType, v1.NodeSelectorOpIn, lo.Map(offerings.Available(), func(o cloudprovider.Offering, _ int) string { return o.CapacityType })...),
		// Well Known to AWS
		scheduling.NewRequirement(v1alpha1.LabelInstanceCPU, v1.NodeSelectorOpIn, fmt.Sprint(aws.Int64Value(info.VCpuInfo.DefaultVCpus))),
		scheduling.NewRequirement(v1alpha1.LabelInstanceMemory, v1.NodeSelectorOpIn, fmt.Sprint(aws.Int64Value(info.MemoryInfo.SizeInMiB))),
		scheduling.NewRequirement(v1alpha1.LabelInstancePods, v1.NodeSelectorOpIn, fmt.Sprint(pods(ctx, info, amiFamily, kc))),
		scheduling.NewRequirement(v1alpha1.LabelInstanceCategory, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceFamily, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceGeneration, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceLocalNVME, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceSize, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceGPUName, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceGPUManufacturer, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceGPUCount, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceGPUMemory, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceHypervisor, v1.NodeSelectorOpIn, aws.StringValue(info.Hypervisor)),
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
	// GPU Labels
	if info.GpuInfo != nil && len(info.GpuInfo.Gpus) == 1 {
		gpu := info.GpuInfo.Gpus[0]
		requirements.Get(v1alpha1.LabelInstanceGPUName).Insert(lowerKabobCase(aws.StringValue(gpu.Name)))
		requirements.Get(v1alpha1.LabelInstanceGPUManufacturer).Insert(lowerKabobCase(aws.StringValue(gpu.Manufacturer)))
		requirements.Get(v1alpha1.LabelInstanceGPUCount).Insert(fmt.Sprint(aws.Int64Value(gpu.Count)))
		requirements.Get(v1alpha1.LabelInstanceGPUMemory).Insert(fmt.Sprint(aws.Int64Value(gpu.MemoryInfo.SizeInMiB)))
	}
	return requirements
}

func getArchitecture(info *ec2.InstanceTypeInfo) string {
	for _, architecture := range info.ProcessorInfo.SupportedArchitectures {
		if value, ok := v1alpha1.AWSToKubeArchitectures[aws.StringValue(architecture)]; ok {
			return value
		}
	}
	return fmt.Sprint(aws.StringValueSlice(info.ProcessorInfo.SupportedArchitectures)) // Unrecognized, but used for error printing
}

func computeCapacity(ctx context.Context, info *ec2.InstanceTypeInfo, amiFamily amifamily.AMIFamily,
	blockDeviceMappings []*v1alpha1.BlockDeviceMapping, kc *v1alpha5.KubeletConfiguration) v1.ResourceList {

	return v1.ResourceList{
		v1.ResourceCPU:               *cpu(info),
		v1.ResourceMemory:            *memory(ctx, info),
		v1.ResourceEphemeralStorage:  *ephemeralStorage(amiFamily, blockDeviceMappings),
		v1.ResourcePods:              *pods(ctx, info, amiFamily, kc),
		v1alpha1.ResourceAWSPodENI:   *awsPodENI(ctx, aws.StringValue(info.InstanceType)),
		v1alpha1.ResourceNVIDIAGPU:   *nvidiaGPUs(info),
		v1alpha1.ResourceAMDGPU:      *amdGPUs(info),
		v1alpha1.ResourceAWSNeuron:   *awsNeurons(info),
		v1alpha1.ResourceHabanaGaudi: *habanaGaudis(info),
	}
}

func cpu(info *ec2.InstanceTypeInfo) *resource.Quantity {
	return resources.Quantity(fmt.Sprint(*info.VCpuInfo.DefaultVCpus))
}

func memory(ctx context.Context, info *ec2.InstanceTypeInfo) *resource.Quantity {
	mem := resources.Quantity(fmt.Sprintf("%dMi", *info.MemoryInfo.SizeInMiB))
	// Account for VM overhead in calculation
	mem.Sub(resource.MustParse(fmt.Sprintf("%dMi", int64(math.Ceil(float64(mem.Value())*awssettings.FromContext(ctx).VMMemoryOverheadPercent/1024/1024)))))
	return mem
}

// Setting ephemeral-storage to be either the default value or what is defined in blockDeviceMappings
func ephemeralStorage(amiFamily amifamily.AMIFamily, blockDeviceMappings []*v1alpha1.BlockDeviceMapping) *resource.Quantity {
	if len(blockDeviceMappings) != 0 {
		switch amiFamily.(type) {
		case *amifamily.Custom:
			return blockDeviceMappings[len(blockDeviceMappings)-1].EBS.VolumeSize
		default:
			ephemeralBlockDevice := amiFamily.EphemeralBlockDevice()
			for _, blockDevice := range blockDeviceMappings {
				// If a block device mapping exists in the provider for the root volume, set the volume size specified in the provider
				if *blockDevice.DeviceName == *ephemeralBlockDevice {
					return blockDevice.EBS.VolumeSize
				}
			}
		}
	}
	return amifamily.DefaultEBS.VolumeSize
}

func awsPodENI(ctx context.Context, name string) *resource.Quantity {
	// https://docs.aws.amazon.com/eks/latest/userguide/security-groups-for-pods.html#supported-instance-types
	limits, ok := Limits[name]
	if awssettings.FromContext(ctx).EnablePodENI && ok && limits.IsTrunkingCompatible {
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

func awsNeurons(info *ec2.InstanceTypeInfo) *resource.Quantity {
	count := int64(0)
	if info.InferenceAcceleratorInfo != nil {
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

// The number of pods per node is calculated using the formula:
// max number of ENIs * (IPv4 Addresses per ENI -1) + 2
// https://github.com/awslabs/amazon-eks-ami/blob/master/files/eni-max-pods.txt#L20
func eniLimitedPods(info *ec2.InstanceTypeInfo) *resource.Quantity {
	return resources.Quantity(fmt.Sprint(*info.NetworkInfo.MaximumNetworkInterfaces*(*info.NetworkInfo.Ipv4AddressesPerInterface-1) + 2))
}

func systemReservedResources(kc *v1alpha5.KubeletConfiguration) v1.ResourceList {
	// default system-reserved resources: https://kubernetes.io/docs/tasks/administer-cluster/reserve-compute-resources/#system-reserved
	resources := v1.ResourceList{
		v1.ResourceCPU:              resource.MustParse("100m"),
		v1.ResourceMemory:           resource.MustParse("100Mi"),
		v1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
	}
	if kc != nil && kc.SystemReserved != nil {
		return lo.Assign(resources, kc.SystemReserved)
	}
	return resources
}

func kubeReservedResources(cpus, pods, eniLimitedPods *resource.Quantity, amiFamily amifamily.AMIFamily, kc *v1alpha5.KubeletConfiguration) v1.ResourceList {
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

func evictionThreshold(memory *resource.Quantity, amiFamily amifamily.AMIFamily, kc *v1alpha5.KubeletConfiguration) v1.ResourceList {
	overhead := v1.ResourceList{
		v1.ResourceMemory: resource.MustParse("100Mi"),
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
		if v, ok := m[memoryAvailable]; ok {
			if strings.HasSuffix(v, "%") {
				p := mustParsePercentage(v)

				// Calculation is node.capacity * evictionHard[memory.available] if percentage
				// From https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#eviction-signals
				temp[v1.ResourceMemory] = resource.MustParse(fmt.Sprint(math.Ceil(float64(memory.Value()) / 100 * p)))
			} else {
				temp[v1.ResourceMemory] = resource.MustParse(v)
			}
		}
		override = resources.MaxResources(override, temp)
	}
	// Assign merges maps from left to right so overrides will always be taken last
	return lo.Assign(overhead, override)
}

func pods(ctx context.Context, info *ec2.InstanceTypeInfo, amiFamily amifamily.AMIFamily, kc *v1alpha5.KubeletConfiguration) *resource.Quantity {
	var count int64
	switch {
	case kc != nil && kc.MaxPods != nil:
		count = int64(ptr.Int32Value(kc.MaxPods))
	case !awssettings.FromContext(ctx).EnableENILimitedPodDensity:
		count = 110
	default:
		count = eniLimitedPods(info).Value()
	}
	if kc != nil && ptr.Int32Value(kc.PodsPerCore) > 0 && amiFamily.FeatureFlags().PodsPerCoreEnabled {
		count = lo.Min([]int64{int64(ptr.Int32Value(kc.PodsPerCore)) * ptr.Int64Value(info.VCpuInfo.DefaultVCpus), count})
	}
	return resources.Quantity(fmt.Sprint(count))
}

func lowerKabobCase(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, " ", "-"))
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
