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

	"github.com/imdario/mergo"
	"github.com/samber/lo"

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
	settingsKubernetes := s.GetKubernetesSettings()
	settingsKubernetes["cluster-name"] = b.ClusterName
	settingsKubernetes["api-server"] = b.ClusterEndpoint
	settingsKubernetes["cluster-certificate"] = b.CABundle

	if b.KubeletConfig != nil && len(b.KubeletConfig.ClusterDNS) > 0 {
		settingsKubernetes["cluster-dns-ip"] = b.KubeletConfig.ClusterDNS[0]
	}

	nodeLabelsMap := s.GetCustomSettingsAsMap(settingsKubernetes, "node-labels")
	nodeLabelAsMapString := make(map[string]string)
	for k, v := range nodeLabelsMap {
		nodeLabelAsMapString[k] = v.(string)
	}
	if err := mergo.Merge(&nodeLabelAsMapString, b.Labels, mergo.WithOverride); err != nil {
		return "", err
	}
	settingsKubernetes["node-labels"] = nodeLabelAsMapString

	nodTaintsAsMapSliceString := make(map[string][]string)
	for _, taint := range b.Taints {
		nodTaintsAsMapSliceString[taint.Key] = append(nodTaintsAsMapSliceString[taint.Key], fmt.Sprintf("%s:%s", taint.Value, taint.Effect))
	}
	settingsKubernetes["node-taints"] = nodTaintsAsMapSliceString

	if lo.FromPtr(b.InstanceStorePolicy) == v1.InstanceStorePolicyRAID0 {
		bootstrapCommands := make(map[string]interface{})
		bootstrapCommands["commands"] = [][]string{{"apiclient", "ephemeral-storage", "init"}, {"apiclient", "ephemeral-storage", "bind", "--dirs", "/var/lib/containerd", "/var/lib/kubelet", "/var/log/pods"}}
		bootstrapCommands["mode"] = BootstrapCommandModeAlways
		bootstrapCommands["essential"] = true

		settingsBootstrapCommand := s.GetCustomSettingsAsMap(s.SettingsRaw, "bootstrap-commands")
		settingsBootstrapCommand["000-mount-instance-storage"] = bootstrapCommands
		s.SettingsRaw["bootstrap-commands"] = settingsBootstrapCommand
	}

	script, err := s.MarshalTOML()
	if err != nil {
		return "", fmt.Errorf("constructing toml UserData %w", err)
	}
	return base64.StdEncoding.EncodeToString(script), nil
}
