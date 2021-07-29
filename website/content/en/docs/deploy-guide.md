---
title: "Development Guide"
linkTitle: "Development Guide"
weight: 50
---

# Resources Karpenter Expects/Creates

- Cluster on AWS
    - min version 1.16? 1.18?
    - must be EKS?
- IAM Roles
    - `KarpenterNode` -- run containers, configure networking
    - `Karpenter` -- provision new instances
    - IRSA vs kube2iam?
- Service Accounts
    - `Karpenter` -- associated with `Karpenter` IAM Role
- Tagged Subnets? 
    - does this make sense for non-EKS clusters?
- EKS?
    - configure cluster to accept new instances as nodes 
    - must use eksctl? find cfn?
- Instance profile, has at least `KarpenterNode` role
- Launch template
    - will provide one by default


# Deploy Karpenter

- Helm Chart
    - manual option?
    - pulls image from ECR
    - creates deployment
- Default Provisioner?
- Recommend to config options?

# Karpenter Changes Bx of Kubernetes?
- `k delete node`
- unusually high ratio of pods to per node daemonsets
- provisioning
    - watches for event
    - preregisters node -- is this unusual?
    - preassigns pods to expected node -- unusual?
    - do other tools interact with pending pods?
- deprovisioning
    - pod disruption budget?
    - finalizer on nodes?
    - expiration of nodes?
    - deletes empty nodes?
