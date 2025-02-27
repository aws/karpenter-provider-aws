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

package lifecycle

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	nodeutils "sigs.k8s.io/karpenter/pkg/utils/node"
	nodeclaimutils "sigs.k8s.io/karpenter/pkg/utils/nodeclaim"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

type Initialization struct {
	kubeClient client.Client
}

// Reconcile checks for initialization based on if:
// a) its current status is set to Ready
// b) all the startup taints have been removed from the node
// c) all extended resources have been registered
// This method handles both nil nodepools and nodes without extended resources gracefully.
func (i *Initialization) Reconcile(ctx context.Context, nodeClaim *v1.NodeClaim) (reconcile.Result, error) {
	if cond := nodeClaim.StatusConditions().Get(v1.ConditionTypeInitialized); !cond.IsUnknown() {
		// Ensure that we always set the status condition to the latest generation
		nodeClaim.StatusConditions().Set(*cond)
		return reconcile.Result{}, nil
	}
	if !nodeClaim.StatusConditions().Get(v1.ConditionTypeRegistered).IsTrue() {
		return reconcile.Result{}, nil
	}
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("provider-id", nodeClaim.Status.ProviderID))
	node, err := nodeclaimutils.NodeForNodeClaim(ctx, i.kubeClient, nodeClaim)
	if err != nil {
		nodeClaim.StatusConditions().SetUnknownWithReason(v1.ConditionTypeInitialized, "NodeNotFound", "Node not registered with cluster")
		return reconcile.Result{}, nil //nolint:nilerr
	}
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("Node", klog.KRef("", node.Name)))
	if nodeutils.GetCondition(node, corev1.NodeReady).Status != corev1.ConditionTrue {
		nodeClaim.StatusConditions().SetUnknownWithReason(v1.ConditionTypeInitialized, "NodeNotReady", "Node status is NotReady")
		return reconcile.Result{}, nil
	}
	if taint, ok := StartupTaintsRemoved(node, nodeClaim); !ok {
		nodeClaim.StatusConditions().SetUnknownWithReason(v1.ConditionTypeInitialized, "StartupTaintsExist", fmt.Sprintf("StartupTaint %q still exists", formatTaint(taint)))
		return reconcile.Result{}, nil
	}
	if taint, ok := KnownEphemeralTaintsRemoved(node); !ok {
		nodeClaim.StatusConditions().SetUnknownWithReason(v1.ConditionTypeInitialized, "KnownEphemeralTaintsExist", fmt.Sprintf("KnownEphemeralTaint %q still exists", formatTaint(taint)))
		return reconcile.Result{}, nil
	}
	if name, ok := RequestedResourcesRegistered(node, nodeClaim); !ok {
		nodeClaim.StatusConditions().SetUnknownWithReason(v1.ConditionTypeInitialized, "ResourceNotRegistered", fmt.Sprintf("Resource %q was requested but not registered", name))
		return reconcile.Result{}, nil
	}
	stored := node.DeepCopy()
	node.Labels = lo.Assign(node.Labels, map[string]string{v1.NodeInitializedLabelKey: "true"})
	if !equality.Semantic.DeepEqual(stored, node) {
		if err = i.kubeClient.Patch(ctx, node, client.MergeFrom(stored)); err != nil {
			return reconcile.Result{}, err
		}
	}
	log.FromContext(ctx).WithValues("allocatable", node.Status.Allocatable).Info("initialized nodeclaim")
	nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeInitialized)
	return reconcile.Result{}, nil
}

// KnownEphemeralTaintsRemoved validates whether all the ephemeral taints are removed
func KnownEphemeralTaintsRemoved(node *corev1.Node) (*corev1.Taint, bool) {
	for _, knownTaint := range scheduling.KnownEphemeralTaints {
		// if the node still has a known ephemeral taint applied, it's not ready
		for i := range node.Spec.Taints {
			if knownTaint.MatchTaint(&node.Spec.Taints[i]) {
				return &node.Spec.Taints[i], false
			}
		}
	}
	return nil, true
}

// StartupTaintsRemoved returns true if there are no startup taints registered for the nodepool, or if all startup
// taints have been removed from the node
func StartupTaintsRemoved(node *corev1.Node, nodeClaim *v1.NodeClaim) (*corev1.Taint, bool) {
	if nodeClaim != nil {
		for _, startupTaint := range nodeClaim.Spec.StartupTaints {
			for i := range node.Spec.Taints {
				// if the node still has a startup taint applied, it's not ready
				if startupTaint.MatchTaint(&node.Spec.Taints[i]) {
					return &node.Spec.Taints[i], false
				}
			}
		}
	}
	return nil, true
}

// RequestedResourcesRegistered returns true if there are no extended resources on the node, or they have all been
// registered by device plugins
func RequestedResourcesRegistered(node *corev1.Node, nodeClaim *v1.NodeClaim) (corev1.ResourceName, bool) {
	for resourceName, quantity := range nodeClaim.Spec.Resources.Requests {
		if quantity.IsZero() {
			continue
		}
		// kubelet will zero out both the capacity and allocatable for an extended resource on startup, so if our
		// annotation says the resource should be there, but it's zero'd in both then the device plugin hasn't
		// registered it yet.
		// We wait on allocatable since this is the value that is used in scheduling
		if resources.IsZero(node.Status.Allocatable[resourceName]) {
			return resourceName, false
		}
	}
	return "", true
}

func formatTaint(taint *corev1.Taint) string {
	if taint == nil {
		return "<nil>"
	}
	if taint.Value == "" {
		return fmt.Sprintf("%s:%s", taint.Key, taint.Effect)
	}
	return fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect)
}
