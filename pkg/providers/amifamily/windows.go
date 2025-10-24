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

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap"

	v1 "k8s.io/api/core/v1"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
)

type Windows struct {
	DefaultFamily
	*Options
	Version string
	Build   string
}

func (w Windows) DefaultAMIs(version string) []DefaultAMIOutput {
	return []DefaultAMIOutput{
		{
			Query: fmt.Sprintf("/aws/service/ami-windows-latest/Windows_Server-%s-English-%s-EKS_Optimized-%s/image_id", w.Version, v1beta1.WindowsCore, version),
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, corev1beta1.ArchitectureAmd64),
				scheduling.NewRequirement(v1.LabelOSStable, v1.NodeSelectorOpIn, string(v1.Windows)),
				scheduling.NewRequirement(v1.LabelWindowsBuild, v1.NodeSelectorOpIn, w.Build),
			),
		},
	}
}

// UserData returns the default userdata script for the AMI Family
func (w Windows) UserData(kubeletConfig *corev1beta1.KubeletConfiguration, taints []v1.Taint, labels map[string]string, caBundle *string, _ []*cloudprovider.InstanceType, customUserData *string, _ *v1beta1.InstanceStorePolicy) bootstrap.Bootstrapper {
	return bootstrap.Windows{
		Options: bootstrap.Options{
			ClusterName:     w.ClusterName,
			ClusterEndpoint: w.ClusterEndpoint,
			KubeletConfig:   kubeletConfig,
			Taints:          taints,
			Labels:          labels,
			CABundle:        caBundle,
			CustomUserData:  customUserData,
		},
	}
}

// DefaultBlockDeviceMappings returns the default block device mappings for the AMI Family
func (w Windows) DefaultBlockDeviceMappings() []*v1beta1.BlockDeviceMapping {
	sda1EBS := DefaultEBS
	sda1EBS.VolumeSize = lo.ToPtr(resource.MustParse("50Gi"))
	return []*v1beta1.BlockDeviceMapping{{
		DeviceName: w.EphemeralBlockDevice(),
		EBS:        &sda1EBS,
	}}
}

func (w Windows) EphemeralBlockDevice() *string {
	return aws.String("/dev/sda1")
}

func (w Windows) FeatureFlags() FeatureFlags {
	return FeatureFlags{
		UsesENILimitedMemoryOverhead: false,
		PodsPerCoreEnabled:           true,
		EvictionSoftEnabled:          true,
		SupportsENILimitedPodDensity: false,
	}
}
