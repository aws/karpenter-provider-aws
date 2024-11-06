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
	"fmt"

	"sigs.k8s.io/karpenter/pkg/scheduling"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap"
	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
)

type Windows struct {
	DefaultFamily
	*Options
	// Version is the major version of Windows Server (2019 or 2022).
	// Only the core version of each version is supported by Karpenter, so this field only indicates the year.
	Version string
	// Build is a specific build code associated with the Version
	Build string
}

func (w Windows) DescribeImageQuery(ctx context.Context, ssmProvider ssm.Provider, k8sVersion string, amiVersion string) (DescribeImageQuery, error) {
	imageID, err := ssmProvider.Get(ctx, ssm.Parameter{
		Name:      fmt.Sprintf("/aws/service/ami-windows-latest/Windows_Server-%s-English-%s-EKS_Optimized-%s/image_id", w.Version, v1.WindowsCore, k8sVersion),
		IsMutable: true,
	})
	if err != nil {
		return DescribeImageQuery{}, fmt.Errorf(`failed to discover any AMIs for alias "windows%s@%s"`, w.Version, amiVersion)
	}
	return DescribeImageQuery{
		Filters: []ec2types.Filter{{
			Name:   lo.ToPtr("image-id"),
			Values: []string{imageID},
		}},
		KnownRequirements: map[string][]scheduling.Requirements{
			imageID: {scheduling.NewRequirements(
				scheduling.NewRequirement(corev1.LabelOSStable, corev1.NodeSelectorOpIn, string(corev1.Windows)),
				scheduling.NewRequirement(corev1.LabelWindowsBuild, corev1.NodeSelectorOpIn, w.Build),
			)},
		},
	}, nil
}

// UserData returns the default userdata script for the AMI Family
func (w Windows) UserData(kubeletConfig *v1.KubeletConfiguration, taints []corev1.Taint, labels map[string]string, caBundle *string, _ []*cloudprovider.InstanceType, customUserData *string, _ *v1.InstanceStorePolicy) bootstrap.Bootstrapper {
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
func (w Windows) DefaultBlockDeviceMappings() []*v1.BlockDeviceMapping {
	sda1EBS := DefaultEBS
	sda1EBS.VolumeSize = lo.ToPtr(resource.MustParse("50Gi"))
	return []*v1.BlockDeviceMapping{{
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
