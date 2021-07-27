---
title: "Amazon Web Services (AWS)"
linkTitle: "AWS"
weight: 10
---

## Control Provisioning with Labels

The [Provisioner CRD]({{< ref "provisioner-crd.md" >}}) supports defining instance types, node labels, and node taints. 

For certain well-known labels (documented below), Karpenter will provision nodes accordingly. For example, in response to a label of 
`topology.kubernetes.io/zone=us-east-1c`, Karpenter will provision nodes in that availability zone. 

Regarding AWS, 3 types of provisioning constraints are recognized: 
- [Instance Type](#instance-type-allowlist)
- [Node Labels](#node-lables-in-provisioner-spec) (e.g., Architecture, Capacity Type)
- [Pod Labels](#pod-labels) (e.g., GPU)

## Instance Type Allowlist

Karpenter supports specifying [AWS instance type](https://aws.amazon.com/ec2/instance-types/). 

If one instance type is listed, Karpenter will always provision that type.

If more than one type is listed, Karpenter will intelligently select the instance type to maximize node utilization. 

The default value includes all instance types with the exclusion of metal (non-virtualized), [non-HVM](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/virtualization_types.html), and GPU instances. View the full list with `aws ec2 describe-instance-types`.

**Example**

*Set Default with provisioner.yaml*

```yaml
spec:
  instanceTypes:
    - m5.large
```

*Override with pod.yaml*

```yaml
spec:
  template:
    spec:
      nodeSelector:
        node.k8s.aws/instance-type: m5.large
```

## Node Lables in Provisioner Spec

### Launch Template

Karpenter uses [AWS Bottlerocket](https://aws.amazon.com/bottlerocket/) by default. 

You can specify a different [launch template](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-launch-templates.html), such as a customized version
of Bottlerocket or a different platform altogether. 

**Template ID**
- key: `node.k8s.aws/launch-template-id`
- value example: `bottlerocket` (default)
- value list: `aws ec2 describe-launch-templates`

**Template Version**
- key: `node.k8s.aws/launch-template-version`
- value example: `3`
- default: `$LATEST`
- value list: `aws ec2 describe-launch-templates`

### Capacity Type (e.g., spot)

- key: `node.k8s.aws/capacity-type`
- values
  - `on-demand` (default)
  - `spot`

Karpenter supports specifying capacity type and defaults to on-demand.

Specify this value on the provisioner to enable spot instance pricing. [Spot instances](https://aws.amazon.com/ec2/spot/) may be preempted, and should not be used for critical workloads. 

**Example**

*Set Default with provisioner.yaml*

```yaml
spec:
  labels: 
    node.k8s.aws/capacity-type: spot
```

*Override with pod.yaml*

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

Karpenter supports `amd64` (e.g., intel) nodes, and `arm64` nodes. The default is `amd64`. 

**Example**

*Set Default with provisioner.yaml*

```yaml
spec:
  labels: 
    kubernetes.io/arch: arm64
```

*Override with pod.yaml*

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

At this time, Karpenter only supports linux nodes. 

### AWS Region

`topology.kubernetes.io/region=us-east-1`

- key: `topology.kubernetes.io/region`
- value example: `us-east-1`
- value list: [AWS regional endpoints](https://docs.aws.amazon.com/general/latest/gr/rande.html)

Karpenter can be configured to create nodes in a particular region. 

### Availability Zones

`topology.kubernetes.io/zone=us-east-1c`

- key: `topology.kubernetes.io/zone`
- value example: `us-east-1c`
- value list: `aws ec2 describe-availability-zones --region <region-name>`

Karpenter can be configured to create nodes in a particular availability zone. 

## Pod Labels

### Accelerators, GPU 

Accelerator (e.g., GPU) values include
- `nvidia.com/gpu`
- `amd.com/gpu`
- `aws.amazon.com/neuron`

Karpenter supports GPUs. To specify a specific GPU type, use the instance type well known label selector (see above).

However, a specific GPU requirement, including type, can be made on a pod spec. 

*Override with pod.yaml*

```yaml
spec:
  template:
    spec:
      containers:
      - resources:
          limits:
            nvidia.com/gpu: "1"
```
