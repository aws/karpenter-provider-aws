# Karpenter - Deprovisioning Controller

Karpenter will implement a mechanism to detect and reconcile nodes that drift from Provisioner and ProviderRef specs. Karpenter will merge all scale-down logic into a `Deprovisioning` controller. This will orchestrate Consolidation, Expiration, Emptiness, and Drift, and any future scale-down logic. The Deprovisioning controller will follow the same structure and build on the Consolidation controller.

## Consolidation Background

Consolidation computes `consolidationActions` to deprovision nodes to optimize resource utilization in the cluster. Consolidation will provision nodes if Karpenter will need to due to deprovisioning. Consolidation executes one `consolidationAction` per reconcile loop, deleting empty nodes first, then consolidating nodes in order of a disruption cost heuristic.

Karpenter will not consolidate a cluster if capacity has changed recently, and will not consolidate a node if Karpenter cannot evict all the scheduled pods.

## DeprovisioningActions

`ConsolidationActions` will now be called `DeprovisioningActions`. The `delete` and `replace` actions will be renamed `deleteConsolidation` and `replaceConsolidation`. This renaming will only affect metrics.

Currently, after empty nodes, nodes closer to expiry are prioritized for consolidation. `DeprovisioningActions` will be suffixed by the reason, and will be prefixed as `replace` if Karpenter provisions a new node in response to the deprovisioning or `delete` if not. Karpenter's future scale down mechanisms will follow this pattern.

Any action will be one of the following, where emptiness will be merged into an edge case of consolidation:
- `deleteConsolidation`
- `replaceConsolidation`
- `deleteExpiration`
- `replaceExpiration`
- `deleteDrift`
- `replaceDrift`

As more mechanisms

### DeprovisioningAction Tunables

With `DeprovisioningActions`, Karpenter will expand the [Internal Tunables](https://github.com/aws/karpenter-provider-aws/blob/main/designs/consolidation.md#internal-tunables) for each of the new `DeprovisioningActions`.

* Order of Node Evaluation - In order, Karpenter will deprovision expired nodes, drifted nodes, then consolidatable nodes. Karpenter will prioritize any custom provider deprovisioning mechanisms such as health checks before any other.
* DeprovisioningTTL - Karpenter consolidation uses a Consolidation TTL per-node. Going forward Karpenter will use a DeprovisioningTTL of 15 seconds for all deprovisioning methods. These can be tuned to be more granular in the future.
* Replacement Node Timeout - Currently this is [9.5 minutes](https://github.com/aws/karpenter-provider-aws/blob/main/pkg/controllers/consolidation/controller.go#L70). Karpenter will not change this.

## Benefits of the Deprovisioning Controller

Merging consolidation, drift, and expiration allows Karpenter to define coordinated behavior between the different mechanisms. Tunables applied to the deprovisioning controller will be applied to all deprovisioning methods. This could be further condensed under one API section within a Provisioner. Karpenter could also more efficiently execute a series of DeprovisioningActions by considering future actions.
