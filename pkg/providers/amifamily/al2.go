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
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	corev1 "k8s.io/api/core/v1"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap"
	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"
)

type AL2 struct {
	DefaultFamily
	*Options
}

func dereferenceStringPointers(ptrs []*string) []string {
	strs := make([]string, 0, len(ptrs))
	for _, ptr := range ptrs {
		if ptr != nil {
			strs = append(strs, *ptr)
		}
	}
	return strs
}

func (a AL2) DescribeImageQuery(ctx context.Context, ssmProvider ssm.Provider, k8sVersion string, amiVersion string) (DescribeImageQuery, error) {
	imageIDs := make([]*string, 0, 5)
	requirements := make(map[string][]scheduling.Requirements)
	// Example Paths:
	// - Latest EKS 1.30 Standard Image: /aws/service/eks/optimized-ami/1.30/amazon-linux-2/recommended/image_id
	// - Specific EKS 1.30 GPU Image: /aws/service/eks/optimized-ami/1.30/amazon-linux-2-gpu/amazon-eks-node-1.30-v20240625/image_id
	for rootPath, variants := range map[string][]Variant{
		fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2", k8sVersion):       {VariantStandard},
		fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-arm64", k8sVersion): {VariantStandard},
		fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2-gpu", k8sVersion):   {VariantNeuron, VariantNvidia},
	} {
		results, err := ssmProvider.List(ctx, rootPath)
		if err != nil {
			log.FromContext(ctx).WithValues("path", rootPath, "family", "al2").Error(err, "discovering AMIs from ssm")
			continue
		}
		for path, value := range results {
			pathComponents := strings.Split(path, "/")
			// Only select image_id paths which match the desired AMI version
			if len(pathComponents) != 9 || pathComponents[8] != "image_id" {
				continue
			}
			if av, err := a.extractAMIVersion(pathComponents[7]); err != nil || av != amiVersion {
				continue
			}
			imageIDs = append(imageIDs, lo.ToPtr(value))
			requirements[value] = lo.Map(variants, func(v Variant, _ int) scheduling.Requirements { return v.Requirements() })
		}
	}
	// Failed to discover any AMIs, we should short circuit AMI discovery
	if len(imageIDs) == 0 {
		return DescribeImageQuery{}, fmt.Errorf(`failed to discover any AMIs for alias "al2@%s"`, amiVersion)
	}
	imageIDStrings := dereferenceStringPointers(imageIDs)
	return DescribeImageQuery{
		Filters: []ec2types.Filter{{
			Name:   lo.ToPtr("image-id"),
			Values: imageIDStrings,
		}},
		KnownRequirements: requirements,
	}, nil
}

func (a AL2) extractAMIVersion(versionStr string) (string, error) {
	if versionStr == "recommended" {
		return AMIVersionLatest, nil
	}
	rgx := regexp.MustCompile(`^.*(v\d{8})$`)
	matches := rgx.FindStringSubmatch(versionStr)
	if len(matches) != 2 {
		return "", fmt.Errorf("failed to extract AMI version")
	}
	return matches[1], nil
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
