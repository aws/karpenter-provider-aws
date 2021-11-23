---
title: "Provisioner CRD"
linkTitle: "Provisioner CRD"
weight: 40
date: 2017-01-05
---

## Example Provisioner Resource

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: default
spec:
  # If nil, the feature is disabled, nodes will never expire
  ttlSecondsUntilExpired: 2592000 # 30 Days = 60 * 60 * 24 * 30 Seconds;

  # If nil, the feature is disabled, nodes will never scale down due to low utilization
  ttlSecondsAfterEmpty: 30

  # Provisioned nodes will have these taints
  # Taints may prevent pods from scheduling if they are not tolerated
  taints:
    - key: example.com/special-taint
      effect: NoSchedule

  # Labels are arbitrary key-values that are applied to all nodes
  labels:
    billing-team: my-team

  # Requirements that constrain the parameters of provisioned nodes.
  # These requirements are combined with pod.spec.affinity.nodeAffinity rules.
  # Operators { In, NotIn } are supported to enable including or excluding values
  requirements:
    - key: "node.kubernetes.io/instance-type" # If not included, all instance types are considered
      operator: In
      values: ["m5.large", "m5.2xlarge"]
    - key: "topology.kubernetes.io/zone" # If not included, all zones are considered
      operator: In
      values: ["us-west-2a", "us-west-2b"]
    - key: "kubernetes.io/arch" # If not included, all architectures are considered
      operator: In
      values: ["arm64", "amd64"]
    - key: "kubernetes.io/os" # If not included, all operating systems are considered
      operator: In
      values: ["linux"]
    - key: "karpenter.sh/capacity-type" # If not included, the webhook for the AWS cloud provider will default to on-demand
      operator: In
      values: ["spot", "on-demand"]
  # These fields vary per cloud provider, see your cloud provider specific documentation
  provider: {}
```
