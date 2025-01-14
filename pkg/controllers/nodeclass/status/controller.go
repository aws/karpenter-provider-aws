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

package status

import (
	"context"

	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/karpenter/pkg/operator/injection"

	"sigs.k8s.io/karpenter/pkg/utils/result"

	"github.com/awslabs/operatorpkg/reasonable"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instanceprofile"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
)

type nodeClassStatusReconciler interface {
	Reconcile(context.Context, *v1.EC2NodeClass) (reconcile.Result, error)
}

type Controller struct {
	kubeClient client.Client

	ami             *AMI
	instanceprofile *InstanceProfile
	subnet          *Subnet
	securitygroup   *SecurityGroup
	validation      *Validation
	readiness       *Readiness //TODO : Remove this when we have sub status conditions
}

func NewController(kubeClient client.Client, subnetProvider subnet.Provider, securityGroupProvider securitygroup.Provider,
	amiProvider amifamily.Provider, instanceProfileProvider instanceprofile.Provider, launchTemplateProvider launchtemplate.Provider, cloudProvider cloudprovider.CloudProvider, instanceProvider instance.Provider) *Controller {
	return &Controller{
		kubeClient: kubeClient,

		ami:             &AMI{amiProvider: amiProvider},
		subnet:          &Subnet{subnetProvider: subnetProvider},
		securitygroup:   &SecurityGroup{securityGroupProvider: securityGroupProvider},
		instanceprofile: &InstanceProfile{instanceProfileProvider: instanceProfileProvider},
		validation:      &Validation{cloudProvider: cloudProvider, instanceProvider: instanceProvider},
		readiness:       &Readiness{launchTemplateProvider: launchTemplateProvider},
	}
}

func (c *Controller) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "nodeclass.status")

	if !controllerutil.ContainsFinalizer(nodeClass, v1.TerminationFinalizer) {
		stored := nodeClass.DeepCopy()
		controllerutil.AddFinalizer(nodeClass, v1.TerminationFinalizer)

		// We use client.MergeFromWithOptimisticLock because patching a list with a JSON merge patch
		// can cause races due to the fact that it fully replaces the list on a change
		// Here, we are updating the finalizer list
		if err := c.kubeClient.Patch(ctx, nodeClass, client.MergeFromWithOptions(stored, client.MergeFromWithOptimisticLock{})); err != nil {
			if errors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			return reconcile.Result{}, err
		}
	}
	stored := nodeClass.DeepCopy()

	var results []reconcile.Result
	var errs error
	for _, reconciler := range []nodeClassStatusReconciler{
		c.ami,
		c.subnet,
		c.securitygroup,
		c.instanceprofile,
		c.validation,
		c.readiness,
	} {
		res, err := reconciler.Reconcile(ctx, nodeClass)
		errs = multierr.Append(errs, err)
		results = append(results, res)
	}

	if !equality.Semantic.DeepEqual(stored, nodeClass) {
		// We use client.MergeFromWithOptimisticLock because patching a list with a JSON merge patch
		// can cause races due to the fact that it fully replaces the list on a change
		// Here, we are updating the status condition list
		if err := c.kubeClient.Status().Patch(ctx, nodeClass, client.MergeFromWithOptions(stored, client.MergeFromWithOptimisticLock{})); err != nil {
			if errors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			errs = multierr.Append(errs, client.IgnoreNotFound(err))
		}
	}
	if errs != nil {
		return reconcile.Result{}, errs
	}
	return result.Min(results...), nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("nodeclass.status").
		For(&v1.EC2NodeClass{}).
		WithOptions(controller.Options{
			RateLimiter:             reasonable.RateLimiter(),
			MaxConcurrentReconciles: 10,
		}).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}
