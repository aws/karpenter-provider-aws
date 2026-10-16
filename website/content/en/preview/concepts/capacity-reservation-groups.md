---
title: "Capacity Reservation Groups"
linkTitle: "Capacity Reservation Groups"
weight: 45
description: >
  Target an AWS capacity reservation group with On-Demand fallback and observe mutable reservation coverage.
---

Capacity reservation groups let AWS associate ordinary On-Demand instances with eligible On-Demand Capacity Reservations (ODCRs).
Set `spec.capacityReservationGroupARN` on an EC2NodeClass and enable the existing `ReservedCapacity` feature gate.
This field is mutually exclusive with `capacityReservationSelectorTerms`.

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: reservation-group
spec:
  role: KarpenterNodeRole-REPLACE_WITH_CLUSTER_NAME
  amiSelectorTerms:
    - alias: al2023@REPLACE_WITH_PINNED_VERSION
  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: REPLACE_WITH_CLUSTER_NAME
  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: REPLACE_WITH_CLUSTER_NAME
  capacityReservationGroupARN: arn:aws:resource-groups:us-west-2:123456789012:group/workload-a
---
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: reservation-group
spec:
  template:
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: reservation-group
      requirements:
        - key: karpenter.sh/capacity-type
          operator: In
          values: [on-demand, reserved]
  disruption:
    consolidationPolicy: WhenEmptyOrUnderutilized
    consolidateAfter: 1m
```

The group must be in the controller's AWS region. Create and manage its membership outside Karpenter.
Alpha acceptance requires ordinary ODCR groups. Before enabling this mode in production, verify Fleet fallback and that mixed groups cannot consume unsupported Capacity Blocks or interruptible reservations in your test account.

## Workload constraints and changing labels

A group-targeted instance launches through the On-Demand market and may run covered or uncovered throughout its lifetime.
Its NodePool and workloads must permit both `on-demand` and `reserved` capacity types.
An omitted capacity-type requirement is allowed if the other hard constraints also permit both states.
Reserved-only or On-Demand-only selectors, affinity, and topology constraints cannot use these mutable offerings.
Do not constrain reservation ID, reservation type, or reservation interruptibility for these launches.
Spot may be included in the NodePool; Spot launches do not target the group.

The `karpenter.sh/capacity-type` and `karpenter.k8s.aws/capacity-reservation-*` labels describe the last observed association and can change without replacing the instance.
They cannot guarantee that a running workload continuously receives reservation coverage.
The persisted `karpenter.k8s.aws/capacity-reservation-group-arn` annotation records launch intent independently of current coverage.

## Pricing and disruption

New group-targeted capacity is priced at On-Demand rates for scheduling and replacement decisions because coverage is not guaranteed.
A freshly observed covered instance receives Karpenter's existing near-zero reserved marginal-cost treatment, preserving relative instance-type ordering.
This models incremental scheduling cost; AWS still bills the reservation.
Matching NodeOverlay price rules apply to observed running-node prices using the current coverage labels.
These prices never advertise new reserved launch slots.

Karpenter reconciles association labels approximately once per minute and reads EC2 again for consolidation decisions.
It compares observations across the affected NodePools and other consumers sharing a group before launch and before deleting the source nodes.
Changes in coverage, reservation ID, effective price, or group consumers invalidate the original decision.
Missing, incomplete, stale, or failed observations defer affected cost-based consolidation. Cached zonal-shift data cannot authorize disruption.
Unrelated groups and health-driven replacement remain independent of this pricing check.
AWS can reassign coverage after any read, so these checks provide bounded observations rather than an atomic reservation guarantee.

Deleting a group-backed node never grants a guaranteed reservation slot to its replacement, including a replacement in another NodePool.
Replacement overlap may be billed at On-Demand rates, and other consumers can acquire released coverage.

## Permissions and rollout

Retain the provider's existing `ec2:CreateFleet`, launch-template, `ec2:RunInstances`, `ec2:DescribeInstances`, and instance lifecycle permissions, including applicable IAM pass-role and tagging permissions.
The current implementation does not create or modify Resource Groups and does not call Resource Groups membership APIs.
Validate launch authorization for your group and any shared reservation owner in the target account; successful `DescribeInstances` alone does not prove Fleet launch permission.

Build the provider with the matching core change. The checked-in `go.mod` dependency must be updated to a published core revision containing both scheduling transitions and the observation contract before distributing a standalone provider build.
A local `go.work` is suitable for paired development only.
Run the [acceptance procedure](https://github.com/aws/karpenter-provider-aws/blob/main/test/manual/capacity-reservation-groups.md) before rollout.
Start with a disposable NodePool and ordinary ODCR group. Monitor controller errors from `RefreshNodeClaimStates`, cost tracking, and capacity-type reconciliation.

## Migration and rollback

Adding, changing, or removing the group target changes launch intent and causes drift for existing non-Spot nodes.
Changing current reservation association alone does not cause drift.
Removing the field changes future launches; it does not retarget already running EC2 instances.

Before downgrading to a controller without group support, migrate workloads to a NodePool without a group target and replace all group-targeted instances while the compatible controller still runs.
Verify that no live group-targeted NodeClaims remain, then remove the group-only API configuration and downgrade.
Keep the existing `ReservedCapacity` gate enabled until this migration finishes.
