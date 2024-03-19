---
title: "NodePools"
linkTitle: "NodePools"
weight: 1
description: >
  Configure Karpenter with NodePools
---

When you first installed Karpenter, you set up a default NodePool. The NodePool sets constraints on the nodes that can be created by Karpenter and the pods that can run on those nodes. The NodePool can be set to do things like:

* Define taints to limit the pods that can run on nodes Karpenter creates
* Define any startup taints to inform Karpenter that it should taint the node initially, but that the taint is temporary.
* Limit node creation to certain zones, instance types, and computer architectures
* Set defaults for node expiration

You can change your NodePool or add other NodePools to Karpenter.
Here are things you should know about NodePools:

* Karpenter won't do anything if there is not at least one NodePool configured.
* Each NodePool that is configured is looped through by Karpenter.
* If Karpenter encounters a taint in the NodePool that is not tolerated by a Pod, Karpenter won't use that NodePool to provision the pod.
* If Karpenter encounters a startup taint in the NodePool it will be applied to nodes that are provisioned, but pods do not need to tolerate the taint.  Karpenter assumes that the taint is temporary and some other system will remove the taint.
* It is recommended to create NodePools that are mutually exclusive. So no Pod should match multiple NodePools. If multiple NodePools are matched, Karpenter will use the NodePool with the highest [weight](#specweight).

For some example `NodePool` configurations, see the [examples in the Karpenter GitHub repository](https://github.com/aws/karpenter/blob/main/examples/v1beta1/).

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: default
spec:
  # Template section that describes how to template out NodeClaim resources that Karpenter will provision
  # Karpenter will consider this template to be the minimum requirements needed to provision a Node using this NodePool
  # It will overlay this NodePool with Pods that need to schedule to further constrain the NodeClaims
  # Karpenter will provision to launch new Nodes for the cluster
  template:
    metadata:
      # Labels are arbitrary key-values that are applied to all nodes
      labels:
        billing-team: my-team

      # Annotations are arbitrary key-values that are applied to all nodes
      annotations:
        example.com/owner: "my-team"
    spec:
      # References the Cloud Provider's NodeClass resource, see your cloud provider specific documentation
      nodeClassRef:
        apiVersion: karpenter.k8s.aws/v1beta1
        kind: EC2NodeClass
        name: default

      # Provisioned nodes will have these taints
      # Taints may prevent pods from scheduling if they are not tolerated by the pod.
      taints:
        - key: example.com/special-taint
          effect: NoSchedule

      # Provisioned nodes will have these taints, but pods do not need to tolerate these taints to be provisioned by this
      # NodePool. These taints are expected to be temporary and some other entity (e.g. a DaemonSet) is responsible for
      # removing the taint after it has finished initializing the node.
      startupTaints:
        - key: example.com/another-taint
          effect: NoSchedule

      # Requirements that constrain the parameters of provisioned nodes.
      # These requirements are combined with pod.spec.topologySpreadConstraints, pod.spec.affinity.nodeAffinity, pod.spec.affinity.podAffinity, and pod.spec.nodeSelector rules.
      # Operators { In, NotIn, Exists, DoesNotExist, Gt, and Lt } are supported.
      # https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#operators
      requirements:
        - key: "karpenter.k8s.aws/instance-category"
          operator: In
          values: ["c", "m", "r"]
          # minValues here enforces the scheduler to consider at least that number of unique instance-category to schedule the pods.
          # This field is ALPHA and can be dropped or replaced at any time 
          minValues: 2
        - key: "karpenter.k8s.aws/instance-family"
          operator: In
          values: ["m5","m5d","c5","c5d","c4","r4"]
          minValues: 5
        - key: "karpenter.k8s.aws/instance-cpu"
          operator: In
          values: ["4", "8", "16", "32"]
        - key: "karpenter.k8s.aws/instance-hypervisor"
          operator: In
          values: ["nitro"]
        - key: "karpenter.k8s.aws/instance-generation"
          operator: Gt
          values: ["2"]
        - key: "topology.kubernetes.io/zone"
          operator: In
          values: ["us-west-2a", "us-west-2b"]
        - key: "kubernetes.io/arch"
          operator: In
          values: ["arm64", "amd64"]
        - key: "karpenter.sh/capacity-type"
          operator: In
          values: ["spot", "on-demand"]

      # Karpenter provides the ability to specify a few additional Kubelet args.
      # These are all optional and provide support for additional customization and use cases.
      kubelet:
        clusterDNS: ["10.0.1.100"]
        systemReserved:
          cpu: 100m
          memory: 100Mi
          ephemeral-storage: 1Gi
        kubeReserved:
          cpu: 200m
          memory: 100Mi
          ephemeral-storage: 3Gi
        evictionHard:
          memory.available: 5%
          nodefs.available: 10%
          nodefs.inodesFree: 10%
        evictionSoft:
          memory.available: 500Mi
          nodefs.available: 15%
          nodefs.inodesFree: 15%
        evictionSoftGracePeriod:
          memory.available: 1m
          nodefs.available: 1m30s
          nodefs.inodesFree: 2m
        evictionMaxPodGracePeriod: 60
        imageGCHighThresholdPercent: 85
        imageGCLowThresholdPercent: 80
        cpuCFSQuota: true
        podsPerCore: 2
        maxPods: 20

  # Disruption section which describes the ways in which Karpenter can disrupt and replace Nodes
  # Configuration in this section constrains how aggressive Karpenter can be with performing operations
  # like rolling Nodes due to them hitting their maximum lifetime (expiry) or scaling down nodes to reduce cluster cost
  disruption:
    # Describes which types of Nodes Karpenter should consider for consolidation
    # If using 'WhenUnderutilized', Karpenter will consider all nodes for consolidation and attempt to remove or replace Nodes when it discovers that the Node is underutilized and could be changed to reduce cost
    # If using `WhenEmpty`, Karpenter will only consider nodes for consolidation that contain no workload pods
    consolidationPolicy: WhenUnderutilized | WhenEmpty

    # The amount of time Karpenter should wait after discovering a consolidation decision
    # This value can currently only be set when the consolidationPolicy is 'WhenEmpty'
    # You can choose to disable consolidation entirely by setting the string value 'Never' here
    consolidateAfter: 30s

    # The amount of time a Node can live on the cluster before being removed
    # Avoiding long-running Nodes helps to reduce security vulnerabilities as well as to reduce the chance of issues that can plague Nodes with long uptimes such as file fragmentation or memory leaks from system processes
    # You can choose to disable expiration entirely by setting the string value 'Never' here
    expireAfter: 720h

    # Budgets control the speed Karpenter can scale down nodes.
    # Karpenter will respect the minimum of the currently active budgets, and will round up
    # when considering percentages. Duration and Schedule must be set together.
    budgets:
    - nodes: 10%
    # On Weekdays during business hours, don't do any deprovisioning.
    - schedule: "0 9 * * mon-fri"
      duration: 8h
      nodes: "0"

  # Resource limits constrain the total size of the cluster.
  # Limits prevent Karpenter from creating new instances once the limit is exceeded.
  limits:
    cpu: "1000"
    memory: 1000Gi

  # Priority given to the NodePool when the scheduler considers which NodePool
  # to select. Higher weights indicate higher priority when comparing NodePools.
  # Specifying no weight is equivalent to specifying a weight of 0.
  weight: 10
```

## spec.template.spec.requirements

Kubernetes defines the following [Well-Known Labels](https://kubernetes.io/docs/reference/labels-annotations-taints/), and cloud providers (e.g., AWS) implement them. They are defined at the "spec.requirements" section of the NodePool API.

In addition to the well-known labels from Kubernetes, Karpenter supports AWS-specific labels for more advanced scheduling. See the full list [here](../scheduling/#well-known-labels).

These well-known labels may be specified at the NodePool level, or in a workload definition (e.g., nodeSelector on a pod.spec). Nodes are chosen using both the NodePool's and pod's requirements. If there is no overlap, nodes will not be launched. In other words, a pod's requirements must be within the NodePool's requirements. If a requirement is not defined for a well known label, any value available to the cloud provider may be chosen.

For example, an instance type may be specified using a nodeSelector in a pod spec. If the instance type requested is not included in the NodePool list and the NodePool has instance type requirements, Karpenter will not create a node or schedule the pod.

### Well-Known Labels

#### Instance Types

- key: `node.kubernetes.io/instance-type`
- key: `karpenter.k8s.aws/instance-family`
- key: `karpenter.k8s.aws/instance-category`
- key: `karpenter.k8s.aws/instance-generation`

Generally, instance types should be a list and not a single value. Leaving these requirements undefined is recommended, as it maximizes choices for efficiently placing pods.

Review [AWS instance types](../instance-types). Most instance types are supported with the exclusion of [non-HVM](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/virtualization_types.html).

#### Availability Zones

- key: `topology.kubernetes.io/zone`
- value example: `us-east-1c`
- value list: `aws ec2 describe-availability-zones --region <region-name>`

Karpenter can be configured to create nodes in a particular zone. Note that the Availability Zone `us-east-1a` for your AWS account might not have the same location as `us-east-1a` for another AWS account.

[Learn more about Availability Zone
IDs.](https://docs.aws.amazon.com/ram/latest/userguide/working-with-az-ids.html)

#### Architecture

- key: `kubernetes.io/arch`
- values
  - `amd64`
  - `arm64`

Karpenter supports `amd64` nodes, and `arm64` nodes.

#### Operating System
 - key: `kubernetes.io/os`
 - values
   - `linux`
   - `windows`

Karpenter supports `linux` and `windows` operating systems.

#### Capacity Type

- key: `karpenter.sh/capacity-type`
- values
  - `spot`
  - `on-demand`

Karpenter supports specifying capacity type, which is analogous to [EC2 purchase options](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-purchasing-options.html).

Karpenter prioritizes Spot offerings if the NodePool allows Spot and on-demand instances. If the provider API (e.g. EC2 Fleet's API) indicates Spot capacity is unavailable, Karpenter caches that result across all attempts to provision EC2 capacity for that instance type and zone for the next 45 seconds. If there are no other possible offerings available for Spot, Karpenter will attempt to provision on-demand instances, generally within milliseconds.

Karpenter also allows `karpenter.sh/capacity-type` to be used as a topology key for enforcing topology-spread.

### Min Values

Along with the combination of [key,operator,values] in the requirements, Karpenter also supports `minValues` in the NodePool requirements block, allowing the scheduler to be aware of user-specified flexibility minimums while scheduling pods to a cluster. If Karpenter cannot meet this minimum flexibility for each key when scheduling a pod, it will fail the scheduling loop for that NodePool, either falling back to another NodePool which meets the pod requirements or failing scheduling the pod altogether.

For example, the below spec will use spot instance type for all provisioned instances and enforces `minValues` to various keys where it is defined
i.e at least 2 unique instance families from [c,m,r], 5 unique instance families [eg: "m5","m5d","r4","c5","c5d","c4" etc], 10 unique instance types [eg: "c5.2xlarge","c4.xlarge" etc] is required for scheduling the pods.

```yaml
spec:
  template:
    spec:
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values: ["amd64"]
        - key: kubernetes.io/os
          operator: In
          values: ["linux"]
        - key: karpenter.k8s.aws/instance-category
          operator: In
          values: ["c", "m", "r"]
          minValues: 2
        - key: karpenter.k8s.aws/instance-family
          operator: Exists
          minValues: 5
        - key: node.kubernetes.io/instance-type
          operator: Exists
          minValues: 10
        - key: karpenter.k8s.aws/instance-generation
          operator: Gt
          values: ["2"]
```

Note that `minValues` can be used with multiple operators and multiple requirements. And if the `minValues` are defined with multiple operators for the same requirement key, scheduler considers the max of all the `minValues` for that requirement. For example, the below spec requires scheduler to consider at least 5 instance-family to schedule the pods.

```yaml
spec:
  template:
    spec:
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values: ["amd64"]
        - key: kubernetes.io/os
          operator: In
          values: ["linux"]
        - key: karpenter.k8s.aws/instance-category
          operator: In
          values: ["c", "m", "r"]
          minValues: 2
        - key: karpenter.k8s.aws/instance-family
          operator: Exists
          minValues: 5
        - key: karpenter.k8s.aws/instance-family
          operator: In
          values: ["m5","m5d","c5","c5d","c4","r4"]
          minValues: 3
        - key: node.kubernetes.io/instance-type
          operator: Exists
          minValues: 10
        - key: karpenter.k8s.aws/instance-generation
          operator: Gt
          values: ["2"]
```

{{% alert title="Recommended" color="primary" %}}
Karpenter allows you to be extremely flexible with your NodePools by only constraining your instance types in ways that are absolutely necessary for your cluster. By default, Karpenter will enforce that you specify the `spec.template.spec.requirements` field, but will not enforce that you specify any requirements within the field. If you choose to specify `requirements: []`, this means that you will completely flexible to _all_ instance types that your cloud provider supports.

Though Karpenter doesn't enforce these defaults, for most use-cases, we recommend that you specify _some_ requirements to avoid odd behavior or exotic instance types. Below, is a high-level recommendation for requirements that should fit the majority of use-cases for generic workloads

```yaml
spec:
  template:
    spec:
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values: ["amd64"]
        - key: kubernetes.io/os
          operator: In
          values: ["linux"]
        - key: karpenter.sh/capacity-type
          operator: In
          values: ["on-demand"]
        - key: karpenter.k8s.aws/instance-category
          operator: In
          values: ["c", "m", "r"]
        - key: karpenter.k8s.aws/instance-generation
          operator: Gt
          values: ["2"]
```

{{% /alert %}}

## spec.template.spec.nodeClassRef

This field points to the Cloud Provider NodeClass resource. Learn more about [EC2NodeClasses]({{<ref "nodeclasses" >}}).

## spec.template.spec.kubelet

Karpenter provides the ability to specify a few additional Kubelet args. These are all optional and provide support for
additional customization and use cases. Adjust these only if you know you need to do so. For more details on kubelet configuration arguments, [see the KubeletConfiguration API specification docs](https://kubernetes.io/docs/reference/config-api/kubelet-config.v1beta1/). The implemented fields are a subset of the full list of upstream kubelet configuration arguments. Please cut an issue if you'd like to see another field implemented.

```yaml
kubelet:
  clusterDNS: ["10.0.1.100"]
  systemReserved:
    cpu: 100m
    memory: 100Mi
    ephemeral-storage: 1Gi
  kubeReserved:
    cpu: 200m
    memory: 100Mi
    ephemeral-storage: 3Gi
  evictionHard:
    memory.available: 5%
    nodefs.available: 10%
    nodefs.inodesFree: 10%
  evictionSoft:
    memory.available: 500Mi
    nodefs.available: 15%
    nodefs.inodesFree: 15%
  evictionSoftGracePeriod:
    memory.available: 1m
    nodefs.available: 1m30s
    nodefs.inodesFree: 2m
  evictionMaxPodGracePeriod: 60
  imageGCHighThresholdPercent: 85
  imageGCLowThresholdPercent: 80
  cpuCFSQuota: true
  podsPerCore: 2
  maxPods: 20
```

### Reserved Resources

Karpenter will automatically configure the system and kube reserved resource requests on the fly on your behalf. These requests are used to configure your node and to make scheduling decisions for your pods. If you have specific requirements or know that you will have additional capacity requirements, you can optionally override the `--system-reserved` configuration defaults with the `.spec.template.spec.kubelet.systemReserved` values and the `--kube-reserved` configuration defaults with the `.spec.template.spec.kubelet.kubeReserved` values.

{{% alert title="Note" color="primary" %}}
Karpenter considers these reserved resources when computing the allocatable ephemeral storage on a given instance type.
If `kubeReserved` is not specified, Karpenter will compute the default reserved [CPU](https://github.com/awslabs/amazon-eks-ami/blob/db28da15d2b696bc08ac3aacc9675694f4a69933/files/bootstrap.sh#L251) and [memory](https://github.com/awslabs/amazon-eks-ami/blob/db28da15d2b696bc08ac3aacc9675694f4a69933/files/bootstrap.sh#L235) resources for the purpose of ephemeral storage computation.
These defaults are based on the defaults on Karpenter's supported AMI families, which are not the same as the kubelet defaults.
You should be aware of the CPU and memory default calculation when using Custom AMI Families. If they don't align, there may be a difference in Karpenter's computed allocatable ephemeral storage and the actually ephemeral storage available on the node.
{{% /alert %}}

### Eviction Thresholds

The kubelet supports eviction thresholds by default. When enough memory or file system pressure is exerted on the node, the kubelet will begin to evict pods to ensure that system daemons and other system processes can continue to run in a healthy manner.

Kubelet has the notion of [hard evictions](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#hard-eviction-thresholds) and [soft evictions](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#soft-eviction-thresholds). In hard evictions, pods are evicted as soon as a threshold is met, with no grace period to terminate. Soft evictions, on the other hand, provide an opportunity for pods to be terminated gracefully. They do so by sending a termination signal to pods that are planning to be evicted and allowing those pods to terminate up to their grace period.

Karpenter supports [hard evictions](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#hard-eviction-thresholds) through the `.spec.template.spec.kubelet.evictionHard` field and [soft evictions](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#soft-eviction-thresholds) through the `.spec.template.spec.kubelet.evictionSoft` field. `evictionHard` and `evictionSoft` are configured by listing [signal names](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#eviction-signals) with either percentage values or resource values.

```yaml
kubelet:
  evictionHard:
    memory.available: 500Mi
    nodefs.available: 10%
    nodefs.inodesFree: 10%
    imagefs.available: 5%
    imagefs.inodesFree: 5%
    pid.available: 7%
  evictionSoft:
    memory.available: 1Gi
    nodefs.available: 15%
    nodefs.inodesFree: 15%
    imagefs.available: 10%
    imagefs.inodesFree: 10%
    pid.available: 10%
```

#### Supported Eviction Signals

| Eviction Signal    | Description                                                                     |
|--------------------|---------------------------------------------------------------------------------|
| memory.available   | memory.available := node.status.capacity[memory] - node.stats.memory.workingSet |
| nodefs.available   | nodefs.available := node.stats.fs.available                                     |
| nodefs.inodesFree  | nodefs.inodesFree := node.stats.fs.inodesFree                                   |
| imagefs.available  | imagefs.available := node.stats.runtime.imagefs.available                       |
| imagefs.inodesFree | imagefs.inodesFree := node.stats.runtime.imagefs.inodesFree                     |
| pid.available      | pid.available := node.stats.rlimit.maxpid - node.stats.rlimit.curproc           |

For more information on eviction thresholds, view the [Node-pressure Eviction](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction) section of the official Kubernetes docs.

#### Soft Eviction Grace Periods

Soft eviction pairs an eviction threshold with a specified grace period. With soft eviction thresholds, the kubelet will only begin evicting pods when the node exceeds its soft eviction threshold over the entire duration of its grace period. For example, if you specify `evictionSoft[memory.available]` of `500Mi` and a `evictionSoftGracePeriod[memory.available]` of `1m30`, the node must have less than `500Mi` of available memory over a minute and a half in order for the kubelet to begin evicting pods.

Optionally, you can specify an `evictionMaxPodGracePeriod` which defines the administrator-specified maximum pod termination grace period to use during soft eviction. If a namespace-owner had specified a pod `terminationGracePeriodInSeconds` on pods in their namespace, the minimum of `evictionPodGracePeriod` and `terminationGracePeriodInSeconds` would be used.

```yaml
kubelet:
  evictionSoftGracePeriod:
    memory.available: 1m
    nodefs.available: 1m30s
    nodefs.inodesFree: 2m
    imagefs.available: 1m30s
    imagefs.inodesFree: 2m
    pid.available: 2m
  evictionMaxPodGracePeriod: 60
```

### Pod Density

By default, the number of pods on a node is limited by both the number of networking interfaces (ENIs) that may be attached to an instance type and the number of IP addresses that can be assigned to each ENI.  See [IP addresses per network interface per instance type](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-eni.html#AvailableIpPerENI) for a more detailed information on these instance types' limits.

{{% alert title="Note" color="primary" %}}
By default, the VPC CNI allocates IPs for a node and pods from the same subnet. With [VPC CNI Custom Networking](https://aws.github.io/aws-eks-best-practices/networking/custom-networking), the pods will receive IP addresses from another subnet dedicated to pod IPs. This approach makes it easier to manage IP addresses and allows for separate Network Access Control Lists (NACLs) applied to your pods. VPC CNI Custom Networking reduces the pod density of a node since one of the ENI attachments will be used for the node and cannot share the allocated IPs on the interface to pods. Karpenter supports VPC CNI Custom Networking and similar CNI setups where the primary node interface is separated from the pods interfaces through a global [setting](./settings.md#configmap) within the karpenter-global-settings configmap: `aws.reservedENIs`. In the common case, `aws.reservedENIs` should be set to `"1"` if using Custom Networking.
{{% /alert %}}

{{% alert title="Windows Support Notice" color="warning" %}}
It's currently not possible to specify custom networking with Windows nodes.
{{% /alert %}}

#### Max Pods

For small instances that require an increased pod density or large instances that require a reduced pod density, you can override this default value with `.spec.template.spec.kubelet.maxPods`. This value will be used during Karpenter pod scheduling and passed through to `--max-pods` on kubelet startup.

{{% alert title="Note" color="primary" %}}
When using small instance types, it may be necessary to enable [prefix assignment mode](https://aws.amazon.com/blogs/containers/amazon-vpc-cni-increases-pods-per-node-limits/) in the AWS VPC CNI plugin to support a higher pod density per node.  Prefix assignment mode was introduced in AWS VPC CNI v1.9 and allows ENIs to manage a broader set of IP addresses.  Much higher pod densities are supported as a result.
{{% /alert %}}

{{% alert title="Windows Support Notice" color="warning" %}}
Presently, Windows worker nodes do not support using more than one ENI.
As a consequence, the number of IP addresses, and subsequently, the number of pods that a Windows worker node can support is limited by the number of IPv4 addresses available on the primary ENI.
Currently, Karpenter will only consider individual secondary IP addresses when calculating the pod density limit.
{{% /alert %}}

#### Pods Per Core

An alternative way to dynamically set the maximum density of pods on a node is to use the `.spec.template.spec.kubelet.podsPerCore` value. Karpenter will calculate the pod density during scheduling by multiplying this value by the number of logical cores (vCPUs) on an instance type. This value will also be passed through to the `--pods-per-core` value on kubelet startup to configure the number of allocatable pods the kubelet can assign to the node instance.

The value generated from `podsPerCore` cannot exceed `maxPods`, meaning, if both are set, the minimum of the `podsPerCore` dynamic pod density and the static `maxPods` value will be used for scheduling.

{{% alert title="Note" color="primary" %}}
`maxPods` may not be set in the `kubelet` of a NodePool, but may still be restricted by the `ENI_LIMITED_POD_DENSITY` value. You may want to ensure that the `podsPerCore` value that will be used for instance families associated with the NodePool will not cause unexpected behavior by exceeding the `maxPods` value.
{{% /alert %}}

{{% alert title="Pods Per Core on Bottlerocket" color="warning" %}}
Bottlerocket AMIFamily currently does not support `podsPerCore` configuration. If a NodePool contains a `provider` or `providerRef` to a node template that will launch a Bottlerocket instance, the `podsPerCore` value will be ignored for scheduling and for configuring the kubelet.
{{% /alert %}}

## spec.disruption

You can configure Karpenter to disrupt Nodes through your NodePool in multiple ways. You can use `spec.disruption.consolidationPolicy`, `spec.disruption.consolidateAfter` or `spec.disruption.expireAfter`. Read [Disruption]({{<ref "disruption" >}}) for more.

## spec.limits

The NodePool spec includes a limits section (`spec.limits`), which constrains the maximum amount of resources that the NodePool will manage.

Karpenter supports limits of any resource type reported by your cloudprovider. It limits instance types when scheduling to those that will not exceed the specified limits.  If a limit has been exceeded, nodes provisioning is prevented until some nodes have been terminated.

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: default
spec:
  template:
    spec:
      requirements:
        - key: karpenter.sh/capacity-type
          operator: In
          values: ["spot"]
  limits:
    cpu: 1000
    memory: 1000Gi
    nvidia.com/gpu: 2
```

{{% alert title="Note" color="primary" %}}
Karpenter provisioning is highly parallel. Because of this, limit checking is eventually consistent, which can result in overrun during rapid scale outs.
{{% /alert %}}

CPU limits are described with a `DecimalSI` value. Note that the Kubernetes API will coerce this into a string, so we recommend against using integers to avoid GitOps skew.

Memory limits are described with a [`BinarySI` value, such as 1000Gi.](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#meaning-of-memory)

You can view the current consumption of cpu and memory on your cluster by running:
```
kubectl get nodepool -o=jsonpath='{.items[0].status}'
```

Review the [Kubernetes core API](https://github.com/kubernetes/api/blob/37748cca582229600a3599b40e9a82a951d8bbbf/core/v1/resource.go#L23) (`k8s.io/api/core/v1`) for more information on `resources`.

## spec.weight

Karpenter allows you to describe NodePool preferences through a `weight` mechanism similar to how weight is described with [pod and node affinities](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity).

For more information on weighting NodePools, see the [Weighted NodePools section]({{<ref "scheduling#weighted-nodepools" >}}) in the scheduling docs.

## Examples

### Isolating Expensive Hardware

A NodePool can be set up to only provision nodes on particular processor types.
The following example sets a taint that only allows pods with tolerations for Nvidia GPUs to be scheduled:

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: gpu
spec:
  disruption:
    consolidationPolicy: WhenUnderutilized
  template:
    spec:
      requirements:
      - key: node.kubernetes.io/instance-type
        operator: In
        values: ["p3.8xlarge", "p3.16xlarge"]
      taints:
      - key: nvidia.com/gpu
        value: "true"
        effect: NoSchedule
```
In order for a pod to run on a node defined in this NodePool, it must tolerate `nvidia.com/gpu` in its pod spec.

### Cilium Startup Taint

Per the Cilium [docs](https://docs.cilium.io/en/stable/installation/taints/#taint-effects), it's recommended to place a taint of `node.cilium.io/agent-not-ready=true:NoExecute` on nodes to allow Cilium to configure networking prior to other pods starting.  This can be accomplished via the use of Karpenter `startupTaints`.  These taints are placed on the node, but pods aren't required to tolerate these taints to be considered for provisioning.

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: cilium-startup
spec:
  disruption:
    consolidationPolicy: WhenUnderutilized
  template:
    spec:
      startupTaints:
      - key: node.cilium.io/agent-not-ready
        value: "true"
        effect: NoExecute
```
