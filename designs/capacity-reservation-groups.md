# RFC: Capacity Reservation Resource Group Targeting in Karpenter

## Summary

Karpenter currently targets individual Capacity Reservations with available capacity. It cannot launch an instance with a stable Capacity Reservation Resource Group target and let AWS manage association over that instance's lifetime.

Group targeting would be an alternative ODCR consumption model. Instead of Karpenter selecting and accounting for a specific reservation ID at launch, Karpenter would delegate reservation selection and later reassociation to AWS within a capacity reservation group. This provides AWS managed matching without the account wide scope of open matching.

This RFC proposes an operator managed group ARN on `EC2NodeClass`. Karpenter would:

- place the group target in the launch template;
- allow AWS's documented On-Demand fallback when the group has no compatible available reservation;
- periodically observe the reservation, if any, currently covering the instance; and
- update capacity type, reservation metadata, and internal pricing as association changes.

Karpenter would not create groups, manage group membership, or create and resize reservations.

## Motivation

### Cover an existing running fleet

A group targeted instance can launch on ordinary On-Demand capacity and later become covered when compatible capacity is added to the group. This allows operators to establish reservation coverage around an already running fleet without replacing that fleet solely to change reservation association.

For example:

| Stage | Running | Reserved | Unused reserved |
|---|---:|---:|---:|
| Group targeted instances on ordinary capacity | 100 | 0 | 0 |
| Compatible reservations added and matched | 100 | 100 | 0 |
| Additional compatible capacity added | 100 | 120 | 20 |

The group itself reserves no capacity. Adding only 20 slots while 100 compatible group targeted instances are uncovered may cause those instances to consume all 20 slots.

### Reacquire coverage after replacement overlap

Suppose ten instances occupy ten reserved slots. A replacement launches before an old instance terminates, while the reservation is full. With a persistent group target, the replacement can launch on On-Demand capacity and later become covered when the old instance releases its slot. It does not need another replacement solely to regain coverage.

Issue [#9518](https://github.com/aws/karpenter-provider-aws/issues/9518) contains independent reports of both problems. The issue requests open matching, not groups; it is evidence for the use cases, not agreement on this proposal.

### Isolate reservation capacity between workloads

Open ODCR matching does not isolate workloads: any eligible compatible instance can consume a reservation, regardless of NodePool, NodeClass, or reservation tags. Separate groups containing targeted reservations let each workload retain AWS managed matching within its own reservation set. Reservations shared across groups remain a shared capacity pool.

## Current behavior

### AWS

AWS documents that:

- `CapacityReservationTarget.CapacityReservationResourceGroupArn` targets a Capacity Reservation Resource Group.
- EC2 can launch on ordinary On-Demand capacity when the group has no compatible available reservation.
- A running group targeted instance can later match compatible capacity added to the group.
- Recipient accounts can add active reservations shared with them to their own groups.

Sources: [group launch](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/cr-groups-launch.html), [group lifecycle](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/cr-groups-lifecycle.html), [group membership](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/cr-groups-add.html), and [sharing](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/capacity-reservation-sharing.html).

AWS also announced group targeting support for launch templates and EC2 Fleet: [AWS announcement](https://aws.amazon.com/about-aws/whats-new/2020/07/amazon-ec2-on-demand-capacity-reservations-now-support-group-targeting/).

`DescribeInstances` reports both the current `CapacityReservationId` and the configured `CapacityReservationSpecification`, including a group target: [DescribeInstances API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html).

### Karpenter

Karpenter consumes reservations at launch by targeting a specific reservation ID with an available slot. Ordinary On-Demand nodes launch with reservation preference `none`, so they cannot automatically match reservations added or freed later even when `capacityReservationSelectorTerms` selects open reservations.

As a result, operators must replace existing nodes to establish coverage. Replacements launched on ordinary On-Demand while a reservation is full also cannot reclaim coverage when the old nodes release their slots. Karpenter currently supports neither open matching nor a persistent group target for these running nodes.

## Goals

- Provide group targeting as an alternative to Karpenter managed, reservation ID specific ODCR consumption.
- Delegate reservation selection and later reassociation to AWS within a capacity reservation group.
- Allow an `EC2NodeClass` to target one Capacity Reservation Resource Group.
- Reflect later reservation association and disassociation accurately.

## Non goals

- Supporting Capacity Blocks or interruptible reservations in Alpha; support is planned for Beta. Blocks terminate instances before the block ends and reclaimed interruptible capacity triggers a two minute termination notice, requiring separate launch and disruption handling.
- Creating groups or managing membership.
- Creating, sharing, resizing, or retaining reservations.
- Defining launch order policy across instance types, Availability Zones, or capacity sources.
- Defining quotas, entitlements, or fairness among group consumers.
- Retrofitting a group target onto a running instance configured with `none`. AWS documents modifying this configuration for stopped instances.
- Guaranteeing reservation availability.

## Proposed API

Add an optional group ARN to `EC2NodeClass`:

```yaml
spec:
  capacityReservationGroupARN: arn:aws:resource-groups:us-west-2:123456789012:group/workload-a
```

### Proposed Behavior

- The ARN references a Capacity Reservation Resource Group.
- Karpenter places the ARN in `CapacityReservationTarget.CapacityReservationResourceGroupArn` for On-Demand launches.
- The launch uses AWS's normal group behavior: use compatible group capacity when available; otherwise use ordinary On-Demand capacity.
- Spot launches are unchanged.
- The field is mutually exclusive with `capacityReservationSelectorTerms`.
- Group membership is authoritative. Existing reservation selectors do not further restrict AWS's later matching.

Mutual exclusion makes group targeting a distinct ODCR mode. Existing selectors keep selection, availability, and per ID accounting in Karpenter; a group deliberately gives reservation selection and lifetime reassociation to AWS.

## Launch, observation, and accounting

Karpenter creates a launch template containing the group ARN and submits it through the existing instant `CreateFleet` path, with `OnDemandOptions.CapacityReservationOptions.UsageStrategy=use-capacity-reservations-first`. This path targets ordinary ODCRs and permits On-Demand fallback.

Expected outcomes:

- EC2 covers the instance with a compatible group reservation when available.
- Otherwise, EC2 launches the instance as ordinary On-Demand while retaining the group target.
- EC2 may later associate or disassociate the running instance as group capacity changes.

### Association reconciliation

After launch, Karpenter must observe EC2 to determine actual reservation coverage rather than infer it from the requested capacity type. Store the persistent group target separately from the current reservation ID so disassociation preserves target intent.

Karpenter should periodically call `DescribeInstances` for group targeted nodes and reconcile reservation metadata, capacity type, and internal pricing. Operations that depend on reservation association must refresh it before acting.

This new behavior would only run on NodePools that uses Capacity Reservation Group which should limit scope of this polling.

### Scheduling and consolidation accounting

When EC2 associates a node with a reservation, Karpenter must update its Node and NodeClaim capacity type to `reserved`, record the reservation ID, and use reserved capacity pricing internally. On disassociation, it must restore `on-demand`, clear reservation metadata, and restore On-Demand pricing. Pricing caches and consolidation inputs must reflect both transitions.

New launches must account for possible On-Demand fallback. Group slots must not be counted as exclusively available to new NodeClaims: AWS can assign them to running nodes. Consolidation must refresh group level association and pricing before evaluation and revalidate before disruption, including possible reassignment of a released slot.

Current individual ID targeting remains unchanged for launches that require a specific available reservation and reserved capacity accounting.

## Implementation scope

Implementation spans the AWS provider and any required Karpenter core integration:

1. Add and validate the `EC2NodeClass` group field.
2. Include the group ARN in provider generated launch templates and configure Fleet to use reservations first.
3. Observe initial coverage and extend reconciliation to handle association, disassociation, and reservation ID changes on both Nodes and NodeClaims. Preserve the group target separately.
4. Update internal pricing and consolidation inputs as coverage changes without advertising occupied group slots as available for launch.
5. Handle capacity type transitions without unintended NodePool drift: require both `on-demand` and `reserved`, or adjust drift semantics.
6. Prevent disruption from crediting a released group slot to a replacement; AWS may assign it to another running node. Refresh association and pricing before disruption.

## Validation

Validate the exact Fleet request with empty and full group fallback, later matching, replacement slot reacquisition, and loss of coverage. Tests must verify metadata and pricing in both directions, no unintended drift, and no guaranteed replacement slot credit. Live Fleet validation remains outstanding.

## Graduation Criteria

Group targeting requires explicit configuration on a NodePool's EC2NodeClass, so no feature gate is needed. Existing NodeClasses without a group retain their current behavior.

- **Alpha:** Support ordinary ODCR group targeting, association reconciliation, and pricing updates with unit, integration, and live Fleet validation.
- **Beta:** Add Capacity Block and interruptible reservation support, validating their launch configuration, pricing, and termination handling. Validate shared reservations, group isolation, safe drift and disruption, and acceptable polling latency and API load. Document permissions, migration, and rollback.
- **GA:** Demonstrate stable production use across releases, a stable API, and no unresolved critical correctness or performance issues.
