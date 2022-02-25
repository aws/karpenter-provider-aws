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

type Ubuntu struct {
	*Options
}

// SSMAlias returns the AMI Alias to query SSM
func (u Ubuntu) SSMAlias(version string, instanceType cloudprovider.InstanceType) string {
	return fmt.Sprintf("/aws/service/canonical/ubuntu/eks/20.04/%s/stable/current/%s/hvm/ebs-gp2/ami-id", version, instanceType.Architecture())
}

// UserData returns the default userdata script for the AMI Family
func (u Ubuntu) UserData(kubeletConfig v1alpha5.KubeletConfiguration, taints []core.Taint, labels map[string]string, caBundle *string) bootstrap.Bootstrapper {
	return bootstrap.EKS{
		Options: bootstrap.Options{
			ClusterName:             u.Options.ClusterName,
			ClusterEndpoint:         u.Options.ClusterEndpoint,
			AWSENILimitedPodDensity: u.Options.AWSENILimitedPodDensity,
			KubeletConfig:           kubeletConfig,
			Taints:                  taints,
			Labels:                  labels,
			CABundle:                caBundle,
		},
	}
}

// DefaultBlockDeviceMappings returns the default block device mappings for the AMI Family
func (u Ubuntu) DefaultBlockDeviceMappings() []*v1alpha1.BlockDeviceMapping {
	return []*v1alpha1.BlockDeviceMapping{{
		DeviceName: aws.String("/dev/sda1"),
		EBS:        &defaultEBS,
	}}
}
