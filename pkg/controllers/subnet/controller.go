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

package subnet

import (
	"context"
	"fmt"
	"time"

	"github.com/mitchellh/hashstructure/v2"
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
	"github.com/patrickmn/go-cache"

	corecloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter-core/pkg/utils/functional"
	"github.com/aws/karpenter-core/pkg/utils/pretty"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
)

type Controller struct {
	cloudprovider corecloudprovider.CloudProvider
	kubeClient    k8sClient.Client
	ec2api        ec2iface.EC2API
	subnetCache   *cache.Cache
	cm            *pretty.ChangeMonitor
}

func NewController(client k8sClient.Client, ec2api ec2iface.EC2API, subnetCache *cache.Cache, cloudProvider corecloudprovider.CloudProvider) *Controller {
	return &Controller{
		cloudprovider: cloudProvider,
		kubeClient:    client,
		ec2api:        ec2api,
		subnetCache:   subnetCache,
		cm:            pretty.NewChangeMonitor(),
	}
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var ant v1alpha1.AWSNodeTemplate
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: req.Name}, &ant); err != nil {
		return reconcile.Result{Requeue: false}, fmt.Errorf("could Not find AWSNodeTemplate %w", err)
	}

	subnetFilters := c.getSubnetFilters(&ant.Spec.AWS)

	subnetHash, err := hashstructure.Hash(subnetFilters, hashstructure.FormatV2, nil)
	if err != nil {
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, err
	}

	subnetOutput, err := c.ec2api.DescribeSubnetsWithContext(ctx, &ec2.DescribeSubnetsInput{Filters: subnetFilters})
	if err != nil {
		// Back off and retry to describe the subnets
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, fmt.Errorf("describing subnets %s, %w", pretty.Concise(subnetFilters), err)
	}
	if len(subnetOutput.Subnets) == 0 {
		// Back off and retry to see if there are any new subnets
		return reconcile.Result{RequeueAfter: 5 * time.Minute}, fmt.Errorf("no subnets matched selector %v", ant.Spec.AWS.SubnetSelector)
	}

	subnetLog := prettySubnets(subnetOutput.Subnets)
	c.subnetCache.SetDefault(fmt.Sprint(subnetHash), subnetOutput.Subnets)
	if c.cm.HasChanged(fmt.Sprintf("subnets-ids (%s)", req.Name), subnetLog) {
		logging.FromContext(ctx).With("subnets", subnetLog).Debugf("discovered subnets for AWSNodeTemplate (%s)", req.Name)
	}

	ant.Status.Subnets = nil
	ant.Status.Subnets = append(ant.Status.Subnets, subnetLog...)

	if err := c.kubeClient.Status().Update(ctx, &ant); err != nil {
		return reconcile.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("could not update status of AWSNodeTemplate %w", err)
	}

	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (c *Controller) Name() string {
	return "Subnets"
}

func (c *Controller) Builder(_ context.Context, m manager.Manager) corecontroller.Builder {
	return corecontroller.Adapt(
		controllerruntime.NewControllerManagedBy(m).
			Named(c.Name()).
			For(&v1alpha1.AWSNodeTemplate{}).
			WithOptions(controller.Options{MaxConcurrentReconciles: 10}))
}

func (c *Controller) getSubnetFilters(provider *v1alpha1.AWS) []*ec2.Filter {
	filters := []*ec2.Filter{}
	// Filter by subnet
	for key, value := range provider.SubnetSelector {
		if key == "aws-ids" {
			filterValues := functional.SplitCommaSeparatedString(value)
			filters = append(filters, &ec2.Filter{
				Name:   aws.String("subnet-id"),
				Values: aws.StringSlice(filterValues),
			})
		} else if value == "*" {
			filters = append(filters, &ec2.Filter{
				Name:   aws.String("tag-key"),
				Values: []*string{aws.String(key)},
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

func prettySubnets(subnets []*ec2.Subnet) []string {
	names := []string{}
	for _, subnet := range subnets {
		names = append(names, fmt.Sprintf("%s (%s)", aws.StringValue(subnet.SubnetId), aws.StringValue(subnet.AvailabilityZone)))
	}
	return names
}
