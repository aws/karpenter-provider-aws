---
title: "Provisioner API"
linkTitle: "Provisioner API"
weight: 70
date: 2017-01-05
description: >
  Provisioner API reference page
---

## Example Provisioner Resource

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: default
spec:
  # If nil, the feature is disabled, nodes will never expire
  ttlSecondsUntilExpired: 2592000 # 30 Days = 60 * 60 * 24 * 30 Seconds;

  # If nil, the feature is disabled, nodes will never scale down due to low utilization
  ttlSecondsAfterEmpty: 30

  # Provisioned nodes will have these taints
  # Taints may prevent pods from scheduling if they are not tolerated
  taints:
    - key: example.com/special-taint
      effect: NoSchedule

  # Labels are arbitrary key-values that are applied to all nodes
  labels:
    billing-team: my-team

  # Requirements that constrain the parameters of provisioned nodes.
  # These requirements are combined with pod.spec.affinity.nodeAffinity rules.
  # Operators { In, NotIn } are supported to enable including or excluding values
  requirements:
    - key: "node.kubernetes.io/instance-type" # If not included, all instance types are considered
      operator: In
      values: ["m5.large", "m5.2xlarge"]
    - key: "topology.kubernetes.io/zone" # If not included, all zones are considered
      operator: In
      values: ["us-west-2a", "us-west-2b"]
    - key: "kubernetes.io/arch" # If not included, all architectures are considered
      operator: In
      values: ["arm64", "amd64"]
    - key: "karpenter.sh/capacity-type" # If not included, the webhook for the AWS cloud provider will default to on-demand
      operator: In
      values: ["spot", "on-demand"]
  # These fields vary per cloud provider, see your cloud provider specific documentation
  provider: {}
```

## spec.requirements

Kubernetes defines the following [well known labels]([[link]]), and cloud providers (e.g., AWS) merely implement them. They are defined at the "spec.requirements" section of the provisioner CRD. 

### Instance Types

Generally, instance types should be a list. Leaving this field undefined is reccommended, as it maximizes choices for efficently placing pods. 

Instance type may also be specified using a nodeSelector in a workload definition. If the instance type requested at the workload (e.g., pod) level is not included in the provisioner list, karpenter will not create a node or schedule the pod. 

☁️ AWS

Review [AWS instance types](https://aws.amazon.com/ec2/instance-types/).

The default value includes all instance types with the exclusion of metal
(non-virtualized),
[non-HVM](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/virtualization_types.html),
and GPU instances.

View the full list of instance types with `aws ec2 describe-instance-types`.

**Example**

*Set Default with provisioner.yaml*

```yaml
spec:
  requirements:
    - key: node.kubernetes.io/instance-type
      operator: In
      values: ["m5.large", "m5.2xlarge"]
```

*Override with workload manifest (e.g., pod)*

```yaml
spec:
  template:
    spec:
      nodeSelector:
        node.kubernetes.io/instance-type: m5.large
```

### Availability Zones

- key: `topology.kubernetes.io/zone`
- value example: `us-east-1c`

☁️ AWS

- value list: `aws ec2 describe-availability-zones --region <region-name>`

Karpenter can be configured to create nodes in a particular zone. Note that the Availability Zone us-east-1a for your AWS account might not have the same location as us-east-1a for another AWS account.

[Learn more about Availability Zone
IDs.](https://docs.aws.amazon.com/ram/latest/userguide/working-with-az-ids.html)

**Example**

```
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
spec:
  requirements:
    # If not included, all instance types are considered
    - key: "topology.kubernetes.io/zone"
      operator: In
      values: ["us-west-2a", "us-west-2b"]

```

**Override**

```yaml
spec:
  template:
    spec:
      nodeSelector:
        topology.kubernetes.io/zone: us-west-2a
```

### Architecture

- key: `kubernetes.io/arch`
- values
  - `amd64` (default)
  - `arm64`

Karpenter supports `amd64` nodes, and `arm64` nodes.

**Example**

*Set Default with provisioner.yaml*

```yaml
spec:
  requirements:
    - key: kubernetes.io/arch
      operator: In
      values: ["arm64", "amd64"]
```

*Override with workload manifest (e.g., pod)*

```yaml
spec:
  template:
    spec:
      nodeSelector:
        kubernetes.io/arch: amd64
```

### Operating System

- key: `kubernetes.io/os`
- values
  - `linux` (default)

At this time, Karpenter on AWS only supports Linux OS nodes.

### Capacity Type

- key: `karpenter.sh/capacity-type`

☁️ AWS

- values
  - `spot` (default)
  - `on-demand` 

Karpenter supports specifying capacity type, which is analogous to EC2 usage classes (aka "market types").

Karpenter defaults to spot instances. [Spot instances](https://aws.amazon.com/ec2/spot/) may be preempted, and should not be used for critical workloads that do not tolerate interruptions.

Set this value to "on-demand" to prevent critical workloads from being interrupted.

**Example**

*Set Default with provisioner.yaml*

```yaml
spec:
  requirements:
    - key: karpenter.sh/capacity-type
      operator: In
      values: ["spot", "on-demand"]
```

*Override with workload manifest (e.g., pod)*

```yaml
spec:
  template:
    spec:
      nodeSelector:
        karpenter.sh/capacity-type: spot
```

