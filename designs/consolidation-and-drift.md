# Karpenter - Node Lifecycle Controller - Consolidation & Drift

Karpenter will implement a mechanism to detect and reconcile nodes that drift from Provisioner and AWSNodeTemplate specs. Karpenter will merge Consolidation, Expiration, Emptiness, and Drift into a `NodeLifecycle` controller which will be responsible for all controllers executing scale-down logic on existing nodes.

## Consolidation Background

Consolidation implements node emptiness, deletion, and replacement as `consolidationActions` named `deleteEmpty`, `delete`, and `replace`, respectively. Consolidation takes one `consolidationAction` per reconcile loop, where any empty nodes are deleted first. After ordering the consolidatable nodes by `disruptionCost`, Karpenter computes a `consolidationAction` for one node at a time.

The `deleteEmpty` operation is by definition able to delete a node immediately without considering PDBs or `do-not-evicts` since no pods exist on the node. In this case, no replacement node needs to be created.

A node cannot be considered for the `delete` and `replace` operations if the node has a blocking PDB, isn’t already being deleted, or has a pod that cannot be evicted. A replacement node is created only for the replace operation.

## ConsolidationActions → LifecycleActions

First, `ConsolidationActions` will now be called `LifecycleActions`. The `delete` and `replace` actions will be renamed `deleteCost` and `replaceCost`. At a high level, this renaming will only affect metrics.

Currently, expired nodes are first in line for the `delete` and `replace` operations since Karpenter scales disruption cost by node age, where an expired node has `0` `disruptionCost`. Expired nodes cannot be deleted if they cannot be terminated or have pods that prevent eviction. Expiration will continue to be blocked as such and determine if a node needs to be replaced. The only difference will be that there will be two new `LifecycleActions`: `deleteExpired` and `replaceExpired`.

Drifted nodes will be second in line for the `delete` and `replace` operations. they will also be be blocked by the same conditions as expired nodes, and will also compute new `LifecycleActions` named `deleteDrifted` and `replaceDrifted`.

### LifecycleAction Tunables

With new LifecycleActions, Karpenter will expand the [Internal Tunables](https://github.com/aws/karpenter/blob/main/designs/consolidation.md#internal-tunables) for each of the new LifecycleActions.

* Order of Node Evaluation (after emptiness) - LifecycleActions will handle expired nodes then drifted nodes before doing cost-based consolidation, prioritizing lower disruption costs for drift and cost-based consolidation.
* Stabilization Window - This will be different based on what the last `LifecycleAction` was. Currently it’s 5 minutes for all actions. We will keep Emptiness, Expiration, Drift, and Cost at 5 minutes, but can allow it to be tunable in the future.
* Replacement Node Timeout - Currently this is [9.5 minutes](https://github.com/aws/karpenter/blob/main/pkg/controllers/consolidation/controller.go#L70). Karpenter will not change this.

## Benefits of Expanding Consolidation

Expanding consolidation for drift and expiration allows Karpenter to easily define behavior between the different controllers. For instance, Consolidation’s stabilization window which acts as a cooldown period can be configured differently for each concept. This could be further condensed under one API section within a Provisioner. Karpenter could also more efficiently execute a series of LifecycleActions by considering future actions.

At first, this means that consolidation in the ProvisionerSpec can now turn on drift enforcement as well, where users that want Karpenter to take more complex actions on their cluster are automatically onboarded to it if they use `consolidation: true` in their spec. This could be modified in the future.
