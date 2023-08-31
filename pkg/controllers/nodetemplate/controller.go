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

	"go.uber.org/multierr"
	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/util/workqueue"
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
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/providers/amifamily"
	"github.com/aws/karpenter/pkg/providers/securitygroup"
	"github.com/aws/karpenter/pkg/providers/subnet"
	nodeclassutil "github.com/aws/karpenter/pkg/utils/nodeclass"
)

type Controller struct {
	kubeClient            client.Client
	subnetProvider        *subnet.Provider
	securityGroupProvider *securitygroup.Provider
	amiProvider           *amifamily.Provider
}

func NewController(kubeClient client.Client, subnetProvider *subnet.Provider,
	securityGroupProvider *securitygroup.Provider, amiProvider *amifamily.Provider) *Controller {
	return &Controller{
		kubeClient:            kubeClient,
		subnetProvider:        subnetProvider,
		securityGroupProvider: securityGroupProvider,
		amiProvider:           amiProvider,
	}
}

func (c *Controller) Reconcile(ctx context.Context, nodeClass *v1beta1.NodeClass) (reconcile.Result, error) {
	stored := nodeClass.DeepCopy()
	nodeClass.Annotations = lo.Assign(nodeClass.Annotations, nodeclassutil.HashAnnotation(nodeClass))
	err := multierr.Combine(
		c.resolveSubnets(ctx, nodeClass),
		c.resolveSecurityGroups(ctx, nodeClass),
		c.resolveAMIs(ctx, nodeClass),
	)
	if !equality.Semantic.DeepEqual(stored, nodeClass) {
		statusCopy := nodeClass.DeepCopy()
		if patchErr := nodeclassutil.Patch(ctx, c.kubeClient, stored, nodeClass); patchErr != nil {
			err = multierr.Append(err, client.IgnoreNotFound(patchErr))
		}
		if patchErr := nodeclassutil.PatchStatus(ctx, c.kubeClient, stored, statusCopy); patchErr != nil {
			err = multierr.Append(err, client.IgnoreNotFound(patchErr))
		}
	}
	return reconcile.Result{RequeueAfter: 5 * time.Minute}, err
}

func (c *Controller) resolveSubnets(ctx context.Context, nodeClass *v1beta1.NodeClass) error {
	subnetList, err := c.subnetProvider.List(ctx, nodeClass)
	if err != nil {
		return err
	}
	if len(subnetList) == 0 {
		nodeClass.Status.Subnets = nil
		return fmt.Errorf("no subnets exist given constraints %v", nodeClass.Spec.SubnetSelectorTerms)
	}
	sort.Slice(subnetList, func(i, j int) bool {
		return int(*subnetList[i].AvailableIpAddressCount) > int(*subnetList[j].AvailableIpAddressCount)
	})
	nodeClass.Status.Subnets = lo.Map(subnetList, func(ec2subnet *ec2.Subnet, _ int) v1beta1.Subnet {
		return v1beta1.Subnet{
			ID:   *ec2subnet.SubnetId,
			Zone: *ec2subnet.AvailabilityZone,
		}
	})
	return nil
}

func (c *Controller) resolveSecurityGroups(ctx context.Context, nodeClass *v1beta1.NodeClass) error {
	securityGroups, err := c.securityGroupProvider.List(ctx, nodeClass)
	if err != nil {
		return err
	}
	if len(securityGroups) == 0 && len(nodeClass.Spec.SecurityGroupSelectorTerms) > 0 {
		nodeClass.Status.SecurityGroups = nil
		return fmt.Errorf("no security groups exist given constraints")
	}
	nodeClass.Status.SecurityGroups = lo.Map(securityGroups, func(securityGroup *ec2.SecurityGroup, _ int) v1beta1.SecurityGroup {
		return v1beta1.SecurityGroup{
			ID:   *securityGroup.GroupId,
			Name: *securityGroup.GroupName,
		}
	})
	return nil
}

func (c *Controller) resolveAMIs(ctx context.Context, nodeClass *v1beta1.NodeClass) error {
	amis, err := c.amiProvider.Get(ctx, nodeClass, &amifamily.Options{})
	if err != nil {
		return err
	}
	if len(amis) == 0 {
		nodeClass.Status.AMIs = nil
		return fmt.Errorf("no amis exist given constraints")
	}
	nodeClass.Status.AMIs = lo.Map(amis, func(ami amifamily.AMI, _ int) v1beta1.AMI {
		return v1beta1.AMI{
			Name:         ami.Name,
			ID:           ami.AmiID,
			Requirements: ami.Requirements.NodeSelectorRequirements(),
		}
	})

	return nil
}

type NodeClassController struct {
	*Controller
}

func NewNodeClassController(kubeClient client.Client, subnetProvider *subnet.Provider,
	securityGroupProvider *securitygroup.Provider, amiProvider *amifamily.Provider) corecontroller.Controller {
	return corecontroller.Typed[*v1beta1.NodeClass](kubeClient, &NodeClassController{
		Controller: NewController(kubeClient, subnetProvider, securityGroupProvider, amiProvider),
	})
}

func (c *NodeClassController) Name() string {
	return "nodeclass"
}

func (c *NodeClassController) Builder(_ context.Context, m manager.Manager) corecontroller.Builder {
	return corecontroller.Adapt(controllerruntime.
		NewControllerManagedBy(m).
		For(&v1beta1.NodeClass{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewMaxOfRateLimiter(
				workqueue.NewItemExponentialFailureRateLimiter(100*time.Millisecond, 1*time.Minute),
				// 10 qps, 100 bucket size
				&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
			),
			MaxConcurrentReconciles: 10,
		}))
}

//nolint:revive
type NodeTemplateController struct {
	*Controller
}

func NewNodeTemplateController(kubeClient client.Client, subnetProvider *subnet.Provider,
	securityGroupProvider *securitygroup.Provider, amiProvider *amifamily.Provider) corecontroller.Controller {
	return corecontroller.Typed[*v1alpha1.AWSNodeTemplate](kubeClient, &NodeTemplateController{
		Controller: NewController(kubeClient, subnetProvider, securityGroupProvider, amiProvider),
	})
}

func (c *NodeTemplateController) Reconcile(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) (reconcile.Result, error) {
	return c.Controller.Reconcile(ctx, nodeclassutil.New(nodeTemplate))
}

func (c *NodeTemplateController) Name() string {
	return "awsnodetemplate"
}

func (c *NodeTemplateController) Builder(_ context.Context, m manager.Manager) corecontroller.Builder {
	return corecontroller.Adapt(controllerruntime.
		NewControllerManagedBy(m).
		For(&v1alpha1.AWSNodeTemplate{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewMaxOfRateLimiter(
				workqueue.NewItemExponentialFailureRateLimiter(100*time.Millisecond, 1*time.Minute),
				// 10 qps, 100 bucket size
				&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
			),
			MaxConcurrentReconciles: 10,
		}))
}
