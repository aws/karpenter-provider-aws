# On demand capacity reservation

This document proposes supporting ODCR in Karpenter

- [On demand capacity reservation](#on-demand-capacity-reservation)
  - [Background](#background)
    - [Capacity Reservations](#capacity-reservations)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
  - [Proposed Solution](#proposed-solution)
    - [Supporting associating Capacity Reservations to EC2NodeClass](#supporting-associating-capacity-reservations-to-ec2nodeclass)
    - [Supporting new capacity-type "capacity-reservation" for NodePool](#supporting-new-capacity-type-capacity-reservation-for-nodepool)
      - [only allow capacity-reservations](#only-allow-capacity-reservations)
      - [capacity-reservation-fleet (falling back to on-demand if capacity-reservation not available)](#capacity-reservation-fleet-falling-back-to-on-demand-if-capacity-reservation-not-available)
    - [Launching Nodes into Capacity Reservation](#launching-nodes-into-capacity-reservation)
      - [All Launch Templates are associated with the specified Capacity Reservation](#all-launch-templates-are-associated-with-the-specified-capacity-reservation)
      - [Pricing and consolidation](#pricing-and-consolidation)
        - [Provisioning](#provisioning)
        - [Consolidating Capacity Reserved Instances](#consolidating-capacity-reserved-instances)
        - [Consolidating into Capacity Reserved Instances](#consolidating-into-capacity-reserved-instances)
      - [Labels](#labels)
    - [Failed to launch Nodes into Capacity Reservation](#failed-to-launch-nodes-into-capacity-reservation)

## Background

In AWS [ODCR](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-capacity-reservations.html) allows users to reserve compute capacity to mitigate the risk of 
getting on-demand capacity. This is very helpful during seasonal holidays where higher traffic is expected or for reserving highly-desired instance types, like the
`p5.48xlarge` or other large GPU instance types.

### Capacity Reservations

Each [Capacity Reservation](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ec2@v1.162.1/types#CapacityReservation) is defined with:

- The Availability Zone in which to reserve the capacity
- The number of instances for which to reserve capacity
- The instance attributes, including the instance type, tenancy, and platform/OS
- Instance match criteria
  - Targeted -- only accept instances that matches all attributes + explicitly targeted the capacity reservation
  - Open -- if capacity reservation accepts all instances that matches all attributes
- A start and end date (if applicable) for when the reservation of capacity is available

AWS also supports grouping Capacity Reservation into Capacity Reservation groups. 
Both these entities are supported in Launch Template's CapacityReservationTarget [definitions](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-launchtemplate-capacityreservationtarget.html).

## Goals

- Support associating targeted ODCRs to EC2NodeClass
- Define Karpenter's behavior when using new Capacity Type "capacity-reservation" in NodePool
- Define Karpenter's behavior when encountering errors when attempting to launch nodes into Capacity Reservation
- Define Karpenter's behavior when consolidating nodes to Capacity Reservation when available

## Non-Goals

- Supporting open Capacity Reservations. _The behaviour of indirectly linked nodes to an open ODCR can cause rotation, adding unnecessary node rotation into the cluster_
- Supporting capacity-blocks as a capacity-type.
- Supporting changes in scaling behavior when ODCR is associated to a NodeClass. _We won't bring up N nodes to match an N node capacity reservation, this would directly interfear with the ability to share a Capacity Reservation between multiple clusters or accounts. The first Karpenter finding the Capacity Reservation would provision all instances for a reservation leaving nothing unused_
- Supporting Capacity Reservation Groups. _Adding this abstraction for now adds additional complexity_
- Supporting updating Capacity Reservations availabile instance count after success of CreateFleet API. _We will currenlty rely on reconsolidation of capacity reservations itself, and rely on CreateFleet API throwing a `ReservationCapacityExceeded`_
- Supporting conditions for capacity reservation instance creations. _By showing the available instance count within each capacity reservation for an EC2NodeClass we have similar information available_

## Proposed Solution

### Supporting associating Capacity Reservations to EC2NodeClass

- Add a new field under `spec` for `capacityReservationSelectorTerms` to `EC2NodeClass` for defining which Capacity Reservation to be used for a specific `EC2NodeClass`
  - This will allow us to attach multiple Capacity Reservations across AZs and Instance Types to a single EC2NodeClass. This capability removes the need for Capacity Reservation Groups for this MVP.
- Add a new field under `status` for the found Capacity Reservations by the `spec.capacityReservationSelectorTerms` for the `EC2NodeClass`

```yaml
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
metadata:
  name: example-node-class
spec:
  capacityReservationSelectorTerms:
    - # The Availability Zone of the Capacity Reservation
      availabilityZone: String | None
      # The platform of operating system for which the Capacity Reservation reserves capacity
      id: String | None
      # The type of operating system for which the Capacity Reservation reserves capacity
      instancePlatform: String | None
      # The instance type for which the Capacity Reservation reserves capacity
      instanceType: String | None
      # The ID of the Amazon Web Services account that owns the Capacity Reservation
      ownerId: String | None
      # Tags is a map of key/value tags used to Capacity Reservations
      # Specifying '*' for a value selects all values for a given tag key.
      tags: Map | None
      # Indicates the tenancy of the Capacity Reservation.
      # A Capacity Reservation can have one of the following tenancy 'default' or 'dedicated':
      #   default - The Capacity Reservation is created on hardware that is shared with other Amazon Web Services accounts.
      #   dedicated - The Capacity Reservation is created on single-tenant hardware that is dedicated to a single Amazon Web Services account.
      tenancy: String | None
status:
  capacityReservations:
    - # Availability Zone of the Capacity Reservation
      availabilityZone: String
      # Available Instance Count of the Capacity Reservation
      availableInstanceCount: Integer
      # End date of the Capacity Reservation
      endDate: Date
      # ID of the Capacity Reservation
      id: String
      # Instance Type of the Capacity Reservation
      instanceType: String
      # Instance Match Criteria of the Capacity Reservation (open | targeted)
      instanceMatchCriteria: String
      # Owner ID of the Capacity Reservation
      ownerId: String
	    # Placement Group Arn of the Capacity Reservation
      placementGroupArn: String | None
      # Reservation Type of the Capacity Reservation (default | capacity-block)
      reservationType: String
      # Start date of the Capacity Reservation
      startDate: Date
      # Tags attached to Capacity Reservation
      tags: Map
      # Total Instance Count of the Capacity Reservation
      totalInstanceCount: Integer
```

This follows closely (does not implement all fields) how EC2 [DescribeCapacityReservations](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeCapacityReservations.html) can filter.

Karpenter will perform validation against the spec to ensure there isn't any violation prior to creating the LaunchTemplates.

### Supporting new capacity-type "capacity-reservation" for NodePool

Adding a new `karpenter.sh/capacity-type: capacity-reservation` allows us to have a EC2NodeClass that does not automatically fallback to `on-demand` if `capacity-reservation` is not available.

#### only allow capacity-reservations

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
spec:
  template:
    spec:
      requirements:
      - key: karpenter.sh/capacity-type
        operator: In
        values:
        - capacity-reservation
```

#### capacity-reservation-fleet (falling back to on-demand if capacity-reservation not available)

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
spec:
  template:
    spec:
      requirements:
      - key: karpenter.sh/capacity-type
        operator: In
        values:
        - capacity-reservation
        - on-demand
```

### Launching Nodes into Capacity Reservation

#### All Launch Templates are associated with the specified Capacity Reservation

EC2NodeClass currently supports automatically generating Launch Templates based on instance types and their AMI requirements. We are implementing a prioritization within Karpenter for Capacity Reservation (as their price will be set to 0 making it be selected over on-demand and spot). We add a check to ensure before calling CreateFleet API existing available Capacity Reservations are being used. In some raise condition scenarios CreateFleet when creating instances against a single targeted Capacity Reservation will fail if the Reservation is fully utilized, resulting in an `ReservationCapacityExceeded` error.
By default it will not fallback to `on-demand` instances, if not specified specifically in the capacity-type in the NodePool as described above [capacity-reservation-fleet (falling back to on-demand if capacity-reservation not available)](#capacity-reservation-fleet-falling-back-to-on-demand-if-capacity-reservation-not-available).

To clarify the NodeClass with Capacity Reservations will pin the NodeClass into all instance types and availability zones the Capacity Reservations reserved, if additionally on-demand is provided it can spin up other instance types, but can have unintended side effects during consolidation phase.

_Note that these aren't permanent restrictions but simply narrowing down what features exist in the first iteration of supporting Capacity Reservation_ 

Pros: 
- This makes the user responsible for verifying the capability of their NodeClasses and requirements. It makes no assumption about what a Capacity Reservation can or cannot do
- Users are still open to define fallbacks of on-demand or spot, and its up them of how they would like to configure it. Making it possible to either define everything with one EC2NodeClass and one NodePool, or using multiple with weights

Cons:
- This implementation is overly restrictive and will be difficult to use when attempting to utilize multiple Instance Types. Because ODCRs are such a specific use case, it can be argued that the flexibility of multiple Instance Types can cause issues with cost savings

#### Pricing and consolidation

##### Provisioning

Pricing is directly considered during provisioning and consolidation as capacity-reservation is prepaid. It is assumed to have a price of 0 during provisioning.

##### Consolidating Capacity Reserved Instances

During consolidation pricing does matter as it affects which candidate will be [prioritized](https://github.com/kubernetes-sigs/karpenter/blob/75826eb51589e546fffb594bfefa91f3850e6c82/pkg/controllers/disruption/consolidation.go#L156). Since all capacity instances are paid ahead of time, their cost is already incurred. Users would likely want to prioritize filling their reserved capacity.
reservation first then fall back into other instances. Because of this reserved instances should likely show up as 0 dollar pricing when we calculate the instance pricing. Since each candidate is tied to a NodePool and EC2NodeClass, we should be able to safely override the pricing per node under a capacity reservation.

##### Consolidating into Capacity Reserved Instances

If we track Capacity Reservation usage, we can optimize the cluster configuration by moving non-Capacity Reserved instances into 
Capacity Reserved instances. We would need to match the instance type, platform and availability zone prior to doing this.

#### Labels

When a node is launched against a CapacityReservation we will expose Capacity Reservation
information as labels `karpenter.k8s.aws/capacity-reservation-id`. This is helpful for users to identify those nodes that are being used
by a capacity reservation and the id of the reservation. Scenarios such as tracking how many nodes are under each capacity reservation and then
checking how close to limit of the reservation.

```yaml
Name:               example-node
Labels:             karpenter.k8s.aws/capacity-reservation-id=cr-12345
                    karpenter.sh/capacity-type=capacity-reservation
```

`karpenter.k8s.aws/capacity-reservation-id` will be the capacity reservation the node launched from. 

We will propagate this information via [instance](https://github.com/aws/karpenter-provider-aws/blob/main/pkg/providers/instance/types.go#L29) by extracting it from [DescribeInstance](https://github.com/aws/karpenter-provider-aws/blob/main/pkg/batcher/describeinstances.go#L48) [aws doc]([https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#EC2.DescribeInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CapacityReservationSpecificationResponse.html)https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CapacityReservationSpecificationResponse.html).

### Failed to launch Nodes into Capacity Reservation

The main failure scenario is when Capacity Reservation limit is hit and no new nodes can be launched from any Capacity Reservation the launch template targets.

1. We filter inside Karpenter before calling CreateFleet API and throwing an InsufficientCapacityError causing a reevaluation,
with then a different capacity-type could be selected if available in NodePool

1. We call CreateFleet API in certain raise conditions, resulting in an InsufficientCapacityError causing a reevaluation,
with then a different capacity-type could be selected if available in NodePool
