---
title: "Provisioning"
linkTitle: "Provisioning"
weight: 5
---

When you first installed Karpenter, you set up a default [Provisioner](../getting-started/#provisioner).
The Provisioner sets constraints on the nodes that can be created by Karpenter and the pods that can run on those nodes.
The Provisioner can be set to do things like:

* Define taints to limit the pods that can run on nodes Karpenter creates
* Limit nodes creation to certain zones, instance types, and computer architectures
* Set defaults for node expiration

You can change your provisioner or add other provisioners to Karpenter.
Here are things you should know about Provisioners:

* Karpenter won't do anything if there is not at least one Provisioner configured.
* Each Provisioner that is configured is looped through by Karpenter.
* If Karpenter encounters a taint in the Provisioner that is not tolerated by a Pod, Karpenter won't use that Provisioner to provision the pod.
* It is best to not create Provisioners that are mutually exclusive. So no Pod should match multiple Provisioners. If multiple Provisioners are matched, Karpenter will randomly choose which to use.

If you want to modify or add provisioners to Karpenter, do the following:

1. Review the following Provisioner documents:
  * [Provisioner](../getting-started/#provisioner) in the Getting Started guide for a sample default Provisioner.
  * [Provisioner API](../provisioner-crd) for a description of Provisioner custom resource.
  * [Provisioning Configuration](../AWS/constraints) for cloud-specific settings.

1. Apply the new or modified Provisioner to the cluster.

The following examples illustrate different aspects of Provisioners.
Refer to [Running pods](../tasks/running-pods) to see how the same features are used in Pod specs to determine where pods run.

## Example: Requirements

This provisioner limits node selection to those in US West availability zones.
It allows both spot and on-demand capacity types, but prefers spot.
The operating system used is defined by the instanceProfile called myprofile-cluster101.

```
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
With these settings, pods requiring availability zones other than the three `us-west` zones shown would not match the provisioner.

## Example: Nvidia GPUs

A provisioner can be set up to only provision nodes on particular processor types.
The following example sets a taint that only allows pods with tolerations for Nvidia GPUs to be scheduled:

```
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
    value: true
    effect: “NoSchedule”
```
In order for a pod to run on a node defined in this provisioner, it must tolerate nvidia.com/gpu it its pod spec.
As long as the pod doesn't request other `instance-type` values, it will be assigned to a `p3.8xlarge` or `p3.16xlarge` instance type.
In this example, if no pods are running for 60 seconds, the nodes will be scaled down.
