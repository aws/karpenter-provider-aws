# Bandwidth Weighting Support

This document proposes supporting EC2 bandwidth weighting configuration in Karpenter.

- [Bandwidth Weighting Support](#bandwidth-weighting-support)
  - [Overview](#overview)
  - [Customer Use Cases](#customer-use-cases)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
  - [EC2NodeClass API](#ec2nodeclass-api)
  - [Instance Type Discovery](#instance-type-discovery)
  - [Validation](#validation)
  - [Scheduling and Launch Behavior](#scheduling-and-launch-behavior)
  - [Labels](#labels)
  - [Drift](#drift)
  - [Release Notes and Compatibility](#release-notes-and-compatibility)
  - [Appendix](#appendix)

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
- R6gd nodes launch with `NetworkPerformanceOptions` omitted from the launch template (Karpenter detects the instance type does not support bandwidth weighting and excludes the field), so they boot with default bandwidth and receive **no** bandwidth-weighting label

This means Karpenter — not EC2 — is responsible for ensuring unsupported instance types never receive `NetworkPerformanceOptions`. The launch template hash differs between supported and unsupported instance types in the same NodePool, producing distinct launch templates per group. See [Validation](#validation) for the safety net when discovery is wrong.

### EBS-Heavy Analytics (ebs-1)

Analytics workloads using large EBS volumes (io2, gp3) for data processing could benefit from `ebs-1` to maximize EBS throughput at the expense of networking bandwidth.

## Goals

1. Allow users to configure bandwidth weighting (`vpc-1`, `ebs-1`, `default`) via EC2NodeClass
2. Conditionally apply `NetworkPerformanceOptions` in the launch template only for instance types that support it
3. Support mixed fleets where some instance types support bandwidth weighting and others don't
4. Label nodes with their bandwidth weighting configuration for scheduling visibility

## Non-Goals

1. **Pod-level bandwidth weighting requests** — bandwidth weighting is an infrastructure decision, not a workload decision. Pods should not request specific bandwidth weighting; they should express bandwidth requirements via existing mechanisms (e.g., `karpenter.k8s.aws/instance-network-bandwidth`).
2. **Filtering instance types based on bandwidth weighting support** — users run mixed fleets for capacity flexibility. Karpenter should not exclude unsupported instance types when `networkPerformanceOptions` is set.
3. **Dynamic bandwidth adjustment** — modifying bandwidth weighting on running instances is not supported by EC2.

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

**Migration impact:** "Field unset" and `bandwidthWeighting: default` are *not* equivalent on the wire. Existing EC2NodeClasses upgraded to a Karpenter version that supports this feature will continue to hash to their current launch template (since the field is absent). Setting `bandwidthWeighting: default` explicitly is supported but produces a new launch template version and triggers drift-driven replacement (see [Drift](#drift)) — operators who only want to opt into observability without touching nodes should leave the field unset.

This distinction is also why setting `bandwidthWeighting: default` is allowed in the enum (rather than forcing operators to "remove the field" to get default behavior): some operators want the explicit declaration in their NodeClass for auditability, and the explicit form gives EC2 a consistent value to validate.

## Instance Type Discovery

Karpenter needs to know which instance types support bandwidth weighting to conditionally include `NetworkPerformanceOptions` in the launch template.

### Option A: DescribeInstanceTypes (Preferred)

The `DescribeInstanceTypes` API returns `NetworkInfo` per instance type. If the API exposes bandwidth weighting capability (e.g., `NetworkInfo.BandwidthWeightingSupport`), Karpenter can dynamically discover support.

**Implementation:** Add a `BandwidthWeightingSupported` field to Karpenter's instance type model, populated during the `DescribeInstanceTypes` call in the instance type provider.

### Option B: Static Instance Family List

If the API does not expose bandwidth weighting capability, maintain a static list of supported instance families (M8, C8, R8, X8) and check the instance type family prefix.

**Implementation:** Parse the instance type name (e.g., `r8gd.48xlarge` → family `r8gd` → generation `8` → supported).

### Recommendation

Option A is preferred for accuracy and forward-compatibility. Option B is a viable fallback if the API doesn't expose the capability.

### Cache and Refresh

Karpenter caches `DescribeInstanceTypes` results in the existing instance type provider. For bandwidth weighting:

- **TTL**: bandwidth-weighting support reuses the same cache TTL as other `NetworkInfo`-derived fields (currently 24h via `instancetype.DefaultTTL`). No new cache layer is introduced.
- **Cold start**: if the API is unreachable on controller startup, the static fallback list (Option B) is used until the next successful refresh. Discovery never blocks launches.
- **Resolution rule**: support is true if **either** the API confirms it OR the static list confirms it. This is intentionally permissive on the static side (we'd rather try and let validation reject than drop capacity for a known-supported family during a transient API outage), but conservative on the API side (a fresh API response that says "not supported" overrides a stale static `true`).
- **New families**: when AWS adds bandwidth weighting to a new family, the API picks it up on the next refresh (≤24h). The static list is updated by Karpenter releases, not at runtime.

### Discovery Misclassifications

Discovery can be wrong in two directions, and the design must handle both:

| Misclassification | Symptom | Mitigation |
|---|---|---|
| **False negative** (supported, but Karpenter thinks not) | Launch template omits `NetworkPerformanceOptions`; node boots with default bandwidth, receives no label | Acceptable. Operator updates static list or waits for cache refresh. No launch failure. |
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

The set is sourced from the EC2 SDK enum at build time, so upgrading the controller's SDK is the explicit step that introduces support for a new value. This produces a `NodeClassReady=False` condition with reason `UnsupportedBandwidthWeighting` so operators see *why* their NodeClass isn't launching nodes.

### 3. Pre-launch instance-type check

Before calling `RunInstances`, the launch path validates that the resolved instance type's `NetworkInfo.BandwidthWeightings` includes the requested value. If not, Karpenter omits `NetworkPerformanceOptions` from that specific launch (same code path as the discovery-based skip) and emits a `BandwidthWeightingUnsupportedForInstanceType` event on the NodeClaim. This is the safety net for a false-positive discovery result and for any future EC2-side hardening of validation.

The pre-launch check is structured so that label application (see [Labels](#labels)) is gated on the same predicate — a node only gets the `karpenter.k8s.aws/instance-bandwidth-weighting` label if the launched instance actually received the configured weighting.

## Scheduling and Launch Behavior

### Launch Template Generation

When `networkPerformanceOptions` is set on the EC2NodeClass:

1. For each instance type in the launch request, check if it supports bandwidth weighting
2. If supported, include `NetworkPerformanceOptions` in the `RequestLaunchTemplateData`
3. If not supported, omit `NetworkPerformanceOptions` from the launch template

Since Karpenter generates separate launch templates per unique configuration (hashed by launch template parameters), instance types with and without bandwidth weighting support will naturally get different launch templates.

```go
func networkPerformanceOptions(
    npo *v1.NetworkPerformanceOptions,
    instanceType *cloudprovider.InstanceType,
) *ec2types.LaunchTemplateNetworkPerformanceOptionsRequest {
    if npo == nil || npo.BandwidthWeighting == nil {
        return nil
    }
    if !instanceType.BandwidthWeightingSupported {
        return nil
    }
    return &ec2types.LaunchTemplateNetworkPerformanceOptionsRequest{
        BandwidthWeighting: ec2types.InstanceBandwidthWeighting(*npo.BandwidthWeighting),
    }
}
```

### Launch Template Hashing

The `NetworkPerformanceOptions` field must be included in the launch template name hash so that different bandwidth weighting configurations produce different launch templates. This ensures instance types with bandwidth weighting support get a launch template with `NetworkPerformanceOptions`, while unsupported types get a launch template without it.

### No Instance Type Filtering

Karpenter does **not** filter out instance types that don't support bandwidth weighting. All instance types allowed by the NodePool remain eligible. The bandwidth weighting is applied opportunistically — when the resolved instance type supports it, the launch template includes it; when it doesn't, the launch template omits it.

This preserves fleet flexibility for customers running mixed-generation instance types.

## Labels

When Karpenter launches an instance with bandwidth weighting configured, it applies the following label to the Node/NodeClaim:

| Label | Values | Description |
|-------|--------|-------------|
| `karpenter.k8s.aws/instance-bandwidth-weighting` | `default`, `vpc-1`, `ebs-1` | The bandwidth weighting **effective on the launched instance**. Set only after the launch path confirms (a) the resolved instance type's `NetworkInfo.BandwidthWeightings` includes the configured value, and (b) `NetworkPerformanceOptions` was included in `RunInstances`. |

### Effective vs. Intended Configuration

The label reflects what the instance actually has, not what the EC2NodeClass requested. This matters for the mixed-fleet case:

- An R8gd in a `bandwidthWeighting: vpc-1` NodePool gets `instance-bandwidth-weighting=vpc-1`.
- An R6gd in the **same** NodePool — where the launch template omitted `NetworkPerformanceOptions` because R6gd doesn't support it — gets **no label**, not `instance-bandwidth-weighting=default`.
- If a future discovery misclassification or EC2 API change causes a launch to silently drop the parameter, the absent label correctly signals "this node does not have the requested weighting" rather than asserting it.

This makes the label safe to use in both monitoring (count nodes with each weighting) and pod affinity (select only nodes that actually have the requested config) without operators having to cross-reference the EC2NodeClass spec.

This label enables:

1. **Observability** — operators can see which nodes actually have bandwidth weighting applied (vs. just configured to)
2. **Pod affinity** — workloads that benefit from boosted networking can express affinity for `vpc-1` nodes
3. **Drift detection** — Karpenter can detect when a node's effective bandwidth weighting doesn't match the EC2NodeClass

Example:

```yaml
metadata:
  labels:
    karpenter.k8s.aws/instance-bandwidth-weighting: "vpc-1"
```

Pods can optionally express preference for bandwidth-weighted nodes:

```yaml
spec:
  affinity:
    nodeAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
        - weight: 50
          preference:
            matchExpressions:
              - key: karpenter.k8s.aws/instance-bandwidth-weighting
                operator: In
                values: ["vpc-1"]
```

Note: This is a `preferred` affinity, not `required` — the pod can still schedule on nodes without bandwidth weighting (e.g., R6gd) if no `vpc-1` nodes are available.

## Drift

`bandwidthWeighting` is a launch-time-only EC2 field — `ModifyInstanceNetworkPerformanceOptions` requires the instance to be in `Stopped` state, and Karpenter does not stop/start nodes. **Drift on this field is always resolved by node replacement; the controller never attempts an in-place modify.** This is enforced in code: the drift handler for `BandwidthWeighting` returns a replacement directive without an in-place path.

### Drift Scenarios

| Scenario | Detection | Recovery |
|----------|-----------|----------|
| `networkPerformanceOptions` added to EC2NodeClass | Existing supported-type nodes lack the bandwidth-weighting label | Replace with the configured weighting |
| `networkPerformanceOptions` removed | Existing nodes have the bandwidth-weighting label | Replace without `NetworkPerformanceOptions` |
| `bandwidthWeighting` value changed (e.g., `vpc-1` → `ebs-1`) | Label value differs from spec | Replace with the new weighting |
| Instance type does not support bandwidth weighting | No label set | No drift, no action |
| Discovery flips a previously-unsupported type to supported | Existing node of that type lacks the label | Replace on next reconcile so the new node picks up the weighting |

### Distinct Drift Reason

Bandwidth weighting drift surfaces as its own reason in NodeClaim events and the `Drifted` status condition message:

- Reason: `BandwidthWeightingDrift`
- Event message: `BandwidthWeighting drift detected: node has %q, EC2NodeClass spec has %q`

This is intentionally separate from existing `NodeClassDrift` / `RequirementsDrift` / `AMIDrift` reasons so operators investigating "why was my node replaced?" can grep for the specific cause. The same drift reason is also used when discovery flips support status for an instance type, with the message indicating that instead.

### Interaction with the AWS API Constraint

Per the [EC2 API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ModifyInstanceNetworkPerformanceOptions.html), `BandwidthWeighting` cannot be changed on a running instance. The Karpenter NodeClass controller does not call `ModifyInstanceNetworkPerformanceOptions` from any code path — replacement is the only mechanism. If a future enhancement wants to support stop/modify/start for cost reasons, that would be a separate proposal; this design intentionally leaves it out.

## Release Notes and Compatibility

The release introducing this feature must call out the following in the changelog:

1. **No-op for existing EC2NodeClasses.** EC2NodeClasses that do not set `networkPerformanceOptions` continue to produce the same launch template hash and do not trigger drift. Operators can upgrade without node churn.
2. **Opt-in is launch-time-only.** Setting `bandwidthWeighting` triggers replacement of existing nodes covered by that NodeClass — there is no in-place change. Operators staging this change in production should expect a rolling replacement, not a hot reconfigure.
3. **Reliance on EC2 silent-ignore for unsupported types.** This design depends on EC2 silently accepting `NetworkPerformanceOptions` on instance types that don't support it as a defense-in-depth fallback (the primary mechanism is Karpenter's discovery-based omission). If AWS hardens this validation in a future API change, false-positive discovery results would surface as launch failures. The pre-launch validation step (see [Validation](#validation)) makes this unlikely in practice.
4. **No new IAM permissions required.** `NetworkPerformanceOptions` is a parameter on existing `RunInstances` / `CreateLaunchTemplate` calls, both already permitted by the standard Karpenter IAM policy.
5. **Static fallback list is release-pinned.** New 9th-gen+ families that support bandwidth weighting are picked up automatically via `DescribeInstanceTypes`. The static fallback list is updated by Karpenter releases for environments that can't reach `DescribeInstanceTypes` at startup.

## Appendix

### Supported Instance Types

Bandwidth weighting is available on 8th-gen instance families:

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
