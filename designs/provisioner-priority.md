# Provisioner Priority

## Goals

- Allowing a mechanism to describe provisioner prioritization to enable provisioner defaults over more specific provisioner requirements
- Allowing provisioner fallbacks when highly-constrained provisioners do not meet the pod criteria

## Use Cases

1. Creating a strongly preferred default provisioner that will always be attempted first, except when more specific configuration is required. This enables scenarios like specifying `kubernetes.io/arch=arm64` for workloads that require this architecture, while preferring `kubernetes.io/arch=amd64` on all other workloads, even if the pod does not contain specific `nodeSelector` constraints. This is particularly useful for those that have high numbers of workloads that only work on `amd64` instances but they do not want to have to go and assign the specific architecture-specific `nodeSelector` to each of these workloads.

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
        operator: In
        values: ["arm64"]
    ```

2. Creating a strongly preferred default provisioner for enterprise users that have `reserved` instance types and want to prefer having these instance types scheduled prior to other instance types. This assumes that we eventually support the `karpenter.sh/capacity-type=reserved` label.

   **Example**

    ```yaml
    # Assuming that I have a reserved instance type associated with my account and we support the karpenter.sh/capacity-type=reserved label

    apiVersion: karpenter.sh/v1alpha5
    kind: Provisioner
    metadata:
      name: reserved
    spec:
      weight: 50
      requirements:
      - key: karpenter.sh/capacity-type
        operator: In
        values: ["reserved"]
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

3. Allowing provisioner with taints to be attempted first so that pods that have tolerations for these taints can be scheduled to specific instance types. Without this ordering, it is possible these pods will be scheduled to nodes that have no tolerations.

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

### Initial Scheduling

__Current State__: Scheduling calls the Kubernetes LIST API for the `karpenter.sh/v1alpha5/Provisioner` and receives a random ordering of Provisioners back for scheduling. When processing pods, Karpenter will find the first provisioner that it is able to schedule the pod onto given the instance type options that are constrained by the pod scheduling requirements (affinity, anti-affinity, topology spread) and the provisioner requirements. This creates some randomness for users and leads to some unpredictability with respect to the provisioner that will be used for a given pod. Currently, the best practices that are recommended by the [Karpenter/EKS docs](https://aws.github.io/aws-eks-best-practices/karpenter/#create-provisioners-that-are-mutually-exclusive) recommend that we create mutually exclusive provisioners such that we reduce this unpredictability.

**Improvement Options**

1. User-Based Priority Ordering with Weights: Scheduling would receive a provisioner ordering based on a `.spec.weight` field, where higher weighted provisioners will be attempted to be scheduled first. In particular, there will be a strict ordering of weights for provisioners, such that higher-weighted provisioners that meet scheduling constraints will have pods scheduled to them first.

2. Define a `karpenter.sh/preferred: true` annotation for Provisioners. Provisioners that have this annotation would be considered first when scheduling pods to provisioners but there would be no ordering among provisioners marked as `preferred` or those that were not marked as `preferred`.

__Recommendation:__ Use a `.spec.weight` to enforce strict ordering of provisioners when scheduling

### Consolidation

The consolidation algorithm requires that the same scheduling algorithm that is used for the initial scheduling should be used during the consolidation scheduling dry-run. This is important such that we do not spin up a new node based on a selected provisioner that is then spun down immediately after a first consolidation run due to the presence of a less expensive instance type on a different provisioner.

**Recommendation:** Because of the requirement to use a consistent scheduling algorithm across initial scheduling and consolidation, we will choose the same ordering-based logic that is used in the initial scheduling in the consolidation algorithm.

### Concerns with User-Based Priority Ordering

1. There are scenarios where this user-based priority ordering could cause undesirable affects to the node provisioning. In particular, consider the scenario where a user has provided the following provisioners

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

2. Provisioners that are ordered first are not always guaranteed to be chosen. A pod could be scheduled to the second provisioner in our ordering and all other pods would be scheduled to that provisioner because it has capacity to support all the pod workloads.

**Recommendation:** Document that placing a high number of constraints on your provisioners can lead to high cost for user nodes in certain scenarios. Additionally, document that provisioner weight is just a preference given in scheduling and may not actually mean that all pods that could be scheduled to a provisioner with higher weight will always end up on a provisioner with high weight.