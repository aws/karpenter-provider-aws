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

	"github.com/awslabs/operatorpkg/serrors"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap"
	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"
	"github.com/aws/karpenter-provider-aws/pkg/providers/version"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Bottlerocket struct {
	DefaultFamily
	*Options
}

func (b Bottlerocket) DescribeImageQuery(ctx context.Context, ssmProvider ssm.Provider, k8sVersion string, amiVersion string) (DescribeImageQuery, error) {
	// Bottlerocket AMIs versions are prefixed with a v on GitHub, but not in the SSM path. We should accept both.
	trimmedAMIVersion := strings.TrimLeft(amiVersion, "v")
	ids := map[string][]Variant{}
	for path, variants := range map[string][]Variant{
		fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/x86_64/%s/image_id", k8sVersion, trimmedAMIVersion):        {VariantStandard, VariantNeuron},
		fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s/arm64/%s/image_id", k8sVersion, trimmedAMIVersion):         {VariantStandard},
		fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia/x86_64/%s/image_id", k8sVersion, trimmedAMIVersion): {VariantNvidia},
		fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s-nvidia/arm64/%s/image_id", k8sVersion, trimmedAMIVersion):  {VariantNvidia},
	} {
		imageID, err := ssmProvider.Get(ctx, ssm.Parameter{
			Name:      path,
			IsMutable: amiVersion == v1.AliasVersionLatest,
		})
		if err != nil {
			continue
		}
		ids[imageID] = variants
	}
	// Failed to discover any AMIs, we should short circuit AMI discovery
	if len(ids) == 0 {
		return DescribeImageQuery{}, serrors.Wrap(fmt.Errorf(`failed to discover any AMIs for alias`), "alias", fmt.Sprintf("bottlerocket@%s", amiVersion))
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

// UserData returns the default userdata script for the AMI Family
func (b Bottlerocket) UserData(kubeletConfig *v1.KubeletConfiguration, taints []corev1.Taint, labels map[string]string, caBundle *string, _ []*cloudprovider.InstanceType, customUserData *string, instanceStorePolicy *v1.InstanceStorePolicy) bootstrap.Bootstrapper {
	return bootstrap.Bottlerocket{
		Options: bootstrap.Options{
			ClusterName:         b.ClusterName,
			ClusterEndpoint:     b.ClusterEndpoint,
			KubeletConfig:       kubeletConfig,
			Taints:              taints,
			Labels:              labels,
			CABundle:            caBundle,
			CustomUserData:      customUserData,
			InstanceStorePolicy: instanceStorePolicy,
		},
		EnableDefaultMountPaths: version.SupportsDefaultBind(b.resolveAMIVersion()),
	}
}

// resolveAMIVersion extracts AMI version from selector terms
func (b Bottlerocket) resolveAMIVersion() string {
	if version := b.getVersionFromAlias(); version != "" {
		return version
	}
	if version := b.getVersionFromName(); version != "" {
		return version
	}
	if version := b.getVersionFromID(); version != "" {
		return version
	}
	if version := b.getVersionFromResolvedAMIs(); version != "" {
		return version
	}
	return ""
}

func (b Bottlerocket) getVersionFromAlias() string {
	for _, term := range b.AMISelectorTerms {
		if term.Alias == "" {
			continue
		}
		parts := strings.Split(term.Alias, "@")
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return ""
}

func (b Bottlerocket) getVersionFromName() string {
	for _, term := range b.AMISelectorTerms {
		if term.Name == "" {
			continue
		}
		if version := b.extractVersionFromName(term.Name); version != "" {
			return version
		}
	}
	return ""
}

func (b Bottlerocket) getVersionFromID() string {
	for _, term := range b.AMISelectorTerms {
		if term.ID == "" {
			continue
		}
		if version := b.findVersionByID(term.ID); version != "" {
			return version
		}
	}
	return ""
}

func (b Bottlerocket) findVersionByID(id string) string {
	for _, ami := range b.AMIs {
		if ami.ID == id && ami.Name != "" && strings.Contains(ami.Name, "bottlerocket") {
			return b.extractVersionFromName(ami.Name)
		}
	}
	return ""
}

func (b Bottlerocket) getVersionFromResolvedAMIs() string {
	for _, ami := range b.AMIs {
		if ami.Name == "" || !strings.Contains(ami.Name, "bottlerocket") {
			continue
		}
		if version := b.extractVersionFromName(ami.Name); version != "" {
			return version
		}
	}
	return ""
}

func (b Bottlerocket) extractVersionFromName(name string) string {
	// Bottlerocket pattern: bottlerocket-aws-k8s-1.33-x86_64-v1.46.0-431fe75a
	parts := strings.SplitSeq(name, "-")
	for part := range parts {
		if strings.HasPrefix(part, "v") && strings.Contains(part, ".") {
			return part
		}
	}
	return ""
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
		SupportsENILimitedPodDensity: true,
	}
}
