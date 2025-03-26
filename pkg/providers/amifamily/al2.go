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

	"github.com/aws/aws-sdk-go-v2/aws"
	corev1 "k8s.io/api/core/v1"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"

	"sigs.k8s.io/karpenter/pkg/scheduling"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap"
	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"
)

type AL2 struct {
	DefaultFamily
	*Options
}

func (a AL2) DescribeImageQuery(ctx context.Context, ssmProvider ssm.Provider, k8sVersion string, amiVersion string) (DescribeImageQuery, error) {
	ids := map[string][]Variant{}
	var ssmErr error
	for path, variants := range map[string][]Variant{
		fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/%s/image_id", k8sVersion, lo.Ternary(
			amiVersion == v1.AliasVersionLatest,
			"recommended",
			fmt.Sprintf("amazon-eks-node-%s-%s", k8sVersion, amiVersion),
		)): {VariantStandard},
		fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-arm64/%s/image_id", k8sVersion, lo.Ternary(
			amiVersion == v1.AliasVersionLatest,
			"recommended",
			fmt.Sprintf("amazon-eks-arm64-node-%s-%s", k8sVersion, amiVersion),
		)): {VariantStandard},
		fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-gpu/%s/image_id", k8sVersion, lo.Ternary(
			amiVersion == v1.AliasVersionLatest,
			"recommended",
			fmt.Sprintf("amazon-eks-gpu-node-%s-%s", k8sVersion, amiVersion),
		)): {VariantNeuron, VariantNvidia},
	} {
		imageID, err := ssmProvider.Get(ctx, ssm.Parameter{
			Name:      path,
			IsMutable: amiVersion == v1.AliasVersionLatest,
		})
		if err != nil {
			ssmErr = err
			continue
		}
		ids[imageID] = variants
	}
	// Failed to discover any AMIs, we should short circuit AMI discovery
	if len(ids) == 0 {
		return DescribeImageQuery{}, awserrors.DescribeImageError(fmt.Sprintf("al2@%s", amiVersion), ssmErr)
	}

	return DescribeImageQuery{
		Filters: []ec2types.Filter{{
			Name:   lo.ToPtr("image-id"),
			Values: lo.Keys(ids),
		}},
		KnownRequirements: lo.MapValues(ids, func(variants []Variant, _ string) []scheduling.Requirements {
			return lo.Map(variants, func(v Variant, _ int) scheduling.Requirements { return v.Requirements() })
		}),
	}, nil
}

// UserData returns the exact same string for equivalent input,
// even if elements of those inputs are in differing orders,
// guaranteeing it won't cause spurious hash differences.
// AL2 userdata also works on Ubuntu
func (a AL2) UserData(kubeletConfig *v1.KubeletConfiguration, taints []corev1.Taint, labels map[string]string, caBundle *string, _ []*cloudprovider.InstanceType, customUserData *string, instanceStorePolicy *v1.InstanceStorePolicy) bootstrap.Bootstrapper {
	return bootstrap.EKS{
		Options: bootstrap.Options{
			ClusterName:         a.Options.ClusterName,
			ClusterEndpoint:     a.Options.ClusterEndpoint,
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
func (a AL2) DefaultBlockDeviceMappings() []*v1.BlockDeviceMapping {
	return []*v1.BlockDeviceMapping{{
		DeviceName: a.EphemeralBlockDevice(),
		EBS:        &DefaultEBS,
	}}
}

func (a AL2) EphemeralBlockDevice() *string {
	return aws.String("/dev/xvda")
}
