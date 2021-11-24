---
title: "AWS Specific Provisioning Constraints"
linkTitle: "Constraints"
weight: 10
---

The [Provisioner CRD]({{< ref "../reference/provisioner-crd.md" >}}) provides two sections for constraining nodes. 

- [`spec.requirements`](#specrequirements)
  - This section includues generally applicable constraints (zone, instance type) that each cloud provider is expected to implement. 
- [`spec.provider`](#specprovider)
- This section defines constraints that are unique to AWS, such as SecurityGroups.

Consider this requirement:
```
requirements:
  - key: "topology.kubernetes.io/zone"
    operator: In
    values: ["us-west-2a", "us-west-2b"]
  ```

In response, Karpenter will provision nodes in only those availability zones. A podspec may *further specify* this with a NodeSelector requesting a specifically a label with the key of "topology.kubernetes.io/zone" and a value of "us-west-2b". However, the podspec (or another deployment resource) may not *override* the requiremnets set at the provisioner. 

## spec.requirements

Kubernetes defines these well known labels, and AWS merely implements them. They are defined at the "spec.requirements" section of the provisioner CRD. 

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

[[confirm: can this value not be in the set defined in the provisioner?]]

### Availability Zones

`topology.kubernetes.io/zone=us-east-1c`

- key: `topology.kubernetes.io/zone`
- value example: `us-east-1c`
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
- values
  - `spot` (default)
  - `on-demand` 
  
Karpenter supports specifying capacity type, which is analogous to EC2 usage classes (aka "market types") and defaults to on-demand.

Karpenter defaults to spot instances. [Spot instances](https://aws.amazon.com/ec2/spot/) may be preempted, and should not
be used for critical workloads that do not tolerate interruptions.

Set this value to "on-demand" to prevent critical workloads from being interrupted.

[[note: I'm still very uneasy with this policy. I thought the AWS offical line was that spot is not for "cheaper EC2" but only for "workloads that are only feasible reasonable to run at lower prices" and "you shouldn't expect to get a new on-demand EC2 instance when a spot one is terminated, because we just told you capacity is constrained".]]

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

## spec.provider

cloud provider specific constrains

[Review these fields in the code.](https://github.com/awslabs/karpenter/blob/main/pkg/cloudprovider/aws/apis/v1alpha1/provider.go#L33)

### InstanceProfile
An `InstanceProfile` is a region specific EC2 resource that is comprised of a reference to a single global IAM role.

It is required, and specified by name. A suitable `KarpenterNodeRole` is created in the getting started guide.

```
spec:
  provider:
    instanceProfile: MyInstanceProfile
```

### LaunchTemplate

A launch template is a set of configuration values sufficent for launching an EC2 instance (e.g., AMI, storage spec).

A custom launch templay *may optionally* be specified by name. If none is specified, Karpenter will automatically create a launch template.

Review the [Launch Template documentation](launch-templates.md) to learn how to create a custom one.

```
spec:
  provider:
    launchTemplate: MyLaunchTemplate
```

### SubnetSelector
By default, Karpenter discovers subnets by tags. Alternatively, cluster subnets may be directly enumerated. 

Subnets may be specified by AWS tag, or by name. Either approach supports wildcards. 

Each instance gets *one* subnet *chosen* from this list.

**Examples**

Select all subnets with a specified tag:
```
spec:
  provider:
    subnetSelector:
      kubernetes.io/cluster/MyCluster: '*'
```

Select subnets by name:
```
 subnetSelector:
   Name: subnet-0fcd7006b3754e95e
```

Select subnets by name using a wildcard:
```
 subnetSelector:
   Name: *public*
```

### SecurityGroupSelector

Karpenter uses the EKS default security group, unless another is specified. The security group of an instance is comperable to a set of firewall rules.

EKS creates at least two security groups, review the documentation for more info.

Each instance gets *all* of the listed security groups.

**Examples**

Select all security groups with a specified tag:
```
spec:
  provider:
    securityGroupSelector:
      kubernetes.io/cluster/MyKarpenterSecurityGroups: '*'
```

Select security groups by name:
```
 subnetSelector:
   Name: sg-01077157b7cf4f5a8
```

Select security groups by name using a wildcard:
```
 subnetSelector:
   Name: *public*
```

### Tags

All listed tags will be added to every node created by this provisioner.

```
spec:
  provider:
    tags:
      InternalAccountingTag: 1234
      dev.corp.net/app: Calculator
      dev.corp.net/team: MyTeam
```

## Other Resources

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
