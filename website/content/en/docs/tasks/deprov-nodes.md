---
title: "Deprovisioning"
linkTitle: "Deprovisioning"
weight: 10
---

Karpenter sets a Kubernetes [finalizer](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/) on each node it provisions.
The finalizer specifies additional actions the Karpenter controller will takes in response to a node deletion request.
These include:

* Marking the node as unschedulable, so no further pods can be deployed there.
* Evicting all workload pods from the node, draining it of all pods other than daemonsets.
* Deleting the node from the Kubernetes cluster.
* Terminating the instance from the cloud provider.

## How Karpenter nodes are deprovisioned

There are both automated and manual ways of deprovisioning nodes provisioned by Karpenter:

* **Node empty**: Karpenter notes when the last workload (non-daemonset) pod stops running on a node. From that point, Karpenter waits the number of seconds set by `ttlSecondsAfterEmpty`  in the provisioner, then Karpenter requests to delete the node. This feature can keep costs down by removing nodes that are no longer being used for workloads.
* **Node expired**: Karpenter asks to delete the node after a set number of seconds, based on the provisioner `ttlSecondsUntilExpired`  value, from the time the node was provisioned. One use case for node expiry is to handle node upgrades. Old nodes (with a potentially outdated Kubernetes version or operating system) are deleted, and replaced with nodes on the current version.

    {{% alert title="Note" color="primary" %}}
    Keep in mind that a small NodeExpiry results in a higher churn in cluster activity. So, for example, if a cluster brings up all nodes at once, all the pods on those nodes would fall into the same batching window on expiration.
    {{% /alert %}}
    
* **Node deleted**: You could use `kubectl` to remove a single Karpenter node:

    ```bash
    kubectl delete <nodename>
    ```

    Or delete all Karpenter nodes at once:

    ```bash
    kubectl delete -l <provisioner label>
 
    ```

Whether through node expiry or manual deletion, Karpenter seeks to follow graceful termination procedures as described in Kubernetes [Graceful node shutdown](https://kubernetes.io/docs/concepts/architecture/nodes/#graceful-node-shutdow) documentation.

{{% alert title="Note" color="primary" %}}
By adding the finalizer, Karpenter improves the default Kubernetes process of node deletion.
When you run `kubectl delete node` on a node without a finalizer, the node gets deleted in the API server but the instance keeps running.
The kubelet isn’t watching for its own existence, so if a node is deleted the kubelet doesn’t terminate itself.
All the pod objects get deleted a minute later, from a garbage collection process because the pods’ nodes are gone.
{{% /alert %}}

## What can cause deprovisioning to fail?

There are a few cases where requesting to deprovision a Karpenter node will fail. These include Pod Disruption Budgets and pods that have the `do-not-evict` annotation set.

### Disruption budgets

Karpenter respects Pod Disruption Budgets (PDBs) when provisioning nodes and can prevent a node from being deprovisioned to maintain those disruption budgets.
Kubernetes PDBs let you specify how much of a Deployment, ReplicationController, ReplicaSet, or StatefulSet must be protected from disruptions when pod eviction requests are made. 

PDBs can be used to strike a balance by protecting the application's availability while still allowing a cluster operator to manage the cluster.
Here is an example where the pods matching the label `myapp` will block node termination if evicting the pod would reduce the number of available pods below 4.

```bash
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

Karpenter will not drain a node if a pod exists with the annotation `karpenter.sh/do-not-evict`.
The node will be cordoned to prevent additional work from scheduling.
That annotation is used for pods that you want to run on one node from start to finish without interruption.
Examples might include a real-time, interactive game that you don't want to interrupt or a long batch job (such as you might have with machine learning) that would need to start over if it were interrupted.

If you want to terminate a `do-not-evict` pod, you can simply remove the annotation and the finalizer will delete the pod and continue the node deprovisioning process.

