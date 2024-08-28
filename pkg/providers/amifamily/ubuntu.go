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
	"github.com/aws/aws-sdk-go/aws"
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
)

type Ubuntu struct {
	DefaultFamily
	*Options
}

// UserData returns the default userdata script for the AMI Family
func (u Ubuntu) UserData(kubeletConfig *v1.KubeletConfiguration, taints []corev1.Taint, labels map[string]string, caBundle *string, _ []*cloudprovider.InstanceType, customUserData *string, _ *v1.InstanceStorePolicy) bootstrap.Bootstrapper {
	return bootstrap.EKS{
		Options: bootstrap.Options{
			ClusterName:     u.Options.ClusterName,
			ClusterEndpoint: u.Options.ClusterEndpoint,
			KubeletConfig:   kubeletConfig,
			Taints:          taints,
			Labels:          labels,
			CABundle:        caBundle,
			CustomUserData:  customUserData,
		},
	}
}

// DefaultBlockDeviceMappings returns the default block device mappings for the AMI Family
func (u Ubuntu) DefaultBlockDeviceMappings() []*v1.BlockDeviceMapping {
	return []*v1.BlockDeviceMapping{{
		DeviceName: u.EphemeralBlockDevice(),
		EBS:        &DefaultEBS,
	}}
}

func (u Ubuntu) EphemeralBlockDevice() *string {
	return aws.String("/dev/sda1")
}
