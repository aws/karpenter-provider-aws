---
title: "Control Pod Density"
linkTitle: "Control Pod Density"
weight: 20
description: >
  Learn ways to specify pod density with Karpenter
---

Pod density is the number of pods per node. 

Kubernetes has a default limit of 110 pods per node. If you are using the EKS Optimized AMI on AWS, the [number of pods is limited by instance type](https://github.com/awslabs/amazon-eks-ami/blob/master/files/eni-max-pods.txt) in the default configuration.

## Increase Pod Density

### Networking Limitations 

*☁️ AWS Specific*

By default, the number of pods on a node is limited by both the number of networking interfaces (ENIs) that may be attached to an instance type and the number of IP addresses that can be assigned to each ENI.  See [IP addresses per network interface per instance type](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-eni.html#AvailableIpPerENI) for a more detailed information on these instance types' limits.

Karpenter can be configured to disable nodes' ENI-based pod density.  This is especially useful for small to medium instance types which have a lower ENI-based pod density. 

{{% alert title="Note" color="primary" %}}
When using small instance types, it may be necessary to enable [prefix assignment mode](https://aws.amazon.com/blogs/containers/amazon-vpc-cni-increases-pods-per-node-limits/) in the AWS VPC CNI plugin to more pods per node.  Prefix assignment mode was introduced in AWS VPC CNI v1.9 and allows ENIs to manage a broader set of IP addresses.  Much higher pod densities are supported as a result.
{{% /alert %}}

### Provisioner-Specific Pod Density

#### Static Pod Density

Static pod density can be configured at the provisioner level by specifying `maxPods` within the `.spec.kubeletConfiguration`. All nodes spawned by this provisioner will set this `maxPods` value on their kubelet and will account for this value during scheduling.

See [Provisioner API Kubelet Configuration](../../provisioner/#max-pods) for more details.

#### Dynamic Pod Density

Dynamic pod density (density that scales with the instance size) can be configured at the provisioner level by specifying `podsPerCore` within the `.spec.kubeletConfiguration`. Karpenter will calculate the expected pod density for each instance based on the instance's number of logical cores (vCPUs) and will account for this during scheduling.

See [Provisioner API Kubelet Configuration](../../provisioner/#pods-per-core) for more details.

### Controller-Wide Pod Density

{{% alert title="Deprecation Warning" color="warning" %}}
`AWS_ENI_LIMITED_POD_DENSITY` is deprecated in favor of the `.spec.kubeletConfiguration.maxPods` set at the Provisioner-level
{{% /alert %}}

Set the environment variable `AWS_ENI_LIMITED_POD_DENSITY: "false"` (or the argument  `--aws-eni-limited-pod-density=false`) in the Karpenter controller to allow nodes to host up to 110 pods by default.

Environment variables for the Karpenter controller may be specified as [helm chart values](https://github.com/aws/karpenter/blob/c73f425e924bb64c3f898f30ca5035a1d8591183/charts/karpenter/values.yaml#L15).


## Limit Pod Density

Generally, increasing pod density is more efficient. However, some use cases exist for limiting pod density. 

### Topology Spread

You can use [topology spread]({{< relref "scheduling.md#topology-spread" >}}) features to reduce blast radius. For example, spreading workloads across EC2 Availability Zones.


### Restrict Instance Types

Exclude large instance sizes to reduce the blast radius of an EC2 instance failure.

Consider setting up upper or lower boundaries on target instance sizes with the node.kubernetes.io/instance-type key. 

The following example shows how to avoid provisioning large Graviton instances in order to reduce the impact of individual instance failures:

```
-key: node.kubernetes.io/instance-type
    operator: NotIn
    values:
      'm6g.16xlarge'
      'm6gd.16xlarge'
      'r6g.16xlarge'
      'r6gd.16xlarge'
      'c6g.16xlarge'
```




