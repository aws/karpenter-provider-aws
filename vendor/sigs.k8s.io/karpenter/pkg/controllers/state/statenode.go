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

package state

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	nodeutils "sigs.k8s.io/karpenter/pkg/utils/node"
	"sigs.k8s.io/karpenter/pkg/utils/pdb"
	podutils "sigs.k8s.io/karpenter/pkg/utils/pod"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

type PodBlockEvictionError struct {
	error
}

func NewPodBlockEvictionError(err error) *PodBlockEvictionError {
	return &PodBlockEvictionError{error: err}
}

func IsPodBlockEvictionError(err error) bool {
	if err == nil {
		return false
	}
	var podBlockEvictionError *PodBlockEvictionError
	return stderrors.As(err, &podBlockEvictionError)
}

func IgnorePodBlockEvictionError(err error) error {
	if IsPodBlockEvictionError(err) {
		return nil
	}
	return err
}

//go:generate controller-gen object:headerFile="../../../hack/boilerplate.go.txt" paths="."

// StateNodes is a typed version of a list of *Node
// nolint: revive
type StateNodes []*StateNode

// Active filters StateNodes that are not in a MarkedForDeletion state
func (n StateNodes) Active() StateNodes {
	return lo.Filter(n, func(node *StateNode, _ int) bool {
		return !node.MarkedForDeletion()
	})
}

// Deleting filters StateNodes that are in a MarkedForDeletion state
func (n StateNodes) Deleting() StateNodes {
	return lo.Filter(n, func(node *StateNode, _ int) bool {
		return node.MarkedForDeletion()
	})
}

// Pods gets the pods assigned to all StateNodes based on the kubernetes api-server bindings
func (n StateNodes) Pods(ctx context.Context, kubeClient client.Client) ([]*corev1.Pod, error) {
	var pods []*corev1.Pod
	for _, node := range n {
		p, err := node.Pods(ctx, kubeClient)
		if err != nil {
			return nil, err
		}
		pods = append(pods, p...)
	}
	return pods, nil
}

func (n StateNodes) ReschedulablePods(ctx context.Context, kubeClient client.Client) ([]*corev1.Pod, error) {
	var pods []*corev1.Pod
	for _, node := range n {
		p, err := node.ReschedulablePods(ctx, kubeClient)
		if err != nil {
			return nil, err
		}
		pods = append(pods, p...)
	}
	return pods, nil
}

// StateNode is a cached version of a node in the cluster that maintains state which is expensive to compute every time it's
// needed.  This currently contains node utilization across all the allocatable resources, but will soon be used to
// compute topology information.
// +k8s:deepcopy-gen=true
// nolint: revive
type StateNode struct {
	Node      *corev1.Node
	NodeClaim *v1.NodeClaim

	// daemonSetRequests is the total amount of resources that have been requested by daemon sets. This allows users
	// of the Node to identify the remaining resources that we expect future daemonsets to consume.
	daemonSetRequests map[types.NamespacedName]corev1.ResourceList
	daemonSetLimits   map[types.NamespacedName]corev1.ResourceList

	podRequests map[types.NamespacedName]corev1.ResourceList
	podLimits   map[types.NamespacedName]corev1.ResourceList

	hostPortUsage *scheduling.HostPortUsage
	volumeUsage   *scheduling.VolumeUsage

	// TODO remove this when v1alpha5 APIs are deprecated. With v1 APIs Karpenter relies on the existence
	// of the karpenter.sh/disruption taint to know when a node is marked for deletion.
	markedForDeletion bool
	nominatedUntil    metav1.Time
}

func NewNode() *StateNode {
	return &StateNode{
		daemonSetRequests: map[types.NamespacedName]corev1.ResourceList{},
		daemonSetLimits:   map[types.NamespacedName]corev1.ResourceList{},
		podRequests:       map[types.NamespacedName]corev1.ResourceList{},
		podLimits:         map[types.NamespacedName]corev1.ResourceList{},
		hostPortUsage:     scheduling.NewHostPortUsage(),
		volumeUsage:       scheduling.NewVolumeUsage(),
	}
}

func (in *StateNode) Name() string {
	if in.Node == nil {
		return in.NodeClaim.Name
	}
	if in.NodeClaim == nil {
		return in.Node.Name
	}
	if !in.Registered() {
		return in.NodeClaim.Name
	}
	return in.Node.Name
}

// ProviderID is the key that is used to map this StateNode
// If the Node and NodeClaim have a providerID, this should map to a real providerID
// If the Node does not have a providerID, this will map to the node name
func (in *StateNode) ProviderID() string {
	if in.Node == nil {
		return in.NodeClaim.Status.ProviderID
	}
	return in.Node.Spec.ProviderID
}

// Pods gets the pods assigned to the Node based on the kubernetes api-server bindings
func (in *StateNode) Pods(ctx context.Context, kubeClient client.Client) ([]*corev1.Pod, error) {
	if in.Node == nil {
		return nil, nil
	}
	return nodeutils.GetPods(ctx, kubeClient, in.Node)
}

// ValidateNodeDisruptable returns an error if the StateNode cannot be disrupted
// This checks all associated StateNode internals, node labels, and do-not-disrupt annotations on the node.
// ValidateNodeDisruptable takes in a recorder to emit events on the nodeclaims when the state node is not a candidate
//
//nolint:gocyclo
func (in *StateNode) ValidateNodeDisruptable() error {
	if in.NodeClaim == nil {
		return fmt.Errorf("node isn't managed by karpenter")
	}
	if in.Node == nil {
		return fmt.Errorf("nodeclaim does not have an associated node")
	}
	if !in.Initialized() {
		return fmt.Errorf("node isn't initialized")
	}
	if in.MarkedForDeletion() {
		return fmt.Errorf("node is deleting or marked for deletion")
	}
	// skip the node if it is nominated by a recent provisioning pass to be the target of a pending pod.
	if in.Nominated() {
		return fmt.Errorf("node is nominated for a pending pod")
	}
	if in.Annotations()[v1.DoNotDisruptAnnotationKey] == "true" {
		return fmt.Errorf("disruption is blocked through the %q annotation", v1.DoNotDisruptAnnotationKey)
	}
	// check whether the node has the NodePool label
	if _, ok := in.Labels()[v1.NodePoolLabelKey]; !ok {
		return fmt.Errorf("node doesn't have required label %q", v1.NodePoolLabelKey)
	}
	return nil
}

// ValidatePodDisruptable returns an error if the StateNode contains a pod that cannot be disrupted
// This checks associated PDBs and do-not-disrupt annotations for each pod on the node.
// ValidatePodDisruptable takes in a recorder to emit events on the nodeclaims when the state node is not a candidate
//
//nolint:gocyclo
func (in *StateNode) ValidatePodsDisruptable(ctx context.Context, kubeClient client.Client, pdbs pdb.Limits) ([]*corev1.Pod, error) {
	pods, err := in.Pods(ctx, kubeClient)
	if err != nil {
		return nil, fmt.Errorf("getting pods from node, %w", err)
	}
	for _, po := range pods {
		// We only consider pods that are actively running for "karpenter.sh/do-not-disrupt"
		// This means that we will allow Mirror Pods and DaemonSets to block disruption using this annotation
		if !podutils.IsDisruptable(po) {
			return pods, NewPodBlockEvictionError(fmt.Errorf(`pod %q has "karpenter.sh/do-not-disrupt" annotation`, client.ObjectKeyFromObject(po)))
		}
	}
	if pdbKey, ok := pdbs.CanEvictPods(pods); !ok {
		return pods, NewPodBlockEvictionError(fmt.Errorf("pdb %q prevents pod evictions", pdbKey))
	}

	return pods, nil
}

// ReschedulablePods gets the pods assigned to the Node that are reschedulable based on the kubernetes api-server bindings
func (in *StateNode) ReschedulablePods(ctx context.Context, kubeClient client.Client) ([]*corev1.Pod, error) {
	if in.Node == nil {
		return nil, nil
	}
	return nodeutils.GetReschedulablePods(ctx, kubeClient, in.Node)
}

func (in *StateNode) HostName() string {
	if in.Labels()[corev1.LabelHostname] == "" {
		return in.Name()
	}
	return in.Labels()[corev1.LabelHostname]
}

func (in *StateNode) Annotations() map[string]string {
	// If the nodeclaim exists and the state node isn't initialized
	// use the nodeclaim representation of the annotations
	if in.Node == nil {
		return in.NodeClaim.Annotations
	}
	if in.NodeClaim == nil {
		return in.Node.Annotations
	}
	if !in.Registered() {
		return in.NodeClaim.Annotations
	}
	return in.Node.Annotations
}

func (in *StateNode) Labels() map[string]string {
	// If the nodeclaim exists and the state node isn't registered
	// use the nodeclaim representation of the labels
	if in.Node == nil {
		return in.NodeClaim.Labels
	}
	if in.NodeClaim == nil {
		return in.Node.Labels
	}
	if !in.Registered() {
		return in.NodeClaim.Labels
	}
	return in.Node.Labels
}

func (in *StateNode) Taints() []corev1.Taint {
	// If we have a managed node that isn't registered, we should use its NodeClaim
	// representation of taints. Likewise, if we don't have a Node representation for this
	// providerID in our state, we should also just use the NodeClaim since this is all that we have
	var taints []corev1.Taint
	if (!in.Registered() && in.Managed()) || in.Node == nil {
		taints = in.NodeClaim.Spec.Taints
	} else {
		taints = in.Node.Spec.Taints
	}
	if !in.Initialized() && in.Managed() {
		// We reject any well-known ephemeral taints and startup taints attached to this node until
		// the node is initialized. Without this, if the taint is generic and re-appears on the node for a
		// different reason (e.g. the node is cordoned) we will assume that pods can schedule against the
		// node in the future incorrectly.
		return lo.Reject(taints, func(taint corev1.Taint, _ int) bool {
			if _, found := lo.Find(scheduling.KnownEphemeralTaints, func(t corev1.Taint) bool {
				return t.MatchTaint(&taint)
			}); found {
				return true
			}
			if _, found := lo.Find(in.NodeClaim.Spec.StartupTaints, func(t corev1.Taint) bool {
				return t.MatchTaint(&taint)
			}); found {
				return true
			}
			return false
		})
	}
	return taints
}

func (in *StateNode) Registered() bool {
	// Node is managed by Karpenter, so we can check for the Registered label
	if in.Managed() {
		return in.Node != nil && in.Node.Labels[v1.NodeRegisteredLabelKey] == "true"
	}
	// Nodes not managed by Karpenter are always considered Registered
	return true
}

func (in *StateNode) Initialized() bool {
	// Node is managed by Karpenter, so we can check for the Initialized label
	if in.Managed() {
		return in.Node != nil && in.Node.Labels[v1.NodeInitializedLabelKey] == "true"
	}
	// Nodes not managed by Karpenter are always considered Initialized
	return true
}

func (in *StateNode) Capacity() corev1.ResourceList {
	if !in.Initialized() && in.NodeClaim != nil {
		// Override any zero quantity values in the node status
		if in.Node != nil {
			ret := lo.Assign(in.Node.Status.Capacity)
			for resourceName, quantity := range in.NodeClaim.Status.Capacity {
				if resources.IsZero(ret[resourceName]) {
					ret[resourceName] = quantity
				}
			}
			return ret
		}
		return in.NodeClaim.Status.Capacity
	}
	return in.Node.Status.Capacity
}

func (in *StateNode) Allocatable() corev1.ResourceList {
	if !in.Initialized() && in.NodeClaim != nil {
		// Override any zero quantity values in the node status
		if in.Node != nil {
			ret := lo.Assign(in.Node.Status.Allocatable)
			for resourceName, quantity := range in.NodeClaim.Status.Allocatable {
				if resources.IsZero(ret[resourceName]) {
					ret[resourceName] = quantity
				}
			}
			return ret
		}
		return in.NodeClaim.Status.Allocatable
	}
	return in.Node.Status.Allocatable
}

// Available is allocatable minus anything allocated to pods.
func (in *StateNode) Available() corev1.ResourceList {
	return resources.Subtract(in.Allocatable(), in.PodRequests())
}

func (in *StateNode) DaemonSetRequests() corev1.ResourceList {
	return resources.Merge(lo.Values(in.daemonSetRequests)...)
}

func (in *StateNode) DaemonSetLimits() corev1.ResourceList {
	return resources.Merge(lo.Values(in.daemonSetLimits)...)
}

func (in *StateNode) HostPortUsage() *scheduling.HostPortUsage {
	return in.hostPortUsage
}

func (in *StateNode) VolumeUsage() *scheduling.VolumeUsage {
	return in.volumeUsage
}

func (in *StateNode) PodRequests() corev1.ResourceList {
	var totalRequests corev1.ResourceList
	for _, requests := range in.podRequests {
		totalRequests = resources.MergeInto(totalRequests, requests)
	}
	return totalRequests
}

func (in *StateNode) PodLimits() corev1.ResourceList {
	return resources.Merge(lo.Values(in.podLimits)...)
}

func (in *StateNode) MarkedForDeletion() bool {
	// The Node is marked for deletion if:
	//  1. The Node has MarkedForDeletion set
	//  2. The Node has a NodeClaim counterpart and is actively deleting (or the nodeclaim is marked as terminating)
	//  3. The Node has no NodeClaim counterpart and is actively deleting
	return in.markedForDeletion || in.Deleted()
}

func (in *StateNode) Deleted() bool {
	return (in.NodeClaim != nil && (!in.NodeClaim.DeletionTimestamp.IsZero() || in.NodeClaim.StatusConditions().Get(v1.ConditionTypeInstanceTerminating).IsTrue())) ||
		(in.Node != nil && in.NodeClaim == nil && !in.Node.DeletionTimestamp.IsZero())
}

func (in *StateNode) Nominate(ctx context.Context) {
	in.nominatedUntil = metav1.Time{Time: time.Now().Add(nominationWindow(ctx))}
}

func (in *StateNode) Nominated() bool {
	return in.nominatedUntil.After(time.Now())
}

func (in *StateNode) Managed() bool {
	return in.NodeClaim != nil
}

func (in *StateNode) updateForPod(ctx context.Context, kubeClient client.Client, pod *corev1.Pod) error {
	podKey := client.ObjectKeyFromObject(pod)
	hostPorts := scheduling.GetHostPorts(pod)
	volumes, err := scheduling.GetVolumes(ctx, kubeClient, pod)
	if err != nil {
		return fmt.Errorf("tracking volume usage, %w", err)
	}
	in.podRequests[podKey] = resources.RequestsForPods(pod)
	in.podLimits[podKey] = resources.LimitsForPods(pod)
	// if it's a daemonset, we track what it has requested separately
	if podutils.IsOwnedByDaemonSet(pod) {
		in.daemonSetRequests[podKey] = resources.RequestsForPods(pod)
		in.daemonSetLimits[podKey] = resources.LimitsForPods(pod)
	}
	in.hostPortUsage.Add(pod, hostPorts)
	in.volumeUsage.Add(pod, volumes)
	return nil
}

func (in *StateNode) cleanupForPod(podKey types.NamespacedName) {
	in.hostPortUsage.DeletePod(podKey)
	in.volumeUsage.DeletePod(podKey)
	delete(in.podRequests, podKey)
	delete(in.podLimits, podKey)
	delete(in.daemonSetRequests, podKey)
	delete(in.daemonSetLimits, podKey)
}

func nominationWindow(ctx context.Context) time.Duration {
	nominationPeriod := 2 * options.FromContext(ctx).BatchMaxDuration
	if nominationPeriod < 10*time.Second {
		nominationPeriod = 10 * time.Second
	}
	return nominationPeriod
}

// RequireNoScheduleTaint will add/remove the karpenter.sh/disruption:NoSchedule taint from the candidates.
// This is used to enforce no taints at the beginning of disruption, and
// to add/remove taints while executing a disruption action.
// nolint:gocyclo
func RequireNoScheduleTaint(ctx context.Context, kubeClient client.Client, addTaint bool, nodes ...*StateNode) error {
	var multiErr error
	for _, n := range nodes {
		// If the StateNode is Karpenter owned and only has a nodeclaim, or is not owned by
		// Karpenter, thus having no nodeclaim, don't touch the node.
		if n.Node == nil || n.NodeClaim == nil {
			continue
		}
		node := &corev1.Node{}
		if err := kubeClient.Get(ctx, client.ObjectKey{Name: n.Node.Name}, node); client.IgnoreNotFound(err) != nil {
			multiErr = multierr.Append(multiErr, fmt.Errorf("getting node, %w", err))
		}
		// If the node already has the taint, continue to the next
		_, hasTaint := lo.Find(node.Spec.Taints, func(taint corev1.Taint) bool {
			return taint.MatchTaint(&v1.DisruptedNoScheduleTaint)
		})
		// Node is being deleted, so no need to remove taint as the node will be gone soon.
		// This ensures that the disruption controller doesn't modify taints that the Termination
		// controller is also modifying
		if hasTaint && !node.DeletionTimestamp.IsZero() {
			continue
		}
		stored := node.DeepCopy()
		// If the taint is present and we want to remove the taint, remove it.
		if !addTaint {
			node.Spec.Taints = lo.Reject(node.Spec.Taints, func(taint corev1.Taint, _ int) bool {
				return taint.MatchTaint(&v1.DisruptedNoScheduleTaint)
			})
			// otherwise, add it.
		} else if addTaint && !hasTaint {
			// If the taint key is present (but with a different value or effect), remove it.
			node.Spec.Taints = lo.Reject(node.Spec.Taints, func(taint corev1.Taint, _ int) bool {
				return taint.MatchTaint(&v1.DisruptedNoScheduleTaint)
			})
			node.Spec.Taints = append(node.Spec.Taints, v1.DisruptedNoScheduleTaint)
		}
		if !equality.Semantic.DeepEqual(stored, node) {
			// We use client.MergeFromWithOptimisticLock because patching a list with a JSON merge patch
			// can cause races due to the fact that it fully replaces the list on a change
			// Here, we are updating the taint list
			if err := kubeClient.Patch(ctx, node, client.MergeFromWithOptions(stored, client.MergeFromWithOptimisticLock{})); err != nil {
				multiErr = multierr.Append(multiErr, fmt.Errorf("patching node %s, %w", node.Name, err))
			}
		}
	}
	return multiErr
}

// ClearNodeClaimsCondition will remove the conditionType from the NodeClaim status of the provided statenodes
func ClearNodeClaimsCondition(ctx context.Context, kubeClient client.Client, conditionType string, nodes ...*StateNode) error {
	return multierr.Combine(lo.Map(nodes, func(s *StateNode, _ int) error {
		if !s.Initialized() || s.NodeClaim == nil {
			return nil
		}
		nodeClaim := &v1.NodeClaim{}
		if err := kubeClient.Get(ctx, client.ObjectKeyFromObject(s.NodeClaim), nodeClaim); err != nil {
			return client.IgnoreNotFound(err)
		}
		stored := nodeClaim.DeepCopy()
		_ = nodeClaim.StatusConditions().Clear(conditionType)

		if !equality.Semantic.DeepEqual(stored, nodeClaim) {
			if err := kubeClient.Status().Patch(ctx, nodeClaim, client.MergeFromWithOptions(stored, client.MergeFromWithOptimisticLock{})); err != nil {
				return client.IgnoreNotFound(err)
			}
		}
		return nil
	})...)
}
