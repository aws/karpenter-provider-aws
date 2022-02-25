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
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	core "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/launchtemplate/bootstrap"
)

type AL2 struct {
	*Options
}

// SSMAlias returns the AMI Alias to query SSM
func (a AL2) SSMAlias(version string, instanceType cloudprovider.InstanceType) string {
	amiSuffix := ""
	if !instanceType.NvidiaGPUs().IsZero() || !instanceType.AWSNeurons().IsZero() {
		amiSuffix = "-gpu"
	} else if instanceType.Architecture() == v1alpha5.ArchitectureArm64 {
		amiSuffix = fmt.Sprintf("-%s", instanceType.Architecture())
	}
	return fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2%s/recommended/image_id", version, amiSuffix)
}

// UserData returns the exact same string for equivalent input,
// even if elements of those inputs are in differing orders,
// guaranteeing it won't cause spurious hash differences.
// AL2 userdata also works on Ubuntu
func (a AL2) UserData(kubeletConfig v1alpha5.KubeletConfiguration, taints []core.Taint, labels map[string]string, caBundle *string) bootstrap.Bootstrapper {
	return bootstrap.EKS{
		Options: bootstrap.Options{
			ClusterName:             a.Options.ClusterName,
			ClusterEndpoint:         a.Options.ClusterEndpoint,
			AWSENILimitedPodDensity: a.Options.AWSENILimitedPodDensity,
			KubeletConfig:           kubeletConfig,
			Taints:                  taints,
			Labels:                  labels,
			CABundle:                caBundle,
		},
	}
}

// DefaultBlockDeviceMappings returns the default block device mappings for the AMI Family
func (a AL2) DefaultBlockDeviceMappings() []*v1alpha1.BlockDeviceMapping {
	return []*v1alpha1.BlockDeviceMapping{{
		DeviceName: aws.String("/dev/xvda"),
		EBS:        &defaultEBS,
	}}
}
