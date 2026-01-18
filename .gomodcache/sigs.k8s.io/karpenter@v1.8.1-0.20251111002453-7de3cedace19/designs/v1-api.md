# Karpenter v1 API

## Overview

In October 2023, the Karpenter maintainer team [released the beta version](https://aws.amazon.com/blogs/containers/karpenter-graduates-to-beta/) of the NodePool, NodeClaim, and EC2NodeClass APIs, promoting them on a trajectory toward stable (v1). After multiple months in beta, we have sufficient confidence in our APIs that we are ready to promote our APIs to stable support. This implies that we will not be changing these APIs in incompatible ways moving forward without a v2 version, which would still require us to provide long-term support for the launched v1 version.

This move to a stable version of our API represents our last opportunity to update our APIs in a way that we believe: sheds technical debt, improves the experience of all users, and offers additional extensibility for Karpenter’s APIs into future post-v1. This list of API changes below represents the minimal set of changes that are needed to ensure proper operational excellence, feature completeness, and stability by v1. For a change to make it on this list, it must meet one of the following criteria:

1. Breaking: The change requires changes or removals from the API that would be considered breaking after a bump to v1
2. Stability: The change ensures proper operational excellence for behavior that is leaky or has race conditions in the beta state
3. Planned Deprecations: The change cleans-up deprecations that were previously planned the project

## Migration Path

Karpenter will **not** be changing its API group or resource kind as part of the v1 API bump. By avoiding this, we can leverage the [existing Kubernetes conversion webhook process](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#webhook-conversion) for upgrading APIs, which allows upgrades to newer versions of the API to occur in-place, without any customer intervention or node rolling. The upgrade process will be executed as follows:

1. Apply the updated NodePool, NodeClaim, and EC2NodeClass CRDs, which will contain a `v1` versions listed under the `versions` section of the CustomResourceDefinition
2. Upgrade Karpenter controller to its `v1` version. This version of Karpenter will start reasoning in terms of the `v1` API schema in its API requests. Resources will be converted from the v1beta1 to the v1 version at runtime, using conversion webhooks shipped by the upstream Karpenter project and the Cloud Providers (for NodeClass changes).
3. Users update their `v1beta1` manifests that they are applying through IaC or GitOps to use the new `v1` version.
   1. Users that are using multiple NodePools with different kubeletConfigurations that reference the same NodeClasses will need to perform migration from a single NodeClass to multiple NodeClasses. Karpenter v1 [shifts the kubeletConfiguration into the NodeClass API](#moving-spectemplatespeckubelet-into-the-nodeclass) which does not make it possible to represent this many-to-one relationship anymore.
4. Users remove any conversion annotations from their v1 resources.
   1. When Karpenter converts from v1beta1 to v1, it maintains round-trippability between the old resource version and the new resource version. To maintain this with schema changes like the `kubeletConfiguration` move, it leverages annotations on the NodePool (e.g. `compatability.karpenter.sh/v1beta1-kubelet-conversion`) to maintain the v1beta1 data. It will be a strict requirement to remove these annotations from the NodePool before upgrading to v1.1.x+.
5. Karpenter drops the `v1beta1` version from the `CustomResourceDefinition` and the conversion webhooks on the next minor version release of Karpenter, leaving no webhooks and only the `v1` version still present

## NodePool API

```
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: default
spec:
  template:
    metadata:
      labels:
        billing-team: my-team
      annotations:
        example.com/owner: "my-team"
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws # Updated since only a single version will be served
        kind: EC2NodeClass
        name: default
      taints:
        - key: example.com/special-taint
          effect: NoSchedule
      startupTaints:
        - key: example.com/another-taint
          effect: NoSchedule
      requirements:
        - key: "karpenter.k8s.aws/instance-category"
          operator: In
          values: ["c", "m", "r"]
          minValues: 2 # Alpha field, added for spot best practices support
      expireAfter: 720h | Never
      terminationGracePeriod: 1d
  disruption:
    budgets:
      - nodes: 0
        schedule: "0 10 * * mon-fri"
        duration: 16h
        reasons:
          - Drifted
          - Expired
      - nodes: 100%
        reasons:
          - Empty
      - nodes: "10%"
      - nodes: 5
    consolidationPolicy: WhenUnderutilized | WhenEmpty
    consolidateAfter: 1m | Never # Added to allow additional control over consolidation aggressiveness
  weight: 10
  limits:
    cpu: "1000"
    memory: 1000Gi
status:
  conditions:
    - lastTransitionTime: "2024-02-02T19:54:34Z"
      status: "True"
      type: NodeClassReady
    - lastTransitionTime: "2024-02-02T19:54:34Z"
      status: "True"
      type: Ready
  resources:
    nodes: "5"
    cpu: "20"
    memory: "8192Mi"
    ephemeral-storage: "100Gi"
```

### Defaults

Defaults are unchanged in the v1 API version but are included in the API proposal for completeness.

- `disruption.budgets` : `{"nodes": "10%"}`
- `disruption.consolidationPolicy`: `WhenUnderutilized`
- `spec.template.spec.expireAfter`: `30d/720h`

### Printer Columns

**Category:** Stability, Breaking

#### Current

```
➜  karpenter git:(main) ✗ kubectl get nodepools -o wide
NAME      NODECLASS   WEIGHT
default   default     100
fallback  fallback
```

#### Proposed

```
➜  karpenter git:(main) ✗ kubectl get nodepools -o wide
NAME      NODECLASS  NODES  READY  AGE   WEIGHT  CPU  MEMORY
default   default    4      True   2d7h  100     10  160Gi
fallback  default    100    True   2d7h          5   30Gi
```

**Standard Columns**

1. Name
2. NodeClass - Allows users to easily see the NodeClass that the NodePool is using
3. Nodes - Allow users to easily view the number of nodes that are associated with a NodePool
4. Ready - NodePools now have status conditions and will only be used for scheduling when ready. This readiness should be easily viewable by users
5. Age - This is a common field for all Kubernetes objects and should be maintained in the standard set of columns

**Wide Columns (-o wide)**

1. Weight - Viewing the NodePools that will be evaluated first should be easily observable but may not be immediately useful to all users, particularly if the NodePools are named in a way that already indicate their ordering e.g. suffixed with fallback
2. CPU - The sum of the capacity of the CPU for all nodes provisioned by this NodePool
3. Memory - The sum of the capacity of the memory for all nodes provisioned by this NodePool

### Changes to the API

#### Moving `spec.template.spec.kubelet` into the NodeClass

**Category:** Breaking

When the KubeletConfiguration was first introduced into the NodePool, the assumption was that the kubelet configuration is a common interface and that every Cloud Provider supports the same set of kubelet configuration fields.

This turned out not to be the case in reality. For instance, Cloud Providers like Azure [do not support configuring the kubelet configuration through the NodePool API](https://learn.microsoft.com/en-us/azure/aks/node-autoprovision?tabs=azure-cli#:~:text=Kubelet%20configuration%20through%20Node%20pool%20configuration%20is%20not%20supported). Kwok also has no need for the Kubelet API. Shifting these fields into the NodeClass API allows CloudProvider to pick on a case-by-case basis what kind of configuration they want to support through the Kubernetes API.

For more details on the need for shifting this field from the NodePool to the NodeClass, see [the conversation in the #karpenter-dev chat](https://kubernetes.slack.com/archives/C04JW2J5J5P/p1709226455964629).

#### Add `spec.template.spec.terminationGracePeriod`

**Category:** Stability

[Users have asked for the ability to configure a max drain timeout](https://github.com/kubernetes-sigs/karpenter/issues/743) for their nodes. This avoids nodes staying stuck in a draining state on the cluster due to fully blocking PDBs or `karpenter.sh/do-not-disrupt` pods. Starting in v1, we will support a feature that will allow you to configure this max time (called `terminationGracePeriod`). This field will be part of the NodeClaim object and will be static for the lifetime of the NodeClaim. You can template this field onto NodeClaims that are created by NodePools through the `spec.template.spec.terminationGracePeriod` field. 

#### Add `spec.disruption.budgets[*].reasons`

**Category:** Stability

[Users have wanted the ability to constrain their disruption budgets by the specific reason they are being disrupted](https://github.com/kubernetes-sigs/karpenter/issues/924). This allows a user, for example, to constrain a budget for `Underutilization` to ensure that nodes that have pods on them are not rolled throughout non-working hours while allowing nodes that are `Empty` to be rolled. Additionally, users who do not want `Drift` to be constrained on their Disruption budgets at any time due to security posture around image upgrades can now define this.

#### Moving `spec.disruption.expireAfter` to `spec.template.spec.expireAfter`

**Category:** Breaking

In general, we are viewing NodeClaim resources as standalone entities that manage the lifecycle of Nodes on their own. Similar to deployments, NodePools orchestrate the creation and removal of these NodeClaims, but it's still possible to manually create NodeClaims similar to how you can manually create pods.

Like `terminationGracePeriod`, we can view the `expireAfter` value for a NodeClaim as a standalone value for the NodeClaim, absent any ownership from any NodePool. As a result, as part of v1, `expireAfter` will appear as part of the NodeClaim spec and can be set as a template field for NodeClaims that are generated by a NodePool through the `spec.template.spec.expireAfter` field. 

This change is covered in greater detail in the [Forceful Expiration RFC](https://github.com/kubernetes-sigs/karpenter/blob/main/designs/forceful-expiration.md).

#### Status Conditions

**Category:** Stability

Defining the complete set of status condition types that we will include on v1 launch is **out of scope** of this document and will be defined with more granularly in Karpenter’s Observability RFC. Minimally for v1, we will add a `NodeClassReady` and `Ready` condition so that we can determine whether a NodePool is ready to provision a new instance based on the NodeClass readiness provided by the Cloud Provider. The `NodeClassReady` condition will be a strict dependency for NodePool readiness to succeed. More detail around why Karpenter needs status conditions for observability and operational excellence for node launches can be found in [#493](https://github.com/kubernetes-sigs/karpenter/issues/493) and [#909.](https://github.com/kubernetes-sigs/karpenter/issues/909)

Based on this, we are requiring that any Cloud Provider must implement the readiness mechanism in the `status.conditions` of their NodeClass.

#### Status Condition Schema

**Category:** Breaking

Karpenter currently uses the [knative schema for status conditions](https://github.com/knative/pkg/blob/main/apis/condition_types.go#L58). This status condition schema contains a `severity` value which is not present in the upstream schema. Severity for status conditions is tough to reason about. Additionally, Karpenter is planning to [remove its references to knative as part of the v1 Roadmap](./v1-roadmap.md).

We should replace our status condition schema to adhere to the [upstream definition for status conditions](https://github.com/kubernetes/apimachinery/blob/master/pkg/apis/meta/v1/types.go#L1532). An example of the upstream status condition schema is shown below. The upstream API currently requires that all fields except the observedGeneration are required to be specified; we will deviate from this initially by only requiring that the `type`, `status`, and `lastTransitionTime` be set.

```
conditions:
  - type: Initialized
    status: "False"
    observedGeneration: 1
    lastTransitionTime: "2024-02-02T19:54:34Z"
    reason: NodeClaimNotLaunched
    message: "NodeClaim hasn't succeeded launch"
```

#### Add `consolidateAfter` for `consolidationPolicy: WhenUnderutilized`

**Category:** Stability

The [Karpenter v1 Roadmap](./v1-roadmap.md) currently proposes the addition of the `consolidateAfter` field to the Karpenter NodePool API in the disruption block to allow users to be able to tune the aggressiveness of consolidation at v1 (e.g. how long a node has to not have churn against it for us to consider it for consolidation). See [#735](https://github.com/kubernetes-sigs/karpenter/issues/735) for more detail.

This feature is proposed to be added to the v1 API because it avoids us releasing a dead portion of the v1 API that is currently only settable when using `consolidationPolicy: WhenEmpty` and ensures that we are able to set a default for this field at v1 (since adding a default to would be a breaking change after v1).

#### Requiring all `nodeClassRef` fields

**Category:** Breaking

`nodeClassRef` was introduced into the API in v1beta1. v1beta1 did not require users to set the `apiVersion` and `kind` of the NodeClass that they were referencing. This was primarily because Karpenter is built as a single binary that supports a single Cloud Provider and each supported Cloud Provider right now (AWS, Azure, and Kwok) only support a singe NodeClass.

If a Cloud Provider introduces a second NodeClass to handle different types of node provisioning, it’s going to become necessary for the Cloud Provider to differentiate between its types. Additionally, being able to dynamically look up this CloudProvider-defined type from the neutral code is critical for the observability and operationalization of Karpenter (see [#909](https://github.com/kubernetes-sigs/karpenter/issues/909)).

#### Updating `spec.nodeClassRef.apiVersion` to `spec.nodeClassRef.group`

**Category:** Breaking

APIVersions in Kubernetes have two parts: the group name and the version. Currently, Karpenter’s nodeClassRef is referencing the entire apiVersion for NodeClasses to determine which NodeClass should be used during a NodeClaim launch. This works fine, but is non-sensical given that the Karpenter controller will never use the **version** portion of the apiVersion to look-up the NodeClass at the apiserver; it will only ever use the group name on a single version.

Given this, we should update our `apiVersion` field in the `nodeClassRef` to be `group`. This updated field name also has parity with newer, existing APIs like the [Gateway API](https://gateway-api.sigs.k8s.io/api-types/gatewayclass/).

#### Removing `resource.requests` from NodePool Schema

**Category:** Stability

`spec.resource.requests` is part of the NodeClaim API and is set by the Karpenter controller when creating new NodeClaim resources from the scheduling loop. This field describes the required resource requests cacluated from the summation of pod resource requests needed to run against a new instance that we launch. This field is also used to establish Karpenter initialization, where we rely on the presence of requested resources in the newly created Node before we will allow the Karpenter disruption controller to disrupt the node.

We are currently templating every field from the NodeClaim API onto the NodePool `spec.template` (in the same way that a Deployment templates every field from Pod). Validation currently blocks users from setting resource requests in the NodePool, since the field does not make any sense in that context.

Since this field is currently dead API in the schema, we should drop this field from the NodePool at v1.

## NodeClaim API

```
apiVersion: karpenter.sh/v1
kind: NodeClaim
metadata:
  name: default
spec:
  nodeClassRef:
    group: karpenter.k8s.aws # Updated since only a single version will be served
    kind: EC2NodeClass
    name: default
  taints:
    - key: example.com/special-taint
      effect: NoSchedule
  startupTaints:
    - key: example.com/another-taint
      effect: NoSchedule
  requirements:
    - key: "karpenter.k8s.aws/instance-category"
      operator: In
      values: ["c", "m", "r"]
      minValues: 2
  resources:
    requests:
      cpu: "20"
      memory: "8192Mi"
  expireAfter: 720h | Never
  terminationGracePeriod: 1d
status:
  allocatable:
    cpu: 1930m
    ephemeral-storage: 17Gi
    memory: 3055Mi
    pods: "29"
    vpc.amazonaws.com/pod-eni: "9"
  capacity:
    cpu: "2"
    ephemeral-storage: 20Gi
    memory: 3729Mi
    pods: "29"
    vpc.amazonaws.com/pod-eni: "9"
  conditions:
    - lastTransitionTime: "2024-02-02T19:54:34Z"
      status: "True"
      type: Launched
    - lastTransitionTime: "2024-02-02T19:54:34Z"
      status: "True"
      type: Registered
    - lastTransitionTime: "2024-02-02T19:54:34Z"
      status: "True"
      type: Initialized
    - lastTransitionTime: "2024-02-02T19:54:34Z"
      status: "True"
      type: Drifted
      reason: RequirementsDrifted
    - lastTransitionTime: "2024-02-02T19:54:34Z"
      status: "True"
      type: Expired
    - lastTransitionTime: "2024-02-02T19:54:34Z"
      status: "True"
      type: Empty
    - lastTransitionTime: "2024-02-02T19:54:34Z"
      status: "True"
      type: Ready
  nodeName: ip-192-168-71-87.us-west-2.compute.internal
  providerID: aws:///us-west-2b/i-053c6b324e29d2275
  imageID: ami-0b1e393fbe12f411c
```

### Printer Columns

**Category:** Stability, Breaking

#### Current

```
➜  karpenter git:(main) ✗ kubectl get nodeclaims -o wide
NAME            TYPE         ZONE         NODE                                            READY   AGE    CAPACITY   NODEPOOL   NODECLASS
default-7lh6k   c6gn.large   us-west-2b   ip-192-168-183-234.us-west-2.compute.internal   True    2d7h   spot       default    default
default-97v9h   c6gn.large   us-west-2b   ip-192-168-71-87.us-west-2.compute.internal     True    2d7h   spot       default    default
default-fhzpm   c7gd.large   us-west-2b   ip-192-168-165-122.us-west-2.compute.internal   True    2d7h   spot       default    default
default-rw4vf   c6gn.large   us-west-2b   ip-192-168-91-38.us-west-2.compute.internal     True    2d7h   spot       default    default
default-v5qfb   c7gd.large   us-west-2a   ip-192-168-58-94.us-west-2.compute.internal     True    2d7h   spot       default    default
```

#### Proposed

```
➜  karpenter git:(main) ✗ kubectl get nodeclaims -A -o wide
NAME            TYPE         CAPACITY       ZONE         NODE                                            READY   AGE    ID                                      NODEPOOL  NODECLASS
default-7lh6k   c6gn.large   spot           us-west-2b   ip-192-168-183-234.us-west-2.compute.internal   True    2d7h   aws:///us-west-2b/i-053c6b324e29d2275   default   default
default-97v9h   c6gn.large   spot           us-west-2b   ip-192-168-71-87.us-west-2.compute.internal     True    2d7h   aws:///us-west-2a/i-053c6b324e29d2275   default   default
default-fhzpm   c7gd.large   spot           us-west-2b   ip-192-168-165-122.us-west-2.compute.internal   True    2d7h   aws:///us-west-2c/i-053c6b324e29d2275   default   default
default-rw4vf   c6gn.large   on-demand      us-west-2b   ip-192-168-91-38.us-west-2.compute.internal     True    2d7h   aws:///us-west-2a/i-053c6b324e29d2275   default   default
default-v5qfb   c7gd.large   spot           us-west-2a   ip-192-168-58-94.us-west-2.compute.internal     True    2d7h   aws:///us-west-2b/i-053c6b324e29d2275   default   default
```

**Standard Columns**

1. Name
2. Instance Type
3. Capacity - Moved from the wide output to the standard output
4. Zone
5. Node
6. Ready
7. Age

**Wide Columns (-o wide)**

1. ID - Proposing adding Provider ID to make copying and finding instances at the Cloud Provider easier
2. NodePool Name
3. NodeClass Name

### Changes to the API

#### NodeClaim Spec Immutability

**Category:** Breaking

Karpenter currently doesn’t enforce immutability on NodeClaims in v1beta1, though we implicitly assume that users should not be acting against these objects after creation, as the NodeClaim lifecycle controller won’t react to any change after the initial instance launch.

Karpenter can make every `spec` field immutable on the NodeClaim after its initial creation. This will be enforced through CEL validation, where you can perform a check like [`self == oldSelf`](https://kubernetes.io/docs/reference/using-api/cel/#language-overview)` to enforce that the fields cannot have changed after the initial apply. Users who are not on K8s 1.25+ that supports CEL will get the same validation enforced by validating webhooks.

## Labels/Annotations/Tags

#### karpenter.sh/do-not-consolidate (Kubernetes Annotation)

**Category:** Planned Deprecations, Breaking

`karpenter.sh/do-not-consolidate` annotation was introduced as a node-level control in alpha. This control was superseded by the `karpenter.sh/do-not-disrupt` annotation that disabled *all* disruption operations rather than just consolidation. The `karpenter.sh/do-not-consolidate` annotation was declared as deprecated throughout beta and is dropped in v1.

#### karpenter.sh/do-not-evict (Kubernetes Annotation)

**Category:** Planned Deprecations, Breaking

`karpenter.sh/do-not-evict` annotation was introduced as a pod-level control in alpha. This control was superseded by the `karpenter.sh/do-not-disrupt` annotation that disable disruption operations against the node where the pod is running on. The `karpenter.sh/do-not-evict` annotation was declared as deprecated throughout beta and is dropped in v1.