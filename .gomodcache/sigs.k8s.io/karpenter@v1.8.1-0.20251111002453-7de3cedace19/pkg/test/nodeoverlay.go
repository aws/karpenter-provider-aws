/*
Copyright The Kubernetes Authors.

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
	"fmt"

	"github.com/imdario/mergo"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"sigs.k8s.io/karpenter/pkg/apis/v1alpha1"
)

// NodeOverlay creates a test NodeOverlay with defaults that can be overridden by overrides.
// Overrides are applied in order, with a last write wins semantic.
func NodeOverlay(overrides ...v1alpha1.NodeOverlay) *v1alpha1.NodeOverlay {
	override := v1alpha1.NodeOverlay{}
	for _, opts := range overrides {
		if err := mergo.Merge(&override, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("failed to merge: %v", err))
		}
	}
	if override.Name == "" {
		override.Name = RandomName()
	}
	if override.Spec.Requirements == nil {
		override.Spec.Requirements = []corev1.NodeSelectorRequirement{}
	}
	no := &v1alpha1.NodeOverlay{
		ObjectMeta: ObjectMeta(override.ObjectMeta),
		Spec:       override.Spec,
		Status:     override.Status,
	}
	return no
}

// ReplaceOverlayRequirements any current requirements on the passed through NodeOverlay with the passed in requirements
// If any of the keys match between the existing requirements and the new requirements, the new requirement with the same
// key will replace the old requirement with that key
func ReplaceOverlayRequirements(nodeOverlay *v1alpha1.NodeOverlay, reqs ...corev1.NodeSelectorRequirement) *v1alpha1.NodeOverlay {
	keys := sets.New[string](lo.Map(reqs, func(r corev1.NodeSelectorRequirement, _ int) string { return r.Key })...)
	nodeOverlay.Spec.Requirements = lo.Reject(nodeOverlay.Spec.Requirements, func(r corev1.NodeSelectorRequirement, _ int) bool {
		return keys.Has(r.Key)
	})
	nodeOverlay.Spec.Requirements = append(nodeOverlay.Spec.Requirements, reqs...)
	return nodeOverlay
}
