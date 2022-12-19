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
	"fmt"

	"github.com/samber/lo"

	"github.com/aws/karpenter/pkg/cloudprovider/amifamily/bootstrap"

	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/utils/resources"

	"github.com/aws/aws-sdk-go/aws"
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
func (b Bottlerocket) SSMAlias(version string, instanceType *cloudprovider.InstanceType) string {
	arch := "x86_64"
	amiSuffix := ""
	if !resources.IsZero(instanceType.Capacity[v1alpha1.ResourceNVIDIAGPU]) {
		amiSuffix = "-nvidia"
	}
	if instanceType.Requirements.Get(v1.LabelArchStable).Has(v1alpha5.ArchitectureArm64) {
		arch = v1alpha5.ArchitectureArm64
	}
	return fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s%s/%s/latest/image_id", version, amiSuffix, arch)
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
