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
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/utils/clock"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/controllers/provisioning/scheduling"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/events"
	"github.com/aws/karpenter/pkg/metrics"
	"github.com/aws/karpenter/pkg/utils/pod"
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

// waitRetryOptions are the retry options used when waiting on a node to become ready or to be deleted
// readiness can take some time as the node needs to come up, have any daemonset extended resoruce plugins register, etc.
// deletion can take some time in the case of restrictive PDBs that throttle the rate at which the node is drained
var waitRetryOptions = []retry.Option{
	retry.Delay(2 * time.Second),
	retry.LastErrorOnly(true),
	retry.Attempts(60),
	retry.MaxDelay(10 * time.Second), // 22 + (60-5)*10 =~ 9.5 minutes in total
}

func NewController(ctx context.Context, clk clock.Clock, kubeClient client.Client, provisioner *provisioning.Provisioner,
	cp cloudprovider.CloudProvider, recorder events.Recorder, cluster *state.Cluster, startAsync <-chan struct{}) *Controller {
	c := &Controller{
		clock:         clk,
		kubeClient:    kubeClient,
		cluster:       cluster,
		provisioner:   provisioner,
		recorder:      recorder,
		cloudProvider: cp,
	}

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-startAsync:
			c.run(ctx)
		}
	}()

	return c
}

func (c *Controller) run(ctx context.Context) {
	logger := logging.FromContext(ctx).Named("consolidation")
	ctx = logging.WithLogger(ctx, logger)
	for {
		select {
		case <-ctx.Done():
			logger.Infof("Shutting down")
			return
		case <-time.After(pollingPeriod):
			// the last cluster consolidation wasn't able to improve things and nothing has changed regarding
			// the cluster that makes us think we would be successful now
			if c.lastConsolidationState == c.cluster.ClusterConsolidationState() {
				continue
			}

			// don't consolidate as we recently scaled down too soon
			stabilizationTime := c.clock.Now().Add(-c.stabilizationWindow(ctx))
			// capture the state before we process so if something changes during consolidation we'll re-look
			// immediately
			clusterState := c.cluster.ClusterConsolidationState()
			if c.cluster.LastNodeDeletionTime().Before(stabilizationTime) {
				result, err := c.ProcessCluster(ctx)
				if err != nil {
					logging.FromContext(ctx).Errorf("consolidating cluster, %s", err)
				} else if result == ProcessResultNothingToDo {
					c.lastConsolidationState = clusterState
				}
			}
		}
	}
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
func (c *Controller) ProcessCluster(ctx context.Context) (ProcessResult, error) {
	candidates, err := c.candidateNodes(ctx)
	if err != nil {
		return ProcessResultFailed, fmt.Errorf("determining candidate nodes, %w", err)
	}
	if len(candidates) == 0 {
		return ProcessResultNothingToDo, nil
	}

	emptyNodes := lo.Filter(candidates, func(n candidateNode, _ int) bool { return len(n.pods) == 0 })
	// first see if there are empty nodes that we can delete immediately, and if so delete them all at once
	if len(emptyNodes) > 0 {
		c.performConsolidation(ctx, consolidationAction{
			oldNodes: lo.Map(emptyNodes, func(n candidateNode, _ int) *v1.Node { return n.Node }),
			result:   consolidateResultDeleteEmpty,
		})
		return ProcessResultConsolidated, nil
	}

	pdbs, err := NewPDBLimits(ctx, c.kubeClient)
	if err != nil {
		return ProcessResultFailed, fmt.Errorf("tracking PodDisruptionBudgets, %w", err)
	}

	// the remaining nodes are all non-empty, so we just consolidate the first one that we can
	sort.Slice(candidates, byNodeDisruptionCost(candidates))
	for _, node := range candidates {
		// is this a node that we can terminate?  This check is meant to be fast so we can save the expense of simulated
		// scheduling unless its really needed
		if err = c.canBeTerminated(node, pdbs); err != nil {
			continue
		}

		action, err := c.nodeConsolidationOptionReplaceOrDelete(ctx, node)
		if err != nil {
			logging.FromContext(ctx).Errorf("calculating consolidation option, %s", err)
			continue
		}
		if action.result == consolidateResultDelete || action.result == consolidateResultReplace {
			// perform the first consolidation we can since we are looking at nodes in ascending order of disruption cost
			c.performConsolidation(ctx, action)
			return ProcessResultConsolidated, nil
		}
	}
	return ProcessResultNothingToDo, nil
}

// candidateNodes returns nodes that appear to be currently consolidatable based off of their provisioner
//
//gocyclo:ignore
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

		if c.cluster.IsNodeNominated(n.Node.Name) {
			return true
		}

		// skip nodes that are annotated as do-not-consolidate
		if n.Node.Annotations[v1alpha5.DoNotConsolidateNodeAnnotationKey] == "true" {
			return true
		}

		// skip nodes that the scheduler thinks will soon have pending pods bound to them
		if c.cluster.IsNodeNominated(n.Node.Name) {
			return true
		}

		pods, err := c.getNodePods(ctx, n.Node.Name)
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

func (c *Controller) performConsolidation(ctx context.Context, action consolidationAction) {
	if action.result != consolidateResultDelete &&
		action.result != consolidateResultReplace &&
		action.result != consolidateResultDeleteEmpty {
		logging.FromContext(ctx).Errorf("Invalid disruption action calculated: %s", action.result)
		return
	}

	consolidationActionsPerformedCounter.With(prometheus.Labels{"action": action.result.String()}).Add(1)

	// action's stringer
	logging.FromContext(ctx).Infof("Consolidating via %s", action.String())

	if action.result == consolidateResultReplace {
		if err := c.launchReplacementNode(ctx, action); err != nil {
			// If we failed to launch the replacement, don't consolidate.  If this is some permanent failure,
			// we don't want to disrupt workloads with no way to provision new nodes for them.
			logging.FromContext(ctx).Errorf("Launching replacement node, %s", err)
			return
		}
	}

	for _, oldNode := range action.oldNodes {
		c.recorder.TerminatingNodeForConsolidation(oldNode, action.String())
		if err := c.kubeClient.Delete(ctx, oldNode); err != nil {
			logging.FromContext(ctx).Errorf("Deleting node, %s", err)
		} else {
			consolidationNodesTerminatedCounter.Add(1)
		}
	}

	// We wait for nodes to delete to ensure we don't start another round of consolidation until this node is fully
	// deleted.
	for _, oldnode := range action.oldNodes {
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
		if errors.IsNotFound(nerr) {
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
func (c *Controller) launchReplacementNode(ctx context.Context, minCost consolidationAction) error {
	defer metrics.Measure(consolidationReplacementNodeInitializedHistogram)()
	if len(minCost.oldNodes) != 1 {
		return fmt.Errorf("expected a single node to replace, found %d", len(minCost.oldNodes))
	}

	// cordon the node before we launch the replacement to prevent new pods from scheduling to the node
	if err := c.setNodeUnschedulable(ctx, minCost.oldNodes[0].Name, true); err != nil {
		return fmt.Errorf("cordoning node %s, %w", minCost.oldNodes[0].Name, err)
	}

	nodeNames, err := c.provisioner.LaunchNodes(ctx, provisioning.LaunchOptions{RecordPodNomination: false}, minCost.replacementNode)
	if err != nil {
		// uncordon the node as the launch may fail (e.g. ICE or incompatible AMI)
		err = multierr.Append(err, c.setNodeUnschedulable(ctx, minCost.oldNodes[0].Name, false))
		return err
	}
	if len(nodeNames) != 1 {
		// shouldn't ever occur as we are only launching a single node
		return fmt.Errorf("expected a single node name, got %d", len(nodeNames))
	}

	consolidationNodesCreatedCounter.Add(1)

	var k8Node v1.Node
	// Wait for the node to be ready
	var once sync.Once
	if err := retry.Do(func() error {
		if err := c.kubeClient.Get(ctx, client.ObjectKey{Name: nodeNames[0]}, &k8Node); err != nil {
			return fmt.Errorf("getting node, %w", err)
		}
		once.Do(func() {
			c.recorder.LaunchingNodeForConsolidation(&k8Node, minCost.String())
		})

		if _, ok := k8Node.Labels[v1alpha5.LabelNodeInitialized]; !ok {
			// make the user aware of why consolidation is paused
			c.recorder.WaitingOnReadinessForConsolidation(&k8Node)
			return fmt.Errorf("node is not initialized")
		}
		return nil
	}, waitRetryOptions...); err != nil {
		// node never become ready, so uncordon the node we were trying to delete and report the error
		return multierr.Combine(c.setNodeUnschedulable(ctx, minCost.oldNodes[0].Name, false),
			fmt.Errorf("timed out checking node readiness, %w", err))
	}
	return nil
}

func (c *Controller) getNodePods(ctx context.Context, nodeName string) ([]*v1.Pod, error) {
	var podList v1.PodList
	if err := c.kubeClient.List(ctx, &podList, client.MatchingFields{"spec.nodeName": nodeName}); err != nil {
		return nil, fmt.Errorf("listing pods, %w", err)
	}
	var pods []*v1.Pod
	for i := range podList.Items {
		// these pods don't need to be rescheduled
		if pod.IsOwnedByNode(&podList.Items[i]) ||
			pod.IsOwnedByDaemonSet(&podList.Items[i]) ||
			pod.IsTerminal(&podList.Items[i]) {
			continue
		}
		pods = append(pods, &podList.Items[i])
	}
	return pods, nil
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

// nolint:gocyclo
func (c *Controller) nodeConsolidationOptionReplaceOrDelete(ctx context.Context, node candidateNode) (consolidationAction, error) {
	defer metrics.Measure(consolidationDurationHistogram.WithLabelValues("Replace/Delete"))()

	var stateNodes []*state.Node
	c.cluster.ForEachNode(func(n *state.Node) bool {
		stateNodes = append(stateNodes, n.DeepCopy())
		return true
	})
	scheduler, err := c.provisioner.NewScheduler(ctx, node.pods, stateNodes, scheduling.SchedulerOptions{
		SimulationMode: true,
		ExcludeNodes:   []string{node.Name},
	})

	if err != nil {
		return consolidationAction{result: consolidateResultUnknown}, fmt.Errorf("creating scheduler, %w", err)
	}

	newNodes, inflightNodes, err := scheduler.Solve(ctx, node.pods)
	if err != nil {
		return consolidationAction{result: consolidateResultUnknown}, fmt.Errorf("simulating scheduling, %w", err)
	}

	// were we able to schedule all the pods on the inflight nodes?
	if len(newNodes) == 0 {
		schedulableCount := 0
		for _, inflight := range inflightNodes {
			schedulableCount += len(inflight.Pods)
		}
		if len(node.pods) == schedulableCount {
			return consolidationAction{
				oldNodes:       []*v1.Node{node.Node},
				disruptionCost: disruptionCost(ctx, node.pods),
				result:         consolidateResultDelete,
			}, nil
		}
	}

	// we're not going to turn a single node into multiple nodes
	if len(newNodes) != 1 {
		return consolidationAction{result: consolidateResultNotPossible}, nil
	}

	// get the current node price based on the offering
	// fallback if we can't find the specific zonal pricing data
	offering, ok := cloudprovider.GetOffering(node.instanceType, node.capacityType, node.zone)
	if !ok {
		return consolidationAction{result: consolidateResultUnknown}, fmt.Errorf("getting offering price from candidate node, %w", err)
	}
	newNodes[0].InstanceTypeOptions = filterByPrice(newNodes[0].InstanceTypeOptions, newNodes[0].Requirements, offering.Price)
	if len(newNodes[0].InstanceTypeOptions) == 0 {
		// no instance types remain after filtering by price
		return consolidationAction{result: consolidateResultNotPossible}, nil
	}

	// If the existing node is spot and the replacement is spot, we don't consolidate.  We don't have a reliable
	// mechanism to determine if this replacement makes sense given instance type availability (e.g. we may replace
	// a spot node with one that is less available and more likely to be reclaimed).
	if node.capacityType == v1alpha1.CapacityTypeSpot &&
		newNodes[0].Requirements.Get(v1alpha5.LabelCapacityType).Has(v1alpha1.CapacityTypeSpot) {
		return consolidationAction{result: consolidateResultNotPossible}, nil
	}

	return consolidationAction{
		oldNodes:        []*v1.Node{node.Node},
		disruptionCost:  disruptionCost(ctx, node.pods),
		result:          consolidateResultReplace,
		replacementNode: newNodes[0],
	}, nil
}

func (c *Controller) hasPendingPods(ctx context.Context) bool {
	var podList v1.PodList
	if err := c.kubeClient.List(ctx, &podList, client.MatchingFields{"spec.nodeName": ""}); err != nil {
		// failed to list pods, assume there must be pending as it's harmless and just ensures we wait longer
		return true
	}
	for i := range podList.Items {
		if pod.IsProvisionable(&podList.Items[i]) {
			return true
		}
	}
	return false
}

func (c *Controller) replicaSetsReady(ctx context.Context) bool {
	var rsList appsv1.ReplicaSetList
	if err := c.kubeClient.List(ctx, &rsList); err != nil {
		// failed to list, assume there must be one non-ready as it's harmless and just ensures we wait longer
		return true
	}
	for _, rs := range rsList.Items {
		desired := ptr.Int32Value(rs.Spec.Replicas)
		if rs.Spec.Replicas == nil {
			// unspecified defaults to 1
			desired = 1
		}
		if rs.Status.ReadyReplicas < desired {
			return false
		}
	}
	return true
}

func (c *Controller) replicationControllersReady(ctx context.Context) bool {
	var rsList v1.ReplicationControllerList
	if err := c.kubeClient.List(ctx, &rsList); err != nil {
		// failed to list, assume there must be one non-ready as it's harmless and just ensures we wait longer
		return true
	}
	for _, rs := range rsList.Items {
		desired := ptr.Int32Value(rs.Spec.Replicas)
		if rs.Spec.Replicas == nil {
			// unspecified defaults to 1
			desired = 1
		}
		if rs.Status.ReadyReplicas < desired {
			return false
		}
	}
	return true
}

func (c *Controller) statefulSetsReady(ctx context.Context) bool {
	var sslist appsv1.StatefulSetList
	if err := c.kubeClient.List(ctx, &sslist); err != nil {
		// failed to list, assume there must be one non-ready as it's harmless and just ensures we wait longer
		return true
	}
	for _, rs := range sslist.Items {
		desired := ptr.Int32Value(rs.Spec.Replicas)
		if rs.Spec.Replicas == nil {
			// unspecified defaults to 1
			desired = 1
		}
		if rs.Status.ReadyReplicas < desired {
			return false
		}
	}
	return true
}

func (c *Controller) stabilizationWindow(ctx context.Context) time.Duration {
	// no pending pods, and all replica sets/replication controllers are reporting ready so quickly consider another consolidation
	if !c.hasPendingPods(ctx) && c.replicaSetsReady(ctx) &&
		c.replicationControllersReady(ctx) && c.statefulSetsReady(ctx) {
		return 0
	}
	return 5 * time.Minute
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
