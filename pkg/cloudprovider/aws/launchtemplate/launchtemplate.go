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

package launchtemplate

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/launchtemplate/bootstrap"
)

var defaultEBS = v1alpha1.BlockDevice{
	Encrypted:  aws.Bool(true),
	VolumeType: aws.String(ec2.VolumeTypeGp3),
	VolumeSize: resource.NewScaledQuantity(20, resource.Giga),
}

type Options struct {
	ClusterName             string
	ClusterEndpoint         string
	AWSENILimitedPodDensity bool
	InstanceProfile         string
	// Level-triggered fields that may change out of sync.
	AMIID             string
	SecurityGroupsIDs []string
	Tags              map[string]string
	Labels            map[string]string `hash:"ignore"`
	CABundle          *string           `hash:"ignore"`
}

type Resolved struct {
	*Options
	UserData            bootstrap.Bootstrapper
	BlockDeviceMappings []*v1alpha1.BlockDeviceMapping
	MetadataOptions     *v1alpha1.MetadataOptions
}

type AMIFamily interface {
	UserData(kubeletConfig v1alpha5.KubeletConfiguration, taints []core.Taint, labels map[string]string, caBundle *string) bootstrap.Bootstrapper
	SSMAlias(version string, instanceTypes cloudprovider.InstanceType) string
	DefaultBlockDeviceMappings() []*v1alpha1.BlockDeviceMapping
	DefaultMetadataOptions() *v1alpha1.MetadataOptions
}

func Get(constraints *v1alpha1.Constraints, instanceTypes []cloudprovider.InstanceType, options *Options) *Resolved {
	amiFamily := GetAMIFamily(constraints.AMIFamily, options)
	resolved := &Resolved{
		Options:             options,
		UserData:            amiFamily.UserData(constraints.KubeletConfiguration, constraints.Taints, options.Labels, options.CABundle),
		BlockDeviceMappings: constraints.BlockDeviceMappings,
		MetadataOptions:     constraints.MetadataOptions,
	}
	if resolved.BlockDeviceMappings == nil {
		resolved.BlockDeviceMappings = amiFamily.DefaultBlockDeviceMappings()
	}
	if resolved.MetadataOptions == nil {
		resolved.MetadataOptions = amiFamily.DefaultMetadataOptions()
	}
	return resolved
}

func GetAMIFamily(amiFamily *string, options *Options) AMIFamily {
	switch aws.StringValue(amiFamily) {
	case v1alpha1.AMIFamilyAL2:
		return &AL2{Options: options}
	case v1alpha1.AMIFamilyBottlerocket:
		return &Bottlerocket{Options: options}
	case v1alpha1.AMIFamilyUbuntu:
		return &Ubuntu{Options: options}
	}
	return &AL2{Options: options}
}

func (Options) DefaultMetadataOptions() *v1alpha1.MetadataOptions {
	return &v1alpha1.MetadataOptions{
		HTTPEndpoint:            aws.String(ec2.LaunchTemplateInstanceMetadataEndpointStateEnabled),
		HTTPProtocolIPv6:        aws.String(ec2.LaunchTemplateInstanceMetadataProtocolIpv6Disabled),
		HTTPPutResponseHopLimit: aws.Int64(2),
		HTTPTokens:              aws.String(ec2.LaunchTemplateHttpTokensStateRequired),
	}
}
