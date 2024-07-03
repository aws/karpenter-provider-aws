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

	"sigs.k8s.io/karpenter/pkg/scheduling"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap"
	"github.com/aws/karpenter-provider-aws/pkg/providers/ssm"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
)

type Windows struct {
	DefaultFamily
	*Options
	Version string
	Build   string
}

func (w Windows) AMIQuery(ctx context.Context, ssmProvider ssm.Provider, k8sVersion string, amiVersion string) (AMIQuery, error) {
	query := AMIQuery{
		Filters: []*ec2.Filter{&ec2.Filter{
			Name: lo.ToPtr("image-id"),
		}},
		KnownRequirements: make(map[string][]scheduling.Requirements),
	}
	// SSM aliases are only maintained for the latest Windows AMI releases
	if amiVersion != AMIVersionLatest {
		return AMIQuery{}, fmt.Errorf("discovering AMIs for alias, %q is an invalid version for Windows", amiVersion)
	}
	results, err := ssmProvider.List(ctx, "/aws/service/ami-windows-latest")
	if err != nil {
		return AMIQuery{}, fmt.Errorf("discovering AMIs from ssm")
	}
	for path, value := range results {
		pathComponents := strings.Split(path, "/")
		if len(pathComponents) != 6 {
			continue
		}
		matches := regexp.MustCompile(`^Windows_Server-(\d+)-English-Core-EKS_Optimized-(\d\.\d+)$`).FindStringSubmatch(pathComponents[4])
		if len(matches) != 3 || matches[1] != w.Version || matches[2] != k8sVersion {
			continue
		}
		query.Filters[0].Values = append(query.Filters[0].Values, lo.ToPtr(value))
		query.KnownRequirements[value] = []scheduling.Requirements{scheduling.NewRequirements(
			scheduling.NewRequirement(corev1.LabelOSStable, corev1.NodeSelectorOpIn, string(corev1.Windows)),
			scheduling.NewRequirement(corev1.LabelWindowsBuild, corev1.NodeSelectorOpIn, w.Build),
		)}
	}
	// Failed to discover any AMIs, we should short circuit AMI discovery
	if len(query.Filters[0].Values) == 0 {
		return AMIQuery{}, fmt.Errorf("failed to discover any AMIs for alias")
	}
	return query, nil
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
