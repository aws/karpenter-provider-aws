/*
Copyright The Kubernetes Authors.

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

package disruption

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/awslabs/operatorpkg/option"
	"github.com/awslabs/operatorpkg/reconciler"
	"github.com/awslabs/operatorpkg/serrors"
	"github.com/awslabs/operatorpkg/singleton"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/metrics"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	nodepoolutils "sigs.k8s.io/karpenter/pkg/utils/nodepool"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"
)

type Controller struct {
	queue         *Queue
	kubeClient    client.Client
	cluster       *state.Cluster
	provisioner   *provisioning.Provisioner
	recorder      events.Recorder
	clock         clock.Clock
	cloudProvider cloudprovider.CloudProvider
	methods       []Method
	mu            sync.Mutex
	lastRun       map[string]time.Time
}

// pollingPeriod that we inspect cluster to look for opportunities to disrupt
const pollingPeriod = 10 * time.Second

type ControllerOptions struct {
	methods []Method
}

func WithMethods(methods ...Method) option.Function[ControllerOptions] {
	return func(o *ControllerOptions) {
		o.methods = methods
	}
}

func NewController(clk clock.Clock, kubeClient client.Client, provisioner *provisioning.Provisioner,
	cp cloudprovider.CloudProvider, recorder events.Recorder, cluster *state.Cluster, queue *Queue, opts ...option.Function[ControllerOptions]) *Controller {

	o := option.Resolve(append([]option.Function[ControllerOptions]{WithMethods(NewMethods(clk, cluster, kubeClient, provisioner, cp, recorder, queue)...)}, opts...)...)
	return &Controller{
		queue:         queue,
		clock:         clk,
		kubeClient:    kubeClient,
		cluster:       cluster,
		provisioner:   provisioner,
		recorder:      recorder,
		cloudProvider: cp,
		lastRun:       map[string]time.Time{},
		methods:       o.methods,
	}
}

func NewMethods(clk clock.Clock, cluster *state.Cluster, kubeClient client.Client, provisioner *provisioning.Provisioner, cp cloudprovider.CloudProvider, recorder events.Recorder, queue *Queue) []Method {
	c := MakeConsolidation(clk, cluster, kubeClient, provisioner, cp, recorder, queue)
	return []Method{
		// Delete any empty NodeClaims as there is zero cost in terms of disruption.
		NewEmptiness(c),
		// Terminate and create replacement for drifted NodeClaims in Static NodePool
		NewStaticDrift(cluster, provisioner, cp),
		// Terminate any NodeClaims that have drifted from provisioning specifications, allowing the pods to reschedule.
		NewDrift(kubeClient, cluster, provisioner, recorder),
		// Attempt to identify multiple NodeClaims that we can consolidate simultaneously to reduce pod churn
		NewMultiNodeConsolidation(c),
		// And finally fall back our single NodeClaim consolidation to further reduce cluster cost.
		NewSingleNodeConsolidation(c),
	}
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("disruption").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}

func (c *Controller) Reconcile(ctx context.Context) (reconciler.Result, error) {
	ctx = injection.WithControllerName(ctx, "disruption")

	// this won't catch if the reconciler loop hangs forever, but it will catch other issues
	c.logAbnormalRuns(ctx)
	defer c.logAbnormalRuns(ctx)
	c.recordRun("disruption-loop")

	// Log if there are any budgets that are misconfigured that weren't caught by validation.
	// Only validate the first reason, since CEL validation will catch invalid disruption reasons
	c.logInvalidBudgets(ctx)

	// We need to ensure that our internal cluster state mechanism is synced before we proceed
	// with making any scheduling decision off of our state nodes. Otherwise, we have the potential to make
	// a scheduling decision based on a smaller subset of nodes in our cluster state than actually exist.
	if !c.cluster.Synced(ctx) {
		return reconciler.Result{RequeueAfter: time.Second}, nil
	}

	// Karpenter taints nodes with a karpenter.sh/disruption taint as part of the disruption process while it progresses in memory.
	// If Karpenter restarts or fails with an error during a disruption action, some nodes can be left tainted.
	// Idempotently remove this taint from candidates that are not in the orchestration queue before continuing.
	outdatedNodes := lo.Reject(c.cluster.DeepCopyNodes(), func(s *state.StateNode, _ int) bool {
		return c.queue.HasAny(s.ProviderID()) || s.MarkedForDeletion()
	})
	if err := state.RequireNoScheduleTaint(ctx, c.kubeClient, false, outdatedNodes...); err != nil {
		if errors.IsConflict(err) {
			return reconciler.Result{Requeue: true}, nil
		}
		return reconciler.Result{}, serrors.Wrap(fmt.Errorf("removing taint from nodes, %w", err), "taint", pretty.Taint(v1.DisruptedNoScheduleTaint))
	}
	if err := state.ClearNodeClaimsCondition(ctx, c.kubeClient, v1.ConditionTypeDisruptionReason, outdatedNodes...); err != nil {
		if errors.IsConflict(err) {
			return reconciler.Result{Requeue: true}, nil
		}
		return reconciler.Result{}, serrors.Wrap(fmt.Errorf("removing condition from nodeclaims, %w", err), "condition", v1.ConditionTypeDisruptionReason)
	}

	// Attempt different disruption methods. We'll only let one method perform an action
	for _, m := range c.methods {
		c.recordRun(fmt.Sprintf("%T", m))
		success, err := c.disrupt(ctx, m)
		if err != nil {
			if errors.IsConflict(err) {
				return reconciler.Result{Requeue: true}, nil
			}
			return reconciler.Result{}, serrors.Wrap(fmt.Errorf("disrupting, %w", err), strings.ToLower(string(m.Reason())), "reason")
		}
		if success {
			return reconciler.Result{RequeueAfter: singleton.RequeueImmediately}, nil
		}
	}

	// All methods did nothing, so return nothing to do
	return reconciler.Result{RequeueAfter: pollingPeriod}, nil
}

func (c *Controller) disrupt(ctx context.Context, disruption Method) (bool, error) {
	defer metrics.Measure(EvaluationDurationSeconds, map[string]string{
		metrics.ReasonLabel:    strings.ToLower(string(disruption.Reason())),
		ConsolidationTypeLabel: disruption.ConsolidationType(),
	})()
	candidates, err := GetCandidates(ctx, c.cluster, c.kubeClient, c.recorder, c.clock, c.cloudProvider, disruption.ShouldDisrupt, disruption.Class(), c.queue)
	if err != nil {
		return false, fmt.Errorf("determining candidates, %w", err)
	}
	EligibleNodes.Set(float64(len(candidates)), map[string]string{
		metrics.ReasonLabel: strings.ToLower(string(disruption.Reason())),
	})

	// If there are no candidates, move to the next disruption
	if len(candidates) == 0 {
		return false, nil
	}
	disruptionBudgetMapping, err := BuildDisruptionBudgetMapping(ctx, c.cluster, c.clock, c.kubeClient, c.cloudProvider, c.recorder, disruption.Reason())
	if err != nil {
		return false, fmt.Errorf("building disruption budgets, %w", err)
	}
	// Determine the disruption action
	cmds, err := disruption.ComputeCommands(ctx, disruptionBudgetMapping, candidates...)
	if err != nil {
		return false, fmt.Errorf("computing disruption decision, %w", err)
	}
	cmds = lo.Filter(cmds, func(c Command, _ int) bool { return c.Decision() != NoOpDecision })
	if len(cmds) == 0 {
		return false, nil
	}

	errs := make([]error, len(cmds))
	workqueue.ParallelizeUntil(ctx, len(cmds), len(cmds), func(i int) {
		cmd := cmds[i]

		// Assign common fields
		cmd.CreationTimestamp = c.clock.Now()
		cmd.ID = uuid.New()
		cmd.Method = disruption

		// Attempt to disrupt
		if err := c.queue.StartCommand(ctx, &cmd); err != nil {
			errs[i] = fmt.Errorf("disrupting candidates, %w", err)
		}
	})
	if err = multierr.Combine(errs...); err != nil {
		return false, fmt.Errorf("disrupting candidates, %w", err)
	}
	return true, nil
}

func (c *Controller) recordRun(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastRun[s] = c.clock.Now()
}

func (c *Controller) logAbnormalRuns(ctx context.Context) {
	const AbnormalTimeLimit = 15 * time.Minute
	c.mu.Lock()
	defer c.mu.Unlock()
	for name, runTime := range c.lastRun {
		if timeSince := c.clock.Since(runTime); timeSince > AbnormalTimeLimit {
			log.FromContext(ctx).V(1).Info(fmt.Sprintf("abnormal time between runs of %s = %s", name, timeSince))
		}
	}
}

// logInvalidBudgets will log if there are any invalid schedules detected
func (c *Controller) logInvalidBudgets(ctx context.Context) {
	nps, err := nodepoolutils.ListManaged(ctx, c.kubeClient, c.cloudProvider)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed listing nodepools")
		return
	}
	var buf bytes.Buffer
	for _, np := range nps {
		// Use a dummy value of 100 since we only care if this errors.
		for _, method := range c.methods {
			if _, err := np.GetAllowedDisruptionsByReason(c.clock, 100, method.Reason()); err != nil {
				fmt.Fprintf(&buf, "invalid disruption budgets in nodepool %s, %s", np.Name, err)
				break // Prevent duplicate error message
			}
		}
	}
	if buf.Len() > 0 {
		log.FromContext(ctx).Error(stderrors.New(buf.String()), "detected disruption budget errors")
	}
}
