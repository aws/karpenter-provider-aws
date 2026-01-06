---
title: "NodeOverlays"
linkTitle: "NodeOverlays"
weight: 40
description: >
  Understand NodeOverlays and how they enable fine-tuning of Karpenter's scheduling simulation for advanced use cases.
---

<i class="fa-solid fa-circle-info"></i> <b>Feature State: </b> [Alpha]({{<ref "../reference/settings#feature-gates" >}})


Karpenter uses NodeOverlays to inject alternative instance type information into the scheduling simulation for more accurate scheduling decisions.
NodeOverlays enable users to fine-tune instance pricing and add extended resources to instance types that should be considered during Karpenter's decision-making process.
They provide a flexible way to account for real-world factors like savings plans, licensing costs, and custom hardware resources that aren't captured in the base instance data from cloud providers.

NodeOverlays work by modifying the instance type information that Karpenter uses during its scheduling simulation.
When Karpenter evaluates which instance types can satisfy pending pod requirements, it applies any matching NodeOverlays to adjust pricing information or add extended resources before making provisioning decisions.

## NodeOverlay Configuration

```yaml
apiVersion: karpenter.sh/v1alpha1
kind: NodeOverlay
metadata:
  name: example-overlay
spec:
  # Optional weight for conflict resolution (higher weight wins)
  weight: 10
  
  # Requirements determine which instance types this overlay applies to
  requirements:
    - key: node.kubernetes.io/instance-type
      operator: In
      values: ["m5.large", "m5.xlarge"]
    - key: karpenter.sh/capacity-type  
      operator: In
      values: ["spot"]
    - key: karpenter.k8s.aws/instance-cpu 
      operator: Gte
      values: ["32"]
  
  # Price and priceAdjustment are mutually exclusive
  # Price override (sets absolute price)
  price: "5.00"
  
  # Price adjustment (modifies existing price)
  priceAdjustment: "+10%"  # or "-0.50" for absolute adjustment
  
  # Extended resources to add to matching instance types
  capacity:
    hugepages-2Mi: 100Mi
    hugepages-1Gi: 2Gi
    custom-device/gpu-slice: 4
```

## spec.weight
Optional integer that determines precedence when multiple NodeOverlays match the same instance type. Higher weights take precedence over lower weights. When weights are equal, alphabetical ordering by name is used for conflict resolution. If not specified, the default weight is 0. If there is a conflict between NodeOverlays with the same weight, it will be indicated in the status and the NodeOverlay will not be applied.

## spec.requirements
Array of requirements that determine which instance types this overlay applies to. Uses the same format as NodePool requirements and supports all standard Kubernetes label selectors. An empty requirements array applies the overlay to all instance types. Kubernetes defines the following [Well-Known Labels](https://kubernetes.io/docs/reference/labels-annotations-taints/), and cloud providers (e.g., AWS) implement them.

Currently, requirements sets are defined based on the well-known labels that are discovered for instance types. In addition to the well-known labels from Kubernetes, Karpenter supports AWS-specific labels for more advanced scheduling. See the full list [here](../scheduling/#well-known-labels).

{{% alert title="Note" color="primary" %}}
There is currently a limit of 100 on the total number of requirements on both the NodeOverlay.
{{% /alert %}}

## spec.price
Absolute price override as a string representing the price in your currency. This completely replaces the original instance price reported by the cloud provider. Karpenter is currency-agnostic, so this works with any currency unit.

## spec.priceAdjustment
Price modification that can be specified as:
- **Absolute adjustment**: `"+5.00"` (increase by 5.00) or `"-2.50"` (decrease by 2.50)
- **Percentage adjustment**: `"+15%"` (increase by 15%) or `"-10%"` (decrease by 10%)

## spec.capacity
Map of extended resources to add to matching instance types. These resources are added to the existing standard capacity and do not replace or modify well-known resources. Only extended resources should be specified here.

```yaml
apiVersion: karpenter.sh/v1alpha1
kind: NodeOverlay
metadata:
  name: custom-devices
spec:
  requirements:
    - key: node.kubernetes.io/instance-type
      operator: In
      values: ["m5.large", "m5.xlarge", "m5.2xlarge"]
  capacity:
    smarter-devices/fuse: 1
    custom-hardware/accelerator: 2
```

## Conflict Resolution

When multiple NodeOverlays match the same instance type, conflicts are resolved using the following rules:

1. **Weight-based precedence**: Higher weight values take precedence over lower weights
2. **Alphabetical ordering**: When weights are equal, overlays are applied in alphabetical order by name
3. **Field-level merging**: Higher-weight overlays override specific fields from lower-weight overlays, but capacity fields from different overlays are merged together

### Example Conflict Resolution

```yaml
apiVersion: karpenter.sh/v1alpha1
kind: NodeOverlay
metadata:
  name: overlay-a
spec:
  weight: 5
  requirements:
    - key: node.kubernetes.io/instance-type
      operator: In
      values: ["m5.large"]
  priceAdjustment: "-10%"
  capacity:
    hugepages-2Mi: 50Mi
---
apiVersion: karpenter.sh/v1alpha1
kind: NodeOverlay
metadata:
  name: overlay-b
spec:
  weight: 10  # Higher weight
  requirements:
    - key: node.kubernetes.io/instance-type
      operator: In
      values: ["m5.large"]
  priceAdjustment: "-20%"  # This overrides overlay-a's adjustment
  capacity:
    custom-device/gpu: 1   # This is merged with hugepages-2Mi from overlay-a
```

**Result for m5.large instances:**
- Price adjustment: `-20%` (from overlay-b, overrides overlay-a)
- Capacity: `hugepages-2Mi: 50Mi` (from overlay-a) + `custom-device/gpu: 1` (from overlay-b)

## Integration with Consolidation

NodeOverlay modifications are automatically integrated into Karpenter's consolidation process:

* **Price adjustments** affect consolidation decisions by changing the cost calculations used to determine optimal instance selections during replacement operations
* **Capacity additions** are considered during consolidation when evaluating whether workloads can be moved between nodes
* Changes take effect through normal consolidation cycles without requiring additional drift detection or forced node replacement

When NodeOverlay configurations change, Karpenter incorporates these changes into its next consolidation evaluation, potentially triggering node replacements if the new configurations significantly change the optimal instance selection for existing workloads.

## Status and Observability

NodeOverlays include status conditions to help you understand their current state and troubleshoot configuration issues.

### Common Status Conditions

* **Ready=True**: The overlay is successfully applied to matching instance types
* **Ready=False**: Configuration conflicts, requirement mismatches, or other errors prevent the overlay from being applied

### Status Messages

When `Ready=False`, the status message provides specific information about the issue:

```yaml
status:
  conditions:
  - type: ValidationSucceeded
    status: "False"
    lastTransitionTime: "2024-07-24T18:30:00Z"
    reason: "Conflict"
    message: "conflict with another overlay"
```

## Limitations and Considerations

* **Resource Scope**: NodeOverlays can only add extended resources; they cannot modify or remove standard resources (CPU, memory, storage)
* **Actual vs. Simulated**: Capacity modifications only affect Karpenter's scheduling simulation; actual node resources must be configured through other means
* **Pricing vs. Billing**: Price adjustments influence Karpenter's scheduling decisions but don't affect actual cloud provider billing
* **Alpha Status**: NodeOverlays are currently in alpha (v1alpha1) and the API may change in future versions