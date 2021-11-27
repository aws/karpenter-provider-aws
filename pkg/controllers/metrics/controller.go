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

package metrics

import (
	"context"
	"time"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Controller struct {
	CloudProvider cloudprovider.CloudProvider
	KubeClient    client.Client
}

func NewController(kubeClient client.Client, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		CloudProvider: cloudProvider,
		KubeClient:    kubeClient,
	}
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, zap.NewNop().Sugar())
	// Does the provisioner exist?
	provisioner := &v1alpha5.Provisioner{}
	if err := c.KubeClient.Get(ctx, req.NamespacedName, provisioner); err != nil {
		if !errors.IsNotFound(err) {
			// Unable to determine existence of the provisioner, try again later.
			return reconcile.Result{}, err
		}

		// The provisioner has been deleted.
		return reconcile.Result{}, nil
	}

	// The provisioner does exist, so update counters.
	if err := c.updateCounts(ctx, provisioner); err != nil {
		return reconcile.Result{}, err
	}

	// Schedule the next run.
	return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(controllerName).
		For(&v1alpha5.Provisioner{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 10,
		}).
		Complete(c)
}

func (c *Controller) updateCounts(ctx context.Context, provisioner *v1alpha5.Provisioner) error {
	updateCountFuncs := []func(context.Context, *v1alpha5.Provisioner) error{
		c.updateNodeCounts,
		c.updatePodCounts,
	}
	updateCountFuncsLen := len(updateCountFuncs)
	errors := make([]error, updateCountFuncsLen)
	workqueue.ParallelizeUntil(ctx, updateCountFuncsLen, updateCountFuncsLen, func(index int) {
		errors[index] = updateCountFuncs[index](ctx, provisioner)
	})

	return multierr.Combine(errors...)
}

func (c *Controller) updateNodeCounts(ctx context.Context, provisioner *v1alpha5.Provisioner) error {
	instanceTypes, err := c.CloudProvider.GetInstanceTypes(ctx, &provisioner.Spec.Constraints)
	if err != nil {
		return err
	}

	archValues := sets.NewString()
	instanceTypeValues := sets.NewString()
	zoneValues := sets.NewString()
	for _, instanceType := range instanceTypes {
		archValues.Insert(instanceType.Architecture())
		instanceTypeValues.Insert(instanceType.Name())
		for _, offering := range instanceType.Offerings() {
			zoneValues.Insert(offering.Zone)
		}
	}
	knownValuesForNodeLabels := map[string]sets.String{
		nodeLabelArch:         archValues,
		nodeLabelInstanceType: instanceTypeValues,
		nodeLabelZone:         zoneValues,
	}

	return publishNodeCounts(provisioner.Name, knownValuesForNodeLabels, func(matchingLabels client.MatchingLabels, consume nodeListConsumerFunc) error {
		nodes := v1.NodeList{}
		if err := c.KubeClient.List(ctx, &nodes, matchingLabels); err != nil {
			return err
		}
		return consume(nodes.Items)
	})
}

func (c *Controller) updatePodCounts(ctx context.Context, provisioner *v1alpha5.Provisioner) error {
	podsForProvisioner, err := c.podsForProvisioner(ctx, provisioner)
	if err != nil {
		return err
	}

	return publishPodCounts(provisioner.Name, podsForProvisioner)
}

// podsForProvisioner returns a map of slices containing all pods scheduled to nodes in each zone.
func (c *Controller) podsForProvisioner(ctx context.Context, provisioner *v1alpha5.Provisioner) ([]v1.Pod, error) {
	// Karpenter does not apply a label, or other marker, to pods.

	results := []v1.Pod{}

	// 1. Fetch all nodes associated with the provisioner.
	nodeList := v1.NodeList{}
	withProvisionerName := client.MatchingLabels{nodeLabelProvisioner: provisioner.Name}
	if err := c.KubeClient.List(ctx, &nodeList, withProvisionerName); err != nil {
		return nil, err
	}

	// 2. Get all the pods scheduled to each node.
	for _, node := range nodeList.Items {
		podList := v1.PodList{}
		withNodeName := client.MatchingFields{"spec.nodeName": node.Name}
		if err := c.KubeClient.List(ctx, &podList, withNodeName); err != nil {
			return nil, err
		}

		results = append(results, podList.Items...)
	}

	return results, nil
}
