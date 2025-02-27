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

package health

import (
	"context"
	"fmt"
	"time"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/metrics"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	nodeutils "sigs.k8s.io/karpenter/pkg/utils/node"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"
)

var allowedUnhealthyPercent = intstr.FromString("20%")

// Controller for the resource
type Controller struct {
	clock         clock.Clock
	recorder      events.Recorder
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, cloudProvider cloudprovider.CloudProvider, clock clock.Clock, recorder events.Recorder) *Controller {
	return &Controller{
		clock:         clock,
		recorder:      recorder,
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
	}
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("node.health").
		For(&corev1.Node{}, builder.WithPredicates(nodeutils.IsManagedPredicateFuncs(c.cloudProvider))).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}

func (c *Controller) Reconcile(ctx context.Context, node *corev1.Node) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "node.health")
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("Node", klog.KRef(node.Namespace, node.Name)))

	// Validate that the node is owned by us
	nodeClaim, err := nodeutils.NodeClaimForNode(ctx, c.kubeClient, node)
	if err != nil {
		return reconcile.Result{}, nodeutils.IgnoreDuplicateNodeClaimError(nodeutils.IgnoreNodeClaimNotFoundError(err))
	}

	// If a nodeclaim does has a nodepool label, validate the nodeclaims inside the nodepool are healthy (i.e bellow the allowed threshold)
	// In the case of standalone nodeclaim, validate the nodes inside the cluster are healthy before proceeding
	// to repair the nodes
	nodePoolName, found := nodeClaim.Labels[v1.NodePoolLabelKey]
	if found {
		nodePoolHealthy, err := c.isNodePoolHealthy(ctx, nodePoolName)
		if err != nil {
			return reconcile.Result{}, client.IgnoreNotFound(err)
		}
		if !nodePoolHealthy {
			return reconcile.Result{}, c.publishNodePoolHealthEvent(ctx, node, nodeClaim, nodePoolName)
		}
	} else {
		clusterHealthy, err := c.isClusterHealthy(ctx)
		if err != nil {
			return reconcile.Result{}, err
		}
		if !clusterHealthy {
			c.recorder.Publish(NodeRepairBlockedUnmanagedNodeClaim(node, nodeClaim, fmt.Sprintf("more then %s nodes are unhealthy in the cluster", allowedUnhealthyPercent.String()))...)
			return reconcile.Result{}, nil
		}
	}

	unhealthyNodeCondition, policyTerminationDuration := c.findUnhealthyConditions(node)
	if unhealthyNodeCondition == nil {
		return reconcile.Result{}, nil
	}

	// If the Node is unhealthy, but has not reached it's full toleration disruption
	// requeue at the termination time of the unhealthy node
	terminationTime := unhealthyNodeCondition.LastTransitionTime.Add(policyTerminationDuration)
	if c.clock.Now().Before(terminationTime) {
		return reconcile.Result{RequeueAfter: terminationTime.Sub(c.clock.Now())}, nil
	}

	// For unhealthy past the tolerationDisruption window we can forcefully terminate the node
	if err := c.annotateTerminationGracePeriod(ctx, nodeClaim); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	return c.deleteNodeClaim(ctx, nodeClaim, node, unhealthyNodeCondition)
}

// deleteNodeClaim removes the NodeClaim from the api-server
func (c *Controller) deleteNodeClaim(ctx context.Context, nodeClaim *v1.NodeClaim, node *corev1.Node, unhealthyNodeCondition *corev1.NodeCondition) (reconcile.Result, error) {
	if !nodeClaim.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}
	if err := c.kubeClient.Delete(ctx, nodeClaim); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	// The deletion timestamp has successfully been set for the Node, update relevant metrics.
	log.FromContext(ctx).V(1).Info("deleting unhealthy node")
	metrics.NodeClaimsDisruptedTotal.Inc(map[string]string{
		metrics.ReasonLabel:       metrics.UnhealthyReason,
		metrics.NodePoolLabel:     node.Labels[v1.NodePoolLabelKey],
		metrics.CapacityTypeLabel: node.Labels[v1.CapacityTypeLabelKey],
	})
	NodeClaimsUnhealthyDisruptedTotal.Inc(map[string]string{
		Condition:                 pretty.ToSnakeCase(string(unhealthyNodeCondition.Type)),
		metrics.NodePoolLabel:     node.Labels[v1.NodePoolLabelKey],
		metrics.CapacityTypeLabel: node.Labels[v1.CapacityTypeLabelKey],
		ImageID:                   nodeClaim.Status.ImageID,
	})
	return reconcile.Result{}, nil
}

// Find a node with a condition that matches one of the unhealthy conditions defined by the cloud provider
// If there are multiple unhealthy status condition we will requeue based on the condition closest to its terminationDuration
func (c *Controller) findUnhealthyConditions(node *corev1.Node) (nc *corev1.NodeCondition, cpTerminationDuration time.Duration) {
	requeueTime := time.Time{}
	for _, policy := range c.cloudProvider.RepairPolicies() {
		// check the status and the type on the condition
		nodeCondition := nodeutils.GetCondition(node, policy.ConditionType)
		if nodeCondition.Status == policy.ConditionStatus {
			terminationTime := nodeCondition.LastTransitionTime.Add(policy.TolerationDuration)
			// Determine requeue time
			if requeueTime.IsZero() || requeueTime.After(terminationTime) {
				nc = lo.ToPtr(nodeCondition)
				cpTerminationDuration = policy.TolerationDuration
				requeueTime = terminationTime
			}
		}
	}
	return nc, cpTerminationDuration
}

func (c *Controller) annotateTerminationGracePeriod(ctx context.Context, nodeClaim *v1.NodeClaim) error {
	if expirationTimeString, exists := nodeClaim.ObjectMeta.Annotations[v1.NodeClaimTerminationTimestampAnnotationKey]; exists {
		expirationTime, err := time.Parse(time.RFC3339, expirationTimeString)
		if err == nil && expirationTime.Before(c.clock.Now()) {
			return nil
		}
	}

	stored := nodeClaim.DeepCopy()
	terminationTime := c.clock.Now().Format(time.RFC3339)
	nodeClaim.ObjectMeta.Annotations = lo.Assign(nodeClaim.ObjectMeta.Annotations, map[string]string{v1.NodeClaimTerminationTimestampAnnotationKey: terminationTime})

	if !equality.Semantic.DeepEqual(stored, nodeClaim) {
		if err := c.kubeClient.Patch(ctx, nodeClaim, client.MergeFrom(stored)); err != nil {
			return err
		}
		log.FromContext(ctx).WithValues(v1.NodeClaimTerminationTimestampAnnotationKey, terminationTime).Info("annotated nodeclaim")
	}

	return nil
}

// isNodePoolHealthy checks if the number of unhealthy nodes managed by the given NodePool exceeds the health threshold.
// defined by the cloud provider
// Up to 20% of Nodes may be unhealthy before the NodePool becomes unhealthy (or the nearest whole number, rounding up).
// For example, given a NodePool with three nodes, one may be unhealthy without rendering the NodePool unhealthy, even though that's 33% of the total nodes.
// This is analogous to how minAvailable and maxUnavailable work for PodDisruptionBudgets: https://kubernetes.io/docs/tasks/run-application/configure-pdb/#rounding-logic-when-specifying-percentages.
func (c *Controller) isNodePoolHealthy(ctx context.Context, nodePoolName string) (bool, error) {
	nodeList := &corev1.NodeList{}
	if err := c.kubeClient.List(ctx, nodeList, client.MatchingLabels(map[string]string{v1.NodePoolLabelKey: nodePoolName})); err != nil {
		return false, err
	}

	return c.isHealthyForNodes(nodeList.Items), nil
}

func (c *Controller) isClusterHealthy(ctx context.Context) (bool, error) {
	nodeList := &corev1.NodeList{}
	if err := c.kubeClient.List(ctx, nodeList); err != nil {
		return false, err
	}

	return c.isHealthyForNodes(nodeList.Items), nil
}

func (c *Controller) isHealthyForNodes(nodes []corev1.Node) bool {
	unhealthyNodeCount := lo.CountBy(nodes, func(node corev1.Node) bool {
		_, found := lo.Find(c.cloudProvider.RepairPolicies(), func(policy cloudprovider.RepairPolicy) bool {
			nodeCondition := nodeutils.GetCondition(lo.ToPtr(node), policy.ConditionType)
			return nodeCondition.Status == policy.ConditionStatus
		})
		return found
	})

	threshold := lo.Must(intstr.GetScaledValueFromIntOrPercent(lo.ToPtr(allowedUnhealthyPercent), len(nodes), true))
	return unhealthyNodeCount <= threshold
}

func (c *Controller) publishNodePoolHealthEvent(ctx context.Context, node *corev1.Node, nodeClaim *v1.NodeClaim, npName string) error {
	np := &v1.NodePool{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: npName}, np); err != nil {
		return client.IgnoreNotFound(err)
	}
	c.recorder.Publish(NodeRepairBlocked(node, nodeClaim, np, fmt.Sprintf("more than %s nodes are unhealthy in the nodepool", allowedUnhealthyPercent.String()))...)
	return nil
}
