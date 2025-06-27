# Capacity Block Support

## Overview

In v1.3.0 Karpenter introduced formal support for on-demand capacity reservations.
However, this did not include a subset of ODCRs: Capacity Blocks.
Capacity Blocks enable users to “reserve highly sought-after GPU instances on a future date to support short duration ML workloads”.
This doc will focus on the extension to Karpenter’s existing ODCR feature to support Capacity Blocks.

## Goals

- Karpenter should enable users to select against Capacity Blocks when scheduling workloads
- Karpenter should discover Capacity Blocks through `ec2nodeclass.spec.capacityReservationSelectorTerms`
- Karpenter should gracefully handle Capacity Block expiration

## API Updates

We will add the `karpenter.k8s.aws/capacity-reservation-type` label, which can take on the values `default` and `capacity-block`.
This mirrors the `reservationType` field in the `ec2:DescribeCapacityReservation` response and will enable users to select on capacity block nodes via NodePool requirements or node selector terms.

```yaml
# Configure a NodePool to only be compatible with instance types with active
# capacity block reservations
kind: NodePool
apiVersion: karpenter.sh/v1
spec:
  template:
    spec:
      requirements:
      - key: karpenter.k8s.aws/capacity-reservation-type
        operator: In
        values: ['capacity-block']
---
# Configure a pod to only schedule against nodes backed by capacity blocks
kind: Pod
apiVersion: v1
spec:
  nodeSelector:
    karpenter.k8s.aws/capacity-reservation-type: capacity-block
```

Additionally, we will update the NodeClass status to reflect the reservation type and state for a given capacity reservation:

```yaml
kind: EC2NodeClass
apiVersion: karpenter.k8s.aws/v1
status:
  capacityReservations:
  - # ...
    reservationType: Enum (default | capacity-block)
    state: Enum (active | expiring)
```

No changes are required for `ec2nodeclass.spec.capacityReservationSelectorTerms`.

## Launch Behavior

Today, when Karpenter creates a NodeClaim targeting reserved capacity, it ensures it is launched into one of the correct reservations by injecting a `karpenter.k8s.aws/capacity-reservation-id` requirement into the NodeClaim.
By injecting this requirement, we ensure Karpenter can maximize flexibility sent to CreateFleet (minimizing risk of ReservedCapacityExceeded errors) while also ensuring Karpenter doesn’t overlaunch into any given reservation.

```yaml
kind: NodeClaim
apiVersion: karpenter.sh/v1
spec:
  requirements:
  - key: karpenter.k8s.aws/capacity-reservation-id
    operator: In
    values: ['cr-foo', 'cr-bar']
  # ...
```

Given the NodeClaim spec above, Karpenter will create launch templates for both `cr-foo` and `cr-bar`, providing both in the CreateFleet request.
However, this breaks down when we begin to mix default and capacity-block ODCRs (e.g. `cr-foo` is a default capacity reservation, and `cr-bar` is a capacity-block).
This is because the `TargetCapacitySpecificationRequest.DefaultTargetCapacityType` field in the CreateFleet request needs to be set to on-demand or capacity-block, preventing us from mixing them in a single request.
Instead, if a NodeClaim is compatible with both types of ODCRs, we must choose a subset of those ODCRs to include in the CreateFleet request.
We have the following options for prioritization when making this selection:

- Prioritize price (the subset with the “cheapest” offering)
- Prioritize flexibility (the subset with the greatest number of offerings)

Although prioritizing flexibility is desireable to reduce the risk of ReservedCapacityExceeded errors, it won’t interact well with consolidation and result in additional node churn.
For that reason, we should prioritize the set of ODCRs with the “cheapest” offering when generating the CreateFleet request.
If there is a tie between a default and capacity-block offering, we will prioritize the capacity-block offering.

## Interruption

Although capacity blocks are modeled as ODCRs, their expiration behavior differs.
Any capacity still in use when a default ODCR expires falls back to a standard on-demand instance.
On the other hand, instances in use from a capacity block reservation are terminated ahead of their end date.

From the [EC2 documentation](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-capacity-blocks.html):

> You can use all the instances you reserved until 30 minutes before the end time of the Capacity Block.
> With 30 minutes left in your Capacity Block reservation, we begin terminating any instances that are running in the Capacity Block.
> We use this time to clean up your instances before delivering the Capacity Block to the next customer.
> We emit an event through EventBridge 10 minutes before the termination process begins.
> For more information, see Monitor Capacity Blocks using EventBridge.

Karpenter should gracefully handle this interruption by draining the nodes ahead of termination.
While we could integrate with the EventBridge event referenced above, this introduces complications when rehydrating state after a controller restart.
Instead, we will rely on the fact that interruption occurs at a fixed time relative to the end date of the capacity reservation, which is already discovered via `ec2:DescribeCapacityReservation`.
Matching the time the expiration warning event is emmitted, Karpenter will begin to drain the node 10 minutes before EC2 begins reclaiming the capacity (40 minutes before the end date).
Once the reclaimation period begins, Karpenter will mark the capacity reservation as expiring in the EC2NodeClass' status.
