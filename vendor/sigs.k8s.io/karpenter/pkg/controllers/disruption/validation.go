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
	"sync"
	"time"

	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/controllers/disruption/orchestration"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/events"
)

type ValidationError struct {
	error
}

func NewValidationError(err error) *ValidationError {
	return &ValidationError{error: err}
}

func IsValidationError(err error) bool {
	if err == nil {
		return false
	}
	var validationError *ValidationError
	return errors.As(err, &validationError)
}

// Validation is used to perform validation on a consolidation command.  It makes an assumption that when re-used, all
// of the commands passed to IsValid were constructed based off of the same consolidation state.  This allows it to
// skip the validation TTL for all but the first command.
type Validation struct {
	start         time.Time
	clock         clock.Clock
	cluster       *state.Cluster
	kubeClient    client.Client
	cloudProvider cloudprovider.CloudProvider
	provisioner   *provisioning.Provisioner
	once          sync.Once
	recorder      events.Recorder
	queue         *orchestration.Queue
	reason        v1.DisruptionReason
}

func NewValidation(clk clock.Clock, cluster *state.Cluster, kubeClient client.Client, provisioner *provisioning.Provisioner,
	cp cloudprovider.CloudProvider, recorder events.Recorder, queue *orchestration.Queue, reason v1.DisruptionReason) *Validation {
	return &Validation{
		clock:         clk,
		cluster:       cluster,
		kubeClient:    kubeClient,
		provisioner:   provisioner,
		cloudProvider: cp,
		recorder:      recorder,
		queue:         queue,
		reason:        reason,
	}
}

func (v *Validation) IsValid(ctx context.Context, cmd Command, validationPeriod time.Duration) error {
	var err error
	v.once.Do(func() {
		v.start = v.clock.Now()
	})

	waitDuration := validationPeriod - v.clock.Since(v.start)
	if waitDuration > 0 {
		select {
		case <-ctx.Done():
			return errors.New("context canceled")
		case <-v.clock.After(waitDuration):
		}
	}
	validatedCandidates, err := v.ValidateCandidates(ctx, cmd.candidates...)
	if err != nil {
		return err
	}
	if err := v.ValidateCommand(ctx, cmd, validatedCandidates); err != nil {
		return err
	}
	// Revalidate candidates after validating the command. This mitigates the chance of a race condition outlined in
	// the following GitHub issue: https://github.com/kubernetes-sigs/karpenter/issues/1167.
	if _, err = v.ValidateCandidates(ctx, validatedCandidates...); err != nil {
		return err
	}
	return nil
}

// ValidateCandidates gets the current representation of the provided candidates and ensures that they are all still valid.
// For a candidate to still be valid, the following conditions must be met:
//
//	a. It must pass the global candidate filtering logic (no blocking PDBs, no do-not-disrupt annotation, etc)
//	b. It must not have any pods nominated for it
//	c. It must still be disruptable without violating node disruption budgets
//
// If these conditions are met for all candidates, ValidateCandidates returns a slice with the updated representations.
func (v *Validation) ValidateCandidates(ctx context.Context, candidates ...*Candidate) ([]*Candidate, error) {
	// GracefulDisruptionClass is hardcoded here because ValidateCandidates is only used for consolidation disruption. All consolidation disruption is graceful disruption.
	validatedCandidates, err := GetCandidates(ctx, v.cluster, v.kubeClient, v.recorder, v.clock, v.cloudProvider, v.ShouldDisrupt, GracefulDisruptionClass, v.queue)
	if err != nil {
		return nil, fmt.Errorf("constructing validation candidates, %w", err)
	}
	validatedCandidates = mapCandidates(candidates, validatedCandidates)
	// If we filtered out any candidates, return nil as some NodeClaims in the consolidation decision have changed.
	if len(validatedCandidates) != len(candidates) {
		return nil, NewValidationError(fmt.Errorf("%d candidates are no longer valid", len(candidates)-len(validatedCandidates)))
	}
	disruptionBudgetMapping, err := BuildDisruptionBudgetMapping(ctx, v.cluster, v.clock, v.kubeClient, v.cloudProvider, v.recorder, v.reason)
	if err != nil {
		return nil, fmt.Errorf("building disruption budgets, %w", err)
	}
	// Return nil if any candidate meets either of the following conditions:
	//  a. A pod was nominated to the candidate
	//  b. Disrupting the candidate would violate node disruption budgets
	for _, vc := range validatedCandidates {
		if v.cluster.IsNodeNominated(vc.ProviderID()) {
			return nil, NewValidationError(fmt.Errorf("a candidate was nominated during validation"))
		}
		if disruptionBudgetMapping[vc.nodePool.Name] == 0 {
			return nil, NewValidationError(fmt.Errorf("a candidate can no longer be disrupted without violating budgets"))
		}
		disruptionBudgetMapping[vc.nodePool.Name]--
	}
	return validatedCandidates, nil
}

// ShouldDisrupt is a predicate used to filter candidates
func (v *Validation) ShouldDisrupt(_ context.Context, c *Candidate) bool {
	return c.nodePool.Spec.Disruption.ConsolidateAfter.Duration != nil && c.NodeClaim.StatusConditions().Get(v1.ConditionTypeConsolidatable).IsTrue()
}

// ValidateCommand validates a command for a Method
func (v *Validation) ValidateCommand(ctx context.Context, cmd Command, candidates []*Candidate) error {
	// None of the chosen candidate are valid for execution, so retry
	if len(candidates) == 0 {
		return NewValidationError(fmt.Errorf("no candidates"))
	}
	results, err := SimulateScheduling(ctx, v.kubeClient, v.cluster, v.provisioner, candidates...)
	if err != nil {
		return fmt.Errorf("simluating scheduling, %w", err)
	}
	if !results.AllNonPendingPodsScheduled() {
		return NewValidationError(errors.New(results.NonPendingPodSchedulingErrors()))
	}

	// We want to ensure that the re-simulated scheduling using the current cluster state produces the same result.
	// There are three possible options for the number of new candidates that we need to handle:
	// len(NewNodeClaims) == 0, as long as we weren't expecting a new node, this is valid
	// len(NewNodeClaims) > 1, something in the cluster changed so that the candidates we were going to delete can no longer
	//                    be deleted without producing more than one node
	// len(NewNodeClaims) == 1, as long as the noe looks like what we were expecting, this is valid
	if len(results.NewNodeClaims) == 0 {
		if len(cmd.replacements) == 0 {
			// scheduling produced zero new NodeClaims and we weren't expecting any, so this is valid.
			return nil
		}
		// if it produced no new NodeClaims, but we were expecting one we should re-simulate as there is likely a better
		// consolidation option now
		return NewValidationError(fmt.Errorf("scheduling simulation produced new results"))
	}

	// we need more than one replacement node which is never valid currently (all of our node replacement is m->1, never m->n)
	if len(results.NewNodeClaims) > 1 {
		return NewValidationError(fmt.Errorf("scheduling simulation produced new results"))
	}

	// we now know that scheduling simulation wants to create one new node
	if len(cmd.replacements) == 0 {
		// but we weren't expecting any new NodeClaims, so this is invalid
		return NewValidationError(fmt.Errorf("scheduling simulation produced new results"))
	}

	// We know that the scheduling simulation wants to create a new node and that the command we are verifying wants
	// to create a new node. The scheduling simulation doesn't apply any filtering to instance types, so it may include
	// instance types that we don't want to launch which were filtered out when the lifecycleCommand was created.  To
	// check if our lifecycleCommand is valid, we just want to ensure that the list of instance types we are considering
	// creating are a subset of what scheduling says we should create.  We check for a subset since the scheduling
	// simulation here does no price filtering, so it will include more expensive types.
	//
	// This is necessary since consolidation only wants cheaper NodeClaims.  Suppose consolidation determined we should delete
	// a 4xlarge and replace it with a 2xlarge. If things have changed and the scheduling simulation we just performed
	// now says that we need to launch a 4xlarge. It's still launching the correct number of NodeClaims, but it's just
	// as expensive or possibly more so we shouldn't validate.
	if !instanceTypesAreSubset(cmd.replacements[0].InstanceTypeOptions, results.NewNodeClaims[0].InstanceTypeOptions) {
		return NewValidationError(fmt.Errorf("scheduling simulation produced new results"))
	}

	// Now we know:
	// - current scheduling simulation says to create a new node with types T = {T_0, T_1, ..., T_n}
	// - our lifecycle command says to create a node with types {U_0, U_1, ..., U_n} where U is a subset of T
	return nil
}
