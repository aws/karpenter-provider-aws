---
title: "Provisioning"
linkTitle: "Provisioning"
weight: 1
description: >
  Learn about Karpenter Provisioners
---

When you first installed Karpenter, you set up a default Provisioner.
The Provisioner sets constraints on the nodes that can be created by Karpenter and the pods that can run on those nodes.
The Provisioner can be set to do things like:

* Define taints to limit the pods that can run on nodes Karpenter creates
* Define any startup taints to inform Karpenter that it should taint the node initially, but that the taint is temporary.
* Limit node creation to certain zones, instance types, and computer architectures
* Set defaults for node expiration

You can change your Provisioner or add other Provisioners to Karpenter.
Here are things you should know about Provisioners:

* Karpenter won't do anything if there is not at least one Provisioner configured.
* Each Provisioner that is configured is looped through by Karpenter.
* If Karpenter encounters a taint in the Provisioner that is not tolerated by a Pod, Karpenter won't use that Provisioner to provision the pod.
* If Karpenter encounters a startup taint in the Provisioner it will be applied to nodes that are provisioned, but pods do not need to tolerate the taint.  Karpenter assumes that the taint is temporary and some other system will remove the taint.
* It is recommended to create Provisioners that are mutually exclusive. So no Pod should match multiple Provisioners. If multiple Provisioners are matched, Karpenter will use the Provisioner with the highest [weight](#specweight).

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: default
spec:
  # References cloud provider-specific custom resource, see your cloud provider specific documentation
  providerRef:
    name: default

  # Provisioned nodes will have these taints
  # Taints may prevent pods from scheduling if they are not tolerated by the pod.
  taints:
    - key: example.com/special-taint
      effect: NoSchedule

  # Provisioned nodes will have these taints, but pods do not need to tolerate these taints to be provisioned by this
  # provisioner. These taints are expected to be temporary and some other entity (e.g. a DaemonSet) is responsible for
  # removing the taint after it has finished initializing the node.
  startupTaints:
    - key: example.com/another-taint
      effect: NoSchedule

  # Labels are arbitrary key-values that are applied to all nodes
  labels:
    billing-team: my-team

  # Requirements that constrain the parameters of provisioned nodes.
  # These requirements are combined with pod.spec.affinity.nodeAffinity rules.
  # Operators { In, NotIn } are supported to enable including or excluding values
  requirements:
    - key: "karpenter.k8s.aws/instance-category"
      operator: In
      values: ["c", "m", "r"]
    - key: "karpenter.k8s.aws/instance-cpu"
      operator: In
      values: ["4", "8", "16", "32"]
    - key: "karpenter.k8s.aws/instance-hypervisor"
      operator: In
      values: ["nitro"]
    - key: "topology.kubernetes.io/zone"
      operator: In
      values: ["us-west-2a", "us-west-2b"]
    - key: "kubernetes.io/arch"
      operator: In
      values: ["arm64", "amd64"]
    - key: "karpenter.sh/capacity-type" # If not included, the webhook for the AWS cloud provider will default to on-demand
      operator: In
      values: ["spot", "on-demand"]

  # Karpenter provides the ability to specify a few additional Kubelet args.
  # These are all optional and provide support for additional customization and use cases.
  kubeletConfiguration:
    clusterDNS: ["10.0.1.100"]
    containerRuntime: containerd
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
    evictionMaxPodGracePeriod: 3m
    podsPerCore: 2
    maxPods: 20

  # Resource limits constrain the total size of the cluster.
  # Limits prevent Karpenter from creating new instances once the limit is exceeded.
  limits:
    resources:
      cpu: "1000"
      memory: 1000Gi

  # Enables consolidation which attempts to reduce cluster cost by both removing un-needed nodes and down-sizing those
  # that can't be removed.  Mutually exclusive with the ttlSecondsAfterEmpty parameter.
  consolidation:
    enabled: true

  # If omitted, the feature is disabled and nodes will never expire.  If set to less time than it requires for a node
  # to become ready, the node may expire before any pods successfully start.
  ttlSecondsUntilExpired: 2592000 # 30 Days = 60 * 60 * 24 * 30 Seconds;

  # If omitted, the feature is disabled, nodes will never scale down due to low utilization
  ttlSecondsAfterEmpty: 30

  # Priority given to the provisioner when the scheduler considers which provisioner
  # to select. Higher weights indicate higher priority when comparing provisioners.
  # Specifying no weight is equivalent to specifying a weight of 0.
  weight: 10
```

## spec.requirements

Kubernetes defines the following [Well-Known Labels](https://kubernetes.io/docs/reference/labels-annotations-taints/), and cloud providers (e.g., AWS) implement them. They are defined at the "spec.requirements" section of the Provisioner API.

These well known labels may be specified at the provisioner level, or in a workload definition (e.g., nodeSelector on a pod.spec). Nodes are chosen using both the provisioner's and pod's requirements. If there is no overlap, nodes will not be launched. In other words, a pod's requirements must be within the provisioner's requirements. If a requirement is not defined for a well known label, any value available to the cloud provider may be chosen.

For example, an instance type may be specified using a nodeSelector in a pod spec. If the instance type requested is not included in the provisioner list and the provisioner has instance type requirements, Karpenter will not create a node or schedule the pod.

📝 None of these values are required.

### Instance Types

- key: `node.kubernetes.io/instance-type`

Generally, instance types should be a list and not a single value. Leaving this field undefined is recommended, as it maximizes choices for efficiently placing pods.

Review [AWS instance types](../instance-types). Most instance types are supported with the exclusion of [non-HVM](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/virtualization_types.html).

**Example**

*Set Default with provisioner.yaml*

```yaml
spec:
  requirements:
    - key: node.kubernetes.io/instance-type
      operator: In
      values: ["m5.large", "m5.2xlarge"]
```

*Override with workload manifest (e.g., pod)*

```yaml
spec:
  template:
    spec:
      nodeSelector:
        node.kubernetes.io/instance-type: m5.large
```

### Availability Zones

- key: `topology.kubernetes.io/zone`
- value example: `us-east-1c`
- value list: `aws ec2 describe-availability-zones --region <region-name>`

Karpenter can be configured to create nodes in a particular zone. Note that the Availability Zone `us-east-1a` for your AWS account might not have the same location as `us-east-1a` for another AWS account.

[Learn more about Availability Zone
IDs.](https://docs.aws.amazon.com/ram/latest/userguide/working-with-az-ids.html)

### Architecture

- key: `kubernetes.io/arch`
- values
  - `amd64` (default)
  - `arm64`

Karpenter supports `amd64` nodes, and `arm64` nodes.


### Capacity Type

- key: `karpenter.sh/capacity-type`
- values
  - `spot`
  - `on-demand` (default)

Karpenter supports specifying capacity type, which is analogous to [EC2 purchase options](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-purchasing-options.html).

Karpenter prioritizes Spot offerings if the provisioner allows Spot and on-demand instances. If the provider API (e.g. EC2 Fleet's API) indicates Spot capacity is unavailable, Karpenter caches that result across all attempts to provision EC2 capacity for that instance type and zone for the next 45 seconds. If there are no other possible offerings available for Spot, Karpenter will attempt to provision on-demand instances, generally within milliseconds.

Karpenter also allows `karpenter.sh/capacity-type` to be used as a topology key for enforcing topology-spread.

## spec.weight

Karpenter allows you to describe provisioner preferences through a `weight` mechanism similar to how weight is described with [pod and node affinities](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity).

For more information on weighting Provisioners, see the [Weighting Provisioners section](../scheduling#weighting-provisioners) in the scheduling details.

## spec.kubeletConfiguration

Karpenter provides the ability to specify a few additional Kubelet args. These are all optional and provide support for
additional customization and use cases. Adjust these only if you know you need to do so.

```yaml
spec:
  ...
  kubeletConfiguration:
    clusterDNS: ["10.0.1.100"]
    containerRuntime: containerd
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
    evictionMaxPodGracePeriod: 3m
    podsPerCore: 2
    maxPods: 20
```

☁️ **AWS**

You can specify the container runtime to be either `dockerd` or `containerd`. By default, `containerd` is used.

* `containerd` is the only valid container runtime when using the Bottlerocket AMI Family or when using the AL2 AMI Family and K8s version 1.24+

### Reserved Resources

Karpenter will automatically configure the system and kube reserved resource requests on the fly on your behalf. These requests are used to configure your node and to make scheduling decisions for your pods. If you have specific requirements or know that you will have additional capacity requirements, you can optionally override the `--system-reserved` configuration defaults with the `.spec.kubeletConfiguration.systemReserved` values and the `--kube-reserved` configuration defaults with the `.spec.kubeletConfiguration.kubeReserved` values.

For more information on the default `--system-reserved` and `--kube-reserved` configuration refer to the [Kubelet Docs](https://kubernetes.io/docs/tasks/administer-cluster/reserve-compute-resources/#kube-reserved)

### Eviction Thresholds

The kubelet supports eviction thresholds by default. When enough memory or file system pressure is exerted on the node, the kubelet will begin to evict pods to ensure that system daemons and other system processes can continue to run in a healthy manner.

Kubelet has the notion of [hard evictions](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#hard-eviction-thresholds) and [soft evictions](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#soft-eviction-thresholds). In hard evictions, pods are evicted as soon as a threshold is met, with no grace period to terminate. Soft evictions, on the other hand, provide an opportunity for pods to be terminated gracefully. They do so by sending a termination signal to pods that are planning to be evicted and allowing those pods to terminate up to their grace period.

Karpenter supports [hard evictions](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#hard-eviction-thresholds) through the `.spec.kubeletConfiguration.evictionHard` field and [soft evictions](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#soft-eviction-thresholds) through the `.spec.kubeletConfiguration.evictionSoft` field. `evictionHard` and `evictionSoft` are configured by listing [signal names](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/#eviction-signals) with either percentage values or resource values.

```yaml
spec:
  ...
  kubeletConfiguration:
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

| Eviction Signal | Description |
| --------------- | ----------- |
| memory.available | memory.available := node.status.capacity[memory] - node.stats.memory.workingSet |
| nodefs.available | nodefs.available := node.stats.fs.available |
| nodefs.inodesFree | nodefs.inodesFree := node.stats.fs.inodesFree |
| imagefs.available | imagefs.available := node.stats.runtime.imagefs.available |
| imagefs.inodesFree | imagefs.inodesFree := node.stats.runtime.imagefs.inodesFree |
| pid.available | pid.available := node.stats.rlimit.maxpid - node.stats.rlimit.curproc |

For more information on eviction thresholds, view the [Node-pressure Eviction](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction) section of the official Kubernetes docs.

#### Soft Eviction Grace Periods

Soft eviction pairs an eviction threshold with a specified grace period. With soft eviction thresholds, the kubelet will only begin evicting pods when the node exceeds its soft eviction threshold over the entire duration of its grace period. For example, if you specify `evictionSoft[memory.available]` of `500Mi` and a `evictionSoftGracePeriod[memory.available]` of `1m30`, the node must have less than `500Mi` of available memory over a minute and a half in order for the kubelet to begin evicting pods.

Optionally, you can specify an `evictionMaxPodGracePeriod` which defines the administrator-specified maximum pod termination grace period to use during soft eviction. If a namespace-owner had specified a pod `terminationGracePeriodInSeconds` on pods in their namespace, the minimum of `evictionPodGracePeriod` and `terminationGracePeriodInSeconds` would be used.

```yaml
spec:
  ...
  kubeletConfiguration:
    evictionSoftGracePeriod:
      memory.available: 1m
      nodefs.available: 1m30s
      nodefs.inodesFree: 2m
      imagefs.available: 1m30s
      imagefs.inodesFree: 2m
      pid.available: 2m
    evictionMaxPodGracePeriod: 3m
```

### Pod Density

#### Max Pods

By default, AWS will configure the maximum density of pods on a node [based on the node instance type](https://github.com/awslabs/amazon-eks-ami/blob/master/files/eni-max-pods.txt). For small instances that require an increased pod density or large instances that require a reduced pod density, you can override this default value with `.spec.kubeletConfiguration.maxPods`. This value will be used during Karpenter pod scheduling and passed through to `--max-pods` on kubelet startup.

{{% alert title="Note" color="primary" %}}
When using small instance types, it may be necessary to enable [prefix assignment mode](https://aws.amazon.com/blogs/containers/amazon-vpc-cni-increases-pods-per-node-limits/) in the AWS VPC CNI plugin to support a higher pod density per node.  Prefix assignment mode was introduced in AWS VPC CNI v1.9 and allows ENIs to manage a broader set of IP addresses.  Much higher pod densities are supported as a result.
{{% /alert %}}

#### Pods Per Core

An alternative way to dynamically set the maximum density of pods on a node is to use the `.spec.kubeletConfiguration.podsPerCore` value. Karpenter will calculate the pod density during scheduling by multiplying this value by the number of logical cores (vCPUs) on an instance type. This value will also be passed through to the `--pods-per-core` value on kubelet startup to configure the number of allocatable pods the kubelet can assign to the node instance.

The value generated from `podsPerCore` cannot exceed `maxPods`, meaning, if both are set, the minimum of the `podsPerCore` dynamic pod density and the static `maxPods` value will be used for scheduling.

{{% alert title="Note" color="primary" %}}
`maxPods` may not be set in the `kubeletConfiguration` of a Provisioner, but may still be restricted by the `ENI_LIMITED_POD_DENSITY` value. You may want to ensure that the `podsPerCore` value that will be used for instance families associated with the Provisioner will not cause unexpected behavior by exceeding the `maxPods` value.
{{% /alert %}}

{{% alert title="Pods Per Core on Bottlerocket" color="warning" %}}
Bottlerocket AMIFamily currently does not support `podsPerCore` configuration. If a Provisioner contains a `provider` or `providerRef` to a node template that will launch a Bottlerocket instance, the `podsPerCore` value will be ignored for scheduling and for configuring the kubelet.
{{% /alert %}}

## spec.limits.resources

The provisioner spec includes a limits section (`spec.limits.resources`), which constrains the maximum amount of resources that the provisioner will manage.

Karpenter supports limits of any resource type that is reported by your cloud provider.

CPU limits are described with a `DecimalSI` value. Note that the Kubernetes API will coerce this into a string, so we recommend against using integers to avoid GitOps skew.

Memory limits are described with a [`BinarySI` value, such as 1000Gi.](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#meaning-of-memory)

Karpenter limits instance types when scheduling to those that will not exceed the specified limits.  If a limit has been exceeded, nodes provisioning is prevented until some nodes have been terminated.

Review [resource limits](../set-resource-limits) for more information.

## spec.providerRef

This field points to the cloud provider-specific custom resource. Learn more about [AWSNodeTemplates](../node-templates/).

## spec.consolidation

You can configure Karpenter to deprovision instances through your Provisioner in multiple ways. You can use `spec.TTLSecondsAfterEmpty`, `spec.ttlSecondsUntilExpired` or `spec.consolidation.enabled`. Read [Deprovisioning](../deprovisioning/) for more.


## Example: Restricting Instance Types

Not all workloads are able to run on any instance type. Some use cases may be sensitive to a specific hardware generation or cannot tolerate burstable compute. You can specify a variety of well known labels to control the set of instance types available to be provisioned.

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: default
spec:
  requirements:
    # Include general purpose instance families
    - key: karpenter.k8s.aws/instance-family
      operator: In
      values: [c5, m5, r5]
    # Exclude smaller instance sizes
    - key: karpenter.k8s.aws/instance-size
      operator: NotIn
      values: [nano, micro, small, large]
    # Exclude a specific instance type
    - key: node.kubernetes.io/instance-type
      operator: NotIn
      values: [m5.24xlarge]
```

## Example: Isolating Expensive Hardware

A provisioner can be set up to only provision nodes on particular processor types.
The following example sets a taint that only allows pods with tolerations for Nvidia GPUs to be scheduled:

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: gpu
spec:
  ttlSecondsAfterEmpty: 60
  requirements:
  - key: node.kubernetes.io/instance-type
    operator: In
    values: ["p3.8xlarge", "p3.16xlarge"]
  taints:
  - key: nvidia.com/gpu
    value: "true"
    effect: NoSchedule
```
In order for a pod to run on a node defined in this provisioner, it must tolerate `nvidia.com/gpu` in its pod spec.

### Example: Adding the Cilium Startup Taint

Per the Cilium [docs](https://docs.cilium.io/en/stable/gettingstarted/taints/),  it's recommended to place a taint of `node.cilium.io/agent-not-ready=true:NoExecute` on nodes to allow Cilium to configure networking prior to other pods starting.  This can be accomplished via the use of Karpenter `startupTaints`.  These taints are placed on the node, but pods aren't required to tolerate these taints to be considered for provisioning.

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: cilium-startup
spec:
  ttlSecondsAfterEmpty: 60
  startupTaints:
  - key: node.cilium.io/agent-not-ready
    value: "true"
    effect: NoExecute
```
