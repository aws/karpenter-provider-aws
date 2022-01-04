# v1alpha4 API Proposal
This document proposes comprehensive Provisioner API improvements prior to the v0.4 release. Due to minor backwards incompatible changes, this will result in a API version bump (v1alpha3 -> v1alpha4). This is in accordance with [Kubernetes API Deprecation Policy](https://kubernetes.io/docs/reference/using-api/deprecation-policy/). This API is not considered to be the final state of the Provisioner API, and is expected to evolve further as we explore more advanced scale down (defragmentation).

These changes recommend:
1. Cloud Provider specific extensions under `spec.provider`.
2. Removal of `spec.cluster`.
3. Pluralization of `spec.architecture` and `spec.operatingSystem`.
4. Provisioning limits for maximum cpu, memory, etc, under `spec.limits`.

## Strongly Typed Vendor Specific Fields
Cloud Providers are currently limited to using well known `spec.labels` for configuration of vendor specific parameters. For example:
- `spec.labels['node.k8s.aws/launch-template-name']`
- `spec.labels['node.k8s.aws/subnet-name']`

One benefit of this approach is that pods may use corresponding `spec.nodeSelector[...]` to request additional constraints on provisioned nodes. This information must be communicated through Kubernetes label values, which is awkward for the following use cases:
1. Parameters are a list (e.g. `subnets: ["subneta", "subnetb", "subnetc"]`)
2. Parameters are a struct (e.g. `tags: { "foo" : "bar" }`)
3. Parameters do not comply with [the label value character set](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set). See: [#646](https://github.com/aws/karpenter/issues/646).

We introduce a new `provider` field that enables strongly typed vendor specific parameters without violating vendor neutrality principles in the Karpenter codebase. We leverage Kubernetes `runtime.RawExtensions` to encapsulate these fields as raw bytes, which are then unmarshaled in vendor specific code. Vendors may implement arbitrary validation, defaulting, and provisioning behavior over the entire structure of these extensions. These structures are versioned separately from the Provisioner GVK to enable Cloud Providers to make backwards incompatible changes to provider specific configuration without requiring a version bump to the Provisioner CRD. If versioning is not specified by the user, it will be inferred and defaulted.

```yaml
apiVersion: karpenter.sh/v1alpha4
kind: Provisioner
spec:
  provider:
    apiVersion: extensions.karpenter.sh/v1alpha1
    kind: AWS
    securityGroups: ["abc", "def"]
    subnets: ["123", "456"]
    launchTemplateName: "foo"
```

The Cloud Provider *may* continue to support corresponding well known labels at the pod level (e.g. `node.k8s.aws/subnet-name`).

```yaml
# PodSpec: Simple key value constraints
spec:
  nodeSelector:
    node.k8s.aws/subnet-name: "123" # Vendor Specific Field
    kubernetes.io/instance-type: "m5.large" # Vendor Neutral Field
```

```yaml
# PodSpec: More expressive than node selectors, can specify multiple or preferences
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: node.k8s.aws/subnet-name
            operator: In
            values: ["123", "456"]
```

*Note: Not all `spec.provider` fields must have a corresponding well known label.*

### Alternative: Inline vs Nested Parameters
It's possible to leverage inline json capabilities to collapse these parameters to the top level. For example
```yaml
spec:
  zones: ["us-west-2a", "us-west-2b"] # Vendor neutral
  securityGroups: ["abc", "def"] # Vendor specific, but sibling of zones
```

1. (+) Parameters that are natural siblings (e.g. `zones`, `securityGroups`) will be no longer be separated by `provider`.
2. (-) It's not immediately clear to readers which parameters are vendor specific.
3. (-) Key name conflicts may arise between provider and vendor fields.
4. (-) `kind` and `apiVersion` are awkwardly placed at the top level of spec.

### Alternative: Duck Typing

It's possible to follow [Knative's Duck Typing approach](https://www.youtube.com/watch?v=kldVg63Utuw) and build vendor specific CRDs that contain vendor neutral snippets (or ducks) that Karpenter can recognize. Cloud Providers would consume Karpenter generic controllers as libraries, which would behave against the generic API snippets. For example:

```yaml
apiVersion: karpenter.k8s.aws/v1alpha1 # Vendor Specific
kind: Provisioner
spec:
  limits: {} # Vendor neutral, recognized by Karpenter generic controllers
  labels: {} # Vendor neutral, recognized by Karpenter generic controllers
  taints: [] # Vendor neutral, recognized by Karpenter generic controllers
  subnets: [] # Vendor specific, only recognized by AWS Cloud Provider code
```

1. (+) Versioning is only defined in one place
2. (+) Vendors may change versions decoupled from each other.
3. (+) Parameters that are natural siblings (e.g. `zones`, `securityGroups`) will be no longer be separated by `provider`.
4. (-) Cloud Provider must do more than simply implementing an interface (they must define an CR, etc).
5. (-) Potential for name collision as duck types evolve or confusion about what fields providers own.
6. (-) Knative duck typing APIs are alpha, missing documentation, and not widely adopted.

## Pluralizion for all Constraints
We expand the constraints of a Provisioner from a scalar to a vector in all cases. This will apply to both `operatingSystem` and `architecture` enabling the operator to specify greater flexibility. This supports use cases such as heterogenous architectures within a single provisioner, selected dynamically at runtime by the cloud provider. More importantly, this change creates a consistent and predictable semantic for all vendor neutral constraints. Similar to other constraints, the semantic of this change allows cloud providers to choose any value in the constraint slice, e.g. prioritizing arm64 for cost reasons.
### Example
```yaml
spec:
  operatingSystems: ["linux", "windows"] # operatingSystem -> operatingSystems
  architectures: ["amd64", "arm64"] # architecture -> architectures
```

## Limits

We introduce a new field `spec.limits` that contains configuration parameters to limit scaling and control costs.

### Example
```yaml
spec:
  limits:
    unready: 20% # Flat or Percentage. Karpenter will not launch additional capacity if current unready nodes exceeds this value
    resources: # Karpenter will not launch additional capacity if current capacity exceeds this value
      cpu: 1000
      memory: 1000Gi
```
