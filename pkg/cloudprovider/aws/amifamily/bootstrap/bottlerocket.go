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

	"github.com/pelletier/go-toml/v2"
)

type Bottlerocket struct {
	Options
}

// config is the root of the bottlerocket config, see more here https://github.com/bottlerocket-os/bottlerocket#using-user-data
type config struct {
	Settings settings `toml:"settings"`
}

// settings is part of the bottlerocket config
type settings struct {
	Kubernetes kubernetes `toml:"kubernetes"`
}

// kubernetes specific configuration for bottlerocket api
type kubernetes struct {
	APIServer          string              `toml:"api-server"`
	ClusterCertificate *string             `toml:"cluster-certificate"`
	ClusterName        string              `toml:"cluster-name,omitempty"`
	ClusterDNSIP       string              `toml:"cluster-dns-ip,omitempty"`
	NodeLabels         map[string]string   `toml:"node-labels,omitempty"`
	NodeTaints         map[string][]string `toml:"node-taints,omitempty"`
	MaxPods            int                 `toml:"max-pods,omitempty"`
}

func (b Bottlerocket) Script() string {
	s := config{Settings: settings{
		Kubernetes: kubernetes{
			ClusterName:        b.ClusterName,
			APIServer:          b.ClusterEndpoint,
			ClusterCertificate: b.CABundle,
			NodeLabels:         b.Labels,
		},
	}}
	if b.KubeletConfig != nil && len(b.KubeletConfig.ClusterDNS) > 0 {
		s.Settings.Kubernetes.ClusterDNSIP = b.KubeletConfig.ClusterDNS[0]
	}
	if !b.AWSENILimitedPodDensity {
		s.Settings.Kubernetes.MaxPods = 110
	}
	s.Settings.Kubernetes.NodeTaints = map[string][]string{}
	for _, taint := range b.Taints {
		s.Settings.Kubernetes.NodeTaints[taint.Key] = append(s.Settings.Kubernetes.NodeTaints[taint.Key], fmt.Sprintf("%s:%s", taint.Value, taint.Effect))
	}
	script, err := toml.Marshal(s)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(script)
}
