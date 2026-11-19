# UserData Templating for EC2NodeClass

This document proposes rendering `EC2NodeClass`'s `spec.userData` through Go's `text/template` engine, giving
users of any AMI family access to the same node-specific values that Karpenter already computes internally.

## Overview

For standard AMI families (e.g. `AL2`, `Bottlerocket`), Karpenter
automatically injects cluster and node context — cluster endpoint, CA bundle, resolved labels/taints, kubelet
configuration — into the `userData` it generates.
But every AMI family also accepts a user-supplied `userData` string, merged with Karpenter's own generated
content, and that user-supplied portion has no access to cluster-specific values today: users must hardcode those
values or have their AMI fetch them independently at boot, duplicating logic Karpenter has already run.
This applies most acutely to `Custom` AMIFamily, where Karpenter generates no bootstrap content at all and the
entire `userData` is user-supplied.

### Community Request

This gap is tracked in [#5134](https://github.com/aws/karpenter-provider-aws/issues/5134).
An earlier PR, [#4504](https://github.com/aws/karpenter-provider-aws/pull/4504), attempted exactly this — templating
labels/taints into `Custom` AMI `userData` — but was closed without merging.
A prior RFC, [#8357](https://github.com/aws/karpenter-provider-aws/pull/8357), proposed solving this together with
a second problem: excluding certain `userData` values from drift detection so that rotating
secrets/tokens wouldn't trigger node replacement ([#8336](https://github.com/aws/karpenter-provider-aws/issues/8336)).

This RFC intentionally narrows scope to templating only, leaving the drift-exemption question to
[#8336](https://github.com/aws/karpenter-provider-aws/issues/8336).

## Workaround

A `Custom` AMI author can sidestep needing templating entirely today: build the image with its own
`/etc/eks/bootstrap.sh`-compatible script, and set `amiFamily: AL2` instead of `Custom`.
Karpenter's existing AL2 bootstrap logic then generates and invokes that script with the standard flags
(`--apiserver-endpoint`, `--b64-cluster-ca`, `--kubelet-extra-args`, etc.) computed from `spec.kubelet` and the cluster.
This `bootstrap.sh` can parse flags and relay the values to the actual custom bootstrap process.

Although this works today, the merged `userData` can hit the size limit of 16 KB, it implicitly relies on `/etc/eks/bootstrap.sh`'s
argument contract and it only works for reproducing AL2's bootstrap flags, not for templating arbitrary `userData` content.

## Customer Use Case

A team runs a Custom AMI whose bootstrap script reads its configuration from an environment file.
It needs the cluster endpoint and CA bundle to join the cluster, and the `maxPods`/`kubeReserved`/`systemReserved`
values Karpenter already resolved for the node (from internal formulas, or a
[`NodeClassCEL`](./dynamic-kubelet-configuration-via-expressions.md) expression in `spec.kubelet`) so the kubelet's
limits stay in sync with what Karpenter assumed while scheduling.
Without templating, the team would have to hardcode these per-cluster values, or reimplement Karpenter's own
resolution logic — including evaluating the same CEL expressions — to keep them from diverging.

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: custom
spec:
  amiFamily: Custom
  amiSelectorTerms:
    - id: ami-0123456789abcdef0
  kubelet:
    maxPods: "((default_enis - 1) * (ips_per_eni - 1)) + 2"
    kubeReserved:
      cpu: "max(60, vcpus * 30)"
      ephemeral-storage: "1Gi"
    systemReserved:
      memory: "max(100, memory_mib / 64)"
  userDataTemplate: true
  userData: |
    #cloud-config
    write_files:
      - path: /etc/kubernetes/environment
        content: |
          CLUSTER_NAME='{{ .ClusterName }}'
          CLUSTER_ENDPOINT='{{ .ClusterEndpoint }}'
          CA_BUNDLE='{{ .CABundle }}'
          MAX_PODS='{{ .Kubelet.MaxPods }}'
          KUBE_RESERVED_CPU='{{ index .Kubelet.KubeReserved "cpu" }}'
          KUBE_RESERVED_EPHEMERAL_STORAGE='{{ index .Kubelet.KubeReserved "ephemeral-storage" }}'
          SYSTEM_RESERVED_MEMORY='{{ index .Kubelet.SystemReserved "memory" }}'
```

The following template reproduces, under `amiFamily: Custom`, both the `/etc/eks/bootstrap.sh` invocation and the
MIME multipart merge with a custom script that `amiFamily: AL2` performs automatically:

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: al2-via-custom
spec:
  amiFamily: Custom
  amiSelectorTerms:
    - id: ami-0123456789abcdef2 # an AL2 AMI
  kubelet:
    clusterDNS: ["fd30:1000:0:1::a"] # an IPv6 cluster
    maxPods: 110
    podsPerCore: 10
    kubeReserved:
      cpu: "100m"
    systemReserved:
      memory: "100Mi"
    evictionHard:
      memory.available: "5%"
    imageGCHighThresholdPercent: 85
    imageGCLowThresholdPercent: 80
    cpuCFSQuota: false
  instanceStorePolicy: RAID0
  userDataTemplate: true
  userData: |
    {{- $nodeLabels := "" -}}
    {{- range $k, $v := .Labels -}}
      {{- if $nodeLabels -}}{{- $nodeLabels = printf "%s,%s=%s" $nodeLabels $k $v -}}{{- else -}}{{- $nodeLabels = printf "%s=%s" $k $v -}}{{- end -}}
    {{- end -}}
    {{- $taints := "" -}}
    {{- range $i, $t := .Taints -}}
      {{- if $i -}}{{- $taints = printf "%s,%s=%s:%s" $taints $t.Key $t.Value $t.Effect -}}{{- else -}}{{- $taints = printf "%s=%s:%s" $t.Key $t.Value $t.Effect -}}{{- end -}}
    {{- end -}}
    {{- $systemReserved := "" -}}
    {{- range $k, $v := .Kubelet.SystemReserved -}}
      {{- if $systemReserved -}}{{- $systemReserved = printf "%s,%s=%s" $systemReserved $k $v -}}{{- else -}}{{- $systemReserved = printf "%s=%s" $k $v -}}{{- end -}}
    {{- end -}}
    {{- $kubeReserved := "" -}}
    {{- range $k, $v := .Kubelet.KubeReserved -}}
      {{- if $kubeReserved -}}{{- $kubeReserved = printf "%s,%s=%s" $kubeReserved $k $v -}}{{- else -}}{{- $kubeReserved = printf "%s=%s" $k $v -}}{{- end -}}
    {{- end -}}
    {{- $evictionHard := "" -}}
    {{- range $k, $v := .Kubelet.EvictionHard -}}
      {{- if $evictionHard -}}{{- $evictionHard = printf "%s,%s<%s" $evictionHard $k $v -}}{{- else -}}{{- $evictionHard = printf "%s<%s" $k $v -}}{{- end -}}
    {{- end -}}
    MIME-Version: 1.0
    Content-Type: multipart/mixed; boundary="//"

    --//
    Content-Type: text/x-shellscript; charset="us-ascii"

    #!/bin/bash
    echo "custom setup here"
    --//
    Content-Type: text/x-shellscript; charset="us-ascii"

    #!/bin/bash -xe
    exec > >(tee /var/log/user-data.log|logger -t user-data -s 2>/dev/console) 2>&1
    /etc/eks/bootstrap.sh '{{ .ClusterName }}' \
      --apiserver-endpoint '{{ .ClusterEndpoint }}' \
      --b64-cluster-ca '{{ .CABundle }}' \
      --ip-family ipv6 \
      {{ with .Kubelet.ClusterDNS }}--dns-cluster-ip '{{ index . 0 }}' \{{ end }}
      --use-max-pods false \
      --kubelet-extra-args '{{ if $nodeLabels }}--node-labels="{{ $nodeLabels }}" {{ end }}--register-with-taints="{{ $taints }}" --max-pods={{ .Kubelet.MaxPods }} --pods-per-core={{ .Kubelet.PodsPerCore }} --system-reserved="{{ $systemReserved }}" --kube-reserved="{{ $kubeReserved }}" --eviction-hard="{{ $evictionHard }}" --image-gc-high-threshold={{ .Kubelet.ImageGCHighThresholdPercent }} --image-gc-low-threshold={{ .Kubelet.ImageGCLowThresholdPercent }} --cpu-cfs-quota={{ .Kubelet.CPUCFSQuota }}' \
      {{ if eq .InstanceStorePolicy "RAID0" }}--local-disks raid0{{ end }}
    --//--
```

## Goals

* Let users of any AMI family reference Karpenter-computed node/cluster values inside their `spec.userData` using
  Go's `text/template` `{{ }}` syntax.
* Define a set of template variables, populated from values Karpenter already computes
  identically for every AMI family, giving `Custom` `userData` access to the same Karpenter-provided data standard
  AMI families already have.
* Ship as an alpha, opt-in feature, off by default, with no change in behavior until explicitly enabled.
* Preserve today's drift behavior exactly: drift continues to compare the literal `spec.userData` template
  source, not a rendered value — matching existing precedent for `spec.kubelet`.

## Non-Goals

* **No user-defined template variables.** Unlike [#8357](https://github.com/aws/karpenter-provider-aws/pull/8357)'s
  `userDataVariables` field, this RFC does not let users define their own named values;
  the only data available is the fixed set in [Template Context](#template-context).
* **No drift-exemption mechanism** — tracked separately in [#8336](https://github.com/aws/karpenter-provider-aws/issues/8336).
* **No custom template helper functions** (e.g. a Sprig-style function library). Only Go's built-in
  `text/template` functions are available; adding a helper function set can be considered separately later.

## Solution: New `userDataTemplate` boolean field on EC2NodeClass

Add a new field, `spec.userDataTemplate`, of type `bool`, defaulting to `false`. When `true`, the existing
`spec.userData` field is rendered as a Go template before any AMI-family-specific handling; when `false` (or
unset), `userData` is used exactly as today, byte for byte. There is only ever one field holding content —
`userData` — the new field is a per-`EC2NodeClass` switch on how that content is interpreted.

**Pros:** no new content field — `userData` remains the single place to put user-data content, so there's nothing
to migrate and no second field whose relationship to the first a user must learn. The opt-in is explicit and
per-`EC2NodeClass`, not cluster-wide, so an unrelated NodeClass is never affected by another one enabling this.

**Cons:** two fields (`userData`, `userDataTemplate`) together determine behavior instead of one, so reading a
NodeClass in isolation requires checking both.

## Alternatives Considered

### Separate `userDataTemplate` string field holding the template content

Add a new field, `spec.userDataTemplate`, of type `string`, holding the template source itself — mutually
exclusive with `spec.userData`. When set, its content, instead of `userData`, is rendered as a Go template before
any AMI-family-specific handling.

**Cons:** two fields can hold user-data content instead of one, so the nodeclass validation controller must enforce
that only one of `userData`/`userDataTemplate` is ever set — a new failure mode the chosen boolean-flag design
doesn't have, since there's only ever one content field.

**Why not chosen:** the added validation (enforcing mutual exclusivity) outweighs the small readability benefit;
the boolean-flag design keeps `userData` as the single, permanent home for content and only changes how that
content is interpreted.

### Feature gate enables templating on the existing `userData` field

Add a new AWS-specific feature gate, e.g. `NodeClassUserDataTemplate`: off by default, configured via the existing AWS
feature-gates flag/environment-variable/Helm value.
When enabled, Karpenter renders the user-supplied `userData` through `text/template` — using the variables in
[Template Context](#template-context) — once, before any AMI-family-specific handling of that string.
This applies uniformly across all AMI families, since every family merges the same user-supplied string; the gate
is cluster-wide, not per-`amiFamily`.
When disabled (the default), `userData` is used exactly as today, byte for byte.

**Validation:** the CRD schema cannot gate a plain string:
the nodeclass validation controller sets `ValidationSucceeded=False` with a dedicated reason if `spec.userData`
contains template syntax while the gate is disabled, rather than silently launching a node with literal
`{{ .ClusterEndpoint }}` baked into its boot script.

**Pros:** no new CRD field or migration burden — an existing NodeClass is unaffected until an operator opts in.

**Cons:** the gate is cluster-wide, not per-`EC2NodeClass`; a user with pre-existing literal `{{ ... }}` text in
`userData` for an unrelated reason will have it reinterpreted the moment the gate is enabled — unlike the chosen
solution, this can surprise a NodeClass whose owner never touched it.

**Why not chosen:** the cluster-wide blast radius outweighs the small benefit of not adding a new field.

### CEL expressions instead of Go templates

Reuse CEL — already used for `spec.kubelet`'s [`NodeClassCEL`](./dynamic-kubelet-configuration-via-expressions.md)
expressions — to render `userData` instead of `text/template`, evaluating it once as a single CEL string expression
with the same [Template Context](#template-context) values exposed as CEL variables.

Unlike `text/template`, CEL has no notion of a literal text body with `{{ }}` interpolation spliced in: the entire
`userData` value must itself be one CEL string expression, so every literal fragment of the file becomes a quoted
string literal joined with `+`, and every newline must be spelled out as `\n`:

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: custom
spec:
  amiFamily: Custom
  amiSelectorTerms:
    - id: ami-0123456789abcdef0
  kubelet:
    maxPods: "((default_enis - 1) * (ips_per_eni - 1)) + 2"
    kubeReserved:
      cpu: "max(60, vcpus * 30)"
  userDataTemplate: true
  userData: >
    "#cloud-config\n" +
    "write_files:\n" +
    "  - path: /etc/kubernetes/environment\n" +
    "    content: |\n" +
    "      CLUSTER_NAME='" + clusterName + "'\n" +
    "      CLUSTER_ENDPOINT='" + clusterEndpoint + "'\n" +
    "      CA_BUNDLE='" + caBundle + "'\n" +
    "      MAX_PODS='" + string(kubelet.maxPods) + "'\n" +
    "      KUBE_RESERVED_CPU='" + kubelet.kubeReserved['cpu'] + "'\n"
```

**Pros:** one expression language across the whole `EC2NodeClass` (`spec.kubelet` already uses CEL), rather than
introducing `text/template` as a second one; CEL's compile-time type checking would also catch some type errors
`text/template` only fails on at render time.

**Cons:** cumbersome and hard to read for anything beyond a one-line substitution — a multi-line `userData` body
becomes a wall of quoted, concatenated string literals rather than a natural block scalar with a few `{{ }}`
placeholders, and every quote character already present in the body (shell single-quotes, YAML syntax) adds a
layer of escaping/nesting risk on top.

**Why not chosen:** `text/template`'s design — literal text as the default, with `{{ }}` as the only escape into
expression syntax — is the better fit for a mostly-literal, multi-line body like `userData`.

## Template Context

The template's data argument (`.` in `text/template`) is a new, explicit set of template variables:
Karpenter's internal representation stays free to be refactored without silently breaking every template in the fleet —
the template variables are the versioned, stable contract, its internals are not.

Optional values Karpenter holds internally as pointers are converted to their plain, non-pointer type, defaulting
to the zero value when absent, rather than passed through as a pointer — this avoids a common Go template footgun
where printing a nil pointer renders the literal text `<nil>`, and makes the value work directly with
`{{ if ... }}`.

| Variable | Type | Example usage |
|---|---|---|
| Cluster name | `string` | `{{ .ClusterName }}` |
| Cluster API server endpoint | `string` | `{{ .ClusterEndpoint }}` |
| Cluster CIDR | `string`, empty when absent | `{{ .ClusterCIDR }}` |
| Cluster CA bundle | `string`, empty when absent | `{{ .CABundle }}` |
| Node labels | `map[string]string`, empty when absent | `{{ range $k, $v := .Labels }}...{{ end }}` |
| Node taints | `[]corev1.Taint`, empty when absent | `{{ range .Taints }}{{ .Key }}={{ .Value }}:{{ .Effect }}{{ end }}` |
| Kubelet cluster DNS | `[]string`, empty when absent | `{{ index .Kubelet.ClusterDNS 0 }}` |
| Kubelet max pods | `*intstr.IntOrString`, nil when absent | `{{ .Kubelet.MaxPods }}` |
| Kubelet pods per core | `*int32`, nil when absent | `{{ .Kubelet.PodsPerCore }}` |
| Kubelet system reserved | `map[string]string`, empty when absent | `{{ index .Kubelet.SystemReserved "memory" }}` |
| Kubelet kube reserved | `map[string]string`, empty when absent | `{{ index .Kubelet.KubeReserved "cpu" }}` |
| Kubelet eviction hard | `map[string]string`, empty when absent | `{{ index .Kubelet.EvictionHard "memory.available" }}` |
| Kubelet eviction soft | `map[string]string`, empty when absent | `{{ index .Kubelet.EvictionSoft "memory.available" }}` |
| Kubelet eviction soft grace period | `map[string]metav1.Duration`, empty when absent | `{{ index .Kubelet.EvictionSoftGracePeriod "memory.available" }}` |
| Kubelet eviction max pod grace period | `*int32`, nil when absent | `{{ .Kubelet.EvictionMaxPodGracePeriod }}` |
| Kubelet image GC high threshold percent | `*int32`, nil when absent | `{{ .Kubelet.ImageGCHighThresholdPercent }}` |
| Kubelet image GC low threshold percent | `*int32`, nil when absent | `{{ .Kubelet.ImageGCLowThresholdPercent }}` |
| Kubelet CPU CFS quota | `*bool`, nil when absent | `{{ .Kubelet.CPUCFSQuota }}` |
| Instance store policy | `string`, empty when absent | `{{ .InstanceStorePolicy }}` |

`Kubelet`'s own optional fields (e.g. `MaxPods`, unset unless `spec.kubelet` configures it) keep their normal
pointer semantics, so templates referencing them should still guard with `{{ if .Kubelet.MaxPods }}...{{ end }}`.

## Considerations

* **Does your change introduce new APIs?** Yes — a new `spec.userDataTemplate` boolean field on `EC2NodeClass`;
  when `true`, the existing `spec.userData` field is rendered as a Go template regardless of `amiFamily`.
* **Does your change behave differently with different cloud providers?** No — confined to `EC2NodeClass`.
* **Does your change expose details users may rely on?** Yes — once past alpha, the template variables become a
  supported contract; adding one is safe, removing or renaming one is breaking.
* **Does your change have a risk of breaking an undocumented invariant?** Yes, narrowly — `spec.userData` today is
  treated as opaque, passed through unmodified aside from base64 encoding and AMI-family merging. This RFC changes
  that invariant, but only for a NodeClass that explicitly sets `userDataTemplate: true`; a NodeClass that never
  sets the new field keeps today's behavior exactly.
* **Does your change impact performance?** Negligible — an in-memory string operation performed once per launch
  template generation, alongside `userData` construction that already happens on that path today.
* **Does your change introduce a security concern?** No new disclosure — the Template Context is limited to values
  Karpenter already passes into the generated bootstrap of standard AMI families (cluster endpoint, CA bundle,
  labels, taints, kubelet config) and templating makes these values available in `userData` for every AMI family.
* **Does your change conflict with other tooling?** Yes — if the `EC2NodeClass` manifest is itself rendered through
  a Go `text/template` engine (e.g. Helm), users must escape the `{{ }}` delimiters meant for Karpenter so the
  outer engine emits them literally, e.g. `KUBE_RESERVED_CPU='{{ "{{" }} index .Kubelet.KubeReserved "cpu" {{ "}}" }}'`.
