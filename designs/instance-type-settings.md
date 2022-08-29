# Instance Type Settings

## Goals

Allow users to:

* Define [custom device resources](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/) on an instance type
* Define allocatable memory, cpu, ephemeral storage, pods on an instance type
* Define pricing overrides for enterprise pricing support on an instance type or instance family
* Define pricing discounts as raw values and percentages off of the default pricing on an instance type or instance family
* Expand ephemeral storage for launching instances that contain pods that surpass the block device mappings default ephemeral storage

## Background

Kubelet-specific configuration for nodes deployed by Karpenter is passed through to the `bootstrap.sh` script and is used when bootstrapping the new node to the cluster. Karpenter currently supports certain static default values that are used when bootstrapping the node. In particular, values like `--max-pods=110` are used when the `AWS_ENABLE_POD_ENI` is set to `false`. Users currently have no way to specify extra arguments to the kubelet beyond `--max-pods` and `--system-reserved`. Additionally, there is currently no supported way to define per-instance type or per-instance family memory/cpu/ephemeral-storage requirements for overhead that is used during scheduling decision-making.

We currently surface all of these values on a per-provisioner basis through `.spec.kubeletConfiguration` for configuration that is passed down into the userData when bootstrapping the cluster and on a per-controller basis for values like `VM_MEMORY_OVERHEAD` values. Because instance types are vastly different in their resources, we need a way to expand this logic so that users can specify different metadata overrides on a per-instance type basis as opposed to a per-provisioner or per-controller basis.

Finally, we make consolidation decision-making based on the approximated cost-savings that we will get by either removing a node and rescheduling its pods on existing nodes or by replacing an existing node that has extra capacity with the pods that are currently scheduled on it with a smaller node that is strictly cheaper than the existing node.
 
Items with a 🔑 symbol call out key points or things of note

## 🔑 Proposed Updated API Surface

**Introduce `v1alpha1/InstanceType` CRD**

```yaml
apiVersion: karpenter.sh/v1alpha1
kind: InstanceType
metadata:
  name: "c5.large"
spec:   
  offerings: # These define the requirement offering possibilities for the instance type
    - karpenter.sh/capacity-type: spot
      topology.kubernetes.io/zone: us-west-2b
      karpenter.sh/price: 0.40
    - karpenter.sh/capacity-type: spot
      topology.kubernetes.io/zone: us-west-2c
  resources: # This defines the resources that the instance has for bin-packing, will be merged with known resource quantities
    hardware-vendor.example/foo: "2"
    hardware-vendor.example/bar: "20"
    pods: 10
  overhead: # Any overhead that should be subtracted from the instance type away from resources
    memory: 200Mi    
```


**Extend `v1alpha5/Provisioner` CRD**

In particular, we will extend the `.spec.kubeletConfiguration` with `kubeReserved` and `evictionThreshold` that will be passed down into the bootstrap scripts.

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: default
spec:
  requirements:
    - key: karpenter.sh/capacity-type
      operator: In
      values: ["spot"]
  limits:
    resources:
      cpu: "1000"
  kubeletConfiguration:
    kubeReserved:
      memory: "1Gi"
      cpu: "300m"
      ephemeral-storage: "1Gi"
    systemReserved:
      memory: "1Gi"
      cpu: "300m"
      ephemeral-storage: "1Gi"
    evictionThreshold:
      hard:
        memoryAvailable: "1Gi"
        imagefsAvailable: "10%"
        nodefsAvailable: "15%"
        nodefsInodesFree: "5%"
      soft:
        ...
```

## Instance Type Setting Examples

1. Configure the VM memory overhead values for `c5.large`

```yaml
apiVersion: karpenter.sh/v1alpha1
kind: InstanceType
metadata:
   name: "c5.large"
spec:
   overhead:
      memory: 200Mi
```

1. Configure custom device resources for custom device resources for `c4.large`

```yaml
apiVersion: karpenter.sh/v1alpha1
kind: InstanceType
metadata:
  name: "c4.large"
spec:
   resources:
      hardware-vendor.example/foo: "2"
```

1. Specify custom offerings with custom pricing information for `c4.large`

```yaml
apiVersion: karpenter.sh/v1alpha1
kind: InstanceType
metadata:
  name: "c4.large"
spec:
  offerings:
  - karpenter.sh/capacity-type: on-demand
    topology.kubernetes.io/zone: us-west-2b
    karpenter.sh/price: "0.60"
  - karpenter.sh/capacity-type: spot
    karpenter.sh/price: "0.40"
```

## Considerations

### Instance Type Overrides for Other Cloud Providers (Azure/CAPI/etc.)

Because Karpenter is intended to be cloud-agnostic, we need to keep this in mind when adding changes to the API surface. For the initial iteration of the `InstanceType` CRD, we will assume that the instance type will be selected by the `.metadata.name` field which will map to the well-defined Kubernetes node field `node.kubernetes.io/instance-type`.

For other clouds, this is expected to be mapped on the nodes as it is in AWS and the `GetInstanceType` call for other cloud providers is expected to retrieve and return all the instance type values that map to the `node.kubernetes.io/instance-type` value. In this way, Karpenter can perform instance type overrides for other clouds when instance types are surfaced.

### Custom Resource Cardinality

One of the primary questions presented is whether we should create a 1-1 mapping between settings and a given instance type or whether we should allow the ability to create more complex structures using set relationships and label selectors. Options are listed below combined with the pros and cons for each:

**Options**

1. Create an `InstanceType` CRD that specifies a 1-1 relationship between the settings and the instance type
    1. Pros
        1. Clearly defined relationship between an instance type and the settings that are defined for it
        2. Users that only have a few instance types will not have an issue configuring the instance types with these values
        3. We can consider adding an `InstanceTypeClass` or similar at a later date that would allow a grouping of instance types to be assigned the same settings
    2. Cons
        1. May not scale well as users that have high levels of customizations across large numbers of instance types may have to maintain a large number of `InstanceType` CRDs
        2. I may want to define settings over an instance family without having to individually configure the instances that exist within that family
2. Create an `InstanceTypeSetting` CRD that contains a list of instance types that allow a 1-many relationship between settings and instance types. These instance types could be selected either with a `selector` or `regex`
    1. Pros
        1. Allows users who want to apply the same instance type settings across a wide range of instance types or across an instance type family to do so
        2. Lower maintenance burden from maintaining less `InstanceTypeSetting` CRDs
    2. Cons
        1. Can create complex relationships that may be difficult to track for users
        2. If we were to allow overlapping `InstanceTypeSettings`, we would have to create a `weight` on the `InstanceTypeSettings` which would further convolute details for the user
        3. If we were to deny overlapping `InstanceTypeSettings` , then the feature of specifying a group of instance types may not be used that often as configuration between instance types may rarely be shared
        4. It’s unclear whether users would want have enough common configuration that they would assign across instance types. It makes more sense to iterate on a grouping mechanism later if there is a user ask for it

**Recommendation:** Use `InstanceType` CRD for initial iteration for 1-1 mappings before considering introducing more complex constructs to the user. While specifying rulesets might reduce the overhead burden for customers to keep track of separate `InstanceTypeSettings` , ultimately without an API surface to view exactly what settings that ruleset will produce, it will ultimately be confusing for users.

### 🔑 Modeling Offerings as Requirements

Currently, we model Offerings with the following static properties in the internal offering struct:

```go
type Offering struct {
  CapacityType string
  Zone string
  Price float64
  Available bool
}
```

Rather than model these values as static properties, we can extend the requirements model that we leverage in the InstanceType internal struct to transform offerings into

```go
type Offering struct {
  Requirements scheduling.Requirements
}
```

Requirements in Offerings would then be modeled by default for a given offering type with Capacity Type: “spot”, Zone: “us-west-2a” and Price per hour in USD of “0.40” as:

```yaml
offerings:
    - requirements:
      - key: "topology.kubernetes.io/zone"
        operator: "In"
        values: ["us-west-2a"]
      - key: "karpenter.sh/capacity-type"
        operator: "In"
        values: ["spot"]
      - key: "karpenter.sh/price"
        operator: "In"
        values: ["0.40"]
```

For an instance type that has no InstanceType overrides, we would use the default values used in the `createOfferings` call that is generated from the `DescribeInstanceType` call. This set of offerings would describe the full combination of options that are available from the cloud provider for a given instance type. When we schedule and attempt to launch an instance, we validate that there is an offering that matches the nodeTemplate requirements that are added from the scheduler.

In the case that the user provides an Instance Type offerings overrides (currently the scenario that they would do this would be for pricing overrides), we will use the provided offerings from the `InstanceType` CR as the offering values as opposed to the offering requirement values that are used by default for the cloud provider. To simplify the semantic, we assume the “In” operator for the offerings and we only allow a single value for each label requirement.

```yaml
offerings:
  - karpenter.sh/capacity-type: spot
    topology.kubernetes.io/zone: us-west-2b
    karpenter.sh/price: "0.40"
  - karpenter.sh/capacity-type: on-demand # No zonal constraint specified
    karpenter.sh/price: "0.60"
```

This implies that I can create offerings that don’t contain all constraints that are imposed by the default offering implementation. For instance, in the above `offerings` overrides, I don’t need a zonal constraint on the second offering constraint set which is essentially saying “give me an on-demand offering in any zone and all on-demand offerings regardless of zone cost ~$6.00 per day.”

The upshot of this is that we become far more flexible with our offerings in that we can now have arbitrary constraints specified by users that can differentiate the pricing for a set of labels (i.e. I can specify different pricing for different OS for instance)

🔑  Offerings specified in `.spec.offerings` would *fully override* the offerings that would come by default for the AWS cloud provider. This means that the ownership is on the user to fully define the set of offerings that they want to make available for a given instance type.

**🔑  Recommendation:** Convert static offering constraints into offering requirements that give more flexibility into how to constrain offerings on instance types. Allow users to define which offerings they want to surface per `InstanceType` using the offering requirements/constraints.

### ✨ Reserved Resources/Resource Overhead

For any given node that is launched as part of Kubernetes, kubelet can be configured with both [system reserved resources](https://kubernetes.io/docs/tasks/administer-cluster/reserve-compute-resources/#system-reserved)  and [kubelet reserved resources.](https://kubernetes.io/docs/tasks/administer-cluster/reserve-compute-resources/#kube-reserved) System reserved resources are more likely to be tied to the system daemons that are shipped with an OS image and will most likely be flat resource overhead values on a per-provisioner basis. As such, they are more closely tied to the AMI that the node is launched with, which is tied to the Provisioner `provider` directly.

Kubelet reserved resources are much more a function of the overhead of the pods that run on the node. Kubelet reserved resources can be affected by:

1. Number of pods running on a given node
2. The size of the images running on a given node

Thus, we can break down resource overhead into two main categories:

1. Flat resource overhead
    1. This includes systemReserved values for system daemons, OS overhead, etc.
    2. Hard eviction threshold for the node
    3. OS memory overhead (assigned to **VM_MEMORY_OVERHEAD** now) to run the instance
2. Dynamic Resource Overhead (per-pod overhead)
    1. This includes kubeReserved values needed for the container runtime. These values are typically dependent on the number of pods that are scheduled to the instance (or at worst the maximum number of pods that could be scheduled to the instance)
    2. This includes needed ephemeral storage since large images that come from multiple pods scheduled to a node can increase the needed ephemeral storage and affect the need to have larger blockDeviceMappings on the provisioner

Based on these two categories, we can tackle user-defined overhead/resource values in the following way:

1. Flat Resource Overhead
    1. `kubeletConfiguration` can live at the `Provisioner` level. If there is need to specify flat `kubeReserved` or `systemReserved` values based on the user’s knowledge of the OS/AMI, they are able to set this at the Provisioner-level
    2. Hard eviction thresholds can be set within the `kubeletConfiguration` at the `Provisioner` level as this will often be some globally defaulted value or vary based on knowledge of the OS or the system daemons
    3. OS memory overhead (**VM_MEMORY_OVERHEAD**) can be moved into this `InstanceType` CRD. Since this overhead in bytes will vary on a per-instance type level, we can allow the user to set a custom value to reserve for VM_OVERHEAD rather than defaulting to the percentage value that is currently reserved for overhead.
2. Dynamic Resource Overhead (per-pod overhead)
    1. [RuntimeClass](https://kubernetes.io/docs/concepts/containers/runtime-class/) provides a mechanism for users to specify per-pod overhead values that can be reserved for each pod. This is the recommended way that Kubernetes provides for specifying any pod overhead values that are dynamic and based on the CRI.
       1. 🔑 Dynamic pod overhead that is tied to the container runtime should move to a `RuntimeClass` if there is pod overhead unique to the CRI that is used by your nodes.
    2. Other per-pod overhead that may be tied to things like image size can be tied to the pod `ephemeral-stroage` requests which will help Karpenter ensure that it accurately provisions pods to nodes while also ensuring that there is parity between the scheduling decisions of Karpenter and the kube-scheduler.
    3. In the meantime, while most users may not be using either mechanism to achieve system-wide pod overheads, we will still calculate the `kubeReserved` values based on pod density. There are still some open questions based on whether we should open up the ability to specify the strategy type (see below)
3. Max Pods
    1. In general, it appears to be an anti-pattern in the Kubernetes community to strictly fix maxPods values (for kubeletConfiguration) on a per-instance type level. Users should be restricting the number of pods on their instances based on pod requests and size of instances as opposed to strict pod density sizing. If users are concerned with Provisioners creating instances that are too large, there is now the option to specify GT or LT operators for setting limits for how large an instance type that is provisioned can get.
    2. 🔑 For users that continue to utilize and/or need `--max-pods`, we will surface a `pods` value in the `.spec.resources` of the `InstanceType`. This `pods` value will be used for pod density for bin-packing and scheduling. Since there would now be a `max-pods` value surfaced at both the `InstanceType` level and at the `Provisioner` level, we will take the `min(provisioner.maxPods, instanceType.maxPods)` to be the pod density for the instance. 
       1. These per-instance type pod density values will be passed into the userData that is used to bootstrap the node. Instance type discovery will be performed in the userData script to determine the max pods value to pass to `bootstrap.sh`.

**🔑 Open Questions**

1. There is still [users asks](https://github.com/aws/karpenter/issues/1803) for calculating the kubeReserved resources value based on the GKE calculation of the kubeReserved resources value as opposed to the EKS default which is based on pod density. Should we allow some flag on the provisioner that can determine which way to base the calculation on? i.e. `kubeReservedMemoryStrategy`

### Extending Ephemeral Storage

There are [user asks](https://github.com/aws/karpenter/issues/1995) to have the ephemeral storage dynamically scaled for nodes based on the pod requests that are scheduled to that node (combined with overhead for kubeletReserved and systemReserved). Ephemeral storage is tied to blockDeviceMappings specified in launch templates. We currently specify default ephemeral storage of 20GB for all AMI families unless the user specifies their own custom launch template or explicitly specifies their own custom blockDeviceMappings.

In general, it seems the asks from users specify the ephemeral-storage as a function of pod density on the node. We could consider adding `blockDeviceMappings` to the `InstanceType` CR; however, this places more maintenance overhead on the user and makes the `InstanceType` CR vendor-specific.

`InstanceType` CR with Block Device Mappings:

```yaml
apiVersion: karpenter.sh/v1alpha1
kind: InstanceType
metadata:
  name: "c5.large"
spec:
  resources: # This defines the starting resources that the instance has
    custom-device.com/resource: "2"
    memory: 1Gi
    cpu: 2Gi
  provider:
    blockDeviceMappings:
    - deviceName: /dev/xvda
      ebs:
        volumeSize: 100Gi
        volumeType: gp3
        iops: 10000
        encrypted: true
        kmsKeyID: "arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"
        deleteOnTermination: true
        throughput: 125
        snapshotID: snap-0123456789
```


**🔑 Recommendation:** Based on the additional overhead and muddiness of putting `blockDeviceMappings` in the `InstanceType` CR, we will punt on block device mappings for now. For now, we will consider extending ephermal-storage through some dynamic mechanism out of scope, though the hope is that we will have some mechanism as a future feature that calculates the combined needed ephemeral-storage based off of the pod requests, kubeReserved and systemReserved.

### Launch Templates

For now, we are avoiding the need for Instance Type changes to affect launch template cardinality. Configuration at the `InstanceType` level is intended to configure the starting point for the scheduler to know how many resources can be used by the bin-packer. The expectation is that when the node comes up this is roughly the allocatable amount of resources the node will have, less kubeReserved, systemReserved, and evictionThresholds. Thus, the calculation we will do will be:

```
Allocatable = Instance Type Resources - kubeReserved - systemReserved - evictionThreshhold
```

By configuring the `InstanceType` CRD this way, we allow users to solve things like custom VM_OVERHEAD values to be more accurate on VM overhead and custom device requests to tell the scheduler to be aware of custom resources requests.

### Pricing Information

An ask is to allow users to override the pricing information for instance types so that users that have enterprise agreements are able to override the global public pricing information for on-demand instances with their own custom enterprise pricing data. Additionally, users that have reserved instances that have their own custom pricing data. For the scheduler/consolidator to take this pricing into account, this pricing information needs to be surfaced through some mechanism. Below describes the possible mechanisms to surface this data


1. Create a `PricingConfiguration` CRD or ConfigMap that contains mappings between offering and the pricing for that offering. Any offerings specified in this `PricingConfiguration` CRD would be read and overriden when generated the pricing data that Karpenter uses for scheduling
    1. Pros
        1. Creates clean separation between pricing configuration for instances and the system/requirements specific information for instances (keep separation between pricing and `InstanceTypeSettings`
    2. Cons
        1. Creates another CR/configMap that the user has to configure, understand, and maintain
2. Add pricing information to the `InstanceType` CRD
    1. Pros
        1. Maintains all instance-specific information in one location
        2. Less CRDs for the user to maintain
    2. Cons
        1. Pricing information is instance-type specific which means we are necessarily requiring a 1-1 mapping between some CRD and an instance type. This creates less flexibility if a user wants to describe common settings across an instance family or across an architecture type.

**🔑 Recommendation:** Pricing information should live within the InstanceType offering requirements as it naturally fits that pricing is directly associated with a given instance type.

### Linked Issues

1. Max Pods
    1. https://github.com/aws/karpenter/issues/2129
    2. https://github.com/aws/karpenter/issues/2180
    3. https://github.com/aws/karpenter/issues/1490
2. Kubelet Overrides
    1. https://github.com/aws/karpenter/issues/2129
    2. https://github.com/aws/karpenter/issues/1803
3. Device Block Volumes
    1. https://github.com/aws/karpenter/issues/1995
4. Custom Device Requests
    1. https://github.com/aws/karpenter/pull/2161

### Links/Additional Issues

* [Kubelet Command Line Arguments](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/)
* [Kubelet Eviction](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/)
* [Reserved Resources](https://kubernetes.io/docs/tasks/administer-cluster/reserve-compute-resources)
* [GKE Memory Calculation](https://cloud.google.com/kubernetes-engine/docs/concepts/cluster-architecture#memory_cpu)

