# Bandwidth Weighting Support

This document proposes supporting EC2 bandwidth weighting configuration in Karpenter.

- [Bandwidth Weighting Support](#bandwidth-weighting-support)
  - [Overview](#overview)
    - [Why Karpenter Needs This](#why-karpenter-needs-this)
  - [Customer Use Cases](#customer-use-cases)
    - [Spark on EKS with NVMe Shuffle (vpc-1)](#spark-on-eks-with-nvme-shuffle-vpc-1)
    - [Implementation scope vs. this RFC](#implementation-scope-vs-this-rfc)
    - [EBS-Heavy Analytics (ebs-1)](#ebs-heavy-analytics-ebs-1)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
  - [EC2NodeClass API](#ec2nodeclass-api)
    - [Value Semantics](#value-semantics)
  - [Instance Type Discovery](#instance-type-discovery)
    - [Source of truth: `NetworkInfo.BandwidthWeightings`](#source-of-truth-networkinfobandwidthweightings)
    - [Cache and Refresh](#cache-and-refresh)
    - [Discovery Misclassifications](#discovery-misclassifications)
  - [Validation](#validation)
    - [1. Admission (CRD schema)](#1-admission-crd-schema)
    - [2. NodeClass reconciliation (runtime guard)](#2-nodeclass-reconciliation-runtime-guard)
    - [3. Launch-template resolution check](#3-launch-template-resolution-check)
  - [Scheduling and Launch Behavior](#scheduling-and-launch-behavior)
    - [Launch Template Generation](#launch-template-generation)
    - [Launch Template Hashing](#launch-template-hashing)
    - [No Instance Type Filtering](#no-instance-type-filtering)
  - [Labels](#labels)
    - [What the label does and does not assert](#what-the-label-does-and-does-not-assert)
  - [Drift](#drift)
    - [Drift is reported as `NodeClassDrift`](#drift-is-reported-as-nodeclassdrift)
    - [Drift Scenarios](#drift-scenarios)
    - [Drift Reason](#drift-reason)
    - [Interaction with the AWS API Constraint](#interaction-with-the-aws-api-constraint)
  - [Release Notes and Compatibility](#release-notes-and-compatibility)
  - [Appendix](#appendix)
    - [Supported Instance Types](#supported-instance-types)
    - [Bandwidth Impact Examples](#bandwidth-impact-examples)
    - [EC2 API Reference](#ec2-api-reference)

## Overview

EC2 [bandwidth weighting](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configure-bandwidth-weighting.html) allows 8th-gen instance types (M8, C8, R8, X8 families) to shift baseline bandwidth between networking and EBS. Three options are available:

- **default** — standard bandwidth configuration for the instance type
- **vpc-1** — increases networking baseline bandwidth, decreases EBS baseline bandwidth
- **ebs-1** — increases EBS baseline bandwidth, decreases networking baseline bandwidth

The combined bandwidth between networking and EBS does not change — bandwidth weighting redistributes the existing allocation. For example, an R8gd.48xlarge with `default` has 50 Gbps networking / 40 Gbps EBS. With `vpc-1`, this shifts to ~62.5 Gbps networking / ~27.5 Gbps EBS.

Bandwidth weighting can only be set at launch time via the `NetworkPerformanceOptions` parameter in the launch template, or modified on a **stopped** instance. It cannot be modified on a running instance. There is no additional cost.

### Why Karpenter Needs This

Currently there is no way to configure bandwidth weighting in Karpenter:

- EC2NodeClass does not expose `networkPerformanceOptions`
- Custom launch templates were removed in v0.33+
- The `modify-instance-network-performance-options` API requires the instance to be in `Stopped` state, so userData-based modification is not possible (validated: returns `InvalidState: not in an allowed state: stopped`)

## Customer Use Cases

### Spark on EKS with NVMe Shuffle (vpc-1)

EMR on EKS workloads running Spark with local NVMe for shuffle data use R8gd instances. These workloads have no EBS dependency but are network-bandwidth-constrained — S3 reads/writes plus shuffle traffic compete for the 50 Gbps default networking baseline. With `vpc-1`, the networking baseline increases to ~62.5 Gbps, providing ~25% more headroom for S3 and shuffle traffic at no cost.

The customer runs a heterogeneous fleet (R8gd.48xl + R6gd.16xl) in the same NodePool for capacity flexibility. R6gd does not support bandwidth weighting. The solution must apply `vpc-1` to R8gd nodes while gracefully handling R6gd nodes that don't support it.

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: spark-workers
spec:
  networkPerformanceOptions:
    bandwidthWeighting: vpc-1
  instanceStorePolicy: RAID0
  amiSelectorTerms:
    - alias: al2023@latest
  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: "my-cluster"
  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: "my-cluster"
  role: "KarpenterNodeRole-my-cluster"
---
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: spark-workers
spec:
  template:
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: spark-workers
      requirements:
        - key: node.kubernetes.io/instance-type
          operator: In
          values: ["r8gd.48xlarge", "r8gd.24xlarge", "r6gd.16xlarge", "r6gd.12xlarge"]
        - key: karpenter.sh/capacity-type
          operator: In
          values: ["on-demand"]
```

In this configuration:
- R8gd nodes launch with `vpc-1` — increased networking bandwidth, and receive the `karpenter.k8s.aws/instance-bandwidth-weighting=vpc-1` label
- R6gd nodes launch with `NetworkPerformanceOptions` omitted from the launch template (Karpenter detects the instance type does not accept the requested value and excludes the field), so they boot with default bandwidth and receive **no** bandwidth-weighting label

This means Karpenter — not EC2 — is responsible for ensuring unsupported instance types never receive `NetworkPerformanceOptions`. The launch template hash differs between supported and unsupported instance types in the same NodePool, producing distinct launch templates per group. See [Validation](#validation) for the safety net when discovery is wrong.

### Implementation scope vs. this RFC

The prototype in #9089 wires the NodeClass field straight into every launch template. It does **not** yet implement capability discovery, the launch-template grouping that splits supported from unsupported instance types, or the applied-weighting label. The mixed-fleet behavior described above is therefore a **requirement this design places on the implementation**, not current behavior — it is follow-up work gated on this RFC landing.

### EBS-Heavy Analytics (ebs-1)

Analytics workloads using large EBS volumes (io2, gp3) for data processing could benefit from `ebs-1` to maximize EBS throughput at the expense of networking bandwidth.

## Goals

1. Allow users to configure bandwidth weighting (`vpc-1`, `ebs-1`, `default`) via EC2NodeClass
2. Conditionally apply `NetworkPerformanceOptions` in the launch template only for instance types that support it
3. Support mixed fleets where some instance types support bandwidth weighting and others don't
4. Label nodes with the bandwidth weighting Karpenter applied, for observability

## Non-Goals

1. **Pod-level bandwidth weighting requests** — bandwidth weighting is an infrastructure decision, not a workload decision. Pods should not request specific bandwidth weighting; they should express bandwidth requirements via existing mechanisms (e.g., `karpenter.k8s.aws/instance-network-bandwidth`).
2. **Filtering instance types based on bandwidth weighting support** — users run mixed fleets for capacity flexibility. Karpenter should not exclude unsupported instance types when `networkPerformanceOptions` is set.
3. **Dynamic bandwidth adjustment** — modifying bandwidth weighting on running instances is not supported by EC2.
4. **Bandwidth weighting as a scheduling constraint** — the applied-weighting label is observability-only and cannot steer provisioning; see [What the label does and does not assert](#what-the-label-does-and-does-not-assert).
5. **A dedicated drift reason** — `bandwidthWeighting` changes report as `NodeClassDrift`; see [Drift](#drift).

## EC2NodeClass API

Add `networkPerformanceOptions` to `EC2NodeClassSpec`:

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: example
spec:
  networkPerformanceOptions:
    bandwidthWeighting: vpc-1  # "default", "vpc-1", or "ebs-1"
```

```go
type EC2NodeClassSpec struct {
    // ...existing fields...

    // NetworkPerformanceOptions configure the network performance options for
    // instances launched with this EC2NodeClass. Allows configuring bandwidth
    // weighting between networking and EBS for supported instance types.
    // When set, Karpenter conditionally includes NetworkPerformanceOptions in
    // the launch template for instance types that support it.
    // +optional
    NetworkPerformanceOptions *NetworkPerformanceOptions `json:"networkPerformanceOptions,omitempty"`
}

type NetworkPerformanceOptions struct {
    // BandwidthWeighting configures bandwidth weighting for the instance.
    // vpc-1 increases networking baseline bandwidth and decreases EBS baseline bandwidth.
    // ebs-1 increases EBS baseline bandwidth and decreases networking baseline bandwidth.
    // +kubebuilder:validation:Enum:={"default","vpc-1","ebs-1"}
    // +optional
    BandwidthWeighting *string `json:"bandwidthWeighting,omitempty"`
}
```

### Value Semantics

The `bandwidthWeighting` field has three meaningful states, and Karpenter must distinguish all three to avoid silently changing behavior on existing fleets:

| Spec state | Behavior on launch | Launch template `NetworkPerformanceOptions` |
|---|---|---|
| Field unset (or `networkPerformanceOptions: {}`) | Karpenter omits `NetworkPerformanceOptions` entirely | absent |
| `bandwidthWeighting: default` | Karpenter sends `BandwidthWeighting=default` explicitly | present, value `default` |
| `bandwidthWeighting: vpc-1` / `ebs-1` | Karpenter sends the requested value | present, value `vpc-1` / `ebs-1` |

**Migration impact:** "Field unset" and `bandwidthWeighting: default` are *not* equivalent on the wire. Existing EC2NodeClasses upgraded to a Karpenter version that supports this feature will continue to hash to their current launch template (since the field is absent). Setting `bandwidthWeighting: default` explicitly is supported but produces a new launch template and triggers drift-driven replacement (see [Drift](#drift)) — operators who only want to opt into observability without touching nodes should leave the field unset.

This distinction is also why setting `bandwidthWeighting: default` is allowed in the enum (rather than forcing operators to "remove the field" to get default behavior): some operators want the explicit declaration in their NodeClass for auditability, and the explicit form gives EC2 a consistent value to validate.

## Instance Type Discovery

Karpenter needs to know which instance types support bandwidth weighting to conditionally include `NetworkPerformanceOptions` in the launch template.

### Source of truth: `NetworkInfo.BandwidthWeightings`

`DescribeInstanceTypes` returns `NetworkInfo.BandwidthWeightings` per instance type: a **list of the weighting values that type accepts**, not a boolean capability flag.

The design keeps that list shape rather than reducing it to a `BandwidthWeightingSupported` bool. The distinction matters — the check Karpenter needs is "does this type accept *the value the operator asked for*", which is a membership test against the list. A boolean cannot express a type that accepts `vpc-1` but not `ebs-1`, and it is not sufficient to implement the membership check used in [Validation](#validation).

**Implementation:** add the allowed-value set to Karpenter's instance type model, populated from `NetworkInfo.BandwidthWeightings` during the existing `DescribeInstanceTypes` call in the instance type provider. All capability decisions in this design are `slices.Contains(it.BandwidthWeightings, requested)`.

We deliberately do **not** maintain a static list of supported instance families. Any such list goes stale the moment AWS adds a family, and it invites future readers to treat the doc as authoritative over the API — the same reasoning `designs/cpu-options-nested-virtualization.md` applies to `ProcessorInfo.SupportedFeatures`. To see the current set in a region:

```bash
aws ec2 describe-instance-types \
  --query 'InstanceTypes[?NetworkInfo.BandwidthWeightings!=`null`].[InstanceType,NetworkInfo.BandwidthWeightings]'
```

### Cache and Refresh

Bandwidth weighting support is carried on the existing instance type objects and inherits their refresh behavior. No new cache layer is introduced.

- **Freshness** is bounded by two existing mechanisms: `cache.InstanceTypesZonesAndOfferingsTTL` (**5 minutes**, `pkg/cache/cache.go`) and the instance type refresh controller, which requeues every **12 hours** (`pkg/controllers/providers/instancetype/controller.go`). Note `cache.DefaultTTL` is 1 minute and serves a different purpose; it does not govern this data.
- **New families** are therefore picked up automatically within one refresh cycle, with no Karpenter release required.
- **Cold start:** the operator hydrates the instance type and offering caches from `DescribeInstanceTypes` **before** dependent controllers start. If that call fails at startup, Karpenter cannot resolve instance types at all — bandwidth weighting is not the limiting factor, and no bandwidth-weighting-specific fallback would keep launches working. This design therefore adds **no** cold-start fallback: capability data has exactly the same availability as the `InstanceTypeInfo` that scheduling already depends on.

### Discovery Misclassifications

Discovery can be wrong in two directions, and the design must handle both:

| Misclassification | Symptom | Mitigation |
|---|---|---|
| **False negative** (supported, but Karpenter thinks not) | Launch template omits `NetworkPerformanceOptions`; node boots with default bandwidth, receives no label | Acceptable. Resolves on the next instance type refresh. No launch failure. |
| **False positive** (unsupported, but Karpenter thinks yes) | Launch template includes `NetworkPerformanceOptions`; EC2 may silently ignore (current behavior) or may reject in a future API change | See [Validation](#validation) for the runtime guard. |

The design relies on EC2's documented behavior of silently accepting `NetworkPerformanceOptions` on instance types that don't support it. If AWS changes this to a hard validation error in the future, false-positive launches would fail with `InvalidParameterValue`. The mitigation is a defense-in-depth pre-launch check (see Validation) plus the [Release Notes](#release-notes-and-compatibility) section calling out the dependency.

## Validation

Validation happens at three layers, each catching a different failure mode:

### 1. Admission (CRD schema)

`+kubebuilder:validation:Enum:={"default","vpc-1","ebs-1"}` rejects unknown values at the apiserver. This is the first line of defense and catches typos.

### 2. NodeClass reconciliation (runtime guard)

Even with the CRD enum, future Karpenter releases may ship a CRD with new values (e.g., `ebs-2`) while running an older controller binary. The reconciler **must** validate `bandwidthWeighting` against a controller-local set of known values and reject unknown values with a clear status condition rather than passing the value through to `RunInstances`:

```go
var supportedBandwidthWeightings = sets.New(
    string(ec2types.InstanceBandwidthWeightingDefault),
    string(ec2types.InstanceBandwidthWeightingVpc1),
    string(ec2types.InstanceBandwidthWeightingEbs1),
)

if npo := nodeClass.Spec.NetworkPerformanceOptions; npo != nil && npo.BandwidthWeighting != nil {
    if !supportedBandwidthWeightings.Has(*npo.BandwidthWeighting) {
        return reconcile.Result{}, fmt.Errorf(
            "unsupported bandwidthWeighting %q (controller knows: %v)",
            *npo.BandwidthWeighting, supportedBandwidthWeightings.UnsortedList())
    }
}
```

The set is sourced from the EC2 SDK enum at build time, so upgrading the controller's SDK is the explicit step that introduces support for a new value.

This surfaces as **`ValidationSucceeded=False`** with reason `UnsupportedBandwidthWeighting`, which is the condition the NodeClass validation controller already owns (`v1.ConditionTypeValidationSucceeded`, set in `pkg/controllers/nodeclass/validation.go`). There is no `NodeClassReady` condition on EC2NodeClass, so reporting one here would describe something operators would never see.

### 3. Launch-template resolution check

The capability check runs during **launch template resolution**, in `amifamily.Resolver` — not immediately before an instance API call. This placement is forced by how Karpenter actually launches nodes:

- The provisioning path is **`CreateFleet`** with pre-created launch template configs (`pkg/providers/instance/instance.go`). `NetworkPerformanceOptions` is baked into the launch template at `CreateLaunchTemplate` time, so by the time Fleet runs, the value is already fixed.
- The `RunInstances` call in this repository is a **dry-run authorization probe** in the NodeClass validation controller, not the launch path. A check described as "before `RunInstances`" would not guard real launches.

So the resolver evaluates `slices.Contains(it.BandwidthWeightings, requested)` per instance type while grouping types into launch templates (see [Launch Template Generation](#launch-template-generation)). Types that do not accept the requested value are grouped into a launch template with `NetworkPerformanceOptions` omitted.

This is the safety net for a false-positive discovery result and for any future EC2-side hardening of validation. Because grouping happens before template creation, the decision is knowable per template rather than per selected override — which is also what makes the label in [Labels](#labels) derivable.

## Scheduling and Launch Behavior

### Launch Template Generation

When `networkPerformanceOptions` is set on the EC2NodeClass:

`NetworkPerformanceOptions` is a property of the **whole launch template**, not of a per-instance-type Fleet override. A single resolved launch template carries one `InstanceTypes` slice into `CreateFleet`, so the field cannot vary across the types sharing that template.

Supported and unsupported types therefore do **not** separate on their own. `amifamily.LaunchTemplate.InstanceTypes` is tagged `hash:"ignore"`, so two templates differing only in their instance types hash to the same `LaunchTemplateName` — the very thing that would have to differ. Splitting must be explicit.

**Karpenter already has the mechanism.** `amifamily.Resolver` groups instance types with `lo.GroupBy` over a `launchTemplateParams` key, and `efaCount` is in that key for exactly this reason: "instance types configured with EFAs require unique launch templates depending on the number of EFAs they support." Bandwidth weighting is the same shape of problem, so it takes the same solution — add the **effective** weighting to the grouping key:

```go
type launchTemplateParams struct {
    efaCount int
    maxPods  int
    // bandwidthWeighting is the weighting this instance type will actually get:
    // the requested value if the type accepts it, otherwise "" meaning the
    // launch template omits NetworkPerformanceOptions entirely. Including it
    // here splits supported and unsupported types into separate templates.
    bandwidthWeighting string
    reservationIDs           string
    reservationType          v1.CapacityReservationType
    reservationInterruptible bool
}
```

```go
// effectiveBandwidthWeighting returns the weighting the given instance type will
// receive, or "" if NetworkPerformanceOptions should be omitted for it.
func effectiveBandwidthWeighting(
    npo *v1.NetworkPerformanceOptions,
    it *cloudprovider.InstanceType,
) string {
    if npo == nil || npo.BandwidthWeighting == nil {
        return ""
    }
    // Membership test against the API-reported allowed values, not a bool.
    if !slices.Contains(it.BandwidthWeightings, *npo.BandwidthWeighting) {
        return ""
    }
    return *npo.BandwidthWeighting
}
```

Each group then resolves to its own launch template, and `NetworkPerformanceOptions` is set from `params.bandwidthWeighting` (omitted when empty). A mixed R8gd + R6gd NodePool produces two launch templates, and Fleet is free to pick either.

### Launch Template Hashing

`LaunchTemplateName` is `hashstructure.Hash` over the whole `amifamily.LaunchTemplate` (`pkg/providers/launchtemplate/launchtemplate.go`), so a new exported `NetworkPerformanceOptions` field participates in the hash and distinct weightings produce distinct templates. Because `bandwidthWeighting` is in the grouping key above, the two groups differ in a hashed field and cannot collide.

The hash options are `FormatV2` with `SlicesAsSets: true` and **no `IgnoreZeroValue`** — which has a backwards-compatibility consequence spelled out in [Release Notes and Compatibility](#release-notes-and-compatibility).

### No Instance Type Filtering

Karpenter does **not** filter out instance types that don't support bandwidth weighting. All instance types allowed by the NodePool remain eligible. The bandwidth weighting is applied opportunistically — when the resolved instance type supports it, the launch template includes it; when it doesn't, the launch template omits it.

This preserves fleet flexibility for customers running mixed-generation instance types.

## Labels

When Karpenter launches an instance with bandwidth weighting configured, it applies the following label to the Node/NodeClaim:

| Label | Values | Description |
|-------|--------|-------------|
| `karpenter.k8s.aws/instance-bandwidth-weighting` | `default`, `vpc-1`, `ebs-1` | The weighting **Karpenter applied in the launch template it used for this node** — i.e. `params.bandwidthWeighting` for the group the node's instance type resolved into. Absent when the launch template omitted `NetworkPerformanceOptions`. |

### What the label does and does not assert

The label states **what Karpenter configured**, not what EC2 independently confirmed. Being precise about this is necessary because Karpenter has no durable source for the latter: the launch result is converted from `CreateFleet`/`DescribeInstances` data into an `Instance` that retains neither the selected launch template's configuration nor any EC2-reported effective weighting. A silently-ignored request is therefore **indistinguishable** from an applied one at that layer, and a label claiming "effective on the instance" would be unfalsifiable.

Scoping it to the applied launch configuration keeps it both truthful and useful, and it is still exactly the signal the mixed-fleet case needs:

- An R8gd in a `bandwidthWeighting: vpc-1` NodePool resolves into the `vpc-1` group and gets `instance-bandwidth-weighting=vpc-1`.
- An R6gd in the **same** NodePool resolves into the omitted group and gets **no label** — not `instance-bandwidth-weighting=default`. Absence means "Karpenter did not request a weighting for this node."
- A discovery false-negative shows up as a missing label, which is the correct and conservative signal.

If EC2 later exposes the effective weighting on `DescribeInstances`, that value could be reconciled against this label to detect silent drops. That is out of scope here.

**This label is for observability only. It is not a scheduling input.** A pod affinity on it cannot steer provisioning toward the supported group, because the value is only known after Karpenter has already resolved the instance type and launch template. `designs/efa-for-static-capacity.md` records the same constraint for `karpenter.k8s.aws/instance-efa-count`: *"Karpenter currently does not support scheduling with dynamic label applications"*, and supporting it "would require significant changes to core Karpenter's scheduling simulation."

Advertising it as an affinity target would therefore be actively misleading — a `required` affinity would not do what operators expect, and a `preferred` one would silently do nothing during provisioning. Workloads that need the boosted baseline should constrain **instance types** in the NodePool instead:

```yaml
# Steer to bandwidth-weighting-capable capacity by constraining instance types,
# not by affinity on the bandwidth-weighting label.
requirements:
  - key: karpenter.k8s.aws/instance-generation
    operator: Gt
    values: ["7"]
```

Example of the label as it appears on a node:

```yaml
metadata:
  labels:
    karpenter.k8s.aws/instance-bandwidth-weighting: "vpc-1"
```

## Drift

`bandwidthWeighting` is a launch-time-only EC2 field — `ModifyInstanceNetworkPerformanceOptions` requires the instance to be in `Stopped` state, and Karpenter does not stop/start nodes. **Drift on this field must always be resolved by node replacement; the controller must never attempt an in-place modify.** Karpenter calls `ModifyInstanceNetworkPerformanceOptions` from no code path today, and this design does not add one.

### Drift is reported as `NodeClassDrift`

A change to `bandwidthWeighting` surfaces as the existing **`NodeClassDrift`** reason. This is a consequence of how static drift already works and is worth stating plainly, because an earlier draft of this design proposed a dedicated `BandwidthWeightingDrift` reason that is not achievable without unrelated refactoring:

- `EC2NodeClass.Hash()` hashes `in.Spec` **wholesale** (`pkg/apis/v1/ec2nodeclass.go`), so any `bandwidthWeighting` change changes the NodeClass hash.
- `isNodeClassDrifted()` calls `areStaticFieldsDrifted()` **first and returns immediately** on a hash mismatch (`pkg/cloudprovider/drift.go`), before any field-specific check could run.

A distinct reason would therefore require excluding `networkPerformanceOptions` from the static hash and adding a field-specific comparison — plus an `AnnotationEC2NodeClassHashVersion` bump and its migration, which would force a one-time re-hash across every existing NodeClass. That cost is not justified by a nicer drift label, so this design accepts `NodeClassDrift`.

Operators lose no information: the drift is still detected, the node is still replaced, and the cause is visible by diffing the NodeClass. If a per-field drift taxonomy is wanted later, it should be proposed once for all static fields rather than special-cased here.

### Drift Scenarios

| Scenario | Detection | Recovery |
|----------|-----------|----------|
| `networkPerformanceOptions` added to EC2NodeClass | Existing supported-type nodes lack the bandwidth-weighting label | Replace with the configured weighting |
| `networkPerformanceOptions` removed | Existing nodes have the bandwidth-weighting label | Replace without `NetworkPerformanceOptions` |
| `bandwidthWeighting` value changed (e.g., `vpc-1` → `ebs-1`) | Label value differs from spec | Replace with the new weighting |
| Instance type does not accept the requested value | No label set | No drift, no action |
| Discovery flips a previously-unaccepted type to accepted | NodeClass hash unchanged, so **no drift is raised** | Not detected. The node keeps running without the weighting until it is replaced for another reason. Called out as a known limitation rather than claimed as handled — detecting it would need a capability-aware drift check, which the static-hash path above precludes. |

### Drift Reason

Bandwidth weighting drift surfaces through the existing static-drift path:

- Reason: `NodeClassDrift` (see above for why a dedicated reason is not proposed)
- The NodeClass hash change is what triggers replacement; no bandwidth-weighting-specific event is required.

### Interaction with the AWS API Constraint

Per the [EC2 API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ModifyInstanceNetworkPerformanceOptions.html), `BandwidthWeighting` cannot be changed on a running instance. The Karpenter NodeClass controller does not call `ModifyInstanceNetworkPerformanceOptions` from any code path — replacement is the only mechanism. If a future enhancement wants to support stop/modify/start for cost reasons, that would be a separate proposal; this design intentionally leaves it out.

## Release Notes and Compatibility

The release introducing this feature must call out the following in the changelog:

1. **Expect a one-time launch template roll on upgrade.** `LaunchTemplateName` hashes the whole `amifamily.LaunchTemplate` with **no `IgnoreZeroValue`**, so adding an exported `NetworkPerformanceOptions` field changes the computed name even for NodeClasses that never set it. Existing EC2NodeClasses will therefore resolve to **new launch templates** after upgrade. Two consequences the implementation must settle before merge:
   - Whether this also triggers node **replacement**, or only new template creation with existing nodes left alone. This depends on whether the NodeClass hash (`EC2NodeClass.Hash()`, which hashes `in.Spec`) changes for an unset field — a nil pointer added to the spec struct. **This must be verified with a test asserting the pre- and post-change hashes, not assumed.**
   - If it does cause churn, either tag the new field so it is excluded from the hash when nil, or ship it with an `AnnotationEC2NodeClassHashVersion` bump so the migration is explicit and one-time rather than a surprise rolling replacement.

   An earlier draft of this design claimed the upgrade was a hash-preserving no-op. That claim was wrong and is retracted here.
2. **Opt-in is launch-time-only.** Setting `bandwidthWeighting` triggers replacement of existing nodes covered by that NodeClass (as `NodeClassDrift`) — there is no in-place change. Operators staging this change in production should expect a rolling replacement, not a hot reconfigure.
3. **Reliance on EC2 silent-ignore for unsupported types.** This design depends on EC2 silently accepting `NetworkPerformanceOptions` on instance types that don't accept it as a defense-in-depth fallback; the primary mechanism is Karpenter's capability-based omission at launch template resolution. If AWS hardens this validation in a future API change, false-positive discovery results would surface as launch failures. The resolution-time check (see [Validation](#validation)) makes this unlikely in practice.
4. **No new IAM permissions required.** `NetworkPerformanceOptions` is a parameter on existing `RunInstances` / `CreateLaunchTemplate` calls, both already permitted by the standard Karpenter IAM policy.
5. **New instance families need no Karpenter release.** Support is read from `NetworkInfo.BandwidthWeightings` on each instance type refresh, so families AWS adds later are picked up automatically. There is no static family list to maintain.

## Appendix

### Supported Instance Types

Bandwidth weighting is available on 8th-gen instance families. **This table is illustrative only — `NetworkInfo.BandwidthWeightings` is the source of truth and Karpenter maintains no static list** (see [Source of truth](#source-of-truth-networkinfobandwidthweightings)):

| Category | Families |
|----------|---------|
| General Purpose | M8a, M8g, M8gd, M8i, M8id, M8i-flex |
| Compute Optimized | C8a, C8g, C8gd, C8i, C8id, C8i-flex |
| Memory Optimized | R8a, R8g, R8gd, R8i, R8id, R8i-flex, X8g, X8aedz, X8i |

### Bandwidth Impact Examples

**R8gd.48xlarge:**

| Config | Network Baseline | EBS Baseline |
|--------|-----------------|-------------|
| default | 50 Gbps | 40 Gbps |
| vpc-1 | ~62.5 Gbps | ~27.5 Gbps |
| ebs-1 | ~37.5 Gbps | ~52.5 Gbps |

### EC2 API Reference

- [Configure bandwidth weighting](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configure-bandwidth-weighting.html)
- [LaunchTemplateNetworkPerformanceOptionsRequest](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_LaunchTemplateNetworkPerformanceOptionsRequest.html)
- [ModifyInstanceNetworkPerformanceOptions](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ModifyInstanceNetworkPerformanceOptions.html) (requires stopped instance)
