---
title: "Amazon Web Services (AWS)"
linkTitle: "AWS"
weight: 10
---

## Control Provisioning with Labels

The [Provisioner CRD]({{< ref "provisioner-crd.md" >}}) supports defining
instance types, node labels, and node taints.

For certain well-known labels (documented below), Karpenter will provision
nodes accordingly. For example, in response to a label of
`topology.kubernetes.io/zone=us-east-1c`, Karpenter will provision nodes in
that availability zone.

Regarding AWS, 3 types of provisioning constraints are recognized: 
- [Instance Type](#instance-type-allowlist)
- [Node Labels](#node-lables-in-provisioner-spec) (e.g., Architecture, Capacity Type)
- [Pod Labels](#pod-labels) (e.g., GPU)

## Instance Type Allowlist

Karpenter supports specifying [AWS instance type](https://aws.amazon.com/ec2/instance-types/). 

The default value includes all instance types with the exclusion of metal
(non-virtualized),
[non-HVM](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/virtualization_types.html),
and GPU instances. 

If necessary, Karpenter supports defining a limited list of instance types. 

If one instance type is listed, Karpenter will always provision that type.

If more than one type is listed, Karpenter will determine the
instance type to minimize the number of new nodes.

View the full list of instance types with `aws ec2 describe-instance-types`.

**Example**

*Set Default with provisioner.yaml*

```yaml
spec:
  instanceTypes:
    - m5.large
```

*Override with workload manifest (e.g., pod)*

```yaml
spec:
  template:
    spec:
      nodeSelector:
        node.kubernetes.io/instance-type: m5.large
```

## Node Labels in Provisioner Spec

### Launch Template

Karpenter uses [AWS Bottlerocket OS](https://aws.amazon.com/bottlerocket/) by
default. More specifically, Karpenter automatically creates a launch template
with the name `Karpenter-<cluster name>-uuid` for each region where a node is
provisioned.

You can specify a different [launch
template](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-launch-templates.html),
such as a customized version of Bottlerocket or a different platform
altogether.

**Template ID**
- key: `node.k8s.aws/launch-template-id`
- value example: `bottlerocket` (default)
- value list: `aws ec2 describe-launch-templates`

### Capacity Type (e.g., spot)

- key: `node.k8s.aws/capacity-type`
- values
  - `on-demand` (default)
  - `spot`

Karpenter supports specifying capacity type and defaults to on-demand.

Specify this value on the provisioner to enable spot instances. [Spot
instances](https://aws.amazon.com/ec2/spot/) may be preempted, and should not
be used for critical workloads.

**Example**

*Set Default with provisioner.yaml*

```yaml
spec:
  labels: 
    node.k8s.aws/capacity-type: spot
```

*Override with workload manifest (e.g., pod)*

```yaml
spec:
  template:
    spec:
      nodeSelector:
        node.k8s.aws/capacity-type: spot
```

### Architecture (e.g., ARM) 

- key: `kubernetes.io/arch`
- values
  - `amd64` (default)
  - `arm64`

Karpenter supports `amd64` nodes, and `arm64` nodes. 

**Example**

*Set Default with provisioner.yaml*

```yaml
spec:
  labels: 
    kubernetes.io/arch: arm64
```

*Override with workload manifest (e.g., pod)*

```yaml
spec:
  template:
    spec:
      nodeSelector:
        kubernetes.io/arch: amd64
```

## Operating System

- key: `kubernetes.io/os`
- values
  - `linux` (default)

At this time, Karpenter only supports Linux OS nodes.

### Availability Zones

`topology.kubernetes.io/zone=us-east-1c`

- key: `topology.kubernetes.io/zone`
- value example: `us-east-1c` or `use1-az1`
- value list: `aws ec2 describe-availability-zones --region <region-name>`

Karpenter can be configured to create nodes in a particular zone. Karpenter
supports (1) availability zone IDs, and (2) availability zone names. 

Availability zone IDs, such as `use1-az1`, are consistent between AWS accounts.

Availability zone names, such as `us-east-1c`, are randomly mapped to zone IDs
on an account level. For example, the Availability Zone us-east-1a for your AWS
account might not have the same location as us-east-1a for another AWS account. 

[Learn more about Availability Zone
IDs.](https://docs.aws.amazon.com/ram/latest/userguide/working-with-az-ids.html)

## Pod Labels

### Accelerators, GPU 

Accelerator (e.g., GPU) values include
- `nvidia.com/gpu`
- `amd.com/gpu`
- `aws.amazon.com/neuron`

Karpenter supports accelerators, such as GPUs. 

To enable instances with accelerators, use the [instance type
well known label selector](#instance-type-allowlist). 

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
