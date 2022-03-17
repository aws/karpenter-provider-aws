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
	"fmt"
	"strings"
	"sync"

	core "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

// Options is the node bootstrapping parameters passed from Karpenter to the provisioning node
type Options struct {
	ClusterName             string
	ClusterEndpoint         string
	KubeletConfig           *v1alpha5.KubeletConfiguration
	Taints                  []core.Taint      `hash:"set"`
	Labels                  map[string]string `hash:"set"`
	CABundle                *string
	AWSENILimitedPodDensity bool
	ContainerRuntime        *string
}

// Bootstrapper can be implemented to generate a bootstrap script
// that uses the params from the Bootstrap type for a specific
// bootstrapping method.
// Examples are the Bottlerocket config and the eks-bootstrap script
type Bootstrapper interface {
	Script() string
}

func (o Options) nodeTaintArg() string {
	nodeTaintsArg := ""
	taintStrings := []string{}
	var once sync.Once
	for _, taint := range o.Taints {
		once.Do(func() { nodeTaintsArg = "--register-with-taints=" })
		taintStrings = append(taintStrings, fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect))
	}
	return fmt.Sprintf("%s%s", nodeTaintsArg, strings.Join(taintStrings, ","))
}

func (o Options) nodeLabelArg() string {
	nodeLabelArg := ""
	labelStrings := []string{}
	var once sync.Once
	for k, v := range o.Labels {
		if v1alpha5.LabelDomainExceptions.Has(k) {
			continue
		}
		once.Do(func() { nodeLabelArg = "--node-labels=" })
		labelStrings = append(labelStrings, fmt.Sprintf("%s=%v", k, v))
	}
	return fmt.Sprintf("%s%s", nodeLabelArg, strings.Join(labelStrings, ","))
}
