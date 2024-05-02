# On demand capacity reservation

This documents proposes supporting ODCR in Karpenter

## Background

In AWS [ODCR](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-capacity-reservations.html) allows users to reserve compute capacity to mitigate against the risk of being 
unabled to get on demand capacity. This is very helpful during seasonal holidays where higher traffic is expected. 

### Capacity Reservations

Each Capacity Reservation is defined with:

- The Availability Zone in which to reserve the capacity
- The number of instances for which to reserve capacity
- The instance attributes, including the instance type, tenancy, and platform/OS
- Instance match criteria
  - Targeted -- only accept instances that matches all attributes + explicitly targeted the capacity reservation
  - Open -- if capacity reservation accepts all instances that matches all attributes

AWS also supports grouping Capacity Reservation into Capacity Reservation groups. 
Both these entities are supported in Launch Template's CapacityReservationTarget [definitions](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-launchtemplate-capacityreservationtarget.html).

## Goals

- Support associating ODCR to EC2NodeClass
- Define Karpenter's behavior when launching nodes into Capacity Reservation
- Define Karpenter's behavior when encountering errors when attempting to launch nodes into Capacity Reservation
- Define Karpenter's behavior when consolidating nodes in Capacit Reservation

## Non-Goals

_We are keeping the scope of this design very targeted so even if these could be things we eventually support, we aren't scoping them into this design_
- Supporting prioritization when launching nodes. We will delegate this to weights within NodePools and [default behavior](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-fleet-on-demand-backup.html#ec2-fleet-on-demand-capacity-reservations) of the CreateFleet API. 
- Supporting changes in scaling behavior when ODCR is associated to a NodeClass. _We won't bring up N nodes to match an N node capacity reservation_

## Proposed Solution

### Supporting associating ODCR to EC2NodeClass

Add a new field `capacityReservationSpec` to `EC2NodeClass` 
```yaml
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
metadata:
  name: example-node-class
spec:
  capacityReservationSpec:
    capacityReservationPreference: open | none | None  # Cannot be defined if capacityReservationTarget is specified
    capacityReservationTarget: # Cannot be defined if capacityReservationPreference is specified
      capacityReservationId: String | None
      capacityReservationResourceGroupArn: String | None
```
This follows exactly how LaunchTemplate defines [CapacityReservationSpecification](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-launchtemplate-capacityreservationspecification.html) with the same
constraints. 

Karpenter will perform validation against the spec to ensure there isn't any violation prior to creating the LaunchTemplates.

### Launching Nodes into Capacity Reservation

#### All Launch Templates are associated with the specified Capacity Reservation

EC2NodeClass currently supports automatically generating Launch Templates based on instance types and their AMI requirements. We won't implement any prioritization within Karpenter for Capacity Reservation and instead rely on existing behavior of CreateFleet and Capacity Reservation. Specifically, CreateFleet when creating instances against a single targeted Capacity Reservation will fail if the Reservation is fully utilized.
However in the case where a Capacity Reservation Group is used it will fall back into On Demand nodes when the Reservation Group limit is hit. 

We won't introduce any other prioritization nor fallback when launching nodes against EC2NodeClasses with a Capacity Reservation. This means that it is the user's responsibility to ensure the Capacity Reservation associated to a EC2NodeClasses will include instance types, availability zone and platform the EC2NodeClasses should launch.
This also means that we will not allow an EC2NodeClass to create Spot instances when user specified a Capacity Reservation. 

Because of this this we will skip [checkODFallback](https://github.com/aws/karpenter-provider-aws/blob/main/pkg/providers/instance/instance.go#L200C14-L200C29) during processing.

In addition, the NodeClass with Capacity Reservations will pin the NodeClass into all instance types and availability zones the Capacity Reservations reserved.

_Note that these aren't permanent restrictions but simply narrowing down what features exist in the first iteration of supporting Capacity Reservation_ 

Pros: 
- This puts the onus of checking on the user to verify capatbility of their NodeClasses and requirements. It makes no assumption about what a Capacity Reservation can do nor implies can't do (Open to checking if Capacity is at limit prior to calling CreateFleet).
- This forces users who wish to leverage fallback and prioritization to use NodePool weights or Capacity Reservation Group rather than relying on EC2NodeClass.

Cons:
- This implementation is overly restrictive causing it to be difficult to use. Its possible that the restrictions makes it too difficult for users to use EC2NodeClasses effectively.

#### Pricing and consolidation


##### Provisioning

Pricing isn't directly(?) considered during provisioning, the logic is offloaded to CreateFleet. This will remain the same as Capacity Reservation will either be used and fails, if limit is hit, or it will fall back to on demand nodes that CreateFleet chooses based on on demand allocation strategy. 

##### Consolidating Capacity Reserved Instances

During consolidation pricing does matter as it affects which candidate will be [prioritized](https://github.com/kubernetes-sigs/karpenter/blob/75826eb51589e546fffb594bfefa91f3850e6c82/pkg/controllers/disruption/consolidation.go#L156). Since all capacity instances are paid ahead of time, their cost is already incurred. Users would likely want to prioritize filling their capacition
reservation first then fall back into other instances. Because of this reserved instances should likely show up as 0 dollar pricing when we calculate the instance pricing. Since each candidate is tied to a NodePool and NodeClass, we should be able to safely override the pricing per node under a capacity reservation.

##### Consolidating into Capacity Reserved Instances

If we track Capacity Reservation usage, we can optimize the cluster configuration by moving non-Capacity Reserved instances into 
Capacity Reserved instances. We would need to match the instance type, platform and availability zone prior to doing this. In addition, I am wondering if it make sense as a follow up rather than the first iteration of the implementation. I believe this 
likely deserves to be a separate controller given the complexity of consolidation

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

#### Status

We will expose this information both in the NodeClaim's, EC2NodeClass' and NodePool's Condition Status. _I know that currently NodeClasses and NodePool status do not have conditions but I wanted to see if we are opened to adding Conditions to these resources_

```yaml
Status:
  Conditions:
    Last Transition Time:  2024-02-22T03:46:42Z
    Status:                LimitExceeded
    Type:                  CapacityReservation
```
The condition will reset if new nodes were able to launch and the Status will return to `Available`. It may also be helpful to show the
utilization of the capacity if we are interested in perform consolidation into Capacity Reserved instances

#### Error handling

We will avoid updating (unavailableOfferingCache)[https://github.com/aws/karpenter-provider-aws/blob/main/pkg/providers/instance/instance.go#L239C41-L239C58] because the pool is different than rest of AWS. However we may want to create a new unavailable offering cache keyed against Capacity Reservations. _Not sure if we want to support to this during the first iteration_ 

## Open Questions
- The UX of adding Capacity Reservation feels wrong because NodeClasses previously didn't fully restrict instance types but with Capacity Reservation it kind of does. There isn't a good primative in Karpenter to expose these kinds of restrictions (I think?) specifically around preflight or static analysis that tells you your NodePool may not be able to launch any nodes because of X, Y and Z in your EC2NodeClass. I believe this is already an issue where if a NodeClass selects for x86 architecture AMIs but the NodePool allows for ARM architecture instance types that Karpenter may just quietly never spawn ARM instances?
