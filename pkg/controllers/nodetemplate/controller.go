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
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/aws-sdk-go/service/ec2"

	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/providers/securitygroup"
	"github.com/aws/karpenter/pkg/providers/subnet"
)

var _ corecontroller.TypedController[*v1alpha1.AWSNodeTemplate] = (*Controller)(nil)

type Controller struct {
	kubeClient            k8sClient.Client
	subnetProvider        *subnet.Provider
	securityGroupProvider *securitygroup.Provider
}

func NewController(client k8sClient.Client, subnetProvider *subnet.Provider, securityGroups *securitygroup.Provider) corecontroller.Controller {
	return corecontroller.Typed[*v1alpha1.AWSNodeTemplate](client, &Controller{
		kubeClient:            client,
		subnetProvider:        subnetProvider,
		securityGroupProvider: securityGroups,
	})
}

func (c *Controller) Reconcile(ctx context.Context, ant *v1alpha1.AWSNodeTemplate) (reconcile.Result, error) {
	subnetList, err := c.subnetProvider.List(ctx, ant)
	if err != nil {
		return reconcile.Result{}, err
	}
	err = c.patchSubnetStatus(ctx, ant, subnetList)
	if err != nil {
		return reconcile.Result{}, err
	}

	securityGroupIds, err := c.securityGroupProvider.List(ctx, ant)
	if err != nil {
		return reconcile.Result{}, err
	}
	err = c.patchSecurityGroupStatus(ctx, ant, securityGroupIds)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: time.Minute}, nil
}

func (c *Controller) Name() string {
	return "awsnodetemplate"
}

func (c *Controller) Builder(ctx context.Context, m manager.Manager) corecontroller.Builder {
	return corecontroller.Adapt(
		controllerruntime.NewControllerManagedBy(m).
			For(&v1alpha1.AWSNodeTemplate{}).
			WithOptions(controller.Options{MaxConcurrentReconciles: 10}))
}

func (c *Controller) patchSubnetStatus(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate, subnets []*ec2.Subnet) error {
	var patchstrings []string
	for _, ec2subnet := range subnets {
		patchstrings = append(patchstrings, fmt.Sprintf(`{"id": "%s", "zone": "%s", "availableIpAddressCount": %d}`, *ec2subnet.SubnetId, *ec2subnet.AvailabilityZone, int(*ec2subnet.AvailableIpAddressCount)))
	}
	patchstring := fmt.Sprintf(`{"status":{"subnet":[%s]}}`, strings.Join(patchstrings, ", "))
	patch := []byte(patchstring)
	err := c.kubeClient.Status().Patch(ctx, nodeTemplate, k8sClient.RawPatch(types.MergePatchType, patch))
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) patchSecurityGroupStatus(ctx context.Context, nodeTemplate *v1alpha1.AWSNodeTemplate, securityGroups []string) error {
	var patchstrings []string
	for _, securitygroupsids := range securityGroups {
		patchstrings = append(patchstrings, fmt.Sprintf(`{"id": "%s"}`, securitygroupsids))
	}
	patch := []byte(fmt.Sprintf(`{"status":{"securityGroup":[%s]}}`, strings.Join(patchstrings, ", ")))
	err := c.kubeClient.Status().Patch(ctx, nodeTemplate, k8sClient.RawPatch(types.MergePatchType, patch))
	if err != nil {
		return err
	}
	return nil
}
