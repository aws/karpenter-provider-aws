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

package amifamily

import (
	"context"
	"fmt"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/log"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	kubeletcel "github.com/aws/karpenter-provider-aws/pkg/cel"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap"
	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"
)

var DefaultEBS = v1.BlockDevice{
	Encrypted:  aws.Bool(true),
	VolumeType: aws.String(string(ec2types.VolumeTypeGp3)),
	VolumeSize: lo.ToPtr(resource.MustParse("20Gi")),
}

type Resolver interface {
	Resolve(*v1.EC2NodeClass, *karpv1.NodeClaim, []*cloudprovider.InstanceType, string, string, *Options, string, int32) ([]*LaunchTemplate, error)
}

// ENILimits holds ENI networking limits for an instance type.
type ENILimits struct {
	DefaultENIs int
	IPv4PerENI  int
}

// ENILookup is a function that returns ENI limits for a given instance type name.
type ENILookup func(instanceTypeName string) (ENILimits, bool)

// DefaultResolver is able to fill-in dynamic launch template parameters
type DefaultResolver struct {
	region    string
	eniLookup ENILookup
}

// Options define the static launch template parameters
type Options struct {
	ClusterName         string
	ClusterEndpoint     string
	ClusterCIDR         *string
	InstanceProfile     string
	CABundle            *string `hash:"ignore"`
	InstanceStorePolicy *v1.InstanceStorePolicy
	AMISelectorTerms    []v1.AMISelectorTerm `hash:"ignore"` // For Bottlerocket version resolution
	AMIs                []v1.AMI             `hash:"ignore"` // Resolved AMIs for version extraction
	// Level-triggered fields that may change out of sync.
	SecurityGroups            []v1.SecurityGroup
	Tags                      map[string]string
	Labels                    map[string]string `hash:"ignore"`
	KubeDNSIP                 net.IP
	AssociatePublicIPAddress  *bool
	IPPrefixCount             *int32
	NodeClassName             string
	ResolvedNetworkInterfaces []*ResolvedNetworkInterface `hash:"ignore"`
}

// LaunchTemplate holds the dynamically generated launch template parameters
type LaunchTemplate struct {
	*Options
	UserData                         bootstrap.Bootstrapper
	BlockDeviceMappings              []*v1.BlockDeviceMapping
	MetadataOptions                  *v1.MetadataOptions
	CPUOptions                       *v1.CPUOptions
	AMIID                            string
	InstanceTypes                    []*cloudprovider.InstanceType `hash:"ignore"`
	DetailedMonitoring               bool
	EFACount                         int
	EnclaveEnabled                   bool
	NetworkInterfaces                []*ResolvedNetworkInterface
	CapacityType                     string
	CapacityReservationID            string
	CapacityReservationType          v1.CapacityReservationType
	CapacityReservationInterruptible bool
	Tenancy                          string
	PlacementGroupID                 string
	PlacementGroupPartition          int32
	// Zone constrains fleet overrides to a single AZ when set.
	Zone               string `hash:"ignore"`
	ConnectionTracking *v1.ConnectionTracking
}

// AMIFamily can be implemented to override the default logic for generating dynamic launch template parameters
type AMIFamily interface {
	DescribeImageQuery(ctx context.Context, ssmProvider ssm.Provider, k8sVersion string, amiVersion string) (DescribeImageQuery, error)
	UserData(kubeletConfig *v1.KubeletConfiguration, taints []corev1.Taint, labels map[string]string, caBundle *string, instanceTypes []*cloudprovider.InstanceType, customUserData *string, instanceStorePolicy *v1.InstanceStorePolicy) bootstrap.Bootstrapper
	DefaultBlockDeviceMappings() []*v1.BlockDeviceMapping
	DefaultMetadataOptions() *v1.MetadataOptions
	EphemeralBlockDevice() *string
	FeatureFlags() FeatureFlags
}

type DefaultAMIOutput struct {
	Query        string
	Requirements scheduling.Requirements
}

// FeatureFlags describes whether the features below are enabled for a given AMIFamily
type FeatureFlags struct {
	UsesENILimitedMemoryOverhead bool
	PodsPerCoreEnabled           bool
	EvictionSoftEnabled          bool
	SupportsENILimitedPodDensity bool
}

// DefaultFamily provides default values for AMIFamilies that compose it
type DefaultFamily struct{}

func (d DefaultFamily) FeatureFlags() FeatureFlags {
	return FeatureFlags{
		UsesENILimitedMemoryOverhead: true,
		PodsPerCoreEnabled:           true,
		EvictionSoftEnabled:          true,
		SupportsENILimitedPodDensity: true,
	}
}

// NewDefaultResolver constructs a new launch template DefaultResolver
func NewDefaultResolver(region string, eniLookup ENILookup) *DefaultResolver {
	return &DefaultResolver{
		region:    region,
		eniLookup: eniLookup,
	}
}

// Resolve generates launch templates using the static options and dynamically generates launch template parameters.
// Multiple ResolvedTemplates are returned based on the instanceTypes passed in to support special AMIs for certain instance types like GPUs.
//
//nolint:gocyclo
func (r DefaultResolver) Resolve(nodeClass *v1.EC2NodeClass, nodeClaim *karpv1.NodeClaim, instanceTypes []*cloudprovider.InstanceType, capacityType string, tenancyType string, options *Options, placementGroupID string, placementGroupPartition int32) ([]*LaunchTemplate, error) {
	amiFamily := GetAMIFamily(nodeClass.AMIFamily(), options)
	if len(nodeClass.Status.AMIs) == 0 {
		return nil, fmt.Errorf("no amis exist given constraints")
	}
	mappedAMIs := MapToInstanceTypes(instanceTypes, nodeClass.Status.AMIs)
	if len(mappedAMIs) == 0 {
		return nil, fmt.Errorf("no instance types satisfy requirements of amis %v", lo.Uniq(lo.Map(nodeClass.Status.AMIs, func(a v1.AMI, _ int) string { return a.ID })))
	}
	var resolvedTemplates []*LaunchTemplate
	for amiID, instanceTypes := range mappedAMIs {
		// In order to support reserved ENIs for CNI custom networking setups,
		// we need to pass down the max-pods calculation to the kubelet.
		// This requires that we resolve a unique launch template per max-pods value.
		// Similarly, instance types configured with EFAs require unique launch templates depending on the number of
		// EFAs they support.
		// Reservations IDs are also included since we need to create a separate LaunchTemplate per reservation ID when
		// launching reserved capacity. If it's a reserved capacity launch, we've already filtered the instance types
		// further up the call stack.
		type launchTemplateParams struct {
			efaCount int
			maxPods  int
			// resolvedKubeReserved and resolvedSystemReserved hold the evaluated resource values
			// (serialized as "key1=val1,key2=val2" for comparability) when CEL expressions are used.
			resolvedKubeReserved   string
			resolvedSystemReserved string
			// reservationIDs is encoded as a string rather than a slice to ensure this type is comparable for use by `lo.GroupBy`.
			reservationIDs           string
			reservationType          v1.CapacityReservationType
			reservationInterruptible bool
		}
		paramsToInstanceTypes := lo.GroupBy(instanceTypes, func(it *cloudprovider.InstanceType) launchTemplateParams {
			var reservationType v1.CapacityReservationType
			var reservationInterruptible bool
			var reservationIDs []string
			if capacityType == karpv1.CapacityTypeReserved {
				for _, o := range it.Offerings {
					if o.CapacityType() != karpv1.CapacityTypeReserved {
						continue
					}
					reservationIDs = append(reservationIDs, o.ReservationID())
					// Offerings are prefiltered such that there is only a single reservation type
					if reservationType == "" {
						reservationType = v1.CapacityReservationType(o.Requirements.Get(v1.LabelCapacityReservationType).Any())
						reservationInterruptible = o.Requirements.Get(v1.LabelCapacityReservationInterruptible).Any() == "true"
					}
				}
			}
			var kubeReserved, systemReserved map[string]string
			if nodeClass.Spec.Kubelet != nil {
				kubeReserved = nodeClass.Spec.Kubelet.KubeReserved
				systemReserved = nodeClass.Spec.Kubelet.SystemReserved
			}
			// kubeReserved and systemReserved are resolved through the same shared CEL evaluation path
			// (kubeletcel.ResolveResourceMap) against the same live-EC2-backed ENI lookup used by the
			// scheduler, so the launch template configures exactly what the scheduler reserved.
			resolvedKubeReserved := resolveResourceExpressionsForLaunchTemplate(kubeReserved, it, r.eniLookup)
			resolvedSystemReserved := resolveResourceExpressionsForLaunchTemplate(systemReserved, it, r.eniLookup)
			return launchTemplateParams{
				efaCount: lo.Ternary(
					lo.Contains(lo.Keys(nodeClaim.Spec.Resources.Requests), v1.ResourceEFA),
					int(lo.ToPtr(it.Capacity[v1.ResourceEFA]).Value()),
					0,
				),
				maxPods:                int(it.Capacity.Pods().Value()),
				resolvedKubeReserved:   serializeResourceMap(resolvedKubeReserved),
				resolvedSystemReserved: serializeResourceMap(resolvedSystemReserved),
				// If we're dealing with reserved instances, there's only going to be a single instance per group. This invariant
				// is due to reservation IDs not being shared across instance types. Because of this, we don't need to worry about
				// ordering in this string.
				reservationIDs:           strings.Join(reservationIDs, ","),
				reservationType:          reservationType,
				reservationInterruptible: reservationInterruptible,
			}
		})

		for params, instanceTypes := range paramsToInstanceTypes {
			reservationIDs := strings.Split(params.reservationIDs, ",")
			resolvedTemplates = append(resolvedTemplates, r.resolveLaunchTemplates(nodeClass, nodeClaim, instanceTypes, capacityType, amiFamily, amiID, params.maxPods, params.efaCount, reservationIDs, params.reservationType, params.reservationInterruptible, options, tenancyType, placementGroupID, placementGroupPartition, deserializeResourceMap(params.resolvedKubeReserved), deserializeResourceMap(params.resolvedSystemReserved))...)
		}
	}
	return resolvedTemplates, nil
}

func GetAMIFamily(amiFamily string, options *Options) AMIFamily {
	switch amiFamily {
	case v1.AMIFamilyBottlerocket:
		return &Bottlerocket{Options: options}
	case v1.AMIFamilyWindows2019:
		return &Windows{Options: options, Version: v1.Windows2019, Build: v1.Windows2019Build}
	case v1.AMIFamilyWindows2022:
		return &Windows{Options: options, Version: v1.Windows2022, Build: v1.Windows2022Build}
	case v1.AMIFamilyWindows2025:
		return &Windows{Options: options, Version: v1.Windows2025, Build: v1.Windows2025Build}
	case v1.AMIFamilyCustom:
		return &Custom{Options: options}
	case v1.AMIFamilyAL2023:
		return &AL2023{Options: options}
	default:
		return &AL2{Options: options}
	}
}

func (o Options) DefaultMetadataOptions() *v1.MetadataOptions {
	return &v1.MetadataOptions{
		HTTPEndpoint:            aws.String(string(ec2types.InstanceMetadataEndpointStateDisabled)),
		HTTPProtocolIPv6:        aws.String(lo.Ternary(o.KubeDNSIP == nil || o.KubeDNSIP.To4() != nil, string(ec2types.LaunchTemplateInstanceMetadataProtocolIpv6Disabled), string(ec2types.LaunchTemplateInstanceMetadataProtocolIpv6Enabled))),
		HTTPPutResponseHopLimit: aws.Int64(2),
		HTTPTokens:              aws.String(string(ec2types.LaunchTemplateHttpTokensStateRequired)),
	}
}

func (r DefaultResolver) defaultClusterDNS(opts *Options, kubeletConfig *v1.KubeletConfiguration) *v1.KubeletConfiguration {
	if opts.KubeDNSIP == nil {
		return kubeletConfig
	}
	if kubeletConfig != nil && len(kubeletConfig.ClusterDNS) != 0 {
		return kubeletConfig
	}
	if kubeletConfig == nil {
		return &v1.KubeletConfiguration{
			ClusterDNS: []string{opts.KubeDNSIP.String()},
		}
	}
	newKubeletConfig := kubeletConfig.DeepCopy()
	newKubeletConfig.ClusterDNS = []string{opts.KubeDNSIP.String()}
	return newKubeletConfig
}

//nolint:gocyclo
func (r DefaultResolver) resolveLaunchTemplates(
	nodeClass *v1.EC2NodeClass,
	nodeClaim *karpv1.NodeClaim,
	instanceTypes []*cloudprovider.InstanceType,
	capacityType string,
	amiFamily AMIFamily,
	amiID string,
	maxPods int,
	efaCount int,
	capacityReservationIDs []string,
	capacityReservationType v1.CapacityReservationType,
	capacityReservationInterruptible bool,
	options *Options,
	tenancyType string,
	placementGroupID string,
	placementGroupPartition int32,
	resolvedKubeReserved map[string]string,
	resolvedSystemReserved map[string]string,
) []*LaunchTemplate {
	kubeletConfig := &v1.KubeletConfiguration{}
	if nodeClass.Spec.Kubelet != nil {
		kubeletConfig = nodeClass.Spec.Kubelet.DeepCopy()
	}
	maxPodsInt32 := int32(min(maxPods, math.MaxInt32)) //nolint:gosec,G115 // maxPods is bounded by Kubernetes pod limits
	if kubeletConfig.MaxPods == nil {
		kubeletConfig.MaxPods = lo.ToPtr(intstr.FromInt32(maxPodsInt32))
	} else if kubeletConfig.MaxPods.Type == intstr.String {
		kubeletConfig.MaxPods = lo.ToPtr(intstr.FromInt32(maxPodsInt32))
	}
	// Use resolved values for kubeReserved/systemReserved when expressions were evaluated
	if resolvedKubeReserved != nil {
		kubeletConfig.KubeReserved = resolvedKubeReserved
	}
	if resolvedSystemReserved != nil {
		kubeletConfig.SystemReserved = resolvedSystemReserved
	}
	taints := lo.Flatten([][]corev1.Taint{
		nodeClaim.Spec.Taints,
		nodeClaim.Spec.StartupTaints,
	})
	if _, found := lo.Find(taints, func(t corev1.Taint) bool {
		return t.MatchTaint(&karpv1.UnregisteredNoExecuteTaint)
	}); !found {
		taints = append(taints, karpv1.UnregisteredNoExecuteTaint)
	}
	// If no reservation IDs are provided, insert an empty string so the end result is a single launch template with no
	// associated capacity reservation.
	// TODO: We can simplify this by creating an initial lt, and then copying it for each cr. However, this requires a deep
	// copy of the LT struct, which contains an interface causing problems for deepcopy-gen. See review comment for context:
	// https://github.com/aws/karpenter-provider-aws/pull/7726#discussion_r1955280055
	if len(capacityReservationIDs) == 0 {
		capacityReservationIDs = append(capacityReservationIDs, "")
	}
	httpProtocolUnsupportedRegions := sets.New(
		"us-iso-east-1",
		"us-iso-west-1",
		"us-isob-east-1",
		"us-isob-west-1",
		"us-isof-south-1",
		"us-isof-east-1",
	)
	return lo.Map(capacityReservationIDs, func(id string, _ int) *LaunchTemplate {
		resolved := &LaunchTemplate{
			Options: options,
			UserData: amiFamily.UserData(
				r.defaultClusterDNS(options, kubeletConfig),
				taints,
				RejectForbiddenLabels(options.Labels),
				options.CABundle,
				instanceTypes,
				nodeClass.Spec.UserData,
				options.InstanceStorePolicy,
			),
			BlockDeviceMappings:              nodeClass.Spec.BlockDeviceMappings,
			MetadataOptions:                  nodeClass.Spec.MetadataOptions,
			CPUOptions:                       nodeClass.Spec.CPUOptions,
			DetailedMonitoring:               aws.ToBool(nodeClass.Spec.DetailedMonitoring),
			AMIID:                            amiID,
			InstanceTypes:                    instanceTypes,
			EFACount:                         efaCount,
			NetworkInterfaces:                ResolveNetworkInterfaces(nodeClass.Spec.NetworkInterfaces),
			CapacityType:                     capacityType,
			CapacityReservationID:            id,
			CapacityReservationType:          capacityReservationType,
			CapacityReservationInterruptible: capacityReservationInterruptible,
			Tenancy:                          tenancyType,
			PlacementGroupID:                 placementGroupID,
			PlacementGroupPartition:          placementGroupPartition,
			EnclaveEnabled:                   lo.Contains(lo.Keys(nodeClaim.Spec.Resources.Requests), v1.ResourceNitroSandbox),
			ConnectionTracking:               nodeClass.Spec.ConnectionTracking,
		}
		if len(resolved.BlockDeviceMappings) == 0 {
			resolved.BlockDeviceMappings = amiFamily.DefaultBlockDeviceMappings()
		}
		if resolved.MetadataOptions == nil {
			resolved.MetadataOptions = amiFamily.DefaultMetadataOptions()
		}
		if httpProtocolUnsupportedRegions.Has(r.region) {
			resolved.MetadataOptions.HTTPProtocolIPv6 = nil
		}
		return resolved
	})
}

// RejectForbiddenLabels rejects any label from the provided set that would be blocked during node admission.
// Ref: https://github.com/kubernetes/kubernetes/blob/8d450ef773127374148abad4daaf28dac6cb2625/plugin/pkg/admission/noderestriction/admission.go#L520-L525
func RejectForbiddenLabels(labels map[string]string) map[string]string {
	filteredLabels := make(map[string]string, len(labels))
	for label, value := range labels {
		if isRestrictedLabel(label) {
			continue
		}
		filteredLabels[label] = value
	}
	return filteredLabels
}

func isRestrictedLabel(label string) bool {
	domain := karpv1.GetLabelDomain(label)
	for _, restrictedDomain := range []string{
		corev1.LabelNamespaceNodeRestriction,
		"kubernetes.io",
		"k8s.io",
	} {
		if domain == restrictedDomain || strings.HasSuffix(domain, "."+restrictedDomain) {
			return true
		}
	}
	return false
}

// resolveResourceExpressionsForLaunchTemplate evaluates CEL expressions in a kubeReserved/systemReserved
// resource map using instance type properties. Values that parse as valid Kubernetes resource quantities
// are left unchanged. It delegates to the shared kubeletcel.ResolveResourceMap so the launch template uses
// the exact same evaluation logic as the scheduler; the only difference is the ENI data source, which is
// unified to live EC2 info via eniLookup.
func resolveResourceExpressionsForLaunchTemplate(resourceMap map[string]string, it *cloudprovider.InstanceType, eniLookup ENILookup) map[string]string {
	return kubeletcel.ResolveResourceMap(resourceMap, func() kubeletcel.InstanceTypeVars {
		return celVarsFromInstanceType(it, eniLookup)
	}, log.Log)
}

// celVarsFromInstanceType builds CEL evaluation variables from a cloudprovider.InstanceType
// using its requirements (CPU, memory labels) and an ENI lookup function.
func celVarsFromInstanceType(it *cloudprovider.InstanceType, eniLookup ENILookup) kubeletcel.InstanceTypeVars {
	vcpus := int64(0)
	if req := it.Requirements.Get(v1.LabelInstanceCPU); req != nil {
		if val, err := strconv.ParseInt(req.Any(), 10, 64); err == nil {
			vcpus = val
		}
	}
	memoryMiB := int64(0)
	if req := it.Requirements.Get(v1.LabelInstanceMemory); req != nil {
		if val, err := strconv.ParseInt(req.Any(), 10, 64); err == nil {
			memoryMiB = val
		}
	}
	defaultENIs := int64(0)
	ipsPerENI := int64(0)
	if eniLookup != nil {
		if limits, ok := eniLookup(it.Name); ok {
			defaultENIs = int64(limits.DefaultENIs)
			ipsPerENI = int64(limits.IPv4PerENI)
		}
	}
	maxPods := it.Capacity.Pods().Value()
	return kubeletcel.InstanceTypeVars{
		VCPUs:        vcpus,
		MemoryMiB:    memoryMiB,
		DefaultENIs:  defaultENIs,
		IPsPerENI:    ipsPerENI,
		MaxPods:      maxPods,
		InstanceType: it.Name,
	}
}

// serializeResourceMap converts a resource map to a sorted, comparable string representation.
func serializeResourceMap(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := lo.Keys(m)
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"="+m[k])
	}
	return strings.Join(parts, ",")
}

// deserializeResourceMap converts a serialized resource map string back to a map.
func deserializeResourceMap(s string) map[string]string {
	if s == "" {
		return nil
	}
	result := make(map[string]string)
	for _, part := range strings.Split(s, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}
