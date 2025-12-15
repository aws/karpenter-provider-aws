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
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/samber/lo"
)

type Windows struct {
	Options
}

// nolint:gocyclo
func (w Windows) Script(_ context.Context) (string, error) {
	var userData bytes.Buffer
	userData.WriteString("<powershell>\n")

	customUserData := lo.FromPtr(w.CustomUserData)
	if customUserData != "" {
		userData.WriteString(customUserData + "\n")
	}

	userData.WriteString("[string]$EKSBootstrapScriptFile = \"$env:ProgramFiles\\Amazon\\EKS\\Start-EKSBootstrap.ps1\"\n")
	userData.WriteString(fmt.Sprintf(`& $EKSBootstrapScriptFile -EKSClusterName '%s' -APIServerEndpoint '%s'`, w.ClusterName, w.ClusterEndpoint))
	if w.CABundle != nil {
		userData.WriteString(fmt.Sprintf(` -Base64ClusterCA '%s'`, *w.CABundle))
	}
	if args := w.kubeletExtraArgs(); len(args) > 0 {
		userData.WriteString(fmt.Sprintf(` -KubeletExtraArgs '%s'`, strings.Join(args, " ")))
	}
	if w.KubeletConfig != nil && len(w.KubeletConfig.ClusterDNS) > 0 {
		userData.WriteString(fmt.Sprintf(` -DNSClusterIP '%s'`, w.KubeletConfig.ClusterDNS[0]))
	}
	userData.WriteString("\n</powershell>")
	return base64.StdEncoding.EncodeToString(userData.Bytes()), nil
}
