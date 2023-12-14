# Karpenter Node Upgrades - AMI Upgrades

## Overview

Karpenter users are requesting for Node Upgrades, generally asking for [more complex node control](https://github.com/aws/karpenter-provider-aws/issues/1738) (91 up-votes and counting). While early versions of Karpenter meant for Provisioners to dictate node provisioning requirements and simple termination methods, Provisioners are becoming more semantically aligned with Node Groups. This is seen with [Provisioners owning nodes](https://github.com/aws/karpenter/pull/1934) and [Consolidation](https://github.com/aws/karpenter/pull/2123), where Karpenter controls the cluster in more ways than provisioning nodes for pending pods. To further align the Provisioner to represent a desired state of both new and existing nodes, Karpenter will implement [Node AMI Upgrades](https://github.com/aws/karpenter/issues/1716) by detecting and remediating drifted nodes.

For the initial implementation, Karpenter's drift will reconcile when a node's AMI drifts from provisioning requirements. This mechanism will be scalable to other methods of drift reconciliation in the future. Addtionally, Karpenter can implement Imperative Upgrades or Maintenance Windows in the future.

This document will answer the following questions:

* How will Karpenter upgrade nodes?
* Which nodes should Karpenter upgrade?
* What can interfere with upgrades?

## User Stories

* Karpenter will be able to automatically upgrade node AMIs to match their provisioning requirements.
* Karpenter will be able to enable other providers to implement their own drift mechanisms.

### Background - What is node drift?

A drifted node is one whose spec and metadata does not match the spec of its Provisioner and ProviderRef. A node can drift when a user changes their Provisioner or ProviderRef. For example, if a user changes their architecture requirements to only `amd64` from `arm64`, any existing `arm64` nodes will drift. In another example, if a user changes the tags in the AWSNodeTemplate, their nodes will drift due to mis-matching tags. These are both actions done within the cluster.

Yet, a node can drift without modifying the Provisioner or ProviderRef. Underlying infrastructure in the Provider can be changed outside of the cluster. For example, using defaults in the `AWSNodeTemplate` `AMISelector` allows the AMI to change when a new AL2 EKS Optimized AMI is released, creating drifted nodes. Additionally, the same can happen if a user changes the tags on their AMIs and uses arbitrary tags in the `AWSNodeTemplate`.

## How will Karpenter upgrade nodes?

When upgrading a node, Karpenter will minimize the downtime of the applications on the node by initating provisioning logic for a replacement node before terminating drifted nodes. Once Karpenter has begun provisioning the replacement node, Karpenter will cordon and drain the old node, terminating it when it’s fully drained, then finishing the upgrade.

Karpenter's currently does not have a dedicated controller for drift. With no rate-limiting, this means using the termination controller to scale down and the provisioning controller to scale back up. Since the termination controller asynchronously scales down the cluster as quick as possible, rate-limiting only happens at the pod level with user-controls. In creating a dedicated controller to orchestrate the logic, Karpenter can begin to rate-limit at the node level. Rate-limiting can be done serially and in parallel.

### Option 1 - Upgrade Serially

Karpenter will only upgrade one node at a time. Each node's upgrade will be complete when its replacement is ready and the old node is terminated. This naturally rate-limits node upgrades to node initialization and pod-rescheduling time and means that users will only have to worry about one node disruption at a time with upgrades.

With serial upgrades, new nodes that cannot become ready or old nodes that cannot fully drain can stop the upgrade process altogether. While Karpenter will not upgrade nodes with blocking PDBs or `do-not-evict` pods, provisioning a new node can fail for many reasons. For one, if the new AMI is incompatible with the cluster's applications, Karpenter should not upgrade more nodes as more upgrades to that AMI would likely fail. On the other hand, sometimes a newly provisioned node can succeed in other cases like with an [Outdated CNI](https://karpenter.sh/v0.16.3/troubleshooting/#nodes-stuck-in-pending-and-not-running-the-kubelet-due-to-outdated-cni) - where using a newer instance type could fail where an older instance type would succeed.

In order to not permanently block serial upgrades, upgrades will have timeout. Relying on Kubernetes native concepts, Karpenter will compute a drain timeout that respects the `GracefulTerminationPeriodSeconds` on the pods on the nodes by summing all remaining `GracefulTerminationPeriodSeconds` fields. This accounts for the worst case where Karpenter can only evict one pod at a time, taking as long as possible. If Karpenter fails to create a replacement node, Karpenter can terminate the unitialized upgraded node, where rolling back is already complete.

### Option 2 - Upgrade in Parallel

For some use cases, upgrading serially may be too slow. To combat this, Karpenter can upgrade in parallel, allowing more than one node to upgrade at a time. In contrast to upgrading serially, upgrading in parallel requires opinionated hyper-parameters or allowing an API for users to configure them.

#### Option 2a - In parallel with controls

Starting with an API, Karpenter could implement a Node Disruption Budget. Similar to PDBs, NDBs rate-limit the termination process of nodes, gating termination requests from the controller or users. Similar to a restrictive PDB, a restrictive NDB can be used to stop termination altogether. Upgrading serially and using a NDB with maxUnavailable to 1 are equivalent.

```
apiVersion: karpenter.sh/v1alpha1
kind: NodeDisruptionBudget
metadata:
  name: default
spec:
  # Merged labelSelectors for multiple provisioners
  labelSelectors:
    - karpenter.sh/provisioner-name: default
    - karpenter.sh/provisioner-name: app-name
  # Following two are mutually exclusive:
  minAvailable: 10% # intOrString
  maxUnavailable: 30 # intOrString
```

Node Disruption Budgets are conceptually extensible from PDBs. Users who already use PDBs will be able to easily reason with the concept. While conceptually similar, pods tend to have more homogenous resource requirements and usually work with ReplicationControllers to ensure a number of replicas. Instead, nodes are stand-alone objects and can have very heterogenous resource requirements. In fact, NDBs can act very differently depending on the distribution of node sizes. Check [NDB Examples](./node-upgrades.md#node-disruption-budget-examples) for more.

#### Option 2b - In parallel with hard-coded hyper parameters and surface controls later

While these defaults may make sense for some use-cases, it ultimately is tough to get right for others. This section could be a first step before implementing Node Disruption Budgets.

Karpenter can upgrade nodes in parallel with a series of hard-coded defaults. Many of the proposed defaults are similar to [Cluster Autoscaler's](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#what-are-the-parameters-to-ca). Karpenter also uses [defaulted tunables in Consolidation](./consolidation.md#internal-tunables). For parllel upgrades, Karpenter proposes the following:

* Polling Period - This is how often a node will scan the cluster for nodes that need upgrading. Karpenter can simply add the `AMI-ID` at provisioning time, where Karpenter falls back to an EC2 API call. The drift controller will poll drifted nodes every second, as this becomes very inexpensive.
* Cooldown Period - How long Karpenter waits to upgrade instances after upgrading a batch. This may not be necessary as Karpenter will pre-spin nodes.
    * This is similar to `scale-down-delay-after-add`, `scale-down-delay-after-delete`, and `scale-down-unneeded-time` in Cluster Autoscaler.
* Minimum Node Lifetime - The minimum age a node must be before it is considered for upgrading. If a node is created and then a new AMI is released, it may be favorable to wait a bit before upgrading it as to not introduce unwanted churn. This can be mitigated with the following section, where upgrade ordering is discussed.
* Max Upgrade Batch - How many nodes will be upgraded at a time. This will default to 5%. For a 1000 node cluster, 50 nodes could upgrade at the same time. For a cluster with 20 or less nodes, this will be 1, equivalent to upgrading serially.

### Choice Recommendation

*Upgrade serially*. If Karpenter upgrades in parallel and uses an API such as NDB, users will need to use NDBs to safely onboard to automatic updates. Newer users may find this too complicated and disable automatic upgrades altogether. If Karpenter hard-codes defaults into the process, some cases may end up working poorly for existing users.

Upgrading serially requires no additional API or hard-coded hyper-parameters. It creates simple requirements on upgrades, and solves some of the existing problems of jitter (https://github.com/aws/karpenter-provider-aws/issues/1884), more granular expiration conditions (https://github.com/aws/karpenter/issues/903), and rate-limiting (https://github.com/aws/karpenter/issues/1018). Upgrading serially is additionally extendable to various forms of the parallel upgrades as called out in each of the sections. Should users want quicker upgrades in the future, Karpenter has a path forward to implementing parallel upgrades.

## What can interfere with upgrades?

### Provisioning Constraints

Due to the fact that Karpenter will pre-spin nodes, it’s possible that upgrades will fail due to strict or incorrect provisioning constraints. In these cases, Karpenter will log the failures and try again later.

#### Provisioner Limits or Other Limits

Users can specify resource limits on their Provisioner as a resource ceiling. If a replacement node cannot be created due to tight provisioner limits, the upgrade will fail. As this is set by the user, Karpenter will not modify the limits. Karpenter's best practices here are to leave some room on the Provisioner limits for nodes to upgrade.

Additionally, a user could fail to provision a node due to IP exhaustion or service quota limits. Since Karpenter will be upgrading serially, a user can be throttled if their usage is very close to their limits. Since Karpenter cannot change any limits outside of the cluster, Karpenter's best practice is to use resource limits that represent these "out-of-cluster" limits as best as possible.

#### Unavailable Offerings (ICE)

Karpenter caches offerings that maintain key information about recent API calls to EC2 about available instances. Offerings contain information about price, capacity type, and availability zone. To maximize the chance that all pods are able to reschedule, Karpenter will execute the provisioning logic to match the scheduling constraints and the resource requirements of the pods on the drifted nodes and all existing pending pods. If a user’s provisioner has restrictive instance requirements, Karpenter's scheduler may be unable to provision a node due to underlying instance availability. If Karpenter is unable to provision capacity due to an Insufficient Capacity Exception (ICE), Karpenter will try this node again later.

#### Invalid Constraints

In the event that a user changes the `AMISelector` or underlying AMI and Karpenter cannot discover an AMI, Karpenter will not attempt to upgrade the node, as the upgrade will fail.

### Concurrent Termination

Karpenter’s termination is triggered by either the user or the controllers, specifically Consolidation, Emptiness, and Expiration. The termination controller executes the cordon and drain logic for each node that is requested to terminate, signaled by the finalizer that is present on all nodes.

In the case where Consolidation is disabled and Emptiness is enabled (they are mutually exclusive), emptiness does not create a replacement node. Expiration is also taken into account as a heuristic by Consolidation.

#### Karpenter Controllers - Deprovisioning

Consolidation currently implements node deletion and node replacement. It deletes a node if it finds that all pods on the node can fit on other existing nodes. It replaces a node if the pods on the node can be replaced with a cheaper node and the rest of the cluster. Boiled down, consolidation takes actions on the cluster prioritizing nodes that are closest to expiring and easiest to fully drain. More details [here](./consolidation.md#selecting-nodes-for-consolidation).

If consolidation and upgrades are executed concurrently, pods that are rescheduling due to consolidation could schedule onto the drifted node's replacement. Pods being drained as part of the upgrade could fail to schedule, requiring another to be provisioned, or failing rescheduling altogether. More detailed examples [here](./node-upgrades#scheduling-mishaps-with-consolidation). To handle this, Karpenter will merge consolidation, node drift, node expiration, and node emptiness into a `Deprovisioning` controller. Merging this logic allows Karpenter to be aware of all controller scale-down logic and orchestrate it together to minimize inefficient logic. Check [deprovisioning.md](./deprovisioning.md) for more.

#### The User

Taking advantage of finalizers present on all nodes, a user can decide to terminate their nodes at any time with `kubectl delete node`. If there are pods scheduled to these nodes, Karpenter will provision a replacement node if they cannot schedule elsewhere. If a user wants to upgrade a node, they can simply delete it. If upgrading serially, this means multiple nodes could be scaling down at once. If upgrading in parallel, this would plug into the existing controls. In either case, Karpenter will rely on the user to manually intervene with any failed scheduling actions, as they have already manually intervened.

#### Kubernetes Concepts

PDBs, the `do-not-evict` pod annotation, and other finalizers are ways that users in Kubernetes can slow down upgrades. With restrictive enough PDBs, upgrades may fail by exceeding their timeout period, where the node could take to long to fully drain. In addition, if the user uses finalizers and does not clean them up, the old node may never terminate. For this reason, the node could be fully upgraded but keep around old capacity. Since this may or may not block upgrades, Karpenter will incorporate this logic into the following section on prioritizations.

## Which nodes should Karpenter upgrade?

When provisioning a node, Karpenter generates a launch template and fills it with an AMI selected by the `AWSNodeTemplate` `AMISelector`. As hinted above, drifted nodes with AMIs not matching the `AWSNodeTemplate` `AMISelector` will be upgraded.

Karpenter will not upgrade drifted nodes that have the `do-not-evict: true` annotation, [misconfigured PDBs, or blocking PDBs](https://kubernetes.io/docs/concepts/scheduling-eviction/api-eviction/#how-api-initiated-eviction-works). Karpenter's deprovisioning controller will respect this as well.

On top of this, Karpenter’s current AMI selection picks a random AMI out of a list of allowed AMIs. Karpenter will use latest AMI out of the discovered list of AMIs. If users need a method of not having the latest EKS Optimized AMI, Karpenter can implement native AMI Versioning controls in the future.

## Appendix

### Node Disruption Budget Examples

NDBs can work in very different ways depending on the sizes of the nodes in the cluster.

#### Homogenous Nodes

Take a cluster with 10 nodes, all m5.xlarge, using one Provisioner default that only allows m5.xlarge instances to be provisioned. Let’s say each m5.xlarge has 3 allocatable vCPU and 14 allocatable GiB. We use the following NDB.

apiVersion: karpenter.sh/v1alpha1
kind: NodeDisruptionBudget
metadata:
  name: default
spec:
  labelSelectors:
    - karpenter.sh/provisioner-name: default
  minAvailable: 70% # all but 3 nodes will be upgrading at a time.

If we change the amiSelector to a new AMI and upgrade in parallel, we would first upgrade 3 nodes at a time, retaining an uptime of 70%. No matter which nodes we pick, the same amount of vCPU and GiB used in applications will be disrupted.

#### Heterogenous Nodes

Take a cluster with 4 nodes, with sizes m5.large, m5.2xlarge, m5.8xlarge, and m5.16xlarge.  Let’s say each instance has 1vCPU and 1GiB of overhead. Let’s also assume that all nodes are fully utilized by one deployment, meaning all pods on the instances share the same resource requirements. Use the following NDB.

apiVersion: karpenter.sh/v1alpha1
kind: NodeDisruptionBudget
metadata:
  name: default
spec:
  labelSelectors:
    - karpenter.sh/provisioner-name: default
  maxUnavailable: 2 # only 2 nodes will be upgrading at a time.

Knowing that the allocatable resources of each of these vary by about a factor of 4 (m5.2xlarge is 1/4 the size of m5.8xlarge), upgrading the nodes by changing the amiSelector can vary how much disruption is created on the cluster. If we upgrade the 2 largest nodes, since these are the largest, there will be a proportionally higher amount of pods moving than any other two nodes. If we upgrade the 2 smallest nodes, a proportionally much smaller amount of pods will be disrupted.

Doing the math, where x is the CPU requested by one of the pods, x + 4x + 16x + 64x, we can see that upgrading the two largest nodes will be 80x where the smallest two nodes will be 5x.  This means that depending on the order in this case, a user with this NDB could have 80/85 = 94% or even 5/80=6% of their cluster upgrading at one point, which is wildly erratic behavior.

### Upgrade Interactions

#### Scheduling Mishaps with Consolidation

Consolidation is terminating node A of instance type m5.2xlarge, and replacing the node with a smaller A' of instance type m5.xlarge. Upgrade is upgrading node B to B’. Pods on A have pod anti-affinity with pods on B.

Say that B' becomes ready before A' and pods from A schedule before any pods from B do. Due to the anti-affinity, the B pods cannot schedule, and the Provisioning controller must create a C node where the size is determined by the rate at which nodes are drained from the B node and the batching window used for provisioning. This could result in the following cases:

* If the pods on B cannot fit on A'
    * If limits allow a node C, we end up with three nodes, A', B', and C.
        * This could be more costly than the original, which is not what we want.
        * Introduces unnecessary churn to the pods upgrading, vastly slowing down the upgrade process.
    * If limits do not allow a node C, Karpenter will be unable to schedule capacity for the upgraded pods, resulting in manual intervention and an undefined amount of downtime.
* If the pods on B can fit on A', this means that consolidation is successful, but applications that needed an upgraded AMI may still be on the outdated AMI. This case has little harm on the cluster, but relies on a smaller subset of conditions on an already smaller subset of conditions created in this situation.
