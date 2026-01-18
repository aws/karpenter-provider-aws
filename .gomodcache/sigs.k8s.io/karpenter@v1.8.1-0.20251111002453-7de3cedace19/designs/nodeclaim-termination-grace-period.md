# NodeClaim Termination Grace Period

## Motivation
Users are requesting the ability to control how long karpenter will wait for the deletion of an individual node to complete before forcefully terminating the node regardless of pod status. [#743](https://github.com/kubernetes-sigs/karpenter/issues/743). This supports two primary use cases.
* Cluster admins who want to ensure that nodes are cycled after a given period of time, regardless of user-defined disruption controls (such as PDBs or preStop hooks) that might prevent eviction of a pod beyond the configured limit. This could be to satisfy security requirements or for convenience.
* Cluster admins who want to allow users to protect long-running jobs from being interrupted by node disruption, up to a configured limit.

This design adds in a new `terminationGracePeriod` field to the `NodeClaim` resource. The field denotes the maximum time a node will wait for graceful termination of its pod before being forcefully torn down.
This new field is immutable on each NodeClaim, and requires cycling nodes in order to change the value.

## Proposed Spec

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodeClaim
spec:
  terminationGracePeriod: 24h   # metav1.Duration, nil if not set
```

## Code Definition

```go
type NodeClaimSpec struct {
  {...}
	// TerminationGracePeriod is the maximum duration the controller will wait before forcefully deleting the pods on a node, measured from when deletion is first initiated.
	//
	// Warning: this feature takes precedence over a Pod's terminationGracePeriodSeconds value, and bypasses any blocked PDBs or the karpenter.sh/do-not-disrupt annotation.
	//
	// This field is intended to be used by cluster administrators to enforce that nodes can be cycled within a given time period.
	// When set, drifted nodes will begin draining even if there are pods blocking eviction. Draining will respect PDBs and the do-not-disrupt annotation until the TGP is reached.
	//
	// Karpenter will preemptively delete pods so their terminationGracePeriodSeconds align with the node's terminationGracePeriod.
	// If a pod would be terminated without being granted its full terminationGracePeriodSeconds prior to the node timeout,
	// that pod will be deleted at T = node timeout - pod terminationGracePeriodSeconds.
	//
	// The feature can also be used to allow maximum time limits for long-running jobs which can delay node termination with preStop hooks.
	// If left undefined, the controller will wait indefinitely for pods to be drained.
	// +kubebuilder:validation:Pattern=`^([0-9]+(s|m|h))+$`
	// +kubebuilder:validation:Type="string"
	// +optional
	TerminationGracePeriod *metav1.Duration `json:"terminationGracePeriod,omitempty"`
}
```

## Alternative Spec
Rather than defining the `terminationGracePeriod` on the NodeClaim spec, it could be added to the NodePool disruption block instead.

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: default
spec:
  disruption:
    terminationGracePeriod: 24h   # metav1.Duration, nil if not set
```

#### Benefits
1. Making updates to the `terminationGracePeriod` in a NodeClaim template is cumbersome and disruptive to users of the cluster since it requires all nodes to be cycled. The NodePool disruption block can be considered mutable and allow cluster admins to change the value without replacing all nodes.

#### Drawbacks
1. Changes to the `terminationGracePeriod` would be more surprising, because current live nodes would be impacted, and might result in immediate deletion of pods bypassing PDBs if the grace period is too low or pod terminationGracePeriodSeconds is too high.
2. Behavior would no longer match the behavior of a Pod's `terminationGracePeriodSeconds`, which is immutable and can only be changed with a full pod rollout.
3. Users of NodeClaims without NodePools would not be able to take advantage of this feature.

## Validation/Defaults
The `terminationGracePeriod` fields accepts a common duration string which defaults to a value of `nil`. Omitting the field results in the default nil value instructing the controller to wait indefinitely for pods to drain gracefully, maintaining the existing behavior.
A value of 0 instructs the controller to evict all pods by force immediately, behaving similarly to `pod.spec.terminationGracePeriodSeconds=0`.

## Prior Art
This is the field from CAPI's MachineDeployment spec which implements similar behavior.
```
$ kubectl explain machinedeployment.spec.template.spec.nodeDrainTimeout
KIND:     MachineDeployment
VERSION:  cluster.x-k8s.io/v1beta1

FIELD:    nodeDrainTimeout <string>

DESCRIPTION:
     NodeDrainTimeout is the total amount of time that the controller will spend
     on draining a node. The default value is 0, meaning that the node can be
     drained without any time limitations. NOTE: NodeDrainTimeout is different
     from `kubectl drain --timeout`
```

```
$ kubectl explain pod.spec.terminationGracePeriod
KIND:     Pod
VERSION:  v1

FIELD:    terminationGracePeriod <integer>

DESCRIPTION:
     Optional duration in seconds the pod needs to terminate gracefully. May be
     decreased in delete request. Value must be non-negative integer. The value
     zero indicates stop immediately via the kill signal (no opportunity to shut
     down). If this value is nil, the default grace period will be used instead.
     The grace period is the duration in seconds after the processes running in
     the pod are sent a termination signal and the time when the processes are
     forcibly halted with a kill signal. Set this value longer than the expected
     cleanup time for your process. Defaults to 30 seconds.
```

## Implementation

### Termination Grace Period Expiration Detection
Because Karpenter already ensures that nodes and their nodeclaims are in a deleting state before performing a node drain during node deletion, we are able to leverage existing `deletionTimestamp` fields to avoid the need for an addition annotation or other tracking label.
We'll use the NodeClaim's `deletionTimestamp` specifically to avoid depending on the Node's `deletionTimestamp`, because [Kubernetes docs](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/) recommend draining a node *before* deleting it, which is counter to how Karpenter behaves today (relying on Node finalizers to handle cleanup).

### Termination Grace Period Expiration Behavior
1. NodeClaim deletion occurs (user initiated via Node finalizer,or when Karpenter begins node rotation from drift detection, etc).
2. NodeClaim deletionTimestamp is set.
3. Standard node drain process begins.
4. Drain attempts fail (due to PDBs, preStop hooks, etc). Karpenter adds an Event to the Node noting that it will be forcibly terminated at X time.
5. Reconciliation of the Node/NodeClaim is requeued (potentially many times).
6. Time passes, and the configured terminationGracePeriod for the NodeClaim is exceeded.
7. Any remaining pods are forcibly drained, ignoring eviction (bypassing PDBs, preStop hooks, pod terminationGracePeriod, etc).
8. The standard Karpenter reconciliation loop for the termination controller succeeds, and the node is formally removed from the cloud provider.

### Eventual Disruption
Karpenter automatically excludes nodes containing do-not-disrupt annotated pods or pods with blocked PDBs from being considered as candidates for disruption. Implementation of this feature will require changing that behavior to allow disruption of those nodes.

### Optimizing Pod Deletion Behavior When Eviction is Blocked
Some pod protection semantics block or delay termination of pods, which could result in a pod being forcibly evicted without any chance for graceful shutdown while using this feature.

Consider the following scenario.
1. A Deployment has 3 replicas, each with a terminationGracePeriodSeconds of 10m.
2. Each of these pods requires the full 10m to terminate gracefully.
3. The Deployment has an associated PDB with maxUnavailable 1 requiring that only one of these pods be disrupted at a time.

1. The cluster admin has configured NodePool.spec.disruption.terminationGracePeriod at 15m.
2. A full node reconcilliation has started which has caused all nodes which contain the pods above to enter a deleting state at the same time.

During the standard node deletion process, k8s would:
1. Evict pod A only, which takes 10m of the available 15m to terminate gracefully. Pod B and C are not evicted, due to their PDB only allowing 1 disruption at a time.
2. After pod A starts up healthy on a new node, pod B is evicted, but has only 5m of the needed 10m to terminate. Both pods B and C are evicted before graceful termination could complete, and pod C never received a termination signal.

In order to improve this case for users of the cluster, we can assume that a pod's terminationGracePeriodSeconds is a good estimate of the time it takes to terminate gracefully and start the eviction process early if the pod would not be able to terminate within its terminationGracePeriodSeconds when a node timeout is looming.
Pods that are deleted in this way should have a clear event log of what deleted their pod (karpenter) and why (current time > node terminationGracePeriod - pod terminationGracePeriod).

Risks
1. This behavior could be confusing to cluster administrators, and could result in immediate pod evictions when a NodePool terminationGracePeriod is set, even to a value of hours.
2. This behavior could be confusing to users, who may have set their pod terminationGracePeriodSeconds to higher than necessary values and have their pods suddenly deleted when a node begins deletion.

### Documented Warnings
This feature carries some risk for cluster administrators given that it can bypass the configured grace periods and PDBs set by users of a cluster. These risks should be called out explicitly in the docs for the feature to minimize the risk of a cluster administrator unintentionally disrupting their users' workloads.
1. Enabling this feature by setting NodePool.spec.template.terminationGracePeriod to any value will result in a drift reconciliation that will replace all nodes. The new terminationGracePeriod value will not be active until new NodeClaims have been provisioned.
2. Enabling this feature can result in termination of user pods, violating their configured grace periods or PDBs. If a pod has a terminationGracePeriodSeconds greater than the NodeClaim's TGP, it will be deleted as soon as the Node begins disruption, effectively limiting a pod's maximum TGPS to the NodeClaim's TGP.
3. Because of the optimization stated above, user pods could be terminated in violdation of their PDBs **before** the configured NodeClaim grace period is reached.

## Manual Intervention
If needed, a cluster-admin can prematurely forcefully terminate a node by starting a `kubectl delete node` followed by `kubectl drain --disable-eviction=true`.
This is already possible today with Karpenter, and means that we shouldn't need to consider manual forced termination as part of this design.

```
$ kubectl drain my-node --grace-period=0 --disable-eviction=true

$ kubectl drain --help
Options:
    --grace-period=-1:
	Period of time in seconds given to each pod to terminate gracefully. If negative, the default value specified
	in the pod will be used.

    --disable-eviction=false:
	Force drain to use delete, even if eviction is supported. This will bypass checking PodDisruptionBudgets, use
	with caution.
```
