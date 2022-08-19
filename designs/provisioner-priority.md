# Prioritizing Provisioners

## Goals

- Allowing a user to describe a logical ordering to their provisioners
- Allowing a user to set defaults and/or fallback orderings for their provisioners

## Background

Currently, there is no logical ordering to how the Karpenter scheduling receives and processes Provisioners. Provisioners are processed in the order they are received from the Kubernetes LIST API, which is randomly. Like pod affinities, users want to convey a preferential relationship to some Provisioners over others. Rather than having to manually map a pod to a specific Provisioner through a `nodeSelector` or `affinity` (which could be cumbersome for clusters with large workloads), a user should be able to specify global preferences to pod assignment to provisioners.

## Use Cases

1. Users who have specific architecture requirements for workloads but can't or don't want to go through the process of retro-fitting `nodeSelectors` across these workloads. In this case, we can create a strongly preferred default provisioner that will always be attempted first, except when a specific `nodeSelector` or `nodeAffinity` is specified. 

   **Example**

    ```yaml
    # Default provisioner that will be attempted to be scheduled first

    apiVersion: karpenter.sh/v1alpha5
    kind: Provisioner
    metadata:
      name: default
    spec:
      weight: 100
      requirements:
      - key: kubernetes.io/arch
        operator: In
        values: ["amd64"]
    ```

    ```yaml
    # ARM-64 specific provisioner that will be scheduled if the other provisioner does not fit the constraints for this provisioner

    apiVersion: karpenter.sh/v1alpha5
    kind: Provisioner
    metadata:
      name: arm64
    spec:
      weight: 50
      requirements:
      - key: kubernetes.io/arch
        operator: Exists
    ```

2. Users who have a large set of stateful workloads that require on-demand instances. These users do not want to retrofit their workloads to add `nodeSelectors` that specify a capacity type. In this case, we want to default to on-demand instances and fallback to spot instances for workloads that can support spot.

    **Example**

    ```yaml
    # Default on-demand capacity type 
    apiVersion: karpenter.sh/v1alpha5
    kind: Provisioner
    metadata:
      name: on-demand
    spec:
      weight: 50
      requirements:
      - key: "karpenter.sh/capacity-type"
        operator: In
        values: ["on-demand"]
   ```

   ```yaml
    # Spot capacity type for those that can use spot
    apiVersion: karpenter.sh/v1alpha5
      kind: Provisioner
      metadata:
        name: spot
      spec:
        requirements:
        - key: "karpenter.sh/capacity-type"
          operator: Exists
   ```

3. Creating a strongly preferred default provisioner for enterprise users that have `reserved` instance types and want to prefer having these instance types scheduled prior to other instance types. To ensure we don't over-provision to an instance type over what we have reserved, we can place a `.spec.limits.cpu` on the provisioner to stop after a given limit.

   **Example**

    ```yaml
    # I have c5.large instance types as reserved instances, so I want to schedule to these nodes first

    apiVersion: karpenter.sh/v1alpha5
    kind: Provisioner
    metadata:
      name: reserved
    spec:
      weight: 50
      requirements:
      - key: "node.kubernetes.io/instance-type"
        operator: In
        values: ["c5.large"]
      limits:
        cpu: 20
    ```

    ```yaml
    # For all other pods, we can use these other instance types

    apiVersion: karpenter.sh/v1alpha5
    kind: Provisioner
    metadata:
      name: fallback
    spec:
      requirements:
      - key: "node.kubernetes.io/instance-type"
        operator: In
        values: ["m5.large", "m5.2xlarge"]
    ```

4. Preferring specific instance types for workloads where you are aware that a specific instance type is optimal for your needs but can define other instance types as backups

    **Example**
    
    ```yaml
    # Prefer p3 instance types for GPU workloads

    apiVersion: karpenter.sh/v1alpha5
    kind: Provisioner
    metadata:
      name: reserved
    spec:
      weight: 50
      requirements:
      - key: "karpenter.k8s.aws/instance-family"
        operator: In
        values: ["p3"]
      - key: "gpu-intensive"
        operator: Exists
    ```

    ```yaml
    # GPU-intensive workloads can run optionally on these as a backup

    apiVersion: karpenter.sh/v1alpha5
    kind: Provisioner
    metadata:
      name: fallback
    spec:
      requirements:
      - key: "node.kubernetes.io/instance-type"
        operator: In
        values: ["g5", "g3"]
      - key: "gpu-intensive"
        operator: Exists
    ```

4. Allowing provisioner with taints to be attempted first so that pods that have tolerations for these taints can be scheduled to specific instance types. Without this ordering, it is possible these pods will be scheduled to nodes that have no taints.

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
    # Workloads that do not have any specific requirements can pick up this provisioner as the backup

    apiVersion: karpenter.sh/v1alpha5
    kind: Provisioner
    metadata:
      name: fallback
    spec:
      weight: 50
      requirements:
      - key: kubernetes.io/arch
        operator: In
        values: ["amd64"]
    ```

## Proposed Design

To enable the ability to define a user-defined relationship between provisioners that will be considered in scheduling, we introduce a `.spec.weight` value in the `karpenter.sh/v1alpha5/Karpenter` provisioner spec. This value will have the following constraints:

1. The provisioner weight value will be an integer from 1-100 if specified
2. A provisioner with no weight is considered to be a provisioner with weight 0
3. Provisioners with the same weight have no guarantee on ordering and will be randomly ordered

These constraints are consistent with pod affinity/pod anti-affinity preference weights described [here](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#an-example-of-a-pod-that-uses-pod-affinity).

## Considerations

### Provisioner Scheduling Ordering

__Current State__: Scheduling calls the Kubernetes LIST API for the `karpenter.sh/v1alpha5/Provisioner` and receives a random ordering of Provisioners back for scheduling. When processing pods, Karpenter will find the first provisioner that it is able to schedule the pod onto given the instance type options that are constrained by the pod scheduling requirements (affinity, anti-affinity, topology spread) and the provisioner requirements. This creates some randomness for users and leads to some unpredictability with respect to the provisioner that will be used for a given pod. Currently, the best practices that are recommended by the [Karpenter/EKS docs](https://aws.github.io/aws-eks-best-practices/karpenter/#create-provisioners-that-are-mutually-exclusive) recommend that we create mutually exclusive provisioners such that we reduce this unpredictability.

**Improvement Options**

1. User-Based Priority Ordering with Weights: Scheduling would receive a provisioner ordering based on a `.spec.weight` field, where higher weighted provisioners will be attempted to be scheduled first. In particular, there will be a strict ordering of weights for provisioners, such that higher-weighted provisioners that meet scheduling constraints will have pods scheduled to them first.

2. Define a `karpenter.sh/preferred: true` annotation for Provisioners. Provisioners that have this annotation would be considered first when scheduling pods to provisioners but there would be no ordering among provisioners marked as `preferred` or those that were not marked as `preferred`.

3. Define a `.spec.preferences` section of the `Provisioner`. This preferences section would have the same schema as requirements but with added `.spec.preferences.[].weight` parameter. Preferences would be treated as requirements and backed off if preferences are too strict to be attainable. Users would need to use mutually exclusive provisioners to have predictable results with this interface.

    ```yaml
    apiVersion: karpenter.sh/v1alpha5
    kind: Provisioner
    metadata:
      name: preference
    spec:
      requirements:
      - key: kubernetes.io/arch
        operator: In
        values: ["amd64", "arm64"]
      preferences:
      - key: kubernetes.io/arch
        operator: In
        values: ["amd64"]
        weight: 100
      
    ```
  
    This implementation has some technical concerns related to how these preferences are understood in the following scenarios:

    1. A node has been provisioned using preferences as part of the node requirements. Should this node be consolidated to a smaller instance during the consolidation loop if there is a smaller instance that meets the requirements but does not meet the preferences? In general, if the scheduling loop is performing the same during initial provisioning and consolidation, the preferences would be observed 

    2. A new node is being prepared to be created with a pod assigned to it. The node has preferences that it is treating as requirements. During the scheduling loop, another pod has to be scheduled but does not meet the new node preferences. Should we attempt to relax this node's preferences such that the pod could be scheduled to the node or create an entirely separate node that this pod can fit on? Basically, this comes down to the question of whether the node can have relaxed requirements after node "creation" has occurred in Karpenter scheduling loop.

__Recommendation:__ Use a `.spec.weight` to enforce strict ordering of provisioners when scheduling

### Pod Scheduling Ordering

When pods are batched, they are not necessarily ordered in any particular way. This means that we may not actually consider the highest priority provisioner when attempting to schedule pods.

As an example, we could receive a pod that does not support a given node taint specified on the highest priority provisioner first. Thus, we would schedule using the second-highest priority or backup provisioner.

Because we scheduled with this provisioner and the node has capacity, all the other pods that were part of this batch may end up on that single node, even though there were some pods in this batch that would have tolerated the taints.

**Options**

1. Use a `karpenter.sh/weight` annotation on pods to tell Karpenter to order these pods first by weight ahead of any resource ordering that is performed for bin-packing.
2. Do not change any of the existing logic and document that this is a consideration to take into account when prioritizing provisioners.

**Recommendation:** Do not change the existing logic. In general, the fact that the scheduler may choose to schedule a second provisioner first due to constraints is a general concern with the kube-scheduler as we can't even guarantee which node each pod will end up on. Instead, document that provisioner weight is just a preference given in scheduling and may not actually mean that all pods that could be scheduled to a provisioner with higher weight will always end up on a provisioner with higher weight.

### Consolidation

The consolidation algorithm requires that the same scheduling algorithm that is used for the initial scheduling should be used during the consolidation scheduling dry-run. This is important such that we do not spin up a new node based on a selected provisioner that is then spun down immediately after a first consolidation run due to the presence of a less expensive instance type on a different provisioner.

**Recommendation:** Because of the requirement to use a consistent scheduling algorithm across initial scheduling and consolidation, we will choose the same ordering-based logic that is used in the initial scheduling in the consolidation algorithm.

### Configuration Considerations

**Creating Inflexible Provisioners**: There are scenarios where this user-based priority ordering could cause undesirable affects to the node provisioning. In particular, consider the scenario where a user has provided the following provisioners

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

**Recommendation:** Document that placing a high number of constraints on your provisioners can lead to high cost for user nodes in certain scenarios.