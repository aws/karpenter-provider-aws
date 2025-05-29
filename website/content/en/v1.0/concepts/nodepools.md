---
title: "NodePools"
linkTitle: "NodePools"
weight: 10
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


{{% alert title="Note" color="primary" %}}
Objects for setting Kubelet features have been moved from the NodePool spec to the EC2NodeClasses spec, to not require other Karpenter providers to support those features.
{{% /alert %}}

For some example `NodePool` configurations, see the [examples in the Karpenter GitHub repository](https://github.com/aws/karpenter/blob/v1.0.10/examples/v1/).

```yaml
apiVersion: karpenter.sh/v1
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
        group: karpenter.k8s.aws  # Updated since only a single version will be served
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

      # The amount of time a Node can live on the cluster before being removed
      # Avoiding long-running Nodes helps to reduce security vulnerabilities as well as to reduce the chance of issues that can plague Nodes with long uptimes such as file fragmentation or memory leaks from system processes
      # You can choose to disable expiration entirely by setting the string value 'Never' here

      # Note: changing this value in the nodepool will drift the nodeclaims.
      expireAfter: 720h | Never

      # The amount of time that a node can be draining before it's forcibly deleted. A node begins draining when a delete call is made against it, starting
      # its finalization flow. Pods with TerminationGracePeriodSeconds will be deleted preemptively before this terminationGracePeriod ends to give as much time to cleanup as possible.
      # If your pod's terminationGracePeriodSeconds is larger than this terminationGracePeriod, Karpenter may forcibly delete the pod
      # before it has its full terminationGracePeriod to cleanup.

      # Note: changing this value in the nodepool will drift the nodeclaims.
      terminationGracePeriod: 48h

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

  # Disruption section which describes the ways in which Karpenter can disrupt and replace Nodes
  # Configuration in this section constrains how aggressive Karpenter can be with performing operations
  # like rolling Nodes due to them hitting their maximum lifetime (expiry) or scaling down nodes to reduce cluster cost
  disruption:
    # Describes which types of Nodes Karpenter should consider for consolidation
    # If using 'WhenEmptyOrUnderutilized', Karpenter will consider all nodes for consolidation and attempt to remove or replace Nodes when it discovers that the Node is empty or underutilized and could be changed to reduce cost
    # If using `WhenEmpty`, Karpenter will only consider nodes for consolidation that contain no workload pods
    consolidationPolicy: WhenEmptyOrUnderutilized | WhenEmpty

    # The amount of time Karpenter should wait to consolidate a node after a pod has been added or removed from the node.
    # You can choose to disable consolidation entirely by setting the string value 'Never' here
    consolidateAfter: 1m | Never # Added to allow additional control over consolidation aggressiveness

    # Budgets control the speed Karpenter can scale down nodes.
    # Karpenter will respect the minimum of the currently active budgets, and will round up
    # when considering percentages. Duration and Schedule must be set together.
    budgets:
    - nodes: 10%
    # On Weekdays during business hours, don't do any deprovisioning.
    - schedule: "0 9 * * mon-fri"
      duration: 8h
      nodes: "0"

  # Resource limits constrain the total size of the pool.
  # Limits prevent Karpenter from creating new instances once the limit is exceeded.
  limits:
    cpu: "1000"
    memory: 1000Gi

  # Priority given to the NodePool when the scheduler considers which NodePool
  # to select. Higher weights indicate higher priority when comparing NodePools.
  # Specifying no weight is equivalent to specifying a weight of 0.
  weight: 10
status:
  conditions:
    - type: Initialized
      status: "False"
      observedGeneration: 1
      lastTransitionTime: "2024-02-02T19:54:34Z"
      reason: NodeClaimNotLaunched
      message: "NodeClaim hasn't succeeded launch"
  resources:
    cpu: "20"
    memory: "8192Mi"
    ephemeral-storage: "100Gi"
```
## metadata.name
The name of the NodePool.

## spec.template.metadata.labels
Arbitrary key/value pairs to apply to all nodes.

## spec.template.metadata.annotations
Arbitrary key/value pairs to apply to all nodes.

## spec.template.spec.nodeClassRef

This field points to the Cloud Provider NodeClass resource. See [EC2NodeClasses]({{<ref "nodeclasses" >}}) for details.

## spec.template.spec.taints

Taints to add to provisioned nodes. Pods that don't tolerate those taints could be prevented from scheduling.
See [Taints and Tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) for details.

## spec.template.spec.startupTaints

Taints that are added to nodes to indicate that a certain condition must be met, such as starting an agent or setting up networking, before the node is can be initialized.
These taints must be cleared before pods can be deployed to a node.

## spec.template.spec.expireAfter

The amount of time a Node can live on the cluster before being deleted by Karpenter. Nodes will begin draining once it's expiration has been hit.

## spec.template.spec.terminationGracePeriod

The amount of time a Node can be draining before Karpenter forcibly cleans up the node. Pods blocking eviction like PDBs and do-not-disrupt will be respected during draining until the `terminationGracePeriod` is reached, where those pods will be forcibly deleted.

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

Review [AWS instance types](../../reference/instance-types). Most instance types are supported with the exclusion of [non-HVM](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/virtualization_types.html).

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

Karpenter prioritizes Spot offerings if the NodePool allows Spot and on-demand instances (note that in this scenario any Spot instances priced higher than the cheapest on-demand instance will be temporarily removed from consideration).
If the provider API (e.g. EC2 Fleet's API) indicates Spot capacity is unavailable, Karpenter caches that result across all attempts to provision EC2 capacity for that instance type and zone for the next 3 minutes.
If there are no other possible offerings available for Spot, Karpenter will attempt to provision on-demand instances, generally within milliseconds.

Karpenter also allows `karpenter.sh/capacity-type` to be used as a topology key for enforcing topology-spread.

{{% alert title="Note" color="primary" %}}
There is currently a limit of 100 on the total number of requirements on both the NodePool and the NodeClaim. It's important to note that `spec.template.metadata.labels` are also propagated as requirements on the NodeClaim when it's created, meaning that you can't have more than 100 requirements and labels combined set on your NodePool.
{{% /alert %}}

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


## spec.disruption

You can configure Karpenter to disrupt Nodes through your NodePool in multiple ways. You can use `spec.disruption.consolidationPolicy`, `spec.disruption.consolidateAfter`, or `spec.template.spec.expireAfter`.
You can also rate limit Karpenter's disruption through the NodePool's `spec.disruption.budgets`.
Read [Disruption]({{<ref "disruption" >}}) for more.

## spec.limits

The NodePool spec includes a limits section (`spec.limits`), which constrains the maximum amount of resources that the NodePool can consume.

If the `NodePool.spec.limits` section is unspecified, it means that there is no default limitation on resource allocation. In this case, the maximum resource consumption is governed by the quotas set by your cloud provider. If a limit has been exceeded, nodes provisioning is prevented until some nodes have been terminated.

```yaml
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

## status.conditions
[Conditions](https://github.com/kubernetes/apimachinery/blob/f14778da5523847e4c07346e3161a4b4f6c9186e/pkg/apis/meta/v1/types.go#L1523) objects add observability features to Karpenter.
* The `status.conditions.type` object reflects node status, such as `Initialized` or `Available`.
* The status of the condition, `status.conditions.status`, indicates if the condition is `True` or `False`.
* The `status.conditions.observedGeneration` indicates  if the instance is out of date with the current state of `.metadata.generation`.
* The `status.conditions.lastTransitionTime` object contains a programatic identifier that indicates the time of the condition's previous transition.
* The `status.conditions.reason` object indicates the reason for the condition's previous transition.
* The `status.conditions.message` object provides human-readable details about the condition's previous transition.

NodePools have the following status conditions:

| Condition Type      | Description                                                                                                                                       |
|---------------------|---------------------------------------------------------------------------------------------------------------------------------------------------|
| NodeClassReady      | Underlying nodeClass is ready                                                                                                                     |
| ValidationSucceeded | NodePool CRD validation succeeded                                                                                                                 |
| Ready               | Top level condition that indicates if the nodePool is ready. This condition will not be true until all the other conditions on nodePool are true. |

If a NodePool is not ready, it will not be considered for scheduling.

## status.resources
Objects under `status.resources` provide information about the status of resources such as `cpu`, `memory`, and `ephemeral-storage`.

## Examples

### Isolating Expensive Hardware

A NodePool can be set up to only provision nodes on particular processor types.
The following example sets a taint that only allows pods with tolerations for Nvidia GPUs to be scheduled:

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: gpu
spec:
  disruption:
    consolidationPolicy: WhenEmptyOrUnderutilized
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

Failure to provide accurate `startupTaints` can result in Karpenter continually provisioning new nodes. When the new node joins and the startup taint that Karpenter is unaware of is added, Karpenter now considers the pending pod to be unschedulable to this node. Karpenter will attempt to provision yet another new node to schedule the pending pod.

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: cilium-startup
spec:
  disruption:
    consolidationPolicy: WhenEmptyOrUnderutilized
  template:
    spec:
      startupTaints:
      - key: node.cilium.io/agent-not-ready
        value: "true"
        effect: NoExecute
```
