# Cluster Consolidation

## Consolidation Mechanisms

Karpenter will implement two consolidation mechanisms for the first release, “node deletion” and “node replacement”.

### Node Deletion

The simplest form of consolidation is to look at the pods on the node and if they all can be evicted, determine if all of the pods can run on other nodes already in the cluster.  This involves examining the pods on the nodes for conditions that can prevent eviction and then performing a simulated scheduling run against the existing nodes in the cluster.  If all of the pods can schedule against existing nodes, the node can be deleted.

The disruption cost here is calculated from the number of pods on the node to be deleted and the cost savings is the price of the deleted node.

### Node Replacement

Node Replacement is slightly more complex.  We need to perform the same calculations as in node deletion, however we also allow the scheduler to  consider creating an additional node in addition to considering the existing cluster capacity.  If some combination of existing capacity plus a single new cheaper node creation can support all of the pods, then we can launch the new cheaper node and *when it is ready* delete the existing more expensive node.

The complication to node replacement is that we need some concept of what the node we are considering replacing costs.  This lead to the recent introduction of accurate pricing information to Karpenter.

To support Node Replacement well, provisioners will need to allow a wide variety of instance sizes.  For example, if a provisioner only allows instance types that are "shaped" similarly with respect to resources, node replacement is unlikely to be able to take advantage of reduced resource usage by workloads via replacing a node with a cheaper variant.

The disruption cost for node replacement is calculated from the number of pods on the node to be deleted and the cost savings is the price of the deleted node minus the price of the replacement node.

## Selecting Nodes for Consolidation

We intend to limit consolidation to making minimal changes to the cluster as we work towards reducing cluster cost. Making large changes (e.g. launch 5 replacement nodes and drain 6 existing nodes) is both expensive to compute and more likely to disrupt workloads. During consolidation there will be points where multiple nodes can be consolidated and we will need to choose a node. In testing, if you have roughly equivalent pods and node sizes, large numbers of nodes are all consolidatable at once. We will score consolidation options by computing a metric based on the number of workloads disrupted (excluding daemonsets, terminated pods, etc.), taking into consideration:

* Number of pods to be evicted
  * e.g. Two identical nodes can be deleted, one has 5 pods and the other has 100. We should delete the node with 5 pods to minimize the number of pods that will need to be rescheduled.
* Pod deletion costs from the pod annotation controller.kubernetes.io/pod-deletion-cost
  * e.g. Two identical nodes can be deleted, both with 5 pods.  One has pods with a larger sum of pod-deletion-cost specified, we should delete the node with the lower pod deletion costs.
* Pod priorities
  * e.g. Two identical nodes can be deleted, both with 5 pods.  One has pods with a higher priority specified, we should the delete the node with the pods with lower priority.
* Node Age - The calculated disruption cost from above is weighted by the node lifetime remaining, 1.0 at node creation linearly to 0.0 at expiration.  If the ttlSecondsUntilExpired provisioner setting is not being used, this has no effect.
  * e.g. Two identical nodes can be deleted, both with 5 pods.  One has 29 days of lifetime remaining and the other has 5 minutes.  We should delete the node with 5 minutes of lifetime left.

The considerations above are intended to avoid making bad decisions in circumstances when everything else is equal.  By blending them together into a single disruption cost, we can choose how we will make decisions that optimize for a) not disrupting workloads that customers have already indicated are important via standard Kubernetes mechanisms and b) disrupting workloads that will be disrupted soon regardless of our decision.  The concept is similar to the ideas behind Boids algorithm (https://en.wikipedia.org/wiki/Boids).

Essentially we choose to delete nodes when that node's pods will run on other nodes in your cluster. If that isn't possible, we will replace a node with a cheaper node if the node's pods can run on a combination of the other nodes in your cluster and the cheaper replacement node. If there are multiple nodes that could be potentially deleted or replaced, we choose to consolidate the node that overall disrupts your workloads the least by preferring to terminate:

* nodes with fewer pods
* nodes that will expire soon
* nodes with lower priority pods

### Pods that Prevent Consolidation

We will not be able to consolidate nodes with:

* Pods that have no controller owner (apart from those owned by the node)
* Pods that have a PDB that would prevent their eviction (e.g. a PDB with a status of  disruptionsAllowed = 0)
  * This only prevents consolidation of a node with pods that are currently at or exceeding the PDB threshold. If disruptionsAllowed is at least 1, we can assume we can delete the pod and a replacement will be rescheduling allowing us to drain the node a the pod replacement rate.
* Pods with the karpenter.sh/do-not-evict annotation
* Pods that can’t be moved due to scheduling constraints (e.g. affinity/anti-affinity, topology spread)


## Other Considerations

### Configuration

All configuration is currently hidden.  I’ve picked some values to test, but they will likely need tweaking based on customer feedback.

### Internal Tunables

* Order of Node Evaluation - We currently evaluate nodes in ascending order of disruption cost.  We could also evaluate them in descending order of savings if the node were to be removed/replaced.  I chose disruption cost as it biases for availability of customer workloads while working together with both native Kubernetes mechanisms (e.g. pod-deletion-cost) and Karpenter’s node expiration to allow customer input into which nodes we prefer to consolidate while making better decisions given our knowledge of node lifetime. Minimizing disruption cost also has some correlation to the likelihood of a given node being consolidatable.
* Polling Period - This is how often we examine the cluster for consolidation.  Currently it’s set to a few seconds with an optimization built in that if we've examined the cluster and found no actions that can be performed, we will pause cluster examination for a period of time unless the cluster state has changed in some way (e.g. pods or nodes added/removed).
* Stabilization Window - This is the time period after a node has been deleted before we consider consolidating again.  This is needed as controllers that replace evicted pods take a small amount of time to act.  As we are only looking at node capacities with respect to the pods bound to them, we need to wait for those pods to be recreated and bind. This value is currently dynamic and is set to 5 minutes if there are pending pods or un-ready standard controllers and zero seconds if there are no pending pods and all standard controllers are ready.
* Pod Disruption Cost - We calculate a disruption cost per pod as described above.  This currently weighs a few factors including pod priorities and the pod-deletion-cost annotation to allow customer input using native Kubernetes concepts that influence our consolidation decisions.
* Minimum Node Lifetime - We use a minimum node lifetime of five minutes. If the node has been initialized for less than this period of time, we don’t consider it for consolidation. This time period can’t be too small as it sometimes takes a few minutes for dynamic PVCs to bind.  If it is too large, then a rapid scale-up/scale-down will be delayed as empty nodes sit idle until they reach the minimum node lifetime.

### Emptiness TTL

The existing ttlSecondsAfterEmpty settings is duplicative with consolidation. To avoid racing between consolidation and the existing empty node removal, we need one single mechanism that is responsible for both general consolidation and eliminating empty nodes

To do this, we will treat ttlSecondsAfterEmpty  and consolidation as mutually exclusive and check this via our validation webhook.  If ttlSecondsAfterEmpty is set and consolidation is turned off, it continues to work as it does now.  If consolidation is turned on, then ttlSecondsAfterEmpty must not be set and consolidation is responsible for empty nodes.

This doesn’t break existing customers that use ttlSecondsAfterEmpty but don’t turn on consolidation. It also allows for customers that are concerned about workload disruption to continue to only have nodes removed if they are entirely unused for a period of time.