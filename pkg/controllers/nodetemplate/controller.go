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
	"sort"
	"time"

	"go.uber.org/multierr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/samber/lo"

	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/providers/amifamily"
	"github.com/aws/karpenter/pkg/providers/securitygroup"
	"github.com/aws/karpenter/pkg/providers/subnet"
)

var _ corecontroller.TypedController[*v1alpha1.AWSNodeTemplate] = (*Controller)(nil)

type Controller struct {
	kubeClient            client.Client
	subnetProvider        *subnet.Provider
	securityGroupProvider *securitygroup.Provider
	amiProvider           *amifamily.Provider
}

func NewController(kubeClient client.Client, subnetProvider *subnet.Provider, securityGroups *securitygroup.Provider, amiprovider *amifamily.Provider) corecontroller.Controller {
	return corecontroller.Typed[*v1alpha1.AWSNodeTemplate](kubeClient, &Controller{
		kubeClient:            kubeClient,
		subnetProvider:        subnetProvider,
		securityGroupProvider: securityGroups,
		amiProvider:           amiprovider,
	})
}

func (c *Controller) Reconcile(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) (reconcile.Result, error) {
	stored := nodeTemplate.DeepCopy()

	err := multierr.Combine(
		c.resolveSubnets(ctx, nodeTemplate),
		c.resolveSecurityGroup(ctx, nodeTemplate),
		c.resolveAMI(ctx, nodeTemplate),
	)

	if patchErr := c.kubeClient.Status().Patch(ctx, nodeTemplate, client.MergeFrom(stored)); patchErr != nil {
		err = multierr.Append(err, client.IgnoreNotFound(patchErr))
	}

	return reconcile.Result{RequeueAfter: 5 * time.Minute}, err
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

	nodeTemplate.Status.Subnets = lo.Map(subnetList, func(ec2subnet *ec2.Subnet, _ int) v1alpha1.Subnet {
		return v1alpha1.Subnet{
			ID:   *ec2subnet.SubnetId,
			Zone: *ec2subnet.AvailabilityZone,
		}
	})

	return nil
}

func (c *Controller) resolveSecurityGroup(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) error {
	securityGroupIds, err := c.securityGroupProvider.List(ctx, nodeTemplate)
	if err != nil {
		return err
	}

	nodeTemplate.Status.SecurityGroups = lo.Map(securityGroupIds, func(id string, _ int) v1alpha1.SecurityGroup {
		return v1alpha1.SecurityGroup{
			ID: id,
		}
	})

	return nil
}

func (c *Controller) resolveAMI(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) error {
	amiFamily := amifamily.GetAMIFamily(nodeTemplate.Spec.AMIFamily, &amifamily.Options{})

	amiRequirement, err := c.amiProvider.GetAMIWithRequirements(ctx, nodeTemplate, amiFamily)
	if err != nil {
		nodeTemplate.Status.AMIs = nil
		return err
	}

	nodeTemplate.Status.AMIs = lo.Map(lo.Keys(amiRequirement), func(ami amifamily.AMI, _ int) v1alpha1.AMI {
		return v1alpha1.AMI{
			ID:           ami.AmiID,
			Requirements: amiRequirement[ami].NodeSelectorRequirements(),
		}
	})

	return nil
}
