# Offerings: Exposing Instance Configuration

**Audience:** contributors adding a new instance configuration option to `karpenter-provider-aws`.

**Status:** This document is **guidance, not law.** It describes the reasoning new designs should
engage with and the defaults they should start from. A design that deviates isn't wrong, but it
should explain why. Maintainers have the final say on every feature interface.

Existing code doesn't consistently follow this guidance. For new work, treat this document as the
source of truth, not the existing code.

Before requesting review, check your design against the evaluation criteria in §2.3.

**Contents**

- [1. The InstanceType model](#1-the-instancetype-model)
  - [1.1 Overview](#11-overview)
  - [1.2 Independent vs. dependent options](#12-independent-vs-dependent-options)
  - [1.3 Surfacing features via instance types and offerings](#13-surfacing-features-via-instance-types-and-offerings)
- [2. Evaluating a feature](#2-evaluating-a-feature)
  - [2.1 Selecting a surface](#21-selecting-a-surface)
  - [2.2 Invariants](#22-invariants)
  - [2.3 Evaluation criteria](#23-evaluation-criteria)
  - [2.4 Wiring checklist](#24-wiring-checklist)

---

## 1. The InstanceType model

### 1.1 Overview

Karpenter's scheduler reasons about two layers.

An **InstanceType** describes a family of possible nodes: its name, resource capacity, overhead, and
a set of **requirements**, which are the labels a node of this type could carry.

An **Offering** is a concrete launch configuration of that instance type (a specific zone, capacity
type, reservation, and so on) along with a price and an availability signal.

```
InstanceType: m5.large
├── Requirements                          # common to every offering; multi-valued = a choice
│   ├── kubernetes.io/arch                : [amd64]
│   ├── topology.kubernetes.io/zone       : [us-west-2a, us-west-2b]
│   ├── karpenter.sh/capacity-type        : [on-demand, spot, reserved]
│   ├── karpenter.k8s.aws/instance-tenancy: [default, dedicated]    <- instance-type-level choice
│   └── karpenter.k8s.aws/capacity-reservation-id: [cr-abc, cr-def] <- union of all offering values
├── Capacity / Overhead                   # per instance type; an offering may override
└── Offerings                             # concrete launch configurations
    ├── {zone: us-west-2a, capacity-type: on-demand}                  $0.096  available
    ├── {zone: us-west-2a, capacity-type: spot}                       $0.031  available
    ├── {zone: us-west-2b, capacity-type: on-demand}                  $0.096  unavailable (ICE)
    ├── {zone: us-west-2b, capacity-type: spot}                       $0.031  available
    ├── {zone: us-west-2a, capacity-type: reserved, ...id: cr-abc}    ~$0     available
    └── {zone: us-west-2b, capacity-type: reserved, ...id: cr-def}    ~$0     available
```

The model is hierarchical: **properties shared by every offering live on the instance type, and
offerings encode only the differences.**

**Which keys live on offerings.** A key appears on offerings when its value changes something
tracked per offering: whether the combination exists at all (a reservation exists in one zone), or
its price, availability, or resources. Every other key stays on the instance type and applies to
every offering. The launch configurations Karpenter can produce are therefore the offerings, each
combined with every value of the instance-type-only keys. The six offerings above, each with either
tenancy, give twelve. Dependency (§1.2) is the most common reason a key needs offerings, but not the
only one: every partition is valid with every offering, yet partitions are enumerated because each
has its own availability (§1.3.2).

### 1.2 Independent vs. dependent options

**Independent** options can be set regardless of the rest of the launch configuration. Instance
tenancy is independent: tenancy can be chosen freely whatever the zone, capacity type, or placement
group. An independent option forms a full cross product with the other dimensions, so it belongs on
the instance type as a multi-valued requirement and needs no offerings, unless it changes something
tracked per offering (§1.1).

**Dependent** options are only valid in combination with specific other parameters. Capacity
reservations are zonal: a reservation exists in exactly one zone, for one instance type. The valid
launch configurations form a sparse matrix, and offerings are the cells:

```
m5.large                us-west-2a    us-west-2b    us-west-2c
  on-demand                 ✓             ✓             ✓
  spot                      ✓             ✓             ✓
  reserved / cr-abc         ✓             ·             ·        <- ODCR in us-west-2a
  reserved / cr-def         ·             ✓             ·        <- capacity block in us-west-2b

InstanceType requirements (union of all rows/columns):
  zone                      : [us-west-2a, us-west-2b, us-west-2c]
  capacity-type             : [on-demand, spot, reserved]
  capacity-reservation-id   : [cr-abc, cr-def]

Offerings (the ✓ cells only):    8 offerings, not 12
```

The instance type advertises everything that is possible *somewhere*, and the offerings say which
combinations are actually possible. A pod that selects `cr-abc` and `us-west-2b` passes the instance
type filter but finds no compatible offering. That is the correct outcome: it's reported as an
unsatisfiable scheduling constraint rather than a failed launch.

### 1.3 Surfacing features via instance types and offerings

There are three surfaces, and two of them can be combined.

#### 1.3.1 Requirement values on the instance type

*Example: instance tenancy.*

The instance type advertises every value it supports:

```go
// pkg/providers/instancetype/types.go — computeRequirements
scheduling.NewRequirement(v1.LabelInstanceTenancy, corev1.NodeSelectorOpIn,
    string(ec2types.TenancyDefault), string(ec2types.TenancyDedicated)),
```

By default nothing narrows that set, so pods drive the choice:

```yaml
# This workload needs a dedicated instance
apiVersion: v1
kind: Pod
spec:
  nodeSelector:
    karpenter.k8s.aws/instance-tenancy: dedicated
```

An administrator can constrain the option on a NodePool, which takes the choice away from the pods
that schedule to it:

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
spec:
  template:
    spec:
      requirements:
        - key: karpenter.k8s.aws/instance-tenancy
          operator: In
          values: ["dedicated"]
```

At launch, `Create` derives the compatible values from the NodeClaim. A fleet request carries a
single tenancy, so tenancy is a select-one dimension and needs a documented rule. Its rule is a
preference order with a default:

```go
// pkg/providers/instance/instance.go — getTenancyType (illustrative)
// If the requirement is unset, or allows both values, prefer `default`.
for _, tenancy := range []string{string(ec2types.TenancyDefault), string(ec2types.TenancyDedicated)} {
    if requirement.Has(tenancy) {
        return tenancy
    }
}
```

The resolved value is then set as a label (`labels[v1.LabelInstanceTenancy] = i.Tenancy`), so the
node reflects what was launched.

Use this surface when the option has a small, closed set of values and the choice is per-workload.

#### 1.3.2 Distinct offerings

*Examples: capacity reservations, placement group partitions.*

Use offerings when the option is **dependent** on other launch parameters, or when it changes an
offering's price, availability, or advertised resources. Offerings are the only layer that can carry
those differences.

To add offerings, implement an `OfferingResolver` and register it with the offering provider.
Resolvers run in order, each receiving the previous resolver's output. A resolver takes one of two
shapes: it either appends sparse cells or fans out existing ones.

**Sparse: append new cells.** Capacity reservations are the example. A reservation is valid for one
instance type in one zone, so the resolver appends one offering per reservation, filling in the
reserved rows of the matrix in §1.2. Each offering carries its own price, availability, and capacity:

```go
// pkg/providers/instancetype/offering/reserved_capacity_resolver.go (illustrative)
offering := &cloudprovider.Offering{
    Requirements: scheduling.NewRequirements(
        scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, karpv1.CapacityTypeReserved),
        scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, reservation.AvailabilityZone),
        scheduling.NewRequirement(cloudprovider.ReservationIDLabel, corev1.NodeSelectorOpIn, reservation.ID),
        // ...
    ),
    Price:               price,
    Available:           /* compatible && has capacity && not expiring && not zonal-shifted */,
    ReservationCapacity: reservationCapacity,
}
```

Use this shape for dependent options, where the valid combinations are a small subset of the cross
product. The offering count grows with the number of underlying resources (here, reservations), not
with the size of the matrix.

**Fan-out: expand existing cells.** Partition placement groups are the example. Partitions are
independent (§1.2), and topology alone doesn't need offerings: the scheduler builds topology domains
from instance-type requirements, so advertising `[1..N]` there is enough for topology spread
constraints. They're offerings because each partition carries its **own availability signal**
(§1.1): an insufficient capacity error in one partition shouldn't block the others. The resolver replaces each offering with one copy per partition, adding the
partition requirement, scoping availability to that partition, and carrying everything else through.
Since the input offerings are already zonal, each copy is a (zone, partition) cell, which matches
EC2's model of up to seven partitions per Availability Zone:

```go
// pkg/providers/instancetype/offering/placement_group_resolver.go (illustrative)
for _, offering := range offerings {
    for partition := 1; partition <= partitionCount; partition++ {
        reqs := scheduling.NewRequirements(offering.Requirements.Values()...)
        reqs.Add(scheduling.NewRequirement(v1.LabelPlacementGroupPartition, corev1.NodeSelectorOpIn, fmt.Sprintf("%d", partition)))
        expanded = append(expanded, &cloudprovider.Offering{
            Requirements: reqs,
            Price:        offering.Price,
            Available:    offering.Available && !partitionUnavailable(offering, partition),
            // ReservationCapacity, CapacityOverride, OverheadOverride copied from the input offering
        })
    }
}
```

Use this shape when an option applies to every offering but each value needs its own availability
or resources. Fan-out multiplies the offering count, so the resolver should be a no-op unless the
feature is configured; the placement group resolver returns its input unchanged when the NodeClass
has no partition placement group (see §2.4). Because a fan-out resolver expands whatever it receives,
it runs after the resolvers that append cells, so reserved offerings are expanded too.

Fan-out is also the expected shape for **mutually exclusive views of the same hardware.** Offerings
can advertise different resources through `CapacityOverride` and `OverheadOverride`, so they're the
only place to model this. For example, GPU device configuration is expected to use a label per
driver whose values name the mode, starting with something like `device-plugin` and `dra`. A fan-out
resolver produces one offering per mode, and rather than copying the base offering's resources, each
advertises the resources that mode exposes. A multi-valued requirement on the instance type can't
express this, because the modes disagree about capacity and capacity only varies at the offering
layer.

#### 1.3.3 NodeClass configuration

*Examples: `blockDeviceMappings`, `networkInterfaces`, `cpuOptions`, `connectionTracking`.*

Every configuration option is a dimension along which nodes can vary. Some have low cardinality
(tenancy has two values). Others are effectively infinite (block device mappings are an arbitrary
list of devices, sizes, volume types, and encryption settings). A high-cardinality dimension can't
viably be enumerated as offerings or as label values.

NodeClass configuration collapses the dimension instead: for a given `EC2NodeClass`, the option has
exactly one value, and every instance type returned for that NodeClass is locked to it. Multiple
configurations in one cluster are expressed as multiple NodeClasses, and workloads select between
them through a NodePool or a NodeClass label.

If the NodeClass configuration makes some instance types unusable, express that with a compatibility
check. The check feeds into offering availability rather than removing the instance type:

```go
// pkg/providers/instancetype/compatibility/compatibility.go (illustrative)
func (c nestedVirtualizationCheck) compatibleCheck(info ec2types.InstanceTypeInfo) bool {
    if c.cpuOptions == nil || lo.FromPtr(c.cpuOptions.NestedVirtualization) != "enabled" {
        return true
    }
    return info.ProcessorInfo != nil && lo.Contains(
        info.ProcessorInfo.SupportedFeatures,
        ec2types.SupportedAdditionalProcessorFeatureNestedVirtualization,
    )
}
```

A capability label can sit alongside NodeClass configuration. For example,
`karpenter.k8s.aws/instance-hypervisor` lets workloads select nitro instance types, while
`connectionTracking`, which requires a nitro hypervisor, is configured on the NodeClass. Keep the
distinction clear. A capability label is a **fact** about an instance type: it has one value per
type, is derived from `DescribeInstanceTypes`, and selects instance types rather than launch
configurations. A label with several values on one instance type is a **choice**, and belongs in
§1.3.1 or §1.3.2.

#### 1.3.4 Combined: NodeClass constrains, labels select

No existing feature uses the fully combined pattern, but it's the recommended way to add
administrator control to an already label-driven option without a breaking change. A hypothetical
version for tenancy:

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
spec:
  # default | dedicated | dynamic; nil is equivalent to dynamic
  instanceTenancy: dedicated
```

| `spec.instanceTenancy` | Advertised values for `karpenter.k8s.aws/instance-tenancy` | Effect |
|---|---|---|
| `nil` (default) | `[default, dedicated]` | Today's behavior, unchanged |
| `dynamic` | `[default, dedicated]` | Explicitly opts into per-pod selection |
| `default` | `[default]` | Pods requesting `dedicated` become unschedulable on this NodeClass |
| `dedicated` | `[dedicated]` | All nodes from this NodeClass are dedicated |

The NodeClass narrows the values the instance type advertises, and pods select from what remains.
Narrowing the advertised set makes conflicts surface in the right place: a pod requiring `dedicated`
against a NodeClass that advertises only `default` finds no compatible instance type and **stays
unschedulable**, rather than producing a NodeClaim that fails to launch.

Because a nil value preserves the existing advertised set, adding the field is non-breaking. The
order matters: **adding a NodeClass field first and labels later is a compatible evolution; adding
labels first and narrowing them later is not.**

#### 1.3.5 Discouraged: signalling through pod resource requests

An older pattern encodes dynamic configuration as an extended resource: every offering advertises
the resource in memory, pods request it, and the provider infers the configuration at launch from
`nodeClaim.Spec.Resources`. Dynamic EFA works this way (`vpc.amazonaws.com/efa`).

**Not recommended for new features.** Avoiding a multiplied offering count is a real performance
benefit, but the pattern has a concrete gap: **it doesn't support static NodePools.** Only pod
requests can activate the feature, so a NodeClaim created without pods driving it has no way to
enable it. Dynamic EFA ran into exactly this problem.

A distinct offering closes the gap. The offering advertises the resource, so a pod can still select
it implicitly by requesting the resource. It also carries a label, so the feature can be selected
explicitly through a label selector or a `spec.requirements` entry on a static NodePool.

A planned refactor will add a post-processing layer that makes distinct offerings and these
"implied offerings" functionally identical, so new work should be modeled as distinct offerings.
That is also the only model that can express mutually exclusive resources (§1.3.2).

---

## 2. Evaluating a feature

### 2.1 Selecting a surface

- **Labels** fit when the configuration is **user-driven**, or when it needs per-pod dynamism rather
  than per-NodePool/NodeClass control. Users drive labels through node affinity. Cluster
  administrators *can* restrict affinity with a validating admission policy, but treat labels as
  user-facing configuration by default.
- **NodeClass configuration** fits when the configuration space is too high-cardinality to enumerate
  as label values, or when only the cluster administrator should control the option.
- **When in doubt, choose NodeClass configuration.** It minimizes the exposed configuration surface,
  it's self-documenting through the CRD (labels are only documented indirectly, through CEL
  validation on the NodePool and NodeClaim CRDs and through the website), and it leaves an upgrade
  path to labels open.
- **Use distinct offerings whenever the option is dependent on other launch parameters, or changes
  price, availability, or advertised resources.** Prefer them over encoding the same information
  implicitly, even when an implicit encoding would be cheaper.
- **Don't give one dimension two competing authorities.** If an option appears both on the NodeClass
  and as a label, the NodeClass must constrain and the label must select within that constraint
  (§1.3.4).

### 2.2 Invariants

These hold regardless of which surface you choose. A design that breaks one of them will either
silently fail to schedule or launch nodes that don't match what the scheduler simulated.

**Labels must round-trip.** Whatever `Create` resolves must come back as a NodeClaim label, so the
node is labeled with what was actually launched. A value that can't be resolved and reported
shouldn't be a label.

**The union rule.** The scheduler filters instance types *before* it looks at offerings: it discards
any instance type whose requirements don't intersect the pod's, and checks offerings only on the
survivors. An offering-level requirement key must therefore appear on the instance type with the
**union** of every value its offerings carry. An offering the instance type doesn't advertise is
unreachable.

**Absence is a value.** Every well-known label must be defined on the instance type's requirements,
using `DoesNotExist` when it has no values. The same applies to offerings: an on-demand offering
declares `DoesNotExist` for the capacity reservation keys so it stays compatible with pods that
require those labels to be absent. Omitting the key means "no constraint", which is a different
statement.

**Unavailable, not absent.** `GetInstanceTypes` always returns every instance type. When something
makes an instance type unusable, such as an insufficient-capacity signal, a zonal shift, or a
NodeClass the instance type can't satisfy, set `Available: false` on the affected offerings instead
of dropping the instance type. Dropping it degrades scheduling error messages and breaks
consolidation's view of the world.

**Launch candidates are re-derived from the NodeClaim.** The NodeClaim's requirements are the whole
contract between the scheduler and the cloud provider. The scheduler doesn't communicate which
offering it settled on during simulation, and `Create` must not depend on the scheduler's
implementation, such as which constraints it chose to write or how it narrowed them. Instead,
`Create` derives the compatible instance types and offerings from `nodeClaim.Spec.Requirements` and
builds its launch candidates from that set.

This has two consequences. First, the compatible set is usually larger than one offering, which is
useful: every compatible (instance type, zone) pair becomes a `CreateFleet` override, so several
offerings coexist in a single launch and EC2 chooses among them. Second, some dimensions can't
coexist in one launch request and must be collapsed to a single value, since a fleet request has one
tenancy and one capacity type. Which case applies is a **per-feature decision**, and the design must
state it: can compatible values coexist as alternatives in one launch, or must one be selected? If
one is selected, what happens when several values are compatible, or when none is constrained?

### 2.3 Evaluation criteria

Use these checks to review a design or implementation against this guidance. Each item describes
what a passing answer looks like. Items are **advisory**: a design may fail one and still be the
right call if it explains why. Flag the gap and the reasoning rather than blocking on the letter of
the rule.

#### Surface selection

- **S1: A surface is named and justified.** The design states whether the option is exposed as
  instance-type requirement values, distinct offerings, NodeClass configuration, or a combination,
  and why. *Fails if the choice is implicit.*
- **S2: Cardinality is addressed.** The design states how many values the dimension can take. A
  high-cardinality or open-ended dimension exposed as label values needs an explicit justification.
- **S3: Audience matches the surface.** Label-driven options are justified by per-pod or user-driven
  need. Options that must be administrator-controlled are on the NodeClass.
- **S4: Dependency is classified.** The design says whether the option is independent of the rest of
  the launch configuration or dependent on it. Dependent options are modeled as distinct offerings,
  not as instance-type requirement values.
- **S5: Divergent price, availability, or resources implies offerings.** If the option changes any
  of these, it is modeled as distinct offerings.
- **S6: Single authority per dimension.** If the option appears both on the NodeClass and as a
  label, the NodeClass constrains, the label selects within that constraint, and the precedence is
  documented.

#### Model correctness

- **M1: Union rule.** Every offering-level requirement key is advertised on the instance type with
  the union of all values its offerings carry. *Failure mode: instance types are filtered out before
  offerings are consulted, and the feature appears to do nothing.*
- **M2: Absence declared.** Keys with no values are declared `DoesNotExist` on the instance type and
  on offerings that don't carry them. *Failure mode: pods requiring the label to be absent match
  offerings they shouldn't, or vice versa.*
- **M3: Unavailable, not absent.** Instance types are never dropped from `GetInstanceTypes` to
  express unavailability; `Available: false` is used instead.
- **M4: Coexist or select, stated.** For each new key, the design says whether multiple compatible
  values can coexist as alternatives in a single launch request (like zones) or one must be selected
  (like tenancy). If one is selected, the behavior when several values are compatible or none is
  constrained is documented user-facing behavior, not an implementation accident.
- **M5: No dependence on scheduler internals.** `Create` derives its launch candidates from the
  NodeClaim's requirements, not from assumptions about which constraints the scheduler writes or how
  it narrowed them.
- **M6: Round-trip.** The launched value is set as a NodeClaim label, and `Create`, `Get`, and
  `List` agree on it.
- **M7: Infeasibility surfaces in scheduling.** A configuration that can't be launched leaves pods
  unschedulable, with no compatible instance type or offering. *Fails if a NodeClaim is created and
  then errors at launch.*

#### Lifecycle and cost

- **L1: Cache key completeness.** Any NodeClass field that affects offerings is part of the offering
  cache key.
- **L2: Drift decided.** The design states whether changing the option drifts existing nodes, and
  the NodeClass hash reflects that decision.
- **L3: Offering growth bounded.** If the change multiplies offering count, it applies only when
  configured, and the expected magnitude is stated.
- **L4: Compatibility.** New fields default to today's behavior. Narrowing an already-advertised set
  of label values is called out as a breaking change.

#### Discouraged patterns

- **D1: No new resource-request signalling.** New dynamic configuration is not inferred from
  extended resources in `nodeClaim.Spec.Resources` (§1.3.5) unless the design explains why offerings
  can't work. Exceptions may be permitted while the offering refactor is pending. *Failure mode: the
  feature can't be enabled on NodeClaims that pods don't drive, such as those from static NodePools.*

#### Documentation and tests

- **T1: Discoverability.** New NodeClass fields have godoc/CRD descriptions. New labels are added to
  the website's label reference and, if the value set is closed, to `WellKnownValuesForRequirements`.
- **T2: Negative coverage.** Tests cover the unavailable/incompatible path and the unconstrained
  launch-time default, not just the happy path.

### 2.4 Wiring checklist

The touchpoints a new option typically hits. Not every option needs all of them.

| Step | Where | Notes |
|---|---|---|
| 1. API surface | `pkg/apis/v1/ec2nodeclass.go` | NodeClass field, CEL validation, defaulting. New fields are hashed by default; `hash:"ignore"` opts out, so changing the field won't drift existing nodes. |
| 2. Label registration | `pkg/apis/v1/labels.go` | Add to `WellKnownLabels`. For a closed value set, also add to `karpv1.WellKnownValuesForRequirements` so NodePool/NodeClaim CEL validation rejects typos. |
| 3. Instance type requirements | `computeRequirements` in `pkg/providers/instancetype/types.go` | Advertise the union of all offering values, or `DoesNotExist`. |
| 4. Offerings | new `OfferingResolver` in `pkg/providers/instancetype/offering/`, registered on the provider | **Only needed for dependent options, which apply to a subset of offerings.** An independent option like instance tenancy never appears in offerings. Set `Requirements`, `Price`, `Available`, and any capacity/overhead override. Resolvers must be deterministic and cheap. |
| 5. Offering cache key | `newCacheKeyBuilder` in `base_resolver.go` | **Any NodeClass field that affects offerings must be in the cache key**, or stale offerings will be served across NodeClasses. |
| 6. Instance type compatibility | `pkg/providers/instancetype/compatibility/` | For NodeClass configuration that makes some instance types unusable. |
| 7. Launch resolution | `pkg/providers/instance/`, `pkg/providers/launchtemplate/` | The deterministic decision rule for unconstrained/multi-valued requirements, plus the EC2 API plumbing. |
| 8. Label round-trip | `pkg/cloudprovider/cloudprovider.go` | Set the resolved value on the NodeClaim so `List`/`Get`/registration agree with `Create`. |
| 9. Drift | NodeClass hash and/or `IsDrifted` | Decide explicitly whether changing this option should replace existing nodes. |
| 10. Docs | `website/content/en/preview/` | NodeClass field reference and/or the labels list, plus a task page if the feature needs a walkthrough. |

Every design should address two cost considerations:

- **Offering count is multiplicative.** Offerings are recomputed per NodeClass across every instance
  type, zone, and capacity type. An expansion that multiplies the offering count (like partition
  placement groups) should apply only when the feature is configured, and should be feature-gated
  while it's new.
- **Cache correctness beats cache hit rate.** A missing cache key component is a correctness bug; an
  extra one only costs recomputation.
