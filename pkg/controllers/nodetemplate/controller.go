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

	controllerruntime "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"

	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider"
)

var _ corecontroller.TypedController[*v1alpha1.AWSNodeTemplate] = (*Controller)(nil)

type Controller struct {
	kubeClient     k8sClient.Client
	subnet         *cloudprovider.SubnetProvider
	securityGroups *cloudprovider.SecurityGroupProvider
}

func NewController(client k8sClient.Client, ec2api ec2iface.EC2API, subnetProvider *cloudprovider.SubnetProvider, securityGroups *cloudprovider.SecurityGroupProvider) corecontroller.Controller {

	return corecontroller.Typed[*v1alpha1.AWSNodeTemplate](client, &Controller{
		kubeClient:     client,
		subnet:         subnetProvider,
		securityGroups: securityGroups,
	})
}

func (c *Controller) Reconcile(ctx context.Context, ant *v1alpha1.AWSNodeTemplate) (reconcile.Result, error) {
	subnetList, err := c.subnet.Get(ctx, ant, true)
	subnetLog := cloudprovider.PrettySubnets(subnetList)
	if err != nil {
		// Back off and retry reconciliation
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, err
	}

	ant.Status.SubnetsIDs = nil
	ant.Status.SubnetsIDs = append(ant.Status.SubnetsIDs, subnetLog...)

	securityGroupIds, err := c.securityGroups.Get(ctx, ant, true)
	if err != nil {
		// Back off and retry reconciliation
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, err
	}

	ant.Status.SecurityGroupsIDs = nil
	ant.Status.SecurityGroupsIDs = append(ant.Status.SecurityGroupsIDs, securityGroupIds...)

	if err := c.kubeClient.Status().Update(ctx, ant); err != nil {
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
