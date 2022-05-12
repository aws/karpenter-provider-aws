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

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Pallinder/go-randomdata"
	"github.com/imdario/mergo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

// ProvisionerOptions customizes a Provisioner.
type ProvisionerOptions struct {
	metav1.ObjectMeta
	Limits        v1.ResourceList
	Provider      interface{}
	Kubelet       *v1alpha5.KubeletConfiguration
	Labels        map[string]string
	Taints        []v1.Taint
	StartupTaints []v1.Taint
	Requirements  []v1.NodeSelectorRequirement
	Status        v1alpha5.ProvisionerStatus
}

// Provisioner creates a test pod with defaults that can be overridden by ProvisionerOptions.
// Overrides are applied in order, with a last write wins semantic.
func Provisioner(overrides ...ProvisionerOptions) *v1alpha5.Provisioner {
	options := ProvisionerOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge pod options: %s", err))
		}
	}
	if options.Name == "" {
		options.Name = strings.ToLower(randomdata.SillyName())
	}
	if options.Limits == nil {
		options.Limits = v1.ResourceList{v1.ResourceCPU: resource.MustParse("100")}
	}
	if options.Provider == nil {
		options.Provider = struct{}{}
	}
	provider, _ := json.Marshal(options.Provider)
	provisioner := &v1alpha5.Provisioner{
		ObjectMeta: ObjectMeta(options.ObjectMeta),
		Spec: v1alpha5.ProvisionerSpec{
			Constraints: v1alpha5.Constraints{
				Requirements:         v1alpha5.NewRequirements(options.Requirements...),
				KubeletConfiguration: options.Kubelet,
				Provider:             &runtime.RawExtension{Raw: provider},
				Taints:               options.Taints,
				StartupTaints:        options.StartupTaints,
				Labels:               options.Labels,
			},
			Limits: &v1alpha5.Limits{Resources: options.Limits},
		},
		Status: options.Status,
	}
	provisioner.SetDefaults(context.TODO())
	_ = provisioner.Validate(context.TODO())
	return provisioner
}
