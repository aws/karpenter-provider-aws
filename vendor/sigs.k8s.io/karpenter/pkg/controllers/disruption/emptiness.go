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
	"context"
	"errors"
	"fmt"

	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	disruptionevents "sigs.k8s.io/karpenter/pkg/controllers/disruption/events"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning/scheduling"
)

// Emptiness is a subreconciler that deletes empty candidates.
type Emptiness struct {
	consolidation
}

func NewEmptiness(c consolidation) *Emptiness {
	return &Emptiness{
		consolidation: c,
	}
}

// ShouldDisrupt is a predicate used to filter candidates
func (e *Emptiness) ShouldDisrupt(_ context.Context, c *Candidate) bool {
	// If consolidation is disabled, don't do anything. This emptiness should run for both WhenEmpty and WhenEmptyOrUnderutilized
	if c.nodePool.Spec.Disruption.ConsolidateAfter.Duration == nil {
		e.recorder.Publish(disruptionevents.Unconsolidatable(c.Node, c.NodeClaim, fmt.Sprintf("NodePool %q has consolidation disabled", c.nodePool.Name))...)
		return false
	}
	// return true if there are no pods and the nodeclaim is consolidatable
	return len(c.reschedulablePods) == 0 && c.NodeClaim.StatusConditions().Get(v1.ConditionTypeConsolidatable).IsTrue()
}

// ComputeCommand generates a disruption command given candidates
//
//nolint:gocyclo
func (e *Emptiness) ComputeCommand(ctx context.Context, disruptionBudgetMapping map[string]int, candidates ...*Candidate) (Command, scheduling.Results, error) {
	if e.IsConsolidated() {
		return Command{}, scheduling.Results{}, nil
	}
	candidates = e.sortCandidates(candidates)

	empty := make([]*Candidate, 0, len(candidates))
	constrainedByBudgets := false
	for _, candidate := range candidates {
		if len(candidate.reschedulablePods) > 0 {
			continue
		}
		if disruptionBudgetMapping[candidate.nodePool.Name] == 0 {
			// set constrainedByBudgets to true if any node was a candidate but was constrained by a budget
			constrainedByBudgets = true
			continue
		}
		// If there's disruptions allowed for the candidate's nodepool,
		// add it to the list of candidates, and decrement the budget.
		empty = append(empty, candidate)
		disruptionBudgetMapping[candidate.nodePool.Name]--
	}
	// none empty, so do nothing
	if len(empty) == 0 {
		// if there are no candidates, but a nodepool had a fully blocking budget,
		// don't mark the cluster as consolidated, as it's possible this nodepool
		// should be consolidated the next time we try to disrupt.
		if !constrainedByBudgets {
			e.markConsolidated()
		}
		return Command{}, scheduling.Results{}, nil
	}

	cmd := Command{
		candidates: empty,
	}

	// Empty Node Consolidation doesn't use Validation as we get to take advantage of cluster.IsNodeNominated.  This
	// lets us avoid a scheduling simulation (which is performed periodically while pending pods exist and drives
	// cluster.IsNodeNominated already).
	select {
	case <-ctx.Done():
		return Command{}, scheduling.Results{}, errors.New("interrupted")
	case <-e.clock.After(consolidationTTL):
	}

	v := NewValidation(e.clock, e.cluster, e.kubeClient, e.provisioner, e.cloudProvider, e.recorder, e.queue, e.Reason())
	validatedCandidates, err := v.ValidateCandidates(ctx, cmd.candidates...)
	if err != nil {
		if IsValidationError(err) {
			log.FromContext(ctx).V(1).Info(fmt.Sprintf("abandoning empty node consolidation attempt due to pod churn, command is no longer valid, %s", cmd))
			return Command{}, scheduling.Results{}, nil
		}
		return Command{}, scheduling.Results{}, err
	}

	// TODO (jmdeal@): better encapsulate within validation
	if lo.ContainsBy(validatedCandidates, func(c *Candidate) bool {
		return len(c.reschedulablePods) != 0
	}) {
		log.FromContext(ctx).V(1).Info(fmt.Sprintf("abandoning empty node consolidation attempt due to pod churn, command is no longer valid, %s", cmd))
		return Command{}, scheduling.Results{}, nil
	}

	return cmd, scheduling.Results{}, nil
}

func (e *Emptiness) Reason() v1.DisruptionReason {
	return v1.DisruptionReasonEmpty
}

func (e *Emptiness) Class() string {
	return GracefulDisruptionClass
}

func (e *Emptiness) ConsolidationType() string {
	return "empty"
}
