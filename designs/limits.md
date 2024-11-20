# Karpenter Limits

This document proposes an approach to limit the scaling that Karpenter will perform in order to control customer costs.

## Problem

Karpenter responds to unschedulable pods stuck in the pending status by launching new worker nodes that will satisfy its constraints.

However, in certain situations new worker nodes may not be able to satisfy these pending pods. Consider the situation where the AMI being used to create the worker nodes is buggy, which prevents the kubelet from ever spinning up in a healthy state. As a result new worker nodes continue to remain in the `NotReady` state and the node lifecycle controller will eventually taint the worker node after a default wait of 5 minutes leading to the already bound pods getting stuck in `Terminating`. Karpenter won't terminate these nodes since they're not empty. When using a deployment, new replacement pods will be spun up which become stuck in `Pending` and the cycle repeats leading to new worker nodes being launched every 5 minutes indefinitely. Similar scenarios can be replicated by forcing worker nodes to be created in a subnet with a bad route table. For the remainder of this document, I'll refer to this as the `runaway-scaling` problem.

The next large problem is the inability to define a hard ceiling on cluster costs. When using AWS, the common pattern for setting up your infrastructure-level scaling is using EC2 AutoScaling groups with Cluster Autoscaler (CA). Typically, by limiting the maximum size of an autoscaling group we can enforce a ceiling on how expensive your cluster can get since CA will honor the bounds specified by the auto scaling group.

We need to provide similar functionality via Karpenter as well wherein there's a hard limit a customer can configure.

## Current State

To address the runaway-scaling problem the current fix in place is to detect if the kubelet for a worker node has never reported its status to the K8s control plane. If it's been longer than 15 minutes, Karpenter assumes that there's a hard failure mode due to which this worker node will never become healthy and terminates the worker node. If the condition map of the node object in the API Server says `NodeStatusNeverUpdated` then we use that as an indicator of the node having never come up.

This fix ensures that if there are other scenarios where a worker node has become unhealthy due to a network partition or power outage in a availability zone, we don't terminate those worker nodes. It's important we don't make the static stability of a cluster worse during such an event. On the other hand, if there is an edge case where worker nodes come online and soon go offline, it will lead to runaway scaling again. This edge case should be unlikely to happen in the near term, so this document focuses on just the ability to limit costs within Karpenter. That way even if runaway scaling does occur there's a way to bound it. A longer-term solution to handle the runaway problem will be discussed separately.

## Proposed Solution for Limits

There are two broad forms of limiting we could apply. The first is that we could introduce a limit to the number of in-flight worker node being provisioned at a point in time. A worker node that's in the `NotReady` state could be considered to be in-flight. The second form is an absolute limit of the number of resources Karpenter can provision.

### **In-flight limit**

```yaml
spec:
  limits:
    unready: 20% # StringOrInt
```

In the above example - `20%` indicates that if at any point in time, more than 20% (rounded up) of all worker nodes in the cluster are in an unready state, then the provisioner will stop scaling up.

The good bit about this approach is that we don't constrain how many total worker nodes can be spun up by Karpenter, while also making sure that if we keep launching worker nodes that aren't healthy, we stop the scaling and save costs.

The two main problems with this approach though are -

1. This limit while meant to just constrain the number of unhealthy worker nodes in a cluster, will also inhibit the rate at which Karpenter can respond to pods that aren't schedulable. This somewhat goes against the goal of minimizing launch times of workers.
2. While this helps ensure that costs don't increase due to runaway scaling, it won't help those who want a stricter cap on the amount of resources that's being provisioned even when nodes are otherwise healthy.

### **Absolute limit**

This is a much simpler semantic to follow, where you provide an upper cap on the amount of resources being provisioned irrespective of their health. This is more in-line with alternative solutions like CA and should be easier to reason with. This is **the approach I recommend**.

The actual limit of the number of resources being spun up can be defined either through something similar to resource requests or just a flat count of instances.

```yaml
spec:
  limits:
    resources:
      cpu: 1000
      memory: 1000Gi
      nvidia.com/gpu: 1
```

Modeling your limit via CPU and Memory constraints aligns well with customers that use resource requests with their pods. This should give everyone a consistent interface to define the capacity requirements of their applications as well as their underlying infrastructure. There shouldn't be a need to think of the actual number and type of instances that are being chosen by Karpenter.

As a cost control mechanism, this requires a little more work from our users if they're looking for higher precision. Since some cloud providers like AWS charge differently per instance type rather than the actual resource request and consumption, a little more translation will be needed to estimate costs. There are other factors like reserved instances (varying duration / purchased up-front) which make the cost estimation even more tricky and therefore a simple aggregate limit will be more usable. The alternative of a flat count of instance types - say `spec.limits.resources.count`, doesn't help solve that problem either since the instance types of differing prices are chosen by Karpenter by default. In case overrides are defined via `kubernetes.io/instance-type`, you should be able to calculate the CPU and Memory bounds fairly trivially through some cloud provider API calls.

[CPU limits](https://kubernetes.io/docs/tasks/configure-pod-container/assign-cpu-resource/#cpu-units), memory limits and GPU limits will be defined similar to resource requests and will not be required by default. Karpenter will also will not default to any limits itself.

The list of supported resource types is -

- `cpu`
- `memory`
- `nvidia.com/gpu`
- `amd.com/gpu`
- `aws.amazon.com/neuron`
- `aws.amazon.com/neuroncore`
- `habana.ai/gaudi`

Limits will be defined at the per-provisioner level. We'll rely on the `karpenter.sh/provisioner-name` node label when calculating resource usage by a specific provisioner. This is useful when multiple teams share a single cluster and use separate provisioners since each team's resource consumption will be limited separately.

A global cluster-wide limit for all resources could be configured too. However, since we only expect a finite list of provisioners in the cluster, inferring the global limit from a sum of provisioner specific limits shouldn't be difficult for a cluster administrator to do either. I think this is another knob we should consider adding only if we find other compelling use cases.
