# Placement Group Support

## Context

Amazon EC2 placement groups let operators influence instance placement for low-latency (`cluster`), failure-domain isolation (`partition`), and small critical workloads (`spread`). The long-standing request in https://github.com/aws/karpenter-provider-aws/issues/3324 is to make these groups usable from `EC2NodeClass`.

Karpenter already treats `EC2NodeClass` as launch configuration for existing AWS resources such as subnets, security groups, AMIs, and instance profiles. Placement groups fit best when modeled the same way.

## Problem

Users can launch Karpenter-managed nodes into subnets, security groups, and capacity reservations, but cannot direct those nodes into an existing placement group. This blocks workloads that already rely on EC2 placement-group semantics, for example:

- tightly-coupled clusters that need cluster placement-group networking
- replicated systems that want partition placement-group isolation
- small critical workloads that want spread placement-group separation

The previously proposed design in #5389 focused on Karpenter creating placement groups. That adds a new EC2 resource lifecycle to reconcile and exposes strategy-specific creation APIs that users may rely on long term.

## Options

### Option 1: Karpenter creates and owns placement groups

Pros:

- users can describe strategy directly in `EC2NodeClass`
- Karpenter could validate strategy-specific configuration at reconciliation time

Cons:

- introduces new lifecycle ownership for EC2 resources outside the current launch path
- expands the stable API surface with strategy creation details such as `cluster`, `spread`, `partition`, partition count, and spread level
- complicates shared placement groups and future AWS-specific variants
- makes rollback and drift semantics harder because the placement group becomes a controller-managed dependency

### Option 2: Karpenter references an existing placement group

Pros:

- matches how `EC2NodeClass` already models other AWS launch dependencies
- keeps the API small: identify the group and optionally pin a partition
- works for user-managed, shared, and externally tagged placement groups
- avoids inventing a placement-group controller lifecycle before demand is proven

Cons:

- users must provision the placement group out of band
- Karpenter cannot configure placement-group strategy on behalf of the user

## Recommendation

Add an optional `spec.placementGroup` field on `EC2NodeClass`:

```yaml
spec:
  placementGroup:
    name: analytics-partition
    partition: 2
```

Behavior:

- `name` or `id` identifies the existing placement group; the fields are mutually exclusive
- `id` supports shared placement groups, which require `GroupId` during launch
- `partition` is optional and only meaningful for partition placement groups
- Karpenter resolves the configured group into `status.placementGroup`
- launch templates include the placement-group reference so both `CreateFleet` and `RunInstances` honor it

## Key Decisions

- Karpenter does not create, tag, delete, or mutate placement groups in this design
- placement-group strategy remains an operator concern because it belongs to the EC2 placement-group resource, not the instance launch request
- partition selection is the only launch-time knob worth exposing initially because AWS applies it at instance launch and it is useful even when the placement group is created elsewhere

## User Guidance

- Use `name` for placement groups in the same account and `id` for shared placement groups
- Pair cluster placement groups with subnet or topology constraints that keep launches in a single Availability Zone
- Omit `partition` to let EC2 distribute instances across partitions, or set it when the workload needs explicit partition affinity

## Future Work

- richer status surfacing for placement-group strategy and readiness
- strategy-aware validation and scheduling hints
- a separate proposal for Karpenter-managed placement-group lifecycle if real demand justifies the larger API
