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

package launchtemplate

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
)

type Bottlerocket struct {
	v1alpha1.BottlerocketOptions
}

func (p *Bottlerocket) GetLaunchTemplates(ctx context.Context, builder *Builder, config *Configuration, instanceTypes []cloudprovider.InstanceType) (map[Input][]cloudprovider.InstanceType, error) {
	return builder.PrepareLauncheTemplates(ctx, p, config, instanceTypes)
}

func (p *Bottlerocket) PrepareLaunchTemplate(ctx context.Context, builder *Builder, config *Configuration, ami *ec2.Image, instanceTypes []cloudprovider.InstanceType) (*ec2.RequestLaunchTemplateData, error) {
	return builder.Template(ctx, p, &p.BasicLaunchTemplateInput, config, ami, instanceTypes)
}

func (p *Bottlerocket) GetImageID(ctx context.Context, builder *Builder, config *Configuration, instanceType cloudprovider.InstanceType) (string, error) {
	if p.ImageID != nil {
		return *p.ImageID, nil
	}
	return builder.SsmClient.GetParameterWithContext(ctx, p.getBottlerocketAlias(config.KubernetesVersion, instanceType))
}

func (p *Bottlerocket) getBottlerocketAlias(k8sVersion K8sVersion, instanceType cloudprovider.InstanceType) string {
	arch := "x86_64"
	amiSuffix := ""
	if !instanceType.NvidiaGPUs().IsZero() {
		amiSuffix = "-nvidia"
	}
	if instanceType.Architecture() == v1alpha5.ArchitectureArm64 {
		arch = instanceType.Architecture()
	}
	return fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s%s/%s/latest/image_id", k8sVersion.String(), amiSuffix, arch)
}

func (p *Bottlerocket) GetUserData(_ context.Context, builder *Builder, config *Configuration, instanceTypes []cloudprovider.InstanceType) (*string, error) {
	userData := make([]string, 0)
	// [settings.kubernetes]
	userData = p.addKubeletConfiguration(userData, config)
	userData = p.addEvictionSettings(userData, config)
	userData = p.addTaintsAndLabels(userData, config)
	userData = p.addContainerRuntimeConfiguration(userData, config)
	userDataMerged := strings.Join(userData, "\n")
	return &userDataMerged, nil
}

//gocyclo:ignore
func (p *Bottlerocket) addKubeletConfiguration(userData []string, config *Configuration) []string {
	constraints := config.Constraints.Constraints
	userData = append(userData, `[settings.kubernetes]`)
	userData = append(userData, fmt.Sprintf(`cluster-name = "%s"`, config.ClusterName))
	userData = append(userData, fmt.Sprintf(`api-server = "%s"`, config.ClusterEndpoint))
	if config.CABundle != nil {
		userData = append(userData, fmt.Sprintf(`cluster-certificate = "%s"`, *config.CABundle))
	}
	if len(constraints.KubeletConfiguration.ClusterDNS) > 0 {
		userData = append(userData, fmt.Sprintf(`cluster-dns-ip = "%s"`, constraints.KubeletConfiguration.ClusterDNS[0]))
	}
	if constraints.KubeletConfiguration.EventRecordQPS != nil {
		userData = append(userData, fmt.Sprintf(`event-qps = %d`, *constraints.KubeletConfiguration.EventRecordQPS))
	}
	if constraints.KubeletConfiguration.EventBurst != nil {
		userData = append(userData, fmt.Sprintf(`event-burst = %d`, *constraints.KubeletConfiguration.EventBurst))
	}
	if constraints.KubeletConfiguration.RegistryPullQPS != nil {
		userData = append(userData, fmt.Sprintf(`registry-qps = %d`, *constraints.KubeletConfiguration.RegistryPullQPS))
	}
	if constraints.KubeletConfiguration.RegistryBurst != nil {
		userData = append(userData, fmt.Sprintf(`registry-burst = %d`, *constraints.KubeletConfiguration.RegistryBurst))
	}
	if constraints.KubeletConfiguration.KubeAPIQPS != nil {
		userData = append(userData, fmt.Sprintf(`kube-api-qps = %d`, *constraints.KubeletConfiguration.KubeAPIQPS))
	}
	if constraints.KubeletConfiguration.KubeAPIBurst != nil {
		userData = append(userData, fmt.Sprintf(`kube-api-burst = %d`, *constraints.KubeletConfiguration.KubeAPIBurst))
	}
	if constraints.KubeletConfiguration.ContainerLogMaxSize != nil && len(*constraints.KubeletConfiguration.ContainerLogMaxSize) > 0 {
		userData = append(userData, fmt.Sprintf(`container-log-max-size = "%s"`, *constraints.KubeletConfiguration.ContainerLogMaxSize))
	}
	if constraints.KubeletConfiguration.ContainerLogMaxFiles != nil {
		userData = append(userData, fmt.Sprintf(`container-log-max-files = %d`, *constraints.KubeletConfiguration.ContainerLogMaxFiles))
	}
	if len(constraints.KubeletConfiguration.AllowedUnsafeSysctls) > 0 {
		userData = append(userData, fmt.Sprintf(`allowed-unsafe-sysctls = ["%s"]`, strings.Join(constraints.KubeletConfiguration.AllowedUnsafeSysctls, `","`)))
	}
	return userData
}

func (p *Bottlerocket) addEvictionSettings(userData []string, config *Configuration) []string {
	constraints := config.Constraints.Constraints
	// [settings.kubernetes.eviction-hard]
	if len(constraints.KubeletConfiguration.EvictionHard) > 0 {
		userData = append(userData, `[settings.kubernetes.eviction-hard]`)
		for key, val := range constraints.KubeletConfiguration.EvictionHard {
			userData = append(userData, fmt.Sprintf(`"%s" = "%s"`, key, val))
		}
	}
	return userData
}

func (p *Bottlerocket) addTaintsAndLabels(userData []string, config *Configuration) []string {
	constraints := &config.Constraints.Constraints
	// [settings.kubernetes.node-taints]
	userData = append(userData, taints2BottlerocketFormat(constraints)...)
	// [settings.kubernetes.node-labels]
	if len(config.NodeLabels) > 0 {
		userData = append(userData, `[settings.kubernetes.node-labels]`)
		for key, val := range config.NodeLabels {
			userData = append(userData, fmt.Sprintf(`"%s" = "%s"`, key, val))
		}
	}
	return userData
}

func (p *Bottlerocket) addContainerRuntimeConfiguration(userData []string, config *Configuration) []string {
	constraints := &config.Constraints.Constraints
	if len(constraints.ContainerRuntimeConfiguration.RegistryMirrors) > 0 {
		for _, val := range constraints.ContainerRuntimeConfiguration.RegistryMirrors {
			userData = append(userData, `[[settings.container-registry.mirrors]]`)
			userData = append(userData, fmt.Sprintf(`registry = "%s"`, strings.TrimSpace(val.Registry)))
			endpoints := make([]string, 0)
			for _, ep := range val.Endpoints {
				endpoints = append(endpoints, fmt.Sprintf(`"%s"`, strings.TrimSpace(ep.URL)))
			}
			userData = append(userData, fmt.Sprintf(`endpoint = [%s]`, strings.Join(endpoints, ",")))
		}
	}
	return userData
}

func taints2BottlerocketFormat(constraints *v1alpha5.Constraints) []string {
	lines := make([]string, 0)
	if constraints != nil && len(constraints.Taints) > 0 {
		lines = append(lines, `[settings.kubernetes.node-taints]`)
		aggregated := make(map[string]map[string]bool)
		for _, taint := range constraints.Taints {
			var valueEffects map[string]bool
			var ok bool
			if valueEffects, ok = aggregated[taint.Key]; !ok {
				valueEffects = make(map[string]bool)
			}
			valueEffects[fmt.Sprintf(`"%s:%s"`, taint.Value, taint.Effect)] = true
			aggregated[taint.Key] = valueEffects
		}
		for key, values := range aggregated {
			valueEffect := make([]string, 0, len(values))
			for k := range values {
				valueEffect = append(valueEffect, k)
			}
			lines = append(lines, fmt.Sprintf(`"%s" = [%s]`, key, strings.Join(valueEffect, ",")))
		}
	}
	return lines
}
