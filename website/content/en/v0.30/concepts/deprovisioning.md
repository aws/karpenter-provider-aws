---
title: "Deprovisioning"
linkTitle: "Deprovisioning"
weight: 4
description: >
  Understand different ways Karpenter deprovisions nodes
---

## Control Flow
Karpenter sets a Kubernetes [finalizer](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/) on each node it provisions.
The finalizer blocks deletion of the node object while the Termination Controller cordons and drains the node, before removing the underlying machine. Deprovisioning is triggered by the Deprovisioning Controller, by the user through manual deprovisioning, or through an external system that sends a delete request to the node object.

### Deprovisioning Controller
Karpenter automatically discovers deprovisionable nodes and spins up replacements when needed. Karpenter deprovisions nodes by executing one [automatic method](#methods) at a time, in order of Expiration, Drift, Emptiness, and then Consolidation. Each method varies slightly but they all follow the standard deprovisioning process:
1. Identify a list of prioritized candidates for the deprovisioning method.
   * If there are [pods that cannot be evicted](#pod-eviction) on the node, Karpenter will ignore the node and try deprovisioning it later.
   * If there are no deprovisionable nodes, continue to the next deprovisioning method.
2. For each deprovisionable node, execute a scheduling simulation with the pods on the node to find if any replacement nodes are needed.
3. Cordon the node(s) to prevent pods from scheduling to it.
4. Pre-spin any replacement nodes needed as calculated in Step (2), and wait for them to become ready.
   * If a replacement node fails to initialize, un-cordon the node(s), and restart from Step (1), starting at the first deprovisioning method again.
5. Delete the node(s) and wait for the Termination Controller to gracefully shutdown the node(s).
6. Once the Termination Controller terminates the node, go back to Step (1), starting at the the first deprovisioning method again.

### Termination Controller
When a Karpenter node is deleted, the Karpenter finalizer will block deletion and the APIServer will set the `DeletionTimestamp` on the node, allowing Karpenter to gracefully shutdown the node, modeled after [K8s Graceful Node Shutdown](https://kubernetes.io/docs/concepts/architecture/nodes/#graceful-node-shutdown). Karpenter's graceful shutdown process will:
1. Cordon the node to prevent pods from scheduling to it.
2. Begin evicting the pods on the node with the [K8s Eviction API](https://kubernetes.io/docs/concepts/scheduling-eviction/api-eviction/) to respect PDBs, while ignoring all non-daemonset pods and [static pods](https://kubernetes.io/docs/tasks/configure-pod-container/static-pod/). Wait for the node to be fully drained before proceeding to Step (3).
   * While waiting, if the underlying machine for the node no longer exists, remove the finalizer to allow the APIServer to delete the node, completing termination.
3. Terminate the machine in the Cloud Provider.
4. Remove the finalizer from the node to allow the APIServer to delete the node, completing termination.

## Methods

There are both automated and manual ways of deprovisioning nodes provisioned by Karpenter:

### Manual Methods
* **Node Deletion**: You could use `kubectl` to manually remove a single Karpenter node:

    ```bash
    # Delete a specific node
    kubectl delete node $NODE_NAME

    # Delete all nodes owned any provisioner
    kubectl delete nodes -l karpenter.sh/provisioner-name

    # Delete all nodes owned by a specific provisioner
    kubectl delete nodes -l karpenter.sh/provisioner-name=$PROVISIONER_NAME
    ```
* **Provisioner Deletion**: Nodes are owned by the Provisioner through an [owner reference](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/#owner-references-in-object-specifications) that launched them. Karpenter will gracefully terminate nodes through cascading deletion when the owning provisioner is deleted.

### Automated Methods
* **Emptiness**: Karpenter notes when the last workload (non-daemonset) pod stops running on a node. From that point, Karpenter waits the number of seconds set by `ttlSecondsAfterEmpty` in the provisioner, then Karpenter requests to delete the node. This feature can keep costs down by removing nodes that are no longer being used for workloads.
* **Expiration**: Karpenter will annotate nodes as expired and deprovision nodes after they have lived a set number of seconds, based on the provisioner `ttlSecondsUntilExpired` value. One use case for node expiry is to periodically recycle nodes. Old nodes (with a potentially outdated Kubernetes version or operating system) are deleted, and replaced with nodes on the current version (assuming that you requested the latest version, rather than a specific version).
* **Consolidation**: Karpenter works to actively reduce cluster cost by identifying when:
  * Nodes can be removed as their workloads will run on other nodes in the cluster.
  * Nodes can be replaced with cheaper variants due to a change in the workloads.
* **Drift**: Karpenter will annotate nodes as drifted and deprovision nodes that have drifted from their desired specification. See [Drift]({{<ref "./deprovisioning/#drift" >}})  to see which fields are considered.
* **Interruption**: If enabled, Karpenter will watch for upcoming involuntary interruption events that could affect your nodes (health events, spot interruption, etc.) and will cordon, drain, and terminate the node(s) ahead of the event to reduce workload disruption.

{{% alert title="Note" color="primary" %}}
- Automated deprovisioning is configured through the ProvisionerSpec `.ttlSecondsAfterEmpty`, `.ttlSecondsUntilExpired` and `.consolidation.enabled` fields. If these are not configured, Karpenter will not set default values for them and will not terminate nodes for that purpose.

- Keep in mind that a small `ttlSecondsUntilExpired` results in a higher churn in cluster activity. For a small enough `ttlSecondsUntilExpired`, nodes may expire faster than Karpenter can safely deprovision them, resulting in constant node deprovisioning.

- Pods without an ownerRef (also called "controllerless" or "naked" pods) will be evicted during automatic node disruption, besides [Interruption](#interruption). A pod with the annotation `karpenter.sh/do-not-evict: "true"` will cause its node to be opted out from the same deprovisioning methods.

- Using preferred anti-affinity and topology spreads can reduce the effectiveness of consolidation. At node launch, Karpenter attempts to satisfy affinity and topology spread preferences. In order to reduce node churn, consolidation must also attempt to satisfy these constraints to avoid immediately consolidating nodes after they launch. This means that consolidation may not deprovision nodes in order to avoid violating preferences, even if kube-scheduler can fit the host pods elsewhere.  Karpenter reports these pods via logging to bring awareness to the possible issues they can cause (e.g. `pod default/inflate-anti-self-55894c5d8b-522jd has a preferred Anti-Affinity which can prevent consolidation`).

- By adding the finalizer, Karpenter improves the default Kubernetes process of node deletion.
When you run `kubectl delete node` on a node without a finalizer, the node is deleted without triggering the finalization logic. The machine will continue running in EC2, even though there is no longer a node object for it.
The kubelet isn’t watching for its own existence, so if a node is deleted, the kubelet doesn’t terminate itself.
All the pod objects get deleted by a garbage collection process later, because the pods’ node is gone.

{{% /alert %}}

## Consolidation

Karpenter has two mechanisms for cluster consolidation:
- Deletion - A node is eligible for deletion if all of its pods can run on free capacity of other nodes in the cluster.
- Replace - A node can be replaced if all of its pods can run on a combination of free capacity of other nodes in the cluster and a single cheaper replacement node.

Consolidation has three mechanisms that are performed in order to attempt to identify a consolidation action:
1) Empty Node Consolidation - Delete any entirely empty nodes in parallel
2) Multi-Node Consolidation - Try to delete two or more nodes in parallel, possibly launching a single replacement that is cheaper than the price of all nodes being removed
3) Single-Node Consolidation - Try to delete any single node, possibly launching a single replacement that is cheaper than the price of that node

It's impractical to examine all possible consolidation options for multi-node consolidation, so Karpenter uses a heuristic to identify a likely set of nodes that can be consolidated.  For single-node consolidation we consider each node in the cluster individually.

When there are multiple nodes that could be potentially deleted or replaced, Karpenter choose to consolidate the node that overall disrupts your workloads the least by preferring to terminate:

* nodes running fewer pods
* nodes that will expire soon
* nodes with lower priority pods

{{% alert title="Note" color="primary" %}}
For spot nodes, Karpenter only uses the deletion consolidation mechanism.  It will not replace a spot node with a cheaper spot node.  Spot instance types are selected with the `price-capacity-optimized` strategy and often the cheapest spot instance type is not launched due to the likelihood of interruption. Consolidation would then replace the spot instance with a cheaper instance negating the `price-capacity-optimized` strategy entirely and increasing interruption rate.
{{% /alert %}}

If consolidation is enabled, Karpenter periodically reports events against nodes that indicate why the node can't be consolidated.  These events can be used to investigate nodes that you expect to have been consolidated, but still remain in your cluster.

```
Events:
  Type     Reason                   Age                From             Message
  ----     ------                   ----               ----             -------
  Normal   Unconsolidatable         66s                karpenter        pdb default/inflate-pdb prevents pod evictions
  Normal   Unconsolidatable         33s (x3 over 30m)  karpenter        can't replace with a cheaper node
  ```

## Interruption

If interruption-handling is enabled, Karpenter will watch for upcoming involuntary interruption events that would cause disruption to your workloads. These interruption events include:

* Spot Interruption Warnings
* Scheduled Change Health Events (Maintenance Events)
* Instance Terminating Events
* Instance Stopping Events

When Karpenter detects one of these events will occur to your nodes, it automatically cordons, drains, and terminates the node(s) ahead of the interruption event to give the maximum amount of time for workload cleanup prior to compute disruption. This enables scenarios where the `terminationGracePeriod` for your workloads may be long or cleanup for your workloads is critical, and you want enough time to be able to gracefully clean-up your pods.

For Spot interruptions, the provisioner will start a new machine as soon as it sees the Spot interruption warning. Spot interruptions have a __2 minute notice__ before Amazon EC2 reclaims the instance. Karpenter's average node startup time means that, generally, there is sufficient time for the new node to become ready and to move the pods to the new node before the machine is reclaimed.

{{% alert title="Note" color="primary" %}}
Karpenter publishes Kubernetes events to the node for all events listed above in addition to __Spot Rebalance Recommendations__. Karpenter does not currently support cordon, drain, and terminate logic for Spot Rebalance Recommendations.
{{% /alert %}}

Karpenter enables this feature by watching an SQS queue which receives critical events from AWS services which may affect your nodes. Karpenter requires that an SQS queue be provisioned and EventBridge rules and targets be added that forward interruption events from AWS services to the SQS queue. Karpenter provides details for provisioning this infrastructure in the [CloudFormation template in the Getting Started Guide](../../getting-started/getting-started-with-karpenter/#create-the-karpenter-infrastructure-and-iam-roles).

To enable the interruption handling feature flag, configure the `karpenter-global-settings` ConfigMap with the following value mapped to the name of the interruption queue that handles interruption events.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: karpenter-global-settings
  namespace: karpenter
data:
  ...
  aws.interruptionQueueName: karpenter-cluster
  ...
```

### Drift
Drift handles changes to the NodePool/EC2NodeClass. For Drift, values in the NodePool/EC2NodeClass are reflected in the NodeClaimTemplateSpec/EC2NodeClassSpec in the same way that they’re set. A NodeClaim will be detected as drifted if the values in its owning NodePool/EC2NodeClass do not match the values in the NodeClaim. Similar to the upstream `deployment.spec.template` relationship to pods, Karpenter will annotate the owning NodePool and EC2NodeClass with a hash of the NodeClaimTemplateSpec to check for drift. Some special cases will be discovered either from Karpenter or through the CloudProvider interface, triggered by NodeClaim/Instance/NodePool/EC2NodeClass changes.

#### Special Cases on Drift
In special cases, drift can correspond to multiple values and must be handled differently. Drift on resolved fields can create cases where drift occurs without changes to CRDs, or where CRD changes do not result in drift. For example, if a NodeClaim has `node.kubernetes.io/instance-type: m5.large`, and requirements change from `node.kubernetes.io/instance-type In [m5.large]` to `node.kubernetes.io/instance-type In [m5.large, m5.2xlarge]`, the NodeClaim will not be drifted because its value is still compatible with the new requirements. Conversely, if a NodeClaim is using a NodeClaim image `ami: ami-abc`, but a new image is published, Karpenter's `EC2NodeClass.spec.amiSelectorTerms` will discover that the new correct value is `ami: ami-xyz`, and detect the NodeClaim as drifted.

##### NodePool
| Fields         |
|----------------|
| Requirements   |

##### EC2NodeClass
| Fields                        |
|-------------------------------|
| Subnet Selector Terms         |
| Security Group Selector Terms |
| AMI Selector Terms            |

#### Behavioral Fields
Behavioral Fields are treated as over-arching settings on the NodePool to dictate how Karpenter behaves. These fields don’t correspond to settings on the NodeClaim or instance. They’re set by the user to control Karpenter’s Provisioning and disruption logic. Since these don’t map to a desired state of NodeClaims, __behavioral fields are not considered for Drift__.

__Behavioral Fields__
- Weight                      
- Limits                      
- ConsolidationPolicy               
- ConsolidateAfter      
- ExpireAfter  
---      

Read the [Drift Design](https://github.com/aws/karpenter-core/blob/main/designs/drift.md) for more.

To enable the drift feature flag, refer to the [Settings Feature Gates]({{<ref "./settings#feature-gates" >}}).

Karpenter will add `MachineDrifted` status condition on the machines if the machine is drifted, and does not have the status condition,

Karpenter will remove the  `MachineDrifted` status condition for the following these scenarios:
1. The `featureGates.driftEnabled` is not enabled but the machine is drifted, karpenter will remove the status condition.
2. The machine isn't drifted, but has the status condition, karpenter will remove it.

If the node is marked as drifted by another controller, karpenter will do nothing.


## Controls

### Pod-Level Controls

You can block Karpenter from voluntarily choosing to disrupt certain pods by setting the `karpenter.sh/do-not-evict: "true"` annotation on the pod. This is useful for pods that you want to run from start to finish without disruption. By opting pods out of this disruption, you are telling Karpenter that it should not voluntarily remove a node containing this pod.

Examples of pods that you might want to opt-out of disruption include an interactive game that you don't want to interrupt or a long batch job (such as you might have with machine learning) that would need to start over if it were interrupted.

```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    metadata:
      annotations:
        karpenter.sh/do-not-evict: "true"
```

{{% alert title="Note" color="primary" %}}
This annotation will be ignored for [terminating pods](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-phase), [terminal pods](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-phase) (Failed/Succeeded), [DaemonSet pods](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/), or [static pods](https://kubernetes.io/docs/tasks/configure-pod-container/static-pod/).
{{% /alert %}}

Examples of voluntary node removal that will be prevented by this annotation include:
- [Consolidation]({{<ref "#consolidation" >}})
- [Drift]({{<ref "#drift" >}})
- Emptiness
- Expiration

{{% alert title="Note" color="primary" %}}
Voluntary node removal does not include [Interruption]({{<ref "#interruption" >}}) or manual deletion initiated through `kubectl delete node`. Both of these are considered involuntary events, since node removal cannot be delayed.
{{% /alert %}}

### Node-Level Controls

Nodes can be opted out of consolidation deprovisioning by setting the annotation `karpenter.sh/do-not-consolidate: "true"` on the node.

```yaml
apiVersion: v1
kind: Node
metadata:
  annotations:
    karpenter.sh/do-not-consolidate: "true"
```

#### Example: Disable Consolidation on Provisioner

Provisioner `.spec.annotations` allow you to set annotations that will be applied to all nodes launched by this provisioner. By setting the annotation `karpenter.sh/do-not-consolidate: "true"` on the provisioner, you will selectively prevent all nodes launched by this Provisioner from being considered in consolidation calculations.

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: default
spec:
  annotations: # will be applied to all nodes
    karpenter.sh/do-not-consolidate: "true"
```
