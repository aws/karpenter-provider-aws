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

package ami

import (
	"context"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/awslabs/operatorpkg/reasonable"
	"github.com/awslabs/operatorpkg/singleton"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	"sort"
	"time"
)

type Controller struct {
	kubeClient  client.Client
	amiProvider amifamily.Provider
}

// NewController is a constructor
func NewController(kubeClient client.Client, amiProvider amifamily.Provider) *Controller {

	return &Controller{
		kubeClient:  kubeClient,
		amiProvider: amiProvider,
	}
}

func (c *Controller) Reconcile(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "nodeclass.ami")

	if !controllerutil.ContainsFinalizer(nodeClass, v1beta1.TerminationFinalizer) {
		stored := nodeClass.DeepCopy()
		controllerutil.AddFinalizer(nodeClass, v1beta1.TerminationFinalizer)
		if err := c.kubeClient.Patch(ctx, nodeClass, client.MergeFrom(stored)); err != nil {
			return reconcile.Result{}, err
		}
	}
	stored := nodeClass.DeepCopy()
	amis, err := c.amiProvider.List(ctx, nodeClass)
	var errs error
	if err != nil {
		nodeClass.StatusConditions().SetFalse(v1beta1.ConditionTypeAMIsReady, "UnresolvedAMIs", "Unable to resolve AMI")
		errs = multierr.Append(errs, err)
	} else {
		if len(amis) == 0 {
			nodeClass.Status.AMIs = nil
			nodeClass.StatusConditions().SetFalse(v1beta1.ConditionTypeAMIsReady, "UnresolvedAMIs", "Unable to resolve AMI")
		} else {
			nodeClass.Status.AMIs = lo.Map(amis, func(ami amifamily.AMI, _ int) v1beta1.AMI {
				reqs := lo.Map(ami.Requirements.NodeSelectorRequirements(), func(item corev1beta1.NodeSelectorRequirementWithMinValues, _ int) v1.NodeSelectorRequirement {
					return item.NodeSelectorRequirement
				})

				sort.Slice(reqs, func(i, j int) bool {
					if len(reqs[i].Key) != len(reqs[j].Key) {
						return len(reqs[i].Key) < len(reqs[j].Key)
					}
					return reqs[i].Key < reqs[j].Key
				})
				return v1beta1.AMI{
					Name:         ami.Name,
					ID:           ami.AmiID,
					Requirements: reqs,
				}
			})
			nodeClass.StatusConditions().SetTrue(v1beta1.ConditionTypeAMIsReady)
		}
	}
	if !equality.Semantic.DeepEqual(stored, nodeClass) {
		if err = c.kubeClient.Status().Update(ctx, nodeClass); err != nil {
			if errors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			return reconcile.Result{}, client.IgnoreNotFound(err)
		}
	}
	if errs != nil {
		return reconcile.Result{}, errs
	}
	if !nodeClass.StatusConditions().IsTrue(v1beta1.ConditionTypeAMIsReady) {
		return reconcile.Result{RequeueAfter: singleton.RequeueImmediately}, nil
	}
	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("nodeclass.ami").
		For(&v1beta1.EC2NodeClass{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{
			RateLimiter:             reasonable.RateLimiter(),
			MaxConcurrentReconciles: 10,
		}).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}
