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
	"sort"
	"strings"
	"sync"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"

	"github.com/samber/lo"
)

type Windows struct {
	Options
}

// nolint:gocyclo
func (w Windows) Script() (string, error) {
	var userData bytes.Buffer
	userData.WriteString("<powershell>\n")
	userData.WriteString("[string]$EKSBootstrapScriptFile = \"$env:ProgramFiles\\Amazon\\EKS\\Start-EKSBootstrap.ps1\"\n")
	userData.WriteString(fmt.Sprintf(`& $EKSBootstrapScriptFile -EKSClusterName "%s" -APIServerEndpoint "%s"`, w.ClusterName, w.ClusterEndpoint))
	if w.CABundle != nil {
		userData.WriteString(fmt.Sprintf(" -Base64ClusterCA \"%s\"", *w.CABundle))
	}
	kubeletExtraArgs := strings.Join([]string{w.nodeLabelArg(), w.nodeTaintArg()}, " ")
	if kubeletExtraArgs = strings.Trim(kubeletExtraArgs, " "); len(kubeletExtraArgs) > 0 {
		userData.WriteString(fmt.Sprintf(` -KubeletExtraArgs "%s"`, kubeletExtraArgs))
	}
	if w.KubeletConfig != nil && len(w.KubeletConfig.ClusterDNS) > 0 {
		userData.WriteString(fmt.Sprintf(` -DNSClusterIP "%s"`, w.KubeletConfig.ClusterDNS[0]))
	}
	userData.WriteString("\n</powershell>")
	return base64.StdEncoding.EncodeToString(userData.Bytes()), nil
}

func (w Windows) nodeTaintArg() string {
	nodeTaintsArg := ""
	var taintStrings []string
	var once sync.Once
	for _, taint := range w.Taints {
		once.Do(func() { nodeTaintsArg = "--register-with-taints=" })
		taintStrings = append(taintStrings, fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect))
	}
	return fmt.Sprintf("%s%s", nodeTaintsArg, strings.Join(taintStrings, ","))
}

func (w Windows) nodeLabelArg() string {
	nodeLabelArg := ""
	var labelStrings []string
	var once sync.Once
	keys := lo.Keys(w.Labels)
	sort.Strings(keys) // ensures this list is deterministic, for easy testing.
	for _, key := range keys {
		if v1alpha5.LabelDomainExceptions.Has(key) {
			continue
		}
		once.Do(func() { nodeLabelArg = "--node-labels=" })
		labelStrings = append(labelStrings, fmt.Sprintf("%s=%v", key, w.Labels[key]))
	}
	return fmt.Sprintf("%s%s", nodeLabelArg, strings.Join(labelStrings, ","))
}
