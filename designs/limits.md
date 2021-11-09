# Karpenter Limits


This document proposes an approach to limit the scaling that Karpenter will perform, in order to control customer costs and to prevent underutilized capacity in cases where pending pods cannot be scheduled on new capacity.


## Problem

Karpenter responds to unschedulable pods stuck in the pending status by launching new worker nodes that will satisfy its constraints.

However, in certain situations new worker nodes may not be able to satisfy these pending pods. Consider the situation where the AMI being used to create the worker nodes is buggy, which prevents the kubelet from ever spinning up healthy. As a result new worker nodes continue to remain in the `NotReady` state and the node lifecycle controller will eventually taint the worker node after a default wait of 5 minutes leading to the already bound pods getting stuck in `Terminating`. Karpenter won't terminate these nodes since they're not empty. When using a deployment, new replacement pods will be spun up which become stuck in `Pending` and the cycle repeats. Similar scenarios can be replicated by forcing worker nodes to be created in a subnet with a bad route table. For the remainder of this document, I'll refer to this as the `runaway-scaling` problem.

The next large problem is the inability to define a hard ceiling on cluster costs. When using AWS, the common pattern for setting up your infrastructure-level scaling is using EC2 AutoScaling groups with Cluster Autoscaler (CA). Typically, by limiting the maximum size of an autoscaling group we can enforce a ceiling on how expensive your cluster can get since CA will honor the bounds specified by the auto scaling group.

We need to provide similar functionality via Karpenter as well wherein there's a hard limit a customer can configure.


## Current Implementation

To address the runaway-scaling problem the current fix in place is to detect if the kubelet for a worker node has never reported its status to the K8s control plane. If it's been longer than 15 minutes, Karpenter assumes that there's a hard failure mode due to which this worker node will never become healthy and terminates the worker node. If the condition map of the node object in the API Server says `NodeStatusNeverUpdated` then we use that as an indicator of the node having never come up.

This ensures that if there are other scenarios where a worker node has become unhealthy due to some network partition or power outage in a availability zone, we don't terminate those worker nodes. It's important we don't make the static stability of a cluster worse during such an event. On the other hand, if there is an edge case where worker nodes come online and soon go offline, it will lead to runaway scaling again. This edge case should be unlikely to happen in the near term, so this document focuses on just the ability to limit costs within Karpenter. That way even if runaway scaling does occur, there's a way to bound it.


## Proposed Solution for Limits

There are two broad forms of limiting we could apply. The first is that we could introduce a limit to the number of in-flight worker node being provisioned at a point in time. A worker node that's in the `NotReady` state could be considered to be in-flight.


### **In-flight limit**

```yaml
spec:
  limits:
    unready: 20% # StringOrInt
```

The good bit about this approach is that we don't artificially constrain how many worker nodes can be spun up by Karpenter. Karpenter doesn't need to be aware of how limits are represented by each cloudprovider, while also making sure that if we're launching worker nodes that aren't healthy we stop the scaling and save costs.

The two main problems with this approach though are -
1. This limit while meant to constrain the number of unhealthy worker nodes in a cluster, will also inihibit the rate at which Karpenter can respond to pods that aren't schedulable. This somewhat goes against the goal of minimizing launch times of workers.
2. While this helps ensure that costs don't increase due to runaway scaling, it won't help those who want a stricter cap on the amount of resources that's being provisioned even when nodes are otherwise healthy.

### **Absolute limit**

This is a much simpler semantic to follow, where you provide an upper cap on the amount of resources being provisioned irrespective of their health. This is more in-line with alternative solutions like CA and should be easier to reason with.

The actual limit of the number of resources being spun up can be defined either through something similar to resource requests or just a flat count of instances.

```yaml
spec:
  limits:
    resources:
      cpu: 1000
      memory: 1000Gi
```

Modeling your limit via CPU and Memory constraints aligns well with customers that use resource requests with their pods. This should give everyone a consistent interface to define the capacity requirements of their applications as well as their underlying infrastructure. There shouldn't be a need to think of the actual number and type of instances that are being chosen by Karpenter.

As a cost control mechanism, this requires a little more work from our users if they're looking for higher precision. Since some cloud providers like AWS charge differently per instance type rather than the actual resource request and consumption, a little more translation will be needed to estimate costs. The alternative of a flat count of instance types - say `spec.limits.resources.count`, doesn't help solve that problem either since the instance types are chosen by Karpenter by default. In case overrides are defined via `kubernetes.io/instance-type`, you should be able to calculate the CPU and Memory bounds fairly trivially through some cloud provider API calls.

[CPU limits](https://v1-20.docs.kubernetes.io/docs/concepts/configuration/manage-resources-containers/#meaning-of-cpu) and memory limits will be defined similar to resource requests and will not be required by default. Karpenter will also will not default to any limits itself.