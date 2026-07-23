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
      cpu: "max(60, vcpus * 30) * 1000000"
      memory: "(11 * max_pods + 255) * 1048576"
    systemReserved:
      cpu: "max(20, vcpus * 10) * 1000000"
      memory: "max(100, memory_mib / 64) * 1048576"
  amiSelectorTerms:
    - alias: al2023@latest
  ...
```

With this configuration on a c6a.4xlarge (16 vCPUs, 30720 MiB):

* kubeReserved.cpu = max(60, 16 * 30) * 1000000 = 480m
* systemReserved.memory = max(100, 30720 / 64) * 1048576 = 480Mi

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
* Ensure expression-based maxPods is passed through to nodeadm's `maxPodsExpression` field on AL2023 when supported
* Ensure computed values from expressions are passed as static values in UserData for AMI families that do not support expression evaluation (AL2, Bottlerocket, Windows)
* Validate expressions at EC2NodeClass admission time to provide early feedback on invalid syntax

## Non-Goals

Below lists the non-goals for this RFC design. Each of these items represents potential follow-ups for the initial implementation and are features we will consider based on feature requests.

* Expression-based blockDeviceMappings (e.g., volume size scaling with instance size) — while this is a natural extension, it involves different subsystems and warrants a separate design
* Supporting arbitrary kubelet flags as expressions — only maxPods, kubeReserved, and systemReserved are in scope
* User-defined variables or custom functions in expressions — only built-in instance type properties are available

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
    # or a CEL expression string that evaluates to base units (nanocores, bytes).
    # Static:
    kubeReserved:
      cpu: "200m"
      memory: "512Mi"
    # OR Expression:
    kubeReserved:
      cpu: "max(60, vcpus * 30) * 1000000"
      memory: "(11 * max_pods + 255) * 1048576"

    # systemReserved follows the same pattern as kubeReserved.
    # Static:
    systemReserved:
      cpu: "100m"
      memory: "256Mi"
    # OR Expression:
    systemReserved:
      cpu: "max(20, vcpus * 10) * 1000000"
      memory: "max(100, memory_mib / 64) * 1048576"
```

**Disambiguation logic (controller-side):**

* For `maxPods`: if the JSON value is a number → static. If it's a string → attempt CEL compilation.
* For `kubeReserved`/`systemReserved` values: attempt `resource.ParseQuantity()` first. If it succeeds → static quantity. If it fails → attempt CEL compilation. If both fail → validation error.

### Expression Language and Available Variables

Expressions use CEL (Common Expression Language) which is already used extensively in Kubernetes for validation rules. CEL provides a safe, sandboxed evaluation environment with no side effects.

The following variables are available in all kubelet expressions, populated from the instance type's InstanceTypeInfo:

| Variable | Type | Description | Example (m5.4xlarge) |
|----------|------|-------------|---------------------|
| `instance_type` | string | The EC2 instance type name | `"m5.4xlarge"` |
| `vcpus` | int | Number of vCPUs | 16 |
| `memory_mib` | int | Memory in MiB | 65536 |
| `default_enis` | int | Maximum number of ENIs | 8 |
| `ips_per_eni` | int | IPv4 addresses per ENI | 30 |
| `max_pods` | int | Karpenter's computed default maxPods (ENI-limited or 110) | 58 |

The `max_pods` variable provides a self-referencing convenience — `kubeReserved` and `systemReserved` expressions can reference the resolved maxPods value (whether from a maxPods expression, a static maxPods value, or the default ENI-limited calculation).

### Expression Validation

**Validation flow (controller-side, during reconciliation):**

1. Extract `maxPods` from the kubelet map:
   * If JSON number → valid static value, must be non-negative
   * If JSON string → compile as CEL. If compilation fails → `ValidationSucceeded: False`
   * If any other JSON type → `ValidationSucceeded: False`

2. Extract `kubeReserved` / `systemReserved` from the kubelet map:
   * Must be a JSON object (map of resource name to value)
   * Keys must be one of: `cpu`, `memory`, `ephemeral-storage`, `pid`
   * For each value string:
     * Try `resource.ParseQuantity()` → if succeeds, valid static quantity (must not be negative)
     * If fails → compile as CEL → if compilation fails → `ValidationSucceeded: False`
     * CEL expressions must reference only permitted variables and return a numeric type

3. CEL compilation validation:
   * Expression must parse as valid CEL syntax
   * Expression must reference only permitted variables (`instance_type`, `vcpus`, `memory_mib`, `default_enis`, `ips_per_eni`, `max_pods`)
   * Expression must type-check to return `int` or `double` (the `instance_type` string may only be used within the expression, e.g. in comparisons — the overall result must still be numeric)

4. Evaluation-time validation (at scheduling):
   * If an expression evaluates to a negative number for a specific instance type, that instance type is excluded from consideration

### Static vs Expression Values

A field's value type determines behavior. There is no precedence or exclusivity logic — each field contains a single value that is either interpreted as a literal or evaluated as a CEL expression based on its JSON type. When neither `maxPods`, `kubeReserved`, nor `systemReserved` is set, Karpenter applies its internal defaults (ENI-limited maxPods, graduated kubeReserved formula) exactly as today.

### Testing Expressions Before Deployment

Admission-time validation only catches syntax and type errors — it cannot tell whether an expression produces the *values* the operator intended. A logic mistake (for example, swapping a nested `min` for a `max`) compiles cleanly but could reserve an unexpectedly large fraction of a node's capacity on certain instance sizes. Operators need a way to preview the resolved values across their fleet before applying an expression to a live cluster.

To support this, we can provide a small standalone script that evaluates an expression against a list of instance types and prints the resolved result for each one — entirely offline, without provisioning any nodes. The script sets up the same CEL environment Karpenter uses (identical variable names, types, and functions), compiles the expression once, and evaluates it against each instance type's properties:

```
$ ./eval-kubelet-expr \
    --field kubeReserved.cpu \
    --expression "max(60, vcpus * 30) * 1000000" \
    --instance-types m5.large,m5.4xlarge,m5.24xlarge

INSTANCE TYPE   vCPUs   RESOLVED kubeReserved.cpu
m5.large        2       120m
m5.4xlarge      16      480m
m5.24xlarge     96      2880m
```

By printing one row per instance type, the script lets an operator eyeball the full range of outputs and catch a value that is too high or too low before it ever reaches a node. Because the script reuses Karpenter's CEL environment definition, an expression that evaluates successfully here behaves identically when Karpenter evaluates it at scheduling time.

## Scheduling and Launch Behavior

### Expression Evaluation at Scheduling Time

Karpenter must evaluate expressions during instance type resolution to produce accurate capacity predictions for scheduling simulation. This evaluation occurs in the existing `DefaultResolver.Resolve()` path in `pkg/providers/instancetype/types.go`.

For each instance type, the flow is:

1. Build the CEL evaluation context with the instance type's properties (vCPUs, memory, ENI counts, etc.)
2. If maxPods expression is set, evaluate it to produce the maxPods value for this instance type
3. If kubeReserved expression is set, evaluate each resource expression to produce the kubeReserved map
4. If systemReserved expression is set, evaluate each resource expression to produce the systemReserved map
5. Use these computed values in `computeCapacity()` and `computeOverhead()` exactly as if they were static values

This ensures the scheduler's capacity model matches what will actually be configured on the node.

**Performance consideration:** CEL evaluation is lightweight. Expression compilation can be cached across evaluations since the expression text doesn't change between instance types.

### Launch Template Generation

At launch time, the expression results feed into UserData generation through the existing `resolveLaunchTemplates()` path in `pkg/providers/amifamily/resolver.go`.

**Key change:** Today, instance types are grouped by maxPods value to minimize launch template proliferation (different maxPods = different UserData = different launch template). With expressions, every instance type may produce a unique maxPods value, potentially increasing the number of launch templates.

**Mitigation strategies:**

* For CEL expressions in `maxPods` with AL2023:
  * Karpenter passes the expression string directly to nodeadm's `maxPodsExpression` field in the generated NodeConfig, allowing nodeadm to evaluate it at boot time
  * Karpenter also evaluates the expression at scheduling time for accurate capacity predictions
* For CEL expressions in `maxPods` with other AMI families (AL2, Bottlerocket, Windows):
  * Karpenter evaluates the expression per-instance-type and passes the resulting integer as the static `--max-pods` flag
* For `kubeReserved`/`systemReserved` on all AMI families:
  * Karpenter always evaluates CEL expressions and passes computed static values in UserData (nodeadm does not support expressions for these fields)

### AMI Family Compatibility

#### CEL Expressions in amazon-eks-ami NodeConfig

The amazon-eks-ami project (https://github.com/awslabs/amazon-eks-ami) recently introduced a `maxPodsExpression` field in the nodeadm NodeConfig specification. This field accepts a CEL (Common Expression Language) expression that is evaluated on the node at boot time using the instance's actual properties:

```yaml
kind: NodeConfig
apiVersion: node.eks.aws/v1alpha1
spec:
  kubelet:
    config:
      maxPodsExpression: "((default_enis - 1) * (ips_per_eni - 1)) + 2"
```

At boot, nodeadm resolves the instance type from IMDS, looks up the instance's networking properties, evaluates the expression, and passes the resulting integer as the kubelet `--max-pods` argument. This allows a single UserData template to produce correct per-instance-type values.

Karpenter can leverage this mechanism for maxPods on AL2023 nodes, while also implementing expression evaluation at scheduling time to maintain accurate capacity predictions. For systemReserved and kubeReserved, which nodeadm does not yet support as expressions, Karpenter must evaluate expressions before launch and pass computed values in UserData.

| AMI Family | maxPods | kubeReserved | systemReserved |
|------------|---------|--------------|----------------|
| AL2023 (nodeadm) | Pass expression string to nodeadm `maxPodsExpression` if supported; otherwise evaluate and pass integer | Evaluate and pass computed value in inline kubelet config | Evaluate and pass computed value in inline kubelet config |
| AL2 (EKS bootstrap) | Evaluate and pass integer via `--max-pods` | Evaluate and pass computed value via `--kube-reserved` | Evaluate and pass computed value via `--system-reserved` |
| Bottlerocket | Evaluate and pass integer via `settings.kubernetes.max-pods` | Evaluate and pass computed value via `settings.kubernetes.kube-reserved` | Evaluate and pass computed value via `settings.kubernetes.system-reserved` |
| Windows | Evaluate and pass integer via `-MaxPods` | Evaluate and pass computed value via `-KubeletExtraArgs` | Evaluate and pass computed value via `-KubeletExtraArgs` |
| Custom | Not applicable — user manages their own UserData | Not applicable | Not applicable |

## Drift

Current drift mechanisms will still detect changes to expressions in the nodeclass, so no drift changes are needed.

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

| Use Case | Field | Expression |
|----------|-------|------------|
| ENI-limited maxPods (default formula) | maxPods | `((default_enis - 1) * (ips_per_eni - 1)) + 2` |
| ENI-limited with prefix delegation (16 IPs/prefix) | maxPods | `min(250, ((default_enis - 1) * (ips_per_eni - 1)) * 16 + 2)` |
| Fixed pods cap | maxPods | `min(110, max_pods)` |
| Graduated CPU reservation (EKS recommended) | kubeReserved.cpu | `(min(vcpus, 1) * 60 + min(max(vcpus - 1, 0), 1) * 10 + min(max(vcpus - 2, 0), 2) * 5 + max(vcpus - 4, 0) * 2.5) * 1000000` |
| Memory reservation scaled by pod count | kubeReserved.memory | `(11 * max_pods + 255) * 1048576` |
| System memory as percentage of total | systemReserved.memory | `max(104857600, memory_mib * 1048576 / 64)` |

### Default Karpenter Formulas as Expressions

For reference, Karpenter's internal default computations for kubeReserved (used when no explicit value is configured) expressed as CEL:

```yaml
kubeReserved:
  # Graduated CPU reservation:
  # 6% of first core, 1% of next core, 0.5% of next 2 cores, 0.25% of remaining
  cpu: "(min(vcpus, 1) * 60 + min(max(vcpus - 1, 0), 1) * 10 + min(max(vcpus - 2, 0), 2) * 5 + max(vcpus - 4, 0) * 2.5) * 1000000"

  # Memory reservation: 11 MiB per pod + 255 MiB base
  memory: "(11 * max_pods + 255) * 1048576"
```

These are provided for documentation purposes. When neither kubeReserved nor a kubeReserved expression is set, Karpenter applies these formulas internally without requiring users to specify them as expressions.

### Supported CEL Functions

* Arithmetic: `+`, `-`, `*`, `/`, `%`
* Comparison: `<`, `<=`, `>`, `>=`, `==`, `!=`
* Logical: `&&`, `||`, `!`
* Built-in: `max(a, b)`, `min(a, b)`, `int()`, `double()`
* Conditional: `condition ? trueValue : falseValue`
