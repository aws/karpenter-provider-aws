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
	v1 "k8s.io/api/core/v1"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
)

type Ubuntu struct {
	DefaultFamily
	*Options
}

// DefaultAMIs returns the AMI name, and Requirements, with an SSM query
func (u Ubuntu) DefaultAMIs(version string) []DefaultAMIOutput {
	return []DefaultAMIOutput{
		{
			Query: fmt.Sprintf("/aws/service/canonical/ubuntu/eks/20.04/%s/stable/current/%s/hvm/ebs-gp2/ami-id", version, corev1beta1.ArchitectureAmd64),
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, corev1beta1.ArchitectureAmd64),
			),
		},
		{
			Query: fmt.Sprintf("/aws/service/canonical/ubuntu/eks/20.04/%s/stable/current/%s/hvm/ebs-gp2/ami-id", version, corev1beta1.ArchitectureArm64),
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, corev1beta1.ArchitectureArm64),
			),
		},
	}
}

// UserData returns the default userdata script for the AMI Family
func (u Ubuntu) UserData(kubeletConfig *corev1beta1.KubeletConfiguration, taints []v1.Taint, labels map[string]string, caBundle *string, _ []*cloudprovider.InstanceType, customUserData *string, _ *v1beta1.InstanceStorePolicy) bootstrap.Bootstrapper {
	return bootstrap.EKS{
		Options: bootstrap.Options{
			ClusterName:     u.ClusterName,
			ClusterEndpoint: u.ClusterEndpoint,
			KubeletConfig:   kubeletConfig,
			Taints:          taints,
			Labels:          labels,
			CABundle:        caBundle,
			CustomUserData:  customUserData,
		},
	}
}

// DefaultBlockDeviceMappings returns the default block device mappings for the AMI Family
func (u Ubuntu) DefaultBlockDeviceMappings() []*v1beta1.BlockDeviceMapping {
	return []*v1beta1.BlockDeviceMapping{{
		DeviceName: u.EphemeralBlockDevice(),
		EBS:        &DefaultEBS,
	}}
}

func (u Ubuntu) EphemeralBlockDevice() *string {
	return aws.String("/dev/sda1")
}
