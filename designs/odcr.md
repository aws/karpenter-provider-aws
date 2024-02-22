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
- Define Karpenter's behavior when interacting launching nodes into Capacity Reservation
- Define Karpenter's behavior when encountering errors when attempting to launch nodes into Capacity Reservation
- Define Karpenter's behavior when capacity reservation is changed

## Non-Goal
_We are keeping the scope of this design very targeted so even if these could be things we eventually support, we aren't scoping them into this design_
- Supporting prioritization when launching nodes 
- Supporting any pre-calcaluation of ODCR capacity utilization before node launch

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

When a node is launched against a CapacityReservation we will defer the handling of node launch to AWS' API. However we will expose Capacity Reservation
information as labels `karpenter.k8s.aws/capacity-reservation-id` and `karpenter.k8s.aws/capacity-reservation-setting`.

```yaml
Name:               example-node
Labels:             karpenter.k8s.aws/capacity-reservation-id=cr-12345
                    karpenter.k8s.aws/capacity-reservation-setting=open
                    karpenter.sh/capacity-type=on-demand
```

`karpenter.k8s.aws/capacity-reservation-id` will be the capacity reservation the node launched from. `karpenter.k8s.aws/capacity-reservation-setting` will depend on the launch template's `capacityReservationSpec`. It will either be a preference or a target.

Will will extract this information by updating [instance](https://github.com/aws/karpenter-provider-aws/blob/main/pkg/providers/instance/types.go#L29) and extract it from [DescribeInstance](https://github.com/aws/karpenter-provider-aws/blob/main/pkg/batcher/describeinstances.go#L48) [aws doc]([https://docs.aws.amazon.com/sdk-for-go/api/service/ec2/#EC2.DescribeInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CapacityReservationSpecificationResponse.html)https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CapacityReservationSpecificationResponse.html).

### Failed to launch Nodes into Capacity Reservation
