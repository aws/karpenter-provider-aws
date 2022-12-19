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

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"

	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/utils/resources"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/amifamily/bootstrap"
)

type AL2 struct {
	DefaultFamily
	*Options
}

// SSMAlias returns the AMI Alias to query SSM
func (a AL2) SSMAlias(version string, instanceType *cloudprovider.InstanceType) string {
	amiSuffix := ""
	if !resources.IsZero(instanceType.Capacity[v1alpha1.ResourceNVIDIAGPU]) || !resources.IsZero(instanceType.Capacity[v1alpha1.ResourceAWSNeuron]) {
		amiSuffix = "-gpu"
	} else if instanceType.Requirements.Get(v1.LabelArchStable).Has(v1alpha5.ArchitectureArm64) {
		amiSuffix = fmt.Sprintf("-%s", v1alpha5.ArchitectureArm64)
	}
	return fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2%s/recommended/image_id", version, amiSuffix)
}

// UserData returns the exact same string for equivalent input,
// even if elements of those inputs are in differing orders,
// guaranteeing it won't cause spurious hash differences.
// AL2 userdata also works on Ubuntu
func (a AL2) UserData(kubeletConfig *v1alpha5.KubeletConfiguration, taints []v1.Taint, labels map[string]string, caBundle *string, instanceTypes []*cloudprovider.InstanceType, customUserData *string) bootstrap.Bootstrapper {
	containerRuntime := aws.String("containerd")
	if kubeletConfig != nil && kubeletConfig.ContainerRuntime != nil {
		containerRuntime = kubeletConfig.ContainerRuntime
	}
	return bootstrap.EKS{
		ContainerRuntime: *containerRuntime,
		Options: bootstrap.Options{
			ClusterName:             a.Options.ClusterName,
			ClusterEndpoint:         a.Options.ClusterEndpoint,
			AWSENILimitedPodDensity: a.Options.AWSENILimitedPodDensity,
			KubeletConfig:           a.defaultIPv6DNS(kubeletConfig),
			Taints:                  taints,
			Labels:                  labels,
			CABundle:                caBundle,
			CustomUserData:          customUserData,
		},
	}
}

func (a AL2) defaultIPv6DNS(kubeletConfig *v1alpha5.KubeletConfiguration) *v1alpha5.KubeletConfiguration {
	if a.KubeDNSIP == nil || a.KubeDNSIP.To4() != nil {
		return kubeletConfig
	}
	if kubeletConfig != nil && len(kubeletConfig.ClusterDNS) != 0 {
		return kubeletConfig
	}
	if kubeletConfig == nil {
		return &v1alpha5.KubeletConfiguration{
			ClusterDNS: []string{a.KubeDNSIP.String()},
		}
	}
	newKubeletConfig := kubeletConfig.DeepCopy()
	newKubeletConfig.ClusterDNS = []string{a.KubeDNSIP.String()}
	return newKubeletConfig
}

// DefaultBlockDeviceMappings returns the default block device mappings for the AMI Family
func (a AL2) DefaultBlockDeviceMappings() []*v1alpha1.BlockDeviceMapping {
	return []*v1alpha1.BlockDeviceMapping{{
		DeviceName: a.EphemeralBlockDevice(),
		EBS:        &DefaultEBS,
	}}
}

func (a AL2) EphemeralBlockDevice() *string {
	return aws.String("/dev/xvda")
}
