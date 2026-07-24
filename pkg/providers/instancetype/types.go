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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/log"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	kubeletcel "github.com/aws/karpenter-provider-aws/pkg/cel"
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
	networkInterfaceHash, _ := hashstructure.Hash(nodeClass.NetworkInterfaces(), hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	return fmt.Sprintf(
		"%016x-%016x-%016x-%016x-%s-%s",
		kcHash,
		blockDeviceMappingsHash,
		capacityReservationHash,
		networkInterfaceHash,
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
	amiFamily := amifamily.GetAMIFamily(nodeClass.AMIFamily(), &amifamily.Options{})
	// maxPods is resolved first so that kubeReserved/systemReserved expressions see the resolved maxPods value
	// (per design: their max_pods reference is the resolved maxPods, whether from a static value, the maxPods
	// expression, or the default).
	maxPods := resolveMaxPods(ctx, info, kc.MaxPods, amiFamily, kc.PodsPerCore, nodeClass.NetworkInterfaces())
	kubeReserved := resolveResourceExpressions(ctx, info, kc.KubeReserved, amiFamily, maxPods, kc.PodsPerCore, nodeClass.NetworkInterfaces())
	systemReserved := resolveResourceExpressions(ctx, info, kc.SystemReserved, amiFamily, maxPods, kc.PodsPerCore, nodeClass.NetworkInterfaces())
	return NewInstanceType(
		ctx,
		info,
		d.region,
		zones,
		nodeClass.ZoneInfo(),
		nodeClass.BlockDeviceMappings(),
		nodeClass.InstanceStorePolicy(),
		nodeClass.NetworkInterfaces(),
		maxPods,
		kc.PodsPerCore,
		kubeReserved,
		systemReserved,
		kc.EvictionHard,
		kc.EvictionSoft,
		nodeClass.AMIFamily(),
		lo.Filter(nodeClass.CapacityReservations(), func(cr v1.CapacityReservation, _ int) bool {
			return cr.InstanceType == string(info.InstanceType)
		}),
	)
}

// EvaluateKubeletExpressions evaluates all CEL expressions in the kubelet configuration against the given
// instance type's variables and returns an error describing the first expression that fails to evaluate,
// produces a negative result, or (for maxPods) overflows int32. It is used at validation time to surface
// per-instance-type evaluation failures that a compile-only check cannot catch. A nil return means every
// expression evaluated to a usable value for this instance type.
func EvaluateKubeletExpressions(ctx context.Context, info ec2types.InstanceTypeInfo, kc *v1.KubeletConfiguration, amiFamily amifamily.AMIFamily, networkInterfaces []*v1.NetworkInterface) error {
	if kc == nil {
		return nil
	}
	// The maxPods expression evaluates against the default max_pods (it can't self-reference), while
	// kubeReserved/systemReserved evaluate against the resolved maxPods.
	maxPodsVars := buildCELVars(ctx, info, amiFamily, nil, kc.PodsPerCore, networkInterfaces)
	if err := evaluateMaxPodsExpression(kc, maxPodsVars, info); err != nil {
		return err
	}
	resolvedMaxPods := resolveMaxPods(ctx, info, kc.MaxPods, amiFamily, kc.PodsPerCore, networkInterfaces)
	reservedVars := buildCELVars(ctx, info, amiFamily, resolvedMaxPods, kc.PodsPerCore, networkInterfaces)
	return evaluateResourceExpressions(kc, reservedVars, info)
}

// evaluateMaxPodsExpression validates the maxPods CEL expression (if any) for the given instance type,
// returning an error if it fails to evaluate or falls outside the valid int32 range.
func evaluateMaxPodsExpression(kc *v1.KubeletConfiguration, celVars kubeletcel.InstanceTypeVars, info ec2types.InstanceTypeInfo) error {
	if kc.MaxPods == nil || kc.MaxPods.Type != intstr.String {
		return nil
	}
	result, err := kubeletcel.EvaluateExpression(kc.MaxPods.StrVal, celVars)
	if err != nil {
		return fmt.Errorf("evaluating maxPods expression %q for instance type %s: %w", kc.MaxPods.StrVal, info.InstanceType, err)
	}
	if result < 0 || result > math.MaxInt32 {
		return fmt.Errorf("maxPods expression %q evaluated to %d for instance type %s, which is outside the valid range [0, %d]", kc.MaxPods.StrVal, result, info.InstanceType, math.MaxInt32)
	}
	return nil
}

// evaluateResourceExpressions validates the kubeReserved and systemReserved CEL expressions for the given
// instance type, returning an error for the first expression that fails to evaluate or produces a negative value.
func evaluateResourceExpressions(kc *v1.KubeletConfiguration, celVars kubeletcel.InstanceTypeVars, info ec2types.InstanceTypeInfo) error {
	for _, resourceExpressions := range []struct {
		field string
		m     map[string]string
	}{
		{"kubeReserved", kc.KubeReserved},
		{"systemReserved", kc.SystemReserved},
	} {
		for k, v := range resourceExpressions.m {
			// Values that parse as valid Kubernetes resource quantities are used as-is, not evaluated.
			if _, qErr := resource.ParseQuantity(v); qErr == nil {
				continue
			}
			result, err := kubeletcel.EvaluateExpression(v, celVars)
			if err != nil {
				return fmt.Errorf("evaluating %s[%s] expression %q for instance type %s: %w", resourceExpressions.field, k, v, info.InstanceType, err)
			}
			if result < 0 {
				return fmt.Errorf("%s[%s] expression %q evaluated to a negative value %d for instance type %s", resourceExpressions.field, k, v, result, info.InstanceType)
			}
		}
	}
	return nil
}

// resolveMaxPods resolves the MaxPods IntOrString value to a concrete int32.
// If it's an integer, it's returned directly. If it's a string, it's evaluated as a CEL expression.
func resolveMaxPods(ctx context.Context, info ec2types.InstanceTypeInfo, maxPods *intstr.IntOrString, amiFamily amifamily.AMIFamily, podsPerCore *int32, networkInterfaces []*v1.NetworkInterface) *int32 {
	if maxPods == nil {
		return nil
	}
	switch maxPods.Type {
	case intstr.Int:
		return &maxPods.IntVal
	case intstr.String:
		// The maxPods expression can't reference its own result, so max_pods exposes the default
		celVars := buildCELVars(ctx, info, amiFamily, nil, podsPerCore, networkInterfaces)
		result, err := kubeletcel.EvaluateExpression(maxPods.StrVal, celVars)
		if err != nil {
			log.FromContext(ctx).Error(err, "failed to evaluate maxPods expression", "instanceType", info.InstanceType)
			return nil
		}
		if result < 0 || result > math.MaxInt32 {
			log.FromContext(ctx).Error(fmt.Errorf("result %d is out of range [0, %d]", result, math.MaxInt32), "maxPods expression evaluated to an invalid value", "expression", maxPods.StrVal, "instanceType", info.InstanceType)
			return nil
		}
		val := int32(result)
		return &val
	default:
		return nil
	}
}

// resolveResourceExpressions evaluates CEL expressions in a resource map (kubeReserved or systemReserved).
// Values that parse as valid Kubernetes resource quantities are left as-is.
// Values that fail to parse as quantities are evaluated as CEL expressions.
func resolveResourceExpressions(ctx context.Context, info ec2types.InstanceTypeInfo, resourceMap map[string]string, amiFamily amifamily.AMIFamily, maxPods, podsPerCore *int32, networkInterfaces []*v1.NetworkInterface) map[string]string {
	return kubeletcel.ResolveResourceMap(resourceMap, func() kubeletcel.InstanceTypeVars {
		return buildCELVars(ctx, info, amiFamily, maxPods, podsPerCore, networkInterfaces)
	}, log.FromContext(ctx))
}

// extractENILimits pulls the default ENI count and IPv4-addresses-per-ENI from the live EC2
// network info (DescribeInstanceTypes). It is the single source of truth for these values so
// that CEL evaluation during scheduling (buildCELVars) and during launch template resolution
// (via the provider's ENILimits accessor) can never diverge.
func extractENILimits(info ec2types.InstanceTypeInfo) (defaultENIs, ipsPerENI int64) {
	if info.NetworkInfo != nil && info.NetworkInfo.NetworkCards != nil && info.NetworkInfo.DefaultNetworkCardIndex != nil {
		defaultENIs = int64(lo.FromPtr(info.NetworkInfo.NetworkCards[lo.FromPtr(info.NetworkInfo.DefaultNetworkCardIndex)].MaximumNetworkInterfaces))
	}
	if info.NetworkInfo != nil && info.NetworkInfo.Ipv4AddressesPerInterface != nil {
		ipsPerENI = int64(lo.FromPtr(info.NetworkInfo.Ipv4AddressesPerInterface))
	}
	return defaultENIs, ipsPerENI
}

// buildCELVars constructs the CEL variable bindings from EC2 instance type info.
func buildCELVars(ctx context.Context, info ec2types.InstanceTypeInfo, amiFamily amifamily.AMIFamily, maxPods, podsPerCore *int32, networkInterfaces []*v1.NetworkInterface) kubeletcel.InstanceTypeVars {
	defaultENIs, ipsPerENI := extractENILimits(info)
	maxPodsVal := pods(ctx, info, amiFamily, maxPods, podsPerCore, networkInterfaces).Value()
	return kubeletcel.InstanceTypeVars{
		VCPUs:        int64(lo.FromPtr(info.VCpuInfo.DefaultVCpus)),
		MemoryMiB:    lo.FromPtr(info.MemoryInfo.SizeInMiB),
		DefaultENIs:  defaultENIs,
		IPsPerENI:    ipsPerENI,
		MaxPods:      maxPodsVal,
		InstanceType: string(info.InstanceType),
	}
}

func NewInstanceType(
	ctx context.Context,
	info ec2types.InstanceTypeInfo,
	region string,
	offeringZones []string,
	subnetZoneInfo []v1.ZoneInfo,
	blockDeviceMappings []*v1.BlockDeviceMapping,
	instanceStorePolicy *v1.InstanceStorePolicy,
	networkInterfaces []*v1.NetworkInterface,
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
		Capacity:     computeCapacity(ctx, info, amiFamily, blockDeviceMappings, instanceStorePolicy, networkInterfaces, maxPods, podsPerCore),
		Overhead: &cloudprovider.InstanceTypeOverhead{
			KubeReserved: kubeReservedResources(cpu(info), lo.Ternary(amiFamily.FeatureFlags().UsesENILimitedMemoryOverhead,
				ENILimitedPods(ctx, info, 0, networkInterfaces), pods(ctx, info, amiFamily, maxPods, podsPerCore, networkInterfaces)), kubeReserved),
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
		scheduling.NewRequirement(v1.LabelInstanceNitroEnclavesSupported, corev1.NodeSelectorOpIn, fmt.Sprint(info.NitroEnclavesSupport == ec2types.NitroEnclavesSupportSupported)),
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
		requirements.Add(scheduling.NewRequirement(v1.LabelCapacityReservationInterruptible, corev1.NodeSelectorOpIn, lo.Map(capacityReservations, func(cr v1.CapacityReservation, _ int) string {
			return fmt.Sprintf("%t", cr.Interruptible)
		})...))
	} else {
		requirements.Add(scheduling.NewRequirement(cloudprovider.ReservationIDLabel, corev1.NodeSelectorOpDoesNotExist))
		requirements.Add(scheduling.NewRequirement(v1.LabelCapacityReservationType, corev1.NodeSelectorOpDoesNotExist))
		requirements.Add(scheduling.NewRequirement(v1.LabelCapacityReservationInterruptible, corev1.NodeSelectorOpDoesNotExist))
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
	networkInterfaces []*v1.NetworkInterface, maxPods *int32, podsPerCore *int32) corev1.ResourceList {

	resourceList := corev1.ResourceList{
		corev1.ResourceCPU:              *cpu(info),
		corev1.ResourceMemory:           *memory(ctx, info),
		corev1.ResourceEphemeralStorage: *ephemeralStorage(info, amiFamily, blockDeviceMapping, instanceStorePolicy),
		corev1.ResourcePods:             *pods(ctx, info, amiFamily, maxPods, podsPerCore, networkInterfaces),
		v1.ResourceAWSPodENI:            *awsPodENI(string(info.InstanceType)),
		v1.ResourceNVIDIAGPU:            *nvidiaGPUs(info),
		v1.ResourceAMDGPU:               *amdGPUs(info),
		v1.ResourceAWSNeuron:            *awsNeuronDevices(info),
		v1.ResourceAWSNeuronCore:        *awsNeuronCores(info),
		v1.ResourceHabanaGaudi:          *habanaGaudis(info),
		v1.ResourceEFA:                  *efas(info, networkInterfaces),
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

func efas(info ec2types.InstanceTypeInfo, networkInterfaces []*v1.NetworkInterface) *resource.Quantity {
	// If the network interface field is specified on the NodeClass, this overrides the EFAs that the instance type supports.
	count := 0
	if networkInterfaces != nil {
		count = lo.CountBy(networkInterfaces, func(nic *v1.NetworkInterface) bool {
			return nic.InterfaceType == v1.InterfaceTypeEFAOnly
		})
	} else if info.NetworkInfo != nil && info.NetworkInfo.EfaInfo != nil && info.NetworkInfo.EfaInfo.MaximumEfaInterfaces != nil {
		count = int(*info.NetworkInfo.EfaInfo.MaximumEfaInterfaces)
	}
	return resources.Quantity(fmt.Sprint(count))
}

func ENILimitedPods(ctx context.Context, info ec2types.InstanceTypeInfo, reservedENIs int, ncNetworkInterfaces []*v1.NetworkInterface) *resource.Quantity {
	// The number of pods per node is calculated using the formula:
	// max number of ENIs * (IPv4 Addresses per ENI -1) + 2
	// https://github.com/awslabs/amazon-eks-ami/blob/main/templates/shared/runtime/eni-max-pods.txt

	// VPC CNI only uses the default network interface
	// https://github.com/aws/amazon-vpc-cni-k8s/blob/3294231c0dce52cfe473bf6c62f47956a3b333b6/scripts/gen_vpc_ip_limits.go#L162
	networkInterfaces := *info.NetworkInfo.NetworkCards[*info.NetworkInfo.DefaultNetworkCardIndex].MaximumNetworkInterfaces

	// EFA-only interfaces consume an ENI and don't support IP networking
	numEFAOnly := lo.CountBy(ncNetworkInterfaces, func(n *v1.NetworkInterface) bool {
		return n.NetworkCardIndex == 0 && n.InterfaceType == v1.InterfaceType(ec2types.NetworkInterfaceTypeEfaOnly)
	})

	usableNetworkInterfaces := lo.Max([]int64{int64(int(networkInterfaces) - reservedENIs - numEFAOnly), 0})
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

func pods(ctx context.Context, info ec2types.InstanceTypeInfo, amiFamily amifamily.AMIFamily, maxPods *int32, podsPerCore *int32, ncNetworkInterfaces []*v1.NetworkInterface) *resource.Quantity {
	var count int64
	switch {
	case maxPods != nil:
		count = int64(lo.FromPtr(maxPods))
	case amiFamily.FeatureFlags().SupportsENILimitedPodDensity:
		count = ENILimitedPods(ctx, info, options.FromContext(ctx).ReservedENIs, ncNetworkInterfaces).Value()
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
