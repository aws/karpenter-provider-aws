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

const (
	BottlerocketTomlClusterNameLabelKey        = "cluster-name"
	BottlerocketTomlApiServerLabelKey          = "api-server"
	BottlerocketTomlClusterCertificateLabelKey = "cluster-certificate"
	BottlerocketTomlNodeLabelsLabelKey         = "node-labels"
	BottlerocketTomlNodeTaintsLabelKey         = "node-taints"
	BottlerocketTomlKubernetesLabelKey         = "kubernetes"
	BottlerocketTomlBootstrapCommandsLabelKey  = "bootstrap-commands"
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
	settingsKubernetes := s.SettingsRaw[BottlerocketTomlKubernetesLabelKey].(map[string]interface{})
	settingsKubernetes[BottlerocketTomlClusterNameLabelKey] = b.ClusterName
	settingsKubernetes[BottlerocketTomlApiServerLabelKey] = b.ClusterEndpoint
	settingsKubernetes[BottlerocketTomlClusterCertificateLabelKey] = b.CABundle

	if settingsKubernetes[BottlerocketTomlNodeLabelsLabelKey] == nil {
		settingsKubernetes[BottlerocketTomlNodeLabelsLabelKey] = map[string]string{}
	}

	nodeLabelsMap := settingsKubernetes[BottlerocketTomlNodeLabelsLabelKey].(map[string]string)

	if err := mergo.Merge(&nodeLabelsMap, b.Labels, mergo.WithOverride); err != nil {
		return "", err
	}

	if settingsKubernetes[BottlerocketTomlNodeTaintsLabelKey] == nil {
		settingsKubernetes[BottlerocketTomlNodeTaintsLabelKey] = map[string][]string{}
	}

	for _, taint := range b.Taints {
		tomlTaint := settingsKubernetes[BottlerocketTomlNodeTaintsLabelKey].(map[string][]string)[taint.Key]
		tomlTaint = append(tomlTaint, fmt.Sprintf("%s:%s", taint.Value, taint.Effect))
	}

	if lo.FromPtr(b.InstanceStorePolicy) == v1.InstanceStorePolicyRAID0 {
		if s.SettingsRaw[BottlerocketTomlBootstrapCommandsLabelKey] == nil {
			s.SettingsRaw[BottlerocketTomlBootstrapCommandsLabelKey] = map[string]interface{}{}
		}
		mountInstanceStorageCmd := s.SettingsRaw[BottlerocketTomlBootstrapCommandsLabelKey].(map[string]interface{})["000-mount-instance-storage"]
		mountInstanceStorageCmd.(map[string]interface{})["commands"] = [][]string{{"apiclient", "ephemeral-storage", "init"}, {"apiclient", "ephemeral-storage", "bind", "--dirs", "/var/lib/containerd", "/var/lib/kubelet", "/var/log/pods"}}
		mountInstanceStorageCmd.(map[string]interface{})["essential"] = true
		mountInstanceStorageCmd.(map[string]interface{})["mode"] = BootstrapCommandModeAlways

	}
	script, err := s.MarshalTOML()
	if err != nil {
		return "", fmt.Errorf("constructing toml UserData %w", err)
	}
	return base64.StdEncoding.EncodeToString(script), nil
}
