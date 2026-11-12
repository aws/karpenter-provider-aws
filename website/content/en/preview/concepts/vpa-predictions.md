---
title: "VPA Predictions"
linkTitle: "VPA Predictions"
weight: 56
description: >
  Learn how Karpenter uses Vertical Pod Autoscaler recommendations to size nodes for pods it moves.
---

{{% alert title="Note" color="primary" %}}
This feature is alpha and disabled by default. Enable it with the `PredictionEnabled` [feature gate]({{<ref "../reference/settings#feature-gates" >}}), or the Helm value `settings.featureGates.predictionEnabled: true`.
{{% /alert %}}

When Karpenter disrupts a node, it simulates rescheduling the node's pods to decide whether they fit on existing nodes or need a replacement, and how large that replacement should be. The pods it moves are recreated, and if a [Vertical Pod Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler) (VPA) manages them, VPA's admission controller sets the new pods' requests to its recommendation. With `PredictionEnabled`, Karpenter sizes nodes using those recommended requests instead of the pods' current requests, so replacements fit the pods VPA will actually create.

## How it works

Karpenter reads every VPA in the cluster every 30 seconds and stores a prediction for each workload a VPA targets. It uses a prediction when it schedules:
* Pods it's moving: pods on nodes it's disrupting, and pods on nodes that are being deleted.
* DaemonSet pods, when it calculates the overhead of a new node.

Pending pods aren't affected. VPA already set their requests when they were created, so Karpenter schedules them using their current requests.

For each container in the VPA's recommendation, Karpenter uses the `target` value, the same value VPA applies to new pods:
* Only the resources in the container policy's `controlledResources` are used, which defaults to `cpu` and `memory`.
* The value is clamped to the container policy's `minAllowed` and `maxAllowed`.
* If the VPA configures a CPU startup boost, it's applied.

Only requests change. Containers without a recommendation keep their current requests, and limits aren't changed. If a pod sets pod-level resources (`spec.resources`), Karpenter uses those for CPU and memory, as kube-scheduler does. VPA doesn't change pod-level resources.

Karpenter ignores a VPA when:
* Its `updateMode` is `Off`, since VPA won't change the pods.
* It has no recommendation yet.
* A container's policy has `mode: Off`. Only that container is skipped.

When a prediction changes, Karpenter reevaluates its nodes for consolidation.

## Supported workloads

Like VPA, Karpenter follows a pod's owner references to its top-level workload and matches it to the VPA whose `targetRef` names that workload. For example, a Deployment's pods match a VPA that targets the Deployment, not one that targets its ReplicaSet.

| VPA `targetRef` | Used by Karpenter |
|---|---|
| Deployment, StatefulSet, DaemonSet, ReplicationController, CronJob | Yes |
| ReplicaSet or Job that isn't managed by another workload | Yes |
| Any other kind, such as an Argo Rollout or another resource with a `/scale` subresource | No. VPA still updates the pods, but Karpenter sizes nodes using their current requests and emits a `PredictionTargetUnsupported` warning event on the VPA. |

When it stores a prediction, Karpenter emits a `PredictionStored` event on the VPA.

If more than one VPA targets the same workload, Karpenter uses the oldest one, and breaks ties by name. This matches VPA's own behavior.

## Before you enable it

* Install VPA, with at least its CRDs and recommender. Without the VPA CRD, Karpenter runs normally and doesn't use predictions. It checks for the CRD every minute.
* Karpenter needs `get`, `list`, and `watch` on `verticalpodautoscalers`, `replicasets`, and `jobs`. The Helm chart grants them. If you manage Karpenter's RBAC yourself, add them before you enable the gate.

{{% alert title="Warning" color="warning" %}}
With the gate enabled, provisioning and disruption wait until Karpenter has read the cluster's VPAs. If Karpenter can't list VPAs within a minute, for example because of missing RBAC, the controller panics and restarts.
{{% /alert %}}

## Disabling it

Predictions are held only in memory. Disabling the feature gate stops Karpenter from using them, and nothing needs to be cleaned up.
