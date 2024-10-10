# Observability for Deprecated AMIs

## Background

With the recent introduction of a significant feature through [PR #6500](https://github.com/aws/karpenter-provider-aws/pull/6500) Karpenter has enhanced its capability to identify and utilize deprecated Amazon Machine Images (AMIs). Karpenter remains effective in provisioning new nodes within production environments where specific AMI IDs are mandated, adhering to these [guidelines](https://karpenter.sh/docs/tasks/managing-amis/#option-2-lock-down-which-amis-are-selected) or when discovering AMIs based on `AMISelectorTerms`. Previously, if an AMI designated in an EC2NodeClass was deprecated, Karpenter faced challenges in launching new nodes, which could lead to potential service interruptions, especially in cases necessitating auto-scaling driven by Horizontal Pod Autoscalers (HPAs).

This new feature would also benefit from enhanced observability, allowing cluster admins to identify which EC2NodeClasses are utilizing deprecated AMIs and take action accordingly.

## Options

### Option 1: Update the EC2NodeClass CRD

This approach will modify the current CRD for the EC2NodeClass by adding a new `deprecated` field to the `status.amis` section, providing a clear and immediate indication of AMI deprecation directly within the resource configuration.

#### Code Definition

[`pkg/apis/v1/ec2nodeclass_status.go`](../pkg/apis/v1/ec2nodeclass_status.go#L53)

```go
type AMI struct {
    // ID of the AMI
    // +required
    ID string `json:"id"`
    // Deprecation status of the AMI
    // +optional
    Deprecated bool `json:"deprecated,omitempty"`
    // Name of the AMI
    // +optional
    Name string `json:"name,omitempty"`
    // Requirements of the AMI to be utilized on an instance type
    // +required
    Requirements []corev1.NodeSelectorRequirement `json:"requirements"`
}
```

#### Proposed Spec

``` yaml
status:
  amis:
    - id: ami-01234567890654321
      name: custom-ami-amd64
      deprecated: true
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values:
            - amd64
    - id: ami-01234567890123456
      name: custom-ami-arm64
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values:
            - arm64
```

#### Pros + Cons

* 👍 This will provide cluster admins a clear, concise indication of which AMIs being utilized by the EC2NodeClass are deprecated.
* 👎 This will add a dependency to require a CRD update for the EC2NodeClass along with the version bump for Karpenter.

### Option 2: Add new status conditions to the EC2NodeClass CRD

This is an alternate approach to update the status conditions for the EC2NodeClass to provide information to cluster admins that deprecated AMIs were discovered as part of the `amiSelectorTerms`.

#### Code Definition

[`pkg/apis/v1/ec2nodeclass_status.go`](../pkg/apis/v1/ec2nodeclass_status.go#L22)

``` go

const (
    ConditionTypeSubnetsReady         = "SubnetsReady"
    ConditionTypeSecurityGroupsReady  = "SecurityGroupsReady"
    ConditionTypeAMIsReady            = "AMIsReady"
    ConditionTypeAMIsDeprecated       = "AMIsDeprecated"
    ConditionTypeInstanceProfileReady = "InstanceProfileReady"
)

```

#### Proposed Spec

``` yaml
status:
  amis:
  - id: ami-01234567890654321
    name: amazon-eks-node-1.29
    requirements:
    - key: kubernetes.io/arch
      operator: In
      values:
      - amd64
  - id: ami-01234567890123456
    name: amazon-eks-arm64-node-1.29
    requirements:
    - key: kubernetes.io/arch
      operator: In
      values:
      - arm64
  conditions:
  - lastTransitionTime: "2024-09-09T04:32:55Z"
    message: ""
    reason: AMIsDeprecated
    status: "True"
    type: AMIsDeprecated
  - lastTransitionTime: "2024-09-09T04:32:55Z"
    message: ""
    reason: InstanceProfileReady
    status: "True"
    type: InstanceProfileReady
  - lastTransitionTime: "2024-09-09T04:32:55Z"
    message: ""
    reason: Ready
    status: "True"
    type: Ready
  - lastTransitionTime: "2024-09-09T04:32:55Z"
    message: ""
    reason: SecurityGroupsReady
    status: "True"
    type: SecurityGroupsReady
  - lastTransitionTime: "2024-09-09T04:32:55Z"
    message: ""
    reason: SubnetsReady
    status: "True"
    type: SubnetsReady
```

#### Pros + Cons

* 👍 This change will not warrant a change to the CRD for the EC2NodeClass, making the Karpenter upgrade seamless without CRD update dependencies
* 👍 This can provide an metric out of the box `operator_status_condition_count` which can be used to check which `EC2NodeClass` are using deprecated AMIs
* 👎 This may cause confusion and indicate to cluster admins that a EC2NodeClass is using deprecated AMIs, when in reality it could be that "one of the AMIs discovered are deprecated".
* 👎 Users will have to do the leg work to figure out which AMIs from the discovered AMIs for the EC2NodeClass are deprecated.


### Option 3: Best of both worlds

The final approach would be a combination of both i.e updates to the EC2NodeClass CRD as well as updates to the status condition combining both the approaches mentioned above.

#### Proposed Spec

``` yaml
status:
  amis:
  - id: ami-01234567890654321
    name: amazon-eks-node-1.29
    deprecated: true
    requirements:
    - key: kubernetes.io/arch
      operator: In
      values:
      - amd64
  - id: ami-01234567890123456
    name: amazon-eks-arm64-node-1.29
    requirements:
    - key: kubernetes.io/arch
      operator: In
      values:
      - arm64
  conditions:
  - lastTransitionTime: "2024-09-09T04:32:55Z"
    message: ""
    reason: AMIsDeprecated
    status: "True"
    type: AMIsDeprecated
  - lastTransitionTime: "2024-09-09T04:32:55Z"
    message: ""
    reason: InstanceProfileReady
    status: "True"
    type: InstanceProfileReady
  - lastTransitionTime: "2024-09-09T04:32:55Z"
    message: ""
    reason: Ready
    status: "True"
    type: Ready
  - lastTransitionTime: "2024-09-09T04:32:55Z"
    message: ""
    reason: SecurityGroupsReady
    status: "True"
    type: SecurityGroupsReady
  - lastTransitionTime: "2024-09-09T04:32:55Z"
    message: ""
    reason: SubnetsReady
    status: "True"
    type: SubnetsReady
```

## Recommendation

Based on the above approaches, the preferred solution would be to leverage [Option 3](#option-3-best-of-both-worlds)
