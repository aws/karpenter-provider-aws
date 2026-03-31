# CPU Options and Nested Virtualization

## Overview

AWS announced nested virtualization support on virtual EC2 instances in February 2026,
enabling KVM-based workloads (container sandboxes, microVMs, development VMs) without
bare-metal instances. The feature is configured via the `CpuOptions.NestedVirtualization`
field in the EC2 `RunInstances` and `CreateLaunchTemplate` APIs.

Karpenter needs to expose this capability on `EC2NodeClass` so users can request nodes
with nested virtualization enabled, and Karpenter needs to filter out instance types that
do not support the feature to avoid launch failures.

## Goals

- Expose `cpuOptions` on `EC2NodeClass.spec` with `coreCount`, `threadsPerCore`, and
  `nestedVirtualization` fields.
- Pass `CpuOptions` through to the EC2 launch template.
- Filter instance types to only those reporting `nested-virtualization` in
  `ProcessorInfo.SupportedFeatures` from `DescribeInstanceTypes`.
- Validate that `nestedVirtualization` is mutually exclusive with `coreCount` and
  `threadsPerCore` (EC2 API constraint).
- Cache `UnsupportedOperation` fleet errors as unfulfillable capacity.

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
  # ... other fields
```

### CPUOptions Struct

```go
type CPUOptions struct {
    CoreCount            *int32  `json:"coreCount,omitempty"`
    ThreadsPerCore       *int32  `json:"threadsPerCore,omitempty"`
    NestedVirtualization *string `json:"nestedVirtualization,omitempty"`
}
```

CEL validation enforces that `nestedVirtualization: enabled` cannot be combined with
`coreCount` or `threadsPerCore` (EC2 rejects the combination).

### Instance Type Label

A new well-known label `karpenter.k8s.aws/instance-nested-virtualization` is populated
from `ProcessorInfo.SupportedFeatures` during instance type resolution. Instance types
that report `nested-virtualization` in their supported features receive the label value
`"true"`.

As of March 2026, only the `*8i*` families support this feature: c8i, c8i-flex, m8i,
m8i-flex, r8i, r8i-flex (54 instance types total). No ARM, Xen, or bare-metal instances
support it.

## Launch Behavior

### Instance Type Filtering

When an `EC2NodeClass` sets `cpuOptions.nestedVirtualization: enabled`, a
`NestedVirtualizationFilter` in the instance filter chain rejects any instance type
lacking the `instance-nested-virtualization=true` label. This runs after the
`CompatibleAvailableFilter` and before capacity reservation filters.

### Launch Template

The `cpuOptions()` converter maps the `CPUOptions` struct to
`LaunchTemplateCpuOptionsRequest`. It returns `nil` when all fields are nil (avoiding an
empty `CpuOptions` block in the API call). The `NestedVirtualization` string is cast to
the SDK enum type `ec2types.NestedVirtualizationSpecification`.

### Error Handling

`UnsupportedOperation` is added to the `unfulfillableCapacityErrorCodes` set so that
launches against incompatible instance types (if they bypass the filter) are cached as
unavailable rather than retried indefinitely.

## Instance Type Compatibility

The authoritative signal is `ProcessorInfo.SupportedFeatures` from `DescribeInstanceTypes`:

```bash
aws ec2 describe-instance-types \
  --filters "Name=processor-info.supported-features,Values=nested-virtualization" \
  --query 'InstanceTypes[*].InstanceType'
```

This returns only the families that actually support the feature, avoiding heuristic-based
filtering (e.g., checking architecture + hypervisor) which would be both over-inclusive
(allowing older Intel families that don't support it) and fragile (breaking when AWS adds
support to new families).
