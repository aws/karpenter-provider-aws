# AMI Selector Terms Scheduling Requirements

## Context and Problem Statement

`EC2NodeClass` `amiSelectorTerms` allows Karpenter to select AMIs based on `tags`, `id`, `name`, `ssmParameter`, or
an `alias`. Multiple AMIs may be specified, and Karpenter will choose the newest compatible AMI when spinning up new
Nodes.

Karpenter's `alias` mechanism for AMI selection automatically discovers AMI variants (e.g., NVIDIA, Neuron) and
injects their corresponding scheduling requirements into the `EC2NodeClass` status. This ensures that Nodes are
provisioned with AMIs compatible with the workload's hardware needs.

```yaml
status:
  amis:
    - id: ami-0bc680bafd3cdf722
      name: bottlerocket-aws-k8s-1.31-nvidia-x86_64-v1.42.0-5ed15786
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values:
            - amd64
        - key: karpenter.k8s.aws/instance-gpu-count
          operator: Exists
    - id: ami-023d869730e37c4a9
      name: bottlerocket-aws-k8s-1.31-x86_64-v1.42.0-5ed15786
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values:
            - amd64
        - key: karpenter.k8s.aws/instance-gpu-count
          operator: DoesNotExist
        - key: karpenter.k8s.aws/instance-accelerator-count
          operator: DoesNotExist
```

When an AMI is specified using a direct selector such as `ssmParameter` or `id`, Karpenter does not currently apply
similar logic. This creates a feature gap for users who manage custom AMIs, a common practice in production environments
for reasons of security, compliance, or tooling standardization. For these AMIs, which may have specific hardware or
software requirements (e.g., GPU drivers, licensed software, specific kernel modules), there is no mechanism to
associate them with their necessary scheduling constraints at the `EC2NodeClass` level.

This limitation forces the responsibility of AMI-specific scheduling onto the `NodePool` configuration, coupling
concerns that could be separate. The result is increased configuration complexity and a higher risk of scheduling errors
due to misaligned constraints between the `NodePool` and the underlying AMIs its `EC2NodeClass` may resolve.

The following considerations were suggested, but are intentionally excluded from the scope of this decision:

* *Selector Term Uniqueness Enforcement*: While it is desirable to ensure that no two `amiSelectorTerms` with
  overlapping nodeRequirements match the same instance type (to avoid ambiguous AMI selection), this constraint is not
  currently enforced for non-alias selectors and is considered an orthogonal concern. This design document does not
  introduce new validation logic for this case but acknowledges it as a potential area for future improvement.
  When multiple AMIs discovered through `amiSelectorTerms` are compatible with a given instance type, Karpenter
  can continue to select the newest compatible AMI based on its creation date.

## Decision Drivers

1. The solution MUST not break existing `amiSelectorTerms` configurations or require schema changes that would
   invalidate current manifests, without introducing a new API version.

2. The solution SHOULD be extensible to support the introduction of new `amiSelectorTerms` in the future.

3. The configuration SHOULD remain consistent with Karpenter's existing selector semantics.

## Considered Options

* [Option 1: Introducing `nodeRequirements` Field to `amiSelectorTerms`](#introducing-noderequirements-field-to-amiselectorterms)
* [Option 2: Nesting the AMI Selector and Node Requirements Separately](#nesting-the-ami-selector-and-node-requirements-separately)
* [Option 3: SSM-Encoded Requirements](#ssm-encoded-requirements)
* [Option 4: A Combination of 1 and 2](#a-combination-of-1-and-2)
* [Option 5: A Combination of 1 and 3](#a-combination-of-1-and-3)
* [Option 6: A Combination of 1, 2, and 3](#a-combination-of-1-2-and-3)

### Introducing `nodeRequirements` Field to `amiSelectorTerms`

Enhance the `amiSelectorTerms` API to allow for the explicit definition of scheduling requirements alongside the AMI
selector itself.

A conceptual implementation could look like:

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: default
spec:
  amiFamily: Custom
  amiSelectorTerms:
    # Standard AMI with no special requirements
    - name: my-ami
      owner: self

    # An ML-focused AMI that requires a GPU instance
    - ssmParameter: /my-org/amis/custom-ml-drivers
      requirements:
        - key: "karpenter.k8s.aws/instance-gpu-count"
          operator: "Exists"
```

The specified requirements would be merged with any constraints that can be statically inferred from the AMI itself
(e.g., architecture). For instance, given the above configuration, the `EC2NodeClass` status might resolve to:

```yaml
status:
  amis:
    - id: ami-0bc680bafd3cdf722
      name: bottlerocket-aws-k8s-1.31-nvidia-x86_64-v1.42.0-5ed15786
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values:
            - amd64
        - key: karpenter.k8s.aws/instance-gpu-count
          operator: Exists
```

In cases where a requirement (e.g., `kubernetes.io/arch`) is specified both in the user-defined requirements and
inferred from the AMI itself, the inferred value should take precedence. This prevents misconfigurations, such
as specifying `kubernetes.io/arch: amd64` for an arm64 AMI.

> [!NOTE]
> We chose the field name `requirements` to align with the existing `status.amis[*].requirements` field on the
> `EC2NodeClass`. `nodeRequirements` was also considered for its explicitness.

* **Pro**: The association between an AMI and its requirements is declarative and self-contained.
* **Pro**: Avoids a breaking change to the `EC2NodeClass` schema (e.g., introducing a nested selector field).
* **Con**: Slight semantic overload: `amiSelectorTerms` now includes both AMI selection and scheduling logic, which
  may blur conceptual boundaries.
    * On one hand, the term now includes both AMI selection logic and node scheduling constraints, which may blur the
      conceptual boundary between "*what AMI to use*" and "*under what conditions it can be used*".
    * On the other hand, while `requirements` is not a direct AMI selector, it meaningfully contributes to the AMI
      selection process by constraining the set of instance types that are compatible with a given AMI. Karpenter
      already selects AMIs based on instance compatibility and recency. These requirements act as an additional filter
      on the AMI's applicability in a given scheduling context. Today, except alias-based terms, these constraints are
      limited to what can be statically inferred from the AMI itself (e.g., architecture). Therefore, including
      requirements as a field within an `amiSelectorTerm` is consistent with its purpose: to define the **AND**ed
      conditions under which an AMI is valid for use.

### Nesting the AMI Selector and Node Requirements Separately

This option proposes introducing a nested structure to separate AMI selection from scheduling constraints.

```yaml
amiSelectorTerms:
  - selector:
      ssmParameter: "/my-org/amis/custom-ml-drivers"
    requirements:
      - key: kubernetes.io/arch
        operator: In
        values: [ 'amd64' ]
      - key: "karpenter.k8s.aws/instance-gpu-count"
        operator: "Exists"
```

* **Pro**: The association between an AMI and its requirements is declarative and self-contained.
* **Pro**: Clear separation of concerns between AMI selection and scheduling logic.
* **Con**: Breaking change to the `EC2NodeClass` schema. Migration could be handled via a new API version or an opt-in
  feature gate.

### SSM-Encoded Requirements

This solution suggests embedding scheduling requirements directly into the SSM Parameter Store parameter associated with
an AMI and/or associating requirements to the parameter with tags.

Given an SSM parameter (e.g. `/my-org/amis/custom-ml-drivers`)

One could then encode scheduling requirements into the SSM parameter as a JSON string, for example:

```
{"id":"ami-02211404f5aa0b76a","requirements":[{"key":"kubernetes.io/arch","operator":"In","values":["arm64"]},{"key":"karpenter.k8s.aws/instance-gpu-count","operator":"DoesNotExist"},{"key":"karpenter.k8s.aws/instance-accelerator-count","operator":"DoesNotExist"}]}
```

<details>
<summary>Click to see formatted JSON...</summary>
<pre><code>
{
    "id": "ami-02211404f5aa0b76a",
    "requirements": [
        {
            "key": "kubernetes.io/arch",
            "operator": "In",
            "values": [
                "arm64"
            ]
        },
        {
            "key": "karpenter.k8s.aws/instance-gpu-count",
            "operator": "DoesNotExist"
        },
        {
            "key": "karpenter.k8s.aws/instance-accelerator-count",
            "operator": "DoesNotExist"
        }
    ]
}
</code></pre>
</details>

Karpenter would then resolve the AMI ID and the associated requirements from the SSM parameter, passing the discovered
requirements on to the resolver, similar to how it does for alias.

* **Pro**: Consistent with the existing `alias` behavior.
* **Pro**: Fills a gap in the `DescribeImages` API by allowing AMI publishers to specify arbitrary
  attributes, outside of just architecture.
* **Neutral**: Introduces new conventions around SSM parameter format when used as an `amiSelectorTerm`.
  SSM parameters for well known AMI families (e.g. `bottlerocket`) do not currently follow this proposed
  convention.
* **Con**: SSM Parameters have a maximum character limit of 2048, which imposes a hard cap on the number and
  complexity of requirements that can be encoded. While this limit is generous and unlikely to be exceeded in typical
  use cases, it may constrain more complex configurations or future extensibility.
* **Con**: Limited to requirements provided by SSM parameter owner.
* **Con**: Not extensible to other `amiSelectorTerms`.

### A Combination of 1 and 2

Support `requirements` in the current schema (Option 1), and plan for a future breaking change to adopt a nested
structure (Option 2) in a new API version.

* **Pro**: Immediate value without breaking changes.
* **Pro**: Provides a migration path to a cleaner long-term model.
* **Con**: Introduces some semantic ambiguity in the short term.

### A Combination of 1 and 3

Support `requirements` in the current schema (Option 1) and read metadata from SSM parameters (Option 3).

* **Pro**: Immediate value without breaking changes.
* **Pro**: Allows both Karpenter users and AMI publishers to specify arbitrary attributes.
* **Con**: Supporting two mechanisms adds implementation and maintenance overhead.

### A Combination of 1, 2, and 3

Support `requirements` in the current schema (Option 1), plan for a future breaking change to adopt a nested
structure (Option 2) in a new API version, and read metadata from SSM parameters (Option 3).

* **Pro**: Immediate value without breaking changes.
* **Pro**: Provides a migration path to a cleaner long-term model.
* **Pro**: Allows both Karpenter users and AMI publishers to specify arbitrary attributes.
* **Con**: Introduces some semantic ambiguity in the short term.
* **Con**: Supporting three mechanisms adds implementation and maintenance overhead.

## Recommended Solution: [Option 5&mdash;A Combination of 1 & 3](#a-combination-of-1-and-3)

This approach delivers immediate value by enabling `amiSelectorTerm` requirements for Karpenter users in
the current schema, while allowing for AMI publishers to additionally provider their own arbitrary
AMI attributes. A breaking change to introduce a more semantically distinct model version can always
be considered in a future Karpenter version, but is deemed unnecessary today.

To avoid abiguous behavior, in cases where requirements are specified both in an SSM parameter and with
NodeClass `amiSelectorTerms`, the latter should take precedence. This could be on a per-key basis, or
the whole set.

## Appendix

* [Karpenter "Managing AMIs" Documentation](https://karpenter.sh/docs/tasks/managing-amis/)
* [Karpenter "EC2NodeClass" Documentation](https://karpenter.sh/docs/concepts/nodeclasses/#specamiselectorterms)
* [Related Issue (#8265)](https://github.com/aws/karpenter-provider-aws/issues/8265)
* [Related RFC](https://github.com/aws/karpenter-provider-aws/blob/v1.6.1/designs/ami-selector.md)
