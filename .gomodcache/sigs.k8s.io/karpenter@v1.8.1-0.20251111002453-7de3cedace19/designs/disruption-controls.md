# Disruption Controls - (in v1beta1+)

## Motivation
Users have been asking for more control around the speed of Consolidation and the ability to restrict disruptions to their nodes. The demand for this is reflected by the +1s in [#1738](https://github.com/aws/karpenter/issues/1738). This is increasingly important as clusters can run at a highly varied scales, where some users have reported scale-down behaving too slow at higher scales, or too quick at smaller scales. Additionally, users that want to control when nodes can be disrupted must add PDBs to block node disruptions, add `do-not-evict` annotations on pods for sensitive workloads, or add `do-not-consolidate` annotations on nodes that shouldn't be consolidated. Natively integrating a better way to ensure nodes aren't disrupted will ease the burden that users have to take on with disruption.

These controls will include a new `Disruption` block in the `v1beta1` `NodePool` that will contain all scale-down behavioral fields. Initially, this will include `Budgets`, which define a (1) parallelism of how many nodes can be deprovisioned at a time and a (2) cron schedule that determines when the parallelism applies, and a `ConsolidateAfter` field that will allow users to affect the speed at which Karpenter will scale down underutilized nodes. Future scale-down behavioral fields should be colocated with these fields.

## Proposed Spec

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: default
spec: # This is not a complete NodePool Spec.
  disruption:
    consolidationPolicy: WhenUnderutilized || WhenEmpty
    consolidateAfter: 10m || Never # metav1.Duration
    expireAfter: 10m || Never # Equivalent to v1alpha5 TTLSecondsUntilExpired
    budgets:
    # On Weekdays during business hours, don't do any deprovisioning.
    - schedule: "0 9 * * mon-fri"
      duration: 8h
      nodes: 0
    # Every other time, only allow 10 nodes to be deprovisioned simultaneously
    - nodes: 10
```

## Code Definition

```go
type Disruption struct {
    {...}
    // Budgets is a list of Budgets.
    // If there are multiple active budgets, the most restrictive budget's value is respected.
    Budgets []Budget `json:"budgets,omitempty" hash:"ignore"`
    // ConsolidateAfter is a nillable duration, parsed as a metav1.Duration.
    // Users can use "Never" to disable Consolidation.
    ConsolidateAfter *NillableDuration `json:"consolidateAfter" hash:"ignore"`
    // ExpireAfter is a nillable duration, parsed as a metav1.Duration.
    // Users can use "Never" to disable Expiration.
    ExpireAfter *NillableDuration `json:"expireAfter" hash:"ignore"`
    // ConsolidationPolicy determines how Karpenter will consider nodes
    // as candidates for Consolidation.
    // WhenEmpty uses the same behavior as v1alpha5 TTLSecondsAfterEmpty
    // WhenUnderutilized uses the same behavior as v1alpha5.ConsolidationEnabled: true
    ConsolidationPolicy string `json:"consolidationPolicy" hash:"ignore"`
}
// Budget specifies periods of times where Karpenter will restrict the
// number of Node Claims that can be terminated at a time.
// Unless specified, a budget is always active.
type Budget struct {
    // Nodes dictates how many NodeClaims owned by this NodePool
    // can be terminating at once.
    // This only respects and considers NodeClaims with the karpenter.sh/disruption taint.
    Nodes intstr.IntOrString `json:"nodes" hash:"ignore"`
    // Schedule specifies when a budget begins being active.
    // Schedule uses the same syntax as a Cronjob.
    // And can support a TZ.
    // "Minute Hour DayOfMonth Month DayOfWeek"
    // This is required if Duration is set.
    Schedule *string `json:"schedule,omitempty" hash:"ignore"`
    // Duration determines how long a Budget is active since each schedule hit.
    // This is required if schedule is set.
    Duration *metav1.Duration `json:"duration,omitempty" hash:"ignore"`
}
```

## Validation/Defaults

For each `Budget`, `Nodes` is required, and must be non-negative. Users can disable scale down for a NodePool by setting this to `0`. Users must either omit both `Schedule` and `Duration` or set both of them, since `Schedule` and `Duration` are inherently linked. Omitting these two fields will be equivalent to an always active `Budget`. Users cannot define a seconds nodes in `Duration`, since the smallest denomination of time in upstream Schedules are minutes. Note that `Nodes` will only refer to nodes with the `karpenter.sh/disruption` taint set.

- Omitting the field `Budgets` will cause the field to be defaulted to one `Budget` with `Nodes: 10%`.
- `ConsolidationPolicy` will be defaulted to `WhenUnderutilized`, with a `consolidateAfter` value of `15s`, which is the same value for Consolidation in v1alpha5.

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: default
spec:
  disruption:
    consolidationPolicy: WhenUnderutilized
    consolidateAfter: 15s
    expireAfter: 30d
    budgets:
    - nodes: 10%
```
Karpenter will not persist a default for the `Schedule` and `Duration` fields.

## API Choices

### In-line with the NodePool

Adding the `Budgets` field into the `NodePool` implicitly defines a per-`NodePool` grouping for the `Budgets`.

* 👍 Karpenter doesn't have to create/manage another CRD
* 👍 All disruption fields are colocated, which creates a natural spot for any future disruption-based behaivoral fields to live. (e.g. `terminationGracePeriodSeconds`, `rolloutStrategy`, `evictionPolicy`)
* 👎 Introduces redundancy for cluster admins that use the same `Disruption` settings for every `NodePool`. If a user wanted to change a parallelism throughout their cluster, they’d need to persist the same value for all known resources. This means users cannot set a cluster-scoped Node Disruption Budget.

### New Custom Resource

Adding a new Custom Resource would require adding an additional `Selector` Field which references a set of nodes with a label set. It would have the same fields as the `Budget` spec above. This enables users to select a set of nodes by a label selector, allowing each `Budget` to refer to any node in the cluster, regardless of the owning `NodePool`.

#### Pros + Cons

* 👍 Using a selector allows a `Budget` to apply to as many or as little NodePools as desired
* 👍 This is a similar logical model as pods → PDBs, an already well-known Kubernetes concept.
* 👎 Users must reason and manage about another CR to understand when their nodes can/cannot be terminated
* 👎👎 Application developers shouldn’t be able to have the ability to create/modify this CR, since they could control disruption behaviors for other NodePools that they shouldn’t have permissions to control.
* 👎 There will be disruption fields on both the Node Pool and the new CR, potentially causing confusion for where future configuration should reside.
  * Generally, we cannot migrate `consolidateAfter` and `expireAfter` fields to this new Custom Resource, as overlapping selectors will associate multiple TTLs to the same nodes. This introduces additional complexity for users to be aware of when these fields apply (in relation to the time-based field), and requires Karpenter to implement special cases to handle these conflicts.

### New Custom Resource with Ref From Node Pool

This would create a new Custom Resource which is referenced within the `NodePool` akin to how the `Provisioner` references a `ProviderRef`. This would exclude the `Selector` field mentioned in the approach above, as each CR would be inherently linked to a `NodePool`.

#### Pros + Cons

* 👍 All disruption-based fields will live on the same spec.
* 👍 Karpenter already uses this mechanism which is done with `Provisioner → ProviderRef` / `NodePool -> NodeClass`.
* 👎 If we wanted to align all Disruption fields together, migrating the other `consolidateAfter` and `expireAfter` fields over to a new could confuse users.
