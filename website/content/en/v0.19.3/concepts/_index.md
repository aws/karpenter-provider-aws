---
title: "Concepts"
linkTitle: "Concepts"
weight: 35
description: >
  Understand key concepts of Karpenter
---

Users fall under two basic roles: Kubernetes cluster administrators and application developers.
This document describes Karpenter concepts through the lens of those two types of users.

## Cluster administrator

As a Kubernetes cluster administrator, you can engage with Karpenter to:

* Install Karpenter
* Configure provisioners to set constraints and other features for managing nodes
* Deprovision nodes
* Upgrade nodes

Concepts associated with this role are described below.


### Installing Karpenter

Karpenter is designed to run on a node in your Kubernetes cluster.
As part of the installation process, you need credentials from the underlying cloud provider to allow nodes to be started up and added to the cluster as they are needed.

[Getting Started with Karpenter on AWS](../getting-started)
describes the process of installing Karpenter on an AWS cloud provider.
Because requests to add and delete nodes and schedule pods are made through Kubernetes, AWS IAM Roles for Service Accounts (IRSA) are needed by your Kubernetes cluster to make privileged requests to AWS.
For example, Karpenter uses AWS IRSA roles to grant the permissions needed to describe EC2 instance types and create EC2 instances.

Once privileges are in place, Karpenter is deployed with a Helm chart.

### Configuring provisioners

Karpenter's job is to add nodes to handle unschedulable pods, schedule pods on those nodes, and remove the nodes when they are not needed.
To configure Karpenter, you create *provisioners* that define how Karpenter manages unschedulable pods and expires nodes.
Here are some things to know about the Karpenter provisioner:

* **Unschedulable pods**: Karpenter only attempts to schedule pods that have a status condition `Unschedulable=True`, which the kube scheduler sets when it fails to schedule the pod to existing capacity.

* **Provisioner CR**: Karpenter defines a Custom Resource called a Provisioner to specify provisioning configuration.
Each provisioner manages a distinct set of nodes, but pods can be scheduled to any provisioner that supports its scheduling constraints.
A provisioner contains constraints that impact the nodes that can be provisioned and attributes of those nodes (such timers for removing nodes).
See [Provisioner API](../provisioner) for a description of settings and the [Provisioning](../tasks/provisioning) task for provisioner examples.

* **Well-known labels**: The provisioner can use well-known Kubernetes labels to allow pods to request only certain instance types, architectures, operating systems, or other attributes when creating nodes.
See [Well-Known Labels, Annotations and Taints](https://kubernetes.io/docs/reference/labels-annotations-taints/) for details.
Keep in mind that only a subset of these labels are supported in Karpenter, as described later.

* **Deprovisioning nodes**: A provisioner can also include time-to-live values to indicate when nodes should be deprovisioned after a set amount of time from when they were created or after they becomes empty of deployed pods.

* **Multiple provisioners**: Multiple provisioners can be configured on the same cluster.
For example, you might want to configure different teams on the same cluster to run on completely separate capacity.
One team could run on nodes using BottleRocket, while another uses EKSOptimizedAMI.

Although most use cases are addressed with a single provisioner for multiple teams, multiple provisioners are useful to isolate nodes for billing, use different node constraints (such as no GPUs for a team), or use different deprovisioning settings.

### Deprovisioning nodes

Karpenter deletes nodes when they are no longer needed.

* **Finalizer**: Karpenter places a finalizer bit on each node it creates.
When a request comes in to delete one of those nodes (such as a TTL or a manual `kubectl delete node`), Karpenter will cordon the node, drain all the pods, terminate the EC2 instance, and delete the node object.
Karpenter handles all clean-up work needed to properly delete the node.
* **Node Expiry**: If a node expiry time-to-live value (`ttlSecondsUntilExpired`) is reached, that node is drained of pods and deleted (even if it is still running workloads).
* **Empty nodes**: When the last workload pod running on a Karpenter-managed node is gone, the node is annotated with an emptiness timestamp.
Once that "node empty" time-to-live (`ttlSecondsAfterEmpty`) is reached, finalization is triggered.
* **Consolidation**: If enabled, Karpenter will work to actively reduce cluster cost by identifying when nodes can be removed as their workloads will run on other nodes in the cluster and when nodes can be replaced with cheaper variants due to a change in the workloads.
* **Interruption**: If enabled, Karpenter will watch for upcoming involuntary interruption events that could affect your nodes (health events, spot interruption, etc.) and will cordon, drain, and terminate the node(s) ahead of the event to reduce workload disruption.

For more details on how Karpenter deletes nodes, see [Deprovisioning nodes](../tasks/deprovisioning) for details.

### Upgrading nodes

A straight-forward way to upgrade nodes is to set `ttlSecondsUntilExpired`.
Nodes will be terminated after a set period of time and will be replaced with newer nodes using the latest [EKS Optimized AMI](https://docs.aws.amazon.com/eks/latest/userguide/eks-optimized-amis.html) or the AMI specified in the `$LATEST` version of your launch template.

Understanding the following concepts will help you in carrying out the tasks just described.

### Constraints

The concept of layered constraints is key to using Karpenter.
With no constraints defined in provisioners and none requested from pods being deployed, Karpenter chooses from the entire universe of features available to your cloud provider.
Nodes can be created using any instance type and run in any zones.

An application developer can tighten the constraints defined in a provisioner by the cluster administrator by defining additional scheduling constraints in their pod spec.
Refer to the description of Karpenter constraints in the Application Developer section below for details.

### Scheduling

Karpenter schedules pods that the Kubernetes scheduler has marked unschedulable.
After solving scheduling constraints and launching capacity, Karpenter creates the Node object and waits for kube-scheduler to bind the pod.
This stateless approach helps to avoid race conditions and improves performance.
If something is wrong with the launched node, Kubernetes will automatically migrate the pods to a new node.

Once Karpenter brings up a node, that node is available for the Kubernetes scheduler to schedule pods on it as well.
This is useful if there is additional room in the node due to imperfect packing shape or because workloads finish over time.

### Cloud provider
Karpenter makes requests to provision new nodes to the associated cloud provider.
The first supported cloud provider is AWS, although Karpenter is designed to work with other cloud providers.
Separating Kubernetes and AWS-specific settings allows Karpenter a clean path to integrating with other cloud providers.

While using Kubernetes well-known labels, the provisioner can set some values that are specific to the cloud provider.
So, for example, to include a certain instance type, you could use the Kubernetes label `node.kubernetes.io/instance-type`, but set its value to an AWS instance type (such as `m5.large` or `m5.2xlarge`).

### Consolidation

If consolidation is enabled for a provisioner, Karpenter attempts to reduce the overall cost of the nodes launched by that provisioner if workloads have changed in two ways:
- Node Deletion
- Node Replacement

To perform these actions, Karpenter simulates all pods being evicted from a candidate node and then looks at the results of the scheduling simulation to determine if those pods can run on a combination of existing nodes in the cluster and a new cheaper node.  This operation takes into consideration all scheduling constraints placed on your workloads and provisioners (e.g. taints, tolerations, node selectors, inter-pod affinity, etc).  

If as a result of the scheduling simulation all pods can run on existing nodes, the candidate node is simply deleted.  If all pods can run on a combination of existing nodes and a cheaper node, we launch the cheaper node and delete the candidate node which causes the pods to be evicted and re-created by their controllers in order to be rescheduled.

For Node Replacement to work well, your provisioner must allow selecting from a variety of instance types with varying amounts of allocatable resources.  Consolidation will only consider launching nodes using instance types which are allowed by your provisioner.

### Interruption

If interruption-handling is enabled for the controller, Karpenter will watch for upcoming involuntary interruption events that would cause disruption to your workloads. These interruption events include:

* Spot Interruption Warnings
* Scheduled Change Health Events (Maintenance Events)
* Instance Terminating Events
* Instance Stopping Events

When Karpenter detects one of these events will occur to your nodes, it automatically cordons, drains, and terminates the node(s) ahead of the interruption event to give the maximum amount of time for workload cleanup prior to compute disruption. This enables scenarios where the `terminationGracePeriod` for your workloads may be long or cleanup for your workloads is critical, and you want enough time to be able to gracefully clean-up your pods.

{{% alert title="Note" color="warning" %}}
Karpenter publishes Kubernetes events to the node for all events listed above in addition to __Spot Rebalance Recommendations__. Karpenter does not currently support cordon, drain, and terminate logic for Spot Rebalance Recommendations.
{{% /alert %}}

### Kubernetes cluster autoscaler
Like Karpenter, [Kubernetes Cluster Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) is
designed to add nodes when requests come in to run pods that cannot be met by current capacity.
Cluster autoscaler is part of the Kubernetes project, with implementations by most major Kubernetes cloud providers.
By taking a fresh look at provisioning, Karpenter offers the following improvements:

* **Designed to handle the full flexibility of the cloud**:
Karpenter has the ability to efficiently address the full range of instance types available through AWS.
Cluster autoscaler was not originally built with the flexibility to handle hundreds of instance types, zones, and purchase options.

* **Group-less node provisioning**: Karpenter manages each instance directly, without use of additional orchestration mechanisms like node groups.
This enables it to retry in milliseconds instead of minutes when capacity is unavailable.
It also allows Karpenter to leverage diverse instance types, availability zones, and purchase options without the creation of hundreds of node groups.

## Application developer

As someone deploying pods that might be evaluated by Karpenter, you should know how to request the properties that your pods need of its compute resources.
Karpenter's job is to efficiently assess and choose compute assets based on requests from pod deployments.
These can include basic Kubernetes features or features that are specific to the cloud provider (such as AWS).

Layered *constraints* are applied when a pod makes requests for compute resources that cannot be met by current capacity.
A pod can specify `nodeAffinity` (to run in a particular zone or instance type) or a `topologySpreadConstraints` spread (to cause a set of pods to be balanced across multiple nodes).
The pod can specify a `nodeSelector` to run only on nodes with a particular label and  `resource.requests` to ensure that the node has enough available memory.

The Kubernetes scheduler tries to match those constraints with available nodes.
If the pod is unschedulable, Karpenter creates compute resources that match its needs.
When Karpenter tries to provision a node, it analyzes scheduling constraints before choosing the node to create.

As long as the requests are not outside of the provisioner's constraints,
Karpenter will look to best match the request, comparing the same well-known labels defined by the pod's scheduling constraints.
Note that if the constraints are such that a match is not possible, the pod will remain unscheduled.

So, what constraints can you use as an application developer deploying pods that could be managed by Karpenter?

Kubernetes features that Karpenter supports for scheduling pods include nodeAffinity and [nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector).
It also supports [PodDisruptionBudget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/), [topologySpreadConstraints](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/), and [inter-pod affinity and anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity).

From the Kubernetes [Well-Known Labels, Annotations and Taints](https://kubernetes.io/docs/reference/labels-annotations-taints/) page,
you can see a full list of Kubernetes labels, annotations and taints that determine scheduling.
Those that are implemented in Karpenter include:

* **kubernetes.io/arch**: For example, kubernetes.io/arch=amd64
* **node.kubernetes.io/instance-type**: For example, node.kubernetes.io/instance-type=m3.medium
* **topology.kubernetes.io/zone**: For example, topology.kubernetes.io/zone=us-east-1c

For more on how, as a developer, you can add constraints to your pod deployment, see [Scheduling](../tasks/scheduling/) for details.
