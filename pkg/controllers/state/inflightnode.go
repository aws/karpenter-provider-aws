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

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"

	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const inflightNodeControllerName = "inflightnode-state"

// InflightNodeController reconciles nodes for the purpose of maintaining state regarding nodes that is expensive to compute.
type InflightNodeController struct {
	kubeClient client.Client
	cluster    *Cluster
}

// NewInflightNodeController constructs a controller instance
func NewInflightNodeController(kubeClient client.Client, cluster *Cluster) *InflightNodeController {
	return &InflightNodeController{
		kubeClient: kubeClient,
		cluster:    cluster,
	}
}

func (c *InflightNodeController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(inflightNodeControllerName).With("inflightnode", req.Name))
	node := &v1alpha5.InFlightNode{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			c.cluster.deleteInflightNode(req.Name)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	c.cluster.updateInflightNode(node)
	return reconcile.Result{Requeue: true, RequeueAfter: stateRetryPeriod}, nil
}

func (c *InflightNodeController) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(inflightNodeControllerName).
		For(&v1alpha5.InFlightNode{}).
		Complete(c)
}
