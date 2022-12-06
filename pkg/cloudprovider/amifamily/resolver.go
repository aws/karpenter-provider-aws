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
	"net"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/aws/karpenter-core/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/amifamily/bootstrap"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/utils/pretty"
)

var DefaultEBS = v1alpha1.BlockDevice{
	Encrypted:  aws.Bool(true),
	VolumeType: aws.String(ec2.VolumeTypeGp3),
	VolumeSize: lo.ToPtr(resource.MustParse("20Gi")),
}

// Resolver is able to fill-in dynamic launch template parameters
type Resolver struct {
	amiProvider *AMIProvider
}

// Options define the static launch template parameters
type Options struct {
	ClusterName             string
	ClusterEndpoint         string
	AWSENILimitedPodDensity bool
	InstanceProfile         string
	CABundle                *string `hash:"ignore"`
	// Level-triggered fields that may change out of sync.
	KubernetesVersion string
	SecurityGroupsIDs []string
	Tags              map[string]string
	Labels            map[string]string `hash:"ignore"`
	KubeDNSIP         net.IP
}

// LaunchTemplate holds the dynamically generated launch template parameters
type LaunchTemplate struct {
	*Options
	UserData            bootstrap.Bootstrapper
	BlockDeviceMappings []*v1alpha1.BlockDeviceMapping
	MetadataOptions     *v1alpha1.MetadataOptions
	AMIID               string
	InstanceTypes       []*cloudprovider.InstanceType `hash:"ignore"`
}

// AMIFamily can be implemented to override the default logic for generating dynamic launch template parameters
type AMIFamily interface {
	UserData(kubeletConfig *v1alpha5.KubeletConfiguration, taints []core.Taint, labels map[string]string, caBundle *string, instanceTypes []*cloudprovider.InstanceType, customUserData *string) bootstrap.Bootstrapper
	SSMAlias(version string, instanceType *cloudprovider.InstanceType) string
	DefaultBlockDeviceMappings() []*v1alpha1.BlockDeviceMapping
	DefaultMetadataOptions() *v1alpha1.MetadataOptions
	EphemeralBlockDevice() *string
	FeatureFlags() FeatureFlags
}

// FeatureFlags describes whether the features below are enabled for a given AMIFamily
type FeatureFlags struct {
	UsesENILimitedMemoryOverhead bool
	PodsPerCoreEnabled           bool
	EvictionSoftEnabled          bool
}

// DefaultFamily provides default values for AMIFamilies that compose it
type DefaultFamily struct{}

func (d DefaultFamily) FeatureFlags() FeatureFlags {
	return FeatureFlags{
		UsesENILimitedMemoryOverhead: true,
		PodsPerCoreEnabled:           true,
		EvictionSoftEnabled:          true,
	}
}

// New constructs a new launch template Resolver
func New(kubeClient client.Client, ssm ssmiface.SSMAPI, ec2api ec2iface.EC2API, ssmCache *cache.Cache, ec2Cache *cache.Cache) *Resolver {
	return &Resolver{
		amiProvider: &AMIProvider{
			ssm:      ssm,
			ssmCache: ssmCache,
			ec2Cache: ec2Cache,
			ec2api:   ec2api,
			cm:       pretty.NewChangeMonitor(),
		},
	}
}

// Resolve generates launch templates using the static options and dynamically generates launch template parameters.
// Multiple ResolvedTemplates are returned based on the instanceTypes passed in to support special AMIs for certain instance types like GPUs.
func (r Resolver) Resolve(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate, machine *corev1alpha1.Machine, instanceTypes []*cloudprovider.InstanceType, options *Options) ([]*LaunchTemplate, error) {
	amiFamily := GetAMIFamily(nodeTemplate.Spec.AMIFamily, options)
	amiIDs, err := r.amiProvider.Get(ctx, nodeTemplate, instanceTypes, options, amiFamily)
	if err != nil {
		return nil, err
	}
	var resolvedTemplates []*LaunchTemplate
	for amiID, instanceTypes := range amiIDs {
		resolved := &LaunchTemplate{
			Options: options,
			UserData: amiFamily.UserData(
				machine.Spec.Kubelet,
				append(machine.Spec.Taints, machine.Spec.StartupTaints...),
				options.Labels,
				options.CABundle,
				instanceTypes,
				nodeTemplate.Spec.UserData,
			),
			BlockDeviceMappings: nodeTemplate.Spec.BlockDeviceMappings,
			MetadataOptions:     nodeTemplate.Spec.MetadataOptions,
			AMIID:               amiID,
			InstanceTypes:       instanceTypes,
		}
		if resolved.BlockDeviceMappings == nil {
			resolved.BlockDeviceMappings = amiFamily.DefaultBlockDeviceMappings()
		}
		if resolved.MetadataOptions == nil {
			resolved.MetadataOptions = amiFamily.DefaultMetadataOptions()
		}
		resolvedTemplates = append(resolvedTemplates, resolved)
	}
	return resolvedTemplates, nil
}

func GetAMIFamily(amiFamily *string, options *Options) AMIFamily {
	switch aws.StringValue(amiFamily) {
	case v1alpha1.AMIFamilyBottlerocket:
		return &Bottlerocket{Options: options}
	case v1alpha1.AMIFamilyUbuntu:
		return &Ubuntu{Options: options}
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
