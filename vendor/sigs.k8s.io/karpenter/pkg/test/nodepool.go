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

	"github.com/awslabs/operatorpkg/object"
	"github.com/imdario/mergo"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

// NodePool creates a test NodePool with defaults that can be overridden by overrides.
// Overrides are applied in order, with a last write wins semantic.
func NodePool(overrides ...v1.NodePool) *v1.NodePool {
	override := v1.NodePool{}
	for _, opts := range overrides {
		if err := mergo.Merge(&override, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("failed to merge: %v", err))
		}
	}
	if override.Name == "" {
		override.Name = RandomName()
	}
	if override.Spec.Limits == nil {
		override.Spec.Limits = v1.Limits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2000")})
	}
	if override.Spec.Template.Spec.NodeClassRef == nil {
		override.Spec.Template.Spec.NodeClassRef = &v1.NodeClassReference{
			Group: object.GVK(defaultNodeClass).Group,
			Kind:  object.GVK(defaultNodeClass).Kind,
			Name:  "default",
		}
	}
	if override.Spec.Template.Spec.Requirements == nil {
		override.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{}
	}
	if override.Status.Conditions == nil {
		override.StatusConditions().SetTrue(v1.ConditionTypeValidationSucceeded)
		override.StatusConditions().SetTrue(v1.ConditionTypeNodeClassReady)
		override.StatusConditions().SetUnknown(v1.ConditionTypeNodeRegistrationHealthy)
	}
	np := &v1.NodePool{
		ObjectMeta: ObjectMeta(override.ObjectMeta),
		Spec:       override.Spec,
		Status:     override.Status,
	}
	np.Spec.Template.ObjectMeta = TemplateObjectMeta(np.Spec.Template.ObjectMeta)
	return np
}

// NodePools creates homogeneous groups of NodePools
// based on the passed in options, evenly divided by the total NodePools requested
func NodePools(total int, options ...v1.NodePool) []*v1.NodePool {
	nodePools := make([]*v1.NodePool, total)
	for _, opts := range options {
		for i := 0; i < total/len(options); i++ {
			nodePool := NodePool(opts)
			nodePools[i] = nodePool
		}
	}
	return nodePools
}

// ReplaceRequirements any current requirements on the passed through NodePool with the passed in requirements
// If any of the keys match between the existing requirements and the new requirements, the new requirement with the same
// key will replace the old requirement with that key
func ReplaceRequirements(nodePool *v1.NodePool, reqs ...v1.NodeSelectorRequirementWithMinValues) *v1.NodePool {
	keys := sets.New[string](lo.Map(reqs, func(r v1.NodeSelectorRequirementWithMinValues, _ int) string { return r.Key })...)
	nodePool.Spec.Template.Spec.Requirements = lo.Reject(nodePool.Spec.Template.Spec.Requirements, func(r v1.NodeSelectorRequirementWithMinValues, _ int) bool {
		return keys.Has(r.Key)
	})
	nodePool.Spec.Template.Spec.Requirements = append(nodePool.Spec.Template.Spec.Requirements, reqs...)
	return nodePool
}

// StaticNodePool creates a test NodePool suitable for static provisioning
// It will keep limits.nodes if provided in overrides, otherwise limits will be nil
func StaticNodePool(overrides ...v1.NodePool) *v1.NodePool {
	// First create the NodePool with all overrides
	nodePool := NodePool(overrides...)

	var hasNodesLimit bool
	var nodesLimit resource.Quantity
	for _, override := range overrides {
		if override.Spec.Limits != nil {
			if limit, ok := override.Spec.Limits[resources.Node]; ok {
				hasNodesLimit = true
				nodesLimit = limit
				break
			}
		}
	}

	// Set limits based on whether nodes limit was provided
	if !hasNodesLimit {
		nodePool.Spec.Limits = nil
	} else {
		nodePool.Spec.Limits = v1.Limits(corev1.ResourceList{
			resources.Node: nodesLimit,
		})
	}

	return nodePool
}
