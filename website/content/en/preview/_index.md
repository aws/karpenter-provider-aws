
---
title: "Documentation"
linkTitle: "Docs"
weight: 20
cascade:
  type: docs
  tags:
    - preview
---
Karpenter is an open-source node provisioning project built for Kubernetes.
Adding Karpenter to a Kubernetes cluster can dramatically improve the efficiency and cost of running workloads on that cluster.
Karpenter works by:

* **Watching** for pods that the Kubernetes scheduler has marked as unschedulable
* **Evaluating** scheduling constraints (resource requests, nodeselectors, affinities, tolerations, and topology spread constraints) requested by the pods
* **Provisioning** nodes that meet the requirements of the pods
* **Scheduling** the pods to run on the new nodes
* **Removing** the nodes when the nodes are no longer needed

As someone using Karpenter, once your Kubernetes cluster and the Karpenter controller are up and running (see [Getting Started]({{<ref "./getting-started" >}}), you can:

* **Set up provisioners**: By applying a provisioner to Karpenter, you can configure constraints on node provisioning and set timeout values for node expiry.

* **Deploy workloads**: When deploying workloads, you can request that scheduling constraints be met to direct which nodes Karpenter provisions for those workloads.

Karpenter supports Kubernetes scheduling constraints, including Kubernetes well-known labels, annotations, and taints.
It also supports constraints specific to the cloud provider (such as AWS).
The following Kubernetes and cloud provider constraints are supported by Karpenter at the provisioner, node, and pod levels:

* Provisioner Level
  - [Zones](https://kubernetes.io/docs/reference/labels-annotations-taints/#topologykubernetesiozone)
  - [Capacity Types]({{<ref "./provisioner" >}}),
  - [Instance Types](https://kubernetes.io/docs/reference/labels-annotations-taints/#nodekubernetesioinstance-type)
  - [Taints](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)
* Node Level
  - [Taints](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)
  - [Resource Capacity](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#resource-units-in-kubernetes)
  - [Instance Type Limitations](https://kubernetes.io/docs/reference/labels-annotations-taints/#nodekubernetesioinstance-type)
* Pod Level
  - [Resource Requests](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#example-1)
  - [Tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)
  - [Node Affinity and Node Selectors](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/)
  - [Topology Spread](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/)
  - [Pod Affinity and Anti-Affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity)

Learn more about Karpenter and how to get started below.
