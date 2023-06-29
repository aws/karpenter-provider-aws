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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/imdario/mergo"
	"github.com/samber/lo"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/providers/amifamily/bootstrap"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/scheduling"
)

var DefaultEBS = v1alpha1.BlockDevice{
	Encrypted:  aws.Bool(true),
	VolumeType: aws.String(ec2.VolumeTypeGp3),
	VolumeSize: lo.ToPtr(resource.MustParse("20Gi")),
}

// Resolver is able to fill-in dynamic launch template parameters
type Resolver struct {
	amiProvider *Provider
}

// Options define the static launch template parameters
type Options struct {
	ClusterName             string
	ClusterEndpoint         string
	AWSENILimitedPodDensity bool
	InstanceProfile         string
	CABundle                *string `hash:"ignore"`
	// Level-triggered fields that may change out of sync.
	SecurityGroups           []v1alpha1.SecurityGroup
	Tags                     map[string]string
	Labels                   map[string]string `hash:"ignore"`
	KubeDNSIP                net.IP
	AssociatePublicIPAddress *bool
}

// LaunchTemplate holds the dynamically generated launch template parameters
type LaunchTemplate struct {
	*Options
	UserData            bootstrap.Bootstrapper
	BlockDeviceMappings []*v1alpha1.BlockDeviceMapping
	MetadataOptions     *v1alpha1.MetadataOptions
	AMIID               string
	InstanceTypes       []*cloudprovider.InstanceType `hash:"ignore"`
	DetailedMonitoring  bool
	NetworkInterfaces   []*v1alpha1.NetworkInterface
}

// AMIFamily can be implemented to override the default logic for generating dynamic launch template parameters
type AMIFamily interface {
	DefaultAMIs(version string) []DefaultAMIOutput
	UserData(kubeletConfig *v1alpha5.KubeletConfiguration, taints []core.Taint, labels map[string]string, caBundle *string, instanceTypes []*cloudprovider.InstanceType, customUserData *string) bootstrap.Bootstrapper
	DefaultBlockDeviceMappings() []*v1alpha1.BlockDeviceMapping
	DefaultMetadataOptions() *v1alpha1.MetadataOptions
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

// New constructs a new launch template Resolver
func New(amiProvider *Provider) *Resolver {
	return &Resolver{
		amiProvider: amiProvider,
	}
}

// Resolve generates launch templates using the static options and dynamically generates launch template parameters.
// Multiple ResolvedTemplates are returned based on the instanceTypes passed in to support special AMIs for certain instance types like GPUs.
func (r Resolver) Resolve(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate, machine *v1alpha5.Machine, instanceTypes []*cloudprovider.InstanceType, options *Options) ([]*LaunchTemplate, error) {
	amiFamily := GetAMIFamily(nodeTemplate.Spec.AMIFamily, options)
	amis, err := r.amiProvider.Get(ctx, nodeTemplate, options)
	if err != nil {
		return nil, err
	}
	if len(amis) == 0 {
		return nil, fmt.Errorf("no amis exist given constraints")
	}
	mappedAMIs := MapInstanceTypes(amis, instanceTypes)
	if len(mappedAMIs) == 0 {
		return nil, fmt.Errorf("no instance types satisfy requirements of amis %v,", amis)
	}
	var resolvedTemplates []*LaunchTemplate
	for amiID, instanceTypes := range mappedAMIs {
		maxPodsToInstanceTypes := lo.GroupBy(instanceTypes, func(instanceType *cloudprovider.InstanceType) int {
			return int(instanceType.Capacity.Pods().Value())
		})
		// In order to support reserved ENIs for CNI custom networking setups,
		// we need to pass down the max-pods calculation to the kubelet.
		// This requires that we resolve a unique launch template per max-pods value.
		for maxPods, instanceTypes := range maxPodsToInstanceTypes {
			kubeletConfig := &v1alpha5.KubeletConfiguration{}
			if machine.Spec.Kubelet != nil {
				if err := mergo.Merge(kubeletConfig, machine.Spec.Kubelet); err != nil {
					return nil, err
				}
			}
			if kubeletConfig.MaxPods == nil {
				kubeletConfig.MaxPods = lo.ToPtr(int32(maxPods))
			}
			resolved := &LaunchTemplate{
				Options: options,
				UserData: amiFamily.UserData(
					r.defaultClusterDNS(options, kubeletConfig),
					append(machine.Spec.Taints, machine.Spec.StartupTaints...),
					options.Labels,
					options.CABundle,
					instanceTypes,
					nodeTemplate.Spec.UserData,
				),
				BlockDeviceMappings: nodeTemplate.Spec.BlockDeviceMappings,
				MetadataOptions:     nodeTemplate.Spec.MetadataOptions,
				DetailedMonitoring:  aws.BoolValue(nodeTemplate.Spec.DetailedMonitoring),
				AMIID:               amiID,
				InstanceTypes:       instanceTypes,
				NetworkInterfaces:   nodeTemplate.Spec.NetworkInterfaces,
			}
			if resolved.BlockDeviceMappings == nil {
				resolved.BlockDeviceMappings = amiFamily.DefaultBlockDeviceMappings()
			}
			if resolved.MetadataOptions == nil {
				resolved.MetadataOptions = amiFamily.DefaultMetadataOptions()
			}
			resolvedTemplates = append(resolvedTemplates, resolved)
		}
	}
	return resolvedTemplates, nil
}

func GetAMIFamily(amiFamily *string, options *Options) AMIFamily {
	switch aws.StringValue(amiFamily) {
	case v1alpha1.AMIFamilyBottlerocket:
		return &Bottlerocket{Options: options}
	case v1alpha1.AMIFamilyUbuntu:
		return &Ubuntu{Options: options}
	case v1alpha1.AMIFamilyWindows2019:
		return &Windows{Options: options, Version: v1alpha1.Windows2019, Build: v1alpha1.Windows2019Build}
	case v1alpha1.AMIFamilyWindows2022:
		return &Windows{Options: options, Version: v1alpha1.Windows2022, Build: v1alpha1.Windows2022Build}
	case v1alpha1.AMIFamilyCustom:
		return &Custom{Options: options}
	default:
		return &AL2{Options: options}
	}
}

func (o Options) DefaultMetadataOptions() *v1alpha1.MetadataOptions {
	return &v1alpha1.MetadataOptions{
		HTTPEndpoint:            aws.String(ec2.LaunchTemplateInstanceMetadataEndpointStateEnabled),
		HTTPProtocolIPv6:        aws.String(lo.Ternary(o.KubeDNSIP == nil || o.KubeDNSIP.To4() != nil, ec2.LaunchTemplateInstanceMetadataProtocolIpv6Disabled, ec2.LaunchTemplateInstanceMetadataProtocolIpv6Enabled)),
		HTTPPutResponseHopLimit: aws.Int64(2),
		HTTPTokens:              aws.String(ec2.LaunchTemplateHttpTokensStateRequired),
	}
}

func (r Resolver) defaultClusterDNS(opts *Options, kubeletConfig *v1alpha5.KubeletConfiguration) *v1alpha5.KubeletConfiguration {
	if opts.KubeDNSIP == nil {
		return kubeletConfig
	}
	if kubeletConfig != nil && len(kubeletConfig.ClusterDNS) != 0 {
		return kubeletConfig
	}
	if kubeletConfig == nil {
		return &v1alpha5.KubeletConfiguration{
			ClusterDNS: []string{opts.KubeDNSIP.String()},
		}
	}
	newKubeletConfig := kubeletConfig.DeepCopy()
	newKubeletConfig.ClusterDNS = []string{opts.KubeDNSIP.String()}
	return newKubeletConfig
}
