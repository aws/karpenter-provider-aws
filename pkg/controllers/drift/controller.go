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

package drift

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/samber/lo"

	"github.com/aws/karpenter-core/pkg/apis/config/settings"
	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corecloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	machineutil "github.com/aws/karpenter-core/pkg/utils/machine"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
)

var _ corecontroller.TypedController[*v1.Node] = (*Controller)(nil)

type Controller struct {
	kubeClient    client.Client
	cloudProvider corecloudprovider.CloudProvider
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, cloudProvider corecloudprovider.CloudProvider) corecontroller.Controller {
	return corecontroller.Typed[*v1.Node](kubeClient, &Controller{
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
	})
}

func (c *Controller) Name() string {
	return "drift"
}

func (c *Controller) Reconcile(ctx context.Context, node *v1.Node) (reconcile.Result, error) {
	if !settings.FromContext(ctx).DriftEnabled {
		return reconcile.Result{}, nil
	}
	provisionerName, provisionerExists := node.Labels[v1alpha5.ProvisionerNameLabelKey]
	if !provisionerExists {
		return reconcile.Result{}, nil
	}

	if node.Annotations[v1alpha5.VoluntaryDisruptionAnnotationKey] == v1alpha5.VoluntaryDisruptionDriftedAnnotationValue {
		return reconcile.Result{}, nil
	}

	provisioner := &v1alpha5.Provisioner{}
	err := c.kubeClient.Get(ctx, types.NamespacedName{Name: provisionerName}, provisioner)
	if errors.IsNotFound(err) {
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("getting provisioner, %w", err)
	}

	drifted, err := c.cloudProvider.IsMachineDrifted(ctx, machineutil.NewFromNode(node))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("getting drift for node, %w", err)
	}
	if drifted {
		node.Annotations = lo.Assign(node.Annotations, map[string]string{
			v1alpha5.VoluntaryDisruptionAnnotationKey: v1alpha5.VoluntaryDisruptionDriftedAnnotationValue,
		})
	}
	// Requeue after 5 minutes for the cache TTL
	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (c *Controller) Builder(ctx context.Context, m manager.Manager) corecontroller.Builder {
	return corecontroller.Adapt(controllerruntime.
		NewControllerManagedBy(m).
		For(&v1.Node{}).
		Watches(
			&source.Kind{Type: &v1alpha5.Provisioner{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) (requests []reconcile.Request) {
				provisioner := o.(*v1alpha5.Provisioner)
				return getReconcileRequests(ctx, provisioner, c.kubeClient)
			})).
		Watches(
			&source.Kind{Type: &v1alpha1.AWSNodeTemplate{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) (requests []reconcile.Request) {
				provisioners := &v1alpha5.ProvisionerList{}
				if err := c.kubeClient.List(ctx, provisioners, client.MatchingFields{".spec.providerRef.name": o.GetName()}); err != nil {
					logging.FromContext(ctx).Errorf("listing provisioners for AWSNodeTemplate reconciliation %w", err)
					return requests
				}
				for i := range provisioners.Items {
					requests = append(requests, getReconcileRequests(ctx, &provisioners.Items[i], c.kubeClient)...)
				}
				return requests
			}),
		).
		WithEventFilter(predicate.NewPredicateFuncs(func(o client.Object) bool {
			_, ok := o.GetLabels()[v1alpha5.ProvisionerNameLabelKey]
			return ok
		})).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}))
}

func getReconcileRequests(ctx context.Context, provisioner *v1alpha5.Provisioner, kubeClient client.Client) (requests []reconcile.Request) {
	nodes := &v1.NodeList{}
	if err := kubeClient.List(ctx, nodes, client.MatchingLabels(map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner.Name})); err != nil {
		logging.FromContext(ctx).Errorf("listing nodes when mapping drift watch events, %s", err)
		return requests
	}
	for _, node := range nodes.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: node.Name}})
	}
	return requests
}
