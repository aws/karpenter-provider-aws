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

package tagging

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/utils"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	corecontroller "sigs.k8s.io/karpenter/pkg/operator/controller"
)

const (
	TagNodeClaim = corev1beta1.Group + "/nodeclaim"
	TagName      = "Name"
)

type Controller struct {
	kubeClient       client.Client
	instanceProvider *instance.Provider
}

func NewController(kubeClient client.Client, instanceProvider *instance.Provider) corecontroller.Controller {
	return corecontroller.Typed[*corev1beta1.NodeClaim](kubeClient, &Controller{
		kubeClient:       kubeClient,
		instanceProvider: instanceProvider,
	})
}

func (c *Controller) Name() string {
	return "nodeclaim.tagging"
}

func (c *Controller) Reconcile(ctx context.Context, nodeClaim *corev1beta1.NodeClaim) (reconcile.Result, error) {
	stored := nodeClaim.DeepCopy()
	if !isTaggable(nodeClaim) {
		return reconcile.Result{}, nil
	}
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With("provider-id", nodeClaim.Status.ProviderID))
	id, err := utils.ParseInstanceID(nodeClaim.Status.ProviderID)
	if err != nil {
		// We don't throw an error here since we don't want to retry until the ProviderID has been updated.
		logging.FromContext(ctx).Errorf("failed to parse instance ID, %w", err)
		return reconcile.Result{}, nil
	}
	if err = c.tagInstance(ctx, nodeClaim, id); err != nil {
		return reconcile.Result{}, cloudprovider.IgnoreNodeClaimNotFoundError(err)
	}
	nodeClaim.Annotations = lo.Assign(nodeClaim.Annotations, map[string]string{v1beta1.AnnotationInstanceTagged: "true"})
	if !equality.Semantic.DeepEqual(nodeClaim, stored) {
		if err := c.kubeClient.Patch(ctx, nodeClaim, client.MergeFrom(stored)); err != nil {
			return reconcile.Result{}, client.IgnoreNotFound(err)
		}
	}
	return reconcile.Result{}, nil
}

func (c *Controller) Builder(_ context.Context, m manager.Manager) corecontroller.Builder {
	return corecontroller.Adapt(
		controllerruntime.
			NewControllerManagedBy(m).
			For(&corev1beta1.NodeClaim{}).
			WithEventFilter(predicate.NewPredicateFuncs(func(o client.Object) bool {
				return isTaggable(o.(*corev1beta1.NodeClaim))
			})),
	)
}

func (c *Controller) tagInstance(ctx context.Context, nc *corev1beta1.NodeClaim, id string) error {
	tags := map[string]string{
		TagName:      nc.Status.NodeName,
		TagNodeClaim: nc.Name,
	}

	// Remove tags which have been already populated
	instance, err := c.instanceProvider.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("tagging nodeclaim, %w", err)
	}
	tags = lo.OmitByKeys(tags, lo.Keys(instance.Tags))
	if len(tags) == 0 {
		return nil
	}

	// Ensures that no more than 1 CreateTags call is made per second. Rate limiting is required since CreateTags
	// shares a pool with other mutating calls (e.g. CreateFleet).
	defer time.Sleep(time.Second)
	if err := c.instanceProvider.CreateTags(ctx, id, tags); err != nil {
		return fmt.Errorf("tagging nodeclaim, %w", err)
	}
	return nil
}

func isTaggable(nc *corev1beta1.NodeClaim) bool {
	// Instance has already been tagged
	if val := nc.Annotations[v1beta1.AnnotationInstanceTagged]; val == "true" {
		return false
	}
	// Node name is not yet known
	if nc.Status.NodeName == "" {
		return false
	}
	// NodeClaim is currently terminating
	if !nc.DeletionTimestamp.IsZero() {
		return false
	}
	return true
}
