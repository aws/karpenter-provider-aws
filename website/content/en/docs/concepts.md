
---
title: "Intro to Karpenter Concepts"
linkTitle: "Concepts"
weight: 20
---

Kubernetes is designed to schedule new pods onto existing nodes. However, when existing nodes get full, Kubernetes struggles with automatically provisioning new nodes from a cloud provider.

Karpenter activates when pending pods are unable to be fit onto existing nodes. Karpenter understands the instance types available from cloud providers, such as AWS. Karpenter’s provisioner intelligently selects new instances to start, using rich data from both podspecs and the cloud provider APIs (such as EC2’s flexible fleet API). 

[[detail on spot and platform availability — or this too platform specific?]]

Karpenter balances responding rapidly to un-schedulable pods and making efficient provisioning decision. Karpenter is substantially faster than the cluster autoscaler, while additionally considering podspec labels and cloud provider availability zones. Specifically, the commonly used kubernetes properties such as labels, taints, affinity (node, pod), and anti-affinity are supported.

Activating Karpenter on your cluster is a combination of a helm chart, and configuring the cloud platform to accept provisioning requests from Karpenter. On AWS, IAM roles for Service Accounts (IRSA) is used. The Kubernetes control plane in EKS oversees cluster-space [[is that a term?]] Karpenter requests being securely elevated and passed on to the cloud platform. 

## Provisioner CRD

The primary kubernetes API object kind is “provisioner”. Notably, one provisioner can handle multiple node profiles (graphics enabled, compute optimized, memory optimized, etc). Karpenter is group-less, and eliminates management of multiple node groups with fixed instance specs. In short, the Karpenter provisioner object is focused on podspecs. This simplifies cluster management, and reduces the complexity of implementing Karpenter. 

Provisioner is the primary Custom Resource Definition (CRD) for Karpenter, and you need at least one. 

The provisioner CRD includes...

### Deprovisioning 

Second, define termination and downscaling values. Setting a value for `ttlSecondsUntilExpired` enables node expiration. The value is the number of seconds after node creation until nodes are viewed as expired by Karpenter. Note, with this value set, all nodes will eventually expire. Expired nodes are drained and replaced. The replacement nodes will have the latest updates, and may be more efficiently sized. Setting a value for `ttlSecondsAfterEmpty` enables deprovisioning empty nodes (no pods besides daemon sets). This only happens if a node becomes empty, and stays empty for the set number of seconds. 
Third, consider any taints and labels you want provisioned nodes to have. Some labels impact the behavior of AWS, such as setting the launch template ID or opting in to spot pricing. 

### Well Known Labels

## Provisioning Walkthrough
- how much talk about cloud provider?

### How instance types are selected (currently)

## Deprovisioning Walkthrough

[[what is terminator?]]