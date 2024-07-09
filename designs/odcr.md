# On demand capacity reservation

This document proposes supporting ODCR in Karpenter

- [On demand capacity reservation](#on-demand-capacity-reservation)
  - [Background](#background)
    - [Capacity Reservations](#capacity-reservations)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
  - [Proposed Solution](#proposed-solution)
    - [Supporting associating Capacity Reservations to EC2NodeClass](#supporting-associating-capacity-reservations-to-ec2nodeclass)
    - [Adding new label `karpenter.k8s.aws/capacity-reservation-id` nodeClaim/node](#adding-new-label-karpenterk8sawscapacity-reservation-id-nodeclaimnode)
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

## Non-Goals

1. Support [Capacity Blocks](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/capacity-blocks-using.html) as a capacity-type -- though capacity blocks are not supported with this design, they are a natural extension of it. We could support selection on capacity blocks through the `capacityReservationSelectorTerms`.
2. Support [Capacity Reservation Groups](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/create-cr-group.html) -- though capacity reservation groups are not supported with this design, they are a natural extension of it. We could support an additional field `reservationGroup` in the `capacityReservationSelectorTerms`.

## Proposed Solution

## Capacity Reservation Selection

### EC2NodeClass API

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
      instanceType: String | None
      # The ID of the Amazon Web Services account that owns the Capacity Reservation
      ownerId: String | None
      # Tags is a map of key/value tags used to select subnets
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
    - # AvailabilityZone of the Capacity Reservation
      availabilityZone: String
      # Available Instance Count of the Capacity Reservation
      availableInstanceCount: Integer
      # The time at which the Capacity Reservation expires. When a Capacity
      # Reservation expires, the reserved capacity is released and you can no longer
      # launch instances into it. The Capacity Reservation's state changes to expired
      # when it reaches its end date and time.
      endTime: String | None
      # ID of the Capacity Reservation
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
      # Instance Type of the Capacity Reservation
      instanceType: String
      # Owner Id of the Capacity Reservation
      ownerId: String
      # Total Instance Count of the Capacity Reservation
      totalInstanceCount: Integer
```

This API follows closely with how [DescribeCapacityReservations](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeCapacityReservations.html) can filter capacity reservations -- allowing Karpenter to receive the server-side filtered version of the capacity reservations to store in its status.

### NodePool API

The EC2NodeClass API allows selection on capacity reservations, which give additional options to the scheduler to choose from when launching instance types; however, it does not offer a mechanism to scope-down whether instances in a NodePool should only launch into an ODCR, fallback between a capacity reservation to on-demand if none is available, or fallback between a capacity reservation to spot and then finally to on-demand. There are effectively 3 options we could take to allow this kind of selection:

1. Constrain a NodePool to only launch ODCR capacity when it references an EC2NodeClass that has `capacityReservationSelectorTerms`. Fallback to OD and to spot requires additional NodePools of lower weights
2. Allow a NodePool to launch all types of capacity when `capacityReservationSelectorTerms` are provided, but create an additional `karpenter.k8s.aws/capacity-type` called `reservation`. A cluster admin could then select to support only launching ODCR capacity and falling back between ODCR capacity to on-demand capacity respectively. _NOTE: This option requires any applications (pods) that are using node selection on `karpenter.k8s.aws/capacity-type: "on-demand"` to expand their selection to include `reservation`_

   a. Only launch ODCR instances
   ```yaml
   apiVersion: karpenter.sh/v1
   kind: NodePool
   metadata:
     name: default
   spec:
     requirements:
     - key: karpenter.k8s.aws/capacity-type
       operator: In
       values: ["reserved"]
   ```
   b. Launch ODCR instances with on-demand fallback
   ```yaml
   apiVersion: karpenter.sh/v1
   kind: NodePool
   metadata:
     name: default
   spec:
     requirements:
       - key: karpenter.k8s.aws/capacity-type
         operator: In
         values: ["on-demand", "reserved"]
   ```
   c. Launch ODCR instances with spot and on-demand fallback
   ```yaml
   apiVersion: karpenter.sh/v1
   kind: NodePool
   metadata:
     name: default
   spec:
     # No additional requirements needed, launch all capacity types by default
     requirements: []
   ```
3. [Recommended] Allow a NodePool to launch all types of capacity when `capacityReservationSelectorTerms` are provided, but create an additional `karpenter.k8s.aws/capacity-reservation-id`. A cluster admin could then select to support only launching ODCR capacity and falling back between ODCR capacity to on-demand capacity respectively. Unlike Option 2, this does not require applications (pods) that are selecting on `karpenter.k8s.aws/capacity-type: on-demand` to change anything about their selection.

   a. Only launch ODCR instances
   ```yaml
   apiVersion: karpenter.sh/v1
   kind: NodePool
   metadata:
     name: default
   spec:
     requirements:
     - key: karpenter.k8s.aws/capacity-type
       operator: In
       values: ["on-demand"]
     - key: karpenter.k8s.aws/capacity-reservation-id
       operator: Exists
   ```
   b. Launch ODCR instances with on-demand fallback
   ```yaml
   apiVersion: karpenter.sh/v1
   kind: NodePool
   metadata:
     name: default
   spec:
     requirements:
       - key: karpenter.k8s.aws/capacity-type
         operator: In
         values: ["on-demand"]
   ```
   c. Launch ODCR instances with spot and on-demand fallback
   ```yaml
   apiVersion: karpenter.sh/v1
   kind: NodePool
   metadata:
     name: default
   spec:
     # No additional requirements needed, launch all capacity types by default
     requirements: []
   ```
   
We are recommending Option 3. Notably, using capacity reservations is not an application owner concern -- app owners only care that they get the capacity they need to run their applications, they don't care how that capacity is acquired. This option does not require application owners to intervene to begin leveraging capacity reservations and allows cluster admins to describe the constraints necessary to ensure capacity availability for harder-to-get instance types.

### Scheduling Representation

Since ODCRs are a AWS-specific concept, there needs to be a mechanism to pass down these ODCR options down for the scheduler to reason about. Importantly, we need the scheduler to know to prioritize these ODCR options when a user has specified them in their EC2NodeClass. Further, we need the scheduler to be aware that it can't launch an unlimited amount of these instances into an ODCR, but has a limited number before it must begin trying other options.

#### Adding ODCRs as Additional Instance Type Offerings


#### Representing ODCR Available Instance Counts in Instance Type Offerings

### Launching Nodes into Capacity Reservation

#### All Launch Templates are associated with the specified Capacity Reservation

EC2NodeClass currently supports automatically generating Launch Templates based on instance types and their AMI requirements. We are implementing a prioritization within Karpenter for Capacity Reservation (as their price will be set to 0 making it be selected over on-demand and spot). We add a check to ensure before calling CreateFleet API existing available Capacity Reservations are being used. In some raise condition scenarios CreateFleet when creating instances against a single targeted Capacity Reservation will fail if the Reservation is fully utilized, resulting in an `ReservationCapacityExceeded` error.
By default it will fallback to `on-demand` instances.

To clarify the NodeClass with Capacity Reservations will pin the NodeClass into all instance types and availability zones the Capacity Reservations reserved, if additionally on-demand is provided it can spin up other instance types, but can have unintended side effects during consolidation phase.

_Note that these aren't permanent restrictions but simply narrowing down what features exist in the first iteration of supporting Capacity Reservation_ 

Pros: 
- Management of ODCRs usage is in hands of administrators (who are creating/updating EC2NodeClass configurations)
  - Application developers are unaware and can use the existing `on-demand` and `spot` capacity types

Cons:
- ***TBD***

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
                    karpenter.sh/capacity-type=on-demand
```

`karpenter.k8s.aws/capacity-reservation-id` will be the capacity reservation the node launched from. 

We will propagate this information via [instance](https://github.com/aws/karpenter-provider-aws/blob/main/pkg/providers/instance/types.go#L29) by extracting it from [DescribeInstance](https://github.com/aws/karpenter-provider-aws/blob/main/pkg/batcher/describeinstances.go#L48) [aws doc]([https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#EC2.DescribeInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CapacityReservationSpecificationResponse.html)https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CapacityReservationSpecificationResponse.html).

### Failed to launch Nodes into Capacity Reservation

The main failure scenario is when Capacity Reservation limit is hit and no new nodes can be launched from any Capacity Reservation the launch template targets.

1. We filter inside Karpenter before calling CreateFleet API and throwing an InsufficientCapacityError causing a reevaluation,
with then a retry recalculation of instances maybe falling back to regular on-demand

1. We call CreateFleet API in certain raise conditions, resulting in an InsufficientCapacityError causing a reevaluation,
with then a fallback to on-demand could be selected if Capacity Reservations not available

## FAQ

### What happens to instances when an ODCR expires?

### What happens when a user changes their `capacityReservationSelectorTerms` and some instances are no longer in a selected ODCR?

### How do we avoid over-launching instances 

## Appendix

### Input/Output for CreateFleet with CapacityReservations

[TODO: Fill in a section on the various results of calling CreateFleet with different ODCR inputs]