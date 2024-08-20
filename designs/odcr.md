# On-Demand Capacity Reservations

This document proposes supporting ODCR in Karpenter

- [On-Demand Capacity Reservations](#on-demand-capacity-reservations)
    * [Background](#background)
        + [Capacity Reservations](#capacity-reservations)
    * [Goals](#goals)
    * [Non-Goals](#non-goals)
    * [Capacity Reservation Selection](#capacity-reservation-selection)
        + [EC2NodeClass API](#ec2nodeclass-api)
        + [NodePool API](#nodepool-api)
    * [Scheduling Representation](#scheduling-representation)
        + [Consider ODCR first during Scheduling](#consider-odcr-first-during-scheduling)
        + [Adding ODCRs as Additional Instance Type Offerings](#adding-odcrs-as-additional-instance-type-offerings)
        + [Representing ODCR Available Instance Counts in Instance Type Offerings](#representing-odcr-available-instance-counts-in-instance-type-offerings)
    * [Capacity Reservation Expiration](#capacity-reservation-expiration)
    * [CloudProvider Launch Behavior](#cloudprovider-launch-behavior)
    * [Pricing/Consolidation](#pricingconsolidation)
        + [Provisioning](#provisioning)
        + [Consolidation](#consolidation)
            - [Consolidating into Capacity Reserved Instances](#consolidating-into-capacity-reserved-instances)
            - [Consolidating between Capacity Reservations](#consolidating-between-capacity-reservations)
    * [Drift](#drift)
        + [The NodePool selects on `karpenter.sh/capacity-type: reserved` but the instance is no longer in a reservation](#the-nodepool-selects-on-karpentershcapacity-type-reserved-but-the-instance-is-no-longer-in-a-reservation)
        + [The `capacityReservationSelectorTerms` no longer selects an instance's capacity reservation](#the-capacityreservationselectorterms-no-longer-selects-an-instances-capacity-reservation)
    * [Launch Failures](#launch-failures)
    * [Open Questions](#open-questions)
    * [Action Items](#action-items)
    * [Appendix](#appendix)
        + [Input/Output for CreateFleet with CapacityReservations](#inputoutput-for-createfleet-with-capacityreservations)

## Background

In AWS [ODCR](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-capacity-reservations.html) allows users to reserve compute capacity to mitigate the risk of getting on-demand capacity. This is very helpful during seasonal holidays where higher traffic is expected or for reserving highly-desired instance types, like the `p5.48xlarge` or other large GPU instance types.

### Capacity Reservations

Each [Capacity Reservation](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ec2@v1.162.1/types#CapacityReservation) is defined with:

- The Availability Zone in which to reserve the capacity
- The count of instances for which to reserve capacity
- The instance attributes, including the instance type, tenancy, and platform/OS
- Instance match criteria
  - Targeted -- only accept instances that matches all attributes + explicitly targeted the capacity reservation
  - Open -- if capacity reservation accepts all instances that matches all attributes
- A start and end date (if applicable) for when the reservation of capacity is available

AWS also supports grouping Capacity Reservation into [Capacity Reservation groups](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/create-cr-group.html). Both these entities are supported in Launch Template's CapacityReservationTarget [definitions](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-launchtemplate-capacityreservationtarget.html).

## Goals

1. Allow selection of targeted and open ODCRs with Karpenter 
2. Ensure multiple ODCRs (with different instance types and zones) can be selected from a single NodePool 
3. Ensure that we only launch capacity into an ODCR as-needed to ensure ODCR sharing between clusters and accounts
4. Ensure ODCRs are prioritized over regular OD and spot capacity 
5. Ensure Karpenter consolidates regular OD and spot instances to ODCR capacity when it is available 
6. Ensure Karpenter consolidates between ODCRs when a smaller/cheaper ODCR is available
7. Allow users to constrain a NodePool to only launch into ODCR capacity without fallback 
8. Allow users to fallback from ODCR to spot capacity and from ODCR to standard OD capacity 
9. Ensure OD capacity is not unnecessarily churned when a capacity reservation is removed to reduce workload disruption
10. Ensure open ODCRs outside of NodePool configuration are not selected automatically (no automatic optout of open ODCRs if not configured)

## Non-Goals

1. Ensure OD instances, created as fallback, can be automatically attached to an ODCR after that fact, if ODCR has availibility later (follow up needed)
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
      # The instance type for the Capacity Reservation
      instanceType: String | None
      # The id of the AWS account that owns the Capacity Reservation
      ownerID: String | None
      # Tags is a map of key/value tags used to select capacity reservations
      # Specifying '*' for a value selects all values for a given tag key.
      tags: Map | None
      # Indicates the type of instance launches that the Capacity Reservation accepts. The options include:
      #    - open:
      #       The Capacity Reservation accepts all instances that have
      #       matching attributes (instance type, platform, and Availability
      #       Zone). Instances that have matching attributes launch into the
      #       Capacity Reservation automatically without specifying any
      #       additional parameters.
      #    - targeted:
      #       The Capacity Reservation only accepts instances that
      #       have matching attributes (instance type, platform, and
      #       Availability Zone), and explicitly target the Capacity
      #       Reservation. This ensures that only permitted instances can use
      #             the reserved capacity.
      type: String | None
status:
  capacityReservations:
    - # AvailabilityZone for the Capacity Reservation
      availabilityZone: String
      # Available Instance Count for the Capacity Reservation
      availableInstanceCount: Integer
      # The time at which the Capacity Reservation expires. When a Capacity
      # Reservation expires, the reserved capacity is released and you can no longer
      # launch instances into it. The Capacity Reservation's state changes to expired
      # when it reaches its end date and time.
      endTime: String | None
      # id for the Capacity Reservation
      id: String
      # Indicates the type of instance launches that the Capacity Reservation accepts. The options include:
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
      type: String
      # Instance Type for the Capacity Reservation
      instanceType: String
      # The id of the AWS account that owns the Capacity Reservation
      ownerID: String
      # Total Instance Count for the Capacity Reservation
      totalInstanceCount: Integer
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

[TODO: Fill in a section on how we are going to prioritize ODCRs during provisioning]

### Adding ODCRs as Additional Instance Type Offerings

We can surface ODCR capacity as additional offerings attached to each instance type. Offerings currently allow us to track the pricing of variants of a specific instance type, primarily based on capacity type and availability zone today.

To track capacity reservation capacity, we can add additional offerings to an instance type when there is a capacity reservation that is matched on by an EC2NodeClass's `capacityReservationSelectorTerms`. This offering will have a price near 0 to model the fact that the reservation is already paid-for and to ensure the offering is prioritized ahead of other offerings. 

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

## Capacity Reservation Expiration

Capacity reservations [support an option to expire the reservation at a specific date and time](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/capacity-reservations-using.html). When the reservation expires, any instances present in the reservation at the time will have their association with the reservation removed and the instances will be charged at the standard on-demand instance rate.

When this occurs, we need to ensure two things in Karpenter:
1. We no longer attempt to launch instances into this reservation (e.g. we ensure the offering is removed from the set of capacity reservation offerings)
2. Instances which were marked as part of a capacity reservation have their label removed so we no longer treat their pricing as capacity reservation pricing

To ensure the first item, we can simply filter out non-active capacity reservations when storing capacity reservations in EC2NodeClass status. Ensuring the second item involves us polling the DescribeInstances API to validate the mapping between the capacity reservation and the instance, removing the mapping from the NodeClaim/Node if the instance is no longer in a reservation.

## CloudProvider Launch Behavior

EC2NodeClass currently supports automatically generating Launch Templates based on instance types and their AMI requirements. We are implementing a prioritization within Karpenter for Capacity Reservation (as their price will be set to 0 making it be selected over on-demand and spot). We add a check to ensure before calling CreateFleet API existing available Capacity Reservations are being used. In some race condition scenarios CreateFleet when creating instances against a single targeted Capacity Reservation will fail if the Reservation is fully utilized, resulting in an `ReservationCapacityExceeded` error. By default, CreateFleet will fallback to `on-demand` instances if using `open` ODCRs and using the `use-capacity-reservations-first` capacity reservation usage strategy.

To clarify, the NodeClass with Capacity Reservations will pin the NodeClass into all instance types and availability zones the Capacity Reservations reserved. If additionally on-demand is provided it can spin up other instance types, but can have unintended side effects during consolidation phase.

## Pricing/Consolidation

### Provisioning

Pricing is directly considered during provisioning and consolidation as capacity-reservation is prepaid. It is assumed to have a price of 0 during provisioning.

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

To solve for this edge case, we won't model the pricing of capacity reservations as `0` but as a "near-0" value. We'll divide the existing price of the on-demand offering by 1000 to represent a price that is less than every other instance type offering, but still maintains the relative ordering of on-demand instance types. 

In practice, this means that if a user has two capacity reservation offerings available: one for a `c6a.48xlarge` and another for a `c6a.large`, where we launch into the `c6a.48xlarge` first, we will still be able to consolidate down to the `c6a.large` when pods are scaled back down.

## Drift

Due to capacity reservation expiration or due to changes from the user in their capacity reservation selection in their EC2NodeClass, instances can drift from the NodePool capacity reservation specification. Below outlines two ways that this drift can occur and how it will be re-reconciled.

### The NodePool selects on `karpenter.sh/capacity-type: reserved` but the instance is no longer in a reservation

Assuming we have a reconciler that we will build as part of this design proposal that will update the `karpenter.sh/capacity-type` label from `reserved` to `on-demand` when a node is no longer in a capacity reservation, Karpenter's dynamic requirement drift checking should cause drift reconciliation in this case.

In this case, since the NodePool is selecting on a label that does not exist on the NodeClaim, drift detection will recognize that the NodeClaim is invalid and will mark it as drifted to be replaced by another node.

### The `capacityReservationSelectorTerms` no longer selects an instance's capacity reservation

In this case, there is no existing mechanism in Karpenter that would catch this. Karpenter will need to implement an additional mechanism that validates that an instance's capacity reservation falls within the valid set of reservations selected-on from the `capacityReservationSelectorTerms`. Specifically, it needs to validate that that id exists with the `capacityReservation` section of the EC2NodeClass status.

## Launch Failures

The main failure scenario is when Capacity Reservation limit is hit and no new nodes can be launched from any Capacity Reservation the launch template targets.

1. We filter inside Karpenter before calling CreateFleet API and throwing an InsufficientCapacityError causing a reevaluation, with then a retry recalculation of instances maybe falling back to regular on-demand
2. We call CreateFleet API in certain race conditions, resulting in an InsufficientCapacityError causing a reevaluation, with then a fallback to on-demand could be selected if Capacity Reservations not available

## Open Questions

1. How do we deal with selecting on an incredibly large array of capacity reservations that we would have to store in the NodeClass status? How do we ensure that this doesn't significantly bloat the output?
2. Do we introduce this feature with a `FEATURE_GATE=CapcityReservations`? This would give us the flexibility to change the implementation of the feature while it's still in alpha.
3. What's our default behavior for selecting capacityReservations when not specifying an owner? Does this select all capacityReservations -- including ones that are not owned by the account? _NOTE: This depends on what the flow is for sharing a capacityReservation. If there is an explicit acceptance criteria, then selecting on all regardless of owner should be fine_ 
4. How are we going to prioritize ODCR offerings when scheduling? These are effectively "0-cost" instances so we should prioritize them if the user selects on them.

## Action Items

- [ ] Make a note that we are going to be passing `none` to the capacity reservation preference during launch so that we don't have the option to match "open" when a user specifies `capacityReservationSelectorTerms`
- [ ] Add to the appendix section with CreateFleet examples
- [ ] Add to the section on prioritizing ODCR offerings during scheduling

## Appendix

### Input/Output for CreateFleet with CapacityReservations

[TODO: Fill in a section on the various results of calling CreateFleet with different ODCR inputs]