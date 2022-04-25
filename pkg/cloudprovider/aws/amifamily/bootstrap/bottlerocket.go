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
	"bytes"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/pelletier/go-toml/v2"
)

type Bottlerocket struct {
	Options
}

func (b Bottlerocket) Script() string {
	s := b.unmarshalCustomUserData()
	// Karpenter will overwrite settings present inside custom UserData
	// based on other fields specified in the provisioner
	s.Settings.Kubernetes.ClusterName = &b.ClusterName
	s.Settings.Kubernetes.APIServer = b.ClusterEndpoint
	s.Settings.Kubernetes.ClusterCertificate = b.CABundle
	s.Settings.Kubernetes.NodeLabels = b.Labels

	if b.KubeletConfig != nil && len(b.KubeletConfig.ClusterDNS) > 0 {
		s.Settings.Kubernetes.ClusterDNSIP = &b.KubeletConfig.ClusterDNS[0]
	}
	if !b.AWSENILimitedPodDensity {
		s.Settings.Kubernetes.MaxPods = aws.Int(110)
	}
	s.Settings.Kubernetes.NodeTaints = map[string][]string{}
	for _, taint := range b.Taints {
		s.Settings.Kubernetes.NodeTaints[taint.Key] = append(s.Settings.Kubernetes.NodeTaints[taint.Key], fmt.Sprintf("%s:%s", taint.Value, taint.Effect))
	}
	script := new(bytes.Buffer)
	tomlEncoder := toml.NewEncoder(script)
	err := tomlEncoder.Encode(&s)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(script.Bytes())
}

func (b Bottlerocket) unmarshalCustomUserData() config {
	var c config
	b64DecodedBytes, _ := base64.StdEncoding.DecodeString(*b.CustomUserData)
	err := toml.Unmarshal(b64DecodedBytes, &c)
	if err != nil {
		panic(err)
	}
	return c
}
