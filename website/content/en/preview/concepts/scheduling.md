---
title: "Scheduling"
linkTitle: "Scheduling"
weight: 40
description: >
  Learn about scheduling workloads with Karpenter
---

If your pods have no requirements for how or where to run, you can let Karpenter choose nodes from the full range of available cloud provider resources.
However, by taking advantage of Karpenter's model of layered constraints, you can be sure that the precise type and amount of resources needed are available to your pods.
Reasons for constraining where your pods run could include:

* Needing to run in zones where dependent applications or storage are available
* Requiring certain kinds of processors or other hardware
* Wanting to use techniques like topology spread to help ensure high availability

Your Cloud Provider defines the first layer of constraints, including all instance types, architectures, zones, and purchase types available to its cloud.
The cluster administrator adds the next layer of constraints by creating one or more NodePools.
The final layer comes from you adding specifications to your Kubernetes pod deployments.
Pod scheduling constraints must fall within a NodePool's constraints or the pods will not deploy.
For example, if the NodePool sets limits that allow only a particular zone to be used, and a pod asks for a different zone, it will not be scheduled.

Constraints you can request include:

* **Resource requests**: Request that certain amount of memory or CPU be available.
* **Node selection**: Choose to run on a node that is has a particular label (`nodeSelector`).
* **Node affinity**: Draws a pod to run on nodes with particular attributes (affinity).
* **Topology spread**: Use topology spread to help ensure availability of the application.
* **Pod affinity/anti-affinity**: Draws pods towards or away from topology domains based on the scheduling of other pods.

Karpenter supports standard Kubernetes scheduling constraints.
This allows you to define a single set of rules that apply to both existing and provisioned capacity.

{{% alert title="Note" color="primary" %}}
Karpenter supports specific [Well-Known Labels, Annotations and Taints](https://kubernetes.io/docs/reference/labels-annotations-taints/) that are useful for scheduling.
{{% /alert %}}

## Resource requests

Within a Pod spec, you can both make requests and set limits on resources a pod needs, such as CPU and memory.
For example:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: myapp
spec:
  containers:
  - name: app
    image: myimage
    resources:
      requests:
        memory: "128Mi"
        cpu: "500m"
      limits:
        memory: "256Mi"
        cpu: "1000m"
```
In this example, the container is requesting 128MiB of memory and .5 CPU.
Its limits are set to 256MiB of memory and 1 CPU.
Instance type selection math only uses `requests`, but `limits` may be configured to enable resource oversubscription.


See [Managing Resources for Containers](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/) for details on resource types supported by Kubernetes, [Specify a memory request and a memory limit](https://kubernetes.io/docs/tasks/configure-pod-container/assign-memory-resource/#specify-a-memory-request-and-a-memory-limit) for examples of memory requests, and [NodePools]({{<ref "./nodepools" >}}) for a list of supported resources.

### Accelerators/GPU Resources

Accelerator (e.g., GPU) values include
- `nvidia.com/gpu`
- `amd.com/gpu`
- `aws.amazon.com/neuron`
- `aws.amazon.com/neuroncore`
- `habana.ai/gaudi`

Karpenter supports accelerators, such as GPUs.

Additionally, include a resource requirement in the workload manifest. This will cause the GPU dependent pod to be scheduled onto the appropriate node.

Here is an example of an accelerator resource in a workload manifest (e.g., pod):

```yaml
spec:
  template:
    spec:
      containers:
      - resources:
          limits:
            nvidia.com/gpu: "1"
```
{{% alert title="Note" color="primary" %}}
If you are provisioning nodes that will utilize accelerators/GPUs, you need to deploy the appropriate device plugin daemonset.
Without the respective device plugin daemonset, Karpenter will not see those nodes as initialized.
Refer to general [Kubernetes GPU](https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/#deploying-amd-gpu-device-plugin) docs and the following specific GPU docs:
* `nvidia.com/gpu`: [NVIDIA device plugin for Kubernetes](https://github.com/NVIDIA/k8s-device-plugin)
* `amd.com/gpu`: [AMD GPU device plugin for Kubernetes](https://github.com/RadeonOpenCompute/k8s-device-plugin)
* `aws.amazon.com/neuron`/`aws.amazon.com/neuroncore`: [AWS Neuron device plugin for Kubernetes](https://awsdocs-neuron.readthedocs-hosted.com/en/latest/containers/kubernetes-getting-started.html#neuron-device-plugin)
* `habana.ai/gaudi`: [Habana device plugin for Kubernetes](https://github.com/HabanaAI/habanalabs-k8s-device-plugin)
  {{% /alert %}}

#### AWS Neuron Resources

The [Neuron scheduler extension](https://awsdocs-neuron.readthedocs-hosted.com/en/latest/containers/kubernetes-getting-started.html#neuron-scheduler-extension) is required for pods that require more than one Neuron core (`aws.amazon.com/neuroncore`) or device (`aws.amazon.com/neuron`) resource, but less than all available Neuron cores or devices on a node. From the AWS Neuron documentation:

> The Neuron scheduler extension finds sets of directly connected devices with minimal communication latency when scheduling containers. On Inf1 and Inf2 instance types where Neuron devices are connected through a ring topology, the scheduler finds sets of contiguous devices. For example, for a container requesting 3 Neuron devices the scheduler might assign Neuron devices 0,1,2 to the container if they are available but never devices 0,2,4 because those devices are not directly connected. On Trn1.32xlarge and Trn1n.32xlarge instance types where devices are connected through a 2D torus topology, the Neuron scheduler enforces additional constraints that containers request 1, 4, 8, or all 16 devices. If your container requires a different number of devices, such as 2 or 5, we recommend that you use an Inf2 instance instead of Trn1 to benefit from more advanced topology.

However, Karpenter is not aware of the decisions made by the Neuron scheduler extension which precludes it from making any optimizations to consolidate and bin pack pods requiring Neuron resources. To ensure Karpenter's bin-packing is consistent with the decisions made by the scheduler extension, containers must have like-sized, power of 2 requests (e.g. 1, 2, 4, etc). Failing to do so may result in permanently pending pods.

### Pod ENI Resources (Security Groups for Pods)
[Pod ENI](https://github.com/aws/amazon-vpc-cni-k8s#enable_pod_eni-v170) is a feature of the AWS VPC CNI Plugin which allows an Elastic Network Interface (ENI) to be allocated directly to a Pod. When enabled, the `vpc.amazonaws.com/pod-eni` extended resource is added to supported nodes. The Pod ENI feature can be used independently, but is most often used in conjunction with Security Groups for Pods.  Follow the below instructions to enable support for Pod ENI and/or Security Groups for Pods in Karpenter.

{{% alert title="Note" color="primary" %}}
You must enable Pod ENI support in the AWS VPC CNI Plugin before enabling Pod ENI support in Karpenter.  Please refer to the [Security Groups for Pods documentation](https://docs.aws.amazon.com/eks/latest/userguide/security-groups-for-pods.html) for instructions.
{{% /alert %}}
{{% alert title="Note" color="primary" %}}
If you've enabled [Security Groups per Pod](https://aws.github.io/aws-eks-best-practices/networking/sgpp/), one of the instance's ENIs is reserved. To avoid discrepancies between the `maxPods` value and the node's supported pod density, you need to set [RESERVED_ENIS]({{<ref "../reference/settings" >}})=1.
{{% /alert %}}

Here is an example of a pod-eni resource defined in a deployment manifest:
```
spec:
  template:
    spec:
      containers:
      - resources:
          limits:
            vpc.amazonaws.com/pod-eni: "1"
```

{{% alert title="Windows Support Notice" color="warning" %}}
Security groups for pods are [currently unsupported for Windows nodes](https://docs.aws.amazon.com/eks/latest/userguide/security-groups-for-pods.html)
{{% /alert %}}

## Selecting nodes

With `nodeSelector` you can ask for a node that matches selected key-value pairs.
This can include well-known labels or custom labels you create yourself.

You can use `affinity` to define more complicated constraints, see [Node Affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) for the complete specification.

### Labels
Well-known labels may be specified as NodePool requirements or pod scheduling constraints. You can also define your own custom labels by specifying `requirements` or `labels` on your NodePool and select them using `nodeAffinity` or `nodeSelectors` on your Pods.

{{% alert title="Warning" color="warning" %}}
Take care to ensure the label domains are correct. A well known label like `karpenter.k8s.aws/instance-family` will enforce node properties, but may be confused with `node.kubernetes.io/instance-family`, which is unknown to Karpenter, and treated as a custom label which will not enforce node properties.
{{% /alert %}}

#### Well-Known Labels

| Label                                                          | Example              | Description                                                                                                                                                                                                                               |
| -------------------------------------------------------------- |----------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| topology.kubernetes.io/zone                                    | us-east-2a           | Zones are defined by your cloud provider ([aws](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html))                                                                                               |
| node.kubernetes.io/instance-type                               | g4dn.8xlarge         | Instance types are defined by your cloud provider ([aws](https://aws.amazon.com/ec2/instance-types/))                                                                                                                                     |
| node.kubernetes.io/windows-build                               | 10.0.17763           | Windows OS build in the format "MajorVersion.MinorVersion.BuildNumber". Can be `10.0.17763` for WS2019, or `10.0.20348` for WS2022. ([k8s](https://kubernetes.io/docs/reference/labels-annotations-taints/#nodekubernetesiowindows-build)) |
| kubernetes.io/os                                               | linux                | Operating systems are defined by [GOOS values](https://github.com/golang/go/blob/master/src/internal/syslist/syslist.go) (`KnownOS`) on the instance                                                                                      |
| kubernetes.io/arch                                             | amd64                | Architectures are defined by [GOARCH values](https://github.com/golang/go/blob/master/src/internal/syslist/syslist.go) (`KnownArch`) on the instance                                                                                      |
| karpenter.sh/capacity-type                                     | spot                 | Capacity types include `reserved`, `spot`, and `on-demand`                                                                                                                                                                                |
| karpenter.k8s.aws/capacity-reservation-id                      | cr-56fac701cc1951b03 | [AWS Specific] The capacity reservation ID. Only present on reserved nodes.                                                                                                                                                               |
| karpenter.k8s.aws/capacity-reservation-type                    | default              | [AWS Specific] The capacity reservation type. Can be `default` or `capacity-block`. Only present on reserved nodes.                                                                                                                       |
| karpenter.k8s.aws/instance-hypervisor                          | nitro                | [AWS Specific] Instance types that use a specific hypervisor                                                                                                                                                                              |
| karpenter.k8s.aws/instance-encryption-in-transit-supported     | true                 | [AWS Specific] Instance types that support (or not) in-transit encryption                                                                                                                                                                 |
| karpenter.k8s.aws/instance-category                            | g                    | [AWS Specific] Instance types of the same category, usually the string before the generation number                                                                                                                                       |
| karpenter.k8s.aws/instance-generation                          | 4                    | [AWS Specific] Instance type generation number within an instance category                                                                                                                                                                |
| karpenter.k8s.aws/instance-family                              | g4dn                 | [AWS Specific] Instance types of similar properties but different resource quantities                                                                                                                                                     |
| karpenter.k8s.aws/instance-size                                | 8xlarge              | [AWS Specific] Instance types of similar resource quantities but different properties                                                                                                                                                     |
| karpenter.k8s.aws/instance-cpu                                 | 32                   | [AWS Specific] Number of CPUs on the instance                                                                                                                                                                                             |
| karpenter.k8s.aws/instance-cpu-manufacturer                    | aws                  | [AWS Specific] Name of the CPU manufacturer                                                                                                                                                                                               |
| karpenter.k8s.aws/instance-cpu-sustained-clock-speed-mhz       | 3600                 | [AWS Specific] The CPU clock speed, in MHz                                                                                                                                                                                                |
| karpenter.k8s.aws/instance-memory                              | 131072               | [AWS Specific] Number of mebibytes of memory on the instance                                                                                                                                                                              |
| karpenter.k8s.aws/instance-ebs-bandwidth                       | 9500                 | [AWS Specific] Number of [maximum megabits](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-optimized.html#ebs-optimization-performance) of EBS available on the instance                                                         |
| karpenter.k8s.aws/instance-network-bandwidth                   | 131072               | [AWS Specific] Number of [baseline megabits](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-network-bandwidth.html) available on the instance                                                                           |
| karpenter.k8s.aws/instance-pods                                | 110                  | [AWS Specific] Number of pods the instance supports                                                                                                                                                                                       |
| karpenter.k8s.aws/instance-gpu-name                            | t4                   | [AWS Specific] Name of the GPU on the instance, if available                                                                                                                                                                              |
| karpenter.k8s.aws/instance-gpu-manufacturer                    | nvidia               | [AWS Specific] Name of the GPU manufacturer                                                                                                                                                                                               |
| karpenter.k8s.aws/instance-gpu-count                           | 1                    | [AWS Specific] Number of GPUs on the instance                                                                                                                                                                                             |
| karpenter.k8s.aws/instance-gpu-memory                          | 16384                | [AWS Specific] Number of mebibytes of memory on the GPU                                                                                                                                                                                   |
| karpenter.k8s.aws/instance-local-nvme                          | 900                  | [AWS Specific] Number of gibibytes of local nvme storage on the instance                                                                                                                                                                  |
| karpenter.k8s.aws/instance-capability-flex                     | true                 | [AWS Specific] Instance with capacity flex                                                                                                                                                                                                |
| karpenter.k8s.aws/tenancy                                      | default              | [AWS Specific] Tenancy types include `default`, and `dedicated`                                                                                                                                                                        |

{{% alert title="Note" color="primary" %}}
Karpenter translates the following deprecated labels to their stable equivalents: `failure-domain.beta.kubernetes.io/zone`, `failure-domain.beta.kubernetes.io/region`, `beta.kubernetes.io/arch`, `beta.kubernetes.io/os`, and `beta.kubernetes.io/instance-type`.
{{% /alert %}}

#### User-Defined Labels

Karpenter is aware of several well-known labels, deriving them from instance type details. If you specify a `nodeSelector` or a required `nodeAffinity` using a label that is not well-known to Karpenter, it will not launch nodes with these labels and pods will remain pending. For Karpenter to become aware that it can schedule for these labels, you must specify the label in the NodePool requirements with the `Exists` operator:

```yaml
requirements:
  - key: user.defined.label/type
    operator: Exists
```

{{% alert title="Note" color="primary" %}}
There is currently a limit of 100 on the total number of requirements on both the NodePool and the NodeClaim. It's important to note that `spec.template.metadata.labels` are also propagated as requirements on the NodeClaim when it's created, meaning that you can't have more than 100 requirements and labels combined set on your NodePool.
{{% /alert %}}

#### Node selectors

Here is an example of a `nodeSelector` for selecting nodes:

```yaml
nodeSelector:
  topology.kubernetes.io/zone: us-west-2a
  karpenter.sh/capacity-type: spot
```
This example features a well-known label (`topology.kubernetes.io/zone`) and a label that is well known to Karpenter (`karpenter.sh/capacity-type`).

If you want to create a custom label, you should do that at the NodePool level.
Then the pod can declare that custom label.


See [nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector) in the Kubernetes documentation for details.

## Preferences

Karpenter is aware of preferences (node affinity, pod affinity, pod anti-affinity, and pod topology) and treats them as requirements in most circumstances. Karpenter uses these preferences when determining if a pod can schedule on a node (absent topology requirements), or when determining if a pod can be shifted to a new node.

Karpenter starts by treating preferred affinities as required affinities when constructing requirements for a pod. When these requirements cannot be met, the pod's preferences are relaxed one-at-a-time by ascending weight (lowest weight is relaxed first), and the remaining requirements are tried again.

{{% alert title="Warning" color="warning" %}}
Karpenter does not interpret preferred affinities as required when constructing topology requirements for scheduling to a node. If these preferences are necessary, required affinities should be used [as documented in Node Affinity](#node-affinity).
{{% /alert %}}

### Node affinity

Examples below illustrate how to use Node affinity to include (`In`) and exclude (`NotIn`) objects.
See [Node affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) for details.
When setting rules, the following Node affinity types define how hard or soft each rule is:

* **requiredDuringSchedulingIgnoredDuringExecution**: This is a hard rule that must be met.
* **preferredDuringSchedulingIgnoredDuringExecution**: This is a preference, but the pod can run on a node where it is not guaranteed.

{{% alert title="Note" color="primary" %}}
Preferred affinities on pods can result in more nodes being created than expected because Karpenter will prefer to create new nodes to satisfy preferences, [see the preferences documentation](#preferences) for details.
{{% /alert %}}

The `IgnoredDuringExecution` part of each tells the pod to keep running, even if conditions change on the node so the rules no longer matched.
You can think of these concepts as `required` and `preferred`, since Kubernetes never implemented other variants of these rules.

All examples below assume that the NodePool doesn't have constraints to prevent those zones from being used. The first constraint says you could use `us-west-2a` or `us-west-2b`, the second constraint makes it so only `us-west-2b` can be used.

```yaml
 affinity:
   nodeAffinity:
     requiredDuringSchedulingIgnoredDuringExecution:
       nodeSelectorTerms:
         - matchExpressions:
           - key: "topology.kubernetes.io/zone"
             operator: "In"
             values: ["us-west-2a", "us-west-2b"]
           - key: "topology.kubernetes.io/zone"
             operator: "In"
             values: ["us-west-2b"]
```

Changing the second operator to `NotIn` would allow the pod to run in `us-west-2a` only:

```yaml
           - key: "topology.kubernetes.io/zone"
             operator: "In"
             values: ["us-west-2a", "us-west-2b"]
           - key: "topology.kubernetes.io/zone"
             operator: "NotIn"
             values: ["us-west-2b"]
```

Continuing to add to the example, `nodeAffinity` lets you define terms so if one term doesn't work it goes to the next one.
Here, if `us-west-2a` is not available, the second term will cause the pod to run on a spot instance in `us-west-2d`.


```yaml
 affinity:
   nodeAffinity:
     requiredDuringSchedulingIgnoredDuringExecution:
       nodeSelectorTerms:
         - matchExpressions: # OR
           - key: "topology.kubernetes.io/zone" # AND
             operator: "In"
             values: ["us-west-2a", "us-west-2b"]
           - key: "topology.kubernetes.io/zone" # AND
             operator: "NotIn"
             values: ["us-west-2b"]
         - matchExpressions: # OR
           - key: "karpenter.sh/capacity-type" # AND
             operator: "In"
             values: ["spot"]
           - key: "topology.kubernetes.io/zone" # AND
             operator: "In"
             values: ["us-west-2d"]
```
In general, Karpenter will go through each of the `nodeSelectorTerms` in order and take the first one that works.
However, if Karpenter fails to provision on the first `nodeSelectorTerms`, it will try again using the second one.
If they all fail, Karpenter will fail to provision the pod.
Karpenter will backoff and retry over time.
So if capacity becomes available, it will schedule the pod without user intervention.

### Taints and tolerations

Taints are the opposite of affinity.
Setting a taint on a node tells the scheduler to not run a pod on it unless the pod has explicitly said it can tolerate that taint. This example shows a NodePool that was set up with a taint for only running pods that require a GPU, such as the following:

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: gpu
spec:
  template:
    spec:
      requirements:
      - key: karpenter.k8s.aws/instance-family
        operator: In
        values:
          - p3
      taints:
      - key: nvidia.com/gpu
        value: "true"
        effect: "NoSchedule"
```

For a pod to request to run on a node that has this NodePool, it could set a toleration as follows:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: mygpupod
spec:
  containers:
  - name: gpuapp
    resources:
      requests:
        nvidia.com/gpu: 1
      limits:
        nvidia.com/gpu: 1
    image: mygpucontainer
  tolerations:
  - key: "nvidia.com/gpu"
    operator: "Exists"
    effect: "NoSchedule"
```
See [Taints and Tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) in the Kubernetes documentation for details.

### Topology Spread

By using the Kubernetes `topologySpreadConstraints` you can ask the NodePool to have pods push away from each other to limit the blast radius of an outage. Think of it as the Kubernetes evolution for pod affinity: it lets you relate pods with respect to nodes while still allowing spread.

{{% alert title="Note" color="primary" %}}
Preferred topology spread (`ScheduleAnyway`) can result in more nodes being created than expected because Karpenter will prefer to create new nodes to satisfy spread constraints, [see the preferences documentation](#preferences) for details.
{{% /alert %}}

For example:

```yaml
spec:
  topologySpreadConstraints:
    - maxSkew: 1
      topologyKey: "topology.kubernetes.io/zone"
      whenUnsatisfiable: ScheduleAnyway
      labelSelector:
        matchLabels:
          dev: jjones
    - maxSkew: 1
      topologyKey: "kubernetes.io/hostname"
      whenUnsatisfiable: ScheduleAnyway
      labelSelector:
        matchLabels:
          dev: jjones
    - maxSkew: 1
      topologyKey: "karpenter.sh/capacity-type"
      whenUnsatisfiable: ScheduleAnyway
      labelSelector:
        matchLabels:
          dev: jjones

```
Adding this to your podspec would result in:

* Pods being spread across zones, hosts, and capacity-type (`topologyKey`).
* The `dev` `labelSelector` will include all pods with the label of `dev=jjones` in topology calculations. It is recommended to use a selector to match all pods in a deployment.
* No more than one pod difference in the number of pods on each host (`maxSkew`).
For example, if there were three nodes and five pods the pods could be spread 1, 2, 2 or 2, 1, 2 and so on.
If instead the maxSkew were 5, pods could be spread 5, 0, 0 or 3, 2, 0, or 2, 1, 2 and so on.

The three supported `topologyKey` values that Karpenter supports are:
- `topology.kubernetes.io/zone`
- `kubernetes.io/hostname`
- `karpenter.sh/capacity-type`

See [Pod Topology Spread Constraints](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/) for details.

{{% alert title="Note" color="primary" %}}
NodePools do not attempt to balance or rebalance the availability zones for their nodes. Availability zone balancing may be achieved by defining zonal Topology Spread Constraints for Pods that require multi-zone durability, and NodePools will respect these constraints while optimizing for compute costs.
{{% /alert %}}

### Pod affinity/anti-affinity

By using the `podAffinity` and `podAntiAffinity` configuration on a pod spec, you can inform the Karpenter scheduler of your desire for pods to schedule together or apart with respect to different topology domains.

{{% alert title="Note" color="primary" %}}
Preferred affinities on pods can result in more nodes being created than expected because Karpenter will prefer to create new nodes to satisfy preferences, [see the preferences documentation](#preferences) for details.
{{% /alert %}}

For example:

```yaml
spec:
  affinity:
    podAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchExpressions:
          - key: system
            operator: In
            values:
            - backend
        topologyKey: topology.kubernetes.io/zone
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            app: inflate
        topologyKey: kubernetes.io/hostname
```

The above pod affinity rule would cause the pod to only schedule in zones where a pod with the label `system=backend` is already running.

The anti-affinity rule would cause it to avoid running on any node with a pod labeled `app=inflate`.  If this anti-affinity term was on a deployment pod spec along with a matching `app=inflate` label, it would prevent more than one pod from the deployment from running on any single node.

See [Inter-pod affinity and anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity) in the Kubernetes documentation for details.

### Persistent Volume Topology

Karpenter automatically detects storage scheduling requirements and includes them in node launch decisions.

In the following example, the `StorageClass` defines zonal topologies for `us-west-2a` and `us-west-2b` and [binding mode `WaitForFirstConsumer`](https://kubernetes.io/docs/concepts/storage/storage-classes/#volume-binding-mode).
When the pod is created, Karpenter follows references from the `Pod` to `PersistentVolumeClaim` to `StorageClass` and identifies that this pod requires storage in `us-west-2a` and `us-west-2b`.
It randomly selects `us-west-2a`, provisions a node in that zone, and waits for kube-scheduler to bind the pod to the node.
The CSI driver creates a `PersistentVolume` according to the `PersistentVolumeClaim` and gives it a node affinity rule for `us-west-2a`.

Later on, the pod is deleted and a new pod is created that requests the same claim. This time, Karpenter identifies that a `PersistentVolume` already exists for the `PersistentVolumeClaim`, and includes its zone `us-west-2a` in the pod's scheduling requirements.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: app
spec:
  containers: ...
  volumes:
    - name: storage
      persistentVolumeClaim:
        claimName: ebs-claim
---
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: ebs
provisioner: ebs.csi.aws.com
volumeBindingMode: WaitForFirstConsumer
allowedTopologies:
- matchLabelExpressions:
  - key: topology.ebs.csi.aws.com/zone
    values: ["us-west-2a", "us-west-2b"]
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ebs-claim
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: ebs
  resources:
    requests:
      storage: 4Gi
```

{{% alert title="Note" color="primary" %}}
☁️ AWS Specific

The EBS CSI driver uses `topology.ebs.csi.aws.com/zone` instead of the standard `topology.kubernetes.io/zone` label. Karpenter is aware of label aliasing and translates this label into `topology.kubernetes.io/zone` in memory. When configuring a `StorageClass` for the EBS CSI Driver, you must use `topology.ebs.csi.aws.com/zone`.
{{% /alert %}}

{{% alert title="Note" color="primary" %}}
The topology key `topology.kubernetes.io/region` is not supported. Legacy in-tree CSI providers specify this label. Instead, install an out-of-tree CSI provider. [Learn more about moving to CSI providers.](https://kubernetes.io/blog/2021/12/10/storage-in-tree-to-csi-migration-status-update/#quick-recap-what-is-csi-migration-and-why-migrate)
{{% /alert %}}

## Weighted NodePools

Karpenter allows you to order your NodePools using the `.spec.weight` field so that the Karpenter scheduler will attempt to schedule one NodePool before another.

### Savings Plans and Reserved Instances

If you have purchased a [Savings Plan](https://aws.amazon.com/savingsplans/) or [Reserved Instances](https://aws.amazon.com/ec2/pricing/reserved-instances/), you may want to tell Karpenter to prioritize this reserved capacity ahead of other instance types.

To enable this, you will need to tell the Karpenter controllers which instance types to prioritize and what is the maximum amount of capacity that should be provisioned using those instance types. We can set the `.spec.limits` field on the NodePool to limit the capacity that can be launched by this NodePool. Combined with the `.spec.weight` value, we can tell Karpenter to pull from instance types in the reserved NodePool before defaulting to generic instance types.

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: reserved-instance
spec:
  weight: 50
  limits:
    cpu: 100
  template:
    spec:
      requirements:
      - key: "node.kubernetes.io/instance-type"
        operator: In
        values: ["c4.large"]
---
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: default
spec:
  template:
    spec:
      requirements:
      - key: karpenter.sh/capacity-type
        operator: In
        values: ["spot", "on-demand"]
      - key: kubernetes.io/arch
        operator: In
        values: ["amd64"]
```

### Fallback

Pods that do not specify node selectors or affinities can potentially be assigned to any node with any configuration. There may be cases where you require these pods to schedule to a specific capacity type or architecture but assigning the relevant node selectors or affinities to all these workload pods may be too tedious or infeasible. Instead, we want to define a cluster-wide default configuration for nodes launched using Karpenter.

By assigning a higher `.spec.weight` value and restricting a NodePool to a specific capacity type or architecture, we can set default configuration for the nodes launched by pods that don't have node configuration restrictions.

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: default
spec:
  weight: 50
  template:
    spec:
      requirements:
      - key: karpenter.sh/capacity-type
        operator: In
        values: ["spot", "on-demand"]
      - key: kubernetes.io/arch
        operator: In
        values: ["amd64"]
---
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: arm64-specific
spec:
  template:
    spec:
      requirements:
      - key: karpenter.sh/capacity-type
        operator: In
        values: ["spot", "on-demand"]
      - key: kubernetes.io/arch
        operator: In
        values: ["arm64"]
      - key: node.kubernetes.io/instance-type
        operator: In
        values: ["a1.large", "a1.xlarge"]
```

{{% alert title="Note" color="primary" %}}
Based on the way that Karpenter performs pod batching and bin packing, it is not guaranteed that Karpenter will always choose the highest priority NodePool given specific requirements. For example, if a pod can't be scheduled with the highest priority NodePool, it will force creation of a node using a lower priority NodePool, allowing other pods from that batch to also schedule on that node. The behavior may also occur if existing capacity is available, as the kube-scheduler will schedule the pods instead of allowing Karpenter to provision a new node.
{{% /alert %}}

## Advanced Scheduling Techniques

### Scheduling based on Node Resources

You may want pods to be able to request resources of nodes that Kubernetes natively does not provide as a schedulable resource or that are aspects of certain nodes like
High Performance Networking or NVME Local Storage. You can use Karpenter's Well-Known Labels to accomplish this.

These can further be applied at the NodePool or Workload level using Requirements, NodeSelectors or Affinities

Pod example of requiring any NVME disk:
```yaml
...
 affinity:
   nodeAffinity:
     requiredDuringSchedulingIgnoredDuringExecution:
       nodeSelectorTerms:
         - matchExpressions:
           - key: "karpenter.k8s.aws/instance-local-nvme"
             operator: "Exists"
...
```

NodePool Example:
```yaml
...
requirement:
  - key: "karpenter.k8s.aws/instance-local-nvme"
    operator: "Exists"
...
```

Pod example of requiring at least 100GB of NVME disk:
```yaml
...
 affinity:
   nodeAffinity:
     requiredDuringSchedulingIgnoredDuringExecution:
       nodeSelectorTerms:
         - matchExpressions:
            - key: "karpenter.k8s.aws/instance-local-nvme"
              operator: Gt
              values: ["99"]
...
```

NodePool Example:
```yaml
...
requirement:
  - key: "karpenter.k8s.aws/instance-local-nvme"
    operator: Gt
    values: ["99"]
...
```

{{% alert title="Note" color="primary" %}}
Karpenter cannot yet take into account ephemeral-storage requests while scheduling pods, we're purely requesting attributes of nodes and getting X amount of resources
as a side effect. You may need to tweak schedulable resources like CPU or Memory to achieve desired fit, especially if Consolidation is enabled.

Your NodeClass will also need to support automatically formatting and mounting NVME Instance Storage if available.
{{% /alert %}}

Pod example of requiring at least 50 Gbps of network bandwidth:
```yaml
...
 affinity:
   nodeAffinity:
     requiredDuringSchedulingIgnoredDuringExecution:
       nodeSelectorTerms:
         - matchExpressions:
            - key: "karpenter.k8s.aws/instance-network-bandwidth"
              operator: Gt
              values: ["49999"]
...
```

NodePool Example:
```yaml
...
requirement:
  - key: "karpenter.k8s.aws/instance-network-bandwidth"
    operator: Gt
    values: ["49999"]
...
```

{{% alert title="Note" color="primary" %}}
If using Gt/Lt operators, make sure to use values under the actual label values of the desired resource.
{{% /alert %}}

### `Exists` Operator

The `Exists` operator can be used on a NodePool to provide workload segregation across nodes.

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
spec:
  template:
    spec:
      requirements:
        - key: company.com/team
          operator: Exists
...
```

With this requirement on the NodePool, workloads can specify the same key (e.g. `company.com/team`) with custom values (e.g. `team-a`, `team-b`, etc.) as a required `nodeAffinity` or `nodeSelector`. Karpenter will then apply the key/value pair to nodes it launches dynamically based on the pod's node requirements.

If each set of pods that can schedule with this NodePool specifies this key in its `nodeAffinity` or `nodeSelector`, you can isolate pods onto different nodes based on their values. This provides a way to more dynamically isolate workloads without requiring a unique NodePool for each workload subset.

For example, providing the following `nodeSelectors` would isolate the pods for each of these deployments on different nodes.

#### Team A Deployment

```yaml
apiVersion: v1
kind: Deployment
metadata:
  name: team-a-deployment
spec:
  replicas: 5
  template:
    spec:
      nodeSelector:
        company.com/team: team-a
```

#### Team A Node

```yaml
apiVersion: v1
kind: Node
metadata:
  labels:
    company.com/team: team-a
```

#### Team B Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: team-b-deployment
spec:
  replicas: 5
  template:
    spec:
      nodeSelector:
        company.com/team: team-b
```

#### Team B Node

```yaml
apiVersion: v1
kind: Node
metadata:
  labels:
    company.com/team: team-b
```

{{% alert title="Note" color="primary" %}}
If a workload matches the NodePool but doesn't specify a label, Karpenter will generate a random label for the node.
{{% /alert %}}

### On-Demand/Spot Ratio Split

Taking advantage of Karpenter's ability to assign labels to node and using a topology spread across those labels enables a crude method for splitting a workload across on-demand and spot instances in a desired ratio.

To do this, we create one NodePool each for spot and on-demand with disjoint values for a unique new label called `capacity-spread`.  In the example below, we provide four unique values for the spot NodePool and one value for the on-demand NodePool.  When we spread across our new label evenly, we'll end up with a ratio of 4:1 spot to on-demand nodes.

{{% alert title="Warning" color="warning" %}}
This is not identical to a topology spread with a specified ratio.  We are constructing 'virtual domains' to spread evenly across and the ratio of those 'virtual domains' to spot and on-demand happen to coincide with the desired spot to on-demand ratio.  As an example, if you launch pods using the provided example, Karpenter will launch nodes with `capacity-spread` labels of 1, 2, 3, 4, and 5. `kube-scheduler` will then schedule evenly across those nodes to give the desired ratio.
{{% /alert %}}

#### NodePools

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: spot
spec:
  template:
    spec:
      requirements:
      - key: "karpenter.sh/capacity-type"
        operator: In
        values: ["spot"]
      - key: capacity-spread
        operator: In
        values:
        - "2"
        - "3"
        - "4"
        - "5"
---
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: on-demand
spec:
  template:
    spec:
      requirements:
      - key: "karpenter.sh/capacity-type"
        operator: In
        values: ["on-demand"]
      - key: capacity-spread
        operator: In
        values:
        - "1"
```

#### Workload Topology Spread Constraint

```yaml
topologySpreadConstraints:
- maxSkew: 1
  topologyKey: capacity-spread
  whenUnsatisfiable: DoNotSchedule
  labelSelector:
    ...
```
