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

package consolidation

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/clock"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/utils/pod"
)

type StabilizationWindow struct {
	clock clock.Clock

	lastConsolidationTime time.Time
	cluster               *state.Cluster
	kubeClient            client.Client
}

// fastWindowTime is the  window used if all the cluster workloads are reporting ready
const fastWindowTime = 15 * time.Second

// standardWindowTime is the window used if workloads aren't ready, to allow us to make progress in the event of
// persistently unready workloads (e.g. a bad deployment)
const standardWindowTime = 3 * time.Minute

// maxWindowTime is the maximum window time to ensure that even if we are in constant flux with pods being added &
// removed, which either cause deployments to go unready or are in the presence of a persistently unready deployment,
// we will still make some progress
const maxWindowTime = 10 * time.Minute

// NewStabilizationWindow maintains a dynamic stabilization window.
// Node/Pod additions/deletions extend the window by standardWindowTime, up to the maxWindowTime
// If there are no pending pods, deployments/replicasets are ready, etc. fastWindowTime is used instead of standardWindowTime
//
// t = 0, we consolidated, fast window point is at t=15s, standard is t=3min, max is=10min
// t = 10s, a pod is deleted, fast window point is t=25s, standard is t=3m10s, max is t=10m
// t = 25s, if the conditions for fast window are true, we consolidate.  Assume not, we won't consolidate until 3m25s.
// t = 30s, a pod is added, fast window is t=45s, standard is t=3m30s, max is t=10m
// assume a pod is added and deleted every few seconds which constantly resets the window
// t = 10m, a pod is added, fast window is t=10m15s, standard is t=13m, max is still t=10m since we haven't
// consolidated, so we consolidate now even though the cluster is in flux since we haven't done so in 10minutes.
func NewStabilizationWindow(clk clock.Clock, cluster *state.Cluster, kubeClient client.Client) *StabilizationWindow {
	return &StabilizationWindow{
		clock:      clk,
		cluster:    cluster,
		kubeClient: kubeClient,
	}
}

// IsStabilized returns true if we are out of the controller's stabilization window
func (w *StabilizationWindow) IsStabilized(ctx context.Context) bool {
	stabilizationTime := w.clock.Now().Add(-w.stabilizationWindow(ctx))

	// has the cluster been unchanged for the desired amount of time?
	clusterStable := w.cluster.LastNodeDeletionTime().Before(stabilizationTime) &&
		w.cluster.LastNodeCreationTime().Before(stabilizationTime) &&
		w.cluster.LastPodDeletionTime().Before(stabilizationTime) &&
		w.cluster.LastPodCreationTime().Before(stabilizationTime)

	// or has our stabilization window exceeded its max length?
	exceededMaxStabWindow := w.lastConsolidationTime.Add(maxWindowTime).Before(w.clock.Now())

	return clusterStable || exceededMaxStabWindow
}

// Reset resets the controller's stabilization window, we must get out of this window again before
// IsStabilized will return true
func (w *StabilizationWindow) Reset() {
	w.lastConsolidationTime = w.clock.Now()
}

func (w *StabilizationWindow) stabilizationWindow(ctx context.Context) time.Duration {
	// no pending pods, and all replica sets/replication controllers are reporting ready so quickly consider another consolidation
	if !w.hasPendingPods(ctx) && w.deploymentsReady(ctx) && w.replicaSetsReady(ctx) &&
		w.replicationControllersReady(ctx) && w.statefulSetsReady(ctx) {
		return fastWindowTime
	}
	return standardWindowTime
}

func (w *StabilizationWindow) hasPendingPods(ctx context.Context) bool {
	var podList v1.PodList
	if err := w.kubeClient.List(ctx, &podList, client.MatchingFields{"spec.nodeName": ""}); err != nil {
		// failed to list pods, assume there must be pending as it's harmless and just ensures we wait longer
		return true
	}
	for i := range podList.Items {
		if pod.IsProvisionable(&podList.Items[i]) {
			return true
		}
	}
	return false
}

func (w *StabilizationWindow) deploymentsReady(ctx context.Context) bool {
	var depList appsv1.DeploymentList
	if err := w.kubeClient.List(ctx, &depList); err != nil {
		// failed to list, assume there must be one non-ready as it's harmless and just ensures we wait longer
		return false
	}
	for _, ds := range depList.Items {
		if !ds.DeletionTimestamp.IsZero() {
			continue
		}
		desired := ptr.Int32Value(ds.Spec.Replicas)
		if ds.Spec.Replicas == nil {
			// unspecified defaults to 1
			desired = 1
		}
		if ds.Status.ReadyReplicas != desired || ds.Status.UpdatedReplicas != desired {
			return false
		}
	}
	return true
}

func (w *StabilizationWindow) replicaSetsReady(ctx context.Context) bool {
	var rsList appsv1.ReplicaSetList
	if err := w.kubeClient.List(ctx, &rsList); err != nil {
		// failed to list, assume there must be one non-ready as it's harmless and just ensures we wait longer
		return false
	}
	for _, rs := range rsList.Items {
		if !rs.DeletionTimestamp.IsZero() {
			continue
		}
		desired := ptr.Int32Value(rs.Spec.Replicas)
		if rs.Spec.Replicas == nil {
			// unspecified defaults to 1
			desired = 1
		}
		if rs.Status.ReadyReplicas != desired {
			return false
		}
	}
	return true
}

func (w *StabilizationWindow) replicationControllersReady(ctx context.Context) bool {
	var rsList v1.ReplicationControllerList
	if err := w.kubeClient.List(ctx, &rsList); err != nil {
		// failed to list, assume there must be one non-ready as it's harmless and just ensures we wait longer
		return false
	}
	for _, rs := range rsList.Items {
		if !rs.DeletionTimestamp.IsZero() {
			continue
		}
		desired := ptr.Int32Value(rs.Spec.Replicas)
		if rs.Spec.Replicas == nil {
			// unspecified defaults to 1
			desired = 1
		}
		if rs.Status.ReadyReplicas != desired {
			return false
		}
	}
	return true
}

func (w *StabilizationWindow) statefulSetsReady(ctx context.Context) bool {
	var sslist appsv1.StatefulSetList
	if err := w.kubeClient.List(ctx, &sslist); err != nil {
		// failed to list, assume there must be one non-ready as it's harmless and just ensures we wait longer
		return false
	}
	for _, ss := range sslist.Items {
		if !ss.DeletionTimestamp.IsZero() {
			continue
		}
		desired := ptr.Int32Value(ss.Spec.Replicas)
		if ss.Spec.Replicas == nil {
			// unspecified defaults to 1
			desired = 1
		}
		if ss.Status.ReadyReplicas != desired || ss.Status.UpdatedReplicas != desired {
			return false
		}
	}
	return true
}
