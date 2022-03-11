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

	"github.com/aws/aws-sdk-go/aws"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/amifamily/bootstrap"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
)

type Bottlerocket struct {
	*Options
}

// SSMAlias returns the AMI Alias to query SSM
func (b Bottlerocket) SSMAlias(version string, instanceType cloudprovider.InstanceType) string {
	arch := "x86_64"
	amiSuffix := ""
	if !instanceType.NvidiaGPUs().IsZero() {
		amiSuffix = "-nvidia"
	}
	if instanceType.Architecture() == v1alpha5.ArchitectureArm64 {
		arch = instanceType.Architecture()
	}
	return fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s%s/%s/latest/image_id", version, amiSuffix, arch)
}

// UserData returns the default userdata script for the AMI Family
func (b Bottlerocket) UserData(kubeletConfig *v1alpha5.KubeletConfiguration, taints []core.Taint, labels map[string]string, caBundle *string) bootstrap.Bootstrapper {
	return bootstrap.Bottlerocket{
		Options: bootstrap.Options{
			ClusterName:             b.Options.ClusterName,
			ClusterEndpoint:         b.Options.ClusterEndpoint,
			AWSENILimitedPodDensity: b.Options.AWSENILimitedPodDensity,
			KubeletConfig:           kubeletConfig,
			Taints:                  taints,
			Labels:                  labels,
			CABundle:                caBundle,
		},
	}
}

// DefaultBlockDeviceMappings returns the default block device mappings for the AMI Family
func (b Bottlerocket) DefaultBlockDeviceMappings() []*v1alpha1.BlockDeviceMapping {
	xvdaEBS := defaultEBS
	xvdaEBS.VolumeSize = resource.NewScaledQuantity(4, resource.Giga)
	return []*v1alpha1.BlockDeviceMapping{
		{
			DeviceName: aws.String("/dev/xvda"),
			EBS:        &xvdaEBS,
		},
		{
			DeviceName: aws.String("/dev/xvdb"),
			EBS:        &defaultEBS,
		},
	}
}
