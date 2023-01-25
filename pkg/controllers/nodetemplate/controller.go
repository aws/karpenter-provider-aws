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
	"sort"
	"time"

	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/providers/securitygroup"
	"github.com/aws/karpenter/pkg/providers/subnet"
)

var _ corecontroller.TypedController[*v1alpha1.AWSNodeTemplate] = (*Controller)(nil)

type Controller struct {
	client                client.Client
	subnetProvider        *subnet.Provider
	securityGroupProvider *securitygroup.Provider
}

func NewController(client client.Client, subnetProvider *subnet.Provider, securityGroups *securitygroup.Provider) corecontroller.Controller {
	return corecontroller.Typed[*v1alpha1.AWSNodeTemplate](client, &Controller{
		client:                client,
		subnetProvider:        subnetProvider,
		securityGroupProvider: securityGroups,
	})
}

func (c *Controller) Reconcile(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) (reconcile.Result, error) {
	fmt.Println(nodeTemplate.Name)
	if err := c.resolveSubnets(ctx, nodeTemplate); err != nil {
		fmt.Print("Hereererere", err)
		return reconcile.Result{}, err
	}

	if err := c.resolveSecurityGroup(ctx, nodeTemplate); err != nil {
		fmt.Print("Hereererere", err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (c *Controller) Name() string {
	return "awsnodetemplate"
}

func (c *Controller) Builder(ctx context.Context, m manager.Manager) corecontroller.Builder {
	return corecontroller.Adapt(controllerruntime.
		NewControllerManagedBy(m).
		For(&v1alpha1.AWSNodeTemplate{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}))
}

func (c *Controller) resolveSubnets(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) error {
	subnetList, err := c.subnetProvider.List(ctx, nodeTemplate)
	if err != nil {
		return err
	}

	sort.Slice(subnetList, func(i, j int) bool {
		return int(*subnetList[i].AvailableIpAddressCount) > int(*subnetList[j].AvailableIpAddressCount)
	})

	for _, ec2subnet := range subnetList {
		status := v1alpha1.SubnetStatus{
			ID:   *ec2subnet.SubnetId,
			Zone: *ec2subnet.AvailabilityZone,
		}
		nodeTemplate.Status.Subnets = append(nodeTemplate.Status.Subnets, status)
	}

	if c.client.Status().Update(ctx, nodeTemplate) != nil {
		return err
	}

	return nil
}

func (c *Controller) resolveSecurityGroup(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) error {
	securityGroupIds, err := c.securityGroupProvider.List(ctx, nodeTemplate)
	if err != nil {
		return err
	}

	for _, securitygroupsids := range securityGroupIds {
		securityGroupSubnet := v1alpha1.SecurityGroupStatus{
			ID: securitygroupsids,
		}
		nodeTemplate.Status.SecurityGroups = append(nodeTemplate.Status.SecurityGroups, securityGroupSubnet)
	}

	if c.client.Status().Update(ctx, nodeTemplate) != nil {
		return err
	}

	return nil
}
