# Capacity Reservations

_Note: This RFC pulls various excerpts from the [`aws/karpenter-provider-aws` On-Demand Capacity Reservation RFC](https://github.com/aws/karpenter-provider-aws/blob/main/designs/odcr.md)._

## Overview

Cloud providers including GCP, Azure, and AWS allow you to pre-reserve VM (or bare-metal server) capacity before you launch. By reserving VMs ahead of time, you can ensure that you are able to launch the type of capacity you want when you need it. Without reserving capacity, it's possible you may encounter errors when launching specific instance types when there is no more capacity available on the Cloud provider for that instance type.

> #### Cloud provider VM reservation docs
> 1. GCP: Reservations - https://cloud.google.com/compute/docs/instances/reservations-overview
> 2. Azure: Capacity Reservations - https://learn.microsoft.com/en-us/azure/virtual-machines/capacity-reservation-overview
> 3. AWS: Capacity Reservations - https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-capacity-reservations.html
> 4. Oracle Cloud Infrastructure (OCI): Capacity Reservations - https://docs.oracle.com/en-us/iaas/Content/Compute/Tasks/reserve-capacity.htm

Karpenter doesn't currently support reasoning about this capacity type. Karpenter may need to be aware about this as a separate capacity type from on-demand for a few reasons:
1. Reservations are pre-paid -- meaning that if a user opts-in to Karpenter using that instance type, it's always preferable to use the reservation before launching capacity outside the reservation
2. Reservations are limited -- a user only reserves a specific count of capacity, meaning that even if Karpenter should favor the instance type while it's using the reservation, it should know when the reservation runs out and no longer continue favoring that instance type

## Proposal

1. Karpenter should introduce a new `karpenter.sh/capacity-type` called `reserved` allowing a user to specify any of `on-demand`, `spot`, or `reserved` for this label.
2. Karpenter should prioritize `reserved` instance types over other instance types while the `reserved` capacity type is available in its scheduling
3. Karpenter should add logic to its scheduler to reason about this availability as an `int` -- ensuring that the scheduler never schedules more offerings of an instance type for a capacity type than are available
4. Karpenter should extend its CloudProvider [InstanceType](https://github.com/kubernetes-sigs/karpenter/blob/35d6197e38e64cd6abfef71a082aee80e38d09fd/pkg/cloudprovider/types.go#L75) struct to allow offerings to represent availability of an offering as an `int` rather than a `bool` -- allowing Cloud Providers to represent the constrained capacity of `reserved`
5. Karpenter should consolidate between `on-demand` and/or `spot` instance types to `reserved` when the capacity type is available
6. Karpenter should introduce a feature flag `FEATURE_FLAG=ReservedCapacity` to gate this new feature in `ALPHA` when it's introduced

### `karpenter.sh/capacity-type` API

_Note: Some excerpts taken from [`aws/karpenter-provider-aws` RFC](https://github.com/aws/karpenter-provider-aws/blob/main/designs/odcr.md#nodepool-api)._

This RFC proposes the addition of a new `karpenter.sh/capacity-type` label value, called `reserved`. A cluster admin could then select to support only launching reserved node capacity and falling back between reserved capacity to on-demand (or even spot) capacity respectively.

_Note: This option requires any applications (pods) that are using node selection on `karpenter.sh/capacity-type: "on-demand"` to expand their selection to include `reserved` or to update it to perform a `NotIn` node affinity on `karpenter.sh/capacity-type: spot`_

#### Only launch nodes using reserved capacity

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
 name: reserved-only
spec:
 requirements:
 - key: karpenter.sh/capacity-type
   operator: In
   values: ["reserved"]
```

#### Launch nodes preferring reserved capacity nodes, falling back to on-demand

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
 name: prefer-reserved
spec:
 requirements:
   - key: karpenter.sh/capacity-type
     operator: In
     values: ["on-demand", "reserved"]
```

#### Launch nodes preferring reserved capacity nodes, falling back to on-demand and spot

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
 name: default
spec:
 # No additional requirements needed, launch all capacity types by default
 requirements: []
```

### Reserved capacity type scheduling prioritization

_Note: Some excerpts taken from [`aws/karpenter-provider-aws` RFC](https://github.com/aws/karpenter-provider-aws/blob/main/designs/odcr.md#consider-odcr-first-during-scheduling)._

Karpenter's current scheduling algorithm uses [First-Fit Decreasing bin-packing](https://en.wikipedia.org/wiki/First-fit-decreasing_bin_packing#:~:text=First%2Dfit%2Ddecreasing%20FFD,is%20at%20most%20the%20capacity). as a heuristic to optimize pod scheduling to nodes. For a new node that Karpenter chooses to launch, it will continue packing pods onto this new node until there are no more available instances type offerings. This happens regardless of the remaining capacity types in the offerings AND regardless of the price as offerings are removed.

This presents a challenge for prioritizing capacity reservations -- since this algorithm may remove `reserved` offerings to continue packing into `on-demand` and `spot` offerings, thus increasing the cost of the cluster and not fully utilizing the available capacity reservations.

To solve for this problem, Karpenter will implement special handling for `karpenter.sh/capacity-type: reserved`. If there are reserved offerings available, we will consider these offerings as "free" and uniquely prioritize them. This means that if we are about to remove the final `reserved` offering in our scheduling simulation such there are no more `reserved` offerings, rather than scheduling this pod to the same node, we will create a new node, retaining the `reserved` offering, ensuring these offerings are prioritized by the scheduler.

### CloudProvider interface Changes

_Note: Some excerpts taken from [`aws/karpenter-provider-aws` RFC](https://github.com/aws/karpenter-provider-aws/blob/main/designs/odcr.md#representing-odcr-available-instance-counts-in-instance-type-offerings)._

Reserved capacity (unlike spot and on-demand capacity) has much more defined, constrained capacity ceilings. For instance, in an extreme example, a user may select on a capacity reservation with only a single available node but launch 10,000 pods that contain hostname anti-affinity. The scheduler would do work to determine that it needs to launch 10,000 nodes for these pods; however, without any kind of cap on the number of times the capacity reservation offering could be used, Karpenter would think that it could launch 10,000 nodes into the capacity reservation offering.

Attempting to launch this would result in a success for a single node and failure for the other 9,999. The next scheduling loop would remediate this, but this results in a lot of extra, unneeded work.

A better way to model this would be to track the available instance count as a numerical value associated with an instance type offering. In this modeling, the scheduler could count the number of simulated NodeClaims that might use the offering and know that it can't simulate NodeClaims into particular offerings once they hit their cap.

Prior to this RFC, we already had an [`available` field](https://github.com/kubernetes-sigs/karpenter/blob/bcd33e924905588b1bdecd5413dc7b268370ec4c/pkg/cloudprovider/types.go#L236) attached to instance type offerings. This field is binary and only tells us whether the instance is or isn't available. With the introduction of `karpenter.sh/capacity-type: reserved` offerings, we could extend this field to be an integer rather than a boolean. This would allow us to exactly represent the number of available instances that can be launched into the offering. Existing spot and on-demand offerings would model this `available` field as `MAX_INT` for current `true` values and `0` for `false` values.

An updated version of the instance type offerings would look like:

```yaml
name: c5.large
offerings:
  - price: 0.085
    available: 5
    requirements:
      ...
      - key: karpenter.sh/capacity-type
        operator: In
        values: ["reserved"]
  - price: 0.085
    available: 4294967295
    requirements:
      ...
      - key: karpenter.sh/capacity-type
        operator: In
        values: ["on-demand"]
  - price: 0.0315
    available: 4294967295
    requirements:
      ...
      - key: karpenter.sh/capacity-type
        operator: In
        values: ["spot"]
```

### Consolidation

_Note: Some excerpts taken from [`aws/karpenter-provider-aws` RFC](https://github.com/aws/karpenter-provider-aws/blob/main/designs/odcr.md#consolidation)._

#### Consolidating into reserved capacity nodes

Karpenter would need to update its consolidation algorithm to ensure that consolidating between a `spot` and/or `on-demand` capacity type to a reserved capacity type is always preferred. This can be done during the cost-checking step. When evaluating cost-savings, if we are able to consolidate all existing nodes into a `reserved` capacity type node, we will choose to do so.

#### Consolidating between reserved capacity options

If we prioritize consolidating into `reserved` capacity types, we also need to ensure that we do not continue to use excessively large instance types in capacity reservations when they are no longer needed. More concretely, if there are other, smaller instance types that are available that are also in a capacity reservation, we should ensure that our consolidation algorithm continues to consolidate between them.

We can ensure this by continuing to model our pricing in our offerings with `karpenter.sh/capacity-type: reserved` as the on-demand price. This ensures that we are still able to maintain the relative ordering instance types in different capacity reservations and consolidate between them.

In practice, this means that if a user has two capacity reservation offerings available: one for a `c6a.48xlarge` and another for a `c6a.large`, where we launch into the `c6a.48xlarge` first, we will still be able to consolidate down to the `c6a.large` when pods are scaled back down.

## Appendix

1. AWS Cloud Provider's RFC for On-Demand Capacity Reservations: https://github.com/aws/karpenter-provider-aws/blob/main/designs/odcr.md
