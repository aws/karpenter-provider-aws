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
  # If omitted, the feature is disabled and nodes will never expire.  If set to less time than it requires for a node
  # to become ready, the node may expire before any pods successfully start.
  ttlSecondsUntilExpired: 2592000 # 30 Days = 60 * 60 * 24 * 30 Seconds;

  # If omitted, the feature is disabled, nodes will never scale down due to low utilization
  ttlSecondsAfterEmpty: 30

  # If omitted, no instance type filtering is performed apart from any implied by requirements on the provisioner or
  # node selectors on workloads.
  instanceTypeFilter:
    minResources:
      cpu: 4
      memory: 16Gi
    maxResources:
      cpu: 8
      memory: 32Gi
    memoryPerCPU:
      min: 7Gi
      max: 9Gi
    nameIncludeExpressions:
      - ^r5
      - ^r6
    nameExcludeExpressions:
      - ^t2
      - ^t3
  
  # Provisioned nodes will have these taints
  # Taints may prevent pods from scheduling if they are not tolerated by the pod.
  taints:
    - key: example.com/special-taint
      effect: NoSchedule
      
      
  # Provisioned nodes will have these taints, but pods do not need to tolerate these taints to be provisioned by this 
  # provisioner. These taints are expected to be temporary and some other entity (e.g. a DaemonSet) is responsible for
  # removing the taint after it has finished initializing the node.
  startupTaints:
    - key: example.com/another-taint
      effect: NoSchedule

  # Labels are arbitrary key-values that are applied to all nodes
  labels:
    billing-team: my-team

  # Requirements that constrain the parameters of provisioned nodes.
  # These requirements are combined with pod.spec.affinity.nodeAffinity rules.
  # Operators { In, NotIn } are supported to enable including or excluding values
  requirements:
    - key: "node.kubernetes.io/instance-type"
      operator: In
      values: ["m5.large", "m5.2xlarge"]
    - key: "topology.kubernetes.io/zone"
      operator: In
      values: ["us-west-2a", "us-west-2b"]
    - key: "kubernetes.io/arch"
      operator: In
      values: ["arm64", "amd64"]
    - key: "karpenter.sh/capacity-type" # If not included, the webhook for the AWS cloud provider will default to on-demand
      operator: In
      values: ["spot", "on-demand"]

  # Karpenter provides the ability to specify a few additional Kubelet args.
  # These are all optional and provide support for additional customization and use cases.
  kubeletConfiguration:
    clusterDNS: ["10.0.1.100"]

  # Resource limits constrain the total size of the cluster.
  # Limits prevent Karpenter from creating new instances once the limit is exceeded.
  limits:
    resources:
      cpu: "1000"
      memory: 1000Gi

  # These fields vary per cloud provider, see your cloud provider specific documentation
  provider: {}
```

## Node deprovisioning 

If neither of these values are set, Karpenter will *not* delete instances. It is recommended to set the `ttlSecondsAfterEmpty` value, to enable scale down of the cluster. 

### spec.ttlSecondsAfterEmpty

Setting a value here enables Karpenter to delete empty/unnecessary instances. DaemonSets are excluded from considering a node "empty". This value is in seconds. 

### spec.ttlSecondsUntilExpired

Setting a value here enables node expiry. After nodes reach the defined age in seconds, they will be deleted, even if in use. This enables nodes to effectively be periodically "upgraded" by replacing them with newly provisioned instances.

Note that Karpenter does not automatically add jitter to this value. If multiple instances are created in a small amount of time, they will expire at very similar times. Consider defining a [pod disruption budget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) to prevent excessive workload disruption. 

### spec.instanceTypeFilter

The `instanceTypeFilter` is optional, as are all of its parameters. It allows for filtering instance types on a provisioner specific basis. This allows you to exclude instance types which while they may fit your workloads are undesirable in some way such as :
- excluding instance types with low CPU and memory to prevent launching nodes that can only support daemonsets and small workloads
- selecting for nodes with the desired memory to CPU ratios based on your workloads
- excluding instance types with high CPU and memory to reduce node blast radius

Karpenter is able to work best when it has a variety of instance types to select from particularly with spot provisioning. Instead of listing a few instance types as a requirement or using the `nameIncludeExpressions`, we recommend that you:
- use `minResources`/`maxResources` to select for a variety of instance types of the desired sizes that match your use cases with respect to DaemonSet overhead, node blast radius, etc. 
- use `nameExcludeExpressions` to exclude any instance types that you specifically do not want

This provides flexibility to Karpenter with respect to the possible node sizes for better node utilization as well as instance type families and generations which can improve the experience using spot.  

### spec.instanceTypeFilter.minResources

The `minResources` parameter filters for instance types with at least the specified resources.  The resources can be any reported by the cloud provider, including extended resources.  The minimum resources is inclusive, so it filters for instance types with the specified or larger quantities of the given resources. 

```yaml
minResources:
  cpu: 4
  memory: 16Gi
```

### spec.instanceTypeFilter.maxResources

The `maxResources` parameter filters for instance types with no more than the specified resources.  The resources can be any reported by the cloud provider, including extended resources.  The maximum resources is inclusive, so it filters for instance types with the specified or smaller quantities of the given resources.

```yaml
maxResources:
  cpu: 8
  memory: 32Gi
```

### spec.instanceTypeFilter.memoryPerCPU

The `memoryPerCPU` parameter filters for instance types with specific memory to CPU ratios.  Both the `min` and `max` parameters are optional allowing excluding instance types with ratios that are out of range. If both values are supplied, for an instance type to be considered the memory to CPU ratio must lie in the range `[min,max]` inclusive.

```yaml
memoryPerCPU:
  min: 7Gi
  max: 9Gi
```

### spec.instanceTypeFilter.nameIncludeExpressions

The `nameIncludeExpressions` parameter filters for instance type names matching regular expressions. If this parameter is supplied, the only instance types that will be considered are those that match any one of the regular expressions in the `nameIncludeExpressions` list.  If an instance type name matches both the `nameIncludeExpressions` and `nameExcludeExpressions`, it will be excluded.   


```yaml
nameIncludeExpressions:
- ^r5
- ^r6
```

### spec.instanceTypeFilter.nameExcludeExpressions

The `nameExcludeExpressions` parameter filters out instance type names matching regular expressions. If this parameter is supplied, no instance type will be considered if it matches any one of the regular expressions in the `nameExcludeExpressions` list. If an instance type name matches both the `nameIncludeExpressions` and `nameExcludeExpressions`, it will be excluded.


```yaml
nameExcludeExpressions:
- ^t2
- ^t3
```

## spec.requirements

Kubernetes defines the following [Well-Known Labels](https://kubernetes.io/docs/reference/labels-annotations-taints/), and cloud providers (e.g., AWS) implement them. They are defined at the "spec.requirements" section of the Provisioner API. 

These well known labels may be specified at the provisioner level, or in a workload definition (e.g., nodeSelector on a pod.spec). Nodes are chosen using both the provisioner's and pod's requirements. If there is no overlap, nodes will not be launched. In other words, a pod's requirements must be within the provisioner's requirements. If a requirement is not defined for a well known label, any value available to the cloud provider may be chosen.

For example, an instance type may be specified using a nodeSelector in a pod spec. If the instance type requested is not included in the provisioner list and the provisioner has instance type requirements, Karpenter will not create a node or schedule the pod. 

📝 None of these values are required.

### Instance Types

- key: `node.kubernetes.io/instance-type`

Generally, instance types should be a list and not a single value. Leaving this field undefined is recommended, as it maximizes choices for efficiently placing pods. 

☁️ **AWS**

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

☁️ **AWS**

- value list: `aws ec2 describe-availability-zones --region <region-name>`

Karpenter can be configured to create nodes in a particular zone. Note that the Availability Zone `us-east-1a` for your AWS account might not have the same location as `us-east-1a` for another AWS account.

[Learn more about Availability Zone
IDs.](https://docs.aws.amazon.com/ram/latest/userguide/working-with-az-ids.html)

### Architecture

- key: `kubernetes.io/arch`
- values
  - `amd64` (default)
  - `arm64`

Karpenter supports `amd64` nodes, and `arm64` nodes.


### Capacity Type

- key: `karpenter.sh/capacity-type`

☁️ **AWS**

- values
  - `spot` 
  - `on-demand` (default)

Karpenter supports specifying capacity type, which is analogous to [EC2 purchase options](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-purchasing-options.html).

Karpenter prioritizes Spot offerings if the provisioner allows Spot and on-demand instances. If the provider API (e.g. EC2 Fleet's API) indicates Spot capacity is unavailable, Karpenter caches that result across all attempts to provision EC2 capacity for that instance type and zone for the next 45 seconds. If there are no other possible offerings available for Spot, Karpenter will attempt to provision on-demand instances, generally within milliseconds. 

Karpenter also allows `karpenter.sh/capacity-type` to be used as a topology key for enforcing topology-spread.

## spec.kubeletConfiguration

Karpenter provides the ability to specify a few additional Kubelet args. These are all optional and provide support for
additional customization and use cases. Adjust these only if you know you need to do so.

```yaml
spec:
  kubeletConfiguration:
    clusterDNS: ["10.0.1.100"]
```

## spec.limits.resources 

The provisioner spec includes a limits section (`spec.limits.resources`), which constrains the maximum amount of resources that the provisioner will manage. 

Presently, Karpenter supports `memory` and `cpu` limits. 

CPU limits are described with a `DecimalSI` value. Note that the Kubernetes API will coerce this into a string, so we recommend against using integers to avoid GitOps skew.

Memory limits are described with a [`BinarySI` value, such as 1000Gi.](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#meaning-of-memory)

Karpenter stops allocating resources once at least one resource limit is met/exceeded.

Review the [resource limit task](../tasks/set-resource-limits) for more information.

## spec.provider

This section is cloud provider specific. Reference the appropriate documentation:

- [AWS](../aws/provisioning/)
