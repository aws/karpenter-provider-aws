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

package allocation

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/packing"
	"golang.org/x/time/rate"
	"knative.dev/pkg/logging"

	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	maxBatchWindow   = 10 * time.Second
	batchIdleTimeout = 2 * time.Second
)

// Controller for the resource
type Controller struct {
	Batcher       *Batcher
	Filter        *Filter
	Binder        *Binder
	Constraints   *Constraints
	Packer        packing.Packer
	CloudProvider cloudprovider.CloudProvider
	KubeClient    client.Client
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client, coreV1Client corev1.CoreV1Interface, cloudProvider cloudprovider.CloudProvider) *Controller {
	return &Controller{
		Filter:        &Filter{KubeClient: kubeClient},
		Binder:        &Binder{KubeClient: kubeClient, CoreV1Client: coreV1Client},
		Batcher:       NewBatcher(maxBatchWindow, batchIdleTimeout),
		Constraints:   &Constraints{KubeClient: kubeClient},
		Packer:        packing.NewPacker(),
		CloudProvider: cloudProvider,
		KubeClient:    kubeClient,
	}
}

// Reconcile executes an allocation control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("Allocation"))

	// 1. Fetch provisioner
	provisioner, err := c.provisionerFor(ctx, req.NamespacedName)
	if err != nil {
		if errors.IsNotFound(err) {
			c.Batcher.Wait(&v1alpha3.Provisioner{})
			logging.FromContext(ctx).Errorf("Provisioner \"%s\" not found. Create the \"default\" provisioner or specify an alternative using the nodeSelector %s", req.Name, v1alpha3.ProvisionerNameLabelKey)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// 2. Wait on a pod batch
	c.Batcher.Wait(provisioner)

	// 3. Filter pods
	pods, err := c.Filter.GetProvisionablePods(ctx, provisioner)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("filtering pods, %w", err)
	}
	if len(pods) == 0 {
		return reconcile.Result{}, nil
	}
	logging.FromContext(ctx).Infof("Found %d provisionable pods", len(pods))

	// 4. Group by constraints
	constraintGroups, err := c.Constraints.Group(ctx, provisioner, pods)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("building constraint groups, %w", err)
	}

	// 5. Get Instance Types Options
	instanceTypes, err := c.CloudProvider.GetInstanceTypes(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("getting instance types, %w", err)
	}

	// 6. Binpack each group
	packings := []*cloudprovider.Packing{}
	for _, constraintGroup := range constraintGroups {
		packings = append(packings, c.Packer.Pack(ctx, constraintGroup, instanceTypes)...)
	}

	// 7. Create capacity
	errs := make([]error, len(packings))
	workqueue.ParallelizeUntil(ctx, len(packings), len(packings), func(index int) {
		packing := packings[index]
		errs[index] = <-c.CloudProvider.Create(ctx, provisioner, packing, func(node *v1.Node) error {
			node.Labels = packing.Constraints.Labels
			node.Spec.Taints = packing.Constraints.Taints
			return c.Binder.Bind(ctx, node, packing.Pods)
		})
	})
	return reconcile.Result{}, multierr.Combine(errs...)
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	err := controllerruntime.
		NewControllerManagedBy(m).
		Named("Allocation").
		For(&v1alpha3.Provisioner{}).
		Watches(
			&source.Kind{Type: &v1.Pod{}},
			handler.EnqueueRequestsFromMapFunc(c.podToProvisioner),
			// Only process pod update events
			builder.WithPredicates(
				predicate.Funcs{
					CreateFunc:  func(_ event.CreateEvent) bool { return false },
					DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
					GenericFunc: func(_ event.GenericEvent) bool { return false },
				},
			),
		).
		WithOptions(
			controller.Options{
				RateLimiter: workqueue.NewMaxOfRateLimiter(
					workqueue.NewItemExponentialFailureRateLimiter(100*time.Millisecond, 10*time.Second),
					// 10 qps, 100 bucket size
					&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
				),
				MaxConcurrentReconciles: 4,
			},
		).
		Complete(c)
	c.Batcher.Start(ctx)
	return err
}

// provisionerFor fetches the provisioner and returns a provisioner w/ default runtime values
func (c *Controller) provisionerFor(ctx context.Context, name types.NamespacedName) (*v1alpha3.Provisioner, error) {
	provisioner := &v1alpha3.Provisioner{}
	if err := c.KubeClient.Get(ctx, name, provisioner); err != nil {
		return nil, err
	}

	// Hydrate provisioner with (dynamic) default values, which must not
	//    be persisted into the original CRD as they might change with each reconciliation
	//    loop iteration.
	defaulted, err := provisioner.WithDynamicDefaults(ctx)
	if err != nil {
		return &defaulted, fmt.Errorf("setting dynamic default values, %w", err)
	}
	return &defaulted, nil
}

// podToProvisioner is a function handler to transform pod objs to provisioner reconcile requests
func (c *Controller) podToProvisioner(o client.Object) (requests []reconcile.Request) {
	pod := o.(*v1.Pod)
	ctx := context.Background()
	if err := c.Filter.isUnschedulable(pod); err != nil {
		return nil
	}
	provisionerKey := v1alpha3.DefaultProvisioner
	if name, ok := pod.Spec.NodeSelector[v1alpha3.ProvisionerNameLabelKey]; ok {
		provisionerKey.Name = name
	}
	provisioner, err := c.provisionerFor(ctx, provisionerKey)
	if err != nil {
		if errors.IsNotFound(err) {
			// Queue and batch a reconcile request for a non-existent, empty provisioner
			// This will reduce the number of repeated error messages about a provisioner not existing
			c.Batcher.Add(&v1alpha3.Provisioner{})
			notFoundProvisioner := v1alpha3.DefaultProvisioner.Name
			if name, ok := pod.Spec.NodeSelector[v1alpha3.ProvisionerNameLabelKey]; ok {
				notFoundProvisioner = name
			}
			return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: notFoundProvisioner}}}
		}
		return nil
	}
	if err = c.Filter.isProvisionable(ctx, pod, provisioner); err != nil {
		return nil
	}
	c.Batcher.Add(provisioner)
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: provisioner.Name}}}
}
