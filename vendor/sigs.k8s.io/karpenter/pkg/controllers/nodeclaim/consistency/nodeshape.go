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

package consistency

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

// NodeShape detects nodes that have launched with 10% or less of any resource than was expected.
type NodeShape struct{}

func NewNodeShape() Check {
	return &NodeShape{}
}

func (n *NodeShape) Check(_ context.Context, node *corev1.Node, nodeClaim *v1.NodeClaim) ([]Issue, error) {
	// ignore NodeClaims that are deleting
	if !nodeClaim.DeletionTimestamp.IsZero() {
		return nil, nil
	}
	// and NodeClaims that haven't initialized yet
	if !nodeClaim.StatusConditions().Get(v1.ConditionTypeInitialized).IsTrue() {
		return nil, nil
	}
	var issues []Issue
	for resourceName, requested := range nodeClaim.Spec.Resources.Requests {
		nodeQuantity := node.Status.Capacity[resourceName]
		expectedQuantity := nodeClaim.Status.Capacity[resourceName]
		if requested.IsZero() || expectedQuantity.IsZero() {
			continue
		}
		pct := nodeQuantity.AsApproximateFloat64() / expectedQuantity.AsApproximateFloat64()
		if pct < 0.90 {
			issues = append(issues, Issue(fmt.Sprintf("expected %s of resource %s, but found %s (%0.1f%% of expected)", expectedQuantity.String(),
				resourceName, nodeQuantity.String(), pct*100)))
		}
	}
	return issues, nil
}
