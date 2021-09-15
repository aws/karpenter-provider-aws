---
title: "Provisioner CRD"
linkTitle: "Provisioner CRD"
weight: 40
date: 2017-01-05
---

## Example Provisioner Resource

```yaml
apiVersion: karpenter.sh/v1alpha3
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

  # Provisioned nodes will have these labels
  # Additional labels may be applied, e.g.: karpenter.sh/provisioner-name: default
  labels:
    foo: bar

  # Constrain instance types, or choose from all if unconstrained (recommended)
  # Overriden by pod.spec.nodeSelector["kubernetes.io/instance-type"]
  instanceTypes: ["m5.large", "m5.2xlarge"]

  # Constrain zones, or choose from all if unconstrained (recommended)
  # Overriden by pod.spec.nodeSelector["topology.kubernetes.io/zone"]
  zones: [ "us-west-2a", "us-west-2b" ]

  # Constrain architectures, or use choose from all if unconstrained (recommended)
  # Overriden by pod.spec.nodeSelector["kubernetes.io/arch"]
  architectures: [ "linux" ]

  # Constrain operating systems, or use choose from all if unconstrained (recommended)
  # Overriden by pod.spec.nodeSelector["kubernetes.io/os"]
  operatingSystems: [ "amd64" ]

  # These fields vary per cloud provider, see your cloud provider specific documentation
  provider: {}
```
