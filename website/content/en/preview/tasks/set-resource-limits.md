---
title: "Set Resource Limits"
linkTitle: "Set Resource Limits"
weight: 10
---

Karpenter automatically provisions instances from the cloud provider. This often incurs hard costs. To control resource utilization and cluster size, use resource limits.

The provisioner spec includes a limits section (`spec.limits.resources`), which constrains the maximum amount of resources that provisioner will manage. 

For example, setting "spec.limits.resources.cpu" to "1000" limits the provisioner to a total of 1000 CPU cores across all instances. This prevents unwanted excessive growth of a cluster. 

The [Kubernetes core API](https://github.com/kubernetes/api/blob/37748cca582229600a3599b40e9a82a951d8bbbf/core/v1/resource.go#L23) (`k8s.io/api/core/v1`) defines the `resources` which may be limited.

These resources are described with a `DecimalSI` value, usually a natural integer. 
- CPU
- Pods

These resources are described with a [`BinarySI` value, such as 1000Gi.](https://github.com/kubernetes/apimachinery/blob/4427f8f31dfbac65d3a044d0168f84c51bfda440/pkg/api/resource/quantity.go#L31)
- memory
- storage
- StorageEphemeral 

[[Question -- If Karpenter is at 995, and wants to provision a 6 core instance, will it auto step down to 4-5 cores, or will it fail at 995 of the limit?]]

### Example Provisioner:

```
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: default
spec:
  requirements:
    - key: karpenter.sh/capacity-type
      operator: In
      values: ["spot"]
  limits:
    resources:
      cpu: 1000 
      memory: 1000Gi 
      storage: 1000Ti 
      pods: 1000
      StorageEphemeral: 1000Gi
```