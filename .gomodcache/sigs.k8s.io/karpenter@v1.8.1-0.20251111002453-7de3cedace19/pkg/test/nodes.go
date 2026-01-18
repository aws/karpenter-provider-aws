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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

type NodeOptions struct {
	metav1.ObjectMeta
	ReadyStatus   corev1.ConditionStatus
	ReadyReason   string
	Conditions    []corev1.NodeCondition
	Unschedulable bool
	ProviderID    string
	Taints        []corev1.Taint
	Allocatable   corev1.ResourceList
	Capacity      corev1.ResourceList
}

func Node(overrides ...NodeOptions) *corev1.Node {
	options := NodeOptions{}
	for _, opts := range overrides {
		if err := mergo.Merge(&options, opts, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge node options: %s", err))
		}
	}
	if options.ReadyStatus == "" {
		options.ReadyStatus = corev1.ConditionTrue
	}
	if options.Capacity == nil {
		options.Capacity = options.Allocatable
	}

	return &corev1.Node{
		ObjectMeta: NamespacedObjectMeta(options.ObjectMeta),
		Spec: corev1.NodeSpec{
			Unschedulable: options.Unschedulable,
			Taints:        options.Taints,
			ProviderID:    options.ProviderID,
		},
		Status: corev1.NodeStatus{
			Allocatable: options.Allocatable,
			Capacity:    options.Capacity,
			Conditions:  []corev1.NodeCondition{{Type: corev1.NodeReady, Status: options.ReadyStatus, Reason: options.ReadyReason}},
		},
	}
}

func NodeClaimLinkedNode(nodeClaim *v1.NodeClaim) *corev1.Node {
	n := Node(
		NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: lo.Assign(map[string]string{
					v1.NodeClassLabelKey(nodeClaim.Spec.NodeClassRef.GroupKind()): nodeClaim.Spec.NodeClassRef.Name,
				}, nodeClaim.Labels),
				Annotations: nodeClaim.Annotations,
				Finalizers:  nodeClaim.Finalizers,
			},
			Taints:      append(nodeClaim.Spec.Taints, nodeClaim.Spec.StartupTaints...),
			Capacity:    nodeClaim.Status.Capacity,
			Allocatable: nodeClaim.Status.Allocatable,
			ProviderID:  nodeClaim.Status.ProviderID,
		},
	)
	n.Spec.Taints = append(n.Spec.Taints, v1.UnregisteredNoExecuteTaint)
	return n
}
