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

package interruption

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/awslabs/operatorpkg/reconciler"
	"github.com/awslabs/operatorpkg/singleton"
	"go.uber.org/multierr"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/operator/injection"

	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages"
	instancestatusmsg "github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages/instancestatus"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancestatus"
)

// unhealthyKey uniquely identifies an unhealthy status check for deduplication.
// The metric is only incremented the first time a given instance+category is observed.
type unhealthyKey struct {
	instanceID string
	category   string
}

var (
	// InstanceStatusInterval is the polling interval for the EC2 DescribeInstanceStatus API.
	InstanceStatusInterval = 1 * time.Minute
	// InstanceStatusDryRun controls whether the instance status controller takes action on
	// unhealthy instances. When true, the controller only emits metrics without cordoning
	// and draining affected nodes. Default is false (full remediation enabled).
	InstanceStatusDryRun = false
	// categoryToKind maps EC2 DescribeInstanceStatus categories to message kinds.
	categoryToKind = map[instancestatus.Category]messages.Kind{
		instancestatus.InstanceStatus: messages.InstanceStatusKind,
		instancestatus.SystemStatus:   messages.SystemStatusKind,
		instancestatus.EventStatus:    messages.EventStatusKind,
	}
)

// InstanceStatusController polls EC2 DescribeInstanceStatus to detect unhealthy instances
// and scheduled maintenance events, then cordons and drains affected nodes.
type InstanceStatusController struct {
	InterruptionHandler
	instanceStatusProvider instancestatus.Provider
	seen                   map[unhealthyKey]struct{}
	mu                     sync.Mutex
}

func NewInstanceStatusController(
	kubeClient client.Client,
	clk clock.Clock,
	cloudProvider cloudprovider.CloudProvider,
	recorder events.Recorder,
	instanceStatusProvider instancestatus.Provider,
) *InstanceStatusController {
	return &InstanceStatusController{
		InterruptionHandler: InterruptionHandler{
			kubeClient:    kubeClient,
			clk:           clk,
			cloudProvider: cloudProvider,
			recorder:      recorder,
		},
		instanceStatusProvider: instanceStatusProvider,
		seen:                   map[unhealthyKey]struct{}{},
	}
}

func (c *InstanceStatusController) Reconcile(ctx context.Context) (reconciler.Result, error) {
	ctx = injection.WithControllerName(ctx, "interruption.instancestatus")

	instanceStatuses, err := c.instanceStatusProvider.List(ctx)
	if err != nil {
		if awserrors.IsUnauthorizedOperationError(err) {
			log.FromContext(ctx).Error(err, "ec2:DescribeInstanceStatus permission is not allowed, update the IAM policy and restart the Karpenter deployment to enable instance status health checks")
			return reconciler.Result{}, nil
		}
		return reconciler.Result{}, fmt.Errorf("getting instance statuses, %w", err)
	}

	// Build the set of keys observed in this poll cycle for pruning stale entries.
	currentKeys := make(map[unhealthyKey]struct{})
	errs := make([]error, len(instanceStatuses))
	workqueue.ParallelizeUntil(ctx, 10, len(instanceStatuses), func(i int) {
		errs[i] = c.handleHealthStatus(ctx, instanceStatuses[i], currentKeys)
	})

	// Prune entries for instances that are no longer reported as unhealthy,
	// so that if the same instance becomes unhealthy again later it gets counted again.
	c.mu.Lock()
	for key := range c.seen {
		if _, ok := currentKeys[key]; !ok {
			delete(c.seen, key)
		}
	}
	c.mu.Unlock()

	if err = multierr.Combine(errs...); err != nil {
		return reconciler.Result{}, err
	}
	return reconciler.Result{RequeueAfter: InstanceStatusInterval}, nil
}

// handleHealthStatus dispatches a message per EC2 status category and records metrics.
func (c *InstanceStatusController) handleHealthStatus(ctx context.Context, hs instancestatus.HealthStatus, currentKeys map[unhealthyKey]struct{}) error {
	categories := make(map[instancestatus.Category]struct{})
	for _, d := range hs.Details {
		categories[d.Category] = struct{}{}
	}
	for category := range categories {
		kind, ok := categoryToKind[category]
		if !ok {
			continue
		}
		f, err := c.handleMessage(ctx, instancestatusmsg.New(hs.InstanceID, kind, hs.ImpairedSince), InstanceStatusDryRun)
		if err != nil {
			return fmt.Errorf("handling instance status check message, %w", err)
		}
		if f {
			c.recordUnhealthyInstance(ctx, hs.InstanceID, category, currentKeys)
		}
	}
	return nil
}

func (c *InstanceStatusController) recordUnhealthyInstance(ctx context.Context, instanceID string, category instancestatus.Category, currentKeys map[unhealthyKey]struct{}) {
	key := unhealthyKey{instanceID: instanceID, category: string(category)}
	c.mu.Lock()
	currentKeys[key] = struct{}{}
	_, already := c.seen[key]
	if !already {
		c.seen[key] = struct{}{}
	}
	c.mu.Unlock()
	if !already {
		log.FromContext(ctx).Info("detected unhealthy instance owned by cluster",
			"instanceID", instanceID,
			"category", string(category))
		InstanceStatusUnhealthy.Inc(map[string]string{categoryLabel: string(category)})
	}
}

func (c *InstanceStatusController) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("interruption.instancestatus").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}
