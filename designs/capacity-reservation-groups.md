# RFC: Capacity Reservation Resource Group Targeting in Karpenter

**Status:** Draft for discussion  
**Last researched:** 2026-10-09

> This is an unimplemented proposal. API names are illustrative. AWS behavior must be validated through Karpenter's exact EC2 Fleet path.

## Summary

Karpenter currently targets individual Capacity Reservations with available capacity. It cannot launch an instance with a stable Capacity Reservation Resource Group target and let AWS manage association over that instance's lifetime.

Group targeting would be an alternative ODCR consumption model. Instead of Karpenter selecting and accounting for a specific reservation ID at launch, Karpenter would delegate reservation selection and later reassociation to AWS within an operator-defined group. This provides AWS-managed matching without the account-wide scope of open matching.

This RFC proposes an operator-managed group ARN on `EC2NodeClass`. Karpenter would:

- place the group target in the launch template;
- allow AWS's documented On-Demand fallback when the group has no compatible available reservation;
- periodically observe the reservation, if any, currently covering the instance; and
- use that observation for status without treating it as durable capacity.

Karpenter would not create groups, manage group membership, or create and resize reservations.

## Motivation

### Cover an existing running fleet

A group-targeted instance can launch on ordinary On-Demand capacity and later become covered when compatible capacity is added to the group. This allows operators to establish reservation coverage around an already-running fleet without replacing that fleet solely to change reservation association.

For example:

| Stage | Running | Reserved | Unused reserved |
|---|---:|---:|---:|
| Group-targeted instances on ordinary capacity | 100 | 0 | 0 |
| Compatible reservations added and matched | 100 | 100 | 0 |
| Additional compatible capacity added | 100 | 120 | 20 |

The group itself reserves no capacity. Adding only 20 slots while 100 compatible group-targeted instances are uncovered may cause those instances to consume all 20 slots.

### Reacquire coverage after replacement overlap

Suppose ten instances occupy ten reserved slots. A replacement launches before an old instance terminates, while the reservation is full. With a persistent group target, the replacement can launch on On-Demand capacity and later become covered when the old instance releases its slot. It does not need another replacement solely to regain coverage.

Issue [#9518](https://github.com/aws/karpenter-provider-aws/issues/9518) contains independent reports of both problems. The issue requests open matching, not groups; it is evidence for the use cases, not agreement on this proposal.

## Current behavior

### AWS

AWS documents that:

- `CapacityReservationTarget.CapacityReservationResourceGroupArn` targets a Capacity Reservation Resource Group.
- EC2 can launch on ordinary On-Demand capacity when the group has no compatible available reservation.
- A running group-targeted instance can later match compatible capacity added to the group.
- Recipient accounts can add active reservations shared with them to their own groups.

Sources: [group launch](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/cr-groups-launch.html), [group lifecycle](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/cr-groups-lifecycle.html), [group membership](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/cr-groups-add.html), and [sharing](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/capacity-reservation-sharing.html).

AWS also announced group-targeting support for launch templates and EC2 Fleet: [AWS announcement](https://aws.amazon.com/about-aws/whats-new/2020/07/amazon-ec2-on-demand-capacity-reservations-now-support-group-targeting/).

`DescribeInstances` reports both the current `CapacityReservationId` and the configured `CapacityReservationSpecification`, including a group target: [DescribeInstances API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html).

### Karpenter

Current reserved-capacity support resolves `capacityReservationSelectorTerms` to individual reservations. Karpenter advertises available reservation slots as `reserved` offerings and launches with:

- `CapacityReservationPreference: capacity-reservations-only`; and
- a specific `CapacityReservationId`.

Ordinary On-Demand and Spot launches use reservation preference `none`. Selecting an open reservation through `capacityReservationSelectorTerms` therefore does not enable EC2 open matching: Karpenter still targets a specific available reservation ID. Conversely, enabling account-wide open matching would not preserve the NodeClass selector boundary during later AWS-managed association.

Group targeting provides a third model: Karpenter selects the group, while AWS selects and updates the reservation association within that bounded set.

The AWS SDK contains `CapacityReservationResourceGroupArn`, but no Karpenter implementation of group targeting was found during this review.

## Goals

- Provide group targeting as an alternative to Karpenter-managed, reservation-ID-specific ODCR consumption.
- Delegate reservation selection and later reassociation to AWS within an operator-defined group.
- Allow an `EC2NodeClass` to target one operator-managed Capacity Reservation Resource Group.
- Preserve the group target when a launch uses ordinary On-Demand capacity.
- Reflect later reservation association and disassociation accurately.
- Keep ordinary provisioning available when the group is empty or full.
- Support groups containing owned or shared reservations.
- Preserve current explicit reservation-ID behavior.

## Non-goals

- Creating groups or managing membership.
- Creating, sharing, resizing, or retaining reservations.
- Defining launch-order policy across instance types, Availability Zones, or capacity sources.
- Defining quotas, entitlements, or fairness among group consumers.
- Retrofitting a group target onto a running instance configured with `none`. AWS documents modifying this configuration for stopped instances.
- Guaranteeing reservation availability.

## Proposed API

Add an optional group ARN to `EC2NodeClass`:

```yaml
spec:
  capacityReservationGroupARN: arn:aws:resource-groups:us-west-2:123456789012:group/workload-a
```

The field name is illustrative.

Initial contract:

- The ARN references an operator-managed Resource Group.
- Karpenter places the ARN in `CapacityReservationTarget.CapacityReservationResourceGroupArn` for On-Demand launches.
- The launch uses AWS's normal group behavior: use compatible group capacity when available; otherwise use ordinary On-Demand capacity.
- Spot launches are unchanged.
- The field is mutually exclusive with `capacityReservationSelectorTerms`.
- Group membership is authoritative. Existing reservation selectors do not further restrict AWS's later matching.

An explicit ARN is the smallest useful contract and does not require Karpenter to discover or manage groups.

Mutual exclusion makes group targeting a distinct ODCR mode. Existing selectors keep selection, availability, and per-ID accounting in Karpenter; a group deliberately gives reservation selection and lifetime reassociation to AWS.

A future API could represent individual selectors, groups, and open matching as variants of one reservation-target field. That API shape is not required for the initial behavior.

## Launch, observation, and accounting

Karpenter creates a launch template containing the group ARN and submits it through the existing `CreateFleet` path.

Expected outcomes:

- EC2 covers the instance with a compatible group reservation when available.
- Otherwise, EC2 launches the instance as ordinary On-Demand while retaining the group target.
- EC2 may later associate or disassociate the running instance as group capacity changes.

### Association reconciliation

Because association can change while the node remains running, the AWS provider must not observe it only at launch or termination. A provider controller should periodically refresh group-targeted instances with `DescribeInstances` and reconcile current reservation metadata.

The refresh interval should balance API cost against status freshness. Reconciliation should also occur before any provider operation whose correctness depends on current association. `DescribeInstances` is the authoritative source.

### Scheduling and consolidation accounting

Group-targeted launches should remain `karpenter.sh/capacity-type=on-demand` for scheduling purposes. Reserved capacity is not required for launch success, and current association can change without replacing the node.

The initial implementation should therefore:

- use the On-Demand price for provisioning and consolidation;
- not advertise group capacity as `reserved` offerings; and
- not reserve group slots in per-scheduling-run reservation accounting.

This conservative model keeps consolidation correct even if observed association is briefly stale. A free group slot is not necessarily available to a new NodeClaim: AWS may assign it to an already-running group-targeted instance.

A future optimization could consider current reservation association during consolidation. That would require a fresh group-level view before evaluation and revalidation before disruption. Refreshing only the candidate node is insufficient: deleting a covered node may cause AWS to assign its slot to another uncovered group target.

Current individual-ID targeting remains unchanged for launches that require a specific available reservation and reserved-capacity accounting.

## Implementation scope

The minimal design is confined to `aws/karpenter-provider-aws`:

1. Add and validate the `EC2NodeClass` group field.
2. Include the group ARN in provider-generated launch templates.
3. Reconcile current association through the provider's EC2 instance controller.
4. Expose provider-owned metadata, conditions, metrics, and events.
5. Continue presenting group-targeted offerings to core as ordinary On-Demand offerings.

Karpenter core does not need to understand groups because the proposal adds no new core capacity type, offering type, scheduling rule, or reservation accounting model.

Core changes would be needed only for a future design in which mutable group association affects core scheduling, reservation allocation, or consolidation economics. That is outside the initial proposal.
