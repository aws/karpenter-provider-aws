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
	"context"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap"
	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"
)

type Custom struct {
	DefaultFamily
	*Options
	OutputOption OutputOption
}

// Enum for the output option
type OutputOption string

const (
	OutputOptionBase64  OutputOption = "base64"
	OutputOptionNodeadm OutputOption = "nodeadm"
)

// UserData returns the default userdata script for the AMI Family
func (c Custom) UserData(kubeletConfig *v1.KubeletConfiguration, taints []corev1.Taint, labels map[string]string, caBundle *string, _ []*cloudprovider.InstanceType, customUserData *string, instanceStorePolicy *v1.InstanceStorePolicy) bootstrap.Bootstrapper {
	switch c.OutputOption {
	case OutputOptionBase64:
		return bootstrap.Custom{
			Options: bootstrap.Options{
				CustomUserData: customUserData,
			},
		}
	case OutputOptionNodeadm:
		return bootstrap.Nodeadm{
			Options: bootstrap.Options{
				ClusterName:         c.ClusterName,
				ClusterEndpoint:     c.ClusterEndpoint,
				ClusterCIDR:         c.ClusterCIDR,
				KubeletConfig:       kubeletConfig,
				Taints:              taints,
				Labels:              labels,
				CABundle:            caBundle,
				CustomUserData:      customUserData,
				InstanceStorePolicy: instanceStorePolicy,
			},
		}
	}
}

func (c Custom) DescribeImageQuery(_ context.Context, _ ssm.Provider, _ string, _ string) (DescribeImageQuery, error) {
	return DescribeImageQuery{}, nil
}

func (c Custom) DefaultBlockDeviceMappings() []*v1.BlockDeviceMapping {
	// By returning nil, we ensure that EC2 will automatically choose the volumes defined by the AMI
	// and we don't need to describe the AMI ourselves.
	return nil
}

// EphemeralBlockDevice is the block device that the pods on the node will use. For an AMI of a custom family, this is unknown
// to us.
func (c Custom) EphemeralBlockDevice() *string {
	return nil
}
