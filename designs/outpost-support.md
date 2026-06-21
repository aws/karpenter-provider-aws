# AWS Outposts Support

This document proposes adding AWS Outposts awareness to Karpenter, enabling node provisioning on Outpost hardware.

## Overview

### AWS Outposts

[AWS Outposts](https://docs.aws.amazon.com/outposts/latest/userguide/what-is-outposts.html) extends AWS infrastructure to customer premises. An Outpost is a pool of compute and storage deployed at a customer site, fully managed by AWS.

EKS supports [two deployment topologies on Outposts](https://docs.aws.amazon.com/eks/latest/userguide/eks-outposts.html): extended clusters (control plane in-region, workers on the Outpost) and local clusters (everything on the Outpost). This design applies to both since Karpenter only cares about node placement.

### Outpost Constraints

The hardware generation (gen1 or gen2) determines EBS volume type support. Generation is a physical property of the rack, not exposed via any API.

| Constraint | Regional AWS | Outpost |
|-----------|-------------|---------|
| Instance types | All types available | Only types physically installed on the rack |
| Capacity type | On-demand and spot | On-demand only (no spot) |
| EBS volumes (gen1) | gp2, gp3, io1, io2, etc. | gp2 only |
| EBS volumes (gen2) | gp2, gp3, io1, io2, etc. | gp2, gp3 |
| Subnets | Any subnet in the VPC | Only subnets associated with the Outpost |
| Capacity | Effectively unlimited | Finite, determined by installed hardware |

Without Outpost awareness, Karpenter would try to use regional subnets, unavailable instance types, spot capacity, and unsupported EBS volume types. All of these cause launch failures.

## Goals

1. Target a specific Outpost via a single optional field on EC2NodeClass.
2. Filter subnets to only those on the specified Outpost.
3. Discover available instance types via the EC2 API rather than hardcoding.
4. Restrict to on-demand capacity (no spot on Outposts).
5. Document that users must configure `blockDeviceMappings` for their Outpost generation.
6. Zero behavioral change when `outpostArn` is unset.
7. Minimal change. Reuse existing signals (ICE errors, caching, reconciliation loops) rather than introducing Outpost-specific logic.

## Non-Goals

1. Multi-Outpost support from a single EC2NodeClass. Use separate EC2NodeClass/NodePool pairs.
2. Outpost generation detection. No API exposes this; users configure `blockDeviceMappings` themselves.
3. Capacity tracking. Outposts expose capacity metrics, but integrating them adds complexity for little benefit. ICE errors are sufficient, same as regional. See [odcr.md](odcr.md) for how Karpenter handles finite-capacity offerings with explicit tracking when the use case demands it.
4. Local Gateway routing. Outside Karpenter's scope.
5. S3 on Outposts or non-EBS storage.

## Proposed API Changes

### EC2NodeClass Spec

Add an optional `outpostArn` field:

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: outpost
spec:
  outpostArn: arn:aws:outposts:us-west-2:123456789012:outpost/op-1234567890abcdef0
  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}"
  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}"
  role: "KarpenterNodeRole-${CLUSTER_NAME}"
```

When set, Karpenter adjusts subnet selection, instance type discovery, and capacity type as described below.

### EC2NodeClass Status

Add `outpostArn` to `status.subnets` entries (omitted when empty, so no change for non-Outpost users):

```yaml
status:
  subnets:
    - id: subnet-0a462d98193ff9fac
      zone: us-west-2b
      outpostArn: arn:aws:outposts:us-west-2:123456789012:outpost/op-1234567890abcdef0
```

### Validation

Enforce the Outpost ARN format via CEL, covering all partitions (commercial, GovCloud, China):

```
^arn:aws[a-zA-Z-]*:outposts:[a-z0-9-]+:[0-9]{12}:outpost/op-[0-9a-f]{17}$
```

## Proposed Behavior

All changes activate only when `outpostArn` is set. Unset means unchanged behavior.

### Subnet Filtering

Filter resolved subnets to only those matching the configured Outpost ARN. The [DescribeSubnets](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Subnet.html) response includes an `OutpostArn` field on each subnet.

If nothing matches, surface a `SubnetsNotFound` condition so the user knows their selector terms don't match any Outpost subnets.

### Instance Type Discovery

Query [DescribeInstanceTypeOfferings](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstanceTypeOfferings.html) with `LocationType=outpost` and the Outpost ARN as location filter. Cache results separately from regional offerings, keyed by Outpost ARN (suggested TTL: 5 minutes). Only instance types returned by this query are eligible for scheduling.

No new IAM permissions needed. The existing `ec2:DescribeInstanceTypeOfferings` permission covers Outpost queries. If an SCP or policy condition restricts the location type, the query returns empty and Karpenter logs a warning.

### Launch Path

The Outpost constraint does not change CreateFleet parameters. Subnet filtering already happens before launch: the subnet overrides passed to CreateFleet will only include Outpost subnets. Instance type filtering restricts the fleet overrides to types available on the rack. No changes to `DefaultTargetCapacityType` or other fleet-level settings are needed.

### Capacity Type Restriction

Exclude spot offerings when `outpostArn` is set. Spot is not available on Outposts.

### EBS Volume Configuration

Outpost EBS support varies by generation (gen1: gp2 only, gen2: gp2 and gp3). Since there's no API to detect generation, we don't try to be clever about it. Users targeting an Outpost must set `spec.blockDeviceMappings` with a valid volume type:

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: outpost-gen1
spec:
  outpostArn: arn:aws:outposts:us-west-2:123456789012:outpost/op-1234567890abcdef0
  blockDeviceMappings:
    - deviceName: /dev/xvda
      ebs:
        volumeType: gp2
        volumeSize: 20Gi
        encrypted: true
  # ...
```

If omitted, Karpenter's default (gp3) applies. On gen1 this causes a launch failure from EC2, which is a clear signal to fix the configuration.

## Operational Considerations

Outposts have finite capacity. ICE errors are more common than in-region but handled identically: the standard ICE cache prevents retry storms, and pods stay `Pending` until capacity frees up. Since all Outpost subnets are in a single zone, an ICE error for a given instance type effectively blocks it until cache expiry. ICE cache entries are scoped by instance type and zone as usual; no Outpost-specific cache key is needed.

Consolidation's launch-before-terminate ordering means it naturally blocks when the Outpost is full. The existing node keeps running; the cluster just stays in a suboptimal state until capacity is available. Similar to [placement group constraints](placement-groups-support.md), no special handling is needed.

When the service link between Outpost and region degrades, API calls from Karpenter will fail. Cached instance type offerings continue serving during brief outages. Node launches fail and retry on subsequent reconciliation loops. Existing nodes keep running regardless of service link state ([by design](https://docs.aws.amazon.com/outposts/latest/userguide/disaster-recovery-resiliency.html)).

Observability uses existing signals: `SubnetsNotFound` / `NodeClassReady=False` conditions for misconfiguration, standard ICE events for capacity exhaustion, and existing Karpenter metrics for launch tracking.

## Drift

Changing `outpostArn` on an existing EC2NodeClass triggers drift on nodes provisioned under the old configuration:

- Added: regional nodes become drifted, replaced with Outpost nodes.
- Removed: Outpost nodes become drifted, replaced with regional nodes.
- Changed to a different Outpost: nodes on the old Outpost become drifted.

Standard drift reconciliation handles all cases. No Outpost-specific drift logic needed.

## End-to-End Example

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: outpost
spec:
  outpostArn: arn:aws:outposts:us-west-2:123456789012:outpost/op-1234567890abcdef0
  blockDeviceMappings:
    - deviceName: /dev/xvda
      ebs:
        volumeType: gp2
        volumeSize: 20Gi
        encrypted: true
  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: my-cluster
  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: my-cluster
  amiSelectorTerms:
    - alias: al2023@latest
  role: "KarpenterNodeRole-my-cluster"
---
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: outpost
spec:
  template:
    metadata:
      labels:
        node.kubernetes.io/outpost: "true"
    spec:
      requirements:
        - key: karpenter.sh/capacity-type
          operator: In
          values: ["on-demand"]
        - key: node.kubernetes.io/instance-type
          operator: In
          values: ["m5.xlarge", "m5.2xlarge", "c5.xlarge", "c5.2xlarge", "r5.xlarge", "r5.2xlarge"]
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: outpost
  disruption:
    consolidationPolicy: WhenEmpty
    consolidateAfter: 5m
```

Standard Karpenter scheduling applies. The only difference is that subnet and instance type resolution uses Outpost-scoped results, and spot offerings are excluded. If the Outpost is full, pods stay `Pending` until capacity frees up.

## Alternatives Considered

### Separate NodeClass CRD for Outposts

We considered a dedicated `OutpostNodeClass` but the behavioral delta is too small to justify a new resource type. If Outpost-specific behavior grows significantly (capacity-aware scheduling, local storage), we could revisit.

### Outpost as a Scheduling Constraint

Modeling this as a NodePool requirement (`node.kubernetes.io/outpost: "true"`) rather than an EC2NodeClass field doesn't work well. The Outpost ARN drives infrastructure decisions (subnet filtering, instance type discovery) that belong on the node class, not the scheduling topology.

### Auto-detection from Subnet Tags

Inferring Outpost targeting from subnet metadata is tempting but ambiguous. A user might have both regional and Outpost subnets matching their selector. Explicit `outpostArn` makes intent clear and error messages actionable.

## References

- [AWS Outposts User Guide](https://docs.aws.amazon.com/outposts/latest/userguide/what-is-outposts.html)
- [Amazon EKS on AWS Outposts](https://docs.aws.amazon.com/eks/latest/userguide/eks-outposts.html)
- [EBS volume types on Outposts](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volume-types.html#outposts-ebs-volume-types)
- [DescribeInstanceTypeOfferings API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstanceTypeOfferings.html)
- [DescribeSubnets API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Subnet.html)
- [Outpost resilience and service link](https://docs.aws.amazon.com/outposts/latest/userguide/disaster-recovery-resiliency.html)
- [Karpenter issue #1284: Support Outposts Instance Type Offerings](https://github.com/aws/karpenter-provider-aws/issues/1284)
