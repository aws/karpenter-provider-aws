---
title: "Set Resource Limits"
exclude_search: true
linkTitle: "Set Resource Limits"
weight: 10
---

Karpenter automatically provisions instances from the cloud provider. This often incurs hard costs. To control resource utilization and cluster size, use resource limits.

The provisioner spec includes a limits section (`spec.limits.resources`), which constrains the maximum amount of resources that the provisioner will manage. 

For example, setting "spec.limits.resources.cpu" to "1000" limits the provisioner to a total of 1000 CPU cores across all instances. This prevents unwanted excessive growth of a cluster. 

At this time, Karpenter only supports:
- CPU
- Memory

CPU limits are described with a `DecimalSI` value, usually a natural integer. 

Memory limits are described with a [`BinarySI` value, such as 1000Gi.](https://github.com/kubernetes/apimachinery/blob/4427f8f31dfbac65d3a044d0168f84c51bfda440/pkg/api/resource/quantity.go#L31)

You can view the current consumption of cpu and memory on your cluster by running:
```
kubectl get provisioner -o=jsonpath='{.items[0].status}'
```

Review the [Kubernetes core API](https://github.com/kubernetes/api/blob/37748cca582229600a3599b40e9a82a951d8bbbf/core/v1/resource.go#L23) (`k8s.io/api/core/v1`) for more information on `resources`.

### Implementation

Karpenter refuses to allocate new resources while at least one resource limit is *exceeded*. In other words, resource limits aren't hard limits, they only apply once Karpenter detects that a limit has been crossed.

**Example:**

A resource limit of 1000 CPUs is set. 996 CPU cores are currently allocated. The resource limit is not met.

In response to pending pods, Karpenter calculates a new 6 core instance is needed. Karpenter *creates* the instance.

1002 CPU cores are now allocated. The resource limit is in now met/exceeded. 

In response to a new set of pending pods, Karpenter calculates another 6 core instance is needed. Karpenter *does not create* the instance, because the resource limit has been met.

{{% alert title="Note" color="primary" %}}
Karpenter provisioning is highly parallel. Because of this, limit checking is eventually consistent, which can result in overrun during rapid scale outs.
{{% /alert %}}

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
```