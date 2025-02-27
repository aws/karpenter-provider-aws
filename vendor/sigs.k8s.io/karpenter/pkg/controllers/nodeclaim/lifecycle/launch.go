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
	"errors"
	"fmt"

	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/metrics"
	"sigs.k8s.io/karpenter/pkg/scheduling"
)

type Launch struct {
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
	cache         *cache.Cache // exists due to eventual consistency on the cache
	recorder      events.Recorder
}

func (l *Launch) Reconcile(ctx context.Context, nodeClaim *v1.NodeClaim) (reconcile.Result, error) {
	if cond := nodeClaim.StatusConditions().Get(v1.ConditionTypeLaunched); !cond.IsUnknown() {
		// Ensure that we always set the status condition to the latest generation
		nodeClaim.StatusConditions().Set(*cond)
		return reconcile.Result{}, nil
	}

	var err error
	var created *v1.NodeClaim

	// One of the following scenarios can happen with a NodeClaim that isn't marked as launched:
	//  1. It was already launched by the CloudProvider but the client-go cache wasn't updated quickly enough or
	//     patching failed on the status. In this case, we use the in-memory cached value for the created NodeClaim.
	//  2. It is a standard NodeClaim launch where we should call CloudProvider Create() and fill in details of the launched
	//     NodeClaim into the NodeClaim CR.
	if ret, ok := l.cache.Get(string(nodeClaim.UID)); ok {
		created = ret.(*v1.NodeClaim)
	} else {
		created, err = l.launchNodeClaim(ctx, nodeClaim)
	}
	// Either the Node launch failed or the Node was deleted due to InsufficientCapacity/NodeClassNotReady/NotFound
	if err != nil || created == nil {
		return reconcile.Result{}, err
	}
	l.cache.SetDefault(string(nodeClaim.UID), created)
	nodeClaim = PopulateNodeClaimDetails(nodeClaim, created)
	nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeLaunched)
	return reconcile.Result{}, nil
}

func (l *Launch) launchNodeClaim(ctx context.Context, nodeClaim *v1.NodeClaim) (*v1.NodeClaim, error) {
	created, err := l.cloudProvider.Create(ctx, nodeClaim)
	if err != nil {
		switch {
		case cloudprovider.IsInsufficientCapacityError(err):
			l.recorder.Publish(InsufficientCapacityErrorEvent(nodeClaim, err))
			log.FromContext(ctx).Error(err, "failed launching nodeclaim")

			if err = l.kubeClient.Delete(ctx, nodeClaim); err != nil {
				return nil, client.IgnoreNotFound(err)
			}
			metrics.NodeClaimsDisruptedTotal.Inc(map[string]string{
				metrics.ReasonLabel:       "insufficient_capacity",
				metrics.NodePoolLabel:     nodeClaim.Labels[v1.NodePoolLabelKey],
				metrics.CapacityTypeLabel: nodeClaim.Labels[v1.CapacityTypeLabelKey],
			})
			return nil, nil
		case cloudprovider.IsNodeClassNotReadyError(err):
			log.FromContext(ctx).Error(err, "failed launching nodeclaim")
			if err = l.kubeClient.Delete(ctx, nodeClaim); err != nil {
				return nil, client.IgnoreNotFound(err)
			}
			metrics.NodeClaimsDisruptedTotal.Inc(map[string]string{
				metrics.ReasonLabel:       "nodeclass_not_ready",
				metrics.NodePoolLabel:     nodeClaim.Labels[v1.NodePoolLabelKey],
				metrics.CapacityTypeLabel: nodeClaim.Labels[v1.CapacityTypeLabelKey],
			})
			return nil, nil
		default:
			var createError *cloudprovider.CreateError
			if errors.As(err, &createError) {
				nodeClaim.StatusConditions().SetUnknownWithReason(v1.ConditionTypeLaunched, createError.ConditionReason, createError.ConditionMessage)
			} else {
				nodeClaim.StatusConditions().SetUnknownWithReason(v1.ConditionTypeLaunched, "LaunchFailed", truncateMessage(err.Error()))
			}
			return nil, fmt.Errorf("launching nodeclaim, %w", err)
		}
	}
	log.FromContext(ctx).WithValues(
		"provider-id", created.Status.ProviderID,
		"instance-type", created.Labels[corev1.LabelInstanceTypeStable],
		"zone", created.Labels[corev1.LabelTopologyZone],
		"capacity-type", created.Labels[v1.CapacityTypeLabelKey],
		"allocatable", created.Status.Allocatable).Info("launched nodeclaim")
	return created, nil
}

func PopulateNodeClaimDetails(nodeClaim, retrieved *v1.NodeClaim) *v1.NodeClaim {
	// These are ordered in priority order so that user-defined nodeClaim labels and requirements trump retrieved labels
	// or the static nodeClaim labels
	nodeClaim.Labels = lo.Assign(
		retrieved.Labels, // CloudProvider-resolved labels
		scheduling.NewNodeSelectorRequirementsWithMinValues(nodeClaim.Spec.Requirements...).Labels(), // Single-value requirement resolved labels
		nodeClaim.Labels, // User-defined labels
	)
	nodeClaim.Annotations = lo.Assign(nodeClaim.Annotations, retrieved.Annotations)
	nodeClaim.Status.ProviderID = retrieved.Status.ProviderID
	nodeClaim.Status.ImageID = retrieved.Status.ImageID
	nodeClaim.Status.Allocatable = retrieved.Status.Allocatable
	nodeClaim.Status.Capacity = retrieved.Status.Capacity
	return nodeClaim
}

func truncateMessage(msg string) string {
	if len(msg) < 300 {
		return msg
	}
	return msg[:300] + "..."
}
