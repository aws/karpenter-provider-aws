# Karpenter - Node Overlay

## Background

Node launch decisions are the output a scheduling simulation algorithm that involves satisfying pod scheduling constraints with available instance type offerings. Cloud provider implementations are responsible for surfacing these offerings to the Karpenter scheduler through the `cloudprovider.GetInstanceTypes()` API. This separation of responsibilities enables cloud providers to make simplifying assumptions, but limits extensibility for more advanced use cases. NodeOverlays enable users to inject alternative assumptions into the scheduling simulation for more accurate scheduling simulations.

## Use Cases 

Price considerations in instance selection present a unique challenge. While each instance offering includes a base price that Karpenter uses in its scheduling algorithm's minimization function, the true cost equation is often more complex. External factors such as vendor fees, negotiated pricing discounts, or environmental considerations like carbon-offset costs remain opaque to Karpenter's decision-making process, potentially leading to suboptimal economic choices. User data points:

* Support for Saving plan: https://github.com/aws/karpenter-provider-aws/issues/3860
* Custom cost adjustment for licensing fee per host [https://github.com/aws/karpenter-provider-aws/issues/5033](https://github.com/aws/karpenter-provider-aws/issues/7346) 
* Capacity type discount as a percentage: https://github.com/aws/karpenter-provider-aws/pull/4697
* Instance Type Price Adjustment for Carbon Efficiency: https://github.com/aws/karpenter-provider-aws/pull/4686

Resource capacity management introduces additional complexity, particularly when dealing with extended resources and system overhead. While InstanceTypes come with well-defined standard resource capacities (CPU, memory, GPUs), Kubernetes' support for arbitrary extended resources creates a blind spot in Karpenter's simulation capabilities. Furthermore, system components like containerd, kubelet, and the host operating system consume varying amounts of resources that must be subtracted from the node's total capacity. Although Karpenter can calculate known overhead requirements, it struggles with custom operating systems and version-specific variations in resource consumption, making accurate capacity predictions challenging. User data points:

* Custom resource requests: https://github.com/kubernetes-sigs/karpenter/issues/751
* GPU time slicing: https://github.com/kubernetes-sigs/karpenter/issues/729
* Resource Operators: https://github.com/kubernetes-sigs/karpenter/issues/1219
* HugePage support: https://github.com/aws/karpenter-provider-aws/issues/6296

While users have requested features to adjustment for node capacity on VM memory overhead, the initial release of Node Overlay will not include this functionality. The team is gathering additional data on memory overhead usage and other capacity-related use cases to inform the future API requirements. Future enhancements may include support for discovered existing capacity patterns, various node overhead configurations, and additional resource adjustment scenarios. These potential expansions, including memory overhead adjustments, will be evaluated based on users feedback and real-world usage patterns.

* Support per provisioner(NodePool)/Instance type VM_MEMORY_OVERHEAD: https://github.com/aws/karpenter-provider-aws/issues/2141
* Set System Reserved as percentage: https://github.com/kubernetes-sigs/karpenter/issues/1595

## Proposal 

NodeOverlays are a new Karpenter API that enable users to fine-tune the scheduling simulation for advanced use cases. NodeOverlays can be configured with a `requirements` which flexibly constrain when they are applied to a simulation. Requirements can match well-known labels like `node.kubernetes.io/instance-type` or  `karpenter.sh/nodepool` and custom labels defined in a NodePool's `.spec.labels` or `.spec.requirements`. When multiple NodeOverlays match an offering for a given simulation, they are merged together using `spec.weight` to resolve conflicts.

Previous attempts to implement instance type modifications focused on a 1:1 mapping approach, but this was abandoned after user feedback indicated that managing the combinatorial explosion of variations would be impractical for real-world use cases.

```
apiVersion: karpenter.sh/v1alpha1
kind: NodeOverlay
metadata:
  name: default
spec:
  weight: ...
  requirements:
    ... 
  price: 
  priceAdjustment: ... 
  capacity: 
    ...
status:
  condition:
  - lastTransitionTime: "..."
    message: ""
    reason: ...
    status: "True"
    type: Ready
```

### Example

**Price:** Define a pricing override for instance types that match the specified labels. Users can override prices using a signed float representing the price override

```
kind: NodeOverlay
metadata:
  name: price-override
spec:
  requirements:
    - key: karpenter.sh/capacity-type
      operator: In
      values: ["on-demand"]
  # Examples of price override:
  price: "5.00" # Set price to $5.00
```

**Price Adjustment:** Define a pricing adjustment for instance types that match the specified labels. Users can adjust prices using either:
- A signed float representing the price adjustment
- A percentage of the original price (e.g., +10% for increase, -15% for decrees)

*Karpenter is currency-agnostic, so while these examples use USD, the same principles apply to all currencies.*

```
kind: NodeOverlay
metadata:
  name: price-adjustment
spec:
  requirements:
    - key: karpenter.sh/capacity-type
      operator: In
      values: ["on-demand"]
  # Examples of price adjustments:
  priceAdjustment: "+5.00" # Set price to $5.00 more than the original
  # or
  priceAdjustment: "-7.50" # Set price to $7.50 less than the original
  # or 
  priceAdjustment: "-10%" # Reduce price by 10% of original
  # or 
  priceAdjustment: "+15%" # Increase price to 15% of original
```

**Capacity:** Define capacity that will **add extended resources only**, and not replace any existing resources on the nodes. 

```
kind: NodeOverlay
metadata:
 name: extended-resource
spec:
  requirements:
  - key: node.kubernetes.io/instance-type
    operator: In
    values: ["m5.large", "m5.2xlarge", "m5.4xlarge", "m5.8xlarge", "m5.12xlarge"]
  capacity: 
    smarter-devices/fuse: 1
```

### HugePage Support 

A primary validation case for NodeOverlay capacity field is its handling of HugePages - a Linux kernel memory management optimization that enables larger page sizes (2MB or 1GB) compared to the default 4KB pages. This feature is particularly important for applications that manage large memory segments, such as databases, as it reduces TLB (Translation Lookaside Buffer) misses and improves overall memory access performance. However, HugePages must be allocated at system boot time and cannot be dynamically resized, requiring careful upfront planning of memory resources. This static nature, combined with their hardware-dependent characteristics and application-specific requirements, makes it challenging for schedulers to make accurate node provisioning decisions without explicit configuration of HugePage requirements. 

HugePages serve as an excellent example of extended resources that present unique challenges beyond simple resource allocation. Their configuration depends on both instance type and specific user requirements, leading to potential variations even within identical instance types. NodeOverlay addresses this challenge by providing a mechanism for users to explicitly define expected HugePage resources for specific instance types, enabling Karpenter to make informed scheduling decisions.

```
kind: NodeOverlay
metadata:
 name: hugepage-overlay
spec:
  requirements: 
  - key: node.kubernetes.io/instance-category
    operator: In
    values: ["c", "m", "r"]
  capacity:  
    hugepages-2Mi: 100Mi
    hugepages-1Gi: 2Gi
```

## Requirements  

The NodeOverlay's requirement mechanism provides precise control over value injection during scheduling simulations. 
Requirements operate the same way as the `spec.template.spec.requirements` from the NodePools CRDs. Requirements can be configured in multiple ways: an empty requirements affects all simulations universally, while specific requirements can establish various relationships with InstanceTypes - from one-to-many, one-to-one, or many-to-one mappings. These relationships can be further refined using additional constraints such as NodePool, NodeClass, availability zone, or other scheduling parameters, allowing for highly flexible and granular control over the simulation process.

However, supporting many-to-one mapping relationships introduces significant complexity in conflict resolution and value precedence. When multiple NodeOverlays target the same InstanceType, the system must deterministically handle overlapping or conflicting specifications. This raises important questions about priority ordering, value merging strategies, and how to maintain predictable outcomes during scheduling decisions. Care must be taken to design clear resolution rules that operators can understand and reason about, particularly when NodeOverlays have intersecting requirements for the same scheduling dimensions.

## Node Overlay Field Behavior and Consolidation Integration

Node Overlay introduces three key fields that affect Karpenter's behavior: `priceAdjustment`, `price` and `capacity`.
The `priceAdjustment` field enables users to modify instance pricing information, functioning similarly to spot price updates. When a Node Overlay with `priceAdjustment` is applied, Karpenter updates the pricing information for all affected instance types. These price adjustments are then incorporated into Karpenter's consolidation decisions, allowing the algorithm to make different choices based on the modified pricing data. For example, if certain instance types receive preferential pricing, Karpenter's consolidation algorithm will factor in these adjustments when making replacement decisions. The same concept will be applied to `price`.

Since Karpenter considers pod resources through its consolidation process, any Node Overlay capacity changes will be automatically applied as part of the regular consolidation cycle. During consolidation, Karpenter reassesses instance type selection based on the pods running on Karpenter-provisioned capacity. This natural integration means that Node Overlay changes take effect through normal consolidation operations, eliminating the need for additional drift detection or forced updates.

## Conflict Resolution 

The n:m relationship between NodeOverlays and instance types creates scenarios where multiple NodeOverlays could match a single instance type with different values. To manage these conflicts, we introduce an optional `.spec.weight` parameter that helps determine precedence. We have evaluated three distinct approaches for handling overlapping NodeOverlays:

**Option 1: Merge by Weight (Recommended)**

* Higher-weight overlays override values from lower-weight ones
* Supports layered configurations (base defaults + specific overrides)
* Example: If a base overlay sets 50Mi memory overhead and a higher-weight overlay specifies 10Mi, the final value is 10Mi
* Allows definition of multiple parameters across different NodeOverlays

**Option 2: Sequential Application by Weight**

* Values from each NodeOverlay are added together in weight order
* Results in cumulative effects that may be undesirable
* Example: A base overlay of 50Mi plus a specific overlay of 10Mi would result in 60Mi
* Not recommended as additive behavior rarely matches real-world requirements

**Option 3: Single Selection by Weight**

* Only applies the highest-weight NodeOverlay, ignoring all others
* Simple but restrictive approach
* Example: Between overlays with 50Mi and 10Mi, only one value would be used
* Lacks support for defining multiple parameters across different NodeOverlays

After careful consideration, **we recommend implementing Option 1 (Merge by Weight)**. This approach best supports common use cases like default configurations with specific overrides, while maintaining the flexibility to define multiple parameters across different NodeOverlays. This choice provides the most versatile solution without introducing the complexity of sequential application or the limitations of single selection.

### Example

```
kind: NodeOverlay
metadata:
  name: default
spec:
  priceAdjustment: "-10%"
  capacity:
    memory: 10Mi
---
kind: NodeOverlay
metadata:
  name: memory
spec:
  weight: 1 # required to resolve conflicts with default
  requirements:
  - key: karpenter.k8s.aws/instance-memory
    operator: Gt
    values: [2048]
  capacity:
    memory: 50Mi
---
kind: NodeOverlay
metadata:
  name: extended-resource
spec:
  # weight is not required since default does not conflict with extended-resource
  requirements:
  - key: node.kubernetes.io/instance-type
    operator: In
    values: ["m5.large", "m5.2xlarge", "m5.4xlarge", "m5.8xlarge", "m5.12xlarge"]
  capacity:
    smarter-devices/fuse: 1
```

### Misconfiguration

When multiple NodeOverlays have conflicting configurations, particularly with identical weights, we must choose between two fundamental approaches: failing open or failing closed for Karpenter’s scale and disruption actions. Each approach presents different tradeoffs for system reliability and operator experience.

**Fail Open Approach (recommended):**

* Using alphabetical ordering to resolve conflicts when weights are equal
* Setting overlay Ready status condition to false when conflicts occur
* Adding status conditions to inform users that the overlays aren't applied to any instance types
* Continuing provisioning/consolidation operations even when overlay configurations don't match users intent

While this approach maintains system availability failing open could lead to unexpected resource allocations and pricing decisions, potentially impacting workload performance and cost optimization goals. Furthermore, if an unapplied overlay defines extended resources (like HugePages), pods requiring these resources would fail to get provisioned capacity - matching Karpenter's current behavior when required resources are unavailable. This means that while the system remains operational, workloads depending on specific overlay-defined resources would still be blocked until the configuration issues are resolved.

```
kind: NodeOverlay
metadata:
  name: memory-1
spec:
  weight: 90
  requirements:
  - key: karpenter.k8s.aws/instance-memory
    operator: Gt
    values: ["m5.large", "m5.2xlarge"]
  capacity:
    memory: 50Mi
status:
  condition:
  - lastTransitionTime: "..."
    message: "conflict with overlay capacity-1"
    reason: "conflict"
    status: "False"
    type: Ready
---
kind: NodeOverlay
metadata:
  name: capacity-1
spec:
  weight: 90
  requirements:
  - key: node.kubernetes.io/instance-type
    operator: In
    values: ["c5.large", "c5.2xlarge", "m5.2xlarge"]
  capacity:
    memory: 100Mi
status:
  condition:
  - lastTransitionTime: "..."
    message: ""
    reason: ...
    status: "True"
    type: Ready
```

**Fail Closed Approach:**

* Halts new node launches until misconfigurations are resolved
* Can be implemented at two levels:

    1. Global: Stops all provisioning across the cluster
    2. NodePool-specific: Only affects misconfigured NodePools

* Provides strong guarantees and explicit failure signals

The fail closed approach, while providing strong guarantees and explicit failure signals, has significant operational drawbacks. By halting node launches until misconfigurations are resolved (either globally or per NodePool), it could unnecessarily impact cluster scalability and workload availability. This aggressive stance on configuration errors could turn minor overlay misconfigurations into broader service disruptions, particularly in scenarios where the misconfiguration affects only a subset of resources that aren't critical to immediate scaling needs. Since pods requiring specific overlay-defined resources would fail to schedule anyway (as they do today), the additional protection of failing closed provides minimal benefit while increasing operational risk.

## Alternative Proposal: NodePool Integration

Instead of creating a new NodeOverlay CRD, an alternative approach would be to extend the existing NodePool API to include `priceAdjustment`, `price` and `capacity` modification capabilities. This section explores this alternative and explains why it was not chosen as the recommended solution.

### Proposed NodePool API Extension

```
apiVersion: karpenter.sh/v1alpha5
kind: NodePool
metadata:
  name: default
spec:
  template:
    spec:
      requirements:
        - key: node.kubernetes.io/instance-type
          operator: In
          values: ["m5.large", "m5.2xlarge"]
      # New fields for overlay functionality
      overlays:
        -  priceAdjustment: "+10%"
          price: "10"
          capacity:
            hugepages-2Mi: 100Mi
            smarter-devices/fuse: 1
```

Pros
- Single CRD simplifies configuration management
- Direct mapping between requirements and modifications
- Clearer configuration ownership
- Familiar model for existing NodePool users

Cons
- Forces creation of multiple NodePools for different override scenarios
- More difficult to implement cross-cutting modifications
- Increases operational complexity through NodePool proliferation

While integrating overlay functionality into NodePools might appear to simplify the API surface, it would force operators to create multiple NodePools to handle different override scenarios, leading to increased operational complexity and potential scheduling inefficiencies. Additionally, this approach would make it more difficult to evolve the API as new requirements emerge and complicate the implementation of gradual rollouts. In contrast, the separate NodeOverlay CRD provides better separation of concerns, more flexible configuration options through overlay stacking, and a more maintainable system that can better adapt to future needs while keeping the NodePool API focused on its core provisioning responsibilities.

## Observability 

Observability for Node Overlay will center around providing users with practical tools to understand the impact of their configurations. We will develop a diagnostic tool that allows users to preview how their Node Overlay configurations affect instance pricing and capacity allocations before deployment. This tool will help operators visualize and validate changes to instance selection patterns, price modifications, and capacity adjustments without requiring extensive logging or event generation. While we maintain basic metrics for overlay application status, this proactive tooling approach provides more actionable insights than high-volume logs or events, which could create operational overhead and scalability concerns in large clusters.

```
kind: NodeOverlay
metadata:
  name: price-modification
spec:
  requirements:
  - key: node.kubernetes.io/instance-type
    operator: In
    values: ["m5.large"]
   priceAdjustment: "+0.10"
---
kind: NodeOverlay
metadata:
  name: extended-resource
spec:
  requirements:
  - key: node.kubernetes.io/instance-type
    operator: In
    values: ["c5.large"]
  capacity:
    smarter-devices/fuse: 1
```

```
> karpenteradm diagnose --type instance-type --overlay price-modification extended-resource --nodepool default
NODEPOOL    OVERLAY                      INSTANCE_TYPE   CAPACITY TYPE      PRICE    CAPCITY
default     "price-modification"         m5.large        spot               0.47     
default     "extended-resource"          c5.large        spot               0         smarter-devices/fuse: 1
...
```

The integration of Node Overlay adjustment values and deeper observability mechanisms will be deferred until clear users requirements emerge. This decision is partly driven by concerns that excessive events or logs produced by some controllers could be highly noisy or negatively impact the API server. By decoupling these observability tooling with the feature release, we can focus on delivering core functionality and validation tooling first, while leaving room for future enhancements. These enhancements will be based on real-world usage patterns and specific users needs for additional visibility into Node Overlay operations.

## Launch Scope

The NodeOverlay CRD will be introduced as a v1alpha1 feature, gated behind a feature flag in Karpenter. While the initial proposal focused solely on the `priceAdjustment` field, emerging users use cases, such as HugePage resource management, demonstrate a clear need for capacity field support. Given the validated users requirements, `priceAdjustment`, `price` and `capacity` fields will be included in the first API iteration. The graduation path for NodeOverlay will align with Karpenter's established CRD maturation process. Rather than adhering to a fixed timeline, the progression to stable status will be driven by community adoption and feedback, primarily measured through GitHub Analytics via issue discussions, feature-related PRs, and community engagement patterns. This data-driven approach ensures the API meets real-world requirements and use patterns before stabilization.

# Appendix

## References and Additional Reading

### Design Documents and Proposals

* [Node Overlay RFC](https://github.com/kubernetes-sigs/karpenter/pull/1305) - Current design proposal for Node Overlay feature
* [Instance Type Settings Proposal](https://github.com/aws/karpenter-provider-aws/pull/2390) - Previous proposal discussing instance type modifications
* [Instance Type Override Discussion](https://github.com/aws/karpenter-provider-aws/pull/2404#issuecomment-1250688882) - Community feedback on 1:1 semantic for overriding instance types

### Extended Resources

* [Understanding HugePages](https://www.netdata.cloud/blog/understanding-huge-pages/) - Comprehensive breakdown of HugePage concepts and implementation
* [Kubernetes HugePage Scheduling](https://kubernetes.io/docs/tasks/manage-hugepages/scheduling-hugepages/) - Official Kubernetes documentation on HugePage management
* [GPU Operator](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/overview.html) - Official NVIDIA GPU Operator documentation  

