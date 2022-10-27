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
	"net/http"
	"time"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorcontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter-core/pkg/operator/scheme"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/controllers/providers"
)

const Name = "nodetemplate"

func init() {
	lo.Must0(apis.AddToScheme(scheme.Scheme))
}

// Controller is the AWSNodeTemplate Controller
// It sub-reconciles by checking if there are any AWSNodeTemplates and provisions infrastructure
// if there is. If there are no templates, then it de-provisions the infrastructure.
type Controller struct {
	kubeClient client.Client
	provider   *providers.Infrastructure

	lastInfrastructureReconcile time.Time
}

func NewController(kubeClient client.Client, sqsProvider *providers.SQS, eventBridgeProvider *providers.EventBridge) *Controller {
	return &Controller{
		kubeClient: kubeClient,
		provider:   providers.NewInfrastructure(sqsProvider, eventBridgeProvider),
	}
}

// Reconcile reconciles the SQS queue and the EventBridge rules with the expected
// configuration prescribed by Karpenter
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
			if err := c.provider.Delete(ctx); err != nil {
				return reconcile.Result{}, err
			}
		}
		mergeFrom := client.MergeFrom(nt.DeepCopy())
		controllerutil.RemoveFinalizer(nt, v1alpha5.TerminationFinalizer)
		if err := c.kubeClient.Patch(ctx, nt, mergeFrom); err != nil {
			return reconcile.Result{}, err
		}
		active.Set(0)
		return reconcile.Result{}, nil
	} else if len(list.Items) >= 1 {
		mergeFrom := client.MergeFrom(nt.DeepCopy())
		controllerutil.AddFinalizer(nt, v1alpha5.TerminationFinalizer)
		if err := c.kubeClient.Patch(ctx, nt, mergeFrom); err != nil {
			return reconcile.Result{}, err
		}
		active.Set(1)
		//if settings.FromContext(ctx).EnableInterruptionHandling &&
		//	c.lastInfrastructureReconcile.Add(time.Hour).Before(time.Now()) {

		if err := c.provider.Create(ctx); err != nil {
			healthy.Set(0)
			return reconcile.Result{}, err
		}
		c.lastInfrastructureReconcile = time.Now()
		healthy.Set(1)
		//}
	}
	// TODO: Implement an alerting mechanism for settings updates; until then, just poll
	return reconcile.Result{RequeueAfter: time.Second * 10}, nil
}

func (c *Controller) Builder(_ context.Context, m manager.Manager) operatorcontroller.Builder {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(Name).
		For(&v1alpha1.AWSNodeTemplate{})
}

func (c *Controller) LivenessProbe(_ *http.Request) error {
	return nil
}
