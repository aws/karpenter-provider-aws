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

package launch_template

import (
	"context"
	"fmt"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"
	"github.com/awslabs/operatorpkg/reasonable"
	"github.com/awslabs/operatorpkg/status"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/errors"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	"time"
)

type Controller struct {
	kubeClient             client.Client
	launchTemplateProvider launchtemplate.Provider
}

// NewController is a constructor
func NewController(kubeClient client.Client, launchTemplateProvider launchtemplate.Provider) *Controller {

	return &Controller{
		kubeClient:             kubeClient,
		launchTemplateProvider: launchTemplateProvider,
	}
}

func (c *Controller) Reconcile(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "nodeclass.launchtemplate")

	if !controllerutil.ContainsFinalizer(nodeClass, v1beta1.TerminationFinalizer) {
		stored := nodeClass.DeepCopy()
		controllerutil.AddFinalizer(nodeClass, v1beta1.TerminationFinalizer)
		if err := c.kubeClient.Patch(ctx, nodeClass, client.MergeFrom(stored)); err != nil {
			return reconcile.Result{}, err
		}
	}
	// A NodeClass that uses AL2023 requires the cluster CIDR for launching nodes.
	// To allow Karpenter to be used for Non-EKS clusters, resolving the Cluster CIDR
	// will not be done at startup but instead in a reconcile loop.
	if lo.FromPtr(nodeClass.Spec.AMIFamily) == v1beta1.AMIFamilyAL2023 {
		if err := c.launchTemplateProvider.ResolveClusterCIDR(ctx); err != nil {
			nodeClass.StatusConditions().SetFalse(status.ConditionReady, "NodeClassNotReady", "Failed to detect the cluster CIDR")
			if e := c.kubeClient.Status().Update(ctx, nodeClass); e != nil {
				if errors.IsConflict(e) {
					return reconcile.Result{Requeue: true}, nil
				}
				return reconcile.Result{}, client.IgnoreNotFound(e)
			}
			return reconcile.Result{}, fmt.Errorf("failed to detect the cluster CIDR, %w", err)
		}
	}
	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("nodeclass.launchtemplate").
		For(&v1beta1.EC2NodeClass{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{
			RateLimiter:             reasonable.RateLimiter(),
			MaxConcurrentReconciles: 10,
		}).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}
