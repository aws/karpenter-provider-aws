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

	"github.com/awslabs/operatorpkg/serrors"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap"
	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"
)

type AL2023 struct {
	DefaultFamily
	*Options
}

func (a AL2023) DescribeImageQuery(ctx context.Context, ssmProvider ssm.Provider, k8sVersion string, amiVersion string) (DescribeImageQuery, error) {
	ids := map[string]Variant{}
	for arch, variants := range map[string][]Variant{
		"x86_64": {VariantStandard, VariantNvidia, VariantNeuron},
		"arm64":  {VariantStandard, VariantNvidia},
	} {
		for _, variant := range variants {
			path := a.resolvePath(arch, string(variant), k8sVersion, amiVersion)
			imageID, err := ssmProvider.Get(ctx, ssm.Parameter{
				Name:      path,
				IsMutable: amiVersion == v1.AliasVersionLatest,
			})
			if err != nil {
				continue
			}
			ids[imageID] = variant
		}
	}
	// Failed to discover any AMIs, we should short circuit AMI discovery
	if len(ids) == 0 {
		return DescribeImageQuery{}, serrors.Wrap(fmt.Errorf("failed to discover any AMIs for alias"), "alias", fmt.Sprintf("al2023@%s", amiVersion))
	}

	return DescribeImageQuery{
		Filters: []ec2types.Filter{{
			Name:   lo.ToPtr("image-id"),
			Values: lo.Keys(ids),
		}},
		KnownRequirements: lo.MapValues(ids, func(v Variant, _ string) []scheduling.Requirements {
			return []scheduling.Requirements{v.Requirements()}
		}),
	}, nil
}

func (a AL2023) resolvePath(architecture, variant, k8sVersion, amiVersion string) string {
	name := lo.Ternary(
		amiVersion == v1.AliasVersionLatest,
		"recommended",
		fmt.Sprintf("amazon-eks-node-al2023-%s-%s-%s-%s", architecture, variant, k8sVersion, amiVersion),
	)
	return fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/%s/%s/%s/image_id", k8sVersion, architecture, variant, name)
}

func (a AL2023) UserData(kubeletConfig *v1.KubeletConfiguration, taints []corev1.Taint, labels map[string]string, caBundle *string, _ []*cloudprovider.InstanceType, customUserData *string, instanceStorePolicy *v1.InstanceStorePolicy) bootstrap.Bootstrapper {
	return bootstrap.Nodeadm{
		Options: bootstrap.Options{
			ClusterName:         a.ClusterName,
			ClusterEndpoint:     a.ClusterEndpoint,
			ClusterCIDR:         a.ClusterCIDR,
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
func (a AL2023) DefaultBlockDeviceMappings() []*v1.BlockDeviceMapping {
	return []*v1.BlockDeviceMapping{{
		DeviceName: a.EphemeralBlockDevice(),
		EBS:        &DefaultEBS,
	}}
}

func (a AL2023) EphemeralBlockDevice() *string {
	return lo.ToPtr("/dev/xvda")
}
