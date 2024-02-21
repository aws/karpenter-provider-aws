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

## Proposed Solution

Add a new field `capacityReservationSpec` to `EC2NodeClass` 
```yaml
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
metadata:
  name: example-node-class
spec:
  capacityReservationSpec:
    capacityReservationPreference: open | None  # Cannot be defined if capacityReservationTarget is specified
    capacityReservationTarget: # Cannot be defined if capacityReservationPreference is specified
      capacityReservationId: String | None
      capacityReservationResourceGroupArn: String | None
```
This follows exactly how LaunchTemplate defines [CapacityReservationSpecification](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-launchtemplate-capacityreservationspecification.html) with the same
constraints. 

