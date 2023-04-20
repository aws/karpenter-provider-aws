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

	"github.com/aws/karpenter-core/pkg/scheduling"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/karpenter/pkg/providers/amifamily/bootstrap"

	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/cloudprovider"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
)

type Windows struct {
	DefaultFamily
	*Options
}

func (w Windows) DefaultAMIs(version string) (results []DefaultAMIOutput) {
	for windowsVersion, windowsBuild := range v1alpha1.SupportedWindowsVersionAndBuildMapping {
		for _, windowsVariant := range v1alpha1.SupportedWindowsVariants {
			results = append(results, DefaultAMIOutput{Name: fmt.Sprintf("windows-server-%s-%s-%s", windowsVersion, windowsVariant, version),
				Query: fmt.Sprintf("/aws/service/ami-windows-latest/Windows_Server-%s-English-%s-EKS_Optimized-%s/image_id", windowsVersion, windowsVariant, version),
				Requirements: scheduling.NewRequirements(
					scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, v1alpha5.ArchitectureAmd64),
					scheduling.NewRequirement(v1.LabelOSStable, v1.NodeSelectorOpIn, string(v1.Windows)),
					scheduling.NewRequirement(v1.LabelWindowsBuild, v1.NodeSelectorOpIn, windowsBuild),
					scheduling.NewRequirement(v1alpha1.LabelWindowsVariant, v1.NodeSelectorOpIn, windowsVariant),
				)})
		}
	}
	return results
}

// UserData returns the default userdata script for the AMI Family
func (w Windows) UserData(kubeletConfig *v1alpha5.KubeletConfiguration, taints []v1.Taint, labels map[string]string, caBundle *string, _ []*cloudprovider.InstanceType, customUserData *string) bootstrap.Bootstrapper {
	return bootstrap.Windows{
		Options: bootstrap.Options{
			ClusterName:     w.Options.ClusterName,
			ClusterEndpoint: w.Options.ClusterEndpoint,
			KubeletConfig:   kubeletConfig,
			Taints:          taints,
			Labels:          labels,
			CABundle:        caBundle,
			CustomUserData:  customUserData,
		},
	}
}

// DefaultBlockDeviceMappings returns the default block device mappings for the AMI Family
func (w Windows) DefaultBlockDeviceMappings() []*v1alpha1.BlockDeviceMapping {
	sda1EBS := DefaultEBS
	sda1EBS.VolumeSize = lo.ToPtr(resource.MustParse("50Gi"))
	return []*v1alpha1.BlockDeviceMapping{{
		DeviceName: w.EphemeralBlockDevice(),
		EBS:        &sda1EBS,
	}}
}

func (w Windows) EphemeralBlockDevice() *string {
	return aws.String("/dev/sda1")
}
