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

package security_group

import (
	"context"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	"github.com/awslabs/operatorpkg/reasonable"
	"github.com/awslabs/operatorpkg/singleton"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	"sort"
	"time"
)

type Controller struct {
	kubeClient            client.Client
	securityGroupProvider securitygroup.Provider
}

// NewController is a constructor
func NewController(kubeClient client.Client, securityGroupProvider securitygroup.Provider) *Controller {

	return &Controller{
		kubeClient:            kubeClient,
		securityGroupProvider: securityGroupProvider,
	}
}
func (c *Controller) Reconcile(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "nodeclass.securitygroup")

	if !controllerutil.ContainsFinalizer(nodeClass, v1beta1.TerminationFinalizer) {
		stored := nodeClass.DeepCopy()
		controllerutil.AddFinalizer(nodeClass, v1beta1.TerminationFinalizer)
		if err := c.kubeClient.Patch(ctx, nodeClass, client.MergeFrom(stored)); err != nil {
			return reconcile.Result{}, err
		}
	}
	stored := nodeClass.DeepCopy()

	securityGroups, err := c.securityGroupProvider.List(ctx, nodeClass)
	var errs error
	if err != nil {
		nodeClass.StatusConditions().SetFalse(v1beta1.ConditionTypeSecurityGroupsReady, "UnresolvedSecurityGroups", "Unable to resolve SecurityGroups")
		errs = multierr.Append(errs, err)
	} else {
		if len(securityGroups) == 0 && len(nodeClass.Spec.SecurityGroupSelectorTerms) > 0 {
			nodeClass.Status.SecurityGroups = nil
			nodeClass.StatusConditions().SetFalse(v1beta1.ConditionTypeSecurityGroupsReady, "UnresolvedSecurityGroups", "Unable to resolve SecurityGroups")
		} else {
			sort.Slice(securityGroups, func(i, j int) bool {
				return *securityGroups[i].GroupId < *securityGroups[j].GroupId
			})
			nodeClass.Status.SecurityGroups = lo.Map(securityGroups, func(securityGroup *ec2.SecurityGroup, _ int) v1beta1.SecurityGroup {
				return v1beta1.SecurityGroup{
					ID:   *securityGroup.GroupId,
					Name: *securityGroup.GroupName,
				}
			})
			nodeClass.StatusConditions().SetTrue(v1beta1.ConditionTypeSecurityGroupsReady)
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
	if !nodeClass.StatusConditions().IsTrue(v1beta1.ConditionTypeSecurityGroupsReady) {
		return reconcile.Result{RequeueAfter: singleton.RequeueImmediately}, nil
	}
	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("nodeclass.securitygroup").
		For(&v1beta1.EC2NodeClass{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{
			RateLimiter:             reasonable.RateLimiter(),
			MaxConcurrentReconciles: 10,
		}).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}
