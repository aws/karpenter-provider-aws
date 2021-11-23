
---
title: "Documentation"
linkTitle: "Docs"
weight: 20
menu:
  main:
    weight: 20
    pre: <i class='fas fa-book'></i>
---
Karpenter is an open-source node provisioning project built for Kubernetes.
Adding Karpenter to a Kubernetes cluster can dramatically improve the efficiency and cost of running workloads on that cluster.
Karpenter is tightly integrated with Kubernetes features to make sure that the right types and amounts of compute resources are available to pods as they are needed.
Karpenter works by:

* **Watching** for pods that the Kubernetes scheduler has marked as unschedulable
* **Evaluating** scheduling constraints (resource requests, nodeselectors, affinities, tolerations, and topology spread constraints) requested by the pods
* **Provisioning** nodes that meet the requirements of the pods
* **Scheduling** the pods to run on the new nodes
* **Removing** the nodes when the nodes are no longer needed

As a cluster operator, you can configure an unconstrained Karpenter provisioner when it is first installed and not change it again.
Other times, you might continue to tweak the provisioner or create multiple provisioners for a cluster used by different teams.
On-going cluster operator tasks include upgrading and decomissioning nodes.

As an application developer, you can make specific requests for capacity and features you want from the nodes running your pods.
Karpenter is designed to quickly create the best possible nodes to meet those needs and schedule the pods to run on them.

Learn more about Karpenter and how to get started below.
