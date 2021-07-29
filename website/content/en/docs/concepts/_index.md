
---
title: "Intro to Karpenter Concepts"
linkTitle: "Concepts"
weight: 20
resources:
- src: "**fig1*.png"
- src: "**fig2*.png"
---

Kubernetes is designed to schedule new pods onto existing nodes. However, when
existing nodes get full, pods are stuck pending.

Karpenter provisions new capacity in response to pending pods that cannot
be scheduled onto existing nodes. 
Karpenter understands the instance types available from cloud providers. Karpenter’s provisioner intelligently selects new instances to start,
using rich data from both podspecs and the cloud provider instance APIs. 

Karpenter balances responding promptly to un-schedulable pods and making
efficient provisioning decisions. Karpenter is more responsive than
cluster autoscaler, while additionally considering podspec labels and cloud
provider availability zones. Specifically, the commonly used Kubernetes
properties such as labels, taints, affinity (node, pod), and anti-affinity are
supported.

Installing Karpenter on your cluster is a combination of a helm chart, and
configuring the cloud platform to accept provisioning requests from Karpenter.
On AWS, IAM roles for Service Accounts (IRSA) is used. The Kubernetes control
plane in EKS oversees cluster-space Karpenter requests being securely elevated
and passed on to the cloud platform. 

## Provisioner CRD

[Provisioner]({{< ref "/docs/provisioner-crd.md" >}}) is the primary Custom
Resource Definition (CRD) for Karpenter, and you need at least one. 

Notably, one provisioner can handle multiple node profiles (graphics enabled, compute optimized, memory optimized, etc.). Karpenter is group-less, and eliminates
management of multiple node groups with fixed instance specs. In short, the
Karpenter provisioner object is focused on podspecs. This simplifies cluster
management, and reduces the complexity of implementing Karpenter. 

## Provisioning Procedure

Karpenter activates when a pod is un-schedulable. In response, it starts
provisioning a new node, including the cloud provider instance. 

Karpenter makes new instances based on the Provisioner CRD. In the default
configuration, Karpenter automatically selects from all the general purpose
instances available, with an intent to accommodate the unscheduled pods
efficiently. Alternatively, the Provisioner CRD can define a list of acceptable
instance types. 

The Provisioner CRD includes labels for the new nodes. Certain labels, such as
`kubernetes.io/arch`, change the instances Karpetner will provision. For
example, setting `kubernetes.io/arch=arm64` configures Karpenter to provision
ARM based instances. All nodes created by a provisioner are labeled with
`karpenter.sh/provisioner-name: <name-from-crd>`.

[[todo: right float image]]
[[todo: land label docs in main, then add links]]

{{< imgproc fig1 Fit "400x450" >}}
{{< /imgproc >}}

### Binpacking

{{< imgproc fig2 Fit "400x450" >}}
{{< /imgproc >}}

## Reallocation

Currently, Karpenter does not support reallocation. A large instance with few
pods will not be terminated based on usage. Review the deprovisioning triggers
below. 

## Deprovisioning Triggers

Currently, Karpenter supports deprovisioning instances based on two node conditions:
- number of seconds from node creation, known as "node expiry"
- number of seconds with no pods, known as "node emptiness" 

### Node Expiry

Setting a value for `ttlSecondsUntilExpired` enables node expiration. The value
is the number of seconds after node creation until nodes are viewed as expired
by Karpenter. Note, with this value set, all nodes will eventually expire.
Expired nodes are drained and replaced. The replacement nodes will have the
latest updates, and may be more efficiently sized.

### Empty Nodes

Setting a value for `ttlSecondsAfterEmpty` enables deprovisioning empty nodes
(no pods besides daemon sets). This only happens if a node becomes empty, and
stays empty for the set number of seconds. 

## Finalizer?

[[todo: karpenter finalizer]]

