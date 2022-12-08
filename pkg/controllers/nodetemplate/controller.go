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
	"time"

	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"

	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/utils"
)

type Controller struct {
	kubeClient     k8sClient.Client
	subnet         *cloudprovider.SubnetProvider
	securityGroups *cloudprovider.SecurityGroupProvider
}

func NewController(client k8sClient.Client, ec2api ec2iface.EC2API, subnetProvider *cloudprovider.SubnetProvider, securityGroups *cloudprovider.SecurityGroupProvider) *Controller {
	return &Controller{
		kubeClient:     client,
		subnet:         subnetProvider,
		securityGroups: securityGroups,
	}
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var ant v1alpha1.AWSNodeTemplate
	err := c.kubeClient.Get(ctx, types.NamespacedName{Name: req.Name}, &ant)
	if err != nil {
		if err.Error() == fmt.Sprintf("AWSNodeTemplate.karpenter.k8s.aws \"%s\" not found", req.Name) {
			logging.FromContext(ctx).Info("could not find AWSNodeTemplate (%s)", req.Name)
			return reconcile.Result{Requeue: false}, nil
		}
		return reconcile.Result{Requeue: false}, fmt.Errorf("%w", err)
	}

	subnetList, err := c.subnet.Get(ctx, &ant, true)
	subnetLog := utils.SubnetIds(subnetList)
	if err != nil {
		// Back off and retry reconciliation
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, err
	}

	ant.Status.Subnets = nil
	ant.Status.Subnets = append(ant.Status.Subnets, subnetLog...)

	securityGroupIds, err := c.securityGroups.Get(ctx, &ant, true)
	if err != nil {
		// Back off and retry reconciliation
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, err
	}

	ant.Status.SecurityGroups = nil
	ant.Status.SecurityGroups = append(ant.Status.SecurityGroups, securityGroupIds...)

	if err := c.kubeClient.Status().Update(ctx, &ant); err != nil {
		return reconcile.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("could not update status of AWSNodeTemplate %w", err)
	}

	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (c *Controller) Name() string {
	return "AWSNodeTemplate Status"
}

func (c *Controller) Builder(ctx context.Context, m manager.Manager) corecontroller.Builder {
	return corecontroller.Adapt(
		controllerruntime.NewControllerManagedBy(m).
			Named(c.Name()).
			For(&v1alpha1.AWSNodeTemplate{}).
			WithOptions(controller.Options{MaxConcurrentReconciles: 10}))
}
