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

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

// NodeClaim creates a test NodeClaim with defaults that can be overridden by overrides.
// Overrides are applied in order, with a last write wins semantic.
func NodeClaim(overrides ...v1.NodeClaim) *v1.NodeClaim {
	override := v1.NodeClaim{}
	for _, opts := range overrides {
		if err := mergo.Merge(&override, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("failed to merge: %v", err))
		}
	}
	if override.Name == "" {
		override.Name = RandomName()
	}
	if override.Status.ProviderID == "" {
		override.Status.ProviderID = RandomProviderID()
	}
	if override.Spec.NodeClassRef == nil {
		override.Spec.NodeClassRef = &v1.NodeClassReference{
			Group: object.GVK(defaultNodeClass).Group,
			Kind:  object.GVK(defaultNodeClass).Kind,
			Name:  "default",
		}
	}
	override.Labels = lo.Assign(map[string]string{
		v1.NodeClassLabelKey(override.Spec.NodeClassRef.GroupKind()): override.Spec.NodeClassRef.Name,
	}, override.Labels)
	if override.Spec.Requirements == nil {
		override.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{}
	}
	return &v1.NodeClaim{
		ObjectMeta: ObjectMeta(override.ObjectMeta),
		Spec:       override.Spec,
		Status:     override.Status,
	}
}

func NodeClaimAndNode(overrides ...v1.NodeClaim) (*v1.NodeClaim, *corev1.Node) {
	nc := NodeClaim(overrides...)
	return nc, NodeClaimLinkedNode(nc)
}

// NodeClaimsAndNodes creates homogeneous groups of NodeClaims and Nodes based on the passed in options, evenly divided by the total nodeclaims requested
func NodeClaimsAndNodes(total int, options ...v1.NodeClaim) ([]*v1.NodeClaim, []*corev1.Node) {
	nodeClaims := make([]*v1.NodeClaim, total)
	nodes := make([]*corev1.Node, total)
	for _, opts := range options {
		for i := 0; i < total/len(options); i++ {
			nodeClaim, node := NodeClaimAndNode(opts)
			nodeClaims[i] = nodeClaim
			nodes[i] = node
		}
	}
	return nodeClaims, nodes
}
