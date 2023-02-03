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

	"knative.dev/pkg/ptr"

	"github.com/aws/karpenter-core/pkg/utils/resources"

	"github.com/aws/aws-sdk-go/aws"
)

type Bottlerocket struct {
	Options
}

func (b Bottlerocket) Script() (string, error) {
	s, err := NewBottlerocketConfig(b.CustomUserData)
	if err != nil {
		return "", fmt.Errorf("invalid UserData %w", err)
	}
	// Karpenter will overwrite settings present inside custom UserData
	// based on other fields specified in the provisioner
	s.Settings.Kubernetes.ClusterName = &b.ClusterName
	s.Settings.Kubernetes.APIServer = &b.ClusterEndpoint
	s.Settings.Kubernetes.ClusterCertificate = b.CABundle
	s.Settings.Kubernetes.NodeLabels = b.Labels

	// Backwards compatibility for AWSENILimitedPodDensity flag
	if b.KubeletConfig != nil && b.KubeletConfig.MaxPods != nil {
		s.Settings.Kubernetes.MaxPods = aws.Int(int(ptr.Int32Value(b.KubeletConfig.MaxPods)))
	} else if !b.AWSENILimitedPodDensity {
		s.Settings.Kubernetes.MaxPods = aws.Int(110)
	}

	if b.KubeletConfig != nil && len(b.KubeletConfig.ClusterDNS) > 0 {
		s.Settings.Kubernetes.ClusterDNSIP = &b.KubeletConfig.ClusterDNS[0]
	}
	if b.KubeletConfig != nil {
		s.Settings.Kubernetes.SystemReserved = resources.StringMap(b.KubeletConfig.SystemReserved)
		s.Settings.Kubernetes.KubeReserved = resources.StringMap(b.KubeletConfig.KubeReserved)
		s.Settings.Kubernetes.EvictionHard = b.KubeletConfig.EvictionHard
		s.Settings.Kubernetes.ImageGCLowThresholdPercent = b.KubeletConfig.ImageGCLowThresholdPercent
		s.Settings.Kubernetes.ImageGCHighThresholdPercent = b.KubeletConfig.ImageGCHighThresholdPercent
	}

	s.Settings.Kubernetes.NodeTaints = map[string][]string{}
	for _, taint := range b.Taints {
		s.Settings.Kubernetes.NodeTaints[taint.Key] = append(s.Settings.Kubernetes.NodeTaints[taint.Key], fmt.Sprintf("%s:%s", taint.Value, taint.Effect))
	}
	script, err := s.MarshalTOML()
	if err != nil {
		return "", fmt.Errorf("constructing toml UserData %w", err)
	}
	return base64.StdEncoding.EncodeToString(script), nil
}
