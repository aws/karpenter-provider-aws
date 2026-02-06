# On-Demand Capacity Reservations

This document proposes supporting ODCR in Karpenter

- [On-Demand Capacity Reservations](#on-demand-capacity-reservations)
    * [Overview](#overview)
        + [Capacity Reservations](#capacity-reservations)
    * [Goals](#goals)
    * [Non-Goals](#non-goals)
    * [Capacity Reservation Selection](#capacity-reservation-selection)
        + [EC2NodeClass API](#ec2nodeclass-api)
        + [NodePool API](#nodepool-api)
            - [Only launch ODCR instances](#only-launch-odcr-instances)
            - [Launch ODCR instances with on-demand fallback](#launch-odcr-instances-with-on-demand-fallback)
            - [Launch ODCR instances with spot and on-demand fallback](#launch-odcr-instances-with-spot-and-on-demand-fallback)
    * [Scheduling Representation](#scheduling-representation)
        + [Consider ODCR first during Scheduling](#consider-odcr-first-during-scheduling)
        + [Adding ODCRs as Additional Instance Type Offerings](#adding-odcrs-as-additional-instance-type-offerings)
        + [Representing ODCR Available Instance Counts in Instance Type Offerings](#representing-odcr-available-instance-counts-in-instance-type-offerings)
    * [CloudProvider Launch Behavior](#cloudprovider-launch-behavior)
        + [Capacity Reservation Targeting and CreateFleet Usage Strategy](#capacity-reservation-targeting-and-createfleet-usage-strategy)
        + [Open Capacity Reservations](#open-capacity-reservations)
    * [Capacity Reservation Expiration/Cancellation](#capacity-reservation-expirationcancellation)
    * [Pricing/Consolidation](#pricingconsolidation)
        + [Provisioning](#provisioning)
        + [Consolidation](#consolidation)
            - [Consolidating into Capacity Reserved Instances](#consolidating-into-capacity-reserved-instances)
            - [Consolidating between Capacity Reservations](#consolidating-between-capacity-reservations)
    * [Drift](#drift)
        + [The NodePool selects on `karpenter.sh/capacity-type: reserved` but the instance is no longer in a reservation](#the-nodepool-selects-on-karpentershcapacity-type-reserved-but-the-instance-is-no-longer-in-a-reservation)
        + [The `capacityReservationSelectorTerms` no longer selects an instance's capacity reservation](#the-capacityreservationselectorterms-no-longer-selects-an-instances-capacity-reservation)
    * [Appendix](#appendix)
        + [Input/Output for CreateFleet with CapacityReservations](#inputoutput-for-createfleet-with-capacityreservations)
            - [Specifying Multiple ODCRs with the same Instance Type/Availability Zone Combination](#specifying-multiple-odcrs-with-the-same-instance-typeavailability-zone-combination)

## Overview

In AWS [ODCR](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-capacity-reservations.html) allows users to reserve compute capacity to mitigate the risk of getting on-demand capacity. This is very helpful during seasonal holidays where higher traffic is expected or for reserving highly-desired instance types, like the `p5.48xlarge` or other large GPU instance types.

This RFC outlines the proposed API and implementation of support for On-Demand Capacity Reservations within Karpenter. Support for this feature and respective API would launch initially in alpha under the `CapacityReservations` feature gate. When enabled, this feature will allow users to select capacity reservations through `capacityReservationSelectorTerms` in their EC2NodeClasses. Karpenter will then discover and use these capacity reservations during scheduling and disruption (including consolidation) simulations to prioritize scheduling pods into the selected reservations.

### Capacity Reservations

Each [Capacity Reservation](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ec2@v1.162.1/types#CapacityReservation) is defined with:

- The Availability Zone in which to reserve the capacity
- The count of instances for which to reserve capacity
- The instance attributes, including the instance type, tenancy, and platform/OS
- Instance match criteria
  - Targeted -- only accept instances that matches all attributes + explicitly targeted the capacity reservation
  - Open -- if capacity reservation accepts all instances that matches all attributes
- Reservation type
  - [Default](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-capacity-reservations.html) -- standard capacity reservations, pre-reserved capacity for on-demand instances in arbitrary instance counts
  - [Capacity Block](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/capacity-blocks-using.html) -- pre-reserved capacity in specific block sizes that is only allocated for a specific amount of time (up to 14 days in 1-day increments or up to 28 days in 7-day increments)
- A start and end date (if applicable) for when the reservation of capacity is available

AWS also supports grouping Capacity Reservation into [Capacity Reservation groups](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/create-cr-group.html). Both these entities are supported in Launch Template's CapacityReservationTarget [definitions](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-launchtemplate-capacityreservationtarget.html).

## Goals

1. Allow selection of targeted and open ODCRs with Karpenter
2. Ensure multiple ODCRs can be selected from a single NodePool
3. Ensure that we only launch capacity into an ODCR in a cluster when an application requires the capacity, ensuring ODCR sharing between clusters and accounts
4. Ensure ODCRs are prioritized over regular OD and spot capacity
5. Ensure Karpenter consolidates regular OD and spot instances to ODCR capacity when it is available
6. Ensure Karpenter consolidates between ODCRs when a smaller/cheaper ODCR is available
7. Allow users to constrain a NodePool to only launch into ODCR capacity without fallback
8. Allow users to fallback from ODCR to spot capacity and from ODCR to standard OD capacity
9. Ensure OD capacity is not automatically drifted to new capacity when a capacity reservation expires or is canceled to reduce workload disruption

## Non-Goals

Below lists the non-goals for _this RFC design._ Each of these items represents potential follow-ups for the initial implementation and are features we will consider based on feature requests.

1. Ensure OD instances can be automatically attached to an ODCR after the fact rather than replaced/drifted if an ODCR has availability later
2. Support [Capacity Blocks](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/capacity-blocks-using.html) as a capacity-type -- though capacity blocks are not supported with this design, they are a natural extension of it. We could support selection on capacity blocks through the `capacityReservationSelectorTerms`.
3. Support [Capacity Reservation Groups](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/create-cr-group.html) -- though capacity reservation groups are not supported with this design, they are a natural extension of it. We could support an additional field `reservationGroup` in the `capacityReservationSelectorTerms`.

## Capacity Reservation Selection

### EC2NodeClass API

- Add a new field under `spec` for `capacityReservationSelectorTerms` to `EC2NodeClass` for defining which Capacity Reservation to be used for a specific `EC2NodeClass`
  - This will allow us to attach multiple Capacity Reservations across AZs and Instance Types to a single EC2NodeClass. This capability removes the need for `Capacity Reservation Groups` for this MVP.
- Add a new field under `status` for the found Capacity Reservations by the `spec.capacityReservationSelectorTerms` for the `EC2NodeClass`

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: example-node-class
spec:
  # capacityReservationSelectorTerms specify selectors which are ORed together to generate
  # a list of filters against the EC2 DescribeCapacityReservation API
  # ID cannot be specified with any other field within a single selector
  # All other fields are not mutually exclusive and can be combined
  capacityReservationSelectorTerms:
    - # The id for the Capacity Reservation
      id: String | None
      # The id of the AWS account that owns the Capacity Reservation
      # If no ownerID is specified, any ODCRs available to the current account will be used
      ownerID: String | None
      # Tags is a map of key/value tags used to select capacity reservations
      # Specifying '*' for a value selects all values for a given tag key.
      tags: Map | None
status:
  capacityReservations:
    - # AvailabilityZone for the Capacity Reservation
      availabilityZone: String
      # The time at which the Capacity Reservation expires. When a Capacity
      # Reservation expires, the reserved capacity is released and you can no longer
      # launch instances into it. The Capacity Reservation's state changes to expired
      # when it reaches its end date and time.
      endTime: String | None
      # id for the Capacity Reservation
      id: String
      # Indicates the instanceMatchCriteria of instance launches that the Capacity Reservation accepts. The options include:
      #   - open:
      #       The Capacity Reservation accepts all instances that have
      #       matching attributes (instance type, platform, and Availability
      #       Zone). Instances that have matching attributes launch into the
      #       Capacity Reservation automatically without specifying any
      #       additional parameters.
      #   - targeted:
      #       The Capacity Reservation only accepts instances that
      #       have matching attributes (instance type, platform, and
      #       Availability Zone), and explicitly target the Capacity
      #       Reservation. This ensures that only permitted instances can use
      #       the reserved capacity.
      instanceMatchCriteria: String
      # Instance Type for the Capacity Reservation
      instanceType: String
      # The id of the AWS account that owns the Capacity Reservation
      ownerID: String
```

This API follows closely with how [DescribeCapacityReservations](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeCapacityReservations.html) can filter capacity reservations -- allowing Karpenter to receive the server-side filtered version of the capacity reservations to store in its status.

### NodePool API

The EC2NodeClass API allows selection on capacity reservations, which give additional options to the scheduler to choose from when launching instance types; however, it does not offer a mechanism to scope-down whether instances in a NodePool should only launch into an ODCR, fallback between a capacity reservation to on-demand if none is available, or fallback between a capacity reservation to spot and then finally to on-demand.

This RFC proposes the addition of a new `karpenter.sh/capacity-type` label value, called `reserved`. A cluster admin could then select to support only launching ODCR capacity and falling back between ODCR capacity to on-demand capacity respectively. _NOTE: This option requires any applications (pods) that are using node selection on `karpenter.sh/capacity-type: "on-demand"` to expand their selection to include `reserved` or to update it to perform a `NotIn` node affinity on `karpenter.sh/capacity-type: spot`_

#### Only launch ODCR instances

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
 name: default
spec:
 requirements:
 - key: karpenter.sh/capacity-type
   operator: In
   values: ["reserved"]
```

#### Launch ODCR instances with on-demand fallback

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
 name: default
spec:
 requirements:
   - key: karpenter.sh/capacity-type
     operator: In
     values: ["on-demand", "reserved"]
```

#### Launch ODCR instances with spot and on-demand fallback

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
 name: default
spec:
 # No additional requirements needed, launch all capacity types by default
 requirements: []
```

## Scheduling Representation

Since ODCRs are a AWS-specific concept, there needs to be a mechanism to pass down these ODCR options down for the scheduler to reason about. Importantly, we need the scheduler to know to prioritize these ODCR options when a user has specified them in their EC2NodeClass. Further, we need the scheduler to be aware that it can't launch an unlimited amount of these instances into an ODCR.

_Note: Updates to the scheduling representation of our offerings, including changes to how the core scheduling behavior works are going to require extensive performance benchmarking to ensure that we do not significantly trade-off performance when we support users selecting on ODCRs._

### Consider ODCR first during Scheduling

Karpenter's current scheduling algorithm uses [First-Fit Decreasing bin-packing](https://en.wikipedia.org/wiki/First-fit-decreasing_bin_packing#:~:text=First%2Dfit%2Ddecreasing%20FFD,is%20at%20most%20the%20capacity). as a heuristic to optimize pod scheduling to nodes. For a new node that Karpenter chooses to launch, it will continue packing pods onto this new node until there are no more available instances type offerings. This happens regardless of the remaining capacity types in the offerings AND regardless of the price as offerings are removed.

This presents a challenge for prioritizing ODCRs -- since this algorithm may remove `reserved` offerings to continue packing into `on-demand` and `spot` offerings, thus increasing the cost of the cluster and not fully utilizing the available capacity reservations.

To solve for this problem, Karpenter will implement special handling for `karpenter.sh/capacity-type: reserved`. If there are reserved offerings available, we will consider these offerings as "free" and uniquely prioritize them. This means that if we are about to remove the final `reserved` offering in our scheduling simulation such there are no more `reserved` offerings, rather than scheduling this pod to the same node, we will create a new node, retaining the `reserved` offering, ensuring these offerings are prioritized by the scheduler.

### Adding ODCRs as Additional Instance Type Offerings

We can surface ODCR capacity as additional offerings attached to each instance type. Offerings currently allow us to track the pricing of variants of a specific instance type, primarily based on capacity type and availability zone today.

To track reservation capacity, we can add additional offerings to an instance type when there is a capacity reservation that is matched on by an EC2NodeClass's `capacityReservationSelectorTerms`. This offering will have a price near 0 to model the fact that the reservation is already paid-for and to ensure the offering is prioritized ahead of other offerings.

When there are multiple capacity reservation offerings for an instance type for different AZs, we will produce separate offerings for these different zones. When there are multiple capacity reservation offerings for instance type in the same AZ, we will only produce a single offering. With this change, an example instance type offerings set will look like the following

```yaml
name: c5.large
offerings:
  - price: ....
    available: ....
    requirements:
      - key: karpenter.sh/capacity-type
        operator: In
        values: ["reserved"]
      - key: topology.kubernetes.io/zone
        operator: In
        values: ["us-west-2a"]
      - key: topology.k8s.aws/zone-id
        operator: In
        values: ["usw2-az1"]
  - price: ....
    available: ....
    requirements:
      - key: karpenter.sh/capacity-type
        operator: In
        values: ["on-demand"]
      - key: topology.kubernetes.io/zone
        operator: In
        values: ["us-west-2a"]
      - key: topology.k8s.aws/zone-id
        operator: In
        values: ["usw2-az1"]
  - price: ....
    available: ....
    requirements:
      - key: karpenter.sh/capacity-type
        operator: In
        values: ["spot"]
      - key: topology.kubernetes.io/zone
        operator: In
        values: ["us-west-2a"]
      - key: topology.k8s.aws/zone-id
        operator: In
        values: ["usw2-az1"]
```

### Representing ODCR Available Instance Counts in Instance Type Offerings

ODCRs (unlike spot and on-demand capacity) have much more defined, constrained capacity ceilings. For instance, in an extreme example, a user may select on a capacity reservation with only a single available instance but launch 10,000 pods that contain hostname anti-affinity. The scheduler would do work to determine that it needs to launch 10,000 instances for these pods; however, without any kind of cap on the number of times the capacity reservation offering could be used, the scheduler would think that it could launch 10,000 instances into the capacity reservation offering.

Attempting to launch this would result in a success for a single instance and an ICE error for the other 9,999. The next scheduling loop would remediate this, but this results in a lot of extra, unneeded work.

A better way to model this would be to track the available instance count as a numerical value associated with an instance type offering. In this modeling, the scheduler could count the number of simulated NodeClaims that might use the offering and know that it can't simulate NodeClaims into particular offerings once they hit their cap.

Today, we already have an [`available` field](https://github.com/kubernetes-sigs/karpenter/blob/bcd33e924905588b1bdecd5413dc7b268370ec4c/pkg/cloudprovider/types.go#L236) attached to instance type offerings. This field is binary and only tells us whether the instance is or isn't available. With the introduction of ODCR offerings, we could extend this field to be an integer rather than a boolean. This would allow us to exactly represent the number of available instances that can be launched into the offering. Existing spot and on-demand offerings would model this `available` field as `MAX_INT` for current `true` values and `0` for `false` values.

An updated version of the instance type offerings for an ODCR, on-demand, and spot capacity would look like

```yaml
name: c5.large
offerings:
  - price: 0.00000001
    available: 5
    requirements:
      - key: karpenter.sh/capacity-type
        operator: In
        values: ["reserved"]
      - key: topology.kubernetes.io/zone
        operator: In
        values: ["us-west-2a"]
      - key: topology.k8s.aws/zone-id
        operator: In
        values: ["usw2-az1"]
  - price: 0.085
    available: 4294967295
    requirements:
      - key: karpenter.sh/capacity-type
        operator: In
        values: ["on-demand"]
      - key: topology.kubernetes.io/zone
        operator: In
        values: ["us-west-2a"]
      - key: topology.k8s.aws/zone-id
        operator: In
        values: ["usw2-az1"]
  - price: 0.0315
    available: 4294967295
    requirements:
      - key: karpenter.sh/capacity-type
        operator: In
        values: ["spot"]
      - key: topology.kubernetes.io/zone
        operator: In
        values: ["us-west-2a"]
      - key: topology.k8s.aws/zone-id
        operator: In
        values: ["usw2-az1"]
```

## CloudProvider Launch Behavior

When a NodeClaim is passed to the CloudProvider `Create()` call that selects the `reserved` capacity type, the AWS Cloud Provider will prioritize launching into the `reserved` capacity type before attempting other capacity types.

Practically, this means that when a NodeClaim allows for the `reserved` capacity type, Karpenter will know that this NodeClaim is requesting to launch into an ODCR and leverage available ODCR offerings from this NodePool that match the instance type and availability zone requirements passed through the NodeClaim.

In order to properly pass these ODCR options into `CreateFleet`, Karpenter will need to generate launch templates for the ODCR possibilities that we can launch into. Each ODCR id will need a separate launch template created during launch. Karpenter will then take each of these launch templates and pass them to `CreateFleet` as separate `launchTemplateConfig` options.

_Note: `CreateFleet` does not currently support passing multiple launch templates that target the same instance type and availability zone. This means that different ODCRs that target the same instance type/availability zone combination cannot be passed-through at the same time. To avoid this, Karpenter will pick the ODCR with the greatest available instance count when choosing from ODCRs with the same instance type and avaialbility zone._

### Capacity Reservation Targeting and CreateFleet Usage Strategy

`CreateFleet` supports a parameter called [`usageStrategy`](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CapacityReservationOptionsRequest.html) within the `capacityReservationOptions` stanza of the `onDemandOptions`. This `usageStrategy` allows you to inform Fleet that you are using capacity reservations and to consider them first when launching. Currently, the only available enum for this field is `use-capacity-reservations-first`, which tells Fleet to use-up any available ODCRs (whether targeted or open) when launching and then fall-back to on-demand to fulfill the remaining capacity. This fall-back behavior isn't necessarily desired for an orchestrator like Karpenter where we might have used a different NodePool or a different instance type option entirely if Karpenter controlled the fallback.

As a result, Karpenter will not use this option when launching instances with `CreateFleet`. Instead, Karpenter will specifically target all ODCRs that are selected-on for a given launch request by passing the ID through the LaunchTemplate. `usageStrategy` will not be specified, but only ODCR launch templates will be included in a request that is targeting ODCRs. As a result, Fleet will choose the cheapest instance type from the available options based on the `lowest-price` allocation strategy and launch an instance the respective ODCR.

In some race condition scenarios, CreateFleet, when creating instances against targeted Capacity Reservations, will fail if all Reservations are already fully utilized, resulting in an `ReservationCapacityExceeded` error across the request. In these cases, Karpenter will throw an Insufficient Capacity error internally, delete the NodeClaim used for the request and attempt to reschedule capacity again given th remaining application pods.

> For more information on how CreateFleet interacts with capacity reservations and `usageStrategy`, see https://docs.aws.amazon.com/emr/latest/ManagementGuide/on-demand-capacity-reservations.html.

### Open Capacity Reservations

"Open" is one of the `instanceMatchCriteria` options for a capacity reservation. This criteria allows EC2 to automatically assign a launched instance to an ODCR so long as the instance matches the instance type and availability zone of an available, open ODCR.

Because EC2 is in control of the assignment between instances and ODCRs and not Karpenter, open ODCRs interact poorly with Karpenter's drift mechanisms.

As an example, take an EC2NodeClass that is selecting on ODCR `cr-123456789`. This capacity reservation is completely utilize, so we launch a standard OD instance from the selected instance types in the NodePool. This launch _happens_ to match an open ODCR, so we see an assignment occur to a capacity reservation. Karpenter now recognizes that this instance is in an ODCR, but this ODCR doesn't match the selected ODCRs, so it begins to replace the instance due to drift. A new instance that is launched matches the ODCR again and this cycle continues.

To avoid this problem, when an EC2NodeClass uses the `capacityReservationSelectorTerms` block, we will opt-out of open matching in our LaunchTemplates by setting `capacityReservationPreference` as `none`. This means that it won't be possible for any instance launched from this EC2NodeClass to join an ODCR that hasn't been explicitly selected on, solving the drift problem.

## Capacity Reservation Expiration/Cancellation

Capacity reservations [support an option to expire the reservation at a specific date and time](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/capacity-reservations-using.html). When the reservation expires, any instances present in the reservation at the time will have their association with the reservation removed and the instances will be charged at the standard on-demand instance rate.

When this occurs, we need to ensure two things in Karpenter:
1. We no longer attempt to launch instances into this reservation (e.g. we ensure the offering is removed from the set of capacity reservation offerings)
2. Instances which were marked as part of a capacity reservation have their label removed so we no longer treat their pricing as capacity reservation pricing

To ensure the first item, we can simply filter out non-active capacity reservations when storing capacity reservations in EC2NodeClass status. Ensuring the second item involves us polling the DescribeInstances API to validate the mapping between the capacity reservation and the instance, removing the mapping from the NodeClaim/Node if the instance is no longer in a reservation.

## Pricing/Consolidation

### Provisioning

Pricing is directly considered during provisioning and consolidation as capacity-reservation is prepaid. It is assumed to have a price of "near 0" during provisioning (see [Consolidating between Capacity Reservations](#consolidating-between-capacity-reservations) for more detail on why this is not just "0").

### Consolidation

During consolidation pricing does matter as it affects which candidate will be [prioritized](https://github.com/kubernetes-sigs/karpenter/blob/75826eb51589e546fffb594bfefa91f3850e6c82/pkg/controllers/disruption/consolidation.go#L156). Since all capacity instances are paid ahead of time, their cost is already incurred. Users would likely want to prioritize filling their reserved capacity.
reservation first then fall back into other instances. Because of this reserved instances should likely show up as 0 dollar pricing when we calculate the instance pricing.

#### Consolidating into Capacity Reserved Instances

If we track Capacity Reservation usage, we can optimize the cluster configuration by moving non-Capacity Reserved instances into
Capacity Reserved instances. We would need to match the instance type, platform and availability zone prior to doing this.

This would be done by the standard consolidation algorithm and should work with minimal changes, since consolidation already optimizes for cost.

#### Consolidating between Capacity Reservations

Treating the price of all capacity reservation offerings as `0` sounds sensical but has some adverse interactions with consolidation. Most notably, if we launch an instance into a capacity reservation offering, we will _never_ consolidate out of that offering -- even if there is a smaller instance type that would work just as well for the pods on that node.

In practicality, that larger capacity reservation could have been used for other work -- perhaps on another cluster, but may have been held by a single pod that prevented Karpenter from consolidating it.

To solve for this edge case, we won't model the pricing of capacity reservations as `0` but as a "near-0" value. We'll divide the existing price of the on-demand offering by the most expensive hourly rate offering (e.g. $407.68 for the `u7in-32tb.224xlarge`) over the least expensive hourly rate offering for a spot instance (e.g. $0.0015 for `t4g.nano`). We'll then take this value and then further divide it by 1,000,000 to get a sufficiently small number that is significantly smaller than the least expensive offering.

This value will be calculated dynamically based on the pricing info that Karpeneter discovers at runtime. This allow us to represent a price that is less than every other instance type offering, but still maintains the relative ordering of on-demand instance types.

In practice, this means that if a user has two capacity reservation offerings available: one for a `c6a.48xlarge` and another for a `c6a.large`, where we launch into the `c6a.48xlarge` first, we will still be able to consolidate down to the `c6a.large` when pods are scaled back down.

## Drift

Due to capacity reservation expiration or due to changes from the user in their capacity reservation selection in their EC2NodeClass, instances can drift from the NodePool capacity reservation specification. Below outlines two ways that this drift can occur and how it will be re-reconciled.

### The NodePool selects on `karpenter.sh/capacity-type: reserved` but the instance is no longer in a reservation

Assuming we have a reconciler that we will build as part of this design proposal that will update the `karpenter.sh/capacity-type` label from `reserved` to `on-demand` when a node is no longer in a capacity reservation, Karpenter's dynamic requirement drift checking should cause drift reconciliation in this case.

In this case, since the NodePool is selecting on a label that does not exist on the NodeClaim, drift detection will recognize that the NodeClaim is invalid and will mark it as drifted to be replaced by another node.

### The `capacityReservationSelectorTerms` no longer selects an instance's capacity reservation

In this case, there is no existing mechanism in Karpenter that would catch this. Karpenter will need to implement an additional mechanism that validates that an instance's capacity reservation falls within the valid set of reservations selected-on from the `capacityReservationSelectorTerms`. Specifically, it needs to validate that the id exists within the `capacityReservation` section of the EC2NodeClass status.

## Appendix

### Input/Output for CreateFleet with CapacityReservations

| ODCR Targeting                      | `usageStrategy`                  | `capacityReservationPreference` | Result                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
|-------------------------------------|----------------------------------|---------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Yes, targeting `targeted` or `open` | `use-capacity-reservation-first` | `null` (default `open`)         | CreateFleet will not use any launch templates that target capacity reservations when using `usageStrategy: use-capacity-reservation-first`.                                                                                                                                                                                                                                                                                                                                                |
| No                                  | `use-capacity-reservation-first` | `null` (default `open`)         | CreateFleet will find available open ODCRs and prioritize launching into these before launching regular on-demand capacity. These ODCRs will be ordered by the `lowest-price` allocation strategy. Once all open ODCRs from passed-through instance type/availability zone combinations have been exhausted, Fleet will launch standard on-demand capacity -- even if targeted ODCRs are available.                                                                                        |
| Yes, targeting `targeted` or `open` | `null`                           | `null` (default `open`)         | CreateFleet does not have any knowledge about ODCRs in this mode will use the `lowest-price` allocation strategy to order the instance types/availability zones. When specifying multiple ODCRs in separate launch templates and running out of capacity in one ODCR, CreateFleet will automatically select the other ODCR that has capacity. When all ODCRs are exhausted, CreateFleet will return `ReservationCapacityExceeded` rather than falling back to standard on-demand capacity. |
| No                                  | `null`                           | `null` (default `open`)         | This is the current state today. CreateFleet has no knowledge about capacity reservations and will not specifically try to launch into them. If a user gets lucky and Fleet happens to launch into an instance type/availability zone combination that _happens_ to match an open ODCR, then the instance will be attached to this ODCR.                                                                                                                                                   |
| No                                  | `null`                           | `none`                          | CreateFleet has no knowledge about capacity reservations and will not specifically try to launch into them. If a user gets lucky and Fleet happens to launch into an instance type/availability zone combination that _happens_ to match an open ODCR, __the instance will not join the ODCR and will become standard on-demand capacity due to the `capacityReservationPreference`.__                                                                                                     |
| Yes, targeting `targeted` or `open` | `null`                           | `none`                          | Errors on launch. A capacity reservation preference of `none` cannot be used while targeting ODCRs in the launch template.                                                                                                                                                                                                                                                                                                                                                                 |

#### Specifying Multiple ODCRs with the same Instance Type/Availability Zone Combination

CreateFleet will reject the call outright since you are not allowed to specify duplicate instance type/availability zone combinations, even if the launch templates contain different data -- such as different capacity reservation ids.

> For more information on CreateFleet's handling when specifying different `usageStrategy` and `capacityReservationPreference` values, see https://docs.aws.amazon.com/emr/latest/ManagementGuide/on-demand-capacity-reservations.html.
