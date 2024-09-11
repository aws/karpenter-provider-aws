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

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

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

// Resolver is able to fill-in dynamic launch template parameters
type Resolver struct {
	amiProvider Provider
}

// Options define the static launch template parameters
type Options struct {
	ClusterName         string
	ClusterEndpoint     string
	ClusterCIDR         *string
	InstanceProfile     string
	CABundle            *string `hash:"ignore"`
	InstanceStorePolicy *v1.InstanceStorePolicy
	// Level-triggered fields that may change out of sync.
	SecurityGroups           []v1.SecurityGroup
	Tags                     map[string]string
	Labels                   map[string]string `hash:"ignore"`
	KubeDNSIP                net.IP
	AssociatePublicIPAddress *bool
	NodeClassName            string
}

// LaunchTemplate holds the dynamically generated launch template parameters
type LaunchTemplate struct {
	*Options
	UserData            bootstrap.Bootstrapper
	BlockDeviceMappings []*v1.BlockDeviceMapping
	MetadataOptions     *v1.MetadataOptions
	AMIID               string
	InstanceTypes       []*cloudprovider.InstanceType `hash:"ignore"`
	DetailedMonitoring  bool
	EFACount            int
	CapacityType        string
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

// NewResolver constructs a new launch template Resolver
func NewResolver(amiProvider Provider) *Resolver {
	return &Resolver{
		amiProvider: amiProvider,
	}
}

// Resolve generates launch templates using the static options and dynamically generates launch template parameters.
// Multiple ResolvedTemplates are returned based on the instanceTypes passed in to support special AMIs for certain instance types like GPUs.
func (r Resolver) Resolve(nodeClass *v1.EC2NodeClass, nodeClaim *karpv1.NodeClaim, instanceTypes []*cloudprovider.InstanceType, capacityType string, options *Options) ([]*LaunchTemplate, error) {
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
		// Similarly, instance types configured with EfAs require unique launch templates depending on the number of
		// EFAs they support.
		type launchTemplateParams struct {
			efaCount int
			maxPods  int
		}
		paramsToInstanceTypes := lo.GroupBy(instanceTypes, func(instanceType *cloudprovider.InstanceType) launchTemplateParams {
			return launchTemplateParams{
				efaCount: lo.Ternary(
					lo.Contains(lo.Keys(nodeClaim.Spec.Resources.Requests), v1.ResourceEFA),
					int(lo.ToPtr(instanceType.Capacity[v1.ResourceEFA]).Value()),
					0,
				),
				maxPods: int(instanceType.Capacity.Pods().Value()),
			}
		})
		for params, instanceTypes := range paramsToInstanceTypes {
			resolved := r.resolveLaunchTemplate(nodeClass, nodeClaim, instanceTypes, capacityType, amiFamily, amiID, params.maxPods, params.efaCount, options)
			resolvedTemplates = append(resolvedTemplates, resolved)
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
		HTTPEndpoint:            aws.String(string(ec2types.LaunchTemplateInstanceMetadataEndpointStateEnabled)),
		HTTPProtocolIPv6:        aws.String(lo.Ternary(o.KubeDNSIP == nil || o.KubeDNSIP.To4() != nil, string(ec2types.LaunchTemplateInstanceMetadataProtocolIpv6Disabled), string(ec2types.LaunchTemplateInstanceMetadataProtocolIpv6Enabled))),
		HTTPPutResponseHopLimit: aws.Int64(2),
		HTTPTokens:              aws.String(string(ec2types.LaunchTemplateHttpTokensStateRequired)),
	}
}

func (r Resolver) defaultClusterDNS(opts *Options, kubeletConfig *v1.KubeletConfiguration) *v1.KubeletConfiguration {
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

func (r Resolver) resolveLaunchTemplate(nodeClass *v1.EC2NodeClass, nodeClaim *karpv1.NodeClaim, instanceTypes []*cloudprovider.InstanceType, capacityType string,
	amiFamily AMIFamily, amiID string, maxPods int, efaCount int, options *Options) *LaunchTemplate {
	kubeletConfig := nodeClass.Spec.Kubelet
	if kubeletConfig == nil {
		kubeletConfig = &v1.KubeletConfiguration{}
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

	resolved := &LaunchTemplate{
		Options: options,
		UserData: amiFamily.UserData(
			r.defaultClusterDNS(options, kubeletConfig),
			taints,
			options.Labels,
			options.CABundle,
			instanceTypes,
			nodeClass.Spec.UserData,
			options.InstanceStorePolicy,
		),
		BlockDeviceMappings: nodeClass.Spec.BlockDeviceMappings,
		MetadataOptions:     nodeClass.Spec.MetadataOptions,
		DetailedMonitoring:  *nodeClass.Spec.DetailedMonitoring,
		AMIID:               amiID,
		InstanceTypes:       instanceTypes,
		EFACount:            efaCount,
		CapacityType:        capacityType,
	}
	if len(resolved.BlockDeviceMappings) == 0 {
		resolved.BlockDeviceMappings = amiFamily.DefaultBlockDeviceMappings()
	}
	if resolved.MetadataOptions == nil {
		resolved.MetadataOptions = amiFamily.DefaultMetadataOptions()
	}
	return resolved
}
