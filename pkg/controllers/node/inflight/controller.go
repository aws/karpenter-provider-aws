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

package inflight

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/utils/functional"

	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/controllers/node"

	clock "k8s.io/apimachinery/pkg/util/clock"

	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/cloudprovider"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

const controllerName = "inflightnode"

// NewController constructs a controller instance.  The inflight controller is responsible for monitoring v1alpha5.InFlightNode
// objects and:
// - deleting them when the real node object is created
// - deleting them and terminating the instance with the cloud provider if the node object fails to come up
func NewController(clock clock.Clock, kubeClient client.Client, cluster *state.Cluster, cp cloudprovider.CloudProvider) *Controller {
	return &Controller{
		kubeClient:    kubeClient,
		clock:         clock,
		cloudProvider: cp,
		cluster:       cluster,
	}
}

// Controller manages a set of properties on karpenter provisioned nodes, such as
// taints, labels, finalizers.
type Controller struct {
	kubeClient    client.Client
	clock         clock.Clock
	cloudProvider cloudprovider.CloudProvider
	cluster       *state.Cluster
}

// Reconcile executes a reallocation control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(controllerName).With("inflightnode", req.Name))

	// Does the InFlightNode still exist?
	stored := &v1alpha5.InFlightNode{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, stored); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// see if the node object that the InFlightNode is representing exists yet
	var k8sNode v1.Node
	realNodeExists := false
	err := c.kubeClient.Get(ctx, client.ObjectKey{Name: stored.Name}, &k8sNode)
	if err != nil && !errors.IsNotFound(err) {
		return reconcile.Result{}, fmt.Errorf("getting Node %s, %w", stored.Name, err)
	} else if err == nil {
		realNodeExists = true
	}

	// if the InFlightNode was deleted, but the real node doesn't exist we should terminate the underlying
	// cloudprovider instance
	if !stored.DeletionTimestamp.IsZero() && !realNodeExists {
		return c.terminateNodeInstance(ctx, stored)
	}

	// both the InFlightNode and the real node exist, so we should delete the inflight node
	if realNodeExists {
		return c.deleteInflightNode(ctx, stored)
	}

	// has kubelet failed to come up and register the node?
	if age := c.clock.Now().Sub(stored.GetCreationTimestamp().Time); age > node.InitializationTimeout {
		return c.expireInflightNode(ctx, stored)
	}

	// Inflight nodes shouldn't persist for long. As soon as Kubelet comes up, we can delete them.
	return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
}

func (c *Controller) deleteInflightNode(ctx context.Context, stored *v1alpha5.InFlightNode) (reconcile.Result, error) {
	/*	// the actual node object exists so we can possibly delete the inflight node
		knownByCluster := false
		c.cluster.ForEachNode(func(n *state.Node) bool {
			if n.Node.Name == stored.Name && !n.InFlightNode {
				knownByCluster = true
			}
			return true
		})

		// cluster state hasn't noticed the v1.Node yet, so we can't delete the inflight node or we will
		// have a race where cluster capacity isn't fully known for a bit.
		if !knownByCluster {
			return reconcile.Result{RequeueAfter: 1 * time.Second}, nil
		}
	*/
	// the node object exists and the cluster state has reconciled it, so we can delete the inflight registration
	if err := c.removeFinalizer(ctx, stored); err != nil {
		return reconcile.Result{}, err
	}
	if err := c.kubeClient.Delete(ctx, stored); err != nil && !errors.IsNotFound(err) {
		return reconcile.Result{}, fmt.Errorf("deleting InflightNode %s, %w", stored.Name, err)
	}
	return reconcile.Result{}, nil
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(controllerName).
		For(&v1alpha5.InFlightNode{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(c)
}

func (c *Controller) expireInflightNode(ctx context.Context, stored *v1alpha5.InFlightNode) (reconcile.Result, error) {
	logging.FromContext(ctx).Infof("Triggering termination for node that failed to become ready (kubelet failed to start)")
	err := c.cloudProvider.Delete(ctx, stored.ToNode())

	if err != nil && !errors.IsNotFound(err) {
		return reconcile.Result{}, fmt.Errorf("deleting node, %w", err)
	}

	// and delete the inflight node object
	err = c.kubeClient.Delete(ctx, stored)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("deleting InFlightNode, %w", err)
	}
	return reconcile.Result{}, nil
}

func (c *Controller) terminateNodeInstance(ctx context.Context, stored *v1alpha5.InFlightNode) (reconcile.Result, error) {
	// 1. Delete the instance associated with node
	err := c.cloudProvider.Delete(ctx, stored.ToNode())
	if err != nil && !errors.IsNotFound(err) {
		return reconcile.Result{}, fmt.Errorf("terminating cloudprovider instance, %w", err)
	}

	// 2. Remove finalizer from the inflight node in APIServer
	if err := c.removeFinalizer(ctx, stored); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (c *Controller) removeFinalizer(ctx context.Context, stored *v1alpha5.InFlightNode) error {
	persisted := stored.DeepCopy()
	stored.Finalizers = functional.StringSliceWithout(stored.Finalizers, v1alpha5.TerminationFinalizer)
	if err := c.kubeClient.Patch(ctx, stored, client.MergeFrom(persisted)); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("removing finalizer from node, %w", err)
	}
	return nil
}
