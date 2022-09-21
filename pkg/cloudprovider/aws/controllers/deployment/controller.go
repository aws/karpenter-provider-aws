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

package deployment

import (
	"context"
	"sync"

	"go.uber.org/multierr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/aws"
	"github.com/aws/karpenter/pkg/utils/injection"
)

const controllerName = "deployment"

// Controller is responsible for watching the Karpenter deployment
// It is responsible for patching the termination finalizer on when the leader pod comes up
// and reacting to the deletion of the deployment so that we can perform some cleanup actions
type Controller struct {
	kubeClient client.Client
	cancel     context.CancelFunc

	sqsProvider         *aws.SQSProvider
	eventBridgeProvider *aws.EventBridgeProvider
}

func NewController(kubeClient client.Client, cancel context.CancelFunc,
	sqsProvider *aws.SQSProvider, eventBridgeProvider *aws.EventBridgeProvider) *Controller {
	return &Controller{
		kubeClient:          kubeClient,
		cancel:              cancel,
		sqsProvider:         sqsProvider,
		eventBridgeProvider: eventBridgeProvider,
	}
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(controllerName))

	deployment := &appsv1.Deployment{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, deployment); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	// If the deletion timestamp is set, that means the deployment is attempting to be deleted
	// and we should perform the cleanup actions associated with the Karpenter deployment
	if !deployment.DeletionTimestamp.IsZero() {
		if err := c.deleteInfrastructure(ctx); err != nil {
			return reconcile.Result{}, err
		}
		patch := client.MergeFrom(deployment.DeepCopy())
		controllerutil.RemoveFinalizer(deployment, v1alpha5.TerminationFinalizer)
		if err := c.kubeClient.Patch(ctx, deployment, patch); err != nil {
			return reconcile.Result{}, err
		}
		c.cancel() // Call cancel to stop the other controllers relying on the infrastructure
		return reconcile.Result{}, nil
	}
	// Otherwise, this is a create/update, so we should just ensure that the finalizer exists
	if !controllerutil.ContainsFinalizer(deployment, v1alpha5.TerminationFinalizer) {
		patch := client.MergeFrom(deployment.DeepCopy())
		controllerutil.AddFinalizer(deployment, v1alpha5.TerminationFinalizer)
		if err := c.kubeClient.Patch(ctx, deployment, patch); err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

// Register the controller to the manager
func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(controllerName).
		For(&appsv1.Deployment{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			// This function ensures that we are filtering out every event that isn't related to the
			// karpenter controller deployment
			if object.GetNamespace() != injection.GetOptions(ctx).DeploymentNamespace {
				return false
			}
			if object.GetName() != injection.GetOptions(ctx).DeploymentName {
				return false
			}
			return true
		})).
		Complete(c)
}

// Delete infrastructure removes the infrastructure that was stood up and reconciled
// by the infrastructure controller for SQS message polling
func (c *Controller) deleteInfrastructure(ctx context.Context) (err error) {
	logging.FromContext(ctx).Infof("Deprovisioning the infrastructure...")
	wg := &sync.WaitGroup{}
	m := &sync.Mutex{}

	wg.Add(2)
	go func() {
		defer wg.Done()
		e := c.sqsProvider.DeleteQueue(ctx)
		m.Lock()
		err = multierr.Append(err, e)
		m.Unlock()
	}()
	go func() {
		defer wg.Done()
		e := c.eventBridgeProvider.DeleteEC2NotificationRules(ctx)
		m.Lock()
		err = multierr.Append(err, e)
		m.Unlock()
	}()
	wg.Wait()
	if err != nil {
		return err
	}
	logging.FromContext(ctx).Infof("Successfully deprovisioned the infrastructure")
	return nil
}
