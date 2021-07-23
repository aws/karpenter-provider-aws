# Karpenter Graceful Node Termination
*Authors: njtran@*
## Overview
Karpenter's scale down implementation is currently a proof of concept. The reallocation controller implements two actions. First, nodes are elected for termination when there aren't any pods scheduled to them. Second, nodes are cordoned and drained and deleted. Node termination follows cordon and drain [best practices](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/).

This design explores improvements to the termination process and proposes the separation of this logic into a new termination controller, installed as part of Karpenter.

The existing reallocation controller will only be responsible for electing nodes for termination, which will be explored in future designs.

## Requirements
* Terminating a node with Karpenter will not leak the instance
* If termination is requested, the node will eventually terminate
* Users will be able to implement node deletion safeguards
* Termination mechanisms will rate-limit at the pod evictions

## Termination Controller
The termination controller is responsible for gracefully terminating instances. The termination controller draws ideas from the [aws-node-termination-handler](https://github.com/aws/aws-node-termination-handler), [Cluster Autoscaler](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#what-types-of-pods-can-prevent-ca-from-removing-a-node), and [best practices](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/) on drain given by the Kubernetes community.

### Termination Workflow
The current termination workflow finds nodes with the label `karpenter.sh/lifecycle-phase: terminable` then cordons, drains, and deletes them. This naively handles a few error cases, has no user safeguards, and makes no effort to rate limit. The new workflow will improve these but continue to monitor nodes with the same labels.

The new termination process will begin with a node that receives a delete request. After Karpenter validates the request, a Karpenter mutating webhook will add the Karpenter [finalizer](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#finalizers) to the node. Then, we cordon and begin draining the node. After no pods remain, we terminate the instance in the cloud provider, then remove the Karpenter finalizer. This will result in the APIServer deleting the node object.
![](../website/static/termination-state-machine.png)
### Triggering Termination

The current termination process acts on a reconcile loop. We will change the termination controller to watch nodes and manage the Karpenter [finalizer](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#finalizers), making it responsible for all node termination and pod eviction logic.

Finalizers allow controllers to implement asynchronous pre-deletion hooks and are commonly used with CRDs like Karpenter’s Provisioners. Today, a user can call `kubectl delete node` to delete a node object, but will end up leaking the underlying instance by only deleting the node object in the cluster. We will use finalizers to gracefully terminate underlying instances before Karpenter provisioned nodes are deleted, preventing instance leaking. Relying on `kubectl` for terminations gives the user more control over their cluster and a Kubernetes-native way of deleting nodes - as opposed to the status quo  of doing it manually in a cloud provider's console.

We will additionally implement a Karpenter [Webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) to validate node deletion requests and add finalizers to nodes that have been cleared for deletion. If the request will not violate a Node Disruption Budget (discussed below) and Karpenter is installed, the webhook will add the Karpenter finalizer to nodes and then allow the deletion request to go through, triggering the workflow.

### Eviction

Node termination will only succeed when all pods are evicted. We use the [Kubernetes Eviction API](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/#eviction-api) to handle eviction logic and [Pod Disruption Budget (PDB)](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) violation errors.

The Kubernetes Eviction API will only return a 200, 429, or 500 for each eviction call. A 200 indicates a successful request, requiring no error handling. A 429 indicates that this call would violate a PDB, and a 500 indicates a misconfiguration, such as multiple PDBs referring to the same pod. In the case of a 429 or 500, we will exponentially back off, sending a log in the Karpenter pod.  It’s possible (but very rare) that we violate a PDB with this API by sending multiple eviction requests to different master nodes simultaneously only if each eviction request would not violate a PDB individually but will when combined. This is an API Server limitation and motivated the decision to evict pods serially per PDB to further reduce this chance (discussed below).

While there is the [rare case](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/#stuck-evictions) where stuck evictions require forceful termination, forcefully deleting a pod can have harsh repercussions in many cases. If evicting the pod would violate a PDB, it would mean not supporting PDBs. In addition, terminating pods that cannot afford downtime or [critical pods](https://kubernetes.io/docs/tasks/administer-cluster/guaranteed-scheduling-critical-addon-pods/#marking-pod-as-critical) can have adverse effects on production workloads. Also, if a pod is owned by a StatefulSet, there is never a reason to forcibly delete, as mentioned [here](https://kubernetes.io/docs/tasks/run-application/force-delete-stateful-set-pod/). In the case that a user wants to forcefully terminate a pod, the user can terminate it with `kubectl delete pod <PODNAME> --grace-period=0 --force --namespace <NAMESPACE>` with the information in the Karpenter logs.

After all pods are evicted, we will attempt to terminate the node. If the cloud provider's API returns an error, preventing instance termination, we will log the error in the Karpenter pod and in the node status then exponentially back off and retry.

In the case where a user has enabled termination protection for their underlying instance, Karpenter will be unable to terminate the instance until the termination protection is removed. Since we only terminate instances corresponding to  Karpenter provisioned nodes, a user would have to manually set this protection after Karpenter creates it. In this case, we depend on the user to resolve this with the node status and Karpenter logs.

### User Configuration

The termination controller and associated webhooks will come installed with Karpenter, requiring no additional configuration on the user’s part.

We will allow users to specify a `karpenter.sh/do-not-evict` label on their pods, guaranteeing that we will not evict certain pods. A node with `do-not-evict` pods will cordon but wait to drain until all `do-not-evict` pods are gone. This way, the cluster will continue to utilize its existing capacity until the `do-not-evict` pods terminate. Users can use Karpenter’s scheduling logic to colocate pods with this label onto similar nodes to load balance these pods.

### Parallel Node Termination

The termination controller will be able to drain and terminate multiple nodes in parallel. We will expedite this process while rate limiting node termination to protect against throttling.

We introduce an optional cluster-scoped CRD, the Node Disruption Budget (NDB), a [Pod Disruption Budget (PDB)](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/#pod-disruption-budgets) for nodes. A user can scope an NDB to Karpenter provisioned nodes through a label selector, since nodes are created with their Provisioner’s labels. `Unavailable` nodes will be `NotReady` or have `metadata.DeletionTimestamp` set. `Available` nodes will be `Ready`.

A termination is allowed if at least minAvailable nodes selected by a selector will still be available after the termination. For example, you can prevent all terminations by specifying “100%”. A termination is also allowed if at most maxUnavailable nodes selected by selector are unavailable after the termination. For example, one can prevent all terminations by specifying 0. The `minAvailable` and `maxUnavailable` fields are mutually exclusive.

Note that this is an experimental idea, and will require robustness improvements for future features such as defragmentation, over-provisioning, and more.

[PodDisruptionBudgetSpec](https://pkg.go.dev/k8s.io/api/policy/v1beta1#PodDisruptionBudgetSpec) for reference.

```{go}
// .go
type NodeDisruptionBudget struct {
   // +optional
   MinAvailable   *intstr.IntOrString   `json:"minAvailable,omitempty" protobuf:"bytes,1,opt,name=minAvailable"`
   // Node Selector
   // Label query over nodes managed by the Disruption Budget
   // A null selector selects no nodes.
   // An empty selector ({}) also selects no nodes, which differs from standard behavior of selecting all nodes.
   // +patchStrategy=replace
   // +optional
   Selector       *metav1.LabelSelector `json:"selector,omitempty" patchStrategy:"replace" protobuf:"bytes,2,opt,name=selector"`
   // +optional
   MaxUnavailable *intstr.IntOrString   `json:"maxUnavailable,omitempty" protobuf:"bytes,1,opt,name=minAvailable"`
}
```

```{yaml}
# .yaml
apiVersion: termination.karpenter.sh/v1alpha2
kind: NodeDisruptionBudget
metadata:
  name: nodeBudget
spec:
  minAvailable: 80%
  selector:
    matchLabels:
      karpenter.sh/name: default
      karpenter.sh/namespace: default
  maxUnavailable: 5%
```

Since most eviction failures come from a PDB violation, we will queue up evictions serially per PDB. We will use the PDB’s selector to distinguish eviction queues. Evictions will run asynchronously and exponentially back off and retry if they fail. This way, evicting pods will only be limited by other evicting pods managed by the same PDB, since each pod can only belong to one PDB. Pods that have a `do-not-evict` label will not be queued up for eviction, as we prevent the node with a `do-not-evict` pod from draining.

### Karpenter Availability and Termination Cases

Karpenter is a node autoscaler, so it does not take responsibility for maintaining the states of the capacity it provisions. In this case, if a user deletes a Provisioner Spec, we will not delete its provisioned capacity. This is in direct contrast to the idea of node groups or pools, which delete managed nodes when the group or pool is deleted.

If a user wants to manually delete a Karpenter provisioned node, this design allows the user to do it safely if Karpenter is installed. Otherwise, the user will need to clean up their resources themselves.

Kubernetes is unable to delete nodes that have finalizers on them. For this reason, we chose to add the Karpenter finalizer only after a delete request is validated. Yet, in the rare case that Karpenter is uninstalled while a node deletion request is processing, to finish terminating the node, the user must either: reinstall Karpenter to resume the termination logic or remove the Karpenter finalizer from the node, allowing the API Server to delete then node.

If a node is unable to become ready for `15 minutes`, we will terminate the node. As we don’t have the ability or responsibility to diagnose the problem, we would worst case terminate a soon-to-be-healthy node. In this case, the orphaned pod(s) would trigger creation of another node.

## Appendix

### FAQ

* How does Cluster Autoscaler handle terminations?
    * By default, Cluster Autoscaler scales down nodes in a cluster if they have been underutilized (resource usage is <50%) for an extended period of time (10 minutes) and their pods can be placed on other existing nodes. [More configuration options](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#what-are-the-parameters-to-ca)
* How can I migrate capacity from one Provisioner to another?
    * You can do this manually by changing node labels to match a Provisioner's labels
    * Or you can use Karpenter to create a new Provisioner, delete the old Provisioner, then delete undesired nodes. The new Provisioner will create capacity for the orphaned pending pods.
* How can I tell when the termination controller is failing to execute some work?
    * A node’s labels will dictate what the termination controller is doing. If a pod is failing to evict because of a misconfiguration, this will be in the Karpenter logs. In addition, if an instance is unable to be terminated, this will also be reflected in the Karpenter logs.

### Potential Scale Down Features

In the future, we may implement the following to account for more scale down situations. These are not in scope, but are included to show how this can work.

* Interruption: If a user is using preemptible instances and the instance is interrupted
* Upgrade: If a node has an old version, and we want to upgrade it
* Defragmentation: If we actively do bin-packing (not just on underutilized nodes) and find a better bin-packing solution
* Garbage collection: If nodes become hanging or too old, and we decide to clean the resources up
* Recycle: If users want to recycle their nodes regularly

### Asynchronous Termination Clarifications

When pods are requested to be evicted, they are put into an Eviction Queue specific to the PDB handling the pods. The controller will call evictions serially that run asynchronously and exponentially back off and retry if they fail.

Finalizers are also handled asynchronously. Adding in a Karpenter finalizer doesn’t prevent or delay other controllers from executing finalizer logic on the same node.
