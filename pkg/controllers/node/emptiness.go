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

package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/clock"

	"github.com/aws/karpenter/pkg/controllers/state"

	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/pod"
)

// Emptiness is a subreconciler that deletes nodes that are empty after a ttl
type Emptiness struct {
	cluster        *state.Cluster
	kubeClient     client.Client
	firstSeenEmpty sync.Map
	clock          clock.Clock
}

var initialEmptyDebouncePeriod = 10 * time.Second

func NewEmptiness(clk clock.Clock, kubeClient client.Client, cluster *state.Cluster) *Emptiness {
	return &Emptiness{
		clock:      clk,
		cluster:    cluster,
		kubeClient: kubeClient,
	}
}

// Reconcile reconciles the node
func (r *Emptiness) Reconcile(ctx context.Context, provisioner *v1alpha5.Provisioner, n *v1.Node) (reconcile.Result, error) {
	// 1. Ignore node if not applicable
	if provisioner.Spec.TTLSecondsAfterEmpty == nil {
		return reconcile.Result{}, nil
	}

	if !r.cluster.IsNodeReady(n, provisioner) {
		return reconcile.Result{}, nil
	}
	// 2. Remove ttl if not empty
	empty, err := r.isEmpty(ctx, n)
	if err != nil {
		return reconcile.Result{}, err
	}

	if r.debounceEmptySignal(n.Name, empty) {
		return reconcile.Result{RequeueAfter: initialEmptyDebouncePeriod}, nil
	}

	emptinessTimestamp, hasEmptinessTimestamp := n.Annotations[v1alpha5.EmptinessTimestampAnnotationKey]
	if !empty {
		if hasEmptinessTimestamp {
			delete(n.Annotations, v1alpha5.EmptinessTimestampAnnotationKey)
			logging.FromContext(ctx).Infof("Removed emptiness TTL from node")
		}
		return reconcile.Result{}, nil
	}
	// 3. Set TTL if not set
	n.Annotations = functional.UnionStringMaps(n.Annotations)
	ttl := time.Duration(ptr.Int64Value(provisioner.Spec.TTLSecondsAfterEmpty)) * time.Second
	if !hasEmptinessTimestamp {
		n.Annotations[v1alpha5.EmptinessTimestampAnnotationKey] = r.clock.Now().Format(time.RFC3339)
		logging.FromContext(ctx).Infof("Added TTL to empty node")
		return reconcile.Result{RequeueAfter: ttl}, nil
	}
	// 4. Delete node if beyond TTL
	emptinessTime, err := time.Parse(time.RFC3339, emptinessTimestamp)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("parsing emptiness timestamp, %s", emptinessTimestamp)
	}
	if r.clock.Now().After(emptinessTime.Add(ttl)) {
		logging.FromContext(ctx).Infof("Triggering termination after %s for empty node", ttl)
		if err := r.kubeClient.Delete(ctx, n); err != nil {
			return reconcile.Result{}, fmt.Errorf("deleting node, %w", err)
		}
	}
	return reconcile.Result{RequeueAfter: emptinessTime.Add(ttl).Sub(r.clock.Now())}, nil
}

func (r *Emptiness) isEmpty(ctx context.Context, n *v1.Node) (bool, error) {
	pods := &v1.PodList{}
	if err := r.kubeClient.List(ctx, pods, client.MatchingFields{"spec.nodeName": n.Name}); err != nil {
		return false, fmt.Errorf("listing pods for node, %w", err)
	}
	for i := range pods.Items {
		p := pods.Items[i]
		if pod.IsTerminal(&p) {
			continue
		}
		if !pod.IsOwnedByDaemonSet(&p) && !pod.IsOwnedByNode(&p) {
			return false, nil
		}
	}
	return true, nil
}

// debounceEmptySignal returns true if the node hasn't been empty for at least initialEmptyDebouncePeriod.  Since we don't
// bind pods, once they are ready they are immediately empty.  This results in confusing log messages about emptiness TTL
// annotations being added and removed for every node as it comes up.  By waiting for the node to be empty for a short
// we allow kube-scheduler an opportunity to schedule pods against the node before we log that it is empty.
func (r *Emptiness) debounceEmptySignal(nodeName string, currentlyEmpty bool) bool {
	if !currentlyEmpty {
		r.firstSeenEmpty.Delete(nodeName)
		return false
	}
	firstSeenEmpty, _ := r.firstSeenEmpty.LoadOrStore(nodeName, r.clock.Now())
	firstSeenEmptyTime := firstSeenEmpty.(time.Time)

	return r.clock.Now().Sub(firstSeenEmptyTime) < initialEmptyDebouncePeriod
}
