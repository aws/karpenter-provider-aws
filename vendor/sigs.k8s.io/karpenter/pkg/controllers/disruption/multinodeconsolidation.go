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
	"fmt"
	"math"
	"time"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning/scheduling"
	scheduler "sigs.k8s.io/karpenter/pkg/scheduling"
)

const MultiNodeConsolidationTimeoutDuration = 1 * time.Minute
const MultiNodeConsolidationType = "multi"

type MultiNodeConsolidation struct {
	consolidation
}

func NewMultiNodeConsolidation(consolidation consolidation) *MultiNodeConsolidation {
	return &MultiNodeConsolidation{consolidation: consolidation}
}

func (m *MultiNodeConsolidation) ComputeCommand(ctx context.Context, disruptionBudgetMapping map[string]int, candidates ...*Candidate) (Command, scheduling.Results, error) {
	if m.IsConsolidated() {
		return Command{}, scheduling.Results{}, nil
	}
	candidates = m.sortCandidates(candidates)

	// In order, filter out all candidates that would violate the budget.
	// Since multi-node consolidation relies on the ordering of
	// these candidates, and does computation in batches of these nodes by
	// simulateScheduling(nodes[0, n]), doing a binary search on n to find
	// the optimal consolidation command, this pre-filters out nodes that
	// would have violated the budget anyway, preserving the ordering
	// and only considering a number of nodes that can be disrupted.
	disruptableCandidates := make([]*Candidate, 0, len(candidates))
	constrainedByBudgets := false
	for _, candidate := range candidates {
		// If there's disruptions allowed for the candidate's nodepool,
		// add it to the list of candidates, and decrement the budget.
		if disruptionBudgetMapping[candidate.nodePool.Name] == 0 {
			constrainedByBudgets = true
			continue
		}
		// Filter out empty candidates. If there was an empty node that wasn't consolidated before this, we should
		// assume that it was due to budgets. If we don't filter out budgets, users who set a budget for `empty`
		// can find their nodes disrupted here.
		if len(candidate.reschedulablePods) == 0 {
			continue
		}
		// set constrainedByBudgets to true if any node was a candidate but was constrained by a budget
		disruptableCandidates = append(disruptableCandidates, candidate)
		disruptionBudgetMapping[candidate.nodePool.Name]--
	}

	// Only consider a maximum batch of 100 NodeClaims to save on computation.
	// This could be further configurable in the future.
	maxParallel := lo.Clamp(len(disruptableCandidates), 0, 100)

	cmd, results, err := m.firstNConsolidationOption(ctx, disruptableCandidates, maxParallel)
	if err != nil {
		return Command{}, scheduling.Results{}, err
	}

	if cmd.Decision() == NoOpDecision {
		// if there are no candidates because of a budget, don't mark
		// as consolidated, as it's possible it should be consolidatable
		// the next time we try to disrupt.
		if !constrainedByBudgets {
			m.markConsolidated()
		}
		return cmd, scheduling.Results{}, nil
	}

	if err := NewValidation(m.clock, m.cluster, m.kubeClient, m.provisioner, m.cloudProvider, m.recorder, m.queue, m.Reason()).IsValid(ctx, cmd, consolidationTTL); err != nil {
		if IsValidationError(err) {
			log.FromContext(ctx).V(1).Info(fmt.Sprintf("abandoning multi-node consolidation attempt due to pod churn, command is no longer valid, %s", cmd))
			return Command{}, scheduling.Results{}, nil
		}
		return Command{}, scheduling.Results{}, fmt.Errorf("validating consolidation, %w", err)
	}
	return cmd, results, nil
}

// firstNConsolidationOption looks at the first N NodeClaims to determine if they can all be consolidated at once.  The
// NodeClaims are sorted by increasing disruption order which correlates to likelihood of being able to consolidate the node
func (m *MultiNodeConsolidation) firstNConsolidationOption(ctx context.Context, candidates []*Candidate, max int) (Command, scheduling.Results, error) {
	// we always operate on at least two NodeClaims at once, for single NodeClaims standard consolidation will find all solutions
	if len(candidates) < 2 {
		return Command{}, scheduling.Results{}, nil
	}
	min := 1
	if len(candidates) <= max {
		max = len(candidates) - 1
	}

	lastSavedCommand := Command{}
	lastSavedResults := scheduling.Results{}
	// Set a timeout
	timeout := m.clock.Now().Add(MultiNodeConsolidationTimeoutDuration)
	// binary search to find the maximum number of NodeClaims we can terminate
	for min <= max {
		if m.clock.Now().After(timeout) {
			ConsolidationTimeoutsTotal.Inc(map[string]string{consolidationTypeLabel: m.ConsolidationType()})
			if lastSavedCommand.candidates == nil {
				log.FromContext(ctx).V(1).Info(fmt.Sprintf("failed to find a multi-node consolidation after timeout, last considered batch had %d", (min+max)/2))
			} else {
				log.FromContext(ctx).V(1).Info(fmt.Sprintf("stopping multi-node consolidation after timeout, returning last valid command %s", lastSavedCommand))
			}
			return lastSavedCommand, lastSavedResults, nil
		}
		mid := (min + max) / 2
		candidatesToConsolidate := candidates[0 : mid+1]

		cmd, results, err := m.computeConsolidation(ctx, candidatesToConsolidate...)
		if err != nil {
			return Command{}, scheduling.Results{}, err
		}

		// ensure that the action is sensical for replacements, see explanation on filterOutSameType for why this is
		// required
		replacementHasValidInstanceTypes := false
		if cmd.Decision() == ReplaceDecision {
			cmd.replacements[0].InstanceTypeOptions, err = filterOutSameType(cmd.replacements[0], candidatesToConsolidate)
			replacementHasValidInstanceTypes = len(cmd.replacements[0].InstanceTypeOptions) > 0 && err == nil
		}

		// replacementHasValidInstanceTypes will be false if the replacement action has valid instance types remaining after filtering.
		if replacementHasValidInstanceTypes || cmd.Decision() == DeleteDecision {
			// We can consolidate NodeClaims [0,mid]
			lastSavedCommand = cmd
			lastSavedResults = results
			min = mid + 1
		} else {
			max = mid - 1
		}
	}
	return lastSavedCommand, lastSavedResults, nil
}

// filterOutSameType filters out instance types that are more expensive than the cheapest instance type that is being
// consolidated if the list of replacement instance types include one of the instance types that is being removed
//
// This handles the following potential consolidation result:
// NodeClaims=[t3a.2xlarge, t3a.2xlarge, t3a.small] -> 1 of t3a.small, t3a.xlarge, t3a.2xlarge
//
// In this case, we shouldn't perform this consolidation at all.  This is equivalent to just
// deleting the 2x t3a.xlarge NodeClaims.  This code will identify that t3a.small is in both lists and filter
// out any instance type that is the same or more expensive than the t3a.small
//
// For another scenario:
// NodeClaims=[t3a.2xlarge, t3a.2xlarge, t3a.small] -> 1 of t3a.nano, t3a.small, t3a.xlarge, t3a.2xlarge
//
// This code sees that t3a.small is the cheapest type in both lists and filters it and anything more expensive out
// leaving the valid consolidation:
// NodeClaims=[t3a.2xlarge, t3a.2xlarge, t3a.small] -> 1 of t3a.nano
func filterOutSameType(newNodeClaim *scheduling.NodeClaim, consolidate []*Candidate) ([]*cloudprovider.InstanceType, error) {
	existingInstanceTypes := sets.New[string]()
	pricesByInstanceType := map[string]float64{}

	// get the price of the cheapest node that we currently are considering deleting indexed by instance type
	for _, c := range consolidate {
		existingInstanceTypes.Insert(c.instanceType.Name)
		compatibleOfferings := c.instanceType.Offerings.Compatible(scheduler.NewLabelRequirements(c.StateNode.Labels()))
		if len(compatibleOfferings) == 0 {
			continue
		}
		existingPrice, ok := pricesByInstanceType[c.instanceType.Name]
		if !ok {
			existingPrice = math.MaxFloat64
		}
		if p := compatibleOfferings.Cheapest().Price; p < existingPrice {
			pricesByInstanceType[c.instanceType.Name] = p
		}
	}

	maxPrice := math.MaxFloat64
	for _, it := range newNodeClaim.InstanceTypeOptions {
		// we are considering replacing multiple NodeClaims with a single NodeClaim of one of the same types, so the replacement
		// node must be cheaper than the price of the existing node, or we should just keep that one and do a
		// deletion only to reduce cluster disruption (fewer pods will re-schedule).
		if existingInstanceTypes.Has(it.Name) {
			if pricesByInstanceType[it.Name] < maxPrice {
				maxPrice = pricesByInstanceType[it.Name]
			}
		}
	}
	// swallow the error since we don't allow min values to impact reschedulability in multi node claim
	newNodeClaim, err := newNodeClaim.RemoveInstanceTypeOptionsByPriceAndMinValues(newNodeClaim.Requirements, maxPrice)
	if err != nil {
		return nil, err
	}
	return newNodeClaim.InstanceTypeOptions, nil
}

func (m *MultiNodeConsolidation) Reason() v1.DisruptionReason {
	return v1.DisruptionReasonUnderutilized
}

func (m *MultiNodeConsolidation) Class() string {
	return GracefulDisruptionClass
}

func (m *MultiNodeConsolidation) ConsolidationType() string {
	return MultiNodeConsolidationType
}
