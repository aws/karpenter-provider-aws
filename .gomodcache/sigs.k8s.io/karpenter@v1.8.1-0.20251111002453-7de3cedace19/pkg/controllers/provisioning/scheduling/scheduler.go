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

package scheduling

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/awslabs/operatorpkg/option"
	"github.com/awslabs/operatorpkg/serrors"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/metrics"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	karpopts "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/utils/pod"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

type ReservedOfferingMode int

// TODO: Evaluate if another mode should be created for drift. The problem with strict is that it assumes we can run
// multiple scheduling loops to make progress, but if scheduling all pods from the drifted node in a single iteration
// requires fallback, we're at a stalemate. This makes strict a non-starter for drift IMO.
// On the other hand, fallback will result in non-ideal launches when there's constrained capacity. This should be
// rectified by consolidation, but if we can be "right" the at initial launch that would be preferable.
// One potential improvement is a "preferences" type strategy, where we attempt to schedule the pod without fallback
// first. This is an improvement over the current fallback strategy since it ensures all new nodeclaims are attempted,
// before then attempting all nodepools, but it still doesn't address the case when offerings are reserved pessimistically.
// I don't believe there's a solution to this short of the max-flow based instance selection algorithm, which has its own
// drawbacks.
const (
	// ReservedOfferingModeFallbackAlways indicates to the scheduler that the addition of a pod to a nodeclaim which
	// results in all potential reserved offerings being filtered out is allowed (e.g. on-demand / spot fallback).
	ReservedOfferingModeFallback ReservedOfferingMode = iota
	// ReservedOfferingModeStrict indicates that the scheduler should fail to add a pod to a nodeclaim if doing so would
	// prevent it from scheduling to reserved capacity, when it would have otherwise.
	ReservedOfferingModeStrict
)

type PreferencePolicy int

const (
	// PreferencePolicyRespect indicates to the scheduler that it should attempt to respect all preference requirements
	// and topologies. The scheduler will treat all preferences as required at first and then will slowly relax
	// these requirements one at a time until it is able to schedule the pod
	PreferencePolicyRespect PreferencePolicy = iota
	// PreferencePolicyIgnore indicates to the scheduler that it should ignore all preference requirements and
	// topologies. Preferences include preferredDuringSchedulingIgnoredDuringExecution affinities and ScheduleAnyways
	// topologySpreadConstraints
	PreferencePolicyIgnore
)

type options struct {
	reservedOfferingMode    ReservedOfferingMode
	preferencePolicy        PreferencePolicy
	minValuesPolicy         karpopts.MinValuesPolicy
	numConcurrentReconciles int
}

type Options = option.Function[options]

var DisableReservedCapacityFallback = func(opts *options) {
	opts.reservedOfferingMode = ReservedOfferingModeStrict
}

var IgnorePreferences = func(opts *options) {
	opts.preferencePolicy = PreferencePolicyIgnore
}

var NumConcurrentReconciles = func(numConcurrentReconciles int) func(*options) {
	return func(opts *options) {
		opts.numConcurrentReconciles = numConcurrentReconciles
	}
}

var MinValuesPolicy = func(policy karpopts.MinValuesPolicy) func(*options) {
	return func(opts *options) {
		opts.minValuesPolicy = policy
	}
}

func NewScheduler(
	ctx context.Context,
	kubeClient client.Client,
	nodePools []*v1.NodePool,
	cluster *state.Cluster,
	stateNodes []*state.StateNode,
	topology *Topology,
	instanceTypes map[string][]*cloudprovider.InstanceType,
	daemonSetPods []*corev1.Pod,
	recorder events.Recorder,
	clock clock.Clock,
	opts ...Options,
) *Scheduler {
	minValuesPolicy := option.Resolve(opts...).minValuesPolicy

	// if any of the nodePools add a taint with a prefer no schedule effect, we add a toleration for the taint
	// during preference relaxation
	toleratePreferNoSchedule := false
	for _, np := range nodePools {
		for _, taint := range np.Spec.Template.Spec.Taints {
			if taint.Effect == corev1.TaintEffectPreferNoSchedule {
				toleratePreferNoSchedule = true
			}
		}
	}
	// Pre-filter instance types eligible for NodePools to reduce work done during scheduling loops for pods
	// if no templates remain, we still want to build the scheduler so that Karpenter can ack pods which can schedule to existing and in-flight capacity
	templates := lo.FilterMap(nodePools, func(np *v1.NodePool, _ int) (*NodeClaimTemplate, bool) {
		var err error
		nct := NewNodeClaimTemplate(np)
		nct.InstanceTypeOptions, _, err = filterInstanceTypesByRequirements(instanceTypes[np.Name], nct.Requirements, corev1.ResourceList{}, corev1.ResourceList{}, corev1.ResourceList{}, minValuesPolicy == karpopts.MinValuesPolicyBestEffort)
		if len(nct.InstanceTypeOptions) == 0 {
			if instanceTypeFilterErr, ok := lo.ErrorsAs[InstanceTypeFilterError](err); ok && instanceTypeFilterErr.minValuesIncompatibleErr != nil {
				recorder.Publish(NoCompatibleInstanceTypes(np, true))
				log.FromContext(ctx).WithValues("NodePool", klog.KObj(np)).Info("skipping, nodepool requirements filtered out all instance types", "minValuesIncompatibleErr", instanceTypeFilterErr.minValuesIncompatibleErr)
			} else {
				recorder.Publish(NoCompatibleInstanceTypes(np, false))
				log.FromContext(ctx).WithValues("NodePool", klog.KObj(np)).Info("skipping, nodepool requirements filtered out all instance types")
			}
			return nil, false
		}
		return nct, true
	})
	s := &Scheduler{
		uuid:                uuid.NewUUID(),
		kubeClient:          kubeClient,
		nodeClaimTemplates:  templates,
		topology:            topology,
		cluster:             cluster,
		daemonOverhead:      getDaemonOverhead(ctx, templates, daemonSetPods),
		daemonHostPortUsage: getDaemonHostPortUsage(ctx, templates, daemonSetPods),
		cachedPodData:       map[types.UID]*PodData{}, // cache pod data to avoid having to continually recompute it
		recorder:            recorder,
		preferences:         &Preferences{ToleratePreferNoSchedule: toleratePreferNoSchedule},
		remainingResources: lo.SliceToMap(nodePools, func(np *v1.NodePool) (string, corev1.ResourceList) {
			return np.Name, corev1.ResourceList(np.Spec.Limits)
		}),
		clock:                   clock,
		reservationManager:      NewReservationManager(instanceTypes),
		reservedOfferingMode:    option.Resolve(opts...).reservedOfferingMode,
		preferencePolicy:        option.Resolve(opts...).preferencePolicy,
		minValuesPolicy:         minValuesPolicy,
		numConcurrentReconciles: lo.Ternary(option.Resolve(opts...).numConcurrentReconciles > 0, option.Resolve(opts...).numConcurrentReconciles, 1),
	}
	s.calculateExistingNodeClaims(ctx, stateNodes, daemonSetPods)
	return s
}

type PodData struct {
	Requests                 corev1.ResourceList
	Requirements             scheduling.Requirements
	StrictRequirements       scheduling.Requirements
	HasResourceClaimRequests bool
}

type Scheduler struct {
	uuid                    types.UID // Unique UUID attached to this scheduling loop
	newNodeClaims           []*NodeClaim
	existingNodes           []*ExistingNode
	nodeClaimTemplates      []*NodeClaimTemplate
	remainingResources      map[string]corev1.ResourceList // (NodePool name) -> remaining resources for that NodePool
	daemonOverhead          map[*NodeClaimTemplate]corev1.ResourceList
	daemonHostPortUsage     map[*NodeClaimTemplate]*scheduling.HostPortUsage
	cachedPodData           map[types.UID]*PodData // (Pod Namespace/Name) -> pre-computed data for pods to avoid re-computation and memory usage
	preferences             *Preferences
	topology                *Topology
	cluster                 *state.Cluster
	recorder                events.Recorder
	kubeClient              client.Client
	clock                   clock.Clock
	reservationManager      *ReservationManager
	reservedOfferingMode    ReservedOfferingMode
	preferencePolicy        PreferencePolicy
	minValuesPolicy         karpopts.MinValuesPolicy
	numConcurrentReconciles int
}

// DRAError indicates a pod will not be attempted to be scheduled because it has Dynamic Resource Allocation requirements
// that are not yet supported by Karpenter
type DRAError struct {
	error
}

func NewDRAError(err error) DRAError {
	return DRAError{error: err}
}

func IsDRAError(err error) bool {
	draErr := &DRAError{}
	return errors.As(err, draErr)
}

func (e DRAError) Unwrap() error {
	return e.error
}

// Results contains the results of the scheduling operation
type Results struct {
	NewNodeClaims []*NodeClaim
	ExistingNodes []*ExistingNode
	PodErrors     map[*corev1.Pod]error
}

// Record sends eventing and log messages back for the results that were produced from a scheduling run
// It also nominates nodes in the cluster state based on the scheduling run to signal to other components
// leveraging the cluster state that a previous scheduling run that was recorded is relying on these nodes
func (r Results) Record(ctx context.Context, recorder events.Recorder, cluster *state.Cluster) {
	// Report failures and nominations
	for p, err := range r.PodErrors {
		if IsReservedOfferingError(err) {
			continue
		}
		if IsDRAError(err) {
			recorder.Publish(PodFailedToScheduleEvent(p, err))
			log.FromContext(ctx).WithValues("Pod", klog.KObj(p)).Info("skipping pod with Dynamic Resource Allocation requirements, not yet supported by Karpenter")
			continue
		}
		log.FromContext(ctx).WithValues("Pod", klog.KObj(p)).Error(err, "could not schedule pod")
		recorder.Publish(PodFailedToScheduleEvent(p, err))
	}
	for _, existing := range r.ExistingNodes {
		if len(existing.Pods) > 0 {
			cluster.NominateNodeForPod(ctx, existing.ProviderID())
		}
		for _, p := range existing.Pods {
			recorder.Publish(NominatePodEvent(p, existing.Node, existing.NodeClaim))
		}
	}
	// Report new nodes, or exit to avoid log spam
	newCount := 0
	for _, nodeClaim := range r.NewNodeClaims {
		newCount += len(nodeClaim.Pods)
	}
	if newCount == 0 {
		return
	}
	log.FromContext(ctx).WithValues("nodeclaims", len(r.NewNodeClaims), "pods", newCount).Info("computed new nodeclaim(s) to fit pod(s)")
	// Report in flight newNodes, or exit to avoid log spam
	inflightCount := 0
	existingCount := 0
	for _, node := range lo.Filter(r.ExistingNodes, func(node *ExistingNode, _ int) bool { return len(node.Pods) > 0 }) {
		inflightCount++
		existingCount += len(node.Pods)
	}
	if existingCount == 0 {
		return
	}
	log.FromContext(ctx).WithValues("nodes", inflightCount, "pods", existingCount).Info("computed unready node(s) will fit pod(s)")
}

func (r Results) ReservedOfferingErrors() map[*corev1.Pod]error {
	return lo.PickBy(r.PodErrors, func(_ *corev1.Pod, err error) bool {
		return IsReservedOfferingError(err)
	})
}

func (r Results) DRAErrors() map[*corev1.Pod]error {
	return lo.PickBy(r.PodErrors, func(_ *corev1.Pod, err error) bool {
		return IsDRAError(err)
	})
}

func (r Results) NodePoolToPodMapping() map[string][]*corev1.Pod {
	result := make(map[string][]*corev1.Pod)

	for _, nc := range r.NewNodeClaims {
		nodePoolName := nc.Labels[v1.NodePoolLabelKey]
		result[nodePoolName] = append(result[nodePoolName], nc.Pods...)
	}

	for _, nc := range r.ExistingNodes {
		nodePoolName := nc.Labels()[v1.NodePoolLabelKey]
		result[nodePoolName] = append(result[nodePoolName], nc.Pods...)
	}

	return result
}

func (r Results) ExistingNodeToPodMapping() map[string][]*corev1.Pod {
	return lo.SliceToMap(lo.Filter(r.ExistingNodes, func(n *ExistingNode, _ int) bool {
		// Filter out nodes that are not managed
		return n.Managed()
	}), func(n *ExistingNode) (string, []*corev1.Pod) {
		return n.NodeClaim.Name, n.Pods
	})
}

// AllNonPendingPodsScheduled returns true if all pods scheduled.
// We don't care if a pod was pending before consolidation and will still be pending after. It may be a pod that we can't
// schedule at all and don't want it to block consolidation.
func (r Results) AllNonPendingPodsScheduled() bool {
	return len(lo.OmitBy(r.PodErrors, func(p *corev1.Pod, err error) bool {
		return pod.IsProvisionable(p)
	})) == 0
}

// NonPendingPodSchedulingErrors creates a string that describes why pods wouldn't schedule that is suitable for presentation
func (r Results) NonPendingPodSchedulingErrors() string {
	errs := lo.OmitBy(r.PodErrors, func(p *corev1.Pod, err error) bool {
		return pod.IsProvisionable(p)
	})
	if len(errs) == 0 {
		return "No Pod Scheduling Errors"
	}
	var msg bytes.Buffer
	fmt.Fprintf(&msg, "not all pods would schedule, ")
	const MaxErrors = 5
	numErrors := 0
	for k, err := range errs {
		fmt.Fprintf(&msg, "%s/%s => %s ", k.Namespace, k.Name, err)
		numErrors++
		if numErrors >= MaxErrors {
			fmt.Fprintf(&msg, " and %d other(s)", len(errs)-MaxErrors)
			break
		}
	}
	return msg.String()
}

// TruncateInstanceTypes filters the result based on the maximum number of instanceTypes that needs
// to be considered. This filters all instance types generated in NewNodeClaims in the Results
func (r Results) TruncateInstanceTypes(ctx context.Context, maxInstanceTypes int) Results {
	var validNewNodeClaims []*NodeClaim
	for _, newNodeClaim := range r.NewNodeClaims {
		// The InstanceTypeOptions are truncated due to limitations in sending the number of instances to launch API.
		var err error
		newNodeClaim.InstanceTypeOptions, err = newNodeClaim.InstanceTypeOptions.Truncate(ctx, newNodeClaim.Requirements, maxInstanceTypes)
		if err != nil {
			// Check if the truncated InstanceTypeOptions in each NewNodeClaim from the results still satisfy the minimum requirements
			// If number of InstanceTypes in the NodeClaim cannot satisfy the minimum requirements, add its Pods to error map with reason.
			for _, pod := range newNodeClaim.Pods {
				r.PodErrors[pod] = serrors.Wrap(fmt.Errorf("pod didn’t schedule because NodePool couldn’t meet minValues requirements, %w", err), "NodePool", klog.KRef("", newNodeClaim.NodePoolName))
			}
		} else {
			validNewNodeClaims = append(validNewNodeClaims, newNodeClaim)
		}
	}
	r.NewNodeClaims = validNewNodeClaims
	return r
}

func (s *Scheduler) Solve(ctx context.Context, pods []*corev1.Pod) (Results, error) {
	defer metrics.Measure(DurationSeconds, map[string]string{ControllerLabel: injection.GetControllerName(ctx)})()
	// We loop trying to schedule unschedulable pods as long as we are making progress.  This solves a few
	// issues including pods with affinity to another pod in the batch. We could topo-sort to solve this, but it wouldn't
	// solve the problem of scheduling pods where a particular order is needed to prevent a max-skew violation. E.g. if we
	// had 5xA pods and 5xB pods were they have a zonal topology spread, but A can only go in one zone and B in another.
	// We need to schedule them alternating, A, B, A, B, .... and this solution also solves that as well.
	podErrors := map[*corev1.Pod]error{}
	// Reset the metric for the controller, so we don't keep old ids around
	UnschedulablePodsCount.DeletePartialMatch(map[string]string{ControllerLabel: injection.GetControllerName(ctx)})
	QueueDepth.DeletePartialMatch(map[string]string{ControllerLabel: injection.GetControllerName(ctx)})
	for _, p := range pods {
		s.updateCachedPodData(p)
	}
	q := NewQueue(pods, s.cachedPodData)

	startTime := s.clock.Now()
	for {
		UnfinishedWorkSeconds.Set(s.clock.Since(startTime).Seconds(), map[string]string{ControllerLabel: injection.GetControllerName(ctx), schedulingIDLabel: string(s.uuid)})
		QueueDepth.Set(float64(len(q.pods)), map[string]string{ControllerLabel: injection.GetControllerName(ctx), schedulingIDLabel: string(s.uuid)})

		// Try the next pod
		pod, ok := q.Pop()
		if !ok {
			break
		}
		// We relax the pod all the way the first time we see it
		// If we don't schedule it, we store the original pod (with preferences)
		// in the queue and give ourselves another chance to schedule it later
		if err := s.trySchedule(ctx, pod.DeepCopy()); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				log.FromContext(ctx).V(1).WithValues("duration", s.clock.Since(startTime).Truncate(time.Second), "scheduling-id", string(s.uuid)).Info("scheduling simulation timed out")
				break
			}
			podErrors[pod] = err
			if e := s.topology.Update(ctx, pod); e != nil && !errors.Is(e, context.DeadlineExceeded) {
				log.FromContext(ctx).Error(e, "failed updating topology")
			}
			// Update the cached podData since the pod was relaxed, and it could have changed its requirement set
			s.updateCachedPodData(pod)
			q.Push(pod)
		} else {
			delete(podErrors, pod)
		}
	}
	UnfinishedWorkSeconds.Delete(map[string]string{ControllerLabel: injection.GetControllerName(ctx), schedulingIDLabel: string(s.uuid)})
	for _, m := range s.newNodeClaims {
		m.FinalizeScheduling()
	}

	return Results{
		NewNodeClaims: s.newNodeClaims,
		ExistingNodes: s.existingNodes,
		PodErrors:     podErrors,
	}, ctx.Err()
}

func (s *Scheduler) trySchedule(ctx context.Context, p *corev1.Pod) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := s.add(ctx, p)
		if err == nil {
			return nil
		}
		// We should only relax the pod's requirements when the error is not a reserved offering error because the pod may be
		// able to schedule later without relaxing constraints. This could occur in this scheduling run, if other NodeClaims
		// release the required reservations when constrained, or in subsequent runs. For an example, reference the following
		// test: "shouldn't relax preferences when a pod fails to schedule due to a reserved offering error".
		if IsReservedOfferingError(err) {
			return err
		}
		// DRA errors are permanent while the IgnoreDRARequests flag is enabled, so we shouldn't attempt to relax
		// pod requirements as we don't want to schedule the pod.
		if IsDRAError(err) {
			return err
		}
		// Eventually we won't be able to relax anymore and this while loop will exit
		if relaxed := s.preferences.Relax(ctx, p); !relaxed {
			return err
		}
		if e := s.topology.Update(ctx, p); e != nil && !errors.Is(e, context.DeadlineExceeded) {
			log.FromContext(ctx).Error(e, "failed updating topology")
		}
		// Update the cached podData since the pod was relaxed, and it could have changed its requirement set
		s.updateCachedPodData(p)
	}
}

func (s *Scheduler) updateCachedPodData(p *corev1.Pod) {
	var requirements scheduling.Requirements
	if s.preferencePolicy == PreferencePolicyIgnore {
		requirements = scheduling.NewStrictPodRequirements(p)
	} else {
		requirements = scheduling.NewPodRequirements(p)
	}
	strictRequirements := requirements
	if scheduling.HasPreferredNodeAffinity(p) {
		// strictPodRequirements is important as it ensures we don't inadvertently restrict the possible pod domains by a
		// preferred node affinity.  Only required node affinities can actually reduce pod domains.
		strictRequirements = scheduling.NewStrictPodRequirements(p)
	}
	s.cachedPodData[p.UID] = &PodData{
		Requests:                 resources.RequestsForPods(p),
		Requirements:             requirements,
		StrictRequirements:       strictRequirements,
		HasResourceClaimRequests: pod.HasDRARequirements(p),
	}
}

func (s *Scheduler) add(ctx context.Context, pod *corev1.Pod) error {
	// Check if pod has DRA requirements - if so, return DRA error when IgnoreDRARequests is enabled
	if s.cachedPodData[pod.UID].HasResourceClaimRequests && karpopts.FromContext(ctx).IgnoreDRARequests {
		return NewDRAError(fmt.Errorf("pod has Dynamic Resource Allocation requirements that are not yet supported by Karpenter"))
	}

	// first try to schedule against an in-flight real node
	if err := s.addToExistingNode(ctx, pod); err == nil {
		return nil
	}
	// Consider using https://pkg.go.dev/container/heap
	sort.Slice(s.newNodeClaims, func(a, b int) bool { return len(s.newNodeClaims[a].Pods) < len(s.newNodeClaims[b].Pods) })

	// Pick existing node that we are about to create
	if err := s.addToInflightNode(ctx, pod); err == nil {
		return nil
	}
	if len(s.nodeClaimTemplates) == 0 {
		return fmt.Errorf("nodepool requirements filtered out all available instance types")
	}
	err := s.addToNewNodeClaim(ctx, pod)
	if err == nil {
		return nil
	}
	return err
}

func (s *Scheduler) addToExistingNode(ctx context.Context, pod *corev1.Pod) error {
	idx := math.MaxInt
	var mu sync.Mutex

	var existingNode *ExistingNode
	var requirements scheduling.Requirements

	// determine the volumes that will be mounted if the pod schedules
	volumes, err := scheduling.GetVolumes(ctx, s.kubeClient, pod)
	if err != nil {
		return err
	}
	parallelizeUntil(s.numConcurrentReconciles, len(s.existingNodes), func(i int) bool {
		r, err := s.existingNodes[i].CanAdd(pod, s.cachedPodData[pod.UID], volumes)
		if err == nil {
			mu.Lock()
			defer mu.Unlock()

			// Ensure that we always take an earlier successful schedule to keep consistent ordering
			if i >= idx {
				return false
			}
			existingNode = s.existingNodes[i]
			requirements = r
			idx = i
			return false
		}
		return true
	})
	// If we set the existingNode to something valid, this means that we successfully scheduled to one of these nodes
	if existingNode != nil {
		existingNode.Add(pod, s.cachedPodData[pod.UID], requirements, volumes)
		return nil
	}
	return fmt.Errorf("failed scheduling pod to existing nodes")
}

func (s *Scheduler) addToInflightNode(ctx context.Context, pod *corev1.Pod) error {
	idx := math.MaxInt
	var mu sync.Mutex

	var inflightNodeClaim *NodeClaim
	var updatedRequirements scheduling.Requirements
	var updatedInstanceTypes []*cloudprovider.InstanceType
	var offeringsToReserve []*cloudprovider.Offering
	parallelizeUntil(s.numConcurrentReconciles, len(s.newNodeClaims), func(i int) bool {
		r, its, ofr, err := s.newNodeClaims[i].CanAdd(ctx, pod, s.cachedPodData[pod.UID], false)
		if err == nil {
			mu.Lock()
			defer mu.Unlock()

			// Ensure that we always take an earlier successful schedule to keep consistent ordering
			if i >= idx {
				return false
			}
			inflightNodeClaim = s.newNodeClaims[i]
			updatedRequirements = r
			updatedInstanceTypes = its
			offeringsToReserve = ofr
			idx = i
			return false
		}
		return true
	})
	if inflightNodeClaim != nil {
		inflightNodeClaim.Add(pod, s.cachedPodData[pod.UID], updatedRequirements, updatedInstanceTypes, offeringsToReserve)
		return nil
	}
	return fmt.Errorf("failed scheduling pod to inflight nodes")
}

//nolint:gocyclo
func (s *Scheduler) addToNewNodeClaim(ctx context.Context, pod *corev1.Pod) error {
	idx := math.MaxInt
	var mu sync.Mutex

	var newNodeClaim *NodeClaim
	var updatedRequirements scheduling.Requirements
	var updatedInstanceTypes []*cloudprovider.InstanceType
	var offeringsToReserve []*cloudprovider.Offering

	errs := make([]error, len(s.nodeClaimTemplates))
	parallelizeUntil(s.numConcurrentReconciles, len(s.nodeClaimTemplates), func(i int) bool {
		its := s.nodeClaimTemplates[i].InstanceTypeOptions
		// if limits have been applied to the nodepool, ensure we filter instance types to avoid violating those limits
		if remaining, ok := s.remainingResources[s.nodeClaimTemplates[i].NodePoolName]; ok {
			its = filterByRemainingResources(its, remaining)
			if len(its) == 0 {
				errs[i] = serrors.Wrap(fmt.Errorf("all available instance types exceed limits for nodepool"), "NodePool", klog.KRef("", s.nodeClaimTemplates[i].NodePoolName))
				return true
			} else if len(s.nodeClaimTemplates[i].InstanceTypeOptions) != len(its) {
				log.FromContext(ctx).V(1).WithValues(
					"NodePool", klog.KRef("", s.nodeClaimTemplates[i].NodePoolName),
				).Info(fmt.Sprintf(
					"%d out of %d instance types were excluded because they would breach limits",
					len(s.nodeClaimTemplates[i].InstanceTypeOptions)-len(its),
					len(s.nodeClaimTemplates[i].InstanceTypeOptions),
				))
			}
		}
		nodeClaim := NewNodeClaim(s.nodeClaimTemplates[i], s.topology, s.daemonOverhead[s.nodeClaimTemplates[i]], s.daemonHostPortUsage[s.nodeClaimTemplates[i]], its, s.reservationManager, s.reservedOfferingMode)
		r, its, ofs, err := nodeClaim.CanAdd(ctx, pod, s.cachedPodData[pod.UID], s.minValuesPolicy == karpopts.MinValuesPolicyBestEffort)
		if err != nil {
			errs[i] = err

			// If the pod is compatible with a NodePool with reserved offerings available, we shouldn't fall back to a NodePool
			// with a lower weight. We could consider allowing "fallback" to NodePools with equal weight if they also have
			// reserved capacity in the future if scheduling latency becomes an issue.
			if IsReservedOfferingError(err) {
				mu.Lock()
				defer mu.Unlock()

				// A reserved offering error means that any subsequent successful after this NodeClaimTemplate isn't valid
				if i >= idx {
					return false
				}
				newNodeClaim = nil
				updatedRequirements = nil
				updatedInstanceTypes = nil
				offeringsToReserve = nil
				idx = i
				return false
			}
			return true
		}
		mu.Lock()
		defer mu.Unlock()

		// Ensure that we always take an earlier successful schedule to keep consistent ordering
		// We care about this particularly with NewNodeClaims because NodeClaims should be evaluated by weight
		if i >= idx {
			return false
		}

		_, minValuesRelaxed := lo.Find(nodeClaim.Requirements.Keys().UnsortedList(), func(k string) bool {
			updated := r.Get(k).MinValues
			original := nodeClaim.Requirements.Get(k).MinValues
			return original != nil && updated != nil && lo.FromPtr(updated) < lo.FromPtr(original)
		})
		if minValuesRelaxed {
			nodeClaim.Annotations[v1.NodeClaimMinValuesRelaxedAnnotationKey] = "true"
		} else {
			nodeClaim.Annotations[v1.NodeClaimMinValuesRelaxedAnnotationKey] = "false"
		}

		newNodeClaim = nodeClaim
		updatedRequirements = r
		updatedInstanceTypes = its
		offeringsToReserve = ofs
		idx = i
		return false
	})
	if newNodeClaim != nil {
		// we will launch this nodeClaim and need to track its maximum possible resource usage against our remaining resources
		newNodeClaim.Add(pod, s.cachedPodData[pod.UID], updatedRequirements, updatedInstanceTypes, offeringsToReserve)
		s.newNodeClaims = append(s.newNodeClaims, newNodeClaim)
		s.remainingResources[newNodeClaim.NodePoolName] = subtractMax(s.remainingResources[newNodeClaim.NodePoolName], newNodeClaim.InstanceTypeOptions)
		return nil
	}
	return multierr.Combine(errs...)
}

func (s *Scheduler) calculateExistingNodeClaims(ctx context.Context, stateNodes []*state.StateNode, daemonSetPods []*corev1.Pod) {
	// create our existing nodes
	for _, node := range stateNodes {
		taints := node.Taints()
		daemons := s.getCompatibleDaemonPods(ctx, node, taints, daemonSetPods)
		s.existingNodes = append(s.existingNodes, NewExistingNode(node, s.topology, taints, resources.RequestsForPods(daemons...)))
		s.updateRemainingResources(node)
	}
	s.sortExistingNodes()
}

// getCompatibleDaemonPods filters daemon pods that can schedule to the given node
func (s *Scheduler) getCompatibleDaemonPods(ctx context.Context, node *state.StateNode, taints []corev1.Taint, daemonSetPods []*corev1.Pod) []*corev1.Pod {
	var daemons []*corev1.Pod
	for _, p := range daemonSetPods {
		if s.shouldSkipDaemonPod(ctx, p) {
			continue
		}
		if s.isDaemonPodCompatibleWithNode(p, taints, node.Labels()) {
			daemons = append(daemons, p)
		}
	}
	return daemons
}

// shouldSkipDaemonPod checks if a daemon pod should be skipped due to DRA requirements
func (s *Scheduler) shouldSkipDaemonPod(ctx context.Context, p *corev1.Pod) bool {
	return pod.HasDRARequirements(p) && karpopts.FromContext(ctx).IgnoreDRARequests
}

// isDaemonPodCompatibleWithNode checks if a daemon pod is compatible with the node
func (s *Scheduler) isDaemonPodCompatibleWithNode(p *corev1.Pod, taints []corev1.Taint, nodeLabels map[string]string) bool {
	if err := scheduling.Taints(taints).ToleratesPod(p); err != nil {
		return false
	}
	if err := scheduling.NewLabelRequirements(nodeLabels).Compatible(scheduling.NewStrictPodRequirements(p)); err != nil {
		return false
	}
	return true
}

// updateRemainingResources updates the remaining resources for the node's nodepool
func (s *Scheduler) updateRemainingResources(node *state.StateNode) {
	// We don't use the status field and instead recompute the remaining resources to ensure we have a consistent view
	// of the cluster during scheduling.  Depending on how node creation falls out, this will also work for cases where
	// we don't create NodeClaim resources.
	if _, ok := s.remainingResources[node.Labels()[v1.NodePoolLabelKey]]; ok {
		s.remainingResources[node.Labels()[v1.NodePoolLabelKey]] = resources.Subtract(s.remainingResources[node.Labels()[v1.NodePoolLabelKey]], node.Capacity())
	}
}

// sortExistingNodes sorts existing nodes with initialized nodes first
func (s *Scheduler) sortExistingNodes() {
	// Order the existing nodes for scheduling with initialized nodes first
	// This is done specifically for consolidation where we want to make sure we schedule to initialized nodes
	// before we attempt to schedule uninitialized ones
	sort.SliceStable(s.existingNodes, func(i, j int) bool {
		if s.existingNodes[i].Initialized() && !s.existingNodes[j].Initialized() {
			return true
		}
		if !s.existingNodes[i].Initialized() && s.existingNodes[j].Initialized() {
			return false
		}
		return s.existingNodes[i].Name() < s.existingNodes[j].Name()
	})
}

// parallelizeUntil is an implementation of workqueue.ParallelizeUntil that modifies the
// doWorkPiece so that a worker always finishes its work when it pulls a piece off of pieces
// The function returns a bool that represents whether the worker should continue doing work
// or whether the worker should finish
func parallelizeUntil(workers, pieces int, doWorkPiece func(int) bool) {
	toProcess := make(chan int, pieces)
	for i := range pieces {
		toProcess <- i
	}
	close(toProcess)
	if pieces < workers {
		workers = pieces
	}
	wg := sync.WaitGroup{}
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for work := range toProcess {
				if !doWorkPiece(work) {
					return
				}
			}
		}()
	}
	wg.Wait()
}

// getDaemonOverhead determines the overhead for each NodeClaimTemplate required for daemons to schedule for any node provisioned by the NodeClaimTemplate
func getDaemonOverhead(ctx context.Context, nodeClaimTemplates []*NodeClaimTemplate, daemonSetPods []*corev1.Pod) map[*NodeClaimTemplate]corev1.ResourceList {
	return lo.SliceToMap(nodeClaimTemplates, func(nct *NodeClaimTemplate) (*NodeClaimTemplate, corev1.ResourceList) {
		return nct, resources.RequestsForPods(lo.Filter(daemonSetPods, func(p *corev1.Pod, _ int) bool {
			// Exclude daemon pods with DRA requirements when IgnoreDRARequests is enabled
			if pod.HasDRARequirements(p) && karpopts.FromContext(ctx).IgnoreDRARequests {
				return false
			}
			return isDaemonPodCompatible(nct, p)
		})...)
	})
}

// getDaemonHostPortUsage determines requested host ports for DaemonSet pods, given a NodeClaimTemplate
func getDaemonHostPortUsage(ctx context.Context, nodeClaimTemplates []*NodeClaimTemplate, daemonSetPods []*corev1.Pod) map[*NodeClaimTemplate]*scheduling.HostPortUsage {
	nctToOccupiedPorts := map[*NodeClaimTemplate]*scheduling.HostPortUsage{}
	for _, nct := range nodeClaimTemplates {
		hostPortUsage := scheduling.NewHostPortUsage()
		// gather compatible DaemonSet pods for the NodeClaimTemplate
		for _, pod := range lo.Filter(daemonSetPods, func(p *corev1.Pod, _ int) bool {
			// Exclude daemon pods with DRA requirements when IgnoreDRARequests is enabled
			if pod.HasDRARequirements(p) && karpopts.FromContext(ctx).IgnoreDRARequests {
				return false
			}
			return isDaemonPodCompatible(nct, p)
		}) {
			hostPortUsage.Add(pod, scheduling.GetHostPorts(pod))
		}
		nctToOccupiedPorts[nct] = hostPortUsage
	}
	return nctToOccupiedPorts
}

// isDaemonPodCompatible determines if the daemon pod is compatible with the NodeClaimTemplate for daemon scheduling
func isDaemonPodCompatible(nodeClaimTemplate *NodeClaimTemplate, pod *corev1.Pod) bool {
	preferences := &Preferences{}
	// Add a toleration for PreferNoSchedule since a daemon pod shouldn't respect the preference
	_ = preferences.toleratePreferNoScheduleTaints(pod)
	if err := scheduling.Taints(nodeClaimTemplate.Spec.Taints).ToleratesPod(pod); err != nil {
		return false
	}
	for {
		// We don't consider pod preferences for scheduling requirements since we know that pod preferences won't matter with Daemonset scheduling
		if nodeClaimTemplate.Requirements.IsCompatible(scheduling.NewStrictPodRequirements(pod), scheduling.AllowUndefinedWellKnownLabels) {
			return true
		}
		// If relaxing the Node Affinity term didn't succeed, then this DaemonSet can't schedule to this NodePool
		// We don't consider other forms of relaxation here since we don't consider pod affinities/anti-affinities
		// when considering DaemonSet schedulability
		if preferences.removeRequiredNodeAffinityTerm(pod) == nil {
			return false
		}
	}
}

// subtractMax returns the remaining resources after subtracting the max resource quantity per instance type. To avoid
// overshooting out, we need to pessimistically assume that if e.g. we request a 2, 4 or 8 CPU instance type
// that the 8 CPU instance type is all that will be available.  This could cause a batch of pods to take multiple rounds
// to schedule.
func subtractMax(remaining corev1.ResourceList, instanceTypes []*cloudprovider.InstanceType) corev1.ResourceList {
	// shouldn't occur, but to be safe
	if len(instanceTypes) == 0 {
		return remaining
	}
	var allInstanceResources []corev1.ResourceList
	for _, it := range instanceTypes {
		allInstanceResources = append(allInstanceResources, it.Capacity)
	}
	result := corev1.ResourceList{}
	itResources := resources.MaxResources(allInstanceResources...)
	for k, v := range remaining {
		cp := v.DeepCopy()
		cp.Sub(itResources[k])
		result[k] = cp
	}
	return result
}

// filterByRemainingResources is used to filter out instance types that if launched would exceed the nodepool limits
func filterByRemainingResources(instanceTypes []*cloudprovider.InstanceType, remaining corev1.ResourceList) []*cloudprovider.InstanceType {
	var filtered []*cloudprovider.InstanceType
	for _, it := range instanceTypes {
		itResources := it.Capacity
		viableInstance := true
		for resourceName, remainingQuantity := range remaining {
			// if the instance capacity is greater than the remaining quantity for this resource
			if resources.Cmp(itResources[resourceName], remainingQuantity) > 0 {
				viableInstance = false
			}
		}
		if viableInstance {
			filtered = append(filtered, it)
		}
	}
	return filtered
}
