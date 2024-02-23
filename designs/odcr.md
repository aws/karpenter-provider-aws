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
  - If Capacity Reservation is targeted, only accept instances that matches all attributes + explicitly targeted the capacity reservation
  - Open if capacity reservation accepts all instances that matches all attributres

AWS also supports grouping Capacity Reservation into Capacity Reservation groups. 
Both these entities are supported in Launch Template's CapacityReservationTarget [definitions](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-launchtemplate-capacityreservationtarget.html).

## Goals

- Support associating ODCR to EC2NodeClass
- Define Karpenter's behavior when launching nodes into Capacity Reservation
- Define Karpenter's behavior when encountering errors when attempting to launch nodes into Capacity Reservation

## Non-Goals

_We are keeping the scope of this design very targeted so even if these could be things we eventually support, we aren't scoping them into this design_
- Supporting prioritization when launching nodes 
- Supporting any pre-calcaluation of ODCR capacity utilization before node launch
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

EC2NodeClass currently supports automatically generating Launch Templates based on instance types and their AMI requirements however as the first iteration of supporting Capacity Reservation, we won't introduce any prioritization nor fallback nor safeguard when launching nodes against EC2NodeClasses with a Capacity Reservation. This means that it is the
user's responsibility to ensure the Capacity Reservation associated to a EC2NodeClasses will include instance types the EC2NodeClasses could launch. Otherwise the EC2NodeClasses will fail during provisioning.

This also means that we will not allow a EC2NodeClass be able to create Spot instances when user specified a Capacity Reservation. 

We will skip [checkODFallback](https://github.com/aws/karpenter-provider-aws/blob/main/pkg/providers/instance/instance.go#L200C14-L200C29).

_Note that these aren't permanent restrictions but simply narrowing down what features exist in the first iteration of supporting Capacity Reservation_ 

Pros: 
- This puts the onus of checking on the user. It doesn't require checking if a Capacity Reservation satisfy the requirements of an instance type by assuming the users is doing this. It makes no assumption about what a Capacity Reservation can do nor implies can do. This is helpful because if there is expectation that Karpenter does this we may be asked to
support checking if there is capacity in the reservation before launching nodes which would significantly increase the scope of this design.

Cons:
- This implementation is overly restrictive causing it to be difficult to use. Its possible that the restrictions makes it too hard for users to use EC2NodeClasses effectively.

Both a pro and a con:
- This forces users who wish to leverage fallback and prioritization to use NodePool weights rather than relying on EC2NodeClass. I can't tell if this is a good or a bad thing but in my
  opinion it isn't always clear what Karpenter wants users to do in this aspect.

#### Labels

When a node is launched against a CapacityReservation we will expose Capacity Reservation
information as labels `karpenter.k8s.aws/capacity-reservation-id` and `karpenter.k8s.aws/capacity-reservation-setting`.

```yaml
Name:               example-node
Labels:             karpenter.k8s.aws/capacity-reservation-id=cr-12345
                    karpenter.k8s.aws/capacity-reservation-setting=open
                    karpenter.sh/capacity-type=on-demand
```

`karpenter.k8s.aws/capacity-reservation-id` will be the capacity reservation the node launched from. `karpenter.k8s.aws/capacity-reservation-setting` will depend on the launch template's `capacityReservationSpec`. It will either be a preference or a target.

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
The condition will reset if new nodes were able to launch and the Status will return to `Available`.

#### Error handling

We will avoid updating (unavailableOfferingCache)[https://github.com/aws/karpenter-provider-aws/blob/main/pkg/providers/instance/instance.go#L239C41-L239C58] because the pool is different than rest of AWS. However we may want to create a new unavailable offering cache keyed against Capacity Reservations. _Not sure if we want to support to this during the first iteration_ 

## Open Questions
- The UX of adding Capacity Reservation feels wrong because NodeClasses previously didn't fully restrict instance types but with Capacity Reservation it kind of does. There isn't a good primative in Karpenter to expose these kinds of restrictions (I think?) specifically around preflight or static analysis that tells you your NodePool may not be able to launch any nodes because of X, Y and Z in your EC2NodeClass. I believe this is already an issue where if a NodeClass selects for x86 architecture AMIs but the NodePool allows for ARM architecture instance types that Karpenter may just quietly never spawn ARM instances?
