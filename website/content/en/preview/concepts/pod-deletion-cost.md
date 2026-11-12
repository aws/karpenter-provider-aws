---
title: "Pod Deletion Cost"
linkTitle: "Pod Deletion Cost"
weight: 55
description: >
  Learn how Karpenter can steer ReplicaSet scale-down toward the nodes it wants to consolidate.
---

{{% alert title="Note" color="primary" %}}
This feature is alpha and disabled by default. Enable it with the `PodDeletionCostManagement` [feature gate]({{<ref "../reference/settings#feature-gates" >}}), or the Helm value `settings.featureGates.podDeletionCostManagement: true`.
{{% /alert %}}

When a Deployment scales in, the [ReplicaSet controller](https://kubernetes.io/docs/concepts/workloads/controllers/replicaset/) doesn't know which nodes Karpenter wants to consolidate, so it often removes pods from nodes Karpenter would keep. With `PodDeletionCostManagement`, Karpenter steers scale-down toward the nodes it wants to remove, so fewer pods are evicted during consolidation.

## How it works

When cluster state changes, Karpenter ranks its nodes by consolidation preference and writes the rank to the [`controller.kubernetes.io/pod-deletion-cost`](https://kubernetes.io/docs/reference/labels-annotations-taints/#pod-deletion-cost) annotation on ReplicaSet-controlled pods on Karpenter-managed nodes. It re-ranks at most once a minute, and at least every 5 minutes. The ReplicaSet controller deletes the lowest-cost pods first. All pods on a node get the same value:

| Node state | Value written |
|---|---|
| Being disrupted (tainted `karpenter.sh/disrupted` or marked for deletion) | `-2147483648` |
| Drifted, within the NodePool's `Drifted` [disruption budget]({{<ref "disruption.md#nodepool-disruption-budgets" >}}) | A negative rank, ahead of non-drifted nodes |
| Any other node that can be disrupted, within the NodePool's `Underutilized` budget | A negative rank, in consolidation order |
| Can't be disrupted | Annotation removed |

Karpenter removes the annotation from a node's pods when:
* The node isn't initialized or is nominated for pending pods.
* The node or one of its pods has `karpenter.sh/do-not-disrupt`, or a PDB blocks eviction of one of its pods.
* The node runs a non-`kube-system` pod not controlled by a ReplicaSet, Job, or DaemonSet, such as a StatefulSet or bare pod.
* The node isn't drifted and its NodePool has `consolidateAfter: Never`.
* The node's instance type isn't one of its NodePool's instance types.
* Ranking the node would exceed its NodePool's disruption budget.

## API server load

Each annotation change is a separate `PATCH` request. Ranks run from `-n` to `-1` across the whole cluster, so one node changing state can shift every other node's rank and rewrite all of their pods. Disruption budget windows opening and closing cause similar bursts. Karpenter limits this to 50 nodes per pass, plus nodes being disrupted, and skips pods that already have the right value. Annotation removals are written after ranked nodes, so a node that can no longer be disrupted can keep its old rank for a few passes.

These writes share Karpenter's `KUBE_CLIENT_QPS` and `KUBE_CLIENT_BURST` [rate limit]({{<ref "../reference/settings.md" >}}) with its other requests, including evictions, and the API server may throttle them through [API Priority and Fairness](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/). On large clusters, watch the metrics below and your API server's throttling metrics, and raise Karpenter's client rate limit if writes fall behind.

## Before you enable it

{{% alert title="Warning" color="warning" %}}
Karpenter overwrites or removes any existing `controller.kubernetes.io/pod-deletion-cost` values on these pods, and consolidation stops reading that annotation.
* If you use it to steer consolidation, switch to [`karpenter.sh/disruption-cost`]({{<ref "disruption.md#disruption-cost" >}}).
* If you use it to order ReplicaSet scale-down, Karpenter replaces your ordering on Karpenter-managed nodes.
{{% /alert %}}

Karpenter needs `patch` permission on `pods` in all namespaces to write the annotation. The Helm chart grants it only when `settings.featureGates.podDeletionCostManagement` is `true`. If you manage Karpenter's RBAC yourself, or enable the feature gate through `FEATURE_GATES` directly instead of the Helm value, add `patch` on `pods` to Karpenter's ClusterRole before you enable it. Without it, every annotation write fails.

## Disabling it

Disabling the feature gate doesn't remove the `controller.kubernetes.io/pod-deletion-cost` annotations Karpenter wrote. With the gate off, consolidation falls back to reading them on pods without `karpenter.sh/disruption-cost`. Pods on nodes that were being disrupted then look free to disrupt. The nodes could then be considered empty and removed without replacement. To roll back safely do the following:
- Disable the gate or rollback. Then wait for the new controller pods to come up and verify that no Karpenter pod with the gate enabled is still running.
- Remove every negative `pod-deletion-cost` that you didn't set for pods on Karpenter nodes.

Note: Empty consolidation running during this entire process may still delete nodes without replacements. If you want to strictly avoid it you can temporarily set an Empty budget of 0 during this process.

## Metrics

| Metric | Labels | Description |
|---|---|---|
| `karpenter_pod_deletion_cost_pod_annotation_writes_total` | `result`: `updated`, `skipped_unchanged`, `skipped_notfound`, `skipped_conflict`, `error` | Annotation write attempts by outcome. Conflicting writes aren't retried until the next pass, so a high `skipped_conflict` rate means rankings lag cluster state. |
| `karpenter_pod_deletion_cost_nodes_with_pending_annotation_writes` | `nodepool` | Nodes with an annotation write queued in the last pass that ran. |
