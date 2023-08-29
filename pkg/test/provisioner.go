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

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"

	corev1alpha5 "github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/v1alpha5"
)

func ProvisionerE2ETests(options ...test.ProvisionerOptions) *corev1alpha5.Provisioner {
	provisioner := v1alpha5.Provisioner(lo.FromPtr(test.Provisioner(options...)))
	provisioner.SetDefaults(context.Background())
	provisioner.Spec.Requirements = append(provisioner.Spec.Requirements, v1.NodeSelectorRequirement{
		Key:      v1alpha1.LabelInstanceFamily,
		Operator: v1.NodeSelectorOpNotIn,
		Values:   []string{"m7a"},
	})
	return lo.ToPtr(corev1alpha5.Provisioner(provisioner))
}

// Only use this for unit tests. Provisioner() will add in provisioner requirements for the e2etests.
func Provisioner(options ...test.ProvisionerOptions) *corev1alpha5.Provisioner {
	provisioner := v1alpha5.Provisioner(lo.FromPtr(test.Provisioner(options...)))
	provisioner.SetDefaults(context.Background())
	return lo.ToPtr(corev1alpha5.Provisioner(provisioner))
}
