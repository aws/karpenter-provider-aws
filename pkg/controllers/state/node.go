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

package state

import (
	"context"
	"sync"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const nodeControllerName = "node-state"

// NodeController reconciles nodes for the purpose of maintaining state regarding nodes that is expensive to compute.
type NodeController struct {
	kubeClient client.Client
	cluster    *Cluster
	labelMap   sync.Map
}

// NewNodeController constructs a controller instance
func NewNodeController(kubeClient client.Client, cluster *Cluster) *NodeController {
	return &NodeController{
		kubeClient: kubeClient,
		cluster:    cluster,
	}
}

func (c *NodeController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(nodeControllerName).With("node", req.Name))

	c.cleanup(req.NamespacedName)

	node := &v1.Node{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			// notify cluster state of the node deletion
			c.cluster.deleteNode(req.Name)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if err := c.cluster.updateNode(ctx, node); err != nil {
		return reconcile.Result{}, err
	}

	if err := c.record(ctx, node); err != nil {
		logging.FromContext(ctx).Errorf("Failed to update gauges: %s", err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{Requeue: true, RequeueAfter: stateRetryPeriod}, nil
}

// TODO: Determine if additional watches conflict w/ cluster state
func (c *NodeController) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(nodeControllerName).
		For(&v1.Node{}).
		Watches(
			// Reconcile all nodes related to a provisioner when it changes.
			&source.Kind{Type: &v1alpha5.Provisioner{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) (requests []reconcile.Request) {
				nodes := &v1.NodeList{}
				if err := c.kubeClient.List(ctx, nodes, client.MatchingLabels(map[string]string{v1alpha5.ProvisionerNameLabelKey: o.GetName()})); err != nil {
					logging.FromContext(ctx).Errorf("Failed to list nodes when mapping expiration watch events, %s", err)
					return requests
				}
				for _, node := range nodes.Items {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: node.Name}})
				}
				return requests
			}),
		).
		Watches(
			// Reconcile nodes where pods have changed
			&source.Kind{Type: &v1.Pod{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) (requests []reconcile.Request) {
				if name := o.(*v1.Pod).Spec.NodeName; name != "" {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
				}
				return requests
			}),
		).
		Complete(c)
}
