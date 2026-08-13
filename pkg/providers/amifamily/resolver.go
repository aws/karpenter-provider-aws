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
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/serrors"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	kubeletcel "github.com/aws/karpenter-provider-aws/pkg/cel"
	karpopts "github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap"
	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"
)

var DefaultEBS = v1.BlockDevice{
	Encrypted:  aws.Bool(true),
	VolumeType: aws.String(string(ec2types.VolumeTypeGp3)),
	VolumeSize: lo.ToPtr(resource.MustParse("20Gi")),
}

type Resolver interface {
	Resolve(context.Context, *v1.EC2NodeClass, *karpv1.NodeClaim, []*cloudprovider.InstanceType, string, string, *Options, string, int32) ([]*LaunchTemplate, error)
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
	celEnv    *kubeletcel.CELEnvironment
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
	UserData(kubeletConfig *v1.ParsedKubeletConfig, unparsedKubeletConfig v1.KubeletConfiguration, taints []corev1.Taint, labels map[string]string, caBundle *string, instanceTypes []*cloudprovider.InstanceType, customUserData *string, instanceStorePolicy *v1.InstanceStorePolicy) bootstrap.Bootstrapper
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
	// SupportsArbitraryKubeletConfig is true for families that render the raw kubelet config
	// through to the node (nodeadm inline config), so any valid kubelet field is honored. Families
	// that leave it false only apply the specific fields Karpenter maps to bootstrap flags/settings;
	// the validation controller rejects unmapped fields for them rather than dropping them silently.
	SupportsArbitraryKubeletConfig bool
}

// DefaultFamily provides default values for AMIFamilies that compose it
type DefaultFamily struct{}

func (d DefaultFamily) FeatureFlags() FeatureFlags {
	return FeatureFlags{
		UsesENILimitedMemoryOverhead:   true,
		PodsPerCoreEnabled:             true,
		EvictionSoftEnabled:            true,
		SupportsENILimitedPodDensity:   true,
		SupportsArbitraryKubeletConfig: false,
	}
}

// NewDefaultResolver constructs a new launch template DefaultResolver
func NewDefaultResolver(region string, eniLookup ENILookup, celEnv *kubeletcel.CELEnvironment) *DefaultResolver {
	return &DefaultResolver{
		region:    region,
		eniLookup: eniLookup,
		celEnv:    celEnv,
	}
}

// Resolve generates launch templates using the static options and dynamically generates launch template parameters.
// Multiple ResolvedTemplates are returned based on the instanceTypes passed in to support special AMIs for certain instance types like GPUs.
//
//nolint:gocyclo
func (r DefaultResolver) Resolve(ctx context.Context, nodeClass *v1.EC2NodeClass, nodeClaim *karpv1.NodeClaim, instanceTypes []*cloudprovider.InstanceType, capacityType string, tenancyType string, options *Options, placementGroupID string, placementGroupPartition int32) ([]*LaunchTemplate, error) {
	amiFamily := GetAMIFamily(nodeClass.AMIFamily(), options)
	if len(nodeClass.Status.AMIs) == 0 {
		return nil, fmt.Errorf("no amis exist given constraints")
	}
	mappedAMIs := MapToInstanceTypes(instanceTypes, nodeClass.Status.AMIs)
	if len(mappedAMIs) == 0 {
		return nil, fmt.Errorf("no instance types satisfy requirements of amis %v", lo.Uniq(lo.Map(nodeClass.Status.AMIs, func(a v1.AMI, _ int) string { return a.ID })))
	}
	parsedKubelet, _ := v1.ParseKubeletConfig(nodeClass.Spec.Kubelet)
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
		// paramsForInstanceType computes the launch template grouping key for an instance type.
		paramsForInstanceType := func(it *cloudprovider.InstanceType) (launchTemplateParams, error) {
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
			if parsedKubelet != nil {
				// Read the reserved-capacity fields (which may hold CEL expressions) from the config
				// parsed once above. A nil parsedKubelet leaves both nil, falling back to AMI-family
				// defaults rather than failing template resolution here.
				kubeReserved = parsedKubelet.KubeReserved
				systemReserved = parsedKubelet.SystemReserved
			}
			// With the NodeClassCEL gate off, expression-valued entries aren't honored: keep only the static
			// quantity entries so the launch template falls back to the AMI family defaults rather than
			// configuring nodes from an expression the user hasn't opted into. This mirrors the scheduler's
			// resolution path (instancetype.resolveResourceExpressions) so the two never diverge.
			if !karpopts.FromContext(ctx).FeatureGates.NodeClassCEL {
				kubeReserved = staticQuantities(kubeReserved)
				systemReserved = staticQuantities(systemReserved)
			}
			// kubeReserved and systemReserved are resolved through the same shared CEL evaluation path
			// (kubeletcel.ResolveResourceMap) against the same live-EC2-backed ENI lookup used by the
			// scheduler, so the launch template configures exactly what the scheduler reserved.
			celVars := func() (kubeletcel.InstanceTypeVars, error) { return celVarsFromInstanceType(it, r.eniLookup) }
			resolvedKubeReserved, err := r.celEnv.ResolveResourceMap(ctx, kubeReserved, celVars)
			if err != nil {
				return launchTemplateParams{}, serrors.Wrap(
					fmt.Errorf("resolving kubeReserved, %w", err),
					"instance-type", it.Name)
			}
			resolvedSystemReserved, err := r.celEnv.ResolveResourceMap(ctx, systemReserved, celVars)
			if err != nil {
				return launchTemplateParams{}, serrors.Wrap(
					fmt.Errorf("resolving systemReserved, %w", err),
					"instance-type", it.Name)
			}
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
			}, nil
		}
		paramsToInstanceTypes := map[launchTemplateParams][]*cloudprovider.InstanceType{}
		for _, it := range instanceTypes {
			params, err := paramsForInstanceType(it)
			if err != nil {
				return nil, err
			}
			paramsToInstanceTypes[params] = append(paramsToInstanceTypes[params], it)
		}

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

// defaultClusterDNS fills in clusterDNS from the discovered kube-dns address when the user hasn't
// set it. It updates raw alongside the parsed config: nodeadm-based AMI families render the raw
// map as inline kubelet config, so a default applied only to the parsed struct would be dropped
// for them. raw is mutated in place, having already been deep-copied from the NodeClass.
func (r DefaultResolver) defaultClusterDNS(opts *Options, kubeletConfig *v1.ParsedKubeletConfig, raw v1.KubeletConfiguration) *v1.ParsedKubeletConfig {
	if opts.KubeDNSIP == nil {
		return kubeletConfig
	}
	if kubeletConfig != nil && len(kubeletConfig.ClusterDNS) != 0 {
		return kubeletConfig
	}
	clusterDNS := []string{opts.KubeDNSIP.String()}
	if raw != nil {
		raw["clusterDNS"] = v1.JSONValue(clusterDNS)
	}
	if kubeletConfig == nil {
		return &v1.ParsedKubeletConfig{ClusterDNS: clusterDNS}
	}
	newKubeletConfig := kubeletConfig.DeepCopy()
	newKubeletConfig.ClusterDNS = clusterDNS
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
	unparsedKubeletConfig := nodeClass.Spec.Kubelet.DeepCopy()
	parsedKubeletConfig, _ := v1.ParseKubeletConfig(unparsedKubeletConfig)
	if parsedKubeletConfig == nil {
		parsedKubeletConfig = &v1.ParsedKubeletConfig{}
	}
	// maxPods is the count already resolved for this instance type, so it stands in both when the
	// user set nothing and when they set a CEL expression: bootstrap needs a concrete number, and
	// an expression can't be evaluated without an instance type.
	//
	// Both representations are updated because AMI families consume different ones -- the parsed
	// struct drives the flag-based bootstrappers, while nodeadm-based AL2023 passes the raw map
	// through as inline kubelet config. Leaving the raw map alone would ship the unevaluated
	// expression to the node, or omit maxPods entirely when it was defaulted here.
	if _, ok := parsedKubeletConfig.MaxPodsValue(); !ok {
		// nolint:gosec
		// We know that it's not possible to have values that would overflow int32 here since we control
		// the maxPods values that we pass in here
		parsedKubeletConfig.MaxPods = lo.ToPtr(intstr.FromInt32(int32(maxPods)))
		if unparsedKubeletConfig == nil {
			unparsedKubeletConfig = v1.KubeletConfiguration{}
		}
		unparsedKubeletConfig["maxPods"] = v1.JSONValue(maxPods)
	}
	// kubeReserved/systemReserved may have been CEL expressions resolved per instance type upstream.
	// Write the resolved quantities back onto both representations for the same reason as maxPods:
	// the parsed struct feeds the flag-based bootstrappers and the raw map feeds nodeadm's inline
	// kubelet config, so an unresolved expression left in either would reach the node verbatim.
	if resolvedKubeReserved != nil {
		parsedKubeletConfig.KubeReserved = resolvedKubeReserved
		if unparsedKubeletConfig == nil {
			unparsedKubeletConfig = v1.KubeletConfiguration{}
		}
		unparsedKubeletConfig["kubeReserved"] = v1.JSONValue(resolvedKubeReserved)
	}
	if resolvedSystemReserved != nil {
		parsedKubeletConfig.SystemReserved = resolvedSystemReserved
		if unparsedKubeletConfig == nil {
			unparsedKubeletConfig = v1.KubeletConfiguration{}
		}
		unparsedKubeletConfig["systemReserved"] = v1.JSONValue(resolvedSystemReserved)
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
				r.defaultClusterDNS(options, parsedKubeletConfig, unparsedKubeletConfig),
				unparsedKubeletConfig,
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

// celVarsFromInstanceType builds CEL evaluation variables from a cloudprovider.InstanceType
// using its requirements (CPU, memory labels) and an ENI lookup function.
func celVarsFromInstanceType(it *cloudprovider.InstanceType, eniLookup ENILookup) (kubeletcel.InstanceTypeVars, error) {
	req := it.Requirements.Get(v1.LabelInstanceCPU)
	if req == nil {
		return kubeletcel.InstanceTypeVars{}, serrors.Wrap(
			fmt.Errorf("instance type is missing the label required for CEL evaluation"),
			"instance-type", it.Name,
			"label", v1.LabelInstanceCPU,
		)
	}
	vcpus, err := strconv.ParseInt(req.Any(), 10, 64)
	if err != nil {
		return kubeletcel.InstanceTypeVars{}, serrors.Wrap(
			fmt.Errorf("parsing label, %w", err),
			"instance-type", it.Name,
			"label", v1.LabelInstanceCPU,
			"value", req.Any(),
		)
	}
	req = it.Requirements.Get(v1.LabelInstanceMemory)
	if req == nil {
		return kubeletcel.InstanceTypeVars{}, serrors.Wrap(
			fmt.Errorf("instance type is missing the label required for CEL evaluation"),
			"instance-type", it.Name,
			"label", v1.LabelInstanceMemory,
		)
	}
	memoryMiB, err := strconv.ParseInt(req.Any(), 10, 64)
	if err != nil {
		return kubeletcel.InstanceTypeVars{}, serrors.Wrap(
			fmt.Errorf("parsing label, %w", err),
			"instance-type", it.Name,
			"label", v1.LabelInstanceMemory,
			"value", req.Any(),
		)
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
	}, nil
}

// staticQuantities returns the subset of a kubeReserved/systemReserved map whose values parse as
// Kubernetes resource quantities, dropping the CEL-expression entries. It's used when the NodeClassCEL
// gate is off so expression-valued entries fall back to the AMI family defaults rather than being resolved.
func staticQuantities(m map[string]string) map[string]string {
	if len(m) == 0 {
		return m
	}
	return lo.PickBy(m, func(_ string, v string) bool {
		_, err := resource.ParseQuantity(v)
		return err == nil
	})
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
