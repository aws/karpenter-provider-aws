---
title: "Deprovisioning"
linkTitle: "Deprovisioning"
weight: 10
description: >
  Understand different ways Karpenter deprovisions nodes
---

Karpenter sets a Kubernetes [finalizer](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/) on each node it provisions.
The finalizer specifies additional actions the Karpenter controller will take in response to a node deletion request.
These include:

* Marking the node as unschedulable, so no further pods can be scheduled there.
* Evicting all pods other than daemonsets from the node.
* Terminating the instance from the cloud provider.
* Deleting the node from the Kubernetes cluster.

## How Karpenter nodes are deprovisioned

There are both automated and manual ways of deprovisioning nodes provisioned by Karpenter:

* **Provisioner Deletion**: Nodes are considered to be "owned" by the Provisioner that launched them. Karpenter will gracefully terminate nodes when a provisioner is deleted. Nodes may be reparented to another provisioner by modifying their labels. For example: `kubectl label node -l karpenter.sh/provisioner-name=source-provisioner-name karpenter.sh/provisioner-name=destination-provisioner-name --overwrite`.
* **Node empty**: Karpenter notes when the last workload (non-daemonset) pod stops running on a node. From that point, Karpenter waits the number of seconds set by `ttlSecondsAfterEmpty` in the provisioner, then Karpenter requests to delete the node. This feature can keep costs down by removing nodes that are no longer being used for workloads.
* **Node expired**: Karpenter requests to delete the node after a set number of seconds, based on the provisioner `ttlSecondsUntilExpired`  value, from the time the node was provisioned. One use case for node expiry is to handle node upgrades. Old nodes (with a potentially outdated Kubernetes version or operating system) are deleted, and replaced with nodes on the current version (assuming that you requested the latest version, rather than a specific version).
* **Consolidation**: Karpenter works to actively reduce cluster cost by identifying when nodes can be removed as their workloads will run on other nodes in the cluster and when nodes can be replaced with cheaper variants due to a change in the workloads.
* **Interruption**: If enabled, Karpenter will watch for upcoming involuntary interruption events that could affect your nodes (health events, spot interruption, etc.) and will cordon, drain, and terminate the node(s) ahead of the event to reduce workload disruption.

{{% alert title="Note" color="primary" %}}
- Automated deprovisioning is configured through the ProvisionerSpec `.ttlSecondsAfterEmpty`
, `.ttlSecondsUntilExpired` and `.consolidation.enabled` fields. If these are not configured, Karpenter will not
default values for them and will not terminate nodes for that purpose.

- Keep in mind that a small NodeExpiry results in a higher churn in cluster activity. So, for
example, if a cluster brings up all nodes at once, all the pods on those nodes would fall into
the same batching window on expiration.

- Pods without an ownerRef (also called "controllerless" or "naked" pods) will be evicted during voluntary node disruption, such as expiration or consolidation. A pod with the annotation `karpenter.sh/do-not-evict: true` will cause its node to be opted out from voluntary node disruption workflows.
{{% /alert %}}

* **Node deleted**: You could use `kubectl` to manually remove a single Karpenter node:

    ```bash
    # Delete a specific node
    kubectl delete node $NODE_NAME

    # Delete all nodes owned any provisioner
    kubectl delete nodes -l karpenter.sh/provisioner-name

    # Delete all nodes owned by a specific provisioner
    kubectl delete nodes -l karpenter.sh/provisioner-name=$PROVISIONER_NAME
    ```

Whether through node expiry or manual deletion, Karpenter seeks to follow graceful termination procedures as described in Kubernetes [Graceful node shutdown](https://kubernetes.io/docs/concepts/architecture/nodes/#graceful-node-shutdown) documentation.
If the Karpenter controller is removed or fails, the finalizers on the nodes are orphaned and will require manual removal.


{{% alert title="Note" color="primary" %}}
By adding the finalizer, Karpenter improves the default Kubernetes process of node deletion.
When you run `kubectl delete node` on a node without a finalizer, the node is deleted without triggering the finalization logic. The instance will continue running in EC2, even though there is no longer a node object for it.
The kubelet isn’t watching for its own existence, so if a node is deleted the kubelet doesn’t terminate itself.
All the pod objects get deleted by a garbage collection process later, because the pods’ node is gone.
{{% /alert %}}

## Consolidation

Karpenter has two mechanisms for cluster consolidation:
- Deletion - A node is eligible for deletion if all of its pods can run on free capacity of other nodes in the cluster.  
- Replace - A node can be replaced if all of its pods can run on a combination of free capacity of other nodes in the cluster and a single cheaper replacement node. 

When there are multiple nodes that could be potentially deleted or replaced, Karpenter choose to consolidate the node that overall disrupts your workloads the least by preferring to terminate:

* nodes running fewer pods
* nodes that will expire soon
* nodes with lower priority pods

{{% alert title="Note" color="primary" %}}
For spot nodes, Karpenter only uses the deletion consolidation mechanism.  It will not replace a spot node with a cheaper spot node.  Spot instance types are selected with the `price-capacity-optimized` strategy and often the cheapest spot instance type is not launched due to the likelihood of interruption. Consolidation would then replace the spot instance with a cheaper instance negating the `price-capacity-optimized` strategy entirely and increasing interruption rate.  
{{% /alert %}}

## Interruption

If interruption-handling is enabled, Karpenter will watch for upcoming involuntary interruption events that would cause disruption to your workloads. These interruption events include:

* Spot Interruption Warnings
* Scheduled Change Health Events (Maintenance Events)
* Instance Terminating Events
* Instance Stopping Events

When Karpenter detects one of these events will occur to your nodes, it automatically cordons, drains, and terminates the node(s) ahead of the interruption event to give the maximum amount of time for workload cleanup prior to compute disruption. This enables scenarios where the `terminationGracePeriod` for your workloads may be long or cleanup for your workloads is critical, and you want enough time to be able to gracefully clean-up your pods.

{{% alert title="Note" color="warning" %}}
Karpenter publishes Kubernetes events to the node for all events listed above in addition to __Spot Rebalance Recommendations__. Karpenter does not currently support cordon, drain, and terminate logic for Spot Rebalance Recommendations.
{{% /alert %}}

Karpenter enables this feature by watching an SQS queue which receives critical events from AWS services which may affect your nodes. Karpenter requires that an SQS queue be provisioned and EventBridge rules and targets be added that forward interruption events from AWS services to the SQS queue. Karpenter provides details for provisioning this infrastructure in the [Cloudformation template in the Getting Started Guide](../../getting-started/getting-started-with-eksctl/#create-the-karpenter-infrastructure-and-iam-roles).

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

## What can cause deprovisioning to fail?

There are a few cases where requesting to deprovision a Karpenter node will fail. These include Pod Disruption Budgets and pods that have the `do-not-evict` annotation set.

### Disruption budgets

Karpenter respects Pod Disruption Budgets (PDBs) by using a backoff retry eviction strategy. Pods will never be forcibly deleted, so pods that fail to shut down will prevent a node from deprovisioning.
Kubernetes PDBs let you specify how much of a Deployment, ReplicationController, ReplicaSet, or StatefulSet must be protected from disruptions when pod eviction requests are made.

PDBs can be used to strike a balance by protecting the application's availability while still allowing a cluster administrator to manage the cluster.
Here is an example where the pods matching the label `myapp` will block node termination if evicting the pod would reduce the number of available pods below 4.

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: myapp-pdb
spec:
  minAvailable: 4
  selector:
    matchLabels:
      app: myapp
```

You can set `minAvailable` or `maxUnavailable` as integers or as a percentage.
Review what [disruptions are](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/), and [how to configure them](https://kubernetes.io/docs/tasks/run-application/configure-pdb/).

### Pod set to do-not-evict

If a pod exists with the annotation `karpenter.sh/do-not-evict: true` on a node, and a request is made to delete the node, Karpenter will not drain any pods from that node or otherwise try to delete the node. Nodes that have pods with a `do-not-evict` annotation are not considered for consolidation, though their unused capacity is considered for the purposes of running pods from other nodes which can be consolidated. This annotation will have no effect for static pods, pods that tolerate `NoSchedule`, or pods terminating past their graceful termination period.

This is useful for pods that you want to run from start to finish without interruption.
Examples might include a real-time, interactive game that you don't want to interrupt or a long batch job (such as you might have with machine learning) that would need to start over if it were interrupted.

If you want to terminate a node with a `do-not-evict` pod, you can simply remove the annotation and the deprovisioning process will continue.

### Scheduling Constraints (Consolidation Only)

Consolidation will be unable to consolidate a node if, as a result of its scheduling simulation, it determines that the pods on a node cannot run on other nodes due to inter-pod affinity/anti-affinity, topology spread constraints, or some other scheduling restriction that couldn't be fulfilled.
