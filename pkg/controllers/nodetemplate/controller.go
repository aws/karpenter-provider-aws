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
	"fmt"
	"net/http"

	"github.com/samber/lo"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	"github.com/aws/karpenter-core/pkg/utils/result"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
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
	kubeClient     client.Client
	finalizer      *Finalizer
	infrastructure *Infrastructure
}

func NewController(kubeClient client.Client, sqsProvider *providers.SQS, eventBridgeProvider *providers.EventBridge) *Controller {
	return &Controller{
		kubeClient:     kubeClient,
		finalizer:      &Finalizer{},
		infrastructure: &Infrastructure{kubeClient: kubeClient, provider: providers.NewInfrastructure(sqsProvider, eventBridgeProvider)},
	}
}

// Reconcile reconciles the AWSNodeTemplate with its sub-reconcilers
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(Name))
	stored := &v1alpha1.AWSNodeTemplate{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, stored); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	nodeTemplate := stored.DeepCopy()
	var results []reconcile.Result
	var errs error
	for _, r := range []interface {
		Reconcile(context.Context, *v1alpha1.AWSNodeTemplate) (reconcile.Result, error)
	}{
		c.infrastructure,
		c.finalizer,
	} {
		res, err := r.Reconcile(ctx, nodeTemplate)
		errs = multierr.Append(errs, err)
		results = append(results, res)
	}
	// If there are any errors, we shouldn't apply the changes, we should requeue
	if errs != nil {
		return reconcile.Result{}, errs
	}
	if !equality.Semantic.DeepEqual(nodeTemplate, stored) {
		if err := c.kubeClient.Patch(ctx, nodeTemplate, client.MergeFrom(stored)); err != nil {
			return reconcile.Result{}, fmt.Errorf("patching AWSNodeTemplate, %w", err)
		}
	}
	return result.Min(results...), nil
}

func (c *Controller) Builder(_ context.Context, m manager.Manager) corecontroller.Builder {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(Name).
		For(&v1alpha1.AWSNodeTemplate{})
}

func (c *Controller) LivenessProbe(_ *http.Request) error {
	return nil
}
