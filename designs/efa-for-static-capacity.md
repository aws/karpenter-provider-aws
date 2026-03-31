# EFA Support For Static Capacity

This document proposes adding support for launch static capacity with EFA devices in Karpenter

- [EFA Support For Static Capacity](#efa-support-for-static-capacity)
    * [Overview](#overview)
        + [EFA / EFA-only Network Interfaces](#efa--efa-only-network-interfaces)
        + [Customer Use Cases](#customer-use-cases)
    * [Goals](#goals)
    * [Non-Goals](#non-goals)
    * [Network Interface Configuration](#network-interface-configuration)
        + [EC2NodeClass API](#ec2nodeclass-api)
    * [Scheduling and Launch Behavior](#scheduling-and-launch-behavior)
        + [Network Interface Configuration and Instance Filtering](#network-interface-configuration-and-instance-filtering)
        + [EFA Resource Request Handling](#efa-resource-request-handling)
        + [Max Pods Calculation](#max-pods-calculation)
        + [IP-Related Launch Template Inputs](#ip-related-launch-template-inputs)
    * [Drift](#drift)
    * [Appendix](#appendix)
        + [Other Design Considerations - InterfacePolicy](#other-design-considerations---interfacepolicy)
        + [EFA Count Label](#efa-count-label)

## Overview
In AWS [EFA](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/efa.html) is critical for Machine Learning training and High Performance Computing workloads requiring high-performance inter-node communication. Currently Karpenter provides a way to configure EFA launches with dynamic capacity, through `vpc.amazonaws.com/efa` requests on pods, but no way to do so for static capacity.

This RFC outlines the proposed API and implementation of support for configuring EFA devices with Static Capacity. The proposed API and implementation supports users configuring network interface configuration in their EC2NodeClass, allowing users to declaratively specify network interface configurations including EFA interfaces.

### EFA / EFA-only Network Interfaces

The EFA device enables low-latency, high-throughput communication by providing build-in OS bypass and congestion control through the Scalable Reliable Datagram (SRD) protocol. It provides better application performance for HPC and ML workloads, e.g. NCCL and MPI, by enabling packets to be put directly on the network.

AWS EC2 instances support three types of network interfaces for high-performance networking:

- __ENA (Elastic Network Adapter)__: Standard network interface providing IP connectivity
- __EFA (Elastic Fabric Adapter)__: Network interface that provides both the EFA device for RDMA communication and ENA device for standard IP traffic
- __EFA-only__: Network interface that provides only the EFA device for RDMA communication, without consuming IP addresses

Each instance's network interfaces are organized into network cards (NC), with each card supporting multiple device indices (DI). Both EFA and EFA-only interfaces provide the EFA device. EFA interfaces, in addition to providing the EFA device, provide the ENA device for IP networking and therefore use an IP address. Instead of using EFA interfaces, the recommended configuration pattern is to use ENA on one device index and EFA-only on others.

### Customer Use Cases

This design supports three primary customer use cases, each requiring different network interface configurations:

__1. Maximum EFA + IP Bandwidth (Bandwidth Optimized)__

- Maximizes both EFA bandwidth for inter-node communication and IP bandwidth.
- Configuration combines ENA and EFA-only interfaces depending on instance type. For example, with the p5.48xlarge instance the resulting configuration is ENA and EFA-only interfaces on every 4 network cards (i.e. NC 0, 4, 8, .., 28) and only EFA-only interfaces on all other network cards.

__2. Minimize IP Consumption (IP Optimized)__

- Minimizes IP address consumption with the tradeoff that IP bandwidth decreases.
- Each EFA / ENA interface consumes an IP address, so the resulting configuration is ENA and EFA-only on the primary network card and EFA-only on all cards. For example with the p5.48xlarge instance the resulting configuration is ENA and EFA-only interface on network card 0 and the EFA-only interface on all other network cards.

__3. Specialized GPU Considerations__

- Certain GPU instances (e.g., P6e-GB200) have unique networking setups / bandwidth constraints, and depending on the checkpointing requirements this can result in unique configurations.

For more information, see [AWS EFA docs](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/efa-acc-inst-types.html).

## Goals

1. Enable EFA support for static nodepools.
2. Support the three use cases: bandwidth-optimized, IP-optimized, and specialized configurations

## Non-Goals

Below lists the non-goals for _this RFC design._ Each of these items represents potential follow-ups for the initial implementation and are features we will consider based on feature requests.

1. __Support delayed node initialization from the extended EFA resource__ - Currently, with dynamic EFA provisioning via `vpc.amazonaws.com/efa` resource requests, node initialization waits until the EFA plugin registers the EFA resource (preventing `ResourceNotRegistered` errors). With static NodeClass configurations, the EFA resource is not injected into the NodeClaim as a resource requirement, so nodes will initialize even if the EFA plugin doesn't register the external resource. Note that there is a path forward through extending provider registration hooks to support initialization delays, this is out of scope for this initial design.

2. __Support EFA interface type in network interface configuration__ - This design will only support ENA and EFA-only interface types. We have not identified a use case for EFA interface type over the recommended pattern of configuring ENA on one device index and EFA-only on the other.

## Network Interface Configuration

### EC2NodeClass API

- Add a new struct under `spec` for `networkInterfaces` to `EC2NodeClass` for defining the network interface configuration to be used for instance launches.

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: example-node-class
spec:
  # NetworkInterfaces specifies network interface configurations to be 
  # attached to provisioned instances.
  # - NetworkCardIndex is the index of the network card to attach the interface to.
  # - InterfaceType is the type of network interface. Valid values are "interface" and "efa-only".
  # - DeviceIndex is the device index for the network interface attachment.
  networkInterfaces:
  - networkCardIndex: int
    deviceIndex: int
    interfaceType: interface | efa-only
  # CEL validation ensures the primary network interface must be configured with interfaceType "interface" (or ENA).
  # CEL validation also ensures that there are no duplicate network card and device index configurations.
  # CEL validation also ensures that both deviceIndex and networkCardIndex are non-negative.
```

This API closely follows how [NetworkInterfaces](https://docs.aws.amazon.com/AWSCloudFormation/latest/TemplateReference/aws-properties-ec2-launchtemplate-networkinterface.html) in the launch template are configured.

## Scheduling and Launch Behavior

### Network Interface Configuration and Instance Filtering

When network interfaces are configured on the EC2NodeClass, any instances launched will be launched with that configuration translated into the launch template. It's possible that not all instance types allowed by the linked NodePool will be able to be launched with the configuration specified in the EC2NodeClass. Karpenter will filter out incompatible instances from instance launch options and during EC2NodeClass validation.

### EFA Resource Request Handling

If a pod requests the EFA resource `vpc.amazonaws.com/efa` without an EC2NodeClass network interface configuration, Karpenter will maintain current dynamic provisioning behavior by maximizing the EFA devices configured for the instance. If the EFA resource is used with an EC2NodeClass network interface configuration, Karpenter will validate compatibility and if compatible, Karpenter launches the instance with the configuration specified in the EC2NodeClass.

### Max Pods Calculation

The max pods calculation changes when using `ENILimitedPods`. For ENI-limited pods, the max pods per node follows this formula:

```bash
max pods = max number of ENIs * (IPv4 Addresses per ENI - 1) + 2
```

The max number of ENIs is calculated only for the primary network card (network card 0). If an EFA-only interface is configured on this network card, then the available ENI count is reduced by 1.


### IP-Related Launch Template Inputs

Users currently configure `spec.ipPrefixCount` on the EC2NodeClass, which correlates to `Ipv4PrefixCount` and `Ipv6PrefixCount` in the launch template. EFA-only interfaces cannot be configured with an IP prefix count and attempting to do so causes ec2:RunInstance calls to fail. To handle this, Karpenter will not set prefix counts for any EFA-only interfaces configured in the network interface specification. This allows users to configure prefix delegation for all other (non-EFA-only) network interfaces. Karpenter will behave similarly for `PrimaryIpv6` and `Ipv6AddressCount` configurations in the launch template.

## Drift

Karpenter will determine drift statically based on the EC2NodeClass network interface configuration. When an EC2NodeClass's `networkInterfaces` configuration changes, Karpenter will drift existing nodes.

## Appendix

### Other Design Considerations - InterfacePolicy

The above configuration does not blend well with non-homogenous instance type NodePools, as many instance types have different network topology that would result in different network interface configurations. To support heterogenous NodePools, we considered the following configuration surface. Note that this configuration could also be combined with the granular network card configuration above.

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: example-node-class
spec:
  # interfacePolicy specifies network interface configurations to be 
  # attached to provisioned instances.
  interfacePolicy: bandwidthOptimized | ipOptimized
```

An `interfacePolicy` of `bandwidthOptimized` results in instance launches with a combination of ENA and EFA-only interfaces to maximize IP and EFA network bandwidth. This is instance type specific, and since there is no API or instance type agnostic calculation for this, Karpenter will internally have the configuration for each instance type. On new instance type release, we will have to generate this configuration for Karpenter releases similar to how bandwidth / VPC and pricing information is done.

An `interfacePolicy` of `ipOptimized` results in the primary network interface (DI=0, NC=0) as ENA and EFA-only for for the rest of the network cards (as well as NC=0, DI=1 if supported).

With the decided approach of network card granular configurations, we can still support this `interfacePolicy` configuration later on.

### EFA Count Label

The idea this label is as follows. When Karpenter launches an instance with EFA devices, it will apply the following well-known labels to the Node/NodeClaim:

| Label | Values | Description |
|-------|--------|-------------|
| `karpenter.k8s.aws/instance-efa-count` | int (e.g., `32`) | Identifies the number of EFA devices the node is launched with. |

This label serves two primary purposes:

1. __Informative labeling__ - Provides visibility into the EFA device count configured on the instance at launch time.

2. __Triggers EFA provisioning__ - The presence of this label triggers Karpenter's dynamic EFA provisioning path (if NodeClass network interfaces are not configured). For instance, if a pod specifies node affinity with `karpenter.k8s.aws/instance-efa-count` greater than 0 and no network interface configuration is specified on the EC2NodeClass, Karpenter will launch instances with all EFA devices configured.

The `karpenter.k8s.aws/instance-efa-count` label differs from the currently supported `vpc.amazonaws.com/efa` resource as it can be used as a scheduling label, while `vpc.amazonaws.com/efa` is a pod level resource request.

We chose to not support this label at launch as it would require signifigant changes to core Karpenters scheduling simulation. Karpenter currently does not support scheduling with dynamic label applications. This rough edge with this is that since Karpenter would not apply `karpenter.k8s.aws/instance-efa-count` if it is 0, then when use the `Exists` operator this would result in NodeClaim launches without the label (as internally Karpenter would consider the EFA count label of 0 as exists).
