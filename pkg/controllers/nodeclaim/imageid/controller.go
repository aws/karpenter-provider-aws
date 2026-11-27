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

package imageid

import (
	"context"
	"time"

	"github.com/awslabs/operatorpkg/reasonable"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	"sigs.k8s.io/karpenter/pkg/utils/nodeclaim"

	"github.com/aws/karpenter-provider-aws/pkg/providers/rootvolume"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

const pollInterval = 30 * time.Second

// Controller keeps a NodeClaim's status.imageID on the AMI its instance runs after a root volume replacement.
// Core never reads the AMI back after a reboot, and the drift check compares status.imageID with the EC2NodeClass.
type Controller struct {
	kubeClient         client.Client
	cloudProvider      cloudprovider.CloudProvider
	rootVolumeProvider rootvolume.Provider
}

func NewController(kubeClient client.Client, cloudProvider cloudprovider.CloudProvider, rootVolumeProvider rootvolume.Provider) *Controller {
	return &Controller{
		kubeClient:         kubeClient,
		cloudProvider:      cloudProvider,
		rootVolumeProvider: rootVolumeProvider,
	}
}

func (c *Controller) Reconcile(ctx context.Context, nodeClaim *karpv1.NodeClaim) (reconcile.Result, error) {
	ctx = injection.WithControllerName(ctx, "nodeclaim.imageid")
	if !hasRebooted(nodeClaim) {
		return reconcile.Result{}, nil
	}
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("Node", klog.KRef("", nodeClaim.Status.NodeName), "provider-id", nodeClaim.Status.ProviderID))
	id, err := utils.ParseInstanceID(nodeClaim.Status.ProviderID)
	if err != nil {
		// We don't throw an error here since we don't want to retry until the ProviderID has been updated.
		log.FromContext(ctx).Error(err, "failed parsing instance id")
		return reconcile.Result{}, nil
	}
	task, err := c.rootVolumeProvider.SyncLatestTask(ctx, id)
	if err != nil {
		return reconcile.Result{}, err
	}
	if task == nil {
		return reconcile.Result{}, nil
	}
	if !task.Finished() {
		return reconcile.Result{RequeueAfter: pollInterval}, nil
	}
	if !task.Succeeded() || task.ImageID == "" || task.ImageID == nodeClaim.Status.ImageID {
		return reconcile.Result{}, nil
	}
	stored := nodeClaim.DeepCopy()
	nodeClaim.Status.ImageID = task.ImageID
	if err := c.kubeClient.Status().Patch(ctx, nodeClaim, client.MergeFrom(stored)); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	log.FromContext(ctx).WithValues("image-id", task.ImageID, "task-id", task.ID).Info("updated image id after root volume replacement")
	return reconcile.Result{}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("nodeclaim.imageid").
		For(&karpv1.NodeClaim{}, builder.WithPredicates(nodeclaim.IsManagedPredicateFuncs(c.cloudProvider))).
		WithEventFilter(predicate.NewPredicateFuncs(func(o client.Object) bool {
			return hasRebooted(o.(*karpv1.NodeClaim))
		})).
		WithOptions(controller.Options{
			RateLimiter: reasonable.RateLimiter(),
		}).
		Complete(reconcile.AsReconciler(m.GetClient(), c))
}

// hasRebooted selects NodeClaims that are rebooting or have rebooted, since only a reboot replaces a root volume.
func hasRebooted(nodeClaim *karpv1.NodeClaim) bool {
	return nodeClaim.DeletionTimestamp.IsZero() && nodeClaim.Status.ProviderID != "" &&
		nodeClaim.StatusConditions().Get(karpv1.ConditionTypeRebooting) != nil
}
