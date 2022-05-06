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
	"strings"
)

type Windows struct {
	Options
}

func (w Windows) Script() string {
	var caBundleArg string
	if w.CABundle != nil {
		caBundleArg = fmt.Sprintf(`-Base64ClusterCA "%s"`, *w.CABundle)
	}
	var userData bytes.Buffer
	userData.WriteString("<powershell>\n")
	userData.WriteString(`[string]$EKSBootstrapScriptFile = "$env:ProgramFiles\Amazon\EKS\Start-EKSBootstrap.ps1"`)
	userData.WriteString("\n")
	userData.WriteString(fmt.Sprintf(`& $EKSBootstrapScriptFile -EKSClusterName "%s" -APIServerEndpoint "%s" %s`, w.ClusterName, w.ClusterEndpoint, caBundleArg))

	kubeletExtraArgs := strings.Join([]string{w.nodeLabelArg(), w.nodeTaintArg()}, " ")

	if kubeletExtraArgs = strings.Trim(kubeletExtraArgs, " "); len(kubeletExtraArgs) > 0 {
		userData.WriteString(fmt.Sprintf(` -KubeletExtraArgs "%s"`, kubeletExtraArgs))
	}
	if w.KubeletConfig != nil && len(w.KubeletConfig.ClusterDNS) > 0 {
		userData.WriteString(fmt.Sprintf(` -DNSClusterIP "%s"`, w.KubeletConfig.ClusterDNS[0]))
	}
	userData.WriteString("\n</powershell>")
	return base64.StdEncoding.EncodeToString(userData.Bytes())
}
