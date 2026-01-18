# Karpenter - Static Capacity

## Background
Karpenter currently operates as a dynamic cluster autoscaler, automatically adjusting node counts based on pending pod demand. However, several important use cases require maintaining a fixed set of nodes such as:

1. Performance-critical applications where just-in-time provisioning latency is unacceptable
2. Workloads that require predictable, always-available capacity
3. Operational models that rely on a fixed number of nodes for budgeting, isolation, or infrastructure boundary

Currently, users attempt to achieve static capacity through workarounds:
- Configuring provisioner requirements to run placeholder pods to maintain minimum capacity
- Using separate node management tools alongside Karpenter

## Proposal
Extend the existing NodePool resource to support static provisioning capabilities by adding new fields:

```yaml
spec:
  # Makes NodePool static when specified
  replicas: 5  # Number of nodes to maintain
limits:
  # Prevents Karpenter from creating new replicas once the limit is exceeded
  nodes: 10 # Constrain the size of static Nodepool
status:
  nodes: 10 # shows the current size of Nodepool
```
The existence of replicas field will determine if NodePool is Static

Key aspects:
1. Static NodePools maintain fixed node count regardless of pod demand
2. Will not be considered as consolidation candidate, so consolidation settings are not allowed other than defaults (at least in v1alpha1)
3. Optional limits.nodes can be set to constrain the maximum size of static Nodepool when bursting (e.g. during drift or expiration)
4. Inherit existing Karpenter features
5. Use existing NodeClaim infrastructure with owner references
  
We will support scaling static node pools using a command such as:

```sh
kubectl scale nodepool <name> --replicas=10
```

To support this we will add Nodes field of type int64 under status. `.status.Nodes` will contain the current Node count for a NodePool.

### Modeling & Validation

When `replicas` is specified, the `NodePool` enters a static provisioning mode where certain disruption-related fields, weight and limits become irrelevant or misleading. Specifically:

- `consolidationPolicy` and `consolidateAfter` (which control Karpenter’s consolidation logic) must not be used when the node count is fixed via `replicas`.
- limits other than `limits.nodes` must not be set
- weight must not be set

We will have strict validation against setting limits (other than limits.nodes) and weight but if consolidation parameters are set then it will simply be ignored.

Once a NodePool has replicas set, it cannot be removed unless the NodePool itself is deleted. This means a NodePool cannot switch between static and dynamic modes:
- If a NodePool is static (replicas set), replicas cannot be unset or modified to nil.
- If a NodePool is dynamic (replicas not set), replicas cannot be added.
- This validation prevents unintentional large-scale terminations or sudden behavior changes in consolidation logic.


#### Validation rules

There will not be a validation against consolidation settings (consolidationPolicy and consolidateAfter), if specified it will simply be ignored

```yaml
If has(self.replicas) != has(oldSelf.replicas)
  => Validation Error
```
```yaml
if replicas != nil:
  - limits other than limits.nodes must not be set
  - weight must not be set
  => ValidationError if not
```

### Static NodePool limits

We will not be supporting limits other than limits.node, while dynamic provisioning does filter NodeClaim templates based on limits (e.g., limits.cpu, limits.memory), it operates under the assumption that pods are batched and packed efficiently into a single NodeClaim. This is fundamentally different from static provisioning, where we attempt to create a fixed number of NodeClaims (replicas). Limits would start to dictate instance shape, not just aggregate capacity. This creates a conflict between honoring replicas vs. satisfying hard limits.

However, we are introducing a new field, limits.nodes, to explicitly cap the maximum number of nodes which can be provisioned for a given NodePool at a point in time.

### Disruption

Karpenter already has Disruption for nodes we can use the same mechanism. 
Since Static Nodepool and NodeClaim is a variant of existing Nodepool/NodeClaim it will inherit Karpenter dynamic provider integration. Drift Detection can also be inherited to trigger replacement of drifted nodes. During this Karpenter will respect the Disruption Budget. If limits.nodes are specified then Karpenter would wait until Nodes are terminated and count is well within node limits before creating replacements.

Replacement will be one-for-one i.e create replacements before terminating the old one to honor replicas if Nodepool node count do not exceed the node limits. 
When a user scales down a static NodePool, Karpenter will delete NodeClaims and gracefully terminate nodes.
Key behaviors:
- User-driven actions (scaling replicas) will not respect disruption budgets. The user explicitly intends to remove capacity, and we honor that request by deleting corresponding NodeClaims. This will be classified as an [automated forceful disruption method](https://karpenter.sh/docs/concepts/disruption/#automated-forceful-methods), a la expiration.
- Supported Karpenter-driven actions (e.g., drift) respect disruption budgets and scheduling safety.

Deletion of nodepool would disrupt all of its NodeClaims. Garbage collection logic do not differ between static and dynamic NodePools.

**Important Note**: The scale-down of replicas in a static NodePool will follow standard forceful disruption semantics. This means that when the desired replica count is reduced, Karpenter will drain nodes and while respecting PodDisruptionBudgets (PDBs), terminationGracePeriod, and other workload disruption safeguards. We will not bypass these constraints, as PDBs represent an explicit agreement between the cluster operator and the controller to maintain application availability during changes. This aligns with Kubernetes conventions, where scaling actions are reconciled towards the desired state without ignoring safety guarantees, and ensures predictable behavior for workloads. While this may mean that scale-down is delayed when eviction blockers are present, it provides consistency with the broader Kubernetes ecosystem.


### Consolidation

Static NodePools are not eligible for consolidation. They act like any other static capacity source (e.g other nodes in the cluster). Their lifecycle is managed directly by the user, not by Karpenter’s dynamic optimization logic.
However, the nodes provisioned by a static pool:
- Participate in scheduling decisions for both launched capacity and yet to launch inflight capacity (i.e., pods can land on them)
- Are monitored for drift, enabling graceful re-creation when necessary

This means they remain first-class citizens in the cluster but are excluded from cost-based disruption decisions.

## Requirements

The requirements field in a static NodePool behaves identically to dynamic pools—it defines the constraints for all NodeClaims launched under that NodePool.
In static pools, we must choose multiple concrete node configurations up front—i.e., for replicas: 10, we select 10 NodeClaims matching the requirement set.
If the requirements allow multiple combinations:
- Karpenter selects the optimal combination based on cost, availability, and zone balancing
- This selection is done once at provisioning time (unlike dynamic pools, where evaluation occurs per provisioning event)

In static provisioning, the NodeClaim requirements are directly derived from the NodeClaimTemplate on the NodePool. These are evaluated once per NodeClaim at creation, meaning the selection is based solely on what the template allows.
As a result, even though all NodeClaims come from the same static NodePool, they may still result in different instance types (shapes/flavors), depending on availability, since that decision happens during cloud-provider Create() call.

#### Zonal Distribution
Static NodePools do not currently support topology-aware spreading in the same way that dynamic NodePools rely on pod-driven scheduling. Initially, we explored automatically spreading static NodeClaims across zones when the topology.kubernetes.io/zone requirement includes multiple values, for example:

```yaml
- key: topology.kubernetes.io/zone
  operator: In
  values: ["zone-2a", "zone-2b", "zone-2c"]
```

However, this introduces implicit behavior that conflicts with existing Karpenter semantics. In current behavior, absence of a requirement is treated as unconstrained, and specifying zones in requirements is interpreted as a hard constraint. Introducing automatic spreading implicitly redefines these semantics, which could have long-term consequences for other requirement types (e.g., capacity type).
Given these concerns, we’re punting zonal spreading to a future enhancement. For now, we will continue to treat the requirements field as the single source of truth for topology decisions. Users who want to spread nodes across zones can do so explicitly by:
- Specifying multiple zones in the topology.kubernetes.io/zone requirement, or
- Creating multiple static NodePools, each pinned to a specific AZ.

We plan to explore spread option to offer users a more flexible and declarative way to express zone-spreading behavior, similar to podTopologySpreadConstraints.


## Example 
I want a Static NodePool that:
- Always keeps exactly 12 running nodes under normal operation.
- Can temporarily scale up to a maximum of 20 nodes, but never exceed that.
- Runs only in a single availability zone (zone-2a).
- Uses a fixed instance type (m5.2xlarge) for predictable capacity.
- Has nodes that expire after 720 hours.
- Follows disruption budgets allowing at most 10% of nodes to be disrupted at a time,
- and blocks all deprovisioning during weekday business hours.

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: static-prod-nodepool
spec:
  replicas: 12 # Maintains 12 nodes
  template:
    metadata:
      labels:
        nodepool-type: static
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: myEc2NodeClass
      expireAfter: 720h
      taints:
        - key: example.com/special-taint
          effect: NoSchedule
      requirements:
        - key: topology.kubernetes.io/zone
          operator: In
          values: ["zone-2a"]
        - key: karpenter.k8s.aws/instance-type
          operator: In
          values: ["m5.2xlarge"]
  disruption:
    budgets:
      - nodes: 10%
      # On Weekdays during business hours, don't do any deprovisioning.
      - schedule: "0 9 * * mon-fri"
        duration: 8h
        nodes: "0"
  limits:
    # Constrain the size of static Nodepool 
    nodes: 20 # Karpenter will not scale up nodes or create replacements if limits are breaching
```

## Implementation Details

### Controller Architecture

We will be creating Static Capacity by adding 2 new controllers under a feature flag. This will be added to v1alpha1 release to get feedback from users.

#### Scale-Up Controller
- **Purpose**: Scale up NodeClaims to meet desired replicas
- **Behavior**: Monitors the difference between desired replicas and current healthy NodeClaims, provisioning new nodes as needed respecting limits.nodes value and topology requirements
- **Instance Selection**: Scale-up controller will be creating NodeClaims and Instance selection will happen during CloudProvider call. During which we Will find the cheapest instance possible based on requirements
- **Consolidation**: Static NodePools will not be consolidated to maintain predictable capacity

#### Deprovisioning Controller  
- **Purpose**: Scale down replicas when desired count is reduced
- **Node Selection**: Will prioritize empty nodes for termination. Incase of non-empty nodes we will pick at random. In future iterations, we will add intelligent algorithms for selecting which nodes to scale down
- **Termination Behavior**: Will gracefully terminate nodes to meet desired size respecting PodDisruptionBudgets or terminationGracePeriod

### Scheduling Integration

We will consider static nodeclaims that are already launched and to be launched during scheduling simulation. This approach will:
- Lessen the churn and possibility of over-provisioning
- Be accounted for during disruption/consolidation operations
- Be considered during new dynamic capacity provisioning decisions

This ensures that the scheduler is aware of both existing and planned static capacity when making provisioning decisions.

### Testing Strategy

For testing, we will add comprehensive integration tests to ensure the feature works correctly across different scenarios:
- Scale-up and Scale-down operations 
- Scale-up operation 
- Ensure NodeClaims created respect topology requirements
- Static NodeClaims get drifted

### Observability

Controller-runtime metrics already provide baseline visibility into reconcile performance and errors. Static NodePools also expose spec.replicas (desired count) and status.nodes (current count), which we will surface as metrics.
We’re not adding extra status fields now to keep the API surface minimal, but can introduce fields like pending/terminating replicas in future if needed.


## Alternative Proposal: New Static NodePool API


The alternative approach would be to create a separate StaticNodePool API focused solely on static provisioning. This would include:
- Dedicated API for static provisioning
- Clear separation from existing Nodepool 
- Validation rules specific to static provisioning

However, this approach was rejected because:
- Many core functionalities would be shared between static and dynamic capacity management
- Creates unnecessary cognitive overhead for users
- Requires duplicate documentation

The better approach is to extend the existing NodePool API since the differences represent different modes of the same fundamental abstraction rather than entirely separate concepts.

