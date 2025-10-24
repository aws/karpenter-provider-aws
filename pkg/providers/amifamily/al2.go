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
	"sigs.k8s.io/karpenter/pkg/scheduling"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap"
)

type AL2 struct {
	DefaultFamily
	*Options
}

// DefaultAMIs returns the AMI name, and Requirements, with an SSM query
func (a AL2) DefaultAMIs(version string) []DefaultAMIOutput {
	return []DefaultAMIOutput{
		{
			Query: fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", version),
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, corev1beta1.ArchitectureAmd64),
				scheduling.NewRequirement(v1beta1.LabelInstanceGPUCount, v1.NodeSelectorOpDoesNotExist),
				scheduling.NewRequirement(v1beta1.LabelInstanceAcceleratorCount, v1.NodeSelectorOpDoesNotExist),
			),
		},
		{
			Query: fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-gpu/recommended/image_id", version),
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, corev1beta1.ArchitectureAmd64),
				scheduling.NewRequirement(v1beta1.LabelInstanceGPUCount, v1.NodeSelectorOpExists),
			),
		},
		{
			Query: fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-gpu/recommended/image_id", version),
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, corev1beta1.ArchitectureAmd64),
				scheduling.NewRequirement(v1beta1.LabelInstanceAcceleratorCount, v1.NodeSelectorOpExists),
			),
		},
		{
			Query: fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-%s/recommended/image_id", version, corev1beta1.ArchitectureArm64),
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, corev1beta1.ArchitectureArm64),
				scheduling.NewRequirement(v1beta1.LabelInstanceGPUCount, v1.NodeSelectorOpDoesNotExist),
				scheduling.NewRequirement(v1beta1.LabelInstanceAcceleratorCount, v1.NodeSelectorOpDoesNotExist),
			),
		},
	}
}

// UserData returns the exact same string for equivalent input,
// even if elements of those inputs are in differing orders,
// guaranteeing it won't cause spurious hash differences.
// AL2 userdata also works on Ubuntu
func (a AL2) UserData(kubeletConfig *corev1beta1.KubeletConfiguration, taints []v1.Taint, labels map[string]string, caBundle *string, _ []*cloudprovider.InstanceType, customUserData *string, instanceStorePolicy *v1beta1.InstanceStorePolicy) bootstrap.Bootstrapper {
	return bootstrap.EKS{
		Options: bootstrap.Options{
			ClusterName:         a.ClusterName,
			ClusterEndpoint:     a.ClusterEndpoint,
			KubeletConfig:       kubeletConfig,
			Taints:              taints,
			Labels:              labels,
			CABundle:            caBundle,
			CustomUserData:      customUserData,
			InstanceStorePolicy: instanceStorePolicy,
		},
	}
}

// DefaultBlockDeviceMappings returns the default block device mappings for the AMI Family
func (a AL2) DefaultBlockDeviceMappings() []*v1beta1.BlockDeviceMapping {
	return []*v1beta1.BlockDeviceMapping{{
		DeviceName: a.EphemeralBlockDevice(),
		EBS:        &DefaultEBS,
	}}
}

func (a AL2) EphemeralBlockDevice() *string {
	return aws.String("/dev/xvda")
}
