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
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corecontroller "sigs.k8s.io/karpenter/pkg/operator/controller"
	"sigs.k8s.io/karpenter/pkg/utils/result"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instanceprofile"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
)

var _ corecontroller.TypedController[*v1beta1.EC2NodeClass] = (*Controller)(nil)

type nodeClassStatusReconciler interface {
	Reconcile(context.Context, *v1beta1.EC2NodeClass) (reconcile.Result, error)
}

type Controller struct {
	kubeClient client.Client

	ami             *AMI
	instanceprofile *InstanceProfile
	subnet          *Subnet
	securitygroup   *SecurityGroup
	launchtemplate  *LaunchTemplate
}

func NewController(kubeClient client.Client, subnetProvider subnet.Provider, securityGroupProvider securitygroup.Provider,
	amiProvider amifamily.Provider, instanceProfileProvider instanceprofile.Provider, launchTemplateProvider launchtemplate.Provider) corecontroller.Controller {
	return corecontroller.Typed[*v1beta1.EC2NodeClass](kubeClient, &Controller{
		kubeClient: kubeClient,

		ami:             &AMI{amiProvider: amiProvider},
		subnet:          &Subnet{subnetProvider: subnetProvider},
		securitygroup:   &SecurityGroup{securityGroupProvider: securityGroupProvider},
		instanceprofile: &InstanceProfile{instanceProfileProvider: instanceProfileProvider},
		launchtemplate:  &LaunchTemplate{launchTemplateProvider: launchTemplateProvider},
	})
}

func (c *Controller) Reconcile(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(nodeClass, v1beta1.TerminationFinalizer) {
		stored := nodeClass.DeepCopy()
		controllerutil.AddFinalizer(nodeClass, v1beta1.TerminationFinalizer)
		if err := c.kubeClient.Patch(ctx, nodeClass, client.MergeFrom(stored)); err != nil {
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
		c.launchtemplate,
	} {
		res, err := reconciler.Reconcile(ctx, nodeClass)
		errs = multierr.Append(errs, err)
		results = append(results, res)
	}

	if !equality.Semantic.DeepEqual(stored, nodeClass) {
		if err := c.kubeClient.Status().Patch(ctx, nodeClass, client.MergeFrom(stored)); err != nil {
			errs = multierr.Append(errs, client.IgnoreNotFound(err))
		}
	}
	if errs != nil {
		return reconcile.Result{}, errs
	}
	return result.Min(results...), nil
}

func (c *Controller) Name() string {
	return "nodeclass.status"
}

func (c *Controller) Builder(_ context.Context, m manager.Manager) corecontroller.Builder {
	return corecontroller.Adapt(controllerruntime.
		NewControllerManagedBy(m).
		For(&v1beta1.EC2NodeClass{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}))
}
