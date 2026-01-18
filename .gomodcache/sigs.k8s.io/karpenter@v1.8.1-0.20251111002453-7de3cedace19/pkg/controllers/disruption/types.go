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
	"time"

	"github.com/awslabs/operatorpkg/option"
	"github.com/awslabs/operatorpkg/serrors"
	"github.com/google/uuid"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	disruptionevents "sigs.k8s.io/karpenter/pkg/controllers/disruption/events"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning/scheduling"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/events"
	disruptionutils "sigs.k8s.io/karpenter/pkg/utils/disruption"
	"sigs.k8s.io/karpenter/pkg/utils/pdb"
	"sigs.k8s.io/karpenter/pkg/utils/pod"
)

const (
	GracefulDisruptionClass = "graceful" // graceful disruption always respects blocking pod PDBs and the do-not-disrupt annotation
	EventualDisruptionClass = "eventual" // eventual disruption is bounded by a NodePool's TerminationGracePeriod, regardless of blocking pod PDBs and the do-not-disrupt annotation
)

type MethodOptions struct {
	validator Validator
}

func WithValidator(v Validator) option.Function[MethodOptions] {
	return func(o *MethodOptions) {
		o.validator = v
	}
}

type Method interface {
	ShouldDisrupt(context.Context, *Candidate) bool
	ComputeCommands(context.Context, map[string]int, ...*Candidate) ([]Command, error)
	Reason() v1.DisruptionReason
	Class() string
	ConsolidationType() string
}

type CandidateFilter func(context.Context, *Candidate) bool

// Candidate is a state.StateNode that we are considering for disruption along with extra information to be used in
// making that determination
type Candidate struct {
	*state.StateNode
	instanceType      *cloudprovider.InstanceType
	NodePool          *v1.NodePool
	zone              string
	capacityType      string
	DisruptionCost    float64
	reschedulablePods []*corev1.Pod
}

func (c *Candidate) OwnedByStaticNodePool() bool {
	return c.NodePool.Spec.Replicas != nil
}

//nolint:gocyclo
func NewCandidate(ctx context.Context, kubeClient client.Client, recorder events.Recorder, clk clock.Clock, node *state.StateNode, pdbs pdb.Limits,
	nodePoolMap map[string]*v1.NodePool, nodePoolToInstanceTypesMap map[string]map[string]*cloudprovider.InstanceType, queue *Queue, disruptionClass string) (*Candidate, error) {
	var err error
	var pods []*corev1.Pod
	// If the orchestration queue is already considering a candidate we want to disrupt, don't consider it a candidate.
	if queue.HasAny(node.ProviderID()) {
		return nil, fmt.Errorf("candidate is already being disrupted")
	}
	if err = node.ValidateNodeDisruptable(); err != nil {
		// Only emit an event if the NodeClaim is not nil, ensuring that we only emit events for Karpenter-managed nodes
		if node.NodeClaim != nil {
			recorder.Publish(disruptionevents.Blocked(node.Node, node.NodeClaim, pretty.Sentence(err.Error()))...)
		}
		return nil, err
	}
	// We know that the node will have the label key because of the node.IsDisruptable check above
	nodePoolName := node.Labels()[v1.NodePoolLabelKey]
	nodePool := nodePoolMap[nodePoolName]
	instanceTypeMap := nodePoolToInstanceTypesMap[nodePoolName]
	// skip any candidates where we can't determine the nodePool
	if nodePool == nil || instanceTypeMap == nil {
		recorder.Publish(disruptionevents.Blocked(node.Node, node.NodeClaim, fmt.Sprintf("NodePool not found (NodePool=%s)", nodePoolName))...)
		return nil, serrors.Wrap(fmt.Errorf("nodepool not found"), "NodePool", klog.KRef("", nodePoolName))
	}
	// We only care if instanceType in non-empty consolidation to do price-comparison.
	instanceType := instanceTypeMap[node.Labels()[corev1.LabelInstanceTypeStable]]
	if pods, err = node.ValidatePodsDisruptable(ctx, kubeClient, pdbs); err != nil {
		// If the NodeClaim has a TerminationGracePeriod set and the disruption class is eventual, the node should be
		// considered a candidate even if there's a pod that will block eviction. Other error types should still cause
		// failure creating the candidate.
		eventualDisruptionCandidate := node.NodeClaim.Spec.TerminationGracePeriod != nil && disruptionClass == EventualDisruptionClass
		if lo.Ternary(eventualDisruptionCandidate, state.IgnorePodBlockEvictionError(err), err) != nil {
			recorder.Publish(disruptionevents.Blocked(node.Node, node.NodeClaim, pretty.Sentence(err.Error()))...)
			return nil, err
		}
	}
	return &Candidate{
		StateNode:         node,
		instanceType:      instanceType,
		NodePool:          nodePool,
		capacityType:      node.Labels()[v1.CapacityTypeLabelKey],
		zone:              node.Labels()[corev1.LabelTopologyZone],
		reschedulablePods: lo.Filter(pods, func(p *corev1.Pod, _ int) bool { return pod.IsReschedulable(p) }),
		// We get the disruption cost from all pods in the candidate, not just the reschedulable pods
		DisruptionCost: disruptionutils.ReschedulingCost(ctx, pods) * disruptionutils.LifetimeRemaining(clk, nodePool, node.NodeClaim),
	}, nil
}

type Replacement struct {
	*scheduling.NodeClaim

	Name string
	// Use a bool track if a node has already been initialized so we can fire metrics for intialization once.
	// This intentionally does not capture nodes that go initialized then go NotReady after as other pods can
	// schedule to this node as well.
	Initialized bool
}

func replacementsFromNodeClaims(newNodeClaims ...*scheduling.NodeClaim) []*Replacement {
	return lo.Map(newNodeClaims, func(n *scheduling.NodeClaim, _ int) *Replacement { return &Replacement{NodeClaim: n} })
}

type Command struct {
	Method

	Succeeded bool

	CreationTimestamp time.Time
	ID                uuid.UUID

	Results      scheduling.Results
	Candidates   []*Candidate
	Replacements []*Replacement
}

type Decision string

var (
	NoOpDecision    Decision = "no-op"
	ReplaceDecision Decision = "replace"
	DeleteDecision  Decision = "delete"
)

func (c Command) Decision() Decision {
	switch {
	case len(c.Candidates) > 0 && len(c.Replacements) > 0:
		return ReplaceDecision
	case len(c.Candidates) > 0 && len(c.Replacements) == 0:
		return DeleteDecision
	default:
		return NoOpDecision
	}
}

func (c Command) LogValues() []any {
	podCount := lo.Reduce(c.Candidates, func(_ int, cd *Candidate, _ int) int { return len(cd.reschedulablePods) }, 0)

	candidateNodes := lo.Map(c.Candidates, func(candidate *Candidate, _ int) interface{} {
		return map[string]interface{}{
			"Node":          klog.KObj(candidate.Node),
			"NodeClaim":     klog.KObj(candidate.NodeClaim),
			"instance-type": candidate.Labels()[corev1.LabelInstanceTypeStable],
			"capacity-type": candidate.Labels()[v1.CapacityTypeLabelKey],
		}
	})
	replacementNodes := lo.Map(c.Replacements, func(replacement *Replacement, _ int) interface{} {
		ct := replacement.Requirements.Get(v1.CapacityTypeLabelKey)
		m := map[string]interface{}{
			"capacity-type": lo.If(
				ct.Has(v1.CapacityTypeReserved), v1.CapacityTypeReserved,
			).ElseIf(
				ct.Has(v1.CapacityTypeSpot), v1.CapacityTypeSpot,
			).Else(v1.CapacityTypeOnDemand),
		}
		if len(c.Replacements) == 1 {
			m["instance-types"] = scheduling.InstanceTypeList(replacement.InstanceTypeOptions)
		}
		return m
	})

	return []any{
		"decision", c.Decision(),
		"disrupted-node-count", len(candidateNodes),
		"replacement-node-count", len(replacementNodes),
		"pod-count", podCount,
		"disrupted-nodes", candidateNodes,
		"replacement-nodes", replacementNodes,
	}
}
