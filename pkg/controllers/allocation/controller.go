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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha2"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/packing"
	"github.com/awslabs/karpenter/pkg/utils/apiobject"
	"golang.org/x/time/rate"

	"go.uber.org/multierr"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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
	persistedProvisioner := &v1alpha2.Provisioner{}
	// 1. Fetch Provisioner
	if err := c.KubeClient.Get(ctx, req.NamespacedName, persistedProvisioner); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// 2. Hydrate provisioner with (dynamic) default values, which must not
	//    be persisted into the original CRD as they might change with each reconciliation
	//    loop iteration.
	provisionerWithDefaults, err := persistedProvisioner.WithDynamicDefaults()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("setting dynamic default values, %w", err)
	}

	// 3. Wait on a pod batch
	c.Batcher.Wait(&provisionerWithDefaults)

	// 4. Filter pods
	pods, err := c.Filter.GetProvisionablePods(ctx, &provisionerWithDefaults)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("filtering pods, %w", err)
	}
	if len(pods) == 0 {
		return reconcile.Result{}, nil
	}
	zap.S().Infof("Found %d provisionable pods", len(pods))

	// 5. Group by constraints
	constraintGroups, err := c.Constraints.Group(ctx, &provisionerWithDefaults, pods)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("building constraint groups, %w", err)
	}

	// 6. Binpack each group
	packings := []*cloudprovider.Packing{}
	for _, constraintGroup := range constraintGroups {
		instanceTypes, err := c.CloudProvider.GetInstanceTypes(ctx)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("getting instance types, %w", err)
		}
		packings = append(packings, c.Packer.Pack(ctx, constraintGroup, instanceTypes)...)
	}

	// 7. Create packedNodes for packings and also copy all Status changes made by the
	//    cloud provider to the original provisioner instance.
	packedNodes, err := c.CloudProvider.Create(ctx, &provisionerWithDefaults, packings)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("creating capacity, %w", err)
	}

	// 8. Bind pods to nodes
	var errs error
	for _, packedNode := range packedNodes {
		zap.S().Infof("Binding pods %v to node %s", apiobject.PodNamespacedNames(packedNode.Pods), packedNode.Node.Name)
		if err := c.Binder.Bind(ctx, packedNode.Node, packedNode.Pods); err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	return reconcile.Result{}, errs
}

func (c *Controller) podToProvisioner(o client.Object) (requests []reconcile.Request) {
	pod := o.(*v1.Pod)
	ctx := context.Background()
	provisioner, err := c.GetProvisionerFor(ctx, pod)
	if err != nil {
		zap.S().Errorf("Retrieving provisioner, %s", err.Error())
		return nil
	}
	if err = c.Filter.isProvisionable(ctx, pod, provisioner); err != nil {
		return nil
	}
	c.Batcher.Add(provisioner)
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: provisioner.Name, Namespace: provisioner.Namespace}}}
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	err := controllerruntime.
		NewControllerManagedBy(m).
		Named("Allocation").
		For(&v1alpha2.Provisioner{}).
		Watches(
			&source.Kind{Type: &v1.Pod{}},
			handler.EnqueueRequestsFromMapFunc(c.podToProvisioner),
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

// GetProvisionerFor retrieves the provisioner responsible for the pod
func (c *Controller) GetProvisionerFor(ctx context.Context, p *v1.Pod) (*v1alpha2.Provisioner, error) {
	provisionerKey := client.ObjectKey{Namespace: "default", Name: "default"}
	if name, ok := p.Spec.NodeSelector[v1alpha2.ProvisionerNameLabelKey]; ok {
		provisionerKey.Name = name
	}
	if namespace, ok := p.Spec.NodeSelector[v1alpha2.ProvisionerNamespaceLabelKey]; ok {
		provisionerKey.Namespace = namespace
	}
	provisioner := &v1alpha2.Provisioner{}
	if err := c.KubeClient.Get(ctx, provisionerKey, provisioner); err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("create a default provisioner, or specify an alternative using the nodeSelector %s", v1alpha2.ProvisionerNameLabelKey)
		}
		return nil, err
	}
	return provisioner, nil
}
