---
title: "Utilizing GPUs and EFAs with Dynamic Resource Allocation"
linkTitle: "Utilizing GPUs and EFAs with DRA"
---

<i class="fa-solid fa-circle-info"></i> <b>Feature State: </b> [Alpha]({{<ref "../reference/settings#aws-specific-feature-gates" >}})

Karpenter can provision nodes for pods that request NVIDIA GPUs and Elastic Fabric Adapters (EFAs) through [Dynamic Resource Allocation](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/) (DRA).
To enable this support, set [`settings.enableDRA`]({{< relref "#enabling-karpenter-support" >}}) in the Helm chart.

{{% alert title="Note" color="primary" %}}
DRA requires Kubernetes 1.34 or later.
[GPU time-slicing]({{< relref "#example-requesting-a-timesliced-gpu" >}}) requires Kubernetes 1.36 or later.
{{% /alert %}}

## Overview

With device plugins, a pod asks for devices as an integer count of an extended resource, such as `nvidia.com/gpu: 1`.
With DRA, a pod references a [ResourceClaim](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/#resourceclaims-templates), which describes the devices it needs.
A claim can select devices by their attributes using [CEL](https://kubernetes.io/docs/reference/using-api/cel/), require several devices to share an attribute (for example, a GPU and an EFA on the same PCIe root), and request a portion of a device's capacity.
A DRA driver runs on each node and publishes the node's devices and their attributes as ResourceSlices. The scheduler allocates each claim from the devices in those ResourceSlices.

The drivers only publish ResourceSlices once a node is running, but Karpenter has to choose an instance type before the node exists.
To close that gap, Karpenter includes metadata for each [supported instance type]({{< relref "#supported-instance-types" >}}) that describes the devices the NVIDIA and EFA DRA drivers publish on that instance type.
When a pod with ResourceClaims is pending, Karpenter evaluates its claims against this metadata, along with the ResourceSlices of existing nodes. If no existing node can satisfy the claims, Karpenter launches the cheapest instance type that can.
This works from zero: no GPU node has to exist first, and NodePools and EC2NodeClasses need no DRA-specific configuration.

{{% alert title="Note" color="primary" %}}
Static NodePools launch nodes without simulating pods, and the DRA drivers and kube-scheduler handle allocation on those nodes.
DRA does not require explicit Karpenter support for static NodePools.
This page covers dynamic provisioning.
{{% /alert %}}


## Capabilities

DRA makes requests possible that extended resources can't express. For example:

* **Select GPUs by their attributes**, such as model, architecture, CUDA compute capability, or memory. See [Requesting a specific GPU]({{< relref "#example-requesting-a-specific-gpu-via-dra" >}}).
* **Align devices on the same PCIe root.** For example, request a GPU and an EFA that share a PCIe root for GPUDirect RDMA, or two GPUs that share a PCIe root. See [Requesting a GPU and EFA that share a PCIe root]({{< relref "#example-requesting-a-gpu-and-efa-that-share-a-pcie-root" >}}).
* **Share a GPU between pods** with consumable capacity, where each claim takes a share of a GPU. See [Requesting a timesliced GPU]({{< relref "#example-requesting-a-timesliced-gpu" >}}).

## Installing the Drivers

Install each driver by following its install guide, then set the Helm values below.

{{% alert title="Warning" color="warning" %}}
A DRA driver and a device plugin must never manage the same device on the same node. If you also run the NVIDIA or EFA device plugins in your cluster, see [Running DRA drivers alongside device plugins]({{< relref "#running-dra-drivers-alongside-device-plugins" >}}).
{{% /alert %}}

### NVIDIA

Install the NVIDIA DRA driver by following the [NVIDIA DRA driver install guide](https://dra-driver-nvidia-gpu.sigs.k8s.io/docs/install/) or the [EKS guide](https://docs.aws.amazon.com/eks/latest/userguide/device-management-nvidia-dra-device-plugin.html#eks-nvidia-dra-driver). Neither value below is the chart's default, so set both in the driver's Helm chart:

| Helm value                         | Default | Value   | Why                                                                                   |
|------------------------------------|---------|---------|---------------------------------------------------------------------------------------|
| `gpuResourcesEnabledOverride`      | `false` | `true`  | Required for GPU allocation.                                                          |
| `resources.computeDomains.enabled` | `true`  | `false` | Optional. Disables ComputeDomains, which Karpenter doesn't support provisioning for. |

To share GPUs between pods, you also set the driver's consumable shares values. See [Requesting a timesliced GPU]({{< relref "#example-requesting-a-timesliced-gpu" >}}).

The chart creates the `gpu.nvidia.com` DeviceClass.
The kubelet plugin tolerates the `nvidia.com/gpu` taint by default. Its default affinity only schedules it on nodes with a GPU presence label, such as `nvidia.com/gpu.present=true`.
The EKS-optimized AL2023 NVIDIA AMI sets that label on GPU nodes, and Karpenter selects it automatically for GPU instance types with the `al2023@latest` alias.
If your nodes use another AMI, check that it sets the label. If it doesn't, add the label to your DRA NodePool. Otherwise the driver won't run on the nodes Karpenter launches, and DRA pods stay pending:

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: gpu-dra
spec:
  template:
    metadata:
      labels:
        nvidia.com/gpu.present: "true"
```

{{% alert title="Note" color="primary" %}}
Karpenter doesn't support dynamic provisioning for [ComputeDomains](https://dra-driver-nvidia-gpu.sigs.k8s.io/docs/concepts/compute-domains/). It has no metadata for the `compute-domain.nvidia.com` driver, so it won't launch nodes for claims on ComputeDomain DeviceClasses.
{{% /alert %}}

### EFA

Install the EFA DRA driver (DRANET) by following [Install the EFA DRA driver](https://docs.aws.amazon.com/eks/latest/userguide/device-management-efa.html#efa-dra-driver). Set this value in the `aws-dranet` Helm chart:

| Helm value     | Value                                                         | Why                                                                                 |
|----------------|---------------------------------------------------------------|-------------------------------------------------------------------------------------|
| `tolerations`  | `[{key: nvidia.com/gpu, operator: Exists, effect: NoSchedule}]` | The DaemonSet doesn't tolerate the `nvidia.com/gpu` taint by default, so it won't run on tainted GPU nodes without this. |

The chart creates the `efa.networking.k8s.aws` DeviceClass. It selects devices whose `dra.net/pciDevice` attribute is `Elastic Fabric Adapter (EFA)`.
DRANET publishes every network interface on a node, but Karpenter's metadata includes only EFA devices, so request EFAs through this DeviceClass.

## Enabling Karpenter Support

Set `settings.enableDRA: true` in Karpenter's Helm values to enable Karpenter's support for every DRA driver it models, currently the NVIDIA GPU and EFA drivers.
For how to change the values of an existing installation, see the [upgrade guide]({{< relref "../upgrading/upgrade-guide" >}}).

This enables the `DRANVIDIAGPU` and `DRAEFA` feature gates and sets `settings.ignoreDRARequests` to `false`.
The chart always grants Karpenter read access to DeviceClasses, ResourceClaims, and ResourceSlices.

`settings.enableDRA` assumes both drivers run on your DRA nodes. If a driver isn't running, Karpenter launches nodes for its claims that can never serve them.
If you only run one of the drivers, [enable its gate individually]({{< relref "#enabling-drivers-individually" >}}) instead.

{{% alert title="Note" color="primary" %}}
When you upgrade an existing release with `helm upgrade --reuse-values`, Helm also reuses the default values of the chart version you installed before.
Earlier chart versions default `settings.ignoreDRARequests` to `true`, so setting `settings.enableDRA` with `--reuse-values` fails with the error above, even if you never set `settings.ignoreDRARequests` yourself.
Use `--reset-then-reuse-values` (Helm 3.14 or later) instead. If you previously set `settings.awsFeatureGates.draNVIDIAGPU` or `settings.awsFeatureGates.draEFA`, unset them in the same upgrade, for example with `--set settings.awsFeatureGates.draNVIDIAGPU=null`.
{{% /alert %}}

## Example: Requesting a specific GPU via DRA

This ResourceClaimTemplate requests one GPU that's Hopper or newer (CUDA compute capability 9.0 or later) with at least 100Gi of memory:

```yaml
apiVersion: resource.k8s.io/v1
kind: ResourceClaimTemplate
metadata:
  name: large-gpu
spec:
  spec:
    devices:
      requests:
      - name: gpu
        exactly:
          deviceClassName: gpu.nvidia.com
          count: 1
          selectors:
          - cel:
              expression: |
                device.attributes["gpu.nvidia.com"].cudaComputeCapability.compareTo(semver("9.0.0")) >= 0 &&
                device.capacity["gpu.nvidia.com"].memory.compareTo(quantity("100Gi")) >= 0
---
apiVersion: v1
kind: Pod
metadata:
  name: inference
spec:
  tolerations:
  - key: nvidia.com/gpu
    operator: Exists
    effect: NoSchedule
  resourceClaims:
  - name: gpu
    resourceClaimTemplateName: large-gpu
  containers:
  - name: model
    image: public.ecr.aws/amazonlinux/amazonlinux:2023-minimal
    command: ["/bin/sh", "-c", "nvidia-smi && sleep infinity"]
    resources:
      claims:
      - name: gpu
```

Karpenter only considers instance types with a GPU that matches the selector: here, H200 (`p5e`, `p5en`) and Blackwell (`p6-b200`, `p6-b300`). It launches the cheapest one the NodePool allows.
To request several GPUs, raise `count`. Karpenter checks that the instance type has enough matching GPUs.

You can only select on attributes that Karpenter knows before launch. See [Supported Attributes by Driver]({{< relref "#supported-attributes-by-driver" >}}).

## Example: Requesting a GPU and EFA that share a PCIe Root

Enable DRA support with `settings.enableDRA` (or both individual gates), and install both drivers.
This ResourceClaimTemplate requests one GPU and one EFA, and requires them to share a PCIe root:

```yaml
apiVersion: resource.k8s.io/v1
kind: ResourceClaimTemplate
metadata:
  name: gpu-efa-aligned
spec:
  spec:
    devices:
      requests:
      - name: gpu
        exactly:
          deviceClassName: gpu.nvidia.com
          count: 1
      - name: efa
        exactly:
          deviceClassName: efa.networking.k8s.aws
          count: 1
      constraints:
      - requests: ["gpu", "efa"]
        matchAttribute: resource.kubernetes.io/pcieRoot
```

Reference the claim from a pod as in the [previous example]({{< relref "#example-requesting-a-specific-gpu-via-dra" >}}).
Karpenter only launches instance types where a GPU and an EFA share a PCIe root. For example, each of the eight GPUs in a `p5.48xlarge` shares a PCIe root with four EFAs.

{{% alert title="Warning" color="warning" %}}
Configuring `spec.networkInterfaces` on an EC2NodeClass used for EFA DRA workloads is currently unsupported and results in undefined behavior. Leave it unset.
{{% /alert %}}

## Example: Requesting a timesliced GPU

{{% alert title="Note" color="primary" %}}
GPU sharing through DRA requires Kubernetes 1.36 or later, where [consumable capacity](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/) is enabled by default.
{{% /alert %}}

With the NVIDIA DRA driver's [consumable shares](https://dra-driver-nvidia-gpu.sigs.k8s.io/docs/guides/gpu-allocation/consumable-capacity/), several claims can share one GPU, and the driver time-slices between them.
First, enable consumable shares by setting these values in the NVIDIA DRA driver's Helm chart. This example splits each GPU into four shares:

| Helm value                      | Value  |
|---------------------------------|--------|
| `featureGates.ConsumableShares` | `true` |
| `consumableShares`              | `4`    |

For the other modes and how the driver applies them, see the [consumable capacity guide](https://dra-driver-nvidia-gpu.sigs.k8s.io/docs/guides/gpu-allocation/consumable-capacity/).

Karpenter can't read the driver's configuration, so set the same mode with the `karpenter.k8s.aws/nvidia-consumable-capacity` annotation on the EC2NodeClass:

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: gpu-dra
  annotations:
    karpenter.k8s.aws/nvidia-consumable-capacity: "4"
spec:
  # ...
```

The annotation accepts the same values as the driver's `consumableShares` setting:

| Value                | Karpenter models each GPU as                                                                   |
|----------------------|-------------------------------------------------------------------------------------------------|
| absent or `disabled` | Not shared. Each GPU serves one claim.                                                           |
| positive integer `N` | Shared, with `N` shares. A claim takes one share by default, and its memory request defaults to 0. |
| `memory`             | Shared by memory. A claim with no memory request takes the GPU's full memory.                   |
| `unlimited`          | Shared without limit. A claim's memory request defaults to 0.                                  |

Then give each pod its own claim from a template. A plain GPU request takes one share:

```yaml
apiVersion: resource.k8s.io/v1
kind: ResourceClaimTemplate
metadata:
  name: gpu-share
spec:
  spec:
    devices:
      requests:
      - name: gpu
        exactly:
          deviceClassName: gpu.nvidia.com
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shared-inference
spec:
  replicas: 8
  selector:
    matchLabels:
      app: shared-inference
  template:
    metadata:
      labels:
        app: shared-inference
    spec:
      tolerations:
      - key: nvidia.com/gpu
        operator: Exists
        effect: NoSchedule
      resourceClaims:
      - name: gpu
        resourceClaimTemplateName: gpu-share
      containers:
      - name: model
        image: public.ecr.aws/amazonlinux/amazonlinux:2023-minimal
        command: ["/bin/sh", "-c", "nvidia-smi && sleep infinity"]
        resources:
          claims:
          - name: gpu
```

Karpenter packs up to four of these claims onto each GPU, so the eight replicas need only two GPUs.

In `memory` mode, request `memory` instead, for example `memory: 10Gi`.
Shares and memory only affect scheduling. They don't limit how much GPU memory a process actually uses.

## Appendix

### Reference configuration

#### EC2NodeClass and NodePool

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: gpu-dra
  # Only for GPU sharing. Must match the driver's consumableShares setting.
  # annotations:
  #   karpenter.k8s.aws/nvidia-consumable-capacity: "4"
spec:
  role: "KarpenterNodeRole-${CLUSTER_NAME}"
  amiSelectorTerms:
  - alias: al2023@latest # Resolves to the NVIDIA variant for GPU instance types
  subnetSelectorTerms:
  - tags:
      karpenter.sh/discovery: "${CLUSTER_NAME}"
  securityGroupSelectorTerms:
  # For EFA, include a security group that allows all traffic to and from itself
  - tags:
      karpenter.sh/discovery: "${CLUSTER_NAME}"
  # Leave networkInterfaces unset. Karpenter attaches every EFA when a pod is allocated dra.net devices.
---
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: gpu-dra
spec:
  template:
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: gpu-dra
      requirements:
      - key: karpenter.k8s.aws/instance-family
        operator: In
        values: ["g6", "g6e", "p5", "p5en", "p6-b200"]
      taints:
      - key: nvidia.com/gpu
        effect: NoSchedule
```

#### Pods

A pod that uses DRA devices needs:

* A `spec.resourceClaims` entry that references a ResourceClaimTemplate (one claim per pod) or a ResourceClaim (shared by every pod that references it).
* A `resources.claims` entry on each container that uses the devices.
* A toleration for the `nvidia.com/gpu` taint.

Karpenter treats a pod as a DRA pod only if it has `spec.resourceClaims` or a container lists `resources.claims`.
A pod that requests the `nvidia.com/gpu` extended resource isn't treated as a DRA pod, even when a DeviceClass serves that resource through DRA.

### Running DRA drivers alongside device plugins

A DRA driver and a device plugin must never manage the same device on the same node. If both run, they can each hand out the same device, oversubscribing it without any error.
If your cluster also runs the [NVIDIA device plugin](https://docs.aws.amazon.com/eks/latest/userguide/device-management-nvidia-dra-device-plugin.html#eks-nvidia-device-plugin) or the [EFA device plugin](https://docs.aws.amazon.com/eks/latest/userguide/device-management-efa.html#eks-efa-device-plugin), keep each node on one mechanism by giving DRA and device plugin NodePools separate labels.

Karpenter's DRA metadata applies to every NodePool that allows a supported instance type, including NodePools whose nodes run the device plugins.
If a DRA pod can schedule to both, Karpenter can launch it on a device plugin node, where no DRA driver publishes devices, and the pod stays pending.
Constrain DRA pods to your DRA NodePools as well.

1. Label each NodePool with the mechanism its nodes use, for example `example.com/device-manager: dra` or `example.com/device-manager: device-plugin`:

    ```yaml
    apiVersion: karpenter.sh/v1
    kind: NodePool
    metadata:
      name: gpu-dra
    spec:
      template:
        metadata:
          labels:
            example.com/device-manager: dra
    ```

2. Set a matching `nodeSelector` in each chart's Helm values:

    | Chart                    | Helm value                   | Value                                       |
    |--------------------------|------------------------------|---------------------------------------------|
    | NVIDIA DRA driver        | `kubeletPlugin.nodeSelector` | `example.com/device-manager: dra`           |
    | EFA DRA driver (DRANET)  | `nodeSelector`               | `example.com/device-manager: dra`           |
    | NVIDIA device plugin     | `nodeSelector`               | `example.com/device-manager: device-plugin` |
    | EFA device plugin        | `nodeSelector`               | `example.com/device-manager: device-plugin` |

3. Add a `nodeSelector` on `example.com/device-manager: dra` to pods that use DRA devices:

    ```yaml
    spec:
      nodeSelector:
        example.com/device-manager: dra
    ```

### Supported Instance Types

Karpenter includes DRA metadata for the instance types below. An instance type with no EFA devices listed doesn't support EFA.
Support for a new instance type requires a new Karpenter release.

| Instance type | GPU | GPUs | EFA devices |
|---------------|-----|------|-------------|
| `g4dn.xlarge` | Tesla T4 | 1 | - |
| `g4dn.2xlarge` | Tesla T4 | 1 | - |
| `g4dn.4xlarge` | Tesla T4 | 1 | - |
| `g4dn.8xlarge` | Tesla T4 | 1 | 1 |
| `g4dn.12xlarge` | Tesla T4 | 4 | 1 |
| `g4dn.16xlarge` | Tesla T4 | 1 | 1 |
| `g4dn.metal` | Tesla T4 | 8 | 1 |
| `g5.xlarge` | NVIDIA A10G | 1 | - |
| `g5.2xlarge` | NVIDIA A10G | 1 | - |
| `g5.4xlarge` | NVIDIA A10G | 1 | - |
| `g5.8xlarge` | NVIDIA A10G | 1 | 1 |
| `g5.12xlarge` | NVIDIA A10G | 4 | 1 |
| `g5.16xlarge` | NVIDIA A10G | 1 | 1 |
| `g5.24xlarge` | NVIDIA A10G | 4 | 1 |
| `g5.48xlarge` | NVIDIA A10G | 8 | 1 |
| `g5g.xlarge` | NVIDIA T4G | 1 | - |
| `g5g.2xlarge` | NVIDIA T4G | 1 | - |
| `g5g.4xlarge` | NVIDIA T4G | 1 | - |
| `g5g.8xlarge` | NVIDIA T4G | 1 | - |
| `g5g.16xlarge` | NVIDIA T4G | 2 | - |
| `g5g.metal` | NVIDIA T4G | 2 | - |
| `g6.xlarge` | NVIDIA L4 | 1 | - |
| `g6.2xlarge` | NVIDIA L4 | 1 | - |
| `g6.4xlarge` | NVIDIA L4 | 1 | - |
| `g6.8xlarge` | NVIDIA L4 | 1 | 1 |
| `g6.12xlarge` | NVIDIA L4 | 4 | 1 |
| `g6.16xlarge` | NVIDIA L4 | 1 | 1 |
| `g6.24xlarge` | NVIDIA L4 | 4 | 1 |
| `g6.48xlarge` | NVIDIA L4 | 8 | 1 |
| `g6e.xlarge` | NVIDIA L40S | 1 | - |
| `g6e.2xlarge` | NVIDIA L40S | 1 | - |
| `g6e.4xlarge` | NVIDIA L40S | 1 | - |
| `g6e.8xlarge` | NVIDIA L40S | 1 | 1 |
| `g6e.12xlarge` | NVIDIA L40S | 4 | 1 |
| `g6e.16xlarge` | NVIDIA L40S | 1 | 1 |
| `g6e.24xlarge` | NVIDIA L40S | 4 | 2 |
| `g6e.48xlarge` | NVIDIA L40S | 8 | 4 |
| `g6f.large` | NVIDIA L4-3Q | 1 | - |
| `g6f.xlarge` | NVIDIA L4-3Q | 1 | - |
| `g6f.2xlarge` | NVIDIA L4-6Q | 1 | - |
| `g6f.4xlarge` | NVIDIA L4-12Q | 1 | - |
| `gr6.4xlarge` | NVIDIA L4 | 1 | - |
| `gr6.8xlarge` | NVIDIA L4 | 1 | 1 |
| `gr6f.4xlarge` | NVIDIA L4-12Q | 1 | - |
| `p3dn.24xlarge` | Tesla V100-SXM2-32GB | 8 | 1 |
| `p4d.24xlarge` | NVIDIA A100-SXM4-40GB | 8 | 4 |
| `p4de.24xlarge` | NVIDIA A100-SXM4-80GB | 8 | 4 |
| `p5.4xlarge` | NVIDIA H100 80GB HBM3 | 1 | 1 |
| `p5.48xlarge` | NVIDIA H100 80GB HBM3 | 8 | 32 |
| `p5e.48xlarge` | NVIDIA H200 | 8 | 32 |
| `p5en.48xlarge` | NVIDIA H200 | 8 | 16 |
| `p6-b200.48xlarge` | NVIDIA B200 | 8 | 8 |
| `p6-b300.48xlarge` | NVIDIA B300 SXM6 AC | 8 | 16 |

### Supported Attributes by Driver

Karpenter can only evaluate a claim against attributes it knows before the node launches. Some attributes can be used in two ways:

* **Selector**: a CEL expression in a claim or DeviceClass that compares the attribute to a value, for example `device.attributes["gpu.nvidia.com"].architecture == "Hopper"`.
* **matchAttribute**: a constraint that requires every device allocated for a set of requests to have the same value for the attribute, without naming the value.

Other attributes can only be used with `matchAttribute`. They describe how devices relate to each other on a node. Their values are resolved at runtime, or are specific to a platform, so Karpenter can tell whether devices will match but not what the value will be.

#### NVIDIA (`gpu.nvidia.com`)

| Attribute | Type | Example value | Selector | matchAttribute |
|-----------|------|---------------|----------|----------------|
| `productName` | string | `NVIDIA H100 80GB HBM3` | ✓ | ✓ |
| `architecture` | string | `Hopper` | ✓ | ✓ |
| `brand` | string | `Nvidia` | ✓ | ✓ |
| `cudaComputeCapability` | version | `9.0.0` | ✓ | ✓ |
| `type` | string | `gpu` | ✓ | ✓ |
| `driverVersion` | version | `580.82.7` | | ✓ |
| `cudaDriverVersion` | version | `13.0.0` | | ✓ |
| `resource.kubernetes.io/pcieRoot` | string | `pci0000:10` | | ✓ |

| Capacity | Example value | Selector | Capacity request |
|----------|---------------|----------|------------------|
| `memory` | `81152Mi` | ✓ | ✓ with consumable capacity |
| `shares` | `4` | ✓ | ✓ with an integer consumable capacity mode |


#### EFA (`dra.net`)

| Attribute | Type | Example value | Selector | matchAttribute |
|-----------|------|---------------|----------|----------------|
| `pciDevice` | string | `Elastic Fabric Adapter (EFA)` | ✓ | ✓ |
| `pciVendor` | string | `Amazon.com, Inc.` | ✓ | ✓ |
| `pciSubsystem` | string | `efa1` | ✓ | ✓ |
| `rdma` | bool | `true` | ✓ | ✓ |
| `numaNode` | int | `0` | ✓ | ✓ |
| `resource.kubernetes.io/pcieRoot` | string | `pci0000:10` | | ✓ |

