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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/patrickmn/go-cache"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/amifamily/bootstrap"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
)

var defaultEBS = v1alpha1.BlockDevice{
	Encrypted:  aws.Bool(true),
	VolumeType: aws.String(ec2.VolumeTypeGp3),
	VolumeSize: resource.NewScaledQuantity(20, resource.Giga),
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
}

// LaunchTemplate holds the dynamically generated launch template parameters
type LaunchTemplate struct {
	*Options
	UserData            bootstrap.Bootstrapper
	BlockDeviceMappings []*v1alpha1.BlockDeviceMapping
	MetadataOptions     *v1alpha1.MetadataOptions
	AMIID               string
	InstanceTypes       []cloudprovider.InstanceType `hash:"ignore"`
}

// AMIFamily can be implemented to override the default logic for generating dynamic launch template parameters
type AMIFamily interface {
	UserData(kubeletConfig *v1alpha5.KubeletConfiguration, taints []core.Taint, labels map[string]string, caBundle *string, instanceTypes []cloudprovider.InstanceType) bootstrap.Bootstrapper
	SSMAlias(version string, instanceType cloudprovider.InstanceType) string
	DefaultBlockDeviceMappings() []*v1alpha1.BlockDeviceMapping
	DefaultMetadataOptions() *v1alpha1.MetadataOptions
}

// New constructs a new launch template Resolver
func New(ssm ssmiface.SSMAPI, c *cache.Cache) *Resolver {
	return &Resolver{
		amiProvider: &AMIProvider{
			ssm:   ssm,
			cache: c,
		},
	}
}

// Resolve generates launch templates using the static options and dynamically generates launch template parameters.
// Multiple ResolvedTemplates are returned based on the instanceTypes passed in to support special AMIs for certain instance types like GPUs.
func (r Resolver) Resolve(ctx context.Context, provider *v1alpha1.AWS, nodeRequest *cloudprovider.NodeRequest, options *Options) ([]*LaunchTemplate, error) {
	amiFamily := r.getAMIFamily(provider.AMIFamily, options)
	amiIDs := map[string][]cloudprovider.InstanceType{}
	for _, instanceType := range nodeRequest.InstanceTypeOptions {
		amiID, err := r.amiProvider.Get(ctx, instanceType, amiFamily.SSMAlias(options.KubernetesVersion, instanceType))
		if err != nil {
			return nil, err
		}
		amiIDs[amiID] = append(amiIDs[amiID], instanceType)
	}
	var resolvedTemplates []*LaunchTemplate
	for amiID, instanceTypes := range amiIDs {
		resolved := &LaunchTemplate{
			Options:             options,
			UserData:            amiFamily.UserData(nodeRequest.Template.KubeletConfiguration, nodeRequest.Template.Taints, options.Labels, options.CABundle, instanceTypes),
			BlockDeviceMappings: provider.BlockDeviceMappings,
			MetadataOptions:     provider.MetadataOptions,
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

func (r Resolver) getAMIFamily(amiFamily *string, options *Options) AMIFamily {
	switch aws.StringValue(amiFamily) {
	case v1alpha1.AMIFamilyBottlerocket:
		return &Bottlerocket{Options: options}
	case v1alpha1.AMIFamilyUbuntu:
		return &Ubuntu{Options: options}
	default:
		return &AL2{Options: options}
	}
}

func (Options) DefaultMetadataOptions() *v1alpha1.MetadataOptions {
	return &v1alpha1.MetadataOptions{
		HTTPEndpoint:            aws.String(ec2.LaunchTemplateInstanceMetadataEndpointStateEnabled),
		HTTPProtocolIPv6:        aws.String(ec2.LaunchTemplateInstanceMetadataProtocolIpv6Disabled),
		HTTPPutResponseHopLimit: aws.Int64(2),
		HTTPTokens:              aws.String(ec2.LaunchTemplateHttpTokensStateRequired),
	}
}
