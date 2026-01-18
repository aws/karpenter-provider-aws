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

package static

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/samber/lo"
	"k8s.io/utils/clock"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/karpenter/pkg/controllers/provisioning"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning/scheduling"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/metrics"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	nodepoolutils "sigs.k8s.io/karpenter/pkg/utils/nodepool"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

type Controller struct {
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
	provisioner   *provisioning.Provisioner
	cluster       *state.Cluster
}

func NewController(kubeClient client.Client, cluster *state.Cluster, recorder events.Recorder, cloudProvider cloudprovider.CloudProvider, provisioner *provisioning.Provisioner, clock clock.Clock) *Controller {
	return &Controller{
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
		cluster:       cluster,
		provisioner:   provisioning.NewProvisioner(kubeClient, recorder, cloudProvider, cluster, clock),
	}
}

// Reconcile the resource
// Requeue after computing Static NodePool to ensure we don't miss any events
func (c *Controller) Reconcile(ctx context.Context, np *v1.NodePool) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "static.provisioning")

	if !nodepoolutils.IsManaged(np, c.cloudProvider) || !np.StatusConditions().Root().IsTrue() || np.Spec.Replicas == nil {
		return reconcile.Result{}, nil
	}

	// We need to wait until our representation of cluster is populated accurately with what we have in api-server or else
	// we would end up over provisioning due to misrepresentation of NodePoolState in our cluster state.
	// This usually happens when there is controller crash, so we check if the cluster has synced atleast once.
	if !c.cluster.HasSynced() && !c.cluster.Synced(ctx) {
		return reconcile.Result{RequeueAfter: time.Second}, nil
	}

	runningNodeClaims, _, nodesPendingDisruptionCount := c.cluster.NodePoolState.GetNodeCount(np.Name)
	desiredReplicas := lo.FromPtr(np.Spec.Replicas)
	// Size down of replicas will be handled in deprovisioning controller to drain nodes and delete NodeClaims
	// If there are drifting NodeClaims we need to count them as Active as disruption controller is in the process of creating replacements
	if int64(runningNodeClaims)+int64(nodesPendingDisruptionCount) >= desiredReplicas {
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}

	limit, ok := np.Spec.Limits[resources.Node]
	nodeLimit := lo.Ternary(ok, limit.Value(), int64(math.MaxInt64))
	countNodeClaimsToProvision := c.cluster.NodePoolState.ReserveNodeCount(np.Name, nodeLimit, desiredReplicas-int64(runningNodeClaims))

	if countNodeClaimsToProvision <= 0 {
		log.FromContext(ctx).Info("nodepool node limit reached")
		return reconcile.Result{RequeueAfter: time.Second * 30}, nil
	}

	log.FromContext(ctx).WithValues("current", runningNodeClaims, "desired", desiredReplicas, "provision-count", countNodeClaimsToProvision).
		Info("provisioning nodeclaims to satisfy replica count")

	nodeClaims := make([]*scheduling.NodeClaim, 0, countNodeClaimsToProvision)
	for range countNodeClaimsToProvision {
		nct := scheduling.NewNodeClaimTemplate(np)
		nodeClaims = append(nodeClaims, &scheduling.NodeClaim{
			NodeClaimTemplate: *nct,
		})
	}

	_, err := c.provisioner.CreateNodeClaims(ctx, nodeClaims, provisioning.WithReason(metrics.ProvisionedReason))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("creating nodeclaims, %w", err)
	}

	return reconcile.Result{RequeueAfter: time.Minute}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("static.provisioning").
		// Reoncile on NodePool Create and Update (when replicas change or when NodePool status moves from NotReady to Ready)
		For(&v1.NodePool{}, builder.WithPredicates(nodepoolutils.IsManagedPredicateFuncs(c.cloudProvider), nodepoolutils.IsStaticPredicateFuncs(),
			predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return true
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldNP := e.ObjectOld.(*v1.NodePool)
					newNP := e.ObjectNew.(*v1.NodePool)
					return HasNodePoolReplicaOrStatusChanged(oldNP, newNP)
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			})).
		// We care about Static NodeClaims Deleting (Delete and Update event that has DeletionTimeStamp newly set) as we might have to provision
		Watches(&v1.NodeClaim{}, nodepoolutils.NodeClaimEventHandler(nodepoolutils.WithClient(c.kubeClient), nodepoolutils.WithStaticOnly), builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return e.ObjectOld.GetDeletionTimestamp().IsZero() && !e.ObjectNew.GetDeletionTimestamp().IsZero()
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return true
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		})).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}

func HasNodePoolReplicaOrStatusChanged(oldNP, newNP *v1.NodePool) bool {
	return lo.FromPtr(oldNP.Spec.Replicas) != lo.FromPtr(newNP.Spec.Replicas) || (!oldNP.StatusConditions().Root().IsTrue() && newNP.StatusConditions().Root().IsTrue())
}
