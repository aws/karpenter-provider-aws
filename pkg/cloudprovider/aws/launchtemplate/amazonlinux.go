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
	"bytes"
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
)

type Amazonlinux struct {
	v1alpha1.AmazonlinuxOptions
}

func (p *Amazonlinux) GetLaunchTemplates(ctx context.Context, builder *Builder, config *Configuration, instanceTypes []cloudprovider.InstanceType) (map[Input][]cloudprovider.InstanceType, error) {
	return builder.PrepareLauncheTemplates(ctx, p, config, instanceTypes)
}

func (p *Amazonlinux) PrepareLaunchTemplate(ctx context.Context, builder *Builder, config *Configuration, ami *ec2.Image, instanceTypes []cloudprovider.InstanceType) (*ec2.RequestLaunchTemplateData, error) {
	return builder.Template(ctx, p, &p.BasicLaunchTemplateInput, config, ami, instanceTypes)
}

func (p *Amazonlinux) GetImageID(ctx context.Context, builder *Builder, config *Configuration, instanceType cloudprovider.InstanceType) (string, error) {
	if p.ImageID != nil {
		return *p.ImageID, nil
	}
	if p.Version == nil {
		// Search for most recent bi-yearly Amazon Linux version
		year, _, _ := time.Now().Date()
		if year%2 == 1 {
			year--
		}
		// Amazon Linux 2022 was the first bi-yearly version, so stop searching once we hit that one.
		for year >= 2022 {
			id, err := builder.SsmClient.GetParameterWithContext(ctx, p.getAL2Alias(config.KubernetesVersion, strconv.Itoa(year), instanceType))
			if err == nil {
				return id, nil
			}
			year -= 2
		}
		// Fallback to Amazon Linux 2
		return builder.SsmClient.GetParameterWithContext(ctx, p.getAL2Alias(config.KubernetesVersion, "2", instanceType))
	}
	return builder.SsmClient.GetParameterWithContext(ctx, p.getAL2Alias(config.KubernetesVersion, *p.Version, instanceType))
}

func (p *Amazonlinux) getAL2Alias(k8sVersion K8sVersion, amazonLinuxVersion string, instanceType cloudprovider.InstanceType) string {
	amiSuffix := ""
	if !instanceType.NvidiaGPUs().IsZero() || !instanceType.AWSNeurons().IsZero() {
		amiSuffix = "-gpu"
	} else if instanceType.Architecture() == v1alpha5.ArchitectureArm64 {
		amiSuffix = fmt.Sprintf("-%s", instanceType.Architecture())
	}
	return fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-%s%s/recommended/image_id", k8sVersion.String(), amazonLinuxVersion, amiSuffix)
}

func (p *Amazonlinux) GetUserData(ctx context.Context, builder *Builder, config *Configuration, instanceTypes []cloudprovider.InstanceType) (*string, error) {
	return getALandUbuntuUserData(config, instanceTypes)
}

// getALandUbuntuUserData returns the exact same string for equivalent input,
// even if elements of those inputs are in differing orders,
// guaranteeing it won't cause spurious hash differences.
// AL2 userdata also works on Ubuntu
func getALandUbuntuUserData(config *Configuration, instanceTypes []cloudprovider.InstanceType) (*string, error) {
	constraints := config.Constraints
	if constraints != nil {
		if len(constraints.ContainerRuntimeConfiguration.RegistryMirrors) > 0 {
			return nil, fmt.Errorf("containerRuntimeConfiguration.registryMirrors is not (yet) supported for Amazon Linux 2")
		}
	}
	userData := make([]string, 0)
	userData = append(userData, `#!/bin/bash -xe`)
	userData = append(userData, `exec > >(tee /var/log/user-data.log|logger -t user-data -s 2>/dev/console) 2>&1`)
	bootstrapArgs := addBootstrapArgs(config, instanceTypes)
	userData = append(userData, fmt.Sprintf(`/etc/eks/bootstrap.sh %s`, strings.Join(bootstrapArgs, " ")))
	userDataMerged := strings.Join(userData, "\n")
	return &userDataMerged, nil
}

// needsDocker returns true if the instance type is unable to use
// containerd directly
func needsDocker(is []cloudprovider.InstanceType) bool {
	for _, i := range is {
		if !i.AWSNeurons().IsZero() || !i.NvidiaGPUs().IsZero() {
			return true
		}
	}
	return false
}

func addBootstrapArgs(config *Configuration, instanceTypes []cloudprovider.InstanceType) []string {
	constraints := config.Constraints.Constraints
	bootstrapArgs := make([]string, 0)
	bootstrapArgs = append(bootstrapArgs, config.ClusterName)
	bootstrapArgs = append(bootstrapArgs, `--apiserver-endpoint`, config.ClusterEndpoint)
	if !needsDocker(instanceTypes) {
		bootstrapArgs = append(bootstrapArgs, `--container-runtime`, `containerd`)
	}
	if config.CABundle != nil {
		bootstrapArgs = append(bootstrapArgs, `--b64-cluster-ca`, *config.CABundle)
	}
	if len(constraints.KubeletConfiguration.ClusterDNS) > 0 {
		bootstrapArgs = append(bootstrapArgs, `--dns-cluster-ip`, fmt.Sprintf(`'%s'`, constraints.KubeletConfiguration.ClusterDNS[0]))
	}
	if !config.AWSENILimitedPodDensity {
		bootstrapArgs = append(bootstrapArgs, `--use-max-pods=false`)
	}
	kubeletExtraArgs := addKubeletExtraArgs(config)
	if len(kubeletExtraArgs) > 0 {
		bootstrapArgs = append(bootstrapArgs, `--kubelet-extra-args`, fmt.Sprintf(`'%s'`, strings.Join(kubeletExtraArgs, " ")))
	}
	return bootstrapArgs
}

//gocyclo:ignore
func addKubeletExtraArgs(config *Configuration) []string {
	constraints := &config.Constraints.Constraints
	kubeletExtraArgs := make([]string, 0)
	nodeLabelArgs := getNodeLabelArgs(config.NodeLabels)
	if len(nodeLabelArgs) > 0 {
		kubeletExtraArgs = append(kubeletExtraArgs, nodeLabelArgs)
	}
	if !config.AWSENILimitedPodDensity {
		kubeletExtraArgs = append(kubeletExtraArgs, `--max-pods=110`)
	}
	nodeTaintsArgs := getNodeTaintArgs(constraints)
	if len(nodeTaintsArgs) > 0 {
		kubeletExtraArgs = append(kubeletExtraArgs, nodeTaintsArgs)
	}
	if constraints.KubeletConfiguration.EventRecordQPS != nil {
		qps := *constraints.KubeletConfiguration.EventRecordQPS
		if qps == 0 {
			// On the CLI kubelet will use the default value if "0" is provided, in kubelet config file "0"
			// means "no-limit". Here we want to mimic the kubelet config file and thus we replace "0" with
			// the max value of an int32 to achieve "no-limit" behavior.
			qps = math.MaxInt32
		}
		kubeletExtraArgs = append(kubeletExtraArgs, fmt.Sprintf(`--event-qps=%d`, qps))
	}
	if constraints.KubeletConfiguration.EventBurst != nil {
		kubeletExtraArgs = append(kubeletExtraArgs, fmt.Sprintf(`--event-burst=%d`, *constraints.KubeletConfiguration.EventBurst))
	}
	if constraints.KubeletConfiguration.RegistryPullQPS != nil {
		kubeletExtraArgs = append(kubeletExtraArgs, fmt.Sprintf(`--registry-qps=%d`, *constraints.KubeletConfiguration.RegistryPullQPS))
	}
	if constraints.KubeletConfiguration.RegistryBurst != nil {
		kubeletExtraArgs = append(kubeletExtraArgs, fmt.Sprintf(`--registry-burst=%d`, *constraints.KubeletConfiguration.RegistryBurst))
	}
	if constraints.KubeletConfiguration.KubeAPIQPS != nil {
		kubeletExtraArgs = append(kubeletExtraArgs, fmt.Sprintf(`--kube-api-qps=%d`, *constraints.KubeletConfiguration.KubeAPIQPS))
	}
	if constraints.KubeletConfiguration.KubeAPIBurst != nil {
		kubeletExtraArgs = append(kubeletExtraArgs, fmt.Sprintf(`--kube-api-burst=%d`, *constraints.KubeletConfiguration.KubeAPIBurst))
	}
	if constraints.KubeletConfiguration.ContainerLogMaxSize != nil && len(*constraints.KubeletConfiguration.ContainerLogMaxSize) > 0 {
		kubeletExtraArgs = append(kubeletExtraArgs, `--container-log-max-size`, *constraints.KubeletConfiguration.ContainerLogMaxSize)
	}
	if constraints.KubeletConfiguration.ContainerLogMaxFiles != nil {
		kubeletExtraArgs = append(kubeletExtraArgs, fmt.Sprintf(`--container-log-max-files=%d`, *constraints.KubeletConfiguration.ContainerLogMaxFiles))
	}
	if len(constraints.KubeletConfiguration.AllowedUnsafeSysctls) > 0 {
		kubeletExtraArgs = append(kubeletExtraArgs, fmt.Sprintf(`--allowed-unsafe-sysctls="%s"`, strings.Join(constraints.KubeletConfiguration.AllowedUnsafeSysctls, ",")))
	}
	if len(constraints.KubeletConfiguration.EvictionHard) > 0 {
		entries := make([]string, 0)
		for _, key := range sortedKeys(constraints.KubeletConfiguration.EvictionHard) {
			if val, found := constraints.KubeletConfiguration.EvictionHard[key]; found {
				entries = append(entries, fmt.Sprintf(`%s=%s`, key, val))
			}
		}
		kubeletExtraArgs = append(kubeletExtraArgs, fmt.Sprintf(`--eviction-hard="%s"`, strings.Join(entries, ",")))
	}
	return kubeletExtraArgs
}

func getNodeLabelArgs(nodeLabels map[string]string) string {
	nodeLabelArgs := ""
	if len(nodeLabels) > 0 {
		labelStrings := []string{}
		// Must be in sorted order or else equivalent options won't
		// hash the same
		for _, k := range sortedKeys(nodeLabels) {
			if v1alpha5.AllowedLabelDomains.Has(k) {
				continue
			}
			labelStrings = append(labelStrings, fmt.Sprintf("%s=%v", k, nodeLabels[k]))
		}
		nodeLabelArgs = fmt.Sprintf("--node-labels=%s", strings.Join(labelStrings, ","))
	}
	return nodeLabelArgs
}

func getNodeTaintArgs(constraints *v1alpha5.Constraints) string {
	var nodeTaintsArgs bytes.Buffer
	if len(constraints.Taints) > 0 {
		nodeTaintsArgs.WriteString("--register-with-taints=")
		first := true
		// Must be in sorted order or else equivalent options won't
		// hash the same.
		sorted := sortedTaints(constraints.Taints)
		for _, taint := range sorted {
			if !first {
				nodeTaintsArgs.WriteString(",")
			}
			first = false
			nodeTaintsArgs.WriteString(fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect))
		}
	}
	return nodeTaintsArgs.String()
}
