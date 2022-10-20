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

package consolidation

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/scheduling"

	nodeutils "github.com/aws/karpenter-core/pkg/utils/node"
	"github.com/aws/karpenter-core/pkg/utils/pod"
	"github.com/aws/karpenter/pkg/cloudproviders/common/cloudprovider"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	pscheduling "github.com/aws/karpenter/pkg/controllers/provisioning/scheduling"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/events"
	"github.com/aws/karpenter/pkg/metrics"
)

// Controller is the consolidation controller.  It is not a standard controller-runtime controller in that it doesn't
// have a reconcile method.
type Controller struct {
	kubeClient             client.Client
	cluster                *state.Cluster
	provisioner            *provisioning.Provisioner
	recorder               events.Recorder
	clock                  clock.Clock
	cloudProvider          cloudprovider.CloudProvider
	lastConsolidationState int64
}

// pollingPeriod that we inspect cluster to look for opportunities to consolidate
const pollingPeriod = 10 * time.Second

var errCandidateNodeDeleting = fmt.Errorf("candidate node is deleting")

// waitRetryOptions are the retry options used when waiting on a node to become ready or to be deleted
// readiness can take some time as the node needs to come up, have any daemonset extended resoruce plugins register, etc.
// deletion can take some time in the case of restrictive PDBs that throttle the rate at which the node is drained
var waitRetryOptions = []retry.Option{
	retry.Delay(2 * time.Second),
	retry.LastErrorOnly(true),
	retry.Attempts(60),
	retry.MaxDelay(10 * time.Second), // 22 + (60-5)*10 =~ 9.5 minutes in total
}

func NewController(clk clock.Clock, kubeClient client.Client, provisioner *provisioning.Provisioner,
	cp cloudprovider.CloudProvider, recorder events.Recorder, cluster *state.Cluster) *Controller {
	return &Controller{
		clock:         clk,
		kubeClient:    kubeClient,
		cluster:       cluster,
		provisioner:   provisioner,
		recorder:      recorder,
		cloudProvider: cp,
	}
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("consolidation"))
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-m.Elected():
			for {
				select {
				case <-ctx.Done():
					logging.FromContext(ctx).Infof("Shutting down")
					return
				case <-c.clock.After(pollingPeriod):
					_, _ = c.Reconcile(ctx, reconcile.Request{})
				}
			}
		}
	}()
	return nil
}

func (c *Controller) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	// the last cluster consolidation wasn't able to improve things and nothing has changed regarding
	// the cluster that makes us think we would be successful now
	if c.lastConsolidationState == c.cluster.ClusterConsolidationState() {
		return reconcile.Result{}, nil
	}

	clusterState := c.cluster.ClusterConsolidationState()
	result, err := c.ProcessCluster(ctx)
	if err != nil {
		logging.FromContext(ctx).Errorf("consolidating cluster, %s", err)
	} else if result == DeprovisioningResultNothingToDo {
		c.lastConsolidationState = clusterState
	}

	return reconcile.Result{}, nil
}

// candidateNode is a node that we are considering for consolidation along with extra information to be used in
// making that determination
type candidateNode struct {
	*v1.Node
	instanceType   cloudprovider.InstanceType
	capacityType   string
	zone           string
	provisioner    *v1alpha5.Provisioner
	disruptionCost float64
	pods           []*v1.Pod
}

// ProcessCluster is exposed for unit testing purposes
func (c *Controller) ProcessCluster(ctx context.Context) (DeprovisioningResult, error) {

	// range over the different lifecycle methods. We'll only let one method perform an action
	for _, fn := range []func(ctx context.Context, candidates []candidateNode) (DeprovisioningResult, error){
		c.deleteEmptyNodes,
		c.consolidateNodes,
	} {
		// capture new cluster state before each attempt
		candidates, err := c.candidateNodes(ctx)
		if err != nil {
			return DeprovisioningResultFailed, fmt.Errorf("determining candidate nodes, %w", err)
		}
		if len(candidates) == 0 {
			return DeprovisioningResultNothingToDo, nil
		}

		result, err := fn(ctx, candidates)
		if err != nil {
			return DeprovisioningResultFailed, err
		}
		// only go on to the next lifecycle action if the previous did nothing
		if result == DeprovisioningResultNothingToDo {
			continue
		}
		return result, nil
	}
	return DeprovisioningResultNothingToDo, nil
}

// consolidateNodes looks at the non-empty nodes and executes the first viable consolidation action
func (c *Controller) consolidateNodes(ctx context.Context, candidates []candidateNode) (DeprovisioningResult, error) {
	pdbs, err := NewPDBLimits(ctx, c.kubeClient)
	if err != nil {
		return DeprovisioningResultFailed, fmt.Errorf("tracking PodDisruptionBudgets, %w", err)
	}

	// the remaining nodes are all non-empty, so we just consolidate the first one that we can
	sort.Slice(candidates, byNodeDisruptionCost(candidates))
	validationFailed := false
	for _, node := range candidates {
		// is this a node that we can terminate?  This check is meant to be fast so we can save the expense of simulated
		// scheduling unless its really needed
		if err = c.canBeTerminated(node, pdbs); err != nil {
			continue
		}

		cmd, err := c.computeNodeConsolidationOption(ctx, node)
		if err != nil {
			logging.FromContext(ctx).Errorf("calculating consolidation option, %s", err)
			continue
		}
		if cmd.action == deprovisionActionDelete || cmd.action == deprovisionActionReplace {
			isValid, err := c.validateCommand(ctx, cmd)
			if err != nil {
				logging.FromContext(ctx).Errorf("validating command, %s", err)
				continue
			}
			if !isValid {
				validationFailed = true
				logging.FromContext(ctx).Debugf("skipping consolidation %s due to failing validation", cmd)
				continue
			}
			// perform the first consolidation we can since we are looking at nodes in ascending order of disruption cost
			c.executeCommand(ctx, cmd)
			return DeprovisioningResultSuccess, nil
		}
	}
	// if any validation failed the cluster state is in flux so we want to retry instead of waiting on the cluster
	// state to change again.
	if validationFailed {
		return DeprovisioningResultRetry, nil
	}
	return DeprovisioningResultNothingToDo, nil
}

// deleteEmptyNodes identifies any empty nodes and if possible, deletes them.
func (c *Controller) deleteEmptyNodes(ctx context.Context, candidates []candidateNode) (DeprovisioningResult, error) {
	emptyNodes := lo.Filter(candidates, func(n candidateNode, _ int) bool { return len(n.pods) == 0 })
	// first see if there are empty nodes that we can delete immediately, and if so delete them all at once
	if len(emptyNodes) == 0 {
		return DeprovisioningResultNothingToDo, nil
	}

	cmd := lifecycleCommand{
		nodesToRemove: lo.Map(emptyNodes, func(n candidateNode, _ int) *v1.Node { return n.Node }),
		action:        deprovisionActionDeleteEmpty,
		created:       c.clock.Now(),
	}
	isValid, err := c.validateCommand(ctx, cmd)
	if err != nil {
		return DeprovisioningResultFailed, fmt.Errorf("validating command, %w", err)
	}
	if !isValid {
		return DeprovisioningResultRetry, nil
	}

	c.executeCommand(ctx, cmd)
	return DeprovisioningResultSuccess, nil
}

// candidateNodes returns nodes that appear to be currently consolidatable based off of their provisioner
// nolint:gocyclo
func (c *Controller) candidateNodes(ctx context.Context) ([]candidateNode, error) {
	provisioners, instanceTypesByProvisioner, err := c.buildProvisionerMap(ctx)
	if err != nil {
		return nil, err
	}

	var nodes []candidateNode
	c.cluster.ForEachNode(func(n *state.Node) bool {
		var provisioner *v1alpha5.Provisioner
		var instanceTypeMap map[string]cloudprovider.InstanceType
		if provName, ok := n.Node.Labels[v1alpha5.ProvisionerNameLabelKey]; ok {
			provisioner = provisioners[provName]
			instanceTypeMap = instanceTypesByProvisioner[provName]
		}
		// skip any nodes that are already marked for deletion and being handled
		if n.MarkedForDeletion {
			return true
		}
		// skip any nodes where we can't determine the provisioner
		if provisioner == nil || instanceTypeMap == nil {
			return true
		}

		instanceType := instanceTypeMap[n.Node.Labels[v1.LabelInstanceTypeStable]]
		// skip any nodes that we can't determine the instance of or for which we don't have consolidation enabled
		if instanceType == nil || provisioner.Spec.Consolidation == nil || !ptr.BoolValue(provisioner.Spec.Consolidation.Enabled) {
			return true
		}

		// skip any nodes that we can't determine the capacity type or the topology zone for
		ct, ok := n.Node.Labels[v1alpha5.LabelCapacityType]
		if !ok {
			return true
		}
		az, ok := n.Node.Labels[v1.LabelTopologyZone]
		if !ok {
			return true
		}

		// Skip nodes that aren't initialized
		if n.Node.Labels[v1alpha5.LabelNodeInitialized] != "true" {
			return true
		}

		// Skip the node if it is nominated by a recent provisioning pass to be the target of a pending pod.
		if c.cluster.IsNodeNominated(n.Node.Name) {
			return true
		}

		// skip nodes that are annotated as do-not-consolidate
		if n.Node.Annotations[v1alpha5.DoNotConsolidateNodeAnnotationKey] == "true" {
			return true
		}

		pods, err := nodeutils.GetNodePods(ctx, c.kubeClient, n.Node)
		if err != nil {
			logging.FromContext(ctx).Errorf("Determining node pods, %s", err)
			return true
		}

		cn := candidateNode{
			Node:           n.Node,
			instanceType:   instanceType,
			capacityType:   ct,
			zone:           az,
			provisioner:    provisioner,
			pods:           pods,
			disruptionCost: disruptionCost(ctx, pods),
		}
		// lifetimeRemaining is the fraction of node lifetime remaining in the range [0.0, 1.0].  If the TTLSecondsUntilExpired
		// is non-zero, we use it to scale down the disruption costs of nodes that are going to expire.  Just after creation, the
		// disruption cost is highest and it approaches zero as the node ages towards its expiration time.
		lifetimeRemaining := c.calculateLifetimeRemaining(cn)
		cn.disruptionCost *= lifetimeRemaining

		nodes = append(nodes, cn)
		return true
	})

	return nodes, nil
}

// buildProvisionerMap builds a provName -> provisioner map and a provName -> instanceName -> instance type map
func (c *Controller) buildProvisionerMap(ctx context.Context) (map[string]*v1alpha5.Provisioner, map[string]map[string]cloudprovider.InstanceType, error) {
	provisioners := map[string]*v1alpha5.Provisioner{}
	var provList v1alpha5.ProvisionerList
	if err := c.kubeClient.List(ctx, &provList); err != nil {
		return nil, nil, fmt.Errorf("listing provisioners, %w", err)
	}
	instanceTypesByProvisioner := map[string]map[string]cloudprovider.InstanceType{}
	for i := range provList.Items {
		p := &provList.Items[i]
		provisioners[p.Name] = p

		provInstanceTypes, err := c.cloudProvider.GetInstanceTypes(ctx, p)
		if err != nil {
			return nil, nil, fmt.Errorf("listing instance types for %s, %w", p.Name, err)
		}
		instanceTypesByProvisioner[p.Name] = map[string]cloudprovider.InstanceType{}
		for _, it := range provInstanceTypes {
			instanceTypesByProvisioner[p.Name][it.Name()] = it
		}
	}
	return provisioners, instanceTypesByProvisioner, nil
}

func (c *Controller) executeCommand(ctx context.Context, action lifecycleCommand) {
	if action.action != deprovisionActionDelete &&
		action.action != deprovisionActionReplace &&
		action.action != deprovisionActionDeleteEmpty {
		logging.FromContext(ctx).Errorf("Invalid disruption action calculated: %s", action.action)
		return
	}

	consolidationActionsPerformedCounter.With(prometheus.Labels{"action": action.action.String()}).Add(1)

	// action's stringer
	logging.FromContext(ctx).Infof("Consolidating via %s", action.String())

	if action.action == deprovisionActionReplace {
		if err := c.launchReplacementNode(ctx, action); err != nil {
			// If we failed to launch the replacement, don't consolidate.  If this is some permanent failure,
			// we don't want to disrupt workloads with no way to provision new nodes for them.
			logging.FromContext(ctx).Errorf("Launching replacement node, %s", err)
			return
		}
	}

	for _, oldNode := range action.nodesToRemove {
		c.recorder.TerminatingNodeForConsolidation(oldNode, action.String())
		if err := c.kubeClient.Delete(ctx, oldNode); err != nil {
			logging.FromContext(ctx).Errorf("Deleting node, %s", err)
		} else {
			metrics.NodesTerminatedCounter.WithLabelValues(metrics.ConsolidationReason).Inc()
		}
	}

	// We wait for nodes to delete to ensure we don't start another round of consolidation until this node is fully
	// deleted.
	for _, oldnode := range action.nodesToRemove {
		c.waitForDeletion(ctx, oldnode)
	}
}

// waitForDeletion waits for the specified node to be removed from the API server. This deletion can take some period
// of time if there are PDBs that govern pods on the node as we need to  wait until the node drains before
// it's actually deleted.
func (c *Controller) waitForDeletion(ctx context.Context, node *v1.Node) {
	if err := retry.Do(func() error {
		var n v1.Node
		nerr := c.kubeClient.Get(ctx, client.ObjectKey{Name: node.Name}, &n)
		// We expect the not node found error, at which point we know the node is deleted.
		if apierrors.IsNotFound(nerr) {
			return nil
		}
		// make the user aware of why consolidation is paused
		c.recorder.WaitingOnDeletionForConsolidation(node)
		if nerr != nil {
			return fmt.Errorf("expected node to be not found, %w", nerr)
		}
		// the node still exists
		return fmt.Errorf("expected node to be not found")
	}, waitRetryOptions...,
	); err != nil {
		logging.FromContext(ctx).Errorf("Waiting on node deletion, %s", err)
	}
}

func byNodeDisruptionCost(nodes []candidateNode) func(i int, j int) bool {
	return func(a, b int) bool {
		if nodes[a].disruptionCost == nodes[b].disruptionCost {
			// if costs are equal, choose the older node
			return nodes[a].CreationTimestamp.Before(&nodes[b].CreationTimestamp)
		}
		return nodes[a].disruptionCost < nodes[b].disruptionCost
	}
}

// launchReplacementNode launches a replacement node and blocks until it is ready
func (c *Controller) launchReplacementNode(ctx context.Context, action lifecycleCommand) error {
	defer metrics.Measure(consolidationReplacementNodeInitializedHistogram)()
	if len(action.nodesToRemove) != 1 {
		return fmt.Errorf("expected a single node to replace, found %d", len(action.nodesToRemove))
	}
	oldNode := action.nodesToRemove[0]

	// cordon the node before we launch the replacement to prevent new pods from scheduling to the node
	if err := c.setNodeUnschedulable(ctx, action.nodesToRemove[0].Name, true); err != nil {
		return fmt.Errorf("cordoning node %s, %w", oldNode.Name, err)
	}

	nodeNames, err := c.provisioner.LaunchNodes(ctx, provisioning.LaunchOptions{RecordPodNomination: false}, action.replacementNode)
	if err != nil {
		// uncordon the node as the launch may fail (e.g. ICE or incompatible AMI)
		err = multierr.Append(err, c.setNodeUnschedulable(ctx, oldNode.Name, false))
		return err
	}
	if len(nodeNames) != 1 {
		// shouldn't ever occur as we are only launching a single node
		return fmt.Errorf("expected a single node name, got %d", len(nodeNames))
	}

	metrics.NodesCreatedCounter.WithLabelValues(metrics.ConsolidationReason).Inc()

	// We have the new node created at the API server so mark the old node for deletion
	c.cluster.MarkForDeletion(oldNode.Name)

	var k8Node v1.Node
	// Wait for the node to be ready
	var once sync.Once
	if err := retry.Do(func() error {
		if err := c.kubeClient.Get(ctx, client.ObjectKey{Name: nodeNames[0]}, &k8Node); err != nil {
			return fmt.Errorf("getting node, %w", err)
		}
		once.Do(func() {
			c.recorder.LaunchingNodeForConsolidation(&k8Node, action.String())
		})

		if _, ok := k8Node.Labels[v1alpha5.LabelNodeInitialized]; !ok {
			// make the user aware of why consolidation is paused
			c.recorder.WaitingOnReadinessForConsolidation(&k8Node)
			return fmt.Errorf("node is not initialized")
		}
		return nil
	}, waitRetryOptions...); err != nil {
		// node never become ready, so uncordon the node we were trying to delete and report the error
		c.cluster.UnmarkForDeletion(oldNode.Name)
		return multierr.Combine(c.setNodeUnschedulable(ctx, oldNode.Name, false),
			fmt.Errorf("timed out checking node readiness, %w", err))
	}
	return nil
}

func (c *Controller) canBeTerminated(node candidateNode, pdbs *PDBLimits) error {
	if !node.DeletionTimestamp.IsZero() {
		return fmt.Errorf("already being deleted")
	}
	if !pdbs.CanEvictPods(node.pods) {
		return fmt.Errorf("not eligible for termination due to PDBs")
	}
	return c.podsPreventEviction(node)
}

func (c *Controller) podsPreventEviction(node candidateNode) error {
	for _, p := range node.pods {
		// don't care about pods that are finishing, finished or owned by the node
		if pod.IsTerminating(p) || pod.IsTerminal(p) || pod.IsOwnedByNode(p) {
			continue
		}

		if pod.HasDoNotEvict(p) {
			return fmt.Errorf("found do-not-evict pod")
		}

		if pod.IsNotOwned(p) {
			return fmt.Errorf("found pod with no controller")
		}
	}
	return nil
}

// calculateLifetimeRemaining calculates the fraction of node lifetime remaining in the range [0.0, 1.0].  If the TTLSecondsUntilExpired
// is non-zero, we use it to scale down the disruption costs of nodes that are going to expire.  Just after creation, the
// disruption cost is highest and it approaches zero as the node ages towards its expiration time.
func (c *Controller) calculateLifetimeRemaining(node candidateNode) float64 {
	remaining := 1.0
	if node.provisioner.Spec.TTLSecondsUntilExpired != nil {
		ageInSeconds := c.clock.Since(node.CreationTimestamp.Time).Seconds()
		totalLifetimeSeconds := float64(*node.provisioner.Spec.TTLSecondsUntilExpired)
		lifetimeRemainingSeconds := totalLifetimeSeconds - ageInSeconds
		remaining = clamp(0.0, lifetimeRemainingSeconds/totalLifetimeSeconds, 1.0)
	}
	return remaining
}

//nolint:gocyclo
func (c *Controller) computeNodeConsolidationOption(ctx context.Context, node candidateNode) (lifecycleCommand, error) {
	defer metrics.Measure(consolidationDurationHistogram.WithLabelValues("Replace/Delete"))()
	newNodes, allPodsScheduled, err := c.simulateScheduling(ctx, node)
	if err != nil {
		// if a candidate node is now deleting, just retry
		if errors.Is(err, errCandidateNodeDeleting) {
			return lifecycleCommand{action: deprovisionActionDoNothing}, nil
		}
		return lifecycleCommand{}, err
	}

	// if not all of the pods were scheduled, we can't do anything
	if !allPodsScheduled {
		return lifecycleCommand{action: deprovisionActionNotPossible}, nil
	}

	// were we able to schedule all the pods on the inflight nodes?
	if len(newNodes) == 0 {
		return lifecycleCommand{
			nodesToRemove: []*v1.Node{node.Node},
			action:        deprovisionActionDelete,
			created:       c.clock.Now(),
		}, nil
	}

	// we're not going to turn a single node into multiple nodes
	if len(newNodes) != 1 {
		return lifecycleCommand{action: deprovisionActionNotPossible}, nil
	}

	// get the current node price based on the offering
	// fallback if we can't find the specific zonal pricing data
	offering, ok := cloudprovider.GetOffering(node.instanceType, node.capacityType, node.zone)
	if !ok {
		return lifecycleCommand{action: deprovisionActionFailed}, fmt.Errorf("getting offering price from candidate node, %w", err)
	}
	newNodes[0].InstanceTypeOptions = filterByPrice(newNodes[0].InstanceTypeOptions, newNodes[0].Requirements, offering.Price)
	if len(newNodes[0].InstanceTypeOptions) == 0 {
		// no instance types remain after filtering by price
		return lifecycleCommand{action: deprovisionActionNotPossible}, nil
	}

	// If the existing node is spot and the replacement is spot, we don't consolidate.  We don't have a reliable
	// mechanism to determine if this replacement makes sense given instance type availability (e.g. we may replace
	// a spot node with one that is less available and more likely to be reclaimed).
	if node.capacityType == v1alpha5.CapacityTypeSpot &&
		newNodes[0].Requirements.Get(v1alpha5.LabelCapacityType).Has(v1alpha5.CapacityTypeSpot) {
		return lifecycleCommand{action: deprovisionActionNotPossible}, nil
	}

	// We are consolidating a node from OD -> [OD,Spot] but have filtered the instance types by cost based on the
	// assumption, that the spot variant will launch. We also need to add a requirement to the node to ensure that if
	// spot capacity is insufficient we don't replace the node with a more expensive on-demand node.  Instead the launch
	// should fail and we'll just leave the node alone.
	ctReq := newNodes[0].Requirements.Get(v1alpha5.LabelCapacityType)
	if ctReq.Has(v1alpha5.CapacityTypeSpot) && ctReq.Has(v1alpha5.CapacityTypeOnDemand) {
		newNodes[0].Requirements.Add(scheduling.NewRequirement(v1alpha5.LabelCapacityType, v1.NodeSelectorOpIn, v1alpha5.CapacityTypeSpot))
	}

	return lifecycleCommand{
		nodesToRemove:   []*v1.Node{node.Node},
		action:          deprovisionActionReplace,
		replacementNode: newNodes[0],
		created:         c.clock.Now(),
	}, nil
}

func (c *Controller) simulateScheduling(ctx context.Context, nodesToDelete ...candidateNode) (newNodes []*pscheduling.Node, allPodsScheduled bool, err error) {
	var stateNodes []*state.Node
	var markedForDeletionNodes []*state.Node
	candidateNodeIsDeleting := false
	candidateNodeNames := sets.NewString(lo.Map(nodesToDelete, func(t candidateNode, i int) string { return t.Name })...)
	c.cluster.ForEachNode(func(n *state.Node) bool {
		// not a candidate node
		if _, ok := candidateNodeNames[n.Node.Name]; !ok {
			if !n.MarkedForDeletion {
				stateNodes = append(stateNodes, n.DeepCopy())
			} else {
				markedForDeletionNodes = append(markedForDeletionNodes, n.DeepCopy())
			}
		} else if n.MarkedForDeletion {
			// candidate node and marked for deletion
			candidateNodeIsDeleting = true
		}
		return true
	})
	// We do one final check to ensure that the node that we are attempting to consolidate isn't
	// already handled for deletion by some other controller. This could happen if the node was markedForDeletion
	// between returning the candidateNodes and getting the stateNodes above
	if candidateNodeIsDeleting {
		return nil, false, errCandidateNodeDeleting
	}

	// We get the pods that are on nodes that are deleting
	deletingNodePods, err := nodeutils.GetNodePods(ctx, c.kubeClient, lo.Map(markedForDeletionNodes, func(n *state.Node, _ int) *v1.Node { return n.Node })...)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get pods from deleting nodes, %w", err)
	}
	var pods []*v1.Pod
	for _, n := range nodesToDelete {
		pods = append(pods, n.pods...)
	}
	pods = append(pods, deletingNodePods...)
	scheduler, err := c.provisioner.NewScheduler(ctx, pods, stateNodes, pscheduling.SchedulerOptions{
		SimulationMode: true,
	})

	if err != nil {
		return nil, false, fmt.Errorf("creating scheduler, %w", err)
	}

	newNodes, ifn, err := scheduler.Solve(ctx, pods)
	if err != nil {
		return nil, false, fmt.Errorf("simulating scheduling, %w", err)
	}

	podsScheduled := 0
	for _, n := range newNodes {
		podsScheduled += len(n.Pods)
	}
	for _, n := range ifn {
		podsScheduled += len(n.Pods)
	}

	return newNodes, podsScheduled == len(pods), nil
}

func (c *Controller) setNodeUnschedulable(ctx context.Context, nodeName string, isUnschedulable bool) error {
	var node v1.Node
	if err := c.kubeClient.Get(ctx, client.ObjectKey{Name: nodeName}, &node); err != nil {
		return fmt.Errorf("getting node, %w", err)
	}

	// node is being deleted already, so no need to un-cordon
	if !isUnschedulable && !node.DeletionTimestamp.IsZero() {
		return nil
	}

	// already matches the state we want to be in
	if node.Spec.Unschedulable == isUnschedulable {
		return nil
	}

	persisted := node.DeepCopy()
	node.Spec.Unschedulable = isUnschedulable
	if err := c.kubeClient.Patch(ctx, &node, client.MergeFrom(persisted)); err != nil {
		return fmt.Errorf("patching node %s, %w", node.Name, err)
	}
	return nil
}

// validateCommand is used to validate that a lifecycleCommand is still able to be performed by re-inspecting nodes and
// re-simulating scheduling
func (c *Controller) validateCommand(ctx context.Context, cmd lifecycleCommand) (bool, error) {
	// verifyDelay is how long we wait before re-validating our lifecycle command
	verifyDelay := 15 * time.Second
	// figure out how long we need to delay for verification based on when the command was constructed
	remainingDelay := verifyDelay - c.clock.Since(cmd.created)
	if remainingDelay > 0 {
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("context canceled")
		case <-c.clock.After(remainingDelay):
		}
	}

	// we've waited and now need to get the new cluster state
	nodes, err := c.mapNodes(ctx, cmd.nodesToRemove)
	if err != nil {
		return false, err
	}

	switch cmd.action {
	case deprovisionActionDeleteEmpty:
		// delete empty isn't quite a special case of replace as we only want to perform the action if the nodes are
		// empty, not if they just won't create a new node if deleted
		return c.validateDeleteEmpty(nodes)
	case deprovisionActionDelete, deprovisionActionReplace:
		// deletion is just a special case of replace where we don't launch a replacement node
		return c.validateReplace(ctx, nodes, cmd.replacementNode)
	default:
		return false, fmt.Errorf("unsupported action %s", cmd.action)
	}
}

// mapNodes maps from a list of *v1.Node to candidateNode
func (c *Controller) mapNodes(ctx context.Context, nodes []*v1.Node) ([]candidateNode, error) {
	verifyNodeNames := sets.NewString(lo.Map(nodes, func(t *v1.Node, i int) string { return t.Name })...)
	candidateNodes, err := c.candidateNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("determining candidate node, %w", err)
	}
	var ret []candidateNode
	for _, c := range candidateNodes {
		if verifyNodeNames.Has(c.Name) {
			ret = append(ret, c)
		}
	}
	return ret, nil
}

// validateDeleteEmpty validates that the given nodes are still empty
func (c *Controller) validateDeleteEmpty(nodesToDelete []candidateNode) (bool, error) {
	// the deletion of empty nodes is easy to validate, we just ensure that all the nodesToDelete are still empty and that
	// the node isn't a target of a recent scheduling simulation
	for _, n := range nodesToDelete {
		if len(n.pods) != 0 && !c.cluster.IsNodeNominated(n.Name) {
			return false, nil
		}
	}
	return true, nil
}

// validateReplace validates that given the nodes to delete and the replacement node, the replacement is still valid
func (c *Controller) validateReplace(ctx context.Context, nodesToDelete []candidateNode, replacementNode *pscheduling.Node) (bool, error) {
	newNodes, allPodsScheduled, err := c.simulateScheduling(ctx, nodesToDelete...)
	if err != nil {
		return false, fmt.Errorf("simluating scheduling, %w", err)
	}
	if !allPodsScheduled {
		return false, nil
	}

	// We want to ensure that the re-simulated scheduling using the current cluster state produces the same result.
	// There are three possible options for the number of new nodesToDelete that we need to handle:
	// len(newNodes) == 0, as long as we weren't expecting a new node, this is valid
	// len(newNodes) > 1, something in the cluster changed so that the nodesToDelete we were going to delete can no longer
	//                    be deleted without producing more than one node
	// len(newNodes) == 1, as long as the node looks like what we were expecting, this is valid
	if len(newNodes) == 0 {
		if replacementNode == nil {
			// scheduling produced zero new nodes and we weren't expecting any, so this is valid.
			return true, nil
		}
		// if it produced no new nodes, but we were expecting one we should re-simulate as there is likely a better
		// consolidation option now
		return false, nil
	}

	// we need more than one replacement node which is never valid currently (all of our node replacement is m->1, never m->n)
	if len(newNodes) > 1 {
		return false, nil
	}

	// we now know that scheduling simulation wants to create one new node
	if replacementNode == nil {
		// but we weren't expecting any new nodes, so this is invalid
		return false, nil
	}

	// We know that the scheduling simulation wants to create a new node and that the command we are verifying wants
	// to create a new node. The scheduling simulation doesn't apply any filtering to instance types, so it may include
	// instance types that we don't want to launch which were filtered out when the lifecycleCommand was created.  To
	// check if our lifecycleCommand is valid, we just want to ensure that the list of instance types we are considering
	// creating are a subset of what scheduling says we should create.
	//
	// This is necessary since consolidation only wants cheaper nodes.  Suppose consolidation determined we should delete
	// a 4xlarge and replace it with a 2xlarge. If things have changed and the scheduling simulation we just performed
	// now says that we need to launch a 4xlarge. It's still launching the correct number of nodes, but it's just
	// as expensive or possibly more so we shouldn't validate.
	if !instanceTypesAreSubset(replacementNode.InstanceTypeOptions, newNodes[0].InstanceTypeOptions) {
		return false, nil
	}

	// Now we know:
	// - current scheduling simulation says to create a new node with types T = {T_0, T_1, ..., T_n}
	// - our lifecycle command says to create a node with types {U_0, U_1, ..., U_n} where U is a subset of T
	return true, nil
}

// instanceTypesAreSubset returns true if the lhs slice of instance types are a subset of the rhs.
func instanceTypesAreSubset(lhs []cloudprovider.InstanceType, rhs []cloudprovider.InstanceType) bool {
	rhsNames := sets.NewString(lo.Map(rhs, func(t cloudprovider.InstanceType, i int) string { return t.Name() })...)
	for _, l := range lhs {
		if _, ok := rhsNames[l.Name()]; !ok {
			return false
		}
	}
	return true
}
