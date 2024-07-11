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
	"strings"

	"github.com/samber/lo"

	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap"
	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	"github.com/aws/aws-sdk-go/aws"
	corev1 "k8s.io/api/core/v1"
	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Bottlerocket struct {
	DefaultFamily
	*Options
}

func (b Bottlerocket) DescribeImageQuery(ctx context.Context, ssmProvider ssm.Provider, k8sVersion string, amiVersion string) (DescribeImageQuery, error) {
	imageIDs := make([]*string, 0, 5)
	requirements := make(map[string][]scheduling.Requirements)
	// Example Paths:
	// - Latest EKS 1.30 amd64 Standard Image: /aws/service/bottlerocket/aws-k8s-1.30/x86_64/latest/image_id
	// - Specific EKS 1.30 arm64 Nvidia Image: /aws/service/bottlerocket/aws-k8s-1.30-nvidia/arm64/1.10.0/image_id
	for rootPath, variants := range map[string][]Variant{
		fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s", k8sVersion):        []Variant{VariantStandard},
		fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia", k8sVersion): []Variant{VariantNeuron, VariantNvidia},
	} {
		results, err := ssmProvider.List(ctx, rootPath)
		if err != nil {
			log.FromContext(ctx).WithValues("path", rootPath).Error(err, "discovering AMIs from ssm")
			continue
		}
		for path, value := range results {
			pathComponents := strings.Split(path, "/")
			// Only select image_id paths which match the desired AMI version
			if len(pathComponents) != 8 || pathComponents[7] != "image_id" || pathComponents[6] != amiVersion {
				continue
			}
			imageIDs = append(imageIDs, lo.ToPtr(value))
			requirements[value] = lo.Map(variants, func(v Variant, _ int) scheduling.Requirements { return v.Requirements() })
		}
	}
	// Failed to discover any AMIs, we should short circuit AMI discovery
	if len(imageIDs) == 0 {
		return DescribeImageQuery{}, fmt.Errorf(`failed to discover any AMIs for alias "bottlerocket@%s"`, amiVersion)
	}
	return DescribeImageQuery{
		Filters: []*ec2.Filter{{
			Name:   lo.ToPtr("image-id"),
			Values: imageIDs,
		}},
		KnownRequirements: make(map[string][]scheduling.Requirements),
	}, nil
}

// UserData returns the default userdata script for the AMI Family
func (b Bottlerocket) UserData(kubeletConfig *v1.KubeletConfiguration, taints []corev1.Taint, labels map[string]string, caBundle *string, _ []*cloudprovider.InstanceType, customUserData *string, _ *v1.InstanceStorePolicy) bootstrap.Bootstrapper {
	return bootstrap.Bottlerocket{
		Options: bootstrap.Options{
			ClusterName:     b.Options.ClusterName,
			ClusterEndpoint: b.Options.ClusterEndpoint,
			KubeletConfig:   kubeletConfig,
			Taints:          taints,
			Labels:          labels,
			CABundle:        caBundle,
			CustomUserData:  customUserData,
		},
	}
}

// DefaultBlockDeviceMappings returns the default block device mappings for the AMI Family
func (b Bottlerocket) DefaultBlockDeviceMappings() []*v1.BlockDeviceMapping {
	xvdaEBS := DefaultEBS
	xvdaEBS.VolumeSize = lo.ToPtr(resource.MustParse("4Gi"))
	return []*v1.BlockDeviceMapping{
		{
			DeviceName: aws.String("/dev/xvda"),
			EBS:        &xvdaEBS,
		},
		{
			DeviceName: b.EphemeralBlockDevice(),
			EBS:        &DefaultEBS,
		},
	}
}

func (b Bottlerocket) EphemeralBlockDevice() *string {
	return aws.String("/dev/xvdb")
}

// PodsPerCoreEnabled is currently disabled for Bottlerocket AMIFamily because it does
// not currently support the podsPerCore parameter passed through the kubernetes settings TOML userData
// If a NodePool sets the podsPerCore value when using the Bottlerocket AMIFamily in the provider,
// podsPerCore will be ignored
// https://github.com/bottlerocket-os/bottlerocket/issues/1721

// EvictionSoftEnabled is currently disabled for Bottlerocket AMIFamily because it does
// not currently support the evictionSoft parameter passed through the kubernetes settings TOML userData
// If a NodePool sets the evictionSoft value when using the Bottlerocket AMIFamily in the provider,
// evictionSoft will be ignored
// https://github.com/bottlerocket-os/bottlerocket/issues/1445

func (b Bottlerocket) FeatureFlags() FeatureFlags {
	return FeatureFlags{
		UsesENILimitedMemoryOverhead: false,
		PodsPerCoreEnabled:           false,
		EvictionSoftEnabled:          false,
		SupportsENILimitedPodDensity: true,
	}
}
