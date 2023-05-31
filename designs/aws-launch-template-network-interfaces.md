# AWS LaunchTemplate's NetworkInterfaces
AWS launchTemplates object enables the [configuration of network interfaces](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-launchtemplate-networkinterface.html). 
This gives the users flexibility they are currently lacking because Karpenter does not expose networkInterfaces in the `AWSNodeTemplate` API. here are some of the usecases requested:

* #2026 Associate a public IPv4 address to the network interface
* #3369 Create a node with multiple network interfaces
* #3369 create a node with EFA(Elastic Fabric Adapter) network interface

# challenges:
exposing networkInterfaces as part of the `AWSNodeTemplate` and using the data to configure AWS LaunchTemplate would solve the problem for the users. However, there are some challenges that we need to address:   

## subnet and security group resolution logic
currently security groups and subnets are part of the `AWSNodeTemplate`; and they are resolved from tags at run-time. This implies the resolved subnet and security group will be the same for all network interfaces.
this is counterintuitive as users expect to be able to configure different subnets and security groups for each network interface.
```yaml
apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
spec:
  subnetSelector:
      label: subnet-1 # users expect this to be used for all network interfaces
  securityGroupSelector:
      securityGroup: sg-1 # users expect this to be used for all network interfaces
  networkInterfaces:
  - subnetSelector:
        subnet: subnet-2 # users expect this to be used for this specific network interface
    securityGroupSelector:
        securityGroup: sg-2 # users expect this to be used for this specific network interface
```
having both options (`AWSNodeTemplate` level and networkInterface level) will complicate the API and will cause confusion for the users. It shall also complicate the logic on karpenter controller side.   
In addition, removing the `AWSNodeTemplate`'s level subnet and security group selectors is a breaking change from API backward compatibility perspective.

one more thing to consider is that subnets are bound to a single AZ. that's why the resolution happen in runtime instead of being part of the AWS launchTemplate (to avoid creating as many launch templates as the AZs in the zone).
moving the resolution logic to the networkInterface level has the following complications: 
1. more resolutions and interface allocations to do
2. NetworkInterfaces can have conflicting requirements (e.g. different subnets in different AZs). which we need to handle in runtime.

## networkInterfaces with specific IP addresses
networkInterfaces allows specifying specific IP addresses for the network interface. This launchTemplate with specific IP addresses is not very useful for karpenter as it can not create many nodes without IP address collisions.

## number of network interfaces per instance type
the [maximum number of network interfaces a node can support](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-eni.html#AvailableIpPerENI) differs based on the instance-type. This creates dependency between the `Provisioner` resource and the `AWSNodeTemplate`.
for example if the provisioner allows the `m5.large` and `m5.xlarge` instance types, while the `AWSNodeTemplate` specifies 4 network interfaces, karpenter controller needs to know that it can only provision `m5.xlarge` instances.

## node drift
there are requirements to reconcile nodes as they drift from the specs defined in the provisioner resource as defined [here](https://github.com/aws/karpenter/issues/1738) and [here](https://github.com/aws/karpenter/issues/1457).
the topic of having multiple network interfaces per node adds to the complexity of the drift detection logic.

# Proposed Solution:
iterative approach where we start by exposing subset of the fields of networkInterfaces and then expand the support based on the demand.
those fields are:
```yaml
  AssociateCarrierIpAddress: Boolean
  AssociatePublicIpAddress: Boolean
  DeleteOnTermination: Boolean
  Description: String
  DeviceIndex: Integer
  InterfaceType: String
  NetworkCardIndex: Integer
```
in addition, we will only support two networkInterfaces per node, this way we guarantee that the node will be able to run on all instance types.

subnet and securityGroup will remain part of the `AWSNodeTemplate` and will be used for both network interfaces.
in following iterations, as the `AWSNodeTemplate` move from `v1alpha1` to higher versions, we can add support for:

```yaml
subnetSelector: String
securityGroupSelector: String
```
this way we can support different subnets and security groups for each network interface, we can also remove the limit of having only 2 network interfaces per LaunchTemplate.
