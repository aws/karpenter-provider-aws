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

	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"

	"github.com/aws/karpenter/pkg/providers/amifamily/bootstrap"

	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/scheduling"
	"github.com/aws/karpenter-core/pkg/utils/pretty"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
)

type Bottlerocket struct {
	DefaultFamily
	*Options
}

// SSMAlias returns the AMI Alias to query SSM
func (b Bottlerocket) SSMAlias(ctx context.Context, version string, ssmCache *cache.Cache, ssmapi ssmiface.SSMAPI, cm *pretty.ChangeMonitor) (map[AMI]scheduling.Requirements, error) {
	amiRequirements := map[AMI]scheduling.Requirements{}
	architectures := []string{"x86_64", v1alpha5.ArchitectureArm64}

	for _, arch := range architectures {
		requirements := scheduling.NewRequirements()
		if arch == "x86_64" {
			requirements.Add(scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, v1alpha5.ArchitectureAmd64))
		} else {
			requirements.Add(scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, arch))
		}
		query := fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/%s/latest/image_id", version, arch)
		amiID, err := b.FetchAMIsFromSSM(ctx, query, ssmCache, ssmapi, cm)
		if err != nil {
			return nil, err
		}
		output := AMI{
			Name:  fmt.Sprintf("bottlerocket-aws-k8s-%s%s", version, "-"+arch),
			AmiID: amiID,
		}
		amiRequirements[output] = requirements

		requirements = scheduling.NewRequirements()
		if arch == "x86_64" {
			requirements.Add(scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, v1alpha5.ArchitectureAmd64))
		} else {
			requirements.Add(scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, arch))
		}
		requirements.Add(scheduling.NewRequirement(v1alpha1.LabelInstanceGPUManufacturer, v1.NodeSelectorOpIn, "nvidia"))

		query = fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s%s/%s/latest/image_id", version, "-nvidia", arch)
		amiID, err = b.FetchAMIsFromSSM(ctx, query, ssmCache, ssmapi, cm)
		if err != nil {
			return nil, err
		}
		output = AMI{
			Name:  fmt.Sprintf("bottlerocket-aws-k8s-%s%s%s", version, "-"+arch, "-nvidia"),
			AmiID: amiID,
		}
		amiRequirements[output] = requirements
	}

	return amiRequirements, nil
}

// UserData returns the default userdata script for the AMI Family
func (b Bottlerocket) UserData(kubeletConfig *v1alpha5.KubeletConfiguration, taints []v1.Taint, labels map[string]string, caBundle *string, _ []*cloudprovider.InstanceType, customUserData *string) bootstrap.Bootstrapper {
	return bootstrap.Bottlerocket{
		Options: bootstrap.Options{
			ClusterName:             b.Options.ClusterName,
			ClusterEndpoint:         b.Options.ClusterEndpoint,
			AWSENILimitedPodDensity: b.Options.AWSENILimitedPodDensity,
			KubeletConfig:           kubeletConfig,
			Taints:                  taints,
			Labels:                  labels,
			CABundle:                caBundle,
			CustomUserData:          customUserData,
		},
	}
}

// DefaultBlockDeviceMappings returns the default block device mappings for the AMI Family
func (b Bottlerocket) DefaultBlockDeviceMappings() []*v1alpha1.BlockDeviceMapping {
	xvdaEBS := DefaultEBS
	xvdaEBS.VolumeSize = lo.ToPtr(resource.MustParse("4Gi"))
	return []*v1alpha1.BlockDeviceMapping{
		{
			DeviceName: aws.String("/dev/xvda"),
			EBS:        &xvdaEBS,
		},
		{
			DeviceName: b.EphemeralBlockDevice(),
			EBS:        &DefaultEBS,
		},
	}
}

func (b Bottlerocket) EphemeralBlockDevice() *string {
	return aws.String("/dev/xvdb")
}

// PodsPerCoreEnabled is currently disabled for Bottlerocket AMIFamily because it does
// not currently support the podsPerCore parameter passed through the kubernetes settings TOML userData
// If a Provisioner sets the podsPerCore value when using the Bottlerocket AMIFamily in the provider,
// podsPerCore will be ignored
// https://github.com/bottlerocket-os/bottlerocket/issues/1721

// EvictionSoftEnabled is currently disabled for Bottlerocket AMIFamily because it does
// not currently support the evictionSoft parameter passed through the kubernetes settings TOML userData
// If a Provisioner sets the evictionSoft value when using the Bottlerocket AMIFamily in the provider,
// evictionSoft will be ignored
// https://github.com/bottlerocket-os/bottlerocket/issues/1445

func (b Bottlerocket) FeatureFlags() FeatureFlags {
	return FeatureFlags{
		UsesENILimitedMemoryOverhead: false,
		PodsPerCoreEnabled:           false,
		EvictionSoftEnabled:          false,
	}
}
