---
title: "Specifying Values to Control AWS Provisioning"
linkTitle: "Spec Fields"
weight: 10
---

The [Provisioner CRD]({{< ref "../../provisioner-crd.md" >}}) supports defining
node properties like instance type and zone. For certain well-known labels (documented below), Karpenter will provision
nodes accordingly. For example, in response to a label of
`topology.kubernetes.io/zone=us-east-1c`, Karpenter will provision nodes in
that availability zone.

### Instance Types

Karpenter supports specifying [AWS instance type](https://aws.amazon.com/ec2/instance-types/).

The default value includes all instance types with the exclusion of metal
(non-virtualized),
[non-HVM](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/virtualization_types.html),
and GPU instances.

If necessary, Karpenter supports defining a limited list of default instance types.

If more than one type is listed, Karpenter will determine the
instance type to minimize the number of new nodes.

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

`topology.kubernetes.io/zone=us-east-1c`

- key: `topology.kubernetes.io/zone`
- value example: `us-east-1c`
- value list: `aws ec2 describe-availability-zones --region <region-name>`

Karpenter can be configured to create nodes in a particular zone. Note that the Availability Zone us-east-1a for your AWS account might not have the same location as us-east-1a for another AWS account.

[Learn more about Availability Zone
IDs.](https://docs.aws.amazon.com/ram/latest/userguide/working-with-az-ids.html)

### Capacity Type

- key: `karpenter.sh/capacity-type`
- values
  - `on-demand` (default)
  - `spot`

Karpenter supports specifying capacity type, which is analogous to EC2 usage classes (aka "market types") and defaults to on-demand.

Specify this value on the provisioner to enable spot instances. [Spot instances](https://aws.amazon.com/ec2/spot/) may be preempted, and should not
be used for critical workloads that do not tolerate interruptions.

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

At this time, Karpenter only supports Linux OS nodes.

### Accelerators, GPU

Accelerator (e.g., GPU) values include
- `nvidia.com/gpu`
- `amd.com/gpu`
- `aws.amazon.com/neuron`

Karpenter supports accelerators, such as GPUs.

To enable instances with accelerators, use the [instance type
well known label selector](#instance-types).

Additionally, include a resource requirement in the workload manifest. Thus,
accelerator dependent pod will be scheduled onto the appropriate node.

*accelerator resource in workload manifest (e.g., pod)*

```yaml
spec:
  template:
    spec:
      containers:
      - resources:
          limits:
            nvidia.com/gpu: "1"
```
