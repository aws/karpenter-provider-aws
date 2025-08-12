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

package pdb

import (
	"context"

	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	podutil "sigs.k8s.io/karpenter/pkg/utils/pod"
)

type evictionBlocker int

const (
	zeroDisruptions evictionBlocker = iota
	fullyBlockingPDBs
)

// Limits is used to evaluate if evicting a list of pods is possible.
type Limits []*pdbItem

func NewLimits(ctx context.Context, kubeClient client.Client) (Limits, error) {
	pdbs := []*pdbItem{}

	var pdbList policyv1.PodDisruptionBudgetList
	if err := kubeClient.List(ctx, &pdbList); err != nil {
		return nil, err
	}
	for _, pdb := range pdbList.Items {
		pi, err := newPdb(pdb)
		if err != nil {
			return nil, err
		}
		pdbs = append(pdbs, pi)
	}

	return pdbs, nil
}

// CanEvictPods returns true if every pod in the list is evictable. They may not all be evictable simultaneously, but
// for every PDB that controls the pods at least one pod can be evicted.
// nolint:gocyclo
func (l Limits) CanEvictPods(pods []*v1.Pod) (client.ObjectKey, bool) {
	for _, pod := range pods {
		pdb, evictable := l.isEvictable(pod, zeroDisruptions)

		if !evictable {
			return pdb, false
		}
	}
	return client.ObjectKey{}, true
}

// isFullyBlocked returns true if the given pod is fully blocked by a PDB.
func (l Limits) isFullyBlocked(pod *v1.Pod) (client.ObjectKey, bool) {
	pdb, evictable := l.isEvictable(pod, fullyBlockingPDBs)

	if !evictable {
		return pdb, true
	}
	return client.ObjectKey{}, false
}

// nolint:gocyclo
func (l Limits) isEvictable(pod *v1.Pod, evictionBlocker evictionBlocker) (client.ObjectKey, bool) {
	// If the pod isn't eligible for being evicted, then the predicate doesn't matter
	// This is due to the fact that we won't call the eviction API on these pods when we are disrupting the node
	if !podutil.IsEvictable(pod) {
		return client.ObjectKey{}, true
	}
	for _, pdb := range l {
		if pdb.key.Namespace == pod.ObjectMeta.Namespace {
			if pdb.selector.Matches(labels.Set(pod.Labels)) {

				// if the PDB policy is set to allow evicting unhealthy pods, then it won't stop us from
				// evicting unhealthy pods
				if pdb.canAlwaysEvictUnhealthyPods {
					for _, c := range pod.Status.Conditions {
						if c.Type == v1.PodReady && c.Status == v1.ConditionFalse {
							return client.ObjectKey{}, true
						}
					}
				}

				switch evictionBlocker {
				case zeroDisruptions:
					if pdb.disruptionsAllowed == 0 {
						return pdb.key, false
					}
				case fullyBlockingPDBs:
					if pdb.isFullyBlocking {
						return pdb.key, false
					}
				}
			}
		}
	}
	return client.ObjectKey{}, true
}

// IsCurrentlyReschedulable checks if a Karpenter should consider this pod when re-scheduling to new capacity by ensuring that the pod:
// - Is reschedulable as per the checks in IsReschedulable(...)
// - Does not have the "karpenter.sh/do-not-disrupt=true" annotation (https://karpenter.sh/docs/concepts/disruption/#pod-level-controls)
// - Does not have fully blocking PDBs which would prevent the pod from being evicted
// The way this is different from IsReschedulable is that this also considers non-permanent conditions which prevent a pod from being rescheduled
// to a different node like the "do-not-disrupt" annotation or fully blocking PDBs.
func (l Limits) IsCurrentlyReschedulable(pod *v1.Pod) bool {
	// Don't provision capacity for pods which will not get evicted due to fully blocking PDBs.
	// Since Karpenter doesn't know when these pods will be successfully evicted, spinning up capacity until these pods are evicted is wasteful.
	_, isFullyBlocked := l.isFullyBlocked(pod)

	return podutil.IsReschedulable(pod) &&
		!podutil.HasDoNotDisrupt(pod) &&
		!isFullyBlocked
}

type pdbItem struct {
	key                         client.ObjectKey
	selector                    labels.Selector
	disruptionsAllowed          int32
	isFullyBlocking             bool
	canAlwaysEvictUnhealthyPods bool
}

// nolint:gocyclo
func newPdb(pdb policyv1.PodDisruptionBudget) (*pdbItem, error) {
	selector, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
	if err != nil {
		return nil, err
	}
	canAlwaysEvictUnhealthyPods := false

	if pdb.Spec.UnhealthyPodEvictionPolicy != nil && *pdb.Spec.UnhealthyPodEvictionPolicy == policyv1.AlwaysAllow {
		canAlwaysEvictUnhealthyPods = true
	}

	return &pdbItem{
		key:                client.ObjectKeyFromObject(&pdb),
		selector:           selector,
		disruptionsAllowed: pdb.Status.DisruptionsAllowed,
		isFullyBlocking: (pdb.Spec.MaxUnavailable != nil && pdb.Spec.MaxUnavailable.Type == intstr.Int && pdb.Spec.MaxUnavailable.IntVal == 0) ||
			(pdb.Spec.MaxUnavailable != nil && pdb.Spec.MaxUnavailable.Type == intstr.String && pdb.Spec.MaxUnavailable.StrVal == "0%") ||
			(pdb.Spec.MinAvailable != nil && pdb.Spec.MinAvailable.Type == intstr.String && pdb.Spec.MinAvailable.StrVal == "100%"),
		canAlwaysEvictUnhealthyPods: canAlwaysEvictUnhealthyPods,
	}, nil
}
