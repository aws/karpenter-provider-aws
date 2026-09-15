# Dynamic Kubelet Configuration via Expressions

This document proposes supporting expression-based kubelet configuration in Karpenter to enable instance-type-aware values for maxPods, systemReserved, and kubeReserved.

## Overview

Karpenter's EC2NodeClass currently requires fixed literal values for kubelet configuration parameters such as maxPods, systemReserved, and kubeReserved. These values are configured in `spec.kubelet` and applied uniformly to all instances launched from that NodeClass — regardless of their size.

This is fundamentally at odds with Karpenter's core value proposition: provisioning heterogeneous instance fleets from a single NodePool. When a NodePool spans instance sizes from c6a.large (2 vCPUs, 2 ENIs) to c6a.48xlarge (192 vCPUs, 15 ENIs), the optimal kubelet parameters vary significantly per instance type.

### The Static Configuration Problem

Today, administrators face an impossible trade-off:

* **Accept a single suboptimal static value across all instance sizes** — A kubeReserved of `cpu: 65m` appropriate for a 4-vCPU instance starves system components on a 96-vCPU instance. A maxPods of 29 (appropriate for m5.large) wastes capacity on m5.24xlarge which could support hundreds of pods.
* **Fracture into per-instance-type NodePools** — Create separate NodePool/EC2NodeClass pairs per instance size range, dramatically increasing management complexity and undermining Karpenter's right-sizing benefits.

Customers override these fields because their workloads have specific resource requirements that Karpenter's defaults don't account for. Custom AMIs may run additional systemd services (monitoring agents, security daemons, log collectors) whose memory and CPU footprint scales with instance size — a node with 96 vCPUs runs more parallel system processes than a 4-vCPU node. Similarly, organizations using custom container runtimes or kernel configurations need system reserved values that reflect their actual overhead, not Karpenter's generic formula. Today, the only way to express "my system daemons need proportionally more resources on larger instances" is to create separate NodeClasses per size range.

Karpenter already computes instance-type-specific values internally for scheduling decisions (e.g., calculating pods from ENI limits, computing kubeReserved overhead). However, when a user explicitly sets `spec.kubelet.kubeReserved` or `spec.kubelet.maxPods`, that static value is what gets baked into the node's UserData — not the dynamically computed value.

**Why not compute the right answer automatically?**

For most users, Karpenter already does — its defaults (ENI-limited maxPods, graduated kubeReserved) work without configuration, and this feature doesn't change that. But Karpenter can't know the right answer when the overhead depends on what's invisible to it: custom AMI daemons, non-standard CNI configurations, or organization-specific system processes. That's why the override fields exist today. This feature doesn't add new configuration — it makes existing overrides work correctly across heterogeneous fleets instead of only for a single instance size.

### Community Request

This feature was requested by the Karpenter community in [#8742](https://github.com/aws/karpenter-provider-aws/issues/8742) (29 upvotes), which consolidated several related issues:

* [#8694](https://github.com/aws/karpenter-provider-aws/issues/8694) — Users requesting percentage-based resource reservations that scale with instance size, citing system component starvation when fixed reservations are used across heterogeneous fleets.
* [#8739](https://github.com/aws/karpenter-provider-aws/issues/8739) — Users attempting to use nodeadm's `maxPodsExpression` with Karpenter for Cilium IPAM, finding that Karpenter overwrites their expression with a static `maxPods` value. Workarounds include post-boot shell scripts patching kubelet config and custom Karpenter forks.
* [#8210](https://github.com/aws/karpenter-provider-aws/issues/8210) — Karpenter's maxPods calculation does not account for prefix delegation, causing pods to exhaust IP capacity before CPU/memory on smaller nodes. Setting maxPods manually per instance type contradicts Karpenter best practices.
* [#5478](https://github.com/aws/karpenter-provider-aws/issues/5478) — Request for Windows prefix delegation support to increase pod density and improve cost savings.
* [PR #9299](https://github.com/aws/karpenter-provider-aws/pull/9299) — A community-contributed implementation that adds a boolean `enablePrefixDelegation` field to EC2NodeClass. CEL expressions offer a more general solution that subsumes this PR's use case (e.g., `maxPods: "min(250, ((default_enis - 1) * (ips_per_eni - 1)) * 16 + 2)"`) without adding a field that's specific to one CNI configuration.

## Customer Use Cases

### Heterogeneous Fleet with ENI-Based maxPods

A platform team runs a single NodePool spanning m5.large through m5.24xlarge. They want maxPods to scale with ENI capacity so that smaller instances don't over-commit and larger instances don't waste capacity.

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: general-purpose
spec:
  kubelet:
    maxPods: "((default_enis - 1) * (ips_per_eni - 1)) + 2"
  amiSelectorTerms:
    - alias: al2023@latest
  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: "my-cluster"
  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: "my-cluster"
---
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: general-purpose
spec:
  template:
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: general-purpose
      requirements:
        - key: node.kubernetes.io/instance-type
          operator: In
          values: ["m5.large", "m5.xlarge", "m5.2xlarge", "m5.4xlarge", "m5.8xlarge", "m5.12xlarge", "m5.24xlarge"]
```

With this configuration:

* m5.large (3 ENIs, 10 IPs/ENI): maxPods = ((3 - 1) * (10 - 1)) + 2 = 20
* m5.24xlarge (15 ENIs, 50 IPs/ENI): maxPods = ((15 - 1) * (50 - 1)) + 2 = 688

### Scaled Resource Reservations Across Instance Sizes

An operations team needs kubeReserved CPU and memory to scale with instance size to prevent kubelet instability on larger nodes while avoiding over-reservation on smaller ones.

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: scaled-reservations
spec:
  kubelet:
    kubeReserved:
      cpu: "max(60, vcpus * 30)"
      memory: "11 * max_pods + 255"
    systemReserved:
      cpu: "max(20, vcpus * 10)"
      memory: "max(100, memory_mib / 64)"
  amiSelectorTerms:
    - alias: al2023@latest
  ...
```

With this configuration on a c6a.4xlarge (16 vCPUs, 30720 MiB):

* kubeReserved.cpu = max(60, 16 * 30) = 480 → `480m`
* systemReserved.memory = max(100, 30720 / 64) = 480 → `480Mi`

Expression results are bare numbers in the unit that key's static quantities are already written in — millicores for `cpu`, MiB for `memory` — so a formula reads like the value it replaces. 

### Prefix Delegation with Dynamic Pod Limits

A team using VPC CNI prefix delegation wants maxPods to account for the increased IP capacity from prefixes.

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: prefix-delegation
spec:
  kubelet:
    maxPods: "min(250, ((default_enis - 1) * (ips_per_eni - 1)) * 16 + 2)"
  ...
```

## Goals

* Allow users to configure maxPods, kubeReserved, and systemReserved as a CEL expression evaluated per instance type
* Ensure Karpenter evaluates expressions at scheduling time to maintain accurate capacity predictions for bin-packing
* Ensure computed values from expressions are passed as static values in UserData on every AMI family, so that the value the scheduler reserved is exactly the value the node is configured with
* Ship the feature behind an alpha feature gate, disabled by default
* Validate expressions on the EC2NodeClass — both compile-time (syntax/type) and per-instance-type evaluation — and surface failures on the NodeClass status

## Non-Goals

Below lists the non-goals for this RFC design. Each of these items represents potential follow-ups for the initial implementation and are features we will consider based on feature requests.

* Expression-based blockDeviceMappings (e.g., volume size scaling with instance size) — while this is a natural extension, it involves different subsystems and warrants a separate design
* Supporting arbitrary kubelet flags as expressions — only maxPods, kubeReserved, and systemReserved are in scope
* User-defined variables or custom functions in expressions — only built-in instance type properties are available

## Feature Gate

Expression support is alpha and gated off by default behind an AWS-specific feature gate, `NodeClassCEL`. AWS-specific gates are configured separately from the core Karpenter gates, via the `AWS_FEATURE_GATES` environment variable / `--aws-feature-gates` flag (Helm: `settings.awsFeatureGates.nodeClassCEL`), defaulting to `NodeClassCEL=false`.

The CRD schema cannot gate expressions itself, since it has no view of operator flags — a string-typed `maxPods` is schema-valid whether or not the gate is on. Instead, the nodeclass validation controller rejects a NodeClass carrying an expression while the gate is disabled, setting `ValidationSucceeded=False` with reason `KubeletExpressionsDisabled`. Failing loudly here matters: silently ignoring the expression would launch nodes with the AMI family defaults the user didn't ask for.

A NodeClass whose kubelet fields are all static literals is unaffected by the gate.

## Expression-Based Kubelet Configuration

### EC2NodeClass API

```yaml
spec:
  kubelet:
    # maxPods accepts either a static integer or a CEL expression string.
    # Static:
    maxPods: 110
    # OR Expression:
    maxPods: "((default_enis - 1) * (ips_per_eni - 1)) + 2"

    # kubeReserved resource values accept either a Kubernetes resource quantity
    # or a CEL expression string returning a bare number in that key's own unit.
    # Static:
    kubeReserved:
      cpu: "200m"
      memory: "512Mi"
    # OR Expression:
    kubeReserved:
      cpu: "max(60, vcpus * 30)"
      memory: "11 * max_pods + 255"

    # systemReserved follows the same pattern as kubeReserved.
    # Static:
    systemReserved:
      cpu: "100m"
      memory: "256Mi"
    # OR Expression:
    systemReserved:
      cpu: "max(20, vcpus * 10)"
      memory: "max(100, memory_mib / 64)"
```

`maxPods` changes type from `*int32` to `*intstr.IntOrString` (`x-kubernetes-int-or-string: true` in the CRD), so a single field accepts both forms. The former `minimum: 0` schema constraint is replaced by a CEL validation rule that only applies to the integer form — `type(self) != int || self >= 0` — since `minimum` is meaningless against a string. Because this is a type change to an already-hashed field, `EC2NodeClassHashVersion` is bumped to `v6`: the same unchanged value hashes differently under the new type.

`kubeReserved`/`systemReserved` remain `map[string]string`, but the quantity-only `pattern` previously applied to their values by `hack/validation/kubelet.sh` is removed. CEL syntax is arbitrary, so a quantity pattern would reject valid expressions at admission before the controller could interpret them. The existing key allowlist and the `!startsWith('-')` negative check are retained.

**Disambiguation logic (controller-side):**

* For `maxPods`: if the JSON value is a number → static. If it's a string → attempt CEL compilation.
* For `kubeReserved`/`systemReserved` values: attempt `resource.ParseQuantity()` first. If it succeeds → static quantity. If it fails → treat as a CEL expression and compile. If compilation fails → validation error.

This disambiguation is centralized on the API type (`KubeletConfiguration.HasExpressions()` / `HasResourceExpressions()`) so that every caller — the gate check, validation, the scheduler, and the launch template resolver — agrees on what counts as an expression.

### Expression Result Units and Rounding

Expressions return a bare number, so a unit is attached per key — the same unit that key's static quantities already use:

| Key | Unit | Example | Rounding |
|-----|------|---------|----------|
| `cpu` | millicores | `480` → `480m` | up to a multiple of 10m |
| `memory` | mebibytes | `630` → `630Mi` | up to a multiple of 16Mi |
| `ephemeral-storage` | gibibytes | `3` → `3Gi` | none |
| `pid` | process count | `4096` → `4096` | none |

For `cpu` the suffix is load-bearing: without it, `resource.ParseQuantity` reads a bare integer as *whole cores*, so `max(60, vcpus * 30)` would reserve 480 cores.

Rounding collapses near-identical results into shared buckets, limiting launch template proliferation (see [Launch Template Generation](#launch-template-generation)). It is always *up* — a reservation is never smaller than the expression asked for — and the granularities match what users already hand-write, so the worst case is +9m of CPU and +15Mi of memory.

Expressions may return an int or a double; doubles are truncated, and non-finite results (`+Inf`, `-Inf`, `NaN`) are rejected. An expression returning any other type fails to compile.

### Expression Language and Available Variables

Expressions use CEL (Common Expression Language) which is already used extensively in Kubernetes for validation rules. CEL provides a safe, sandboxed evaluation environment with no side effects.

The following variables are available in all kubelet expressions, populated from the instance type's InstanceTypeInfo:

| Variable | Type | Description | Example (m5.4xlarge) |
|----------|------|-------------|---------------------|
| `instance_type` | string | The EC2 instance type name | `"m5.4xlarge"` |
| `vcpus` | int | Number of vCPUs | 16 |
| `memory_mib` | int | Memory in MiB | 65536 |
| `default_enis` | int | Maximum network interfaces on the default network card | 8 |
| `ips_per_eni` | int | IPv4 addresses per ENI | 30 |
| `max_pods` | int | The resolved maxPods for this instance type | 58 |


The `max_pods` variable lets `kubeReserved` and `systemReserved` expressions reference the resolved maxPods — from a maxPods expression, a static maxPods value, or Karpenter's default (ENI-limited or 110, then capped by `podsPerCore` where applicable). 

### Static vs Expression Values

A field's value type determines behavior. There is no precedence or exclusivity logic — each field contains a single value that is either interpreted as a literal or evaluated as a CEL expression based on its JSON type. When neither `maxPods`, `kubeReserved`, nor `systemReserved` is set, Karpenter applies its internal defaults (ENI-limited maxPods, graduated kubeReserved formula) exactly as today.

### Validation

Validation happens in the nodeclass validation controller, in two stages, with failures surfaced on the `ValidationSucceeded` status condition rather than rejected at admission:

1. **Compile-time**, per expression: the expression must parse, type-check against the declared variables, and have an output type of int or double. Failures set reason `KubeletExpressionInvalid`.
2. **Evaluation-time**, per expression *per known instance type*: the expression must evaluate without error and produce a usable value — non-negative, and within int32 range for maxPods. Failures set reason `KubeletExpressionEvalFailed`.

The second stage exists because a compile-only check can't catch what depends on the instance type's actual values — division by a zero `ips_per_eni`, a subtraction that goes negative only on small instances, an overflow only on large ones. Both stages return a terminal error, so a NodeClass with a bad expression goes NotReady instead of launching misconfigured nodes.

One case is deliberately not terminal: if the instance-type cache hasn't hydrated yet, evaluation-time validation requeues instead of failing, so a valid NodeClass isn't left stuck false during startup.

CRD-level validation is intentionally minimal — the quantity `pattern` is removed and only the non-negative-integer rule on `maxPods` remains, since the schema can't distinguish a CEL expression from a malformed quantity.

### Testing Expressions Before Deployment

Validation only catches expressions that fail to compile or evaluate — it cannot tell whether an expression produces the *values* the operator intended. A logic mistake (for example, swapping a nested `min` for a `max`) compiles and evaluates cleanly but could reserve an unexpectedly large fraction of a node's capacity on certain instance sizes. Operators need a way to preview the resolved values across their fleet before applying an expression to a live cluster.

To support this, we can provide a small standalone script that evaluates an expression against a list of instance types and prints the resolved result for each one — entirely offline, without provisioning any nodes. The script sets up the same CEL environment Karpenter uses (identical variable names, types, and functions), compiles the expression once, and evaluates it against each instance type's properties. This is not part of the initial implementation.

## Scheduling and Launch Behavior

### Expression Evaluation at Scheduling Time

Karpenter must evaluate expressions during instance type resolution to produce accurate capacity predictions for scheduling simulation. This evaluation occurs in the existing `DefaultResolver.Resolve()` path in `pkg/providers/instancetype/types.go`.

For each instance type, the flow is:

1. Resolve maxPods — either the static integer, or evaluate the expression against variables in which `max_pods` holds Karpenter's default
2. Build the CEL variables for the reserved-resource expressions, now with `max_pods` set to the resolved value from step 1
3. Evaluate each `kubeReserved` value that isn't a parseable quantity, producing the resolved kubeReserved map
4. Do the same for `systemReserved`
5. Use these computed values in `computeCapacity()` and `computeOverhead()` exactly as if they were static values

This ensures the scheduler's capacity model matches what will actually be configured on the node.

Because resolution can now fail, `Resolver.Resolve()` returns an error, which propagates up through the instance type provider. An instance type whose expressions can't be resolved is no longer silently dropped from the list — the failure surfaces instead.

**Performance consideration:** CEL evaluation is lightweight. Compiled programs are cached by expression string in a single shared CEL environment, so an expression is compiled once and reused across every instance type. The cache uses a sliding TTL (refreshed on access) and only caches successful compilations, so a corrected expression isn't pinned to an earlier error. The environment is constructed once at operator startup and injected into the scheduler, the launch template resolver, and validation.

### Launch Template Generation

At launch time, the expression results feed into UserData generation through the existing `resolveLaunchTemplates()` path in `pkg/providers/amifamily/resolver.go`.

**Key change:** Today, instance types are grouped by maxPods value to minimize launch template proliferation (different maxPods = different UserData = different launch template). With expressions, both maxPods *and* the reserved-resource values vary per instance type, so the grouping key is extended to include the resolved `kubeReserved` and `systemReserved` maps (serialized to a sorted `k=v,...` string to keep the key comparable). Instance types that resolve to identical values still share a launch template.

Two things limit the resulting proliferation:

* Reserved-resource results are rounded up into shared buckets (10m of CPU, 16Mi of memory), collapsing near-identical values — see [Expression Result Units and Rounding](#expression-result-units-and-rounding).
* Expressions are only evaluated when a value isn't already a parseable quantity, so a NodeClass mixing static and expression values only fans out on the expression ones.

Because grouping now depends on evaluation, the grouping step can fail, and `Resolve()` takes a `context.Context` and returns the error rather than dropping the instance type.

### AMI Family Compatibility

Karpenter evaluates every expression itself and writes concrete values into UserData on all AMI families. Nothing expression-shaped reaches the node. Concretely, the resolved values replace the expression in the `KubeletConfiguration` passed to the AMI family's UserData generation, so each family's existing static-value path is reused unchanged:

| AMI Family | maxPods | kubeReserved | systemReserved |
|------------|---------|--------------|----------------|
| AL2023 (nodeadm) | Evaluate and pass integer in inline kubelet config | Evaluate and pass computed value in inline kubelet config | Evaluate and pass computed value in inline kubelet config |
| AL2 (EKS bootstrap) | Evaluate and pass integer via `--max-pods` | Evaluate and pass computed value via `--kube-reserved` | Evaluate and pass computed value via `--system-reserved` |
| Bottlerocket | Evaluate and pass integer via `settings.kubernetes.max-pods` | Evaluate and pass computed value via `settings.kubernetes.kube-reserved` | Evaluate and pass computed value via `settings.kubernetes.system-reserved` |
| Windows | Evaluate and pass integer via `-MaxPods` | Evaluate and pass computed value via `-KubeletExtraArgs` | Evaluate and pass computed value via `-KubeletExtraArgs` |
| Custom | Not applicable — user manages their own UserData | Not applicable | Not applicable |

#### Delegating maxPods evaluation to nodeadm

nodeadm's NodeConfig supports a [`maxPodsExpression`](https://github.com/awslabs/amazon-eks-ami) field that it evaluates at boot, which would let a single UserData template cover every instance type and eliminate the launch template fan-out for maxPods on AL2023. We are not using it initially: Karpenter must evaluate the expression at scheduling time regardless, so delegating to nodeadm means evaluating twice against two sources of instance-type data (`DescribeInstanceTypes` vs. nodeadm's IMDS view). Divergence would produce a node whose pod capacity disagrees with what the scheduler reserved — the exact failure this feature exists to prevent. This is a reasonable follow-up if launch template proliferation becomes a real problem, and would be additive.

## Drift

Current drift mechanisms will still detect changes to expressions in the nodeclass, so no drift changes are needed. Note that changing `maxPods` from `*int32` to `*intstr.IntOrString` changes how an unchanged value hashes, so `EC2NodeClassHashVersion` is bumped to `v6` — existing nodes are annotated with the new hash version rather than being treated as drifted.

## Alternative Solution: NodeOverlays

Another possible way to implement this would be through changing NodeOverlay:

**Pros:**

* Explicitly deals with ensuring Karpenter scheduling knows an accurate picture of resources
* Paired with granular rules/filtering around instance types

**Cons:**

* **Semantic mismatch:** NodeOverlay's purpose is to inform Karpenter of *out-of-band changes* to node shape — external systems (like third-party device plugins or custom capacity adjustments) that modify what a node looks like after Karpenter provisions it. It is not designed to *drive* configuration that Karpenter itself writes into UserData. Using it to set kubelet parameters reverses its information flow: instead of "tell Karpenter what changed externally," it becomes "tell Karpenter what to configure," which is EC2NodeClass's role.
* **No UserData generation:** NodeOverlay only adjusts Karpenter's internal scheduling model. It doesn't write kubelet flags into UserData — you'd still need a separate mechanism to actually configure the node, creating a split-brain where the overlay and the node config must be kept in sync manually.
* **Configuration explosion:** Users would need to create separate NodeOverlay objects per instance type (or per instance type range), since overlays match by label selectors, not by formula. This reintroduces the management complexity problem that CEL expressions solve.
* NodeOverlays aren't performant (at the moment)

**Example:** To achieve scaled kubeReserved across a fleet, a user would need multiple NodeOverlay objects:

```yaml
# One overlay per instance size range
apiVersion: karpenter.sh/v1alpha1
kind: NodeOverlay
metadata:
  name: small-instances-reserved
spec:
  weight: 1
  requirements:
    - key: karpenter.k8s.aws/instance-cpu
      operator: Lte
      values: ["4"]
  capacity:
    # Can only add extended resources — cannot override cpu/memory/pods
    # So this doesn't actually work for kubeReserved/systemReserved
---
apiVersion: karpenter.sh/v1alpha1
kind: NodeOverlay
metadata:
  name: large-instances-reserved
spec:
  weight: 1
  requirements:
    - key: karpenter.k8s.aws/instance-cpu
      operator: Gte
      values: ["16"]
  capacity:
    # Same limitation — no mechanism to set kubelet flags
```

NodeOverlay's capacity field only adds extended resources and explicitly cannot modify cpu, memory, ephemeral-storage, or pods (enforced by CRD validation: `self.all(x, !(x in ['cpu', 'memory', 'ephemeral-storage', 'pods']))`). It has price/priceAdjustment fields for cost modeling but no field for kubelet configuration. The API would need significant extension to support this use case, at which point it would be duplicating what EC2NodeClass already does.

Using CEL expressions and changing the NodeClass instead would be a better fit so that NodeOverlay doesn't get changed given the cons outweigh the pros.

## Appendix

### Expression Examples

Reserved-resource expressions return millicores for `cpu` and MiB for `memory`, so no scale factor is needed.

| Use Case | Field | Expression |
|----------|-------|------------|
| ENI-limited maxPods (default formula) | maxPods | `((default_enis - 1) * (ips_per_eni - 1)) + 2` |
| ENI-limited with prefix delegation (16 IPs/prefix) | maxPods | `min(250, ((default_enis - 1) * (ips_per_eni - 1)) * 16 + 2)` |
| Fixed pods cap | maxPods | `min(110, max_pods)` |
| Graduated CPU reservation (EKS recommended) | kubeReserved.cpu | `min(vcpus, 1) * 60 + min(max(vcpus - 1, 0), 1) * 10 + min(max(vcpus - 2, 0), 2) * 5 + max(vcpus - 4, 0) * 2.5` |
| Memory reservation scaled by pod count | kubeReserved.memory | `11 * max_pods + 255` |
| System memory as percentage of total | systemReserved.memory | `max(100, memory_mib / 64)` |

### Default Karpenter Formulas as Expressions

For reference, Karpenter's internal default computations for kubeReserved (used when no explicit value is configured) expressed as CEL:

```yaml
kubeReserved:
  # Graduated CPU reservation, in millicores:
  # 6% of first core, 1% of next core, 0.5% of next 2 cores, 0.25% of remaining
  cpu: "min(vcpus, 1) * 60 + min(max(vcpus - 1, 0), 1) * 10 + min(max(vcpus - 2, 0), 2) * 5 + max(vcpus - 4, 0) * 2.5"

  # Memory reservation, in MiB: 11 MiB per pod + 255 MiB base
  memory: "11 * max_pods + 255"
```

These are provided for documentation purposes. When neither kubeReserved nor a kubeReserved expression is set, Karpenter applies these formulas internally without requiring users to specify them as expressions.

Note that these expressions reproduce the defaults only up to rounding — the CPU formula can yield sub-10m steps, which are rounded up to the next 10m when written as a quantity.

### Supported CEL Functions

* Arithmetic: `+`, `-`, `*`, `/`, `%`
* Comparison: `<`, `<=`, `>`, `>=`, `==`, `!=`
* Logical: `&&`, `||`, `!`
* Built-in: `max(a, b)`, `min(a, b)`, `int()`, `double()`
* Conditional: `condition ? trueValue : falseValue`

`max` and `min` are custom two-argument overloads registered on the environment (int/int, double/double, and the mixed int/double pairs, which return a double). They are not CEL's variadic list-based macros — `max(a, b, c)` is not supported.

Note: This plan was made in mind to be compatible with potential future changes to the kubelet config struct switching from selected, hardcoded fields to an open map that gives users access to any field from a selected Kubernetes library version.  