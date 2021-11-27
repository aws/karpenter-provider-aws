---
title: "Provisioning Configuration"
linkTitle: "Provisioning"
weight: 10
---

The [Provisioner CRD]({{< ref "../reference/provisioner-crd.md" >}}) provides two sections for constraining nodes. 

- [`spec.requirements`](../reference/provisioner-crd#specrequirements)
  - Cloud Provider Agnostic 
  - Kubernetes Well Known Labels
  - This section includes generally applicable constraints (zone, instance type) that each cloud provider is expected to implement. 
  - Reference the Provisioner CRD for more information. 
- [`spec.provider`](#specprovider)
  - Cloud Provider Specific
  - This section defines constraints that are unique to AWS, such as SecurityGroups.
  - Defined below. 

## spec.provider

cloud provider specific constrains

[Review these fields in the code.](https://github.com/awslabs/karpenter/blob/main/pkg/cloudprovider/aws/apis/v1alpha1/provider.go#L33)

### InstanceProfile
An `InstanceProfile` is a way to pass a single IAM role to an EC2 instance.

It is required, and specified by name. A suitable `KarpenterNodeRole` is created in the getting started guide.

```
spec:
  provider:
    instanceProfile: MyInstanceProfile
```

### LaunchTemplate

A launch template is a set of configuration values sufficient for launching an EC2 instance (e.g., AMI, storage spec).

A custom launch template be specified by name. If none is specified, Karpenter will automatically create a launch template.

Review the [Launch Template documentation](launch-templates.md) to learn how to create a custom one.

```
spec:
  provider:
    launchTemplate: MyLaunchTemplate
```

### SubnetSelector
By default, Karpenter discovers subnets by tags. Alternatively, cluster subnets may list specific subnets.

Subnets may be specified by AWS tag, or by name. Either approach supports wildcards. 

When creating an instance, Karpenter picks a single subnet from this this. 

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

Select subnets by an arbitrary AWS tag key/value pair:
```
  subnetSelector:
    MySubnetTag: value

Select subnets using wildcards:
```
  subnetSelector:
    Name: *public* 
    MySubnetTag: '' # all resources with this tag

```

### SecurityGroupSelector

Karpenter uses the EKS default security group, unless another is specified. The security group of an instance is comparable to a set of firewall rules.

EKS creates at least two security groups, [review the documentation](https://docs.aws.amazon.com/eks/latest/userguide/sec-group-reqs.html) for more info.

Security Groups may be specified by AWS tag, or by name. Either approach supports wildcards. 

Each instance gets *all* of the listed security groups.

**Examples**

Select all security groups with a specified tag:
```
spec:
  provider:
    securityGroupSelector:
      kubernetes.io/cluster/MyKarpenterSecurityGroups: '*'
```

Select security groups by name, or another tag:
```
 securityGroupSSelector:
   Name: sg-01077157b7cf4f5a8
   MySecurityTag: '' # matches all resources with the tag
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

Additionally, include a resource requirement in the workload manifest. This will cause the GPU dependent pod will be scheduled onto the appropriate node.

*Accelerator resource in workload manifest (e.g., pod)*

```yaml
spec:
  template:
    spec:
      containers:
      - resources:
          limits:
            nvidia.com/gpu: "1"
```
