# Nested Virtualization Support

## Overview

AWS announced nested virtualization support on virtual EC2 instances in February 2026,
enabling KVM-based workloads (container sandboxes, microVMs, development VMs) without
bare-metal instances. The feature is configured via the `CpuOptions.NestedVirtualization`
field in the EC2 `RunInstances` and `CreateLaunchTemplate` APIs.

Karpenter needs to expose this capability on `EC2NodeClass` so users can request nodes
with nested virtualization enabled, and Karpenter needs to filter out instance types that
do not support the feature to avoid launch failures.

## Goals

- Expose `cpuOptions.nestedVirtualization` on `EC2NodeClass.spec`.
- Pass `CpuOptions` through to the EC2 launch template.
- Filter instance types to only those reporting `nested-virtualization` in
  `ProcessorInfo.SupportedFeatures` from `DescribeInstanceTypes`.

## Non-Goals (Future Work)

`coreCount` and `threadsPerCore` are valid `CpuOptions` fields in the EC2 API and can be
combined with `nestedVirtualization` (verified empirically). However, they introduce design
complexity in Karpenter: each instance type has different `ValidCores` values, so a static
`coreCount` on the NodeClass would need instance-type-aware filtering to avoid restricting
NodePool diversity. These fields will be addressed in a follow-up PR with dynamic selection.

## API Updates

### EC2NodeClass Spec

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: nested-virt
spec:
  cpuOptions:
    nestedVirtualization: enabled
```

### CPUOptions Struct

```go
type CPUOptions struct {
    NestedVirtualization *string `json:"nestedVirtualization,omitempty"`
}
```

No well-known label is exposed. The NodeClass configuration is the only user-facing surface
for this feature. Label-based selection can be added in the future based on real-world
use cases, consistent with the project's convention of introducing labels only when they
drive instance configuration (e.g., `instance-tenancy`).

## Launch Behavior

### Instance Type Compatibility

When an `EC2NodeClass` sets `cpuOptions.nestedVirtualization: enabled`, a
`nestedVirtualizationCheck` in the instance type compatibility chain rejects any
instance type that does not report `nested-virtualization` in
`ProcessorInfo.SupportedFeatures`. The check runs during instance type resolution,
so the Karpenter scheduler never considers incompatible types and no NodeClaim is
created for them. This prevents the create/delete churn that would occur if the
check lived in the launch-path filter chain.

### Launch Template

The `cpuOptions()` converter maps the `CPUOptions` struct to
`LaunchTemplateCpuOptionsRequest`. It returns `nil` when `NestedVirtualization` is nil
(avoiding an empty `CpuOptions` block in the API call). The value is cast to
`ec2types.NestedVirtualizationSpecification`.

## Instance Type Compatibility

The authoritative signal is `ProcessorInfo.SupportedFeatures` from `DescribeInstanceTypes`:

```bash
aws ec2 describe-instance-types \
  --filters "Name=processor-info.supported-features,Values=nested-virtualization" \
  --query 'InstanceTypes[*].InstanceType'
```

As of April 2026, this returns 54 instance types across c8i, c8i-flex, m8i, m8i-flex,
r8i, r8i-flex. No ARM, Xen, or bare-metal instances support the feature. Using the API
filter avoids heuristic-based filtering which would be both over-inclusive and fragile.
