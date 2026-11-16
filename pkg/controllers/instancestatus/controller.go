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

package instancestatus

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/awslabs/operatorpkg/reconciler"
	"github.com/awslabs/operatorpkg/singleton"
	"github.com/awslabs/operatorpkg/status"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	nodeutils "sigs.k8s.io/karpenter/pkg/utils/node"
	nodeclaimutils "sigs.k8s.io/karpenter/pkg/utils/nodeclaim"

	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancestatus"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

const pollInterval = time.Minute

// Controller publishes EC2 instance and system status checks as the EC2StatusReady Node condition (False when a
// reachability check fails, True otherwise) so that Node Repair can replace impaired nodes. It is only registered when NodeRepair is enabled; otherwise the interruption
// instance status controller terminates impaired nodes directly.
type Controller struct {
	kubeClient    client.Client
	clock         clock.Clock
	cloudProvider cloudprovider.CloudProvider
	provider      instancestatus.Provider
}

func NewController(kubeClient client.Client, clk clock.Clock, cloudProvider cloudprovider.CloudProvider, provider instancestatus.Provider) *Controller {
	return &Controller{
		kubeClient:    kubeClient,
		clock:         clk,
		cloudProvider: cloudProvider,
		provider:      provider,
	}
}

//nolint:gocyclo
func (c *Controller) Reconcile(ctx context.Context) (reconciler.Result, error) {
	ctx = injection.WithControllerName(ctx, "node.instancestatus")

	statuses, err := c.provider.List(ctx, instancestatus.InstanceStatus, instancestatus.SystemStatus)
	if err != nil {
		if awserrors.IsUnauthorizedOperationError(err) {
			return reconciler.Result{}, fmt.Errorf("ec2:DescribeInstanceStatus permission is not allowed, update the IAM policy to enable instance status health checks, %w", err)
		}
		return reconciler.Result{}, fmt.Errorf("getting instance statuses, %w", err)
	}
	impaired := lo.SliceToMap(statuses, func(hs instancestatus.HealthStatus) (string, instancestatus.HealthStatus) {
		return hs.InstanceID, hs
	})
	nodeClaims, err := nodeclaimutils.ListManaged(ctx, c.kubeClient, c.cloudProvider, client.UnsafeDisableDeepCopy)
	if err != nil {
		return reconciler.Result{}, fmt.Errorf("listing nodeclaims, %w", err)
	}
	// Node labels can be written before the NodeClaim's Registered condition and can outlive the NodeClaim, so only
	// publish health for Nodes that belong to a registered NodeClaim. Status checks can fail while an instance reboots,
	// so rebooting NodeClaims are skipped. ObservedOnly avoids initializing missing conditions on objects shared with
	// the informer cache.
	nodeClaims = lo.Filter(nodeClaims, func(nodeClaim *karpv1.NodeClaim, _ int) bool {
		conditions := nodeClaim.StatusConditions(status.WithObservedOnly())
		return conditions.IsTrue(karpv1.ConditionTypeRegistered) && !conditions.IsTrue(karpv1.ConditionTypeRebooting)
	})

	// Stop dispatching patches once one is forbidden, since every remaining patch would fail the same way.
	patchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make([]error, len(nodeClaims))
	workqueue.ParallelizeUntil(patchCtx, 10, len(nodeClaims), func(i int) {
		node, err := nodeclaimutils.NodeForNodeClaim(ctx, c.kubeClient, nodeClaims[i])
		if err != nil {
			errs[i] = nodeclaimutils.IgnoreNodeNotFoundError(err)
			return
		}
		instanceID, err := utils.ParseInstanceID(nodeClaims[i].Status.ProviderID)
		if err != nil {
			errs[i] = err
			return
		}
		condition := corev1.NodeCondition{
			Type:               instancestatus.ConditionTypeEC2StatusReady,
			Status:             corev1.ConditionTrue,
			Reason:             instancestatus.ReasonNoImpairmentReported,
			Message:            "EC2 reports no recognized instance or system reachability impairment",
			LastTransitionTime: metav1.NewTime(c.clock.Now()),
		}
		if hs, ok := impaired[instanceID]; ok {
			condition.Status = corev1.ConditionFalse
			// A system failure takes precedence because it requires moving off the host, which also resolves an
			// instance failure.
			condition.Reason = lo.Ternary(lo.ContainsBy(hs.Details, func(d instancestatus.Details) bool { return d.Category == instancestatus.SystemStatus }),
				instancestatus.ReasonSystemReachabilityFailed, instancestatus.ReasonInstanceReachabilityFailed)
			condition.Message = fmt.Sprintf("EC2 reports failing reachability checks: %s", strings.Join(lo.Uniq(lo.Map(hs.Details, func(d instancestatus.Details, _ int) string { return string(d.Category) })), ", "))
			// Toleration is measured from when EC2 first reported the impairment, not from when Karpenter observed it.
			// EC2 may omit ImpairedSince, in which case the observation time is used.
			condition.LastTransitionTime = metav1.NewTime(lo.Ternary(hs.ImpairedSince.IsZero(), c.clock.Now(), hs.ImpairedSince))
		}
		current := nodeutils.GetCondition(node, condition.Type)
		if current.Status == condition.Status {
			condition.LastTransitionTime = current.LastTransitionTime
		}
		if equality.Semantic.DeepEqual(current, condition) {
			return
		}
		stored := node.DeepCopy()
		if _, idx, ok := lo.FindIndexOf(node.Status.Conditions, func(nodeCondition corev1.NodeCondition) bool { return nodeCondition.Type == condition.Type }); ok {
			node.Status.Conditions[idx] = condition
		} else {
			node.Status.Conditions = append(node.Status.Conditions, condition)
		}
		if err := c.kubeClient.Status().Patch(ctx, node, client.StrategicMergeFrom(stored)); client.IgnoreNotFound(err) != nil {
			errs[i] = fmt.Errorf("patching node %q status, %w", node.Name, err)
			if apierrors.IsForbidden(err) {
				cancel()
			}
		}
	})
	if err := multierr.Combine(errs...); err != nil {
		return reconciler.Result{}, err
	}
	return reconciler.Result{RequeueAfter: pollInterval}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("node.instancestatus").
		WatchesRawSource(singleton.Source()).
		// Retry failures at the poll interval. Faster retries add EC2 and API server load, and slower ones could leave a
		// stale EC2StatusReady=False past the repair toleration.
		WithOptions(controller.Options{RateLimiter: workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](pollInterval, pollInterval)}).
		Complete(singleton.AsReconciler(c))
}
