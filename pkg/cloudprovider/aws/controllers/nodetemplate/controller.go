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

package nodetemplate

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/infrastructure"
	"github.com/aws/karpenter/pkg/controllers/polling"
)

const Name = "aws.nodetemplate"

// Controller is the AWS Node Template counter and finalizer reconciler. It performs certain operations based on the
// number of AWS Node Templates on the cluster
type Controller struct {
	kubeClient             client.Client
	infraProvider          *infrastructure.Provider
	infraController        polling.ControllerInterface
	notificationController polling.ControllerInterface
}

func NewController(kubeClient client.Client, infraProvider *infrastructure.Provider,
	infraController, notificationController polling.ControllerInterface) *Controller {
	return &Controller{
		kubeClient:             kubeClient,
		infraProvider:          infraProvider,
		infraController:        infraController,
		notificationController: notificationController,
	}
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(Name))
	nt := &v1alpha1.AWSNodeTemplate{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, nt); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	list := &v1alpha1.AWSNodeTemplateList{}
	if err := c.kubeClient.List(ctx, list); err != nil {
		return reconcile.Result{}, err
	}

	// Handle removing the finalizer and also cleaning up the infrastructure on the last AWSNodeTemplate deletion
	if !nt.DeletionTimestamp.IsZero() {
		if len(list.Items) == 1 {
			c.infraController.Stop(ctx)
			c.notificationController.Stop(ctx)
			if err := c.infraProvider.DeleteInfrastructure(ctx); err != nil {
				return reconcile.Result{}, err
			}
		}
		mergeFrom := client.MergeFrom(nt.DeepCopy())
		controllerutil.RemoveFinalizer(nt, v1alpha5.TerminationFinalizer)
		if err := c.kubeClient.Patch(ctx, nt, mergeFrom); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}
	if len(list.Items) >= 1 {
		c.infraController.Start(ctx)
	}
	mergeFrom := client.MergeFrom(nt.DeepCopy())
	controllerutil.AddFinalizer(nt, v1alpha5.TerminationFinalizer)
	if err := c.kubeClient.Patch(ctx, nt, mergeFrom); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(Name).
		For(&v1alpha1.AWSNodeTemplate{}).
		Complete(c)
}
