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

package securitygroups

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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"

	corecloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter-core/pkg/utils/functional"
	"github.com/aws/karpenter-core/pkg/utils/pretty"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
)

type Controller struct {
	cloudprovider      corecloudprovider.CloudProvider
	kubeClient         k8sClient.Client
	ec2api             ec2iface.EC2API
	securityGroupCache *cache.Cache
	cm                 *pretty.ChangeMonitor
}

func NewController(client k8sClient.Client, ec2api ec2iface.EC2API, SecurityGroupCache *cache.Cache, cloudProvider corecloudprovider.CloudProvider) *Controller {
	return &Controller{
		cloudprovider:      cloudProvider,
		kubeClient:         client,
		ec2api:             ec2api,
		securityGroupCache: SecurityGroupCache,
		cm:                 pretty.NewChangeMonitor(),
	}
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var ant v1alpha1.AWSNodeTemplate
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: req.Name}, &ant); err != nil {
		return reconcile.Result{Requeue: false}, fmt.Errorf("could Not find AWSNodeTemplate %w", err)
	}

	securityGroupFilters := c.getSecurityGroupFilters(&ant.Spec.AWS)

	securityGroupHash, err := hashstructure.Hash(securityGroupFilters, hashstructure.FormatV2, nil)
	if err != nil {
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, err
	}

	securityGroupOutput, err := c.ec2api.DescribeSecurityGroupsWithContext(ctx, &ec2.DescribeSecurityGroupsInput{Filters: securityGroupFilters})
	if err != nil {
		// Back off and retry to describe the subnets
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, fmt.Errorf("describing security groups %+v, %w", securityGroupFilters, err)
	}

	securityGroupIds := c.securityGroupIds(securityGroupOutput.SecurityGroups)
	c.securityGroupCache.SetDefault(fmt.Sprint(securityGroupHash), securityGroupOutput.SecurityGroups)
	if c.cm.HasChanged("security-groups", securityGroupOutput.SecurityGroups) {
		logging.FromContext(ctx).With("security-groups", securityGroupIds).Debugf("discovered security groups")
	}

	ant.Status.SecurityGroups = nil
	ant.Status.SecurityGroups = append(ant.Status.SecurityGroups, securityGroupIds...)

	if err := c.kubeClient.Status().Update(ctx, &ant); err != nil {
		return reconcile.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("could not update status of AWSNodeTemplate %w", err)
	}

	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (c *Controller) Name() string {
	return "Security Groups"
}

func (c *Controller) Builder(_ context.Context, m manager.Manager) corecontroller.Builder {
	return corecontroller.Adapt(
		controllerruntime.NewControllerManagedBy(m).
			Named(c.Name()).
			For(&v1alpha1.AWSNodeTemplate{}).
			WithOptions(controller.Options{MaxConcurrentReconciles: 10}))
}

func (c *Controller) getSecurityGroupFilters(provider *v1alpha1.AWS) []*ec2.Filter {
	filters := []*ec2.Filter{}
	for key, value := range provider.SecurityGroupSelector {
		if key == "aws-ids" {
			filterValues := functional.SplitCommaSeparatedString(value)
			filters = append(filters, &ec2.Filter{
				Name:   aws.String("group-id"),
				Values: aws.StringSlice(filterValues),
			})
		} else {
			filters = append(filters, &ec2.Filter{
				Name:   aws.String(fmt.Sprintf("tag:%s", key)),
				Values: []*string{aws.String(value)},
			})
		}
	}
	return filters
}

func (c *Controller) securityGroupIds(securityGroups []*ec2.SecurityGroup) []string {
	names := []string{}
	for _, securityGroup := range securityGroups {
		names = append(names, aws.StringValue(securityGroup.GroupId))
	}
	return names
}
