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

package bootstrap

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/imdario/mergo"
	"github.com/samber/lo"

	"github.com/aws/aws-sdk-go-v2/aws"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

type Bottlerocket struct {
	Options
}

// nolint:gocyclo
func (b Bottlerocket) Script() (string, error) {
	s, err := NewBottlerocketConfig(b.CustomUserData)
	if err != nil {
		return "", fmt.Errorf("invalid UserData %w", err)
	}
	// Karpenter will overwrite settings present inside custom UserData
	// based on other fields specified in the NodePool
	s.Settings.Kubernetes.ClusterName = &b.ClusterName
	s.Settings.Kubernetes.APIServer = &b.ClusterEndpoint
	s.Settings.Kubernetes.ClusterCertificate = b.CABundle
	if err := mergo.MergeWithOverwrite(&s.Settings.Kubernetes.NodeLabels, b.Labels); err != nil {
		return "", err
	}

	if b.KubeletConfig != nil && b.KubeletConfig.MaxPods != nil {
		s.Settings.Kubernetes.MaxPods = aws.Int(int(lo.FromPtr(b.KubeletConfig.MaxPods)))
	}

	if b.KubeletConfig != nil {
		if len(b.KubeletConfig.ClusterDNS) > 0 {
			s.Settings.Kubernetes.ClusterDNSIP = b.KubeletConfig.ClusterDNS
		}
		if b.KubeletConfig.SystemReserved != nil {
			s.Settings.Kubernetes.SystemReserved = b.KubeletConfig.SystemReserved
		}
		if b.KubeletConfig.KubeReserved != nil {
			s.Settings.Kubernetes.KubeReserved = b.KubeletConfig.KubeReserved
		}
		if b.KubeletConfig.EvictionHard != nil {
			s.Settings.Kubernetes.EvictionHard = b.KubeletConfig.EvictionHard
		}
		if b.KubeletConfig.ImageGCHighThresholdPercent != nil {
			s.Settings.Kubernetes.ImageGCHighThresholdPercent = lo.ToPtr(strconv.FormatInt(int64(*b.KubeletConfig.ImageGCHighThresholdPercent), 10))
		}
		if b.KubeletConfig.ImageGCLowThresholdPercent != nil {
			s.Settings.Kubernetes.ImageGCLowThresholdPercent = lo.ToPtr(strconv.FormatInt(int64(*b.KubeletConfig.ImageGCLowThresholdPercent), 10))
		}
		if b.KubeletConfig.CPUCFSQuota != nil {
			s.Settings.Kubernetes.CPUCFSQuota = b.KubeletConfig.CPUCFSQuota
		}
	}

	s.Settings.Kubernetes.NodeTaints = map[string][]string{}
	for _, taint := range b.Taints {
		s.Settings.Kubernetes.NodeTaints[taint.Key] = append(s.Settings.Kubernetes.NodeTaints[taint.Key], fmt.Sprintf("%s:%s", taint.Value, taint.Effect))
	}

	if lo.FromPtr(b.InstanceStorePolicy) == v1.InstanceStorePolicyRAID0 {
		if s.Settings.BootstrapCommands == nil {
			s.Settings.BootstrapCommands = map[string]BootstrapCommand{}
		}
		s.Settings.BootstrapCommands["000-mount-instance-storage"] = BootstrapCommand{
			Commands:  [][]string{{"apiclient", "ephemeral-storage", "init"}, {"apiclient", "ephemeral-storage", "bind", "--dirs", "/var/lib/containerd", "/var/lib/kubelet", "/var/log/pods"}},
			Essential: true,
			Mode:      BootstrapCommandModeAlways,
		}
	}
	script, err := s.MarshalTOML()
	if err != nil {
		return "", fmt.Errorf("constructing toml UserData %w", err)
	}
	return base64.StdEncoding.EncodeToString(script), nil
}
