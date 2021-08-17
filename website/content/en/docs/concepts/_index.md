
---
title: "Intro to Karpenter Concepts"
linkTitle: "Concepts"
weight: 20
resources:
- src: "**fig1*.png"
- src: "**fig2*.png"
---

Kubernetes is designed to schedule new pods onto existing cluster nodes. However, when existing nodes don't have available capacity, new Pods end up in a Pending state

Karpenter provisions new capacity in response to pending pods that cannot
be scheduled onto existing nodes. 
Karpenter understands the instance types available from cloud providers. Karpenter’s provisioner selects new instances to start,
using data from both podspecs and the cloud provider instance APIs. 

Karpenter strikes a balance between provisioning new resources quickly, and making the most efficient provisioning decisions while doing so. Karpenter is more responsive than
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

Karpenter provisions new instances based on the Provisioner CRD. In the default
configuration, Karpenter automatically selects from all the general purpose
instances available, with an intent to accommodate the unscheduled pods
efficiently. Alternatively, the Provisioner CRD can define a list of acceptable
instance types. 

The Provisioner CRD includes labels for the new nodes. Certain labels, such as
`kubernetes.io/arch`, change the instances Karpetner will provision. For
example, setting `kubernetes.io/arch=arm64` configures Karpenter to provision
Arm-based instances. All nodes created by a provisioner are labeled with
`karpenter.sh/provisioner-name: <name-from-crd>`.

[[todo: right float image]]

{{< imgproc fig1 Fit "400x450" >}}
{{< /imgproc >}}

## Constraints
Constraints control the nodes provisioned by karpenter, and how pods are scheduled onto nodes. 

Importantly, Karpenter uses a "defaults and overrides" model to handle constrains.

This behavior is exemplified by instance types. 

The global default for karpenter is to include all cloud provider instance types. This may be changed for a specific provisioner at `spec.instanceTypes`.

Now, for pods under the scope of the provisioner, karpenter will automatically chose an instance type from that list.

However, pods may also have a node selector that further ovverides the provisioner list of default instance types. For example, if a pod includes a Pod NodeSelector value at `node.kubernetes.io/instance-type` of a speicific instance type, then Karpenter will provision that instance type for it.

Note, Karpenter will provision this because it's an override, and not outright reject the request due to a conflict. 

In general, contrains have global default values, (optionally) values set at the provisioner, and (optionally) a specific override in the pod spec. 

What are constraints?

Constraints describe an instance. In terms of the instance a pod wants specifically, or a general type of instance karpenter should provision. 

A pod may want a specific GPU. You may want to generally have karpenter provision in specific zones or with security groups. 

Constrains often take the form of labels. We have constrains as first class objects in the provisioner spec if they are [an upstream kubernetes concept]. Alternatively, constrains can be any form of label. Some lables are well known, and honored by the clodu provider. We don't put cloud provider specific stuff in the spec.

From the pod perspective, it's always a node selector label. 

### Well Known Labels
    Vendor Neutral (upstream concept)
    Vendor Specific (see AWS docs, provide example)
    Fields, corresponding pod label (move to elsewhere)

### Taints
    When/why to use taints

## Allocation (Binpacking)

{{< imgproc fig2 Fit "400x450" >}}
{{< /imgproc >}}

//concept
//how it works now, basically largest

## Reallocation (Scale Down)

Currently, Karpenter does not support reallocation. A large instance with few
pods will not be terminated based on usage. Review the deprovisioning triggers
below. 

## Deprovisioning Triggers

Currently, Karpenter supports deprovisioning instances based on two node conditions:
- number of seconds from node creation, known as "node expiry"
- number of seconds with no pods running, known as "node emptiness" 

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

## Finalizer

- need to delete this if you delete karpenter with nodes still out there 