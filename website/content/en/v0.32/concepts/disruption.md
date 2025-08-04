---
title: "Disruption"
linkTitle: "Disruption"
weight: 4
description: >
  Understand different ways Karpenter disrupts nodes
---

## Control Flow

Karpenter sets a Kubernetes [finalizer](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/) on each node and node claim it provisions.
The finalizer blocks deletion of the node object while the Termination Controller taints and drains the node, before removing the underlying NodeClaim. Disruption is triggered by the Disruption Controller, by the user through manual disruption, or through an external system that sends a delete request to the node object.

### Disruption Controller

Karpenter automatically discovers disruptable nodes and spins up replacements when needed. Karpenter disrupts nodes by executing one [automated method](#automated-methods) at a time, in order of Expiration, Drift, and then Consolidation. Each method varies slightly, but they all follow the standard disruption process:
1. Identify a list of prioritized candidates for the disruption method.
   * If there are [pods that cannot be evicted](#pod-eviction) on the node, Karpenter will ignore the node and try disrupting it later.testts
   * If there are no disruptable nodes, continue to the next disruption method.
2. For each disruptable node, execute a scheduling simulation with the pods on the node to find if any replacement nodes are needed.
3. Add the `karpenter.sh/disruption:NoSchedule` taint to the node(s) to prevent pods from scheduling to it.
4. Pre-spin any replacement nodes needed as calculated in Step (2), and wait for them to become ready.
   * If a replacement node fails to initialize, un-taint the node(s), and restart from Step (1), starting at the first disruption method again.
5. Delete the node(s) and wait for the Termination Controller to gracefully shutdown the node(s).
6. Once the Termination Controller terminates the node, go back to Step (1), starting at the first disruption method again.

### Termination Controller

When a Karpenter node is deleted, the Karpenter finalizer will block deletion and the APIServer will set the `DeletionTimestamp` on the node, allowing Karpenter to gracefully shutdown the node, modeled after [Kubernetes Graceful Node Shutdown](https://kubernetes.io/docs/concepts/cluster-administration/node-shutdown/#graceful-node-shutdown). Karpenter's graceful shutdown process will:
1. Add the `karpenter.sh/disruption:NoSchedule` taint to the node to prevent pods from scheduling to it.
2. Begin evicting the pods on the node with the [Kubernetes Eviction API](https://kubernetes.io/docs/concepts/scheduling-eviction/api-eviction/) to respect PDBs, while ignoring all daemonset pods and [static pods](https://kubernetes.io/docs/tasks/configure-pod-container/static-pod/). Wait for the node to be fully drained before proceeding to Step (3).
   * While waiting, if the underlying NodeClaim for the node no longer exists, remove the finalizer to allow the APIServer to delete the node, completing termination.
3. Terminate the NodeClaim in the Cloud Provider.
4. Remove the finalizer from the node to allow the APIServer to delete the node, completing termination.

## Manual Methods
* **Node Deletion**: You can use `kubectl` to manually remove a single Karpenter node or nodeclaim. Since each Karpenter node is owned by a NodeClaim, deleting either the node or the nodeclaim will cause cascade deletion of the other:

    ```bash
    # Delete a specific nodeclaim
    kubectl delete nodeclaim $NODECLAIM_NAME

    # Delete a specific node
    kubectl delete node $NODE_NAME

    # Delete all nodeclaims
    kubectl delete nodeclaims --all

    # Delete all nodes owned by any nodepool
    kubectl delete nodes -l karpenter.sh/nodepool

    # Delete all nodeclaims owned by a specific nodepoolXS
    kubectl delete nodeclaims -l karpenter.sh/nodepool=$NODEPOOL_NAME
    ```
* **NodePool Deletion**: NodeClaims are owned by the NodePool through an [owner reference](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/#owner-references-in-object-specifications) that launched them. Karpenter will gracefully terminate nodes through cascading deletion when the owning NodePool is deleted.

{{% alert title="Note" color="primary" %}}
By adding the finalizer, Karpenter improves the default Kubernetes process of node deletion.
When you run `kubectl delete node` on a node without a finalizer, the node is deleted without triggering the finalization logic. The instance will continue running in EC2, even though there is no longer a node object for it. The kubelet isn’t watching for its own existence, so if a node is deleted, the kubelet doesn’t terminate itself. All the pod objects get deleted by a garbage collection process later, because the pods’ node is gone.
{{% /alert %}}

## Automated Methods

* **Expiration**: Karpenter will mark nodes as expired and disrupt them after they have lived a set number of seconds, based on the NodePool's `spec.disruption.expireAfter` value. You can use node expiry to periodically recycle nodes due to security concerns.
* [**Consolidation**]({{<ref "#consolidation" >}}): Karpenter works to actively reduce cluster cost by identifying when:
  * Nodes can be removed because the node is empty
  * Nodes can be removed as their workloads will run on other nodes in the cluster.
  * Nodes can be replaced with cheaper variants due to a change in the workloads.
* [**Drift**]({{<ref "#drift" >}}): Karpenter will mark nodes as drifted and disrupt nodes that have drifted from their desired specification. See [Drift]({{<ref "#drift" >}}) to see which fields are considered.
* [**Interruption**]({{<ref "#interruption" >}}): Karpenter will watch for upcoming interruption events that could affect your nodes (health events, spot interruption, etc.) and will taint, drain, and terminate the node(s) ahead of the event to reduce workload disruption.

{{% alert title="Defaults" color="secondary" %}}
Disruption is configured through the NodePool's disruption block by the `consolidationPolicy`, `expireAfter` and `consolidateAfter` fields. Karpenter will configure these fields with the following values by default if they are not set:

```yaml
spec:
  disruption:
    consolidationPolicy: WhenUnderutilized
    expireAfter: 720h
```
{{% /alert %}}

### Consolidation

Karpenter has two mechanisms for cluster consolidation:
1. **Deletion** - A node is eligible for deletion if all of its pods can run on free capacity of other nodes in the cluster.
2. **Replace** - A node can be replaced if all of its pods can run on a combination of free capacity of other nodes in the cluster and a single cheaper replacement node.

Consolidation has three mechanisms that are performed in order to attempt to identify a consolidation action:
1. **Empty Node Consolidation** - Delete any entirely empty nodes in parallel
2. **Multi Node Consolidation** - Try to delete two or more nodes in parallel, possibly launching a single replacement that is cheaper than the price of all nodes being removed
3. **Single Node Consolidation** - Try to delete any single node, possibly launching a single replacement that is cheaper than the price of that node

It's impractical to examine all possible consolidation options for multi-node consolidation, so Karpenter uses a heuristic to identify a likely set of nodes that can be consolidated.  For single-node consolidation we consider each node in the cluster individually.

When there are multiple nodes that could be potentially deleted or replaced, Karpenter chooses to consolidate the node that overall disrupts your workloads the least by preferring to terminate:

* Nodes running fewer pods
* Nodes that will expire soon
* Nodes with lower priority pods

If consolidation is enabled, Karpenter periodically reports events against nodes that indicate why the node can't be consolidated.  These events can be used to investigate nodes that you expect to have been consolidated, but still remain in your cluster.

```bash
Events:
  Type     Reason                   Age                From             Message
  ----     ------                   ----               ----             -------
  Normal   Unconsolidatable         66s                karpenter        pdb default/inflate-pdb prevents pod evictions
  Normal   Unconsolidatable         33s (x3 over 30m)  karpenter        can't replace with a cheaper node
```

{{% alert title="Warning" color="warning" %}}
Using preferred anti-affinity and topology spreads can reduce the effectiveness of consolidation. At node launch, Karpenter attempts to satisfy affinity and topology spread preferences. In order to reduce node churn, consolidation must also attempt to satisfy these constraints to avoid immediately consolidating nodes after they launch. This means that consolidation may not disrupt nodes in order to avoid violating preferences, even if kube-scheduler can fit the host pods elsewhere.  Karpenter reports these pods via logging to bring awareness to the possible issues they can cause (e.g. `pod default/inflate-anti-self-55894c5d8b-522jd has a preferred Anti-Affinity which can prevent consolidation`).
{{% /alert %}}

{{% alert title="Note" color="primary" %}}
For spot nodes, Karpenter only uses the deletion consolidation mechanism.  It will not replace a spot node with a cheaper spot node.  Spot instance types are selected with the `price-capacity-optimized` strategy and often the cheapest spot instance type is not launched due to the likelihood of interruption. Consolidation would then replace the spot instance with a cheaper instance negating the `price-capacity-optimized` strategy entirely and increasing interruption rate.
{{% /alert %}}

### Drift
Drift handles changes to the NodePool/EC2NodeClass. For Drift, values in the NodePool/EC2NodeClass are reflected in the NodeClaimTemplateSpec/EC2NodeClassSpec in the same way that they’re set. A NodeClaim will be detected as drifted if the values in its owning NodePool/EC2NodeClass do not match the values in the NodeClaim. Similar to the upstream `deployment.spec.template` relationship to pods, Karpenter will annotate the owning NodePool and EC2NodeClass with a hash of the NodeClaimTemplateSpec to check for drift. Some special cases will be discovered either from Karpenter or through the CloudProvider interface, triggered by NodeClaim/Instance/NodePool/EC2NodeClass changes.

#### Special Cases on Drift
In special cases, drift can correspond to multiple values and must be handled differently. Drift on resolved fields can create cases where drift occurs without changes to CRDs, or where CRD changes do not result in drift. For example, if a NodeClaim has `node.kubernetes.io/instance-type: m5.large`, and requirements change from `node.kubernetes.io/instance-type In [m5.large]` to `node.kubernetes.io/instance-type In [m5.large, m5.2xlarge]`, the NodeClaim will not be drifted because its value is still compatible with the new requirements. Conversely, if a NodeClaim is using a NodeClaim image `ami: ami-abc`, but a new image is published, Karpenter's `EC2NodeClass.spec.amiSelectorTerms` will discover that the new correct value is `ami: ami-xyz`, and detect the NodeClaim as drifted.

##### NodePool
| Fields         |
|----------------|
| spec.template.spec.requirements   |

##### EC2NodeClass
| Fields                        |
|-------------------------------|
| spec.subnetSelectorTerms      |
| spec.securityGroupSelectorTerms  |
| spec.amiSelectorTerms  |

#### Behavioral Fields
Behavioral Fields are treated as over-arching settings on the NodePool to dictate how Karpenter behaves. These fields don’t correspond to settings on the NodeClaim or instance. They’re set by the user to control Karpenter’s Provisioning and disruption logic. Since these don’t map to a desired state of NodeClaims, __behavioral fields are not considered for Drift__.

##### NodePool
| Fields         |
|----------------|
| spec.weight         |
| spec.limits         |
| spec.disruption.*   |

Read the [Drift Design](https://github.com/aws/karpenter-core/blob/main/designs/drift.md) for more.

To enable the drift feature flag, refer to the [Feature Gates]({{<ref "../reference/settings#feature-gates" >}}).

Karpenter will add the `Drifted` status condition on NodeClaims if the NodeClaim is drifted from its owning NodePool. Karpenter will also remove the `Drifted` status condition if either:
1. The `Drift` feature gate is not enabled but the NodeClaim is drifted, Karpenter will remove the status condition.
2. The NodeClaim isn't drifted, but has the status condition, Karpenter will remove it.

### Interruption

If interruption-handling is enabled, Karpenter will watch for upcoming involuntary interruption events that would cause disruption to your workloads. These interruption events include:

* Spot Interruption Warnings
* Scheduled Change Health Events (Maintenance Events)
* Instance Terminating Events
* Instance Stopping Events

When Karpenter detects one of these events will occur to your nodes, it automatically taints, drains, and terminates the node(s) ahead of the interruption event to give the maximum amount of time for workload cleanup prior to compute disruption. This enables scenarios where the `terminationGracePeriod` for your workloads may be long or cleanup for your workloads is critical, and you want enough time to be able to gracefully clean-up your pods.

For Spot interruptions, the NodePool will start a new node as soon as it sees the Spot interruption warning. Spot interruptions have a __2 minute notice__ before Amazon EC2 reclaims the instance. Once Karpenter has received this warning it will begin draining the node while in parallel provisioning a new node. Karpenter's average node startup time means that, generally, there is sufficient time for the new node to become ready before EC2 initiates termination for the spot instance.

{{% alert title="Note" color="primary" %}}
Karpenter publishes Kubernetes events to the node for all events listed above in addition to [__Spot Rebalance Recommendations__](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/rebalance-recommendations.html). Karpenter does not currently support taint, drain, and terminate logic for Spot Rebalance Recommendations.

If you require handling for Spot Rebalance Recommendations, you can use the [AWS Node Termination Handler (NTH)](https://github.com/aws/aws-node-termination-handler) alongside Karpenter; however, note that the AWS Node Termination Handler cordons and drains nodes on rebalance recommendations, potentially causing more node churn in the cluster than with interruptions alone. Further information can be found in the [Troubleshooting Guide]({{< ref "../troubleshooting#aws-node-termination-handler-nth-interactions" >}}).
{{% /alert %}}

Karpenter enables this feature by watching an SQS queue which receives critical events from AWS services which may affect your nodes. Karpenter requires that an SQS queue be provisioned and EventBridge rules and targets be added that forward interruption events from AWS services to the SQS queue. Karpenter provides details for provisioning this infrastructure in the [CloudFormation template in the Getting Started Guide](../../getting-started/getting-started-with-karpenter/#create-the-karpenter-infrastructure-and-iam-roles).

To enable interruption handling, configure the `--interruption-queue` CLI argument with the name of the interruption queue provisioned to handle interruption events.

## Controls

### Pod-Level Controls

Pods with blocking PDBs will not be evicted by the [Termination Controller]({{<ref "#termination-controller">}}) or be considered for voluntary disruption actions. When multiple pods on a node have different PDBs, none of the PDBs may be blocking for Karpenter to voluntary disrupt a node. This can create complex eviction scenarios:
  - If a pod matches multiple PDBs (via label selectors), ALL of these PDBs must allow for disruption
  - When different pods on the same node belong to different PDBs, ALL PDBs must simultaneously permit eviction
  - A single blocking PDB can prevent the entire node from being voluntary disrupted

For example, consider a node with these pods and PDBs:
- Pod A: Matches PDB-1 (maxUnavailable: 0) and PDB-2 (maxUnavailable: 1)
- Pod B: Matches PDB-3 (minAvailable: 100%)
- Pod C: No PDB

In this scenario, Karpenter cannot voluntary disrupt the node because:
1. Pod A is blocked by PDB-1 even though PDB-2 would allow disruption
2. Pod B is blocked by PDB-3's requirement for 100% availability

As seen in this example, the more PDBs there are affecting a Node, the more difficult it will be for Karpenter to find an opportunity to perform voluntary disruption actions. 

Secondly, you can block Karpenter from voluntarily choosing to disrupt certain pods by setting the `karpenter.sh/do-not-disrupt: "true"` annotation on the pod. This is useful for pods that you want to run from start to finish without disruption. By opting pods out of this disruption, you are telling Karpenter that it should not voluntarily remove a node containing this pod.

Examples of pods that you might want to opt-out of disruption include an interactive game that you don't want to interrupt or a long batch job (such as you might have with machine learning) that would need to start over if it were interrupted.

```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    metadata:
      annotations:
        karpenter.sh/do-not-disrupt: "true"
```

{{% alert title="Note" color="primary" %}}
This annotation will be ignored for [terminating pods](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-phase), [terminal pods](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-phase) (Failed/Succeeded), [DaemonSet pods](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/), or [static pods](https://kubernetes.io/docs/tasks/configure-pod-container/static-pod/).
{{% /alert %}}

Examples of voluntary node removal that will be prevented by this annotation include:
- [Consolidation]({{<ref "#consolidation" >}})
- [Drift]({{<ref "#drift" >}})
- Expiration

{{% alert title="Note" color="primary" %}}
Voluntary node removal does not include [Interruption]({{<ref "#interruption" >}}) or manual deletion initiated through `kubectl delete node`. Both of these are considered involuntary events, since node removal cannot be delayed.
{{% /alert %}}

### Node-Level Controls

You can block Karpenter from voluntarily choosing to disrupt certain nodes by setting the `karpenter.sh/do-not-disrupt: "true"` annotation on the node. This will prevent disruption actions on the node.

```yaml
apiVersion: v1
kind: Node
metadata:
  annotations:
    karpenter.sh/do-not-disrupt: "true"
```

#### Example: Disable Disruption on a NodePool

NodePool `.spec.annotations` allow you to set annotations that will be applied to all nodes launched by this NodePool. By setting the annotation `karpenter.sh/do-not-disrupt: "true"` on the NodePool, you will selectively prevent all nodes launched by this NodePool from being considered in disruption actions.

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: default
spec:
  template:
    metadata:
      annotations: # will be applied to all nodes
        karpenter.sh/do-not-disrupt: "true"
```
