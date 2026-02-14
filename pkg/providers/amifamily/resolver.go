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
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap"
	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"
)

var DefaultEBS = v1.BlockDevice{
	Encrypted:  aws.Bool(true),
	VolumeType: aws.String(string(ec2types.VolumeTypeGp3)),
	VolumeSize: lo.ToPtr(resource.MustParse("20Gi")),
}

type Resolver interface {
	Resolve(*v1.EC2NodeClass, *karpv1.NodeClaim, []*cloudprovider.InstanceType, string, string, *Options) ([]*LaunchTemplate, error)
}

// DefaultResolver is able to fill-in dynamic launch template parameters
type DefaultResolver struct {
	region string
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
	SecurityGroups           []v1.SecurityGroup
	Tags                     map[string]string
	Labels                   map[string]string `hash:"ignore"`
	KubeDNSIP                net.IP
	AssociatePublicIPAddress *bool
	IPPrefixCount            *int32
	NodeClassName            string
}

// LaunchTemplate holds the dynamically generated launch template parameters
type LaunchTemplate struct {
	*Options
	UserData                bootstrap.Bootstrapper
	BlockDeviceMappings     []*v1.BlockDeviceMapping
	MetadataOptions         *v1.MetadataOptions
	CPUOptions              *v1.CPUOptions
	AMIID                   string
	InstanceTypes           []*cloudprovider.InstanceType `hash:"ignore"`
	DetailedMonitoring      bool
	EFACount                int
	CapacityType            string
	CapacityReservationID   string
	CapacityReservationType v1.CapacityReservationType
	Tenancy                 string
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
func NewDefaultResolver(region string) *DefaultResolver {
	return &DefaultResolver{
		region: region,
	}
}

// Resolve generates launch templates using the static options and dynamically generates launch template parameters.
// Multiple ResolvedTemplates are returned based on the instanceTypes passed in to support special AMIs for certain instance types like GPUs.
func (r DefaultResolver) Resolve(nodeClass *v1.EC2NodeClass, nodeClaim *karpv1.NodeClaim, instanceTypes []*cloudprovider.InstanceType, capacityType string, tenancyType string, options *Options) ([]*LaunchTemplate, error) {
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
			// reservationIDs is encoded as a string rather than a slice to ensure this type is comparable for use by `lo.GroupBy`.
			reservationIDs  string
			reservationType v1.CapacityReservationType
		}
		paramsToInstanceTypes := lo.GroupBy(instanceTypes, func(it *cloudprovider.InstanceType) launchTemplateParams {
			var reservationType v1.CapacityReservationType
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
					}
				}
			}
			return launchTemplateParams{
				efaCount: lo.Ternary(
					lo.Contains(lo.Keys(nodeClaim.Spec.Resources.Requests), v1.ResourceEFA),
					int(lo.ToPtr(it.Capacity[v1.ResourceEFA]).Value()),
					0,
				),
				maxPods: int(it.Capacity.Pods().Value()),
				// If we're dealing with reserved instances, there's only going to be a single instance per group. This invariant
				// is due to reservation IDs not being shared across instance types. Because of this, we don't need to worry about
				// ordering in this string.
				reservationIDs:  strings.Join(reservationIDs, ","),
				reservationType: reservationType,
			}
		})

		for params, instanceTypes := range paramsToInstanceTypes {
			reservationIDs := strings.Split(params.reservationIDs, ",")
			resolvedTemplates = append(resolvedTemplates, r.resolveLaunchTemplates(nodeClass, nodeClaim, instanceTypes, capacityType, amiFamily, amiID, params.maxPods, params.efaCount, reservationIDs, params.reservationType, options, tenancyType)...)
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
	options *Options,
	tenancyType string,
) []*LaunchTemplate {
	kubeletConfig := &v1.KubeletConfiguration{}
	if nodeClass.Spec.Kubelet != nil {
		kubeletConfig = nodeClass.Spec.Kubelet.DeepCopy()
	}
	if kubeletConfig.MaxPods == nil {
		// nolint:gosec
		// We know that it's not possible to have values that would overflow int32 here since we control
		// the maxPods values that we pass in here
		kubeletConfig.MaxPods = lo.ToPtr(int32(maxPods))
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
			BlockDeviceMappings:     nodeClass.Spec.BlockDeviceMappings,
			MetadataOptions:         nodeClass.Spec.MetadataOptions,
			CPUOptions:              nodeClass.Spec.CPUOptions,
			DetailedMonitoring:      aws.ToBool(nodeClass.Spec.DetailedMonitoring),
			AMIID:                   amiID,
			InstanceTypes:           instanceTypes,
			EFACount:                efaCount,
			CapacityType:            capacityType,
			CapacityReservationID:   id,
			CapacityReservationType: capacityReservationType,
			Tenancy:                 tenancyType,
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
