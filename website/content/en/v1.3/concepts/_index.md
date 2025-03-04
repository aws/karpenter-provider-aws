---
title: "Concepts"
linkTitle: "Concepts"
weight: 20
description: >
  Understand key concepts of Karpenter
---

Users fall under two basic roles: [Kubernetes cluster administrators]({{<ref "#cluster-administrator" >}}) and [application developers]({{<ref "#application-developer" >}}). This document describes Karpenter concepts through the lens of those two types of users.

## Cluster Administrator

As a Kubernetes cluster administrator, you can engage with Karpenter to:

* Install Karpenter
* Configure NodePools to set constraints and other features for managing nodes
* Disrupting nodes

Concepts associated with this role are described below.


### Installing Karpenter

Karpenter is designed to run on a node in your Kubernetes cluster. As part of the installation process, you need credentials from the underlying cloud provider to allow nodes to be started up and added to the cluster as they are needed.

[Getting Started with Karpenter]({{<ref "../getting-started/getting-started-with-karpenter" >}}) describes the process of installing Karpenter. Because requests to add and delete nodes and schedule pods are made through Kubernetes, AWS IAM Roles for Service Accounts (IRSA) are needed by your Kubernetes cluster to make privileged requests to AWS. For example, Karpenter uses AWS IRSA roles to grant the permissions needed to describe EC2 instance types and create EC2 instances.

Once privileges are in place, Karpenter is deployed with a Helm chart.

### Configuring NodePools

Karpenter's job is to add nodes to handle unschedulable pods, schedule pods on those nodes, and remove the nodes when they are not needed. To configure Karpenter, you create [NodePools]({{<ref "nodepools" >}}) that define how Karpenter manages unschedulable pods and configures nodes. You will also define behaviors for your NodePools, capturing details like how Karpenter handles disruption of nodes and setting limits and weights for each NodePool.

Here are some things to know about Karpenter's NodePools:

* **Unschedulable pods**: Karpenter only attempts to schedule pods that have a status condition `Unschedulable=True`, which the kube scheduler sets when it fails to schedule the pod to existing capacity.

* [**Defining Constraints**]({{<ref "nodepools" >}}): Karpenter defines a Custom Resource called a NodePool to specify configuration. Each NodePool manages a distinct set of nodes, but pods can be scheduled to any NodePool that supports its scheduling constraints. A NodePool contains constraints that impact the nodes that can be provisioned and attributes of those nodes. See the [NodePools Documentation]({{<ref "nodepools" >}}) docs for a description of configuration and NodePool examples.

* [**Defining Disruption**]({{<ref "disruption" >}}): A NodePool can also include values to indicate when nodes should be disrupted. This includes configuration around concepts like [Consolidation]({{<ref "disruption#consolidation" >}}), [Drift]({{<ref "disruption#drift" >}}), and [Expiration]({{<ref "disruption#automated-methods" >}}).

* **Well-known labels**: The NodePool can use well-known Kubernetes labels to allow pods to request only certain instance types, architectures, operating systems, or other attributes when creating nodes. See [Well-Known Labels, Annotations and Taints](https://kubernetes.io/docs/reference/labels-annotations-taints/) for details. Keep in mind that only a subset of these labels are supported in Karpenter, as described later.

* **Multiple NodePools**: Multiple NodePools can be configured on the same cluster. For example, you might want to configure different teams on the same cluster to run on completely separate capacity. One team could run on nodes using BottleRocket, while another uses EKSOptimizedAMI.

Although most use cases are addressed with a single NodePool for multiple teams, multiple NodePools are useful to isolate nodes for billing, use different node constraints (such as no GPUs for a team), or use different disruption settings.

### Disrupting nodes

Karpenter deletes nodes when they are no longer needed.

* [**Finalizer**]({{<ref "disruption#manual-methods" >}}): Karpenter places a finalizer bit on each node it creates.
When a request comes in to delete one of those nodes (such as a TTL or a manual `kubectl delete node`), Karpenter will cordon the node, drain all the pods, terminate the EC2 instance, and delete the node object.
Karpenter handles all clean-up work needed to properly delete the node.
* [**Expiration**]({{<ref "disruption" >}}): Karpenter will mark nodes as expired and disrupt them after they have lived a set number of seconds, based on the NodePool's `spec.disruption.expireAfter` value. You can use node expiry to periodically recycle nodes due to security concerns.
* [**Consolidation**]({{<ref "disruption#consolidation" >}}): Karpenter works to actively reduce cluster cost by identifying when:
  * Nodes can be removed because the node is empty
  * Nodes can be removed as their workloads will run on other nodes in the cluster.
  * Nodes can be replaced with cheaper variants due to a change in the workloads.
* [**Drift**]({{<ref "disruption#drift" >}}): Karpenter will mark nodes as drifted and disrupt nodes that have drifted from their desired specification. See [Drift]({{<ref "#drift" >}}) to see which fields are considered.
* [**Interruption**]({{<ref "disruption#interruption" >}}): Karpenter will watch for upcoming interruption events that could affect your nodes (health events, spot interruption, etc.) and will cordon, drain, and terminate the node(s) ahead of the event to reduce workload disruption.

For more details on how Karpenter deletes nodes, see the [Disruption Documentation]({{<ref "disruption" >}}).

### Scheduling

Karpenter launches nodes in response to pods that the Kubernetes scheduler has marked unschedulable. After solving scheduling constraints and launching capacity, Karpenter launches a machine in your chosen cloud provider.

Once Karpenter brings up a node, that node is available for the Kubernetes scheduler to schedule pods on it as well.

#### Constraints

The concept of layered constraints is key to using Karpenter. With no constraints defined in NodePools and none requested from pods being deployed, Karpenter chooses from the entire universe of features available to your cloud provider. Nodes can be created using any instance type and run in any zones.

An application developer can tighten the constraints defined in a NodePool by the cluster administrator by defining additional scheduling constraints in their pod spec. Refer to the description of Karpenter constraints in the Application Developer section below for details.

### Cloud Provider

Karpenter makes requests to provision new nodes to the associated cloud provider. The first supported cloud provider is AWS, although Karpenter is designed to work with other cloud providers. Separating Kubernetes and AWS-specific settings allows Karpenter a clean path to integrating with other cloud providers.

While using Kubernetes well-known labels, the NodePool can set some values that are specific to the cloud provider. For example, to include a certain instance type, you could use the Kubernetes label `node.kubernetes.io/instance-type`, but set its value to an AWS instance type (such as `m5.large` or `m5.2xlarge`).

### Kubernetes Cluster Autoscaler

Like Karpenter, [Kubernetes Cluster Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) is designed to add nodes when requests come in to run pods that cannot be met by current capacity. Cluster autoscaler is part of the Kubernetes project, with implementations by most major Kubernetes cloud providers. By taking a fresh look at provisioning, Karpenter offers the following improvements:

* **Designed to handle the full flexibility of the cloud**: Karpenter has the ability to efficiently address the full range of instance types available through AWS. Cluster autoscaler was not originally built with the flexibility to handle hundreds of instance types, zones, and purchase options.

* **Quick node provisioning**: Karpenter manages each instance directly, without use of additional orchestration mechanisms like node groups. This enables it to retry in milliseconds instead of minutes when capacity is unavailable. It also allows Karpenter to leverage diverse instance types, availability zones, and purchase options without the creation of hundreds of node groups.

## Application Developer

As someone deploying pods that might be evaluated by Karpenter, you should know how to request the properties that your pods need of its compute resources. Karpenter's job is to efficiently assess and choose compute assets based on requests from pod deployments. These can include basic Kubernetes features or features that are specific to the cloud provider (such as AWS).

Layered *constraints* are applied when a pod makes requests for compute resources that cannot be met by current capacity. A pod can specify `nodeAffinity` (to run in a particular zone or instance type) or a `topologySpreadConstraints` spread (to cause a set of pods to be balanced across multiple nodes).
The pod can specify a `nodeSelector` to run only on nodes with a particular label and  `resource.requests` to ensure that the node has enough available memory.

The Kubernetes scheduler tries to match those constraints with available nodes. If the pod is unschedulable, Karpenter creates compute resources that match its needs. When Karpenter tries to provision a node, it analyzes scheduling constraints before choosing the node to create.

As long as the requests are not outside the NodePool's constraints, Karpenter will look to best match the request, comparing the same well-known labels defined by the pod's scheduling constraints. Note that if the constraints are such that a match is not possible, the pod will remain unscheduled.

So, what constraints can you use as an application developer deploying pods that could be managed by Karpenter?

Kubernetes features that Karpenter supports for scheduling pods include nodeAffinity and [nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector).
It also supports [PodDisruptionBudget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/), [topologySpreadConstraints](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/), and [inter-pod affinity and anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity).

From the Kubernetes [Well-Known Labels, Annotations and Taints](https://kubernetes.io/docs/reference/labels-annotations-taints/) page, you can see a full list of Kubernetes labels, annotations and taints that determine scheduling. Those that are implemented in Karpenter include:

* **kubernetes.io/arch**: For example, kubernetes.io/arch=amd64
* **node.kubernetes.io/instance-type**: For example, node.kubernetes.io/instance-type=m3.medium
* **topology.kubernetes.io/zone**: For example, topology.kubernetes.io/zone=us-east-1c

For more on how, as a developer, you can add constraints to your pod deployment, see [Scheduling](./scheduling/) for details.
