# Bandwidth Weighting Support

## Overview

EC2 [bandwidth weighting](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configure-bandwidth-weighting.html)
lets an instance shift its baseline bandwidth between networking and EBS. `vpc-1` raises the
networking baseline and lowers EBS; `ebs-1` does the reverse; `default` leaves it unchanged. The
combined bandwidth doesn't change; it's a redistribution, and it costs nothing.

For a workload that uses local NVMe and touches EBS lightly, `vpc-1` is free network headroom.
The motivating case is Spark on EMR-on-EKS using instance-store shuffle on R8gd: no EBS
dependency, but S3 reads plus shuffle traffic saturate the default networking baseline. On
`r8gd.48xlarge`, `vpc-1` moves the split from 50 Gbps network / 40 Gbps EBS to roughly
62.5 / 27.5.

There's no way to get this through Karpenter today. `EC2NodeClass` has no
`networkPerformanceOptions` field, custom launch templates were removed in v0.33+, and the
weighting can only be set at launch — `ModifyInstanceNetworkPerformanceOptions` requires a
`Stopped` instance, so userData can't reach it either.

This design adds the field to `EC2NodeClass` and, when it's set, restricts Karpenter to
instance types that support the requested value.

## Goals

- Expose `networkPerformanceOptions.bandwidthWeighting` on `EC2NodeClass.spec`.
- Pass `NetworkPerformanceOptions` through to the EC2 launch template.
- Only let Karpenter choose instance types whose `NetworkInfo.BandwidthWeightings` includes the
  requested value, so a NodeClaim is never created for a type that would reject it.

## Non-Goals

**Pod-level bandwidth weighting requests.** Bandwidth weighting is a property of the node, not a
per-pod request. Workloads that care should express bandwidth needs through the existing
`karpenter.k8s.aws/instance-network-bandwidth` label.

**Mixed capable/incapable instance types under one weighted NodeClass.** This design filters
instead; mixed fleets are expressed with a second NodePool (see Mixed Fleets).

**In-place changes.** EC2 cannot modify the weighting on a running instance, so Karpenter never
attempts it. A `bandwidthWeighting` change replaces nodes.

## API

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: high-network
spec:
  networkPerformanceOptions:
    bandwidthWeighting: vpc-1   # default | vpc-1 | ebs-1
```

```go
type NetworkPerformanceOptions struct {
    // BandwidthWeighting shifts baseline bandwidth between networking and EBS.
    // vpc-1 favors networking, ebs-1 favors EBS.
    // +kubebuilder:validation:Enum:={"default","vpc-1","ebs-1"}
    // +optional
    BandwidthWeighting *string `json:"bandwidthWeighting,omitempty"`
}
```

The pointer matters: NodeClasses that don't set the field must not write an empty
`NetworkPerformanceOptions` block into every launch template.

Unset and `bandwidthWeighting: default` are not the same thing. Unset omits
`NetworkPerformanceOptions` from the launch template entirely and applies no instance type
filtering. `default` sends the value explicitly, so the field is set and filtering applies: the
NodeClass is pinned to capable types at the default split. Operators who want today's behavior
should leave it unset.

The CRD enum is the only validation; nothing re-checks the value at runtime.

## Instance Type Compatibility

`DescribeInstanceTypes` reports `NetworkInfo.BandwidthWeightings` per instance type: the list of
values that type accepts. That list is the only source of truth. There is no
static list of supported families: it would go stale the moment AWS adds one (9th-gen families
already support this) and would invite readers to trust the doc over the API.

When `bandwidthWeighting` is set, the compatibility check in
`pkg/providers/instancetype/compatibility` drops any instance type whose
`BandwidthWeightings` doesn't contain the requested value. This follows the existing
`CompatibleCheck` pattern in that package, alongside the network-interface and
nested-virtualization checks:

```go
func (c bandwidthWeightingCheck) compatibleCheck(info ec2types.InstanceTypeInfo) bool {
    if c.npo == nil || c.npo.BandwidthWeighting == nil {
        return true
    }
    if info.NetworkInfo == nil {
        return false
    }
    for _, w := range info.NetworkInfo.BandwidthWeightings {
        if string(w) == *c.npo.BandwidthWeighting {
            return true
        }
    }
    return false
}
```

Filtering has to happen here, during instance type resolution, and not in the launch-path
filter in `pkg/providers/instance/filter`. The compatibility check runs before the scheduler
picks a type, so incompatible types are never offered and no NodeClaim is created for one. The
launch-path filter runs after the scheduler has committed: it would reject the launch, fail the
NodeClaim, and the scheduler would create another, repeating until the compatible types were
exhausted. Operators would see that as Karpenter churning NodeClaims.

Filtering also removes any dependence on how EC2 treats an unsupported request. Karpenter never
sends `NetworkPerformanceOptions` to a type that doesn't accept it, so whether EC2 ignores the
parameter or rejects it with `InvalidParameterValue` stops mattering.


## Mixed Fleets

Filtering narrows the instance types a weighted NodeClass can use, which matters for anyone
running mixed generations for capacity flexibility. That case is expressed with two NodePools
pointing at two NodeClasses — one weighted, one not — and a weight preferring the weighted pool:

```yaml
# Preferred: capable types, vpc-1 applied.
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: spark-weighted
spec:
  weight: 100
  template:
    spec:
      nodeClassRef: { group: karpenter.k8s.aws, kind: EC2NodeClass, name: spark-vpc1 }
---
# Fallback: any type, no weighting.
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: spark-fallback
spec:
  weight: 10
  template:
    spec:
      nodeClassRef: { group: karpenter.k8s.aws, kind: EC2NodeClass, name: spark-default }
```

A single workload spans both. Pods don't need to select a NodePool: Karpenter attempts the
higher-weight NodePool first and falls back to the other when it can't satisfy the pod, so a
Spark job's executors can land on nodes from either. This is the same `.spec.weight` mechanism
the docs describe for preferring reserved capacity before on-demand.

## Labels

Nodes launched with a weighting get:

```
karpenter.k8s.aws/instance-bandwidth-weighting: vpc-1
```

Filtering means every node under a weighted NodeClass has the requested value, so the label is
fixed per NodeClass and known before launch. Pods can use it as a scheduling requirement or
preference across the two-NodePool setup, not only for post-launch monitoring.

## Drift

A `bandwidthWeighting` change is picked up by the existing static drift path: it's part of
`EC2NodeClass.Spec`, so it changes `EC2NodeClass.Hash()` and `areStaticFieldsDrifted()` reports
`NodeClassDrift`, and nodes are replaced.

## Upgrade Impact

Adding the field to the launch template struct changes the computed launch template name, so
existing NodeClasses that never set it will resolve to new launch templates on upgrade.

Whether that also replaces nodes depends on whether the added nil field changes
`EC2NodeClass.Hash()`. The implementation must assert the pre- and post-change hashes in a test.
If it does churn, ship it with an `AnnotationEC2NodeClassHashVersion` bump so the migration is
one-time and explicit instead of a surprise rolling replacement.

No new IAM permissions are needed; `NetworkPerformanceOptions` is a parameter on
`CreateLaunchTemplate`, which Karpenter already calls.

---

*AI was used to review and help shape and enrich this document. The work and all technical
decisions are authored and reviewed by a human.*
