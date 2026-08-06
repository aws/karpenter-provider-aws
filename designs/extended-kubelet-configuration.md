# Extended Kubelet Configuration

## Motivation

Karpenter's `KubeletConfiguration` struct (in `spec.kubelet`) is a strongly-typed Go struct that mirrors a fixed subset of upstream kubelet fields. It contains only 12 fields that fall into two categories:

* **Scheduling-relevant fields** (Karpenter reads these to compute instance type `Capacity` and `Overhead`):
  * `maxPods` — determines `Capacity.Pods`
  * `podsPerCore` — caps pod capacity at `podsPerCore * vCPUs`
  * `kubeReserved` — subtracted from allocatable as `Overhead.KubeReserved`
  * `systemReserved` — subtracted from allocatable as `Overhead.SystemReserved`
  * `evictionHard` — subtracted from allocatable as `Overhead.EvictionThreshold` (`memory.available` and `nodefs.available` signals only)
* **Passthrough fields** (written to UserData but do not affect scheduling):
  * `clusterDNS`
  * `evictionSoft`
  * `evictionSoftGracePeriod`
  * `evictionMaxPodGracePeriod`
  * `imageGCHighThresholdPercent`
  * `imageGCLowThresholdPercent`
  * `cpuCFSQuota`

This creates two problems:

* **Version lag:** Kubernetes adds new kubelet configuration fields with each release (e.g., `memoryThrottlingFactor` in 1.22, `topologyManagerPolicy`, `containerLogMaxSize`). Users who want to configure these fields through Karpenter must wait for a Karpenter code change to add each field to the struct, get it reviewed, released, and adopted. Karpenter's version-independent release model (one release supports many Kubernetes versions) means this lag is structural, not incidental.
* **Incomplete coverage:** The upstream `KubeletConfiguration` ([k8s.io/kubelet/config/v1beta1](https://pkg.go.dev/k8s.io/kubelet/config/v1beta1)) has 100+ fields. Karpenter exposes only 12. Users who need anything beyond these 12 must resort to custom UserData, which bypasses Karpenter's capacity calculation entirely and is error-prone.

### Community Request

Multiple open issues demonstrate users blocked by the current 12-field limit:

* [#5833](https://github.com/aws/karpenter-provider-aws/issues/5833) — Mega tracking issue for non-scheduling kubelet fields, linking 7+ individual feature requests (`topologyManagerPolicy`, `containerLogMaxSize`, `shutdownGracePeriod`, etc.)
* [#7982](https://github.com/aws/karpenter-provider-aws/issues/7982) — Customer requesting 9 specific fields (`eventBurst`, `kubeAPIBurst`, `podPIDsLimit`, `shutdownGracePeriod`, etc.) to avoid custom bootstrapping workarounds
* [#8189](https://github.com/aws/karpenter-provider-aws/issues/8189) — Customer requesting `imageMaximumGCAge` (added in Kubernetes 1.29), a single field that remains unavailable through Karpenter despite being upstream for multiple releases

## Design: Open Map, Validated Against a Pinned Upstream Type

`spec.kubelet` becomes an open map, and Karpenter validates its contents against the upstream `k8s.io/kubelet/config/v1beta1.KubeletConfiguration` Go type that Karpenter compiles against:

* Any field of the upstream `KubeletConfiguration` for the pinned Kubernetes version may be set
* Karpenter extracts the fields it needs for scheduling and bootstrap; everything else is passthrough
* **Every** key and its type are validated by decoding the map against the upstream Go type, plus the semantic rules the Go types can't express (which map keys are meaningful, value ranges, cross-field relationships)
* Validation runs in the EC2NodeClass controller and surfaces as `ValidationSucceeded=False`, which blocks node launch

### Why Not a Typed Struct

Two reasons, beyond version lag:

1. **Upstream types can't express "unset".** Upstream declares `maxPods` and `podsPerCore` as non-pointer `int32` with `omitempty`, so a typed mirror can't distinguish "unset" from an explicit `0` for two of the five fields Karpenter reads to make scheduling decisions.
2. **New kubelet fields need no Go change.** With a map, a `k8s.io/kubelet` bump in `go.mod` is the entire change required to accept a newly released field and pass it through to UserData.

### Why Not Enumerate Every Field in the CRD

Generating a closed CRD schema covering all 100+ upstream fields would restore admission-time validation, at the cost of a very large CRD and a schema change (and therefore a CRD upgrade) on every `k8s.io/kubelet` bump. Karpenter also can't express the semantic rules it needs (eviction-signal keys, reserved-resource keys, CEL expressions in place of integers) against a generated upstream schema.

### Tradeoff

Today, invalid kubelet config is rejected at `kubectl apply` time via CEL validation rules on the CRD. With an open map, the API server does no validation of `spec.kubelet` at all — it can't check fields it has no schema for, and it refuses to compile `x-kubernetes-validations` against a map marked `x-kubernetes-preserve-unknown-fields`. Errors surface on `status.conditions` instead, so the user reads a status condition rather than having their apply rejected. We accept this because invalid configuration still never reaches a node, and it eliminates the version lag problem entirely.

## EC2NodeClass API Change

The API shape does not change. Users continue writing kubelet configuration at `spec.kubelet.<field>` exactly as they do today. The only difference is that the CRD stops rejecting fields it doesn't recognize — it becomes open rather than closed.

### User Experience

```yaml
spec:
  kubelet:
    maxPods: 110
    podsPerCore: 2
    kubeReserved:
      cpu: "200m"
      memory: "512Mi"
    evictionHard:
      memory.available: "5%"
    # NEW: any upstream kubelet field now accepted without Karpenter code changes
    containerLogMaxSize: "50Mi"
    topologyManagerPolicy: "best-effort"
    serializeImagePulls: false
```

The field path is identical. Existing manifests work unchanged. Users simply start adding fields they need.

### Go Type Change

`KubeletConfiguration` becomes a map type that preserves all fields:

```go
// KubeletConfiguration mirrors the upstream kubelet KubeletConfiguration as an open map.
// +kubebuilder:pruning:PreserveUnknownFields
// +kubebuilder:validation:Type=object
type KubeletConfiguration map[string]apiextensionsv1.JSON
```

```go
// +kubebuilder:pruning:PreserveUnknownFields
// +kubebuilder:validation:Type=object
// +optional
Kubelet KubeletConfiguration `json:"kubelet,omitempty"`
```

The generated CRD schema for `spec.kubelet` is an unconstrained object:

```yaml
kubelet:
  additionalProperties:
    x-kubernetes-preserve-unknown-fields: true
  type: object
  x-kubernetes-preserve-unknown-fields: true
```

Since JSON keys remain identical (`maxPods`, `kubeReserved`, etc.), existing serialized objects and user manifests continue to deserialize correctly. No stored version conversion is required.

### Kubernetes Library Dependency

`k8s.io/kubelet` is a pinned direct dependency in `go.mod` (currently `v0.35.0`). It is the authority for which fields exist and what type each takes; nothing in Karpenter duplicates that list. Bumping it is what makes newly released kubelet fields available, and is a one-line change with no CRD regeneration.

The pin also bounds what Karpenter claims to support: a field added in a kubelet newer than the pinned version is rejected as unknown, and a field present in the pinned version but not in the older kubelet binary running on the node is accepted by Karpenter and rejected at boot. The second case is unavoidable for any version-independent controller — Karpenter does not know the kubelet version of the node it is about to launch.

## Field Extraction for Scheduling and Bootstrap

Karpenter extracts the fields it interprets by marshalling the map and unmarshalling it into a typed struct:

```go
// ParsedKubeletConfig holds the extracted kubelet configuration fields that Karpenter
// uses for scheduling decisions and bootstrap scripting.
type ParsedKubeletConfig struct {
    ClusterDNS []string `json:"clusterDNS,omitempty"`
    // MaxPods is an integer or a CEL expression evaluated per instance type. It's typed as
    // IntOrString rather than *int32 so that an expression parses instead of failing the whole
    // config: ParseKubeletConfig is a single Unmarshal, so one field that won't decode loses
    // every other field with it. Use MaxPodsValue for the resolved count.
    MaxPods                     *intstr.IntOrString        `json:"maxPods,omitempty"`
    PodsPerCore                 *int32                     `json:"podsPerCore,omitempty"`
    SystemReserved              map[string]string          `json:"systemReserved,omitempty"`
    KubeReserved                map[string]string          `json:"kubeReserved,omitempty"`
    EvictionHard                map[string]string          `json:"evictionHard,omitempty"`
    EvictionSoft                map[string]string          `json:"evictionSoft,omitempty"`
    EvictionSoftGracePeriod     map[string]metav1.Duration `json:"evictionSoftGracePeriod,omitempty"`
    EvictionMaxPodGracePeriod   *int32                     `json:"evictionMaxPodGracePeriod,omitempty"`
    ImageGCHighThresholdPercent *int32                     `json:"imageGCHighThresholdPercent,omitempty"`
    ImageGCLowThresholdPercent  *int32                     `json:"imageGCLowThresholdPercent,omitempty"`
    CPUCFSQuota                 *bool                      `json:"cpuCFSQuota,omitempty"`
}
```

This is the same 12 fields Karpenter interpreted before — five that affect scheduling, seven that only affect UserData. Extraction is deliberately lenient: `ParseKubeletConfig` is a single `Unmarshal`, so a field that won't decode would take every other field in the same document with it. Callers that fail to parse fall back to an empty struct and let the validation reconciler report the problem, rather than making scheduling decisions from a partially decoded config.

The instance type resolver uses the extracted values in `computeCapacity()` and `computeOverhead()` exactly as before, and applies the same defaults as today when a field is absent (ENI-limited `maxPods`, graduated `kubeReserved` formula). All other fields in the map are passed to the bootstrap layer without interpretation.

`ParsedKubeletConfig` is also part of the instance type cache key: the resolver hashes the raw `spec.kubelet` map, so any change to any field — interpreted or passthrough — invalidates the cached instance types.

## Validation

`ValidateKubeletConfig` runs in the EC2NodeClass validation reconciler and returns every error it finds, which are joined into a single condition message.

### Validation Flow

* **User applies EC2NodeClass** — The API server accepts any content in `spec.kubelet` (structural JSON validation only).
* **NodeClass controller reconciles** — `ValidateKubeletConfig` decodes the map against the upstream type and applies the semantic rules. On failure it sets `ValidationSucceeded=False` with reason `InvalidKubeletConfiguration` and all error messages.
* **Launch is blocked** — `ValidationSucceeded` is a required condition, so no nodes launch for this NodeClass. Pending pods remain unschedulable until the configuration is fixed.
* **User sees the error** — The condition message appears on the EC2NodeClass object:

```
kubectl get ec2nodeclass default -o yaml

status:
  conditions:
  - type: ValidationSucceeded
    status: "False"
    reason: InvalidKubeletConfiguration
    message: 'spec.kubelet: unknown field "maxPod"'
```

### Type and Field-Name Validation

The map is marshalled and decoded with `sigs.k8s.io/json`'s `UnmarshalStrict` against `kubeletconfigv1beta1.KubeletConfiguration`. `sigs.k8s.io/json` is used rather than `encoding/json` because it reports unknown fields as errors instead of ignoring them.

This one decode covers both unknown field names and wrong types, for every field in the map — not just the 12 Karpenter interprets. A field name that isn't in the upstream type fails, and so does `containerLogMaxSize: 50` when upstream types it as a string.

Unknown fields are reported per-field, so several typos surface together. A type mismatch fails the whole decode, so it is reported alone.

Fields that may hold a CEL expression are decoded with a placeholder standing in for the expression: upstream types `maxPods` as `int32`, so an expression string would fail to decode and mask every other field in the same document. Only the shape matters at this stage; the expression itself is validated where it's evaluated.

### Semantic Validation

The upstream Go types constrain field names and types but not which map keys are meaningful, what ranges values may take, or how fields relate. These rules are enforced separately:

| Field(s) | Rule |
|---|---|
| `evictionHard`, `evictionSoft`, `evictionSoftGracePeriod`, `evictionMinimumReclaim` | Keys must be valid eviction signals (`memory.available`, `nodefs.available`, `nodefs.inodesFree`, `imagefs.available`, `imagefs.inodesFree`, `pid.available`) |
| `evictionHard`, `evictionSoft`, `evictionMinimumReclaim` | Values must be a percentage or a resource quantity |
| `evictionSoft` ↔ `evictionSoftGracePeriod` | Keys must match in both directions — the kubelet ignores a soft threshold with no grace period and rejects a grace period for a signal it isn't evicting on |
| `kubeReserved`, `systemReserved` | Keys must be reservable resources (`cpu`, `memory`, `ephemeral-storage`, `pid`); values must not be negative |
| `imageGCHighThresholdPercent`, `imageGCLowThresholdPercent` | Must be between 0 and 100, and high must be greater than low — an inverted pair would never collect |
| `maxPods`, `podsPerCore`, `evictionMaxPodGracePeriod` | Must not be negative |

Two notes on interaction with CEL expressions:

* `kubeReserved` and `systemReserved` values are not required to parse as a resource quantity, since a value may be an expression evaluated per instance type (e.g. `"vcpus * 10"`). The negative check is a string prefix check for the same reason.
* `maxPods` is bounds-checked only in its literal integer form. An expression is bounds-checked after it has been evaluated against an instance type.

The eviction-signal and reserved-resource key checks exist because the kubelet *ignores* unrecognized keys rather than rejecting them: without this check, a misspelled signal would silently drop the threshold the user set, and the node would come up looking healthy with the wrong eviction behavior.

Passthrough field *values* beyond these rules are not semantically validated. Karpenter checks that `containerLogMaxSize` is a string, not that it's a parseable quantity. A value the kubelet rejects at boot surfaces as a node that fails to register.

### Error Surfacing Summary

| Error type | When detected | Effect |
|---|---|---|
| Unknown field name (not in the pinned upstream type) | Controller reconciliation | Launch blocked. `ValidationSucceeded: False`. |
| Wrong type for any field, interpreted or passthrough | Controller reconciliation | Launch blocked. `ValidationSucceeded: False`. |
| Semantic violation (bad map key, out-of-range value, mismatched pair) | Controller reconciliation | Launch blocked. `ValidationSucceeded: False`. |
| Correctly typed passthrough value the kubelet rejects, or a field newer than the node's kubelet | Node boot | Node fails to register. `Registered: False` on the NodeClaim. |

The last row is the only case that reaches a node, and it is the case Karpenter cannot detect without knowing the node's kubelet version:

```
status:
  conditions:
  - type: Registered
    status: "False"
    reason: NodeRegistrationFailed
    message: "Node failed to register within timeout. Check kubelet logs for configuration errors."
```

## Bootstrap Integration

The bootstrap layer receives both representations: the parsed struct (`KubeletConfig`) and the raw map (`UnparsedKubeletConfig`). AMI families consume different ones, so both exist rather than one being derived at the boundary.

Before either is handed to an AMI family, the launch template resolver fills in `maxPods` on both from the count already resolved for the instance type, if the user didn't set a literal integer. That covers two cases with one path: the user set nothing, and the user set a CEL expression that only has meaning against a specific instance type. Updating only the parsed struct would ship the unevaluated expression to a nodeadm-based node, or omit `maxPods` from its inline config entirely.

**AL2023 (nodeadm)** — the raw map is passed through to `NodeConfig.Spec.Kubelet.Config`, which already accepts `map[string]runtime.RawExtension`. Karpenter injects `registerWithTaints` and the node label flag on top. This is the natural fit: nodeadm was designed for arbitrary kubelet config, so passthrough fields need no per-field handling.

**AL2 / Ubuntu (EKS bootstrap script)** — the parsed struct drives explicit kubelet flags (`--max-pods`, `--kube-reserved`, `--eviction-hard`, …) via `--kubelet-extra-args`, and `clusterDNS` becomes `--dns-cluster-ip`. Only a resolved integer `maxPods` becomes a flag, so an unevaluated expression is never passed to the kubelet verbatim.

**Bottlerocket** — the parsed struct is mapped to its TOML equivalents under `settings.kubernetes.*`. Bottlerocket uses an allowlist model, so a field with no settings-API equivalent isn't expressible; unknown keys in the Kubernetes settings are logged when the settings are re-decoded in strict mode.

**Windows** — like AL2, the parsed struct becomes PowerShell parameters (`-KubeletExtraArgs`, `-DNSClusterIP`).

**Custom** — the kubelet config map isn't used; users manage their own UserData entirely.

### Passthrough Coverage Is AMI-Family-Dependent

Passthrough fields currently reach the node on AL2023 only. On the flag-based families, a field outside the extracted 12 passes validation and is then dropped, because those bootstrappers build their arguments from the parsed struct.

Closing this gap means generating a `KubeletConfiguration` file in UserData for AL2 and Windows, since many kubelet fields have no CLI flag equivalent and can only be set through a config file. For Bottlerocket it means mapping additional fields onto the settings API where an equivalent exists, and there is no path for fields where one doesn't.

Until then, a user setting a passthrough field on a flag-based family gets silence rather than an error, which is the weakest part of this design. Surfacing it — a warning on the EC2NodeClass naming the fields that won't take effect for the configured AMI family — is worth doing regardless of whether the config-file work lands, since Bottlerocket will always have fields it can't express.

## Migration and Backwards Compatibility

The transition is transparent to users:

* **Existing manifests:** Work unchanged. The same fields at the same paths with the same values.
* **No deprecation needed:** There is no old API to deprecate — the paths haven't moved.
* **Upgrade behavior:** On upgrade, the API server stops pruning unknown fields from `spec.kubelet`. Existing stored objects retain their fields. Users can immediately start adding new kubelet fields.

The one behavioral change is *when* an invalid config is reported. A manifest that `kubectl apply` used to reject now applies successfully and fails in the controller, so tooling asserting on apply-time rejection needs to read `ValidationSucceeded` instead.

## Drift

Drift detection works unchanged. The EC2NodeClass spec hash already covers all of `spec.kubelet`. Adding, removing, or modifying any field in the map changes the hash and triggers drift.

## Interaction with CEL Expressions

The extended kubelet config and CEL expression features compose naturally. Expressions are values within the kubelet map, not peer fields, which is why the extraction and validation layers accommodate a string where upstream expects an integer:

```yaml
spec:
  kubelet:
    containerLogMaxSize: "50Mi"
    topologyManagerPolicy: "best-effort"
    evictionHard:
      memory.available: "5%"
    maxPods: "((default_enis - 1) * (ips_per_eni - 1)) + 2"
    kubeReserved:
      cpu: "max(60, vcpus * 30) * 1000000"
```

In this example:

* `containerLogMaxSize` and `topologyManagerPolicy` are passthrough fields that the closed struct couldn't accept
* `evictionHard` is extracted for scheduling
* `maxPods` and `kubeReserved.cpu` hold expressions rather than static values, evaluated per instance type and resolved before they reach the kubelet

Non-scheduling fields in the map are always passthrough regardless of expression usage.