# API Reference

## Packages
- [karpenter.sh/v1beta1](#karpentershv1beta1)


## karpenter.sh/v1beta1


### Resource Types
- [NodeClaim](#nodeclaim)
- [NodeClaimList](#nodeclaimlist)
- [NodePool](#nodepool)
- [NodePoolList](#nodepoollist)



#### ConsolidationPolicy

_Underlying type:_ _string_



_Appears in:_
- [Disruption](#disruption)



#### Disruption





_Appears in:_
- [NodePoolSpec](#nodepoolspec)

| Field | Description |
| --- | --- |
| `consolidateAfter` _[NillableDuration](#nillableduration)_ | ConsolidateAfter is the duration the controller will wait before attempting to terminate nodes that are underutilized. Refer to ConsolidationPolicy for how underutilization is considered. |
| `consolidationPolicy` _[ConsolidationPolicy](#consolidationpolicy)_ | ConsolidationPolicy describes which nodes Karpenter can disrupt through its consolidation algorithm. This policy defaults to "WhenUnderutilized" if not specified |
| `expireAfter` _[NillableDuration](#nillableduration)_ | ExpireAfter is the duration the controller will wait before terminating a node, measured from when the node is created. This is useful to implement features like eventually consistent node upgrade, memory leak protection, and disruption testing. |


#### KubeletConfiguration



KubeletConfiguration defines args to be used when configuring kubelet on provisioned nodes. They are a subset of the upstream types, recognizing not all options may be supported. Wherever possible, the types and names should reflect the upstream kubelet types. https://pkg.go.dev/k8s.io/kubelet/config/v1beta1#KubeletConfiguration https://github.com/kubernetes/kubernetes/blob/9f82d81e55cafdedab619ea25cabf5d42736dacf/cmd/kubelet/app/options/options.go#L53

_Appears in:_
- [NodeClaimSpec](#nodeclaimspec)

| Field | Description |
| --- | --- |
| `clusterDNS` _string array_ | clusterDNS is a list of IP addresses for the cluster DNS server. Note that not all providers may use all addresses. |
| `maxPods` _integer_ | MaxPods is an override for the maximum number of pods that can run on a worker node instance. |
| `podsPerCore` _integer_ | PodsPerCore is an override for the number of pods that can run on a worker node instance based on the number of cpu cores. This value cannot exceed MaxPods, so, if MaxPods is a lower value, that value will be used. |
| `systemReserved` _[ResourceList](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcelist-v1-core)_ | SystemReserved contains resources reserved for OS system daemons and kernel memory. |
| `kubeReserved` _[ResourceList](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcelist-v1-core)_ | KubeReserved contains resources reserved for Kubernetes system components. |
| `evictionHard` _object (keys:string, values:string)_ | EvictionHard is the map of signal names to quantities that define hard eviction thresholds |
| `evictionSoft` _object (keys:string, values:string)_ | EvictionSoft is the map of signal names to quantities that define soft eviction thresholds |
| `evictionSoftGracePeriod` _object (keys:string, values:[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#duration-v1-meta))_ | EvictionSoftGracePeriod is the map of signal names to quantities that define grace periods for each eviction signal |
| `evictionMaxPodGracePeriod` _integer_ | EvictionMaxPodGracePeriod is the maximum allowed grace period (in seconds) to use when terminating pods in response to soft eviction thresholds being met. |
| `imageGCHighThresholdPercent` _integer_ | ImageGCHighThresholdPercent is the percent of disk usage after which image garbage collection is always run. The percent is calculated by dividing this field value by 100, so this field must be between 0 and 100, inclusive. When specified, the value must be greater than ImageGCLowThresholdPercent. |
| `imageGCLowThresholdPercent` _integer_ | ImageGCLowThresholdPercent is the percent of disk usage before which image garbage collection is never run. Lowest disk usage to garbage collect to. The percent is calculated by dividing this field value by 100, so the field value must be between 0 and 100, inclusive. When specified, the value must be less than imageGCHighThresholdPercent |
| `cpuCFSQuota` _boolean_ | CPUCFSQuota enables CPU CFS quota enforcement for containers that specify CPU limits. |


#### Limits

_Underlying type:_ _[ResourceList](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcelist-v1-core)_



_Appears in:_
- [NodePoolSpec](#nodepoolspec)



#### NillableDuration



NillableDuration is a wrapper around time.Duration which supports correct marshaling to YAML and JSON. It uses the value "Never" to signify that the duration is disabled and sets the inner duration as nil

_Appears in:_
- [Disruption](#disruption)

| Field | Description |
| --- | --- |
| `Duration` _[Duration](#duration)_ |  |


#### NodeClaim



NodeClaim is the Schema for the NodeClaims API

_Appears in:_
- [NodeClaimList](#nodeclaimlist)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `karpenter.sh/v1beta1`
| `kind` _string_ | `NodeClaim`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[NodeClaimSpec](#nodeclaimspec)_ |  |


#### NodeClaimList



NodeClaimList contains a list of NodeClaims



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `karpenter.sh/v1beta1`
| `kind` _string_ | `NodeClaimList`
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `items` _[NodeClaim](#nodeclaim) array_ |  |


#### NodeClaimSpec



NodeClaimSpec describes the desired state of the NodeClaim

_Appears in:_
- [NodeClaim](#nodeclaim)
- [NodeClaimTemplate](#nodeclaimtemplate)

| Field | Description |
| --- | --- |
| `taints` _[Taint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#taint-v1-core) array_ | Taints will be applied to the NodeClaim's node. |
| `startupTaints` _[Taint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#taint-v1-core) array_ | StartupTaints are taints that are applied to nodes upon startup which are expected to be removed automatically within a short period of time, typically by a DaemonSet that tolerates the taint. These are commonly used by daemonsets to allow initialization and enforce startup ordering.  StartupTaints are ignored for provisioning purposes in that pods are not required to tolerate a StartupTaint in order to have nodes provisioned for them. |
| `requirements` _[NodeSelectorRequirement](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#nodeselectorrequirement-v1-core) array_ | Requirements are layered with GetLabels and applied to every node. |
| `resources` _[ResourceRequirements](#resourcerequirements)_ | Resources models the resource requirements for the NodeClaim to launch |
| `kubelet` _[KubeletConfiguration](#kubeletconfiguration)_ | Kubelet defines args to be used when configuring kubelet on provisioned nodes. They are a subset of the upstream types, recognizing not all options may be supported. Wherever possible, the types and names should reflect the upstream kubelet types. |
| `nodeClassRef` _[NodeClassReference](#nodeclassreference)_ | NodeClassRef is a reference to an object that defines provider specific configuration |




#### NodeClaimTemplate





_Appears in:_
- [NodePoolSpec](#nodepoolspec)

| Field | Description |
| --- | --- |
| `metadata` _[ObjectMeta](#objectmeta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[NodeClaimSpec](#nodeclaimspec)_ |  |


#### NodeClassReference





_Appears in:_
- [NodeClaimSpec](#nodeclaimspec)

| Field | Description |
| --- | --- |
| `kind` _string_ | Kind of the referent; More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds" |
| `name` _string_ | Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names |
| `apiVersion` _string_ | API version of the referent |


#### NodePool



NodePool is the Schema for the NodePools API

_Appears in:_
- [NodePoolList](#nodepoollist)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `karpenter.sh/v1beta1`
| `kind` _string_ | `NodePool`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[NodePoolSpec](#nodepoolspec)_ |  |


#### NodePoolList



NodePoolList contains a list of NodePool



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `karpenter.sh/v1beta1`
| `kind` _string_ | `NodePoolList`
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `items` _[NodePool](#nodepool) array_ |  |


#### NodePoolSpec



NodePoolSpec is the top level provisioner specification. Provisioners launch nodes in response to pods that are unschedulable. A single provisioner is capable of managing a diverse set of nodes. Node properties are determined from a combination of provisioner and pod scheduling constraints.

_Appears in:_
- [NodePool](#nodepool)

| Field | Description |
| --- | --- |
| `template` _[NodeClaimTemplate](#nodeclaimtemplate)_ | Template contains the template of possibilities for the provisioning logic to launch a NodeClaim with. NodeClaims launched from this NodePool will often be further constrained than the template specifies. |
| `disruption` _[Disruption](#disruption)_ | Disruption contains the parameters that relate to Karpenter's disruption logic |
| `limits` _[Limits](#limits)_ | Limits define a set of bounds for provisioning capacity. |
| `weight` _integer_ | Weight is the priority given to the provisioner during scheduling. A higher numerical weight indicates that this provisioner will be ordered ahead of other provisioners with lower weights. A provisioner with no weight will be treated as if it is a provisioner with a weight of 0. |




#### ObjectMeta





_Appears in:_
- [NodeClaimTemplate](#nodeclaimtemplate)

| Field | Description |
| --- | --- |
| `labels` _object (keys:string, values:string)_ | Map of string keys and values that can be used to organize and categorize (scope and select) objects. May match selectors of replication controllers and services. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels |
| `annotations` _object (keys:string, values:string)_ | Annotations is an unstructured key value map stored with a resource that may be set by external tools to store and retrieve arbitrary metadata. They are not queryable and should be preserved when modifying objects. More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations |




#### ResourceRequirements



ResourceRequirements models the required resources for the NodeClaim to launch Ths will eventually be transformed into v1.ResourceRequirements when we support resources.limits

_Appears in:_
- [NodeClaimSpec](#nodeclaimspec)

| Field | Description |
| --- | --- |
| `requests` _[ResourceList](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcelist-v1-core)_ | Requests describes the minimum required resources for the NodeClaim to launch |


