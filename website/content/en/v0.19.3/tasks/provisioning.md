---
title: "Provisioning"
linkTitle: "Provisioning"
weight: 5
description: >
  Learn about Karpenter provisioners
---

When you first installed Karpenter, you set up a default Provisioner.
The Provisioner sets constraints on the nodes that can be created by Karpenter and the pods that can run on those nodes.
The Provisioner can be set to do things like:

* Define taints to limit the pods that can run on nodes Karpenter creates
* Define any startup taints to inform Karpenter that it should taint the node initially, but that the taint is temporary.
* Limit node creation to certain zones, instance types, and computer architectures
* Set defaults for node expiration

You can change your provisioner or add other provisioners to Karpenter.
Here are things you should know about Provisioners:

* Karpenter won't do anything if there is not at least one Provisioner configured.
* Each Provisioner that is configured is looped through by Karpenter.
* If Karpenter encounters a taint in the Provisioner that is not tolerated by a Pod, Karpenter won't use that Provisioner to provision the pod.
* If Karpenter encounters a startup taint in the Provisioner it will be applied to nodes that are provisioned, but pods do not need to tolerate the taint.  Karpenter assumes that the taint is temporary and some other system will remove the taint.
* It is recommended to create Provisioners that are mutually exclusive. So no Pod should match multiple Provisioners. If multiple Provisioners are matched, Karpenter will randomly choose which to use.

If you want to modify or add provisioners to Karpenter, do the following:

1. Review the following Provisioner documents:

  * [Provisioner](../../getting-started/getting-started-with-eksctl/#provisioner) in the Getting Started guide for a sample default Provisioner
  * [Provisioner API](../../provisioner/) for descriptions of Provisioner API values
  * [Provisioning Configuration](../../AWS/provisioning) for cloud-specific settings

2. Apply the new or modified Provisioner to the cluster.

The following examples illustrate different aspects of Provisioners.
Refer to [Scheduling](../scheduling) to see how the same features are used in Pod specs to determine where pods run.

## Example: Requirements

This provisioner limits nodes to specific zones.
It is flexible to both spot and on-demand capacity types.

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: westzones
spec:
  requirements:
    - key: "topology.kubernetes.io/zone"
      operator: In
      values: ["us-west-2a", "us-west-2b", "us-west-2c"]
    - key: "karpenter.sh/capacity-type"
      operator: In
      values: ["spot", "on-demand"]
  provider:
    instanceProfile: myprofile-cluster101
```
With these settings, the provisioner is able to launch nodes in three availability zones and is flexible to both spot and on-demand purchase types.

## Example: Restricting Instance Types

Not all workloads are able to run on any instance type. Some use cases may be sensitive to a specific hardware generation or cannot tolerate burstable compute. You can specify a variety of well known labels to control the set of instance types available to be provisioned.

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: default
spec:
  provider:
    requirements:
      # Include general purpose instance families
      - key: karpenter.k8s.aws/instance-family
        operator: In
        values: [c5, m5, r5]
      # Exclude smaller instance sizes
      - key: karpenter.k8s.aws/instance-size
        operator: NotIn
        values: [nano, micro, small, large]
      # Exclude a specific instance type
      - key: node.kubernetes.io/instance-type
        operator: NotIn
        values: [m5.24xlarge]
    subnetSelector:
      karpenter.sh/discovery: "${CLUSTER_NAME}" # replace with your cluster name
    securityGroupSelector:
      karpenter.sh/discovery: "${CLUSTER_NAME}" # replace with your cluster name
```

## Example: Isolating Expensive Hardware

A provisioner can be set up to only provision nodes on particular processor types.
The following example sets a taint that only allows pods with tolerations for Nvidia GPUs to be scheduled:

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: gpu
spec:
  ttlSecondsAfterEmpty: 60
  requirements:
  - key: node.kubernetes.io/instance-type
    operator: In
    values: ["p3.8xlarge", "p3.16xlarge"]
  taints:
  - key: nvidia.com/gpu
    value: "true"
    effect: NoSchedule
```
In order for a pod to run on a node defined in this provisioner, it must tolerate `nvidia.com/gpu` in its pod spec.

### Example: Adding the Cilium Startup Taint

Per the Cilium [docs](https://docs.cilium.io/en/stable/gettingstarted/taints/),  it's recommended to place a taint of `node.cilium.io/agent-not-ready=true:NoExecute` on nodes to allow Cilium to configure networking prior to other pods starting.  This can be accomplished via the use of Karpenter `startupTaints`.  These taints are placed on the node, but pods aren't required to tolerate these taints to be considered for provisioning.

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: cilium-startup
spec:
  ttlSecondsAfterEmpty: 60
  startupTaints:
  - key: node.cilium.io/agent-not-ready
    value: "true"
    effect: NoExecute
```
