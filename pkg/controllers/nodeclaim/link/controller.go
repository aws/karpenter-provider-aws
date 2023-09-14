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

package link

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/apis/v1beta1"
	corecloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/metrics"
	"github.com/aws/karpenter-core/pkg/operator/controller"
	machineutil "github.com/aws/karpenter-core/pkg/utils/machine"
	nodeclaimutil "github.com/aws/karpenter-core/pkg/utils/nodeclaim"
	"github.com/aws/karpenter/pkg/cloudprovider"
)

const creationReasonLabel = "linking"

type Controller struct {
	kubeClient    client.Client
	cloudProvider *cloudprovider.CloudProvider
	Cache         *cache.Cache // exists due to eventual consistency on the controller-runtime cache
}

func NewController(kubeClient client.Client, cloudProvider *cloudprovider.CloudProvider) *Controller {
	return &Controller{
		kubeClient:    kubeClient,
		cloudProvider: cloudProvider,
		Cache:         cache.New(time.Minute, time.Second*10),
	}
}

func (c *Controller) Name() string {
	return "machine.link"
}

func (c *Controller) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	// We LIST machines on the CloudProvider BEFORE we grab Machines/Nodes on the cluster so that we make sure that, if
	// LISTing instances takes a long time, our information is more updated by the time we get to Machine and Node LIST
	retrieved, err := c.cloudProvider.List(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("listing cloudprovider machines, %w", err)
	}
	machineList := &v1alpha5.MachineList{}
	if err := c.kubeClient.List(ctx, machineList); err != nil {
		return reconcile.Result{}, err
	}
	nodeList := &v1.NodeList{}
	if err := c.kubeClient.List(ctx, nodeList, client.HasLabels{v1alpha5.ProvisionerNameLabelKey}); err != nil {
		return reconcile.Result{}, err
	}
	retrievedIDs := sets.NewString(lo.Map(retrieved, func(nc *v1beta1.NodeClaim, _ int) string { return nc.Status.ProviderID })...)
	// Inject any nodes that are re-owned using karpenter.sh/provisioner-name but aren't found from the cloudprovider.List() call
	for i := range nodeList.Items {
		if _, ok := lo.Find(retrieved, func(r *v1beta1.NodeClaim) bool {
			return retrievedIDs.Has(nodeList.Items[i].Spec.ProviderID)
		}); !ok {
			retrieved = append(retrieved, nodeclaimutil.NewFromNode(&nodeList.Items[i]))
		}
	}
	// Filter out any machines that shouldn't be linked
	retrieved = lo.Filter(retrieved, func(nc *v1beta1.NodeClaim, _ int) bool {
		_, ok := nc.Annotations[v1alpha5.MachineManagedByAnnotationKey]
		return !ok && nc.DeletionTimestamp.IsZero() && nc.Labels[v1alpha5.ProvisionerNameLabelKey] != ""
	})
	errs := make([]error, len(retrieved))
	workqueue.ParallelizeUntil(ctx, 100, len(retrieved), func(i int) {
		errs[i] = c.link(ctx, retrieved[i], machineList.Items)
	})
	// Effectively, don't requeue this again once it succeeds
	return reconcile.Result{RequeueAfter: math.MaxInt64}, multierr.Combine(errs...)
}

func (c *Controller) link(ctx context.Context, retrieved *v1beta1.NodeClaim, existingMachines []v1alpha5.Machine) error {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("provider-id", retrieved.Status.ProviderID, "provisioner", retrieved.Labels[v1alpha5.ProvisionerNameLabelKey]))
	provisioner := &v1alpha5.Provisioner{}
	if err := c.kubeClient.Get(ctx, types.NamespacedName{Name: retrieved.Labels[v1alpha5.ProvisionerNameLabelKey]}, provisioner); err != nil {
		return client.IgnoreNotFound(err)
	}
	if c.shouldCreateLinkedMachine(retrieved, existingMachines) {
		machine := machineutil.New(&v1.Node{}, provisioner)
		machine.GenerateName = fmt.Sprintf("%s-", provisioner.Name)
		// This annotation communicates to the machine controller that this is a machine linking scenario, not
		// a case where we want to provision a new machine
		machine.Annotations = lo.Assign(machine.Annotations, map[string]string{
			v1alpha5.MachineLinkedAnnotationKey: retrieved.Status.ProviderID,
		})
		if err := c.kubeClient.Create(ctx, machine); err != nil {
			return err
		}
		logging.FromContext(ctx).With("machine", machine.Name).Debugf("generated cluster machine from cloudprovider")
		metrics.MachinesCreatedCounter.With(prometheus.Labels{
			metrics.ReasonLabel:      creationReasonLabel,
			metrics.ProvisionerLabel: machine.Labels[v1alpha5.ProvisionerNameLabelKey],
		}).Inc()
		c.Cache.SetDefault(retrieved.Status.ProviderID, nil)
	}
	return corecloudprovider.IgnoreNodeClaimNotFoundError(c.cloudProvider.Link(ctx, retrieved))
}

func (c *Controller) shouldCreateLinkedMachine(retrieved *v1beta1.NodeClaim, existingMachines []v1alpha5.Machine) bool {
	// Machine was already created but controller-runtime cache didn't update
	if _, ok := c.Cache.Get(retrieved.Status.ProviderID); ok {
		return false
	}
	// We have a machine registered for this, so no need to hydrate it
	if _, ok := lo.Find(existingMachines, func(m v1alpha5.Machine) bool {
		return m.Annotations[v1alpha5.MachineLinkedAnnotationKey] == retrieved.Status.ProviderID ||
			m.Status.ProviderID == retrieved.Status.ProviderID
	}); ok {
		return false
	}
	return true
}

func (c *Controller) Builder(_ context.Context, m manager.Manager) controller.Builder {
	return controller.NewSingletonManagedBy(m)
}
