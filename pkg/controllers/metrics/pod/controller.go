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

package pod

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha4"
	podutils "github.com/awslabs/karpenter/pkg/utils/pod"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	controllerName  = "PodMetrics"
	requeueInterval = 10 * time.Second
)

type Controller struct {
	Filter     *podutils.Filter
	KubeClient client.Client
}

func NewController(kubeClient client.Client) *Controller {
	return &Controller{
		Filter:     &podutils.Filter{KubeClient: kubeClient},
		KubeClient: kubeClient,
	}
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(controllerName))

	// 1. Has the provisioner been deleted?
	provisioner := v1alpha4.Provisioner{}
	if err := c.KubeClient.Get(ctx, req.NamespacedName, &provisioner); err != nil {
		if !errors.IsNotFound(err) {
			// Unable to determine existence of the provisioner, try again later.
			return reconcile.Result{Requeue: true}, err
		}

		// The provisioner has been deleted. Reset all the associated counts to zero.
		if err := publishPodCounts(provisioner.Name, map[string][]v1.Pod{}, []*v1.Pod{}); err != nil {
			// One or more metrics were not zeroed. Try again later.
			return reconcile.Result{Requeue: true}, err
		}

		// Since the provisioner is gone, do not requeue.
		return reconcile.Result{}, nil
	}

	// 2. Update pod counts associated with the provisioner.
	podsByZone, err := c.podsByZone(ctx, &provisioner)
	if err != nil {
		return reconcile.Result{Requeue: true}, err
	}
	provisionablePods, err := c.Filter.GetProvisionablePods(ctx, &provisioner)
	if err != nil {
		return reconcile.Result{Requeue: true}, err
	}
	if err := publishPodCounts(provisioner.Name, podsByZone, provisionablePods); err != nil {
		// An updated value for one or more metrics was not published. Try again later.
		return reconcile.Result{Requeue: true}, err
	}

	// 3. Schedule the next run.
	return reconcile.Result{RequeueAfter: requeueInterval}, nil
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(controllerName).
		For(&v1alpha4.Provisioner{}, builder.WithPredicates(
			predicate.Funcs{
				CreateFunc:  func(_ event.CreateEvent) bool { return true },
				DeleteFunc:  func(_ event.DeleteEvent) bool { return true },
				UpdateFunc:  func(_ event.UpdateEvent) bool { return false },
				GenericFunc: func(_ event.GenericEvent) bool { return false },
			},
		)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(c)
}

// podsByZone returns a map of slices containing all pods scheduled to nodes in each zone.
func (c *Controller) podsByZone(ctx context.Context, provisioner *v1alpha4.Provisioner) (map[string][]v1.Pod, error) {
	// Karpenter does not apply a label, or other marker, to pods.

	results := map[string][]v1.Pod{}

	// 1. Fetch all nodes associated with the provisioner -- since a label is applied
	//    to the node.
	nodeList := v1.NodeList{}
	withProvisionerName := client.MatchingLabels{v1alpha4.ProvisionerNameLabelKey: provisioner.Name}
	if err := c.KubeClient.List(ctx, &nodeList, withProvisionerName); err != nil {
		return map[string][]v1.Pod{}, err
	}

	// 2. Get all the pods scheduled to each node and append them to the slice
	//    associated with the node's zone.
	for _, node := range nodeList.Items {
		zone := node.Labels[v1.LabelTopologyZone]
		if zone == "" {
			return map[string][]v1.Pod{}, fmt.Errorf(
				"Node %q provisioned by %q is missing label %s",
				node.Name, provisioner.Name, v1.LabelTopologyZone,
			)
		}

		podList := v1.PodList{}
		withNodeName := client.MatchingFields{"spec.nodeName": node.Name}
		if err := c.KubeClient.List(ctx, &podList, withNodeName); err != nil {
			return map[string][]v1.Pod{}, err
		}

		results[zone] = append(results[zone], podList.Items...)
	}

	return results, nil
}
