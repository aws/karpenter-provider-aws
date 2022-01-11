---
title: "Deprovisioning nodes"
linkTitle: "Deprovisioning nodes"
weight: 20
---


## Deletion Workflow

### Finalizer

Karpenter adds a finalizer to provisioned nodes. [Review how finalizers work.](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/#how-finalizers-work) 

### Drain Nodes

Review how to [safely drain a node](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/).

## Delete Node

Karpenter changes the behavior of `kubectl delete node`. Nodes will be drained, and then the underlying instance will be deleted.

## Disruption Budget

Karpenter respects Pod Disruption Budgets. Review what [disruptions are](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/), and [how to configure them](https://kubernetes.io/docs/tasks/run-application/configure-pdb/).

Generally, pod workloads may be configured with `.spec.minAvailable` and/or `.spec.maxUnavailable`. Karpenter provisions nodes to accommodate these constraints. 

## Emptiness

Karpenter will delete nodes (and the instance) that are considered empty of pods. Daemonset pods are not included in this calculation. 

## Expiry

Nodes may be configured to expire. That is, a maximum lifetime in seconds starting with the node joining the cluster. Review the `ttlSecondsUntilExpired` field of the [provisioner API](../../provisioner/).

Note that newly created nodes have a Kubernetes version matching the control plane. One use case for node expiry is to handle node upgrades. Old nodes (with a potentially outdated Kubernetes version) are deleted, and replaced with nodes on the current version. 
