# Nested Virtualization Support

## Overview

Nested virtualization lets a virtual machine run its own hypervisor inside itself.
That matters for workloads that need direct access to the CPU's virtualization
instructions: container sandboxes, per-tenant microVMs, development environments
that boot their own VMs. Before February 2026, the only way to do this on AWS
was to rent a bare-metal instance. Now it's a flag.

EC2 added a new `CpuOptions.NestedVirtualization` field to the `RunInstances`
and `CreateLaunchTemplate` APIs. Set it to `enabled` and the instance boots
with nested virt turned on. The catch is that only some instance families can
do this, and if you try it on one that can't, the launch fails with
`UnsupportedOperation`.

This PR gives Karpenter two things. First, a way for users to request nodes
with nested virt enabled via their `EC2NodeClass`. Second, logic that keeps
Karpenter from ever picking an instance type that doesn't support the feature,
so the scheduler never creates a NodeClaim that would die at launch.

## Goals

- Expose `cpuOptions.nestedVirtualization` on `EC2NodeClass.spec`.
- Pass `CpuOptions` through to the EC2 launch template.
- Only let Karpenter choose instance types that report `nested-virtualization`
  in `ProcessorInfo.SupportedFeatures` from `DescribeInstanceTypes`.

## Non-Goals (Future Work)

EC2's `CpuOptions` also has `coreCount` and `threadsPerCore`, which pin the
instance to a specific core/thread count. The API happily accepts these
alongside `nestedVirtualization` — I verified this by calling
`create-launch-template` with all three set and watching it succeed. They're
still out of scope for this PR though, because supporting them properly needs
instance-type-aware logic that doesn't exist yet.

Here's the wrinkle: every instance type has its own list of valid core counts
(`VCpuInfo.ValidCores` from `DescribeInstanceTypes`). `m8i.xlarge` takes
`[1, 2]`. `c8i.2xlarge` takes `[1, 2, 3, 4]`. If a NodeClass hardcodes
`coreCount: 4`, Karpenter would silently drop every instance type that
doesn't list 4 as valid, shrinking the candidate pool without saying anything
to the user. A later PR will add logic to pick the value dynamically based on
which type Karpenter actually selects, which keeps `coreCount` and
`threadsPerCore` useful without wrecking NodePool diversity.

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

The pointer matters: unset NodeClasses (the common case) shouldn't write an
empty `CpuOptions` block into every launch template.

No Kubernetes node label is exposed. The NodeClass spec is the only user-facing
surface. A label could make sense later if we wire it up to drive
instance-type configuration from pod-level requirements, the way
`instance-tenancy` does. Shipping one without that machinery would be
actively misleading though — a label like `instance-nested-virtualization=true`
reads as "the feature is on" but would really mean "the hardware could support
it", and users will not read it the way we mean it.

## Launch Behavior

### Instance Type Selection

When a user sets `cpuOptions.nestedVirtualization: enabled`, the provider's
instance-type compatibility check in
`pkg/providers/instancetype/compatibility` drops any type that doesn't
advertise the `nested-virtualization` processor feature. The reason this lives
in the compatibility package and not the launch-path filter in
`pkg/providers/instance/filter` is subtle but worth spelling out.

The compatibility check runs during instance type resolution, before the
scheduler picks a type. So the scheduler never sees incompatible types and
never creates a NodeClaim for one in the first place.

The launch-path filter runs later, after the scheduler has already committed.
If an incompatible type slipped through to that layer, the filter would reject
it, Karpenter would fail the NodeClaim, the scheduler would turn around and
create another one, and we'd repeat until every compatible type was exhausted.
The user would see this as "Karpenter keeps creating and deleting NodeClaims."
Not great.

### Launch Template

The `cpuOptions()` helper in `pkg/providers/launchtemplate` turns the
`CPUOptions` struct into EC2's `LaunchTemplateCpuOptionsRequest`. If
`NestedVirtualization` is nil it returns nil, so NodeClasses that don't set
the field produce launch templates with no `CpuOptions` block at all. When
it's set, the string (`enabled` or `disabled`) gets cast to the SDK's
`ec2types.NestedVirtualizationSpecification` enum.

## Instance Type Compatibility

`ProcessorInfo.SupportedFeatures` from `DescribeInstanceTypes` is the source
of truth. To see the currently supported types in a region:

```bash
aws ec2 describe-instance-types \
  --filters "Name=processor-info.supported-features,Values=nested-virtualization" \
  --query 'InstanceTypes[*].InstanceType'
```

We don't maintain a hand-written list of supported families anywhere in the
code or in this doc on purpose. Any such list would go stale the moment AWS
added another family, and worse, it would tempt future readers to treat the
doc as authoritative over the API. The compatibility check just calls
`lo.Contains(info.ProcessorInfo.SupportedFeatures, "nested-virtualization")`,
which means the answer always tracks what EC2 actually says.
