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

package termination

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"
	"knative.dev/pkg/logging"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	provisioning "github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/events"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/injection"
)

const controllerName = "termination"

// Controller for the resource
type Controller struct {
	Terminator *Terminator
	KubeClient client.Client
	Recorder   events.Recorder
}

// NewController constructs a controller instance
func NewController(ctx context.Context, kubeClient client.Client, coreV1Client corev1.CoreV1Interface, recorder events.Recorder, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		KubeClient: kubeClient,
		Terminator: &Terminator{
			KubeClient:    kubeClient,
			CoreV1Client:  coreV1Client,
			CloudProvider: cloudProvider,
			EvictionQueue: NewEvictionQueue(ctx, coreV1Client, recorder),
		},
		Recorder: recorder,
	}
}

// Reconcile executes a termination control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(controllerName).With("node", req.Name))
	ctx = injection.WithControllerName(ctx, controllerName)

	// 1. Retrieve node from reconcile request
	node := &v1.Node{}
	if err := c.KubeClient.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// 2. Check if node is terminable
	if node.DeletionTimestamp.IsZero() || !functional.ContainsString(node.Finalizers, provisioning.TerminationFinalizer) {
		return reconcile.Result{}, nil
	}
	// 3. Cordon node
	if err := c.Terminator.cordon(ctx, node); err != nil {
		return reconcile.Result{}, fmt.Errorf("cordoning node %s, %w", node.Name, err)
	}
	// 4. Drain node
	drained, err := c.Terminator.drain(ctx, node)
	if err != nil {
		if IsNodeDrainErr(err) {
			c.Recorder.NodeFailedToDrain(node, err)
		} else {
			return reconcile.Result{}, fmt.Errorf("draining node %s, %w", node.Name, err)
		}
	}
	if !drained {
		return reconcile.Result{Requeue: true}, nil
	}
	// 5. If fully drained, terminate the node
	if err := c.Terminator.terminate(ctx, node); err != nil {
		return reconcile.Result{}, fmt.Errorf("terminating node %s, %w", node.Name, err)
	}
	return reconcile.Result{}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(controllerName).
		For(&v1.Node{}).
		WithOptions(
			controller.Options{
				RateLimiter: workqueue.NewMaxOfRateLimiter(
					workqueue.NewItemExponentialFailureRateLimiter(100*time.Millisecond, 10*time.Second),
					// 10 qps, 100 bucket size
					&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
				),
				MaxConcurrentReconciles: 10,
			},
		).
		Complete(c)
}
