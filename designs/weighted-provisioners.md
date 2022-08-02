# Weighted Provisioners

## Goals

- Describe a way to order provisioners rather than attempting a random ordering of provisioners
- Allow the ability for customers to define a logical ordering for provisioners based on user-defined preferences

## Use Cases

1. Defining a "default" provisioner that will be attempted before attempting to schedule nodes with default labels (see [#783](https://github.com/aws/karpenter/issues/783))

   a. This restricts nodes with specific instance types from being deployed unless there is a specific `nodeSelector` on a pod that requires this provisioner to be used. Otherwise, the default provisioner would be used based on the ordering. Without the ordering, it is possible that the provisioner with labels is used even for pods without this `nodeSelector`.

   b. Pods that do not have any specific `nodeSelectors` or `affinity` to go to particular architecture types but may still require a specific architecture type (see [#2024](https://github.com/aws/karpenter/issues/2024)) by default can be provisioned to a default node architecture type. Without the ordering, it is possible that these pods could get provisioned to other architectures.

2. Allowing provisioner with taints to be attempted first so that pods that have tolerations for these taints can be scheduled to specific instance types (see [#783](https://github.com/aws/karpenter/issues/783)). Without this ordering, it is possible these pods will be scheduled to nodes that have no tolerations.

_Note: It is still possible that pods with tolerations may not be scheduled to nodes with taints even if they are preferred, since pods with tolerations can technically be scheduled anywhere they are tolerated (including nodes that contain no taints)._

**Example**

```yaml
# Custom Workload for Pods that Tolerate these Taints should be attempted first

apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: 
spec:
  weight: 100
  requirements:
  - key: node.kubernetes.io/instance-type
    operator: In
    values: ["p3.8xlarge", "p3.16xlarge"]
  taints:
  - key: custom-workloads
    value: "true"
    effect: NoSchedule
```

```yaml
# Workloads that do not have any specific requirements can pick up this provisioner next. This provisioner will always pick AMD64 by default, even for nodes that don't specify a specific architecture

apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: default
spec:
  weight: 50
  requirements:
  - key: kubernetes.io/arch
    operator: In
    values: ["amd64"]
```

```yaml
# Other workloads that have node selectors can pick-up this provisioner as a special provisioner, but don't want other workloads with no nodeSelector to pick-up this one by default

apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: nginx
spec:
  labels:
    app-name: nginx
  requirements:
  - key: node.kubernetes.io/instance-type
    operator: In
    values: ["c4.large", "c4.xlarge"]
  - key: kubernetes.io/arch
    operator: In
    values: ["amd64"]
```

```yaml
# Workloads that support ARM64 should be scheduled on specific instance types

apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: arm64
spec:
  requirements:
  - key: node.kubernetes.io/instance-type
    operator: In
    values: ["m5.large", "m5.2xlarge"]
  - key: kubernetes.io/arch
    operator: In
    values: ["arm64"]
```

## Proposed Design

To enable the ability to define a user-defined relationship between provisioners that will be considered in scheduling, we introduce a `.spec.weight` value in the `karpenter.sh/v1alpha5/Karpenter` provisioner spec. This value will have the following constraints:

1. The provisioner weight value will be an integer from 1-100 if specified
2. A provisioner with no weight is considered to be a provisioner with weight 0
3. Provisioners with the same weight have no guarantee on ordering and will be randomly ordered

These constraints are consistent with pod affinity/pod anti-affinity preference weights described [here](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#an-example-of-a-pod-that-uses-pod-affinity).

## Considerations

### Initial Scheduling

__Current State__: Scheduling calls the Kubernetes LIST API for the `karpenter.sh/v1alpha5/Provisioner` and receives a random ordering of Provisioners back for scheduling. When processing pods, Karpenter will find the first provisioner that it is able to schedule the pod onto given the instance type options that are constrained by the pod scheduling requirements (affinity, anti-affinity, topology spread) and the provisioner requirements. This creates some randomness for users and leads to some unpredictability with respect to the provisioner that will be used for a given pod. Currently, the best practices that are recommended by the [Karpenter/EKS docs](https://aws.github.io/aws-eks-best-practices/karpenter/#create-provisioners-that-are-mutually-exclusive) recommend that we create mutually exclusive provisioners such that we reduce this unpredictability.

__User-Based Strict Priority Ordering__: Scheduling would receive a provisioner ordering based on the user-specified priorities. In particular, there will be a strict ordering of weights for provisioners, such that higher-weighted provisioners that meet scheduling constraints will have pods scheduled to them first.

__Alternatives__: An alternative behavior could be to treat the weights in these provisioners as strong preferences toward choosing this provisioner over another provisioner (similar to `preferredDuringSchedulingIgnoredDuringExecution` affinities). This user-defined preference value could be combined with some cost function that attempts to estimate the cost of scheduling using a given provisioner vs. the cost of scheduling using a different provisioner.

In practice, this cost function is difficult to approximate since:

1. It is difficult to say whether one ordering of provisioners would perform better during scheduling compared to some other ordering of provisioners without permuting the entire possible provisioner set (which is an NP-hard problem).
2. We have to make calls to the EC2 fleet API to determine the instance type we will receive, which, in the case of a spot instance, is not always guaranteed to be the cheapest spot instance available.

Additionally, the notion of treating the weights as preference values from customers rather than a strict ordering could lead to confusion as it will be difficult for someone who has deployed weighted provisioners to reason about how those weighted provisioners will be scored combined with other criteria like cost and availability.

__Concerns with User-Based Priority Ordering__: There are scenarios where this user-based priority ordering could cause undesirable affects to the node provisioning. In particular, consider the scenario where a user has provided the following provisioners

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: expensive
spec:
  requirements:
    - key: "node.kubernetes.io/instance-type"
      operator: In
      values: ["p3.16xlarge"]
  weight: 100
```

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: inexpensive
spec:
  requirements:
    - key: "node.kubernetes.io/instance-type"
      operator: In
      values: ["t3.small"]
```

In this scenario, since the first provisioner marked `expensive` has the highest weight, it will always be considered first. On a deployment with 20 replicas and strict pod anti-affinity, this will cause the `expensive` provisioner to always be considered for scheduling for this deployment and will deploy 20 new nodes all with the most expensive instance type when a cheaper/more available instance type could have fit the same number of pods across the same number of nodes.

This is an extreme example; however, it is worth noting that users who constrain their instance types with a hierarchical structure that prioritizes larger instances should take care when placing weights on these instance types that would order expensive instance types before less expensive ones.

_Note: In general, the recommendation should be to avoid placing a high number of constraints on your provisioners that would reduce the number of instance types down to a low cardinality and cause high cost, low availability to the instances that would be provisioned._

### Consolidation

The consolidation algorithm requires that the same scheduling algorithm that is used for the initial scheduling should be used during the consolidation scheduling dry-run. This is important such that we do not spin up a new node based on a selected provisioner that is then spun down immediately after a first consolidation run due to the presence of a less expensive instance type.

_In particular, this would be the case if we took a global/preferential view on scheduling and considered more than one provisioner in the case of consolidation but not in the case of the initial provisioning._

Consolidation scheduling should also take into account the first provisioner that pods can be scheduled to with weight as the ordering mechanism. As with initial scheduling, only the instance types from the first provisioner (ordered by weight) that will support all the pods for consolidation will be considered.
