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
	"github.com/aws/karpenter/pkg/providers/amifamily"
	"github.com/aws/karpenter/pkg/providers/securitygroup"
	"github.com/aws/karpenter/pkg/providers/subnet"
	nodeclassutil "github.com/aws/karpenter/pkg/utils/nodeclass"
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

	nodeTemplate.Annotations = lo.Assign(nodeTemplate.ObjectMeta.Annotations, map[string]string{v1alpha1.AnnotationNodeTemplateHash: nodeTemplate.Hash()})
	err := multierr.Combine(
		c.resolveSubnets(ctx, nodeTemplate),
		c.resolveSecurityGroups(ctx, nodeTemplate),
		c.resolveAMIs(ctx, nodeTemplate),
	)

	if !equality.Semantic.DeepEqual(stored, nodeTemplate) {
		statusCopy := nodeTemplate.DeepCopy()
		if patchErr := c.kubeClient.Patch(ctx, nodeTemplate, client.MergeFrom(stored)); patchErr != nil {
			err = multierr.Append(err, client.IgnoreNotFound(patchErr))
		}
		if patchErr := c.kubeClient.Status().Patch(ctx, statusCopy, client.MergeFrom(stored)); patchErr != nil {
			err = multierr.Append(err, client.IgnoreNotFound(patchErr))
		}
	}

	return reconcile.Result{RequeueAfter: 5 * time.Minute}, err
}

func (c *Controller) Name() string {
	return "awsnodetemplate"
}

func (c *Controller) Builder(_ context.Context, m manager.Manager) corecontroller.Builder {
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

func (c *Controller) resolveSubnets(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) error {
	subnetList, err := c.subnetProvider.List(ctx, nodeclassutil.New(nodeTemplate))
	if err != nil {
		return err
	}
	if len(subnetList) == 0 {
		nodeTemplate.Status.Subnets = nil
		return fmt.Errorf("no subnets exist given constraints %v", nodeTemplate.Spec.SubnetSelector)
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

func (c *Controller) resolveSecurityGroups(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) error {
	securityGroups, err := c.securityGroupProvider.List(ctx, nodeclassutil.New(nodeTemplate))
	if err != nil {
		return err
	}
	if len(securityGroups) == 0 && nodeTemplate.Spec.SecurityGroupSelector != nil {
		nodeTemplate.Status.SecurityGroups = nil
		return fmt.Errorf("no security groups exist given constraints")
	}

	nodeTemplate.Status.SecurityGroups = lo.Map(securityGroups, func(securityGroup *ec2.SecurityGroup, _ int) v1alpha1.SecurityGroup {
		return v1alpha1.SecurityGroup{
			ID:   *securityGroup.GroupId,
			Name: *securityGroup.GroupName,
		}
	})

	return nil
}

func (c *Controller) resolveAMIs(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate) error {
	amis, err := c.amiProvider.Get(ctx, nodeclassutil.New(nodeTemplate), &amifamily.Options{})
	if err != nil {
		return err
	}
	if len(amis) == 0 {
		nodeTemplate.Status.AMIs = nil
		return fmt.Errorf("no amis exist given constraints")
	}

	nodeTemplate.Status.AMIs = lo.Map(amis, func(ami amifamily.AMI, _ int) v1alpha1.AMI {
		return v1alpha1.AMI{
			Name:         ami.Name,
			ID:           ami.AmiID,
			Requirements: ami.Requirements.NodeSelectorRequirements(),
		}
	})

	return nil
}
