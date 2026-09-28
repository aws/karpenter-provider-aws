# CPU Options: Threads Per Core

## Overview

EC2 lets you disable simultaneous multithreading (SMT, or hyperthreading) on an instance by launching it
with one thread per core. Users want that for per-vCPU licensed software, for latency-sensitive or
security-sensitive workloads that shouldn't share a physical core, and for workloads that simply benchmark
better without SMT. Today the only way to get such nodes from Karpenter is to leave `cpuOptions` alone and
disable SMT from user data at boot, which leaves Karpenter scheduling against a vCPU count the node no
longer has.

The `EC2NodeClass` already carries `spec.cpuOptions` for nested virtualization
([cpu-options-nested-virtualization.md](./cpu-options-nested-virtualization.md)). That design deliberately
left EC2's `CoreCount` and `ThreadsPerCore` out of scope because supporting them "needs instance-type-aware
logic that doesn't exist yet": the valid values are a property of each instance type, and a value hardcoded
in the NodeClass would silently shrink the candidate pool. This document adds that logic for
`threadsPerCore`, which is the value users actually want to set. `coreCount` stays out of scope.

## Goals

- Expose `cpuOptions.threadsPerCore` on `EC2NodeClass.spec`.
- Launch every instance type with the requested threads per core, deriving the core count EC2 also requires
  from the instance type itself, so NodePool diversity is preserved.
- Make Karpenter's scheduling arithmetic reflect the vCPU count the instance actually boots with. The
  scheduler must never pack pods against more CPU than the kubelet will report.
- Exclude instance types that can't launch with the requested value, so the scheduler never creates a
  NodeClaim EC2 would reject with `UnsupportedOperation`.

## Non-Goals

- `cpuOptions.coreCount`. The reasons in the nested virtualization design still hold, and there is no
  instance-type-independent way to express it. A user who wants fewer cores should pick a smaller instance
  type.
- Modifying the CPU options of existing instances (`ModifyInstanceCpuOptions` needs a stopped instance).
  Changing `threadsPerCore` drifts the NodeClass and replaces nodes, as any other launch template change does.
- A node label for threads per core, for the same reason the nested virtualization design gave: it would be
  read as "the feature is on" when it can only mean "the NodeClass asked for it".

## API

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: no-smt
spec:
  cpuOptions:
    threadsPerCore: 1
```

```go
type CPUOptions struct {
    NestedVirtualization *string `json:"nestedVirtualization,omitempty"`
    // +kubebuilder:validation:Minimum:=1
    // +kubebuilder:validation:Maximum:=2
    ThreadsPerCore *int32 `json:"threadsPerCore,omitempty"`
}
```

`threadsPerCore` is the *effective* number of threads per core the user wants, not a raw pass-through of
the EC2 field. That distinction is what makes the feature usable across a mixed NodePool (see below).
The field is a pointer so unset NodeClasses keep producing launch templates with no `CpuOptions` block.
EC2 only accepts `1` or `2`.

## Design

### Resolving the CPU topology per instance type

EC2's `RunInstances`/`CreateLaunchTemplate` require `CoreCount` and `ThreadsPerCore` to be set together
("Both the core count and threads per core must be specified in the request"), and `CreateFleet` overrides
can't vary `CpuOptions` per instance type. The core count is a property of the instance type
(`VCpuInfo.DefaultCores` from `DescribeInstanceTypes`), so Karpenter resolves it per instance type.

`pkg/providers/instancetype/cpuoptions` holds the one function that decides what an instance type launches
with:

| NodeClass `threadsPerCore` | Instance type default | Result |
|---|---|---|
| unset | any | default layout, no `CpuOptions` |
| equal to `DefaultThreadsPerCore` | e.g. `1` on Graviton, `2` on x86 | default layout, no `CpuOptions` |
| different | e.g. `1` on x86 | `CoreCount = DefaultCores`, `ThreadsPerCore = <requested>`, vCPUs = `DefaultCores * requested` |

Only requesting `CpuOptions` where the layout actually changes has two effects that matter:

1. `threadsPerCore: 1` on a NodePool that mixes Graviton and x86 does the right thing. Graviton instance
   types already run one thread per core and launch exactly as before; only the x86 types get `CpuOptions`.
2. Instance types that don't support `CpuOptions` at all (bare metal, and a few others EC2 lists without
   `ValidThreadsPerCore`) are still usable when their default already satisfies the request, and are only
   excluded when the request would change their layout.

The same package answers whether the instance type *can* launch that way: an explicit layout is supported
only when EC2 lists the requested value in `VCpuInfo.ValidThreadsPerCore` and the default core count in
`VCpuInfo.ValidCores`. As with nested virtualization, there is no hand-written list of supported families;
`DescribeInstanceTypes` is the source of truth.

### Scheduling arithmetic

The instance type provider computes everything it derives from `VCpuInfo.DefaultVCpus` today from the
resolved vCPU count instead:

- `capacity[cpu]`
- the default kube-reserved CPU (the tiered percentage table is applied to the launched vCPUs)
- `podsPerCore`
- the `vcpus` variable exposed to kubelet CEL expressions, on both the scheduling path and the launch
  template path (which reads it back from the `instance-cpu` requirement)
- the `karpenter.k8s.aws/instance-cpu` requirement/label

`instance-cpu` follows the launched vCPU count on purpose. It is documented as the number of vCPUs on the
instance, users select on it to size nodes, and it must agree with `capacity[cpu]` because the launch
template resolver derives the CEL `vcpus` variable from it. The `karpenter_cloudprovider_instance_type_cpu_cores`
metric keeps reporting the instance type's default vCPUs: it is labelled by instance type only, not by
NodeClass, so it can't carry a per-NodeClass value.

`cpuOptions` is added to the instance type cache key so a NodeClass with `threadsPerCore` set doesn't
share cached instance types with one that doesn't.

Pricing is untouched. EC2 charges the same for an instance regardless of its CPU options, so a
`threadsPerCore: 1` node is, per vCPU, twice as expensive; that is the user's explicit choice and the
scheduler's cost comparison across instance types remains correct because every candidate is priced the
same way.

### Instance type compatibility

`pkg/providers/instancetype/compatibility` gains a `threadsPerCore` check next to the nested virtualization
one. It runs during instance type resolution, before the scheduler picks a type, for the reason the nested
virtualization design spells out: a launch-path filter would let the scheduler commit to an instance type,
fail the NodeClaim, and loop through every incompatible type before finding one that works.

### Launch templates

The launch template resolver groups instance types into launch templates by AMI, max pods, EFA count and
reservation today. The resolved core count joins that key: instance types with the same default core count
share a launch template, and the template's `CpuOptions` carries `CoreCount`/`ThreadsPerCore` for all of
them. Instance types launching with their default layout have a core count of zero and stay grouped as they
are today.

The resolver has no access to `DescribeInstanceTypes` data, only to Karpenter's `InstanceType`, so it gets a
`VCPUInfoLookup` wired from the instance type provider, the same way it already gets an ENI lookup for the
CEL variables. Both paths therefore resolve the topology from the same cached EC2 data, which is the property
that keeps the launched instance's vCPUs equal to what the scheduler planned for. If the lookup misses (the
cache was replaced between resolution and launch), the launch fails and is retried rather than launching
without `CpuOptions` and booting a node with more vCPUs than the NodeClaim was sized for.

`CpuOptions` is emitted only when something is set: nested virtualization on its own, core count and threads
per core on their own, or both together. A NodeClass that sets neither still produces launch templates with
no `CpuOptions` block.

### Launch template count

Setting `threadsPerCore` can raise the number of launch templates Karpenter creates for a NodeClass, from
"one per distinct max pods value" to "one per distinct (max pods, core count)". In practice core counts
collide heavily across families of the same size, so the increase is modest, and launch templates are cached
and reused across launches. `CreateFleet` accepts at most 50 launch template configurations per request; the
existing max pods grouping is already subject to that limit and this change doesn't introduce a new
enforcement point for it.

## Alternatives Considered

**Always pass `CpuOptions` when `threadsPerCore` is set.** Simpler to explain, but it would put every
Graviton instance type on its own launch template for no change in behaviour, and it would exclude instance
types that don't support `CpuOptions` even when their default layout already matches. Resolving the
effective layout per instance type is the more useful contract.

**Expose `coreCount` too.** Rejected for the reasons in the nested virtualization design. A per-NodeClass
core count can only be valid for a handful of instance types.

**Disable SMT in user data.** Works today, but Karpenter would keep scheduling against the full vCPU count,
overcommitting every node by a factor of two until the kubelet's real capacity is observed, and the
`instance-cpu` label would be wrong. The point of this feature is that Karpenter's arithmetic is right
before the node exists.

## Testing

- Unit tests for the topology resolution and support check.
- Compatibility tests for the exclusion rules.
- Instance type provider tests for capacity, kube-reserved, pods-per-core, CEL `vcpus`, the `instance-cpu`
  requirement, exclusion, and the cache key.
- Launch template tests asserting, per launched instance type, that the launch template requests the
  instance type's default core count with the configured threads per core, that single-threaded instance
  types get no `CpuOptions`, that instance types with different core counts land on different launch
  templates, and that nested virtualization composes with it.
- An e2e test in the scheduling suite that launches an x86 instance with `threadsPerCore: 1` and checks the
  instance's `CpuOptions`, the node's reported CPU capacity, the NodeClaim's capacity and the
  `instance-cpu` label all agree.
