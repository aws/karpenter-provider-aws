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

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/scheduling"
	"github.com/aws/karpenter-core/pkg/utils/resources"
)

const (
	memoryAvailable = "memory.available"
)

var (
	_                  cloudprovider.InstanceType = (*InstanceType)(nil)
	instanceTypeScheme                            = regexp.MustCompile(`(^[a-z]+)(\-[0-9]+tb)?([0-9]+).*\.`)
)

type InstanceType struct {
	*ec2.InstanceTypeInfo
	offerings    []cloudprovider.Offering
	overhead     v1.ResourceList
	requirements scheduling.Requirements
	resources    v1.ResourceList
	provider     *v1alpha1.AWS
	maxPods      *int64
	region       string
}

func NewInstanceType(ctx context.Context, info *ec2.InstanceTypeInfo, kc *v1alpha5.KubeletConfiguration, region string, provider *v1alpha1.AWS, offerings []cloudprovider.Offering) *InstanceType {
	instanceType := &InstanceType{
		InstanceTypeInfo: info,
		provider:         provider,
		offerings:        offerings,
		region:           region,
	}
	instanceType.maxPods = instanceType.computeMaxPods(ctx, kc)

	// Precompute to minimize memory/compute overhead
	instanceType.resources = instanceType.computeResources(awssettings.FromContext(ctx).EnablePodENI)
	instanceType.overhead = instanceType.computeOverhead(awssettings.FromContext(ctx).VMMemoryOverheadPercent, kc)
	instanceType.requirements = instanceType.computeRequirements()
	return instanceType
}

func (i *InstanceType) Name() string {
	return aws.StringValue(i.InstanceType)
}

func (i *InstanceType) Requirements() scheduling.Requirements {
	return i.requirements
}

func (i *InstanceType) Offerings() []cloudprovider.Offering {
	return i.offerings
}

func (i *InstanceType) Resources() v1.ResourceList {
	return i.resources
}

// Overhead computes overhead for https://kubernetes.io/docs/tasks/administer-cluster/reserve-compute-resources/#node-allocatable
// using calculations copied from https://github.com/bottlerocket-os/bottlerocket#kubernetes-settings.
func (i *InstanceType) Overhead() v1.ResourceList {
	return i.overhead
}

func (i *InstanceType) computeRequirements() scheduling.Requirements {
	requirements := scheduling.NewRequirements(
		// Well Known Upstream
		scheduling.NewRequirement(v1.LabelInstanceTypeStable, v1.NodeSelectorOpIn, i.Name()),
		scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, i.architecture()),
		scheduling.NewRequirement(v1.LabelOSStable, v1.NodeSelectorOpIn, string(v1.Linux), string(v1.Windows)),
		scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, lo.Map(cloudprovider.AvailableOfferings(i), func(o cloudprovider.Offering, _ int) string { return o.Zone })...),
		scheduling.NewRequirement(v1.LabelTopologyRegion, v1.NodeSelectorOpIn, i.region),
		// Well Known to Karpenter
		scheduling.NewRequirement(v1alpha5.LabelCapacityType, v1.NodeSelectorOpIn, lo.Map(cloudprovider.AvailableOfferings(i), func(o cloudprovider.Offering, _ int) string { return o.CapacityType })...),
		// Well Known to AWS
		scheduling.NewRequirement(v1alpha1.LabelInstanceCPU, v1.NodeSelectorOpIn, fmt.Sprint(aws.Int64Value(i.VCpuInfo.DefaultVCpus))),
		scheduling.NewRequirement(v1alpha1.LabelInstanceMemory, v1.NodeSelectorOpIn, fmt.Sprint(aws.Int64Value(i.MemoryInfo.SizeInMiB))),
		scheduling.NewRequirement(v1alpha1.LabelInstancePods, v1.NodeSelectorOpIn, fmt.Sprint(i.pods().Value())),
		scheduling.NewRequirement(v1alpha1.LabelInstanceCategory, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceFamily, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceGeneration, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceLocalNVME, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceSize, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceGPUName, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceGPUManufacturer, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceGPUCount, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceGPUMemory, v1.NodeSelectorOpDoesNotExist),
		scheduling.NewRequirement(v1alpha1.LabelInstanceHypervisor, v1.NodeSelectorOpIn, aws.StringValue(i.Hypervisor)),
	)
	// Instance Type Labels
	instanceFamilyParts := instanceTypeScheme.FindStringSubmatch(aws.StringValue(i.InstanceType))
	if len(instanceFamilyParts) == 4 {
		requirements[v1alpha1.LabelInstanceCategory].Insert(instanceFamilyParts[1])
		requirements[v1alpha1.LabelInstanceGeneration].Insert(instanceFamilyParts[3])
	}
	instanceTypeParts := strings.Split(aws.StringValue(i.InstanceType), ".")
	if len(instanceTypeParts) == 2 {
		requirements.Get(v1alpha1.LabelInstanceFamily).Insert(instanceTypeParts[0])
		requirements.Get(v1alpha1.LabelInstanceSize).Insert(instanceTypeParts[1])
	}
	if i.InstanceStorageInfo != nil && aws.StringValue(i.InstanceStorageInfo.NvmeSupport) != ec2.EphemeralNvmeSupportUnsupported {
		requirements[v1alpha1.LabelInstanceLocalNVME].Insert(fmt.Sprint(aws.Int64Value(i.InstanceStorageInfo.TotalSizeInGB)))
	}
	// GPU Labels
	if i.GpuInfo != nil && len(i.GpuInfo.Gpus) == 1 {
		gpu := i.GpuInfo.Gpus[0]
		requirements.Get(v1alpha1.LabelInstanceGPUName).Insert(lowerKabobCase(aws.StringValue(gpu.Name)))
		requirements.Get(v1alpha1.LabelInstanceGPUManufacturer).Insert(lowerKabobCase(aws.StringValue(gpu.Manufacturer)))
		requirements.Get(v1alpha1.LabelInstanceGPUCount).Insert(fmt.Sprint(aws.Int64Value(gpu.Count)))
		requirements.Get(v1alpha1.LabelInstanceGPUMemory).Insert(fmt.Sprint(aws.Int64Value(gpu.MemoryInfo.SizeInMiB)))
	}
	return requirements
}

func (i *InstanceType) architecture() string {
	for _, architecture := range i.ProcessorInfo.SupportedArchitectures {
		if value, ok := v1alpha1.AWSToKubeArchitectures[aws.StringValue(architecture)]; ok {
			return value
		}
	}
	return fmt.Sprint(aws.StringValueSlice(i.ProcessorInfo.SupportedArchitectures)) // Unrecognized, but used for error printing
}

func (i *InstanceType) computeResources(enablePodENI bool) v1.ResourceList {
	return v1.ResourceList{
		v1.ResourceCPU:                  *i.cpu(),
		v1.ResourceMemory:               *i.memory(),
		v1.ResourceEphemeralStorage:     *i.ephemeralStorage(),
		v1.ResourcePods:                 *i.pods(),
		v1alpha1.ResourceAWSPodENI:      *i.awsPodENI(enablePodENI),
		v1alpha1.ResourceAWSPrivateIPv4: *i.awsPodPrivateIPv4(),
		v1alpha1.ResourceNVIDIAGPU:      *i.nvidiaGPUs(),
		v1alpha1.ResourceAMDGPU:         *i.amdGPUs(),
		v1alpha1.ResourceAWSNeuron:      *i.awsNeurons(),
	}
}

func (i *InstanceType) cpu() *resource.Quantity {
	return resources.Quantity(fmt.Sprint(*i.VCpuInfo.DefaultVCpus))
}

func (i *InstanceType) memory() *resource.Quantity {
	return resources.Quantity(
		fmt.Sprintf("%dMi", *i.MemoryInfo.SizeInMiB),
	)
}

// Setting ephemeral-storage to be either the default value or what is defined in blockDeviceMappings
func (i *InstanceType) ephemeralStorage() *resource.Quantity {
	if len(i.provider.BlockDeviceMappings) != 0 {
		if aws.StringValue(i.provider.AMIFamily) == v1alpha1.AMIFamilyCustom {
			// For Custom AMIFamily, use the volume size of the last defined block device mapping.
			// TODO: Consider giving better control to define which block device will be used for pods.
			return i.provider.BlockDeviceMappings[len(i.provider.BlockDeviceMappings)-1].EBS.VolumeSize
		}
		ephemeralBlockDevice := amifamily.GetAMIFamily(i.provider.AMIFamily, &amifamily.Options{}).EphemeralBlockDevice()
		for _, blockDevice := range i.provider.BlockDeviceMappings {
			// If a block device mapping exists in the provider for the root volume, set the volume size specified in the provider
			if *blockDevice.DeviceName == *ephemeralBlockDevice {
				return blockDevice.EBS.VolumeSize
			}
		}
	}
	return amifamily.DefaultEBS.VolumeSize
}

func (i *InstanceType) pods() *resource.Quantity {
	return resources.Quantity(fmt.Sprint(ptr.Int64Value(i.maxPods)))
}

func (i *InstanceType) awsPodENI(enablePodENI bool) *resource.Quantity {
	// https://docs.aws.amazon.com/eks/latest/userguide/security-groups-for-pods.html#supported-instance-types
	limits, ok := Limits[aws.StringValue(i.InstanceType)]
	if enablePodENI && ok && limits.IsTrunkingCompatible {
		return resources.Quantity(fmt.Sprint(limits.BranchInterface))
	}
	return resources.Quantity("0")
}

func (i *InstanceType) awsPodPrivateIPv4() *resource.Quantity {
	// Resource automatically added to Windows resources by the AWS VPC Resource Controller:
	// https://github.com/aws/amazon-vpc-resource-controller-k8s/blob/master/docs/windows/workflow.md
	limits, ok := Limits[aws.StringValue(i.InstanceType)]
	if ok {
		return resources.Quantity(fmt.Sprint(limits.IPv4PerInterface - 1))
	}
	return resources.Quantity("0")
}

func (i *InstanceType) nvidiaGPUs() *resource.Quantity {
	count := int64(0)
	if i.GpuInfo != nil {
		for _, gpu := range i.GpuInfo.Gpus {
			if *gpu.Manufacturer == "NVIDIA" {
				count += *gpu.Count
			}
		}
	}
	return resources.Quantity(fmt.Sprint(count))
}

func (i *InstanceType) amdGPUs() *resource.Quantity {
	count := int64(0)
	if i.GpuInfo != nil {
		for _, gpu := range i.GpuInfo.Gpus {
			if *gpu.Manufacturer == "AMD" {
				count += *gpu.Count
			}
		}
	}
	return resources.Quantity(fmt.Sprint(count))
}

func (i *InstanceType) awsNeurons() *resource.Quantity {
	count := int64(0)
	if i.InferenceAcceleratorInfo != nil {
		for _, accelerator := range i.InferenceAcceleratorInfo.Accelerators {
			count += *accelerator.Count
		}
	}
	return resources.Quantity(fmt.Sprint(count))
}

func (i *InstanceType) computeOverhead(vmMemOverhead float64, kc *v1alpha5.KubeletConfiguration) v1.ResourceList {
	srr := i.systemReservedResources(kc)
	krr := i.kubeReservedResources(kc)
	misc := i.miscResources(vmMemOverhead)
	et := i.evictionThreshold(kc, misc[v1.ResourceMemory])
	overhead := resources.Merge(srr, krr, et, misc)

	return overhead
}

// The number of pods per node is calculated using the formula:
// max number of ENIs * (IPv4 Addresses per ENI -1) + 2
// https://github.com/awslabs/amazon-eks-ami/blob/master/files/eni-max-pods.txt#L20
func (i *InstanceType) eniLimitedPods() int64 {
	return *i.NetworkInfo.MaximumNetworkInterfaces*(*i.NetworkInfo.Ipv4AddressesPerInterface-1) + 2
}

func (i *InstanceType) systemReservedResources(kc *v1alpha5.KubeletConfiguration) v1.ResourceList {
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

func (i *InstanceType) kubeReservedResources(kc *v1alpha5.KubeletConfiguration) v1.ResourceList {
	// We reserve memory based off of --max-pods unless we are using the EKS-optimized AMI
	// which relies on ENI-limited pod density regardless of the --max-pods value
	// https://github.com/awslabs/amazon-eks-ami/issues/782
	amiFamily := amifamily.GetAMIFamily(i.provider.AMIFamily, &amifamily.Options{})
	pods := i.pods().Value()
	if amiFamily.FeatureFlags().UsesENILimitedMemoryOverhead {
		pods = i.eniLimitedPods()
	}

	resources := v1.ResourceList{
		v1.ResourceMemory:           resource.MustParse(fmt.Sprintf("%dMi", (11*pods)+255)),
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
		cpuSt := i.cpu()
		if cpu := cpuSt.MilliValue(); cpu >= cpuRange.start {
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

func (i *InstanceType) evictionThreshold(kc *v1alpha5.KubeletConfiguration, vmMemoryOverhead resource.Quantity) v1.ResourceList {
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
	if kc.EvictionSoft != nil && i.amiFamily().FeatureFlags().EvictionSoftEnabled {
		evictionSignals = append(evictionSignals, kc.EvictionSoft)
	}

	for _, m := range evictionSignals {
		temp := v1.ResourceList{}
		if v, ok := m[memoryAvailable]; ok {
			if strings.HasSuffix(v, "%") {
				p := mustParsePercentage(v)

				// Calculation is node.capacity * evictionHard[memory.available] if percentage
				// From https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#eviction-signals
				totalAllocatable := i.resources.Memory().DeepCopy()
				totalAllocatable.Sub(vmMemoryOverhead)
				temp[v1.ResourceMemory] = resource.MustParse(fmt.Sprint(math.Ceil(float64(totalAllocatable.Value()) / 100 * p)))
			} else {
				temp[v1.ResourceMemory] = resource.MustParse(v)
			}
		}
		override = resources.MaxResources(override, temp)
	}
	// Assign merges maps from left to right so overrides will always be taken last
	return lo.Assign(overhead, override)
}

func (i *InstanceType) miscResources(overheadPercentage float64) v1.ResourceList {
	memory := i.memory().Value()
	return v1.ResourceList{
		v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", int64(math.Ceil(float64(memory)*overheadPercentage/1024/1024)))),
	}
}

func (i *InstanceType) computeMaxPods(ctx context.Context, kc *v1alpha5.KubeletConfiguration) *int64 {
	var mp *int64

	switch {
	case kc != nil && kc.MaxPods != nil:
		mp = ptr.Int64(int64(ptr.Int32Value(kc.MaxPods)))
	case !awssettings.FromContext(ctx).EnableENILimitedPodDensity:
		mp = ptr.Int64(110)
	default:
		mp = ptr.Int64(i.eniLimitedPods())
	}

	if kc != nil && ptr.Int32Value(kc.PodsPerCore) > 0 && i.amiFamily().FeatureFlags().PodsPerCoreEnabled {
		mp = ptr.Int64(lo.Min([]int64{int64(ptr.Int32Value(kc.PodsPerCore)) * ptr.Int64Value(i.VCpuInfo.DefaultVCpus), ptr.Int64Value(mp)}))
	}
	return mp
}

func (i *InstanceType) amiFamily() amifamily.AMIFamily {
	return amifamily.GetAMIFamily(i.provider.AMIFamily, &amifamily.Options{})
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
