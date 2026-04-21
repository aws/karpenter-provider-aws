# Bandwidth Weighting Support

This document proposes supporting EC2 bandwidth weighting configuration in Karpenter.

- [Bandwidth Weighting Support](#bandwidth-weighting-support)
  - [Overview](#overview)
  - [Customer Use Cases](#customer-use-cases)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
  - [EC2NodeClass API](#ec2nodeclass-api)
  - [Instance Type Discovery](#instance-type-discovery)
  - [Scheduling and Launch Behavior](#scheduling-and-launch-behavior)
  - [Labels](#labels)
  - [Drift](#drift)
  - [Appendix](#appendix)

## Overview

EC2 [bandwidth weighting](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configure-bandwidth-weighting.html) allows 8th-gen instance types (M8, C8, R8, X8 families) to shift baseline bandwidth between networking and EBS. Three options are available:

- **default** — standard bandwidth configuration for the instance type
- **vpc-1** — increases networking baseline bandwidth, decreases EBS baseline bandwidth
- **ebs-1** — increases EBS baseline bandwidth, decreases networking baseline bandwidth

The combined bandwidth between networking and EBS does not change — bandwidth weighting redistributes the existing allocation. For example, an R8gd.48xlarge with `default` has 50 Gbps networking / 40 Gbps EBS. With `vpc-1`, this shifts to ~62.5 Gbps networking / ~27.5 Gbps EBS.

Bandwidth weighting can only be set at launch time via the `NetworkPerformanceOptions` parameter in the launch template, or modified on a **stopped** instance. It cannot be modified on a running instance. There is no additional cost.

### Why Karpenter Needs This

Currently there is no way to configure bandwidth weighting in Karpenter:

- EC2NodeClass does not expose `networkPerformanceOptions`
- Custom launch templates were removed in v0.33+
- The `modify-instance-network-performance-options` API requires the instance to be in `Stopped` state, so userData-based modification is not possible (validated: returns `InvalidState: not in an allowed state: stopped`)

## Customer Use Cases

### Spark on EKS with NVMe Shuffle (vpc-1)

EMR on EKS workloads running Spark with local NVMe for shuffle data use R8gd instances. These workloads have no EBS dependency but are network-bandwidth-constrained — S3 reads/writes plus shuffle traffic compete for the 50 Gbps default networking baseline. With `vpc-1`, the networking baseline increases to ~62.5 Gbps, providing ~25% more headroom for S3 and shuffle traffic at no cost.

The customer runs a heterogeneous fleet (R8gd.48xl + R6gd.16xl) in the same NodePool for capacity flexibility. R6gd does not support bandwidth weighting. The solution must apply `vpc-1` to R8gd nodes while gracefully handling R6gd nodes that don't support it.

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: spark-workers
spec:
  networkPerformanceOptions:
    bandwidthWeighting: vpc-1
  instanceStorePolicy: RAID0
  amiSelectorTerms:
    - alias: al2023@latest
  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: "my-cluster"
  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: "my-cluster"
  role: "KarpenterNodeRole-my-cluster"
---
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: spark-workers
spec:
  template:
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: spark-workers
      requirements:
        - key: node.kubernetes.io/instance-type
          operator: In
          values: ["r8gd.48xlarge", "r8gd.24xlarge", "r6gd.16xlarge", "r6gd.12xlarge"]
        - key: karpenter.sh/capacity-type
          operator: In
          values: ["on-demand"]
```

In this configuration:
- R8gd nodes launch with `vpc-1` — increased networking bandwidth
- R6gd nodes launch without `NetworkPerformanceOptions` — default bandwidth (parameter silently ignored for unsupported types)

### EBS-Heavy Analytics (ebs-1)

Analytics workloads using large EBS volumes (io2, gp3) for data processing could benefit from `ebs-1` to maximize EBS throughput at the expense of networking bandwidth.

## Goals

1. Allow users to configure bandwidth weighting (`vpc-1`, `ebs-1`, `default`) via EC2NodeClass
2. Conditionally apply `NetworkPerformanceOptions` in the launch template only for instance types that support it
3. Support mixed fleets where some instance types support bandwidth weighting and others don't
4. Label nodes with their bandwidth weighting configuration for scheduling visibility

## Non-Goals

1. **Pod-level bandwidth weighting requests** — bandwidth weighting is an infrastructure decision, not a workload decision. Pods should not request specific bandwidth weighting; they should express bandwidth requirements via existing mechanisms (e.g., `karpenter.k8s.aws/instance-network-bandwidth`).
2. **Filtering instance types based on bandwidth weighting support** — users run mixed fleets for capacity flexibility. Karpenter should not exclude unsupported instance types when `networkPerformanceOptions` is set.
3. **Dynamic bandwidth adjustment** — modifying bandwidth weighting on running instances is not supported by EC2.

## EC2NodeClass API

Add `networkPerformanceOptions` to `EC2NodeClassSpec`:

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: example
spec:
  networkPerformanceOptions:
    bandwidthWeighting: vpc-1  # "default", "vpc-1", or "ebs-1"
```

```go
type EC2NodeClassSpec struct {
    // ...existing fields...

    // NetworkPerformanceOptions configure the network performance options for
    // instances launched with this EC2NodeClass. Allows configuring bandwidth
    // weighting between networking and EBS for supported instance types.
    // When set, Karpenter conditionally includes NetworkPerformanceOptions in
    // the launch template for instance types that support it.
    // +optional
    NetworkPerformanceOptions *NetworkPerformanceOptions `json:"networkPerformanceOptions,omitempty"`
}

type NetworkPerformanceOptions struct {
    // BandwidthWeighting configures bandwidth weighting for the instance.
    // vpc-1 increases networking baseline bandwidth and decreases EBS baseline bandwidth.
    // ebs-1 increases EBS baseline bandwidth and decreases networking baseline bandwidth.
    // +kubebuilder:validation:Enum:={"default","vpc-1","ebs-1"}
    // +optional
    BandwidthWeighting *string `json:"bandwidthWeighting,omitempty"`
}
```

## Instance Type Discovery

Karpenter needs to know which instance types support bandwidth weighting to conditionally include `NetworkPerformanceOptions` in the launch template.

### Option A: DescribeInstanceTypes (Preferred)

The `DescribeInstanceTypes` API returns `NetworkInfo` per instance type. If the API exposes bandwidth weighting capability (e.g., `NetworkInfo.BandwidthWeightingSupport`), Karpenter can dynamically discover support.

**Implementation:** Add a `BandwidthWeightingSupported` field to Karpenter's instance type model, populated during the `DescribeInstanceTypes` call in the instance type provider.

### Option B: Static Instance Family List

If the API does not expose bandwidth weighting capability, maintain a static list of supported instance families (M8, C8, R8, X8) and check the instance type family prefix.

**Implementation:** Parse the instance type name (e.g., `r8gd.48xlarge` → family `r8gd` → generation `8` → supported).

### Recommendation

Option A is preferred for accuracy and forward-compatibility. Option B is a viable fallback if the API doesn't expose the capability.

## Scheduling and Launch Behavior

### Launch Template Generation

When `networkPerformanceOptions` is set on the EC2NodeClass:

1. For each instance type in the launch request, check if it supports bandwidth weighting
2. If supported, include `NetworkPerformanceOptions` in the `RequestLaunchTemplateData`
3. If not supported, omit `NetworkPerformanceOptions` from the launch template

Since Karpenter generates separate launch templates per unique configuration (hashed by launch template parameters), instance types with and without bandwidth weighting support will naturally get different launch templates.

```go
func networkPerformanceOptions(
    npo *v1.NetworkPerformanceOptions,
    instanceType *cloudprovider.InstanceType,
) *ec2types.LaunchTemplateNetworkPerformanceOptionsRequest {
    if npo == nil || npo.BandwidthWeighting == nil {
        return nil
    }
    if !instanceType.BandwidthWeightingSupported {
        return nil
    }
    return &ec2types.LaunchTemplateNetworkPerformanceOptionsRequest{
        BandwidthWeighting: ec2types.InstanceBandwidthWeighting(*npo.BandwidthWeighting),
    }
}
```

### Launch Template Hashing

The `NetworkPerformanceOptions` field must be included in the launch template name hash so that different bandwidth weighting configurations produce different launch templates. This ensures instance types with bandwidth weighting support get a launch template with `NetworkPerformanceOptions`, while unsupported types get a launch template without it.

### No Instance Type Filtering

Karpenter does **not** filter out instance types that don't support bandwidth weighting. All instance types allowed by the NodePool remain eligible. The bandwidth weighting is applied opportunistically — when the resolved instance type supports it, the launch template includes it; when it doesn't, the launch template omits it.

This preserves fleet flexibility for customers running mixed-generation instance types.

## Labels

When Karpenter launches an instance with bandwidth weighting configured, it applies the following label to the Node/NodeClaim:

| Label | Values | Description |
|-------|--------|-------------|
| `karpenter.k8s.aws/instance-bandwidth-weighting` | `default`, `vpc-1`, `ebs-1` | The bandwidth weighting applied to the instance. Only set when the instance type supports bandwidth weighting and the EC2NodeClass specifies `networkPerformanceOptions`. |

This label enables:

1. **Observability** — operators can see which nodes have bandwidth weighting applied
2. **Pod affinity** — workloads that benefit from boosted networking can express affinity for `vpc-1` nodes
3. **Drift detection** — Karpenter can detect when a node's bandwidth weighting doesn't match the EC2NodeClass

Example:

```yaml
metadata:
  labels:
    karpenter.k8s.aws/instance-bandwidth-weighting: "vpc-1"
```

Pods can optionally express preference for bandwidth-weighted nodes:

```yaml
spec:
  affinity:
    nodeAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
        - weight: 50
          preference:
            matchExpressions:
              - key: karpenter.k8s.aws/instance-bandwidth-weighting
                operator: In
                values: ["vpc-1"]
```

Note: This is a `preferred` affinity, not `required` — the pod can still schedule on nodes without bandwidth weighting (e.g., R6gd) if no `vpc-1` nodes are available.

## Drift

Nodes are marked as drifted when their bandwidth weighting label no longer matches the EC2NodeClass's `networkPerformanceOptions`:

| Scenario | Detection | Recovery |
|----------|-----------|----------|
| `networkPerformanceOptions` added to EC2NodeClass | Existing nodes lack bandwidth weighting label | Nodes drifted, replaced with bandwidth weighting |
| `networkPerformanceOptions` removed | Existing nodes have bandwidth weighting label | Nodes drifted, replaced without bandwidth weighting |
| `bandwidthWeighting` value changed (e.g., `vpc-1` → `ebs-1`) | Label value differs from spec | Nodes drifted, replaced with new weighting |
| Instance type doesn't support bandwidth weighting | No label set, no drift | No action needed |

## Appendix

### Supported Instance Types

Bandwidth weighting is available on 8th-gen instance families:

| Category | Families |
|----------|---------|
| General Purpose | M8a, M8g, M8gd, M8i, M8id, M8i-flex |
| Compute Optimized | C8a, C8g, C8gd, C8i, C8id, C8i-flex |
| Memory Optimized | R8a, R8g, R8gd, R8i, R8id, R8i-flex, X8g, X8aedz, X8i |

### Bandwidth Impact Examples

**R8gd.48xlarge:**

| Config | Network Baseline | EBS Baseline |
|--------|-----------------|-------------|
| default | 50 Gbps | 40 Gbps |
| vpc-1 | ~62.5 Gbps | ~27.5 Gbps |
| ebs-1 | ~37.5 Gbps | ~52.5 Gbps |

### EC2 API Reference

- [Configure bandwidth weighting](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configure-bandwidth-weighting.html)
- [LaunchTemplateNetworkPerformanceOptionsRequest](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_LaunchTemplateNetworkPerformanceOptionsRequest.html)
- [ModifyInstanceNetworkPerformanceOptions](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ModifyInstanceNetworkPerformanceOptions.html) (requires stopped instance)
