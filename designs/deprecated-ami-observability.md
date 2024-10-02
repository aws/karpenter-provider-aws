# Observability for Deprecated AMIs

## Background

A new feature was recently introduced with [PR #6500](https://github.com/aws/karpenter-provider-aws/pull/6500) that enables Karpenter to identify and utilize deprecated AMIs. This enhancement ensures that Karpenter can continue provisioning new nodes in production environments where users have pinned specific AMI IDs according to these [guidelines](https://karpenter.sh/docs/tasks/managing-amis/#option-2-lock-down-which-amis-are-selected). Without this feature, if an AMI specified in an EC2NodeClass becomes deprecated, Karpenter would be unable to launch new nodes, potentially leading to a service disruption in scenarios where auto-scaling based on HPAs is required.

This new feature would also benefit from enhanced observability, allowing users to identify which EC2NodeClasses are utilizing deprecated AMIs. With visibility into this data, users can proactively address the use of deprecated AMIs or plan their next steps without being forced into a constant cycle of upgrading AMIs in production to avoid outages.

## Solutions

There are multiple possible solutions that can be implemented to add observability into usage of deprecated AMIs by EC2NodeClasses, 3 of which are outlined below:

1. Introduce a new field `deprecated` in the `status.ami` object of the EC2NodeClass - example

``` yaml
status:
  amis:
    - id: ami-01234567890123456
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

2. Introduce a new status condition for EC2NodeClass with a reason `AMIsDeprecated` - example

``` yaml
status:
  amis:
  - id: ami-054c1b6d4be926123
    name: amazon-eks-node-1.29
    requirements:
    - key: kubernetes.io/arch
      operator: In
      values:
      - amd64
  - id: ami-0d20e6af81d7ce999
    name: amazon-eks-arm64-node-1.29
    requirements:
    - key: kubernetes.io/arch
      operator: In
      values:
      - arm64
  conditions:
  - lastTransitionTime: "2024-09-09T04:32:55Z"
    message: AMISelector matched deprecated AMIs ami-054c1b6d4be926123, ami-0d20e6af81d7ce999
    reason: AMIsDeprecated
    status: "True"
    type: AMIsReady
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

3. Add both options from #1 and #2 i.e an update to the `status.ami` and status condition of the EC2NodeClass.

## Recommendation

Solution #2 would be the prefered solution since this would not warrant a change to the CRDs and would be transparent to the users during Karpenter upgrade. However, solution #1 would need a CRD change and would require users to update CRDs along with Karpenter.