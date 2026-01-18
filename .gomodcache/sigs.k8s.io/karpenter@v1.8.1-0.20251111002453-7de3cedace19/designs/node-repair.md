# Node Auto Repair 

## Problem Statement

Nodes can experience failure modes that cause degradation to the underlying hardware, file systems, or container environments. Some of these failure modes are surfaced through the Node object such as network unavailability, disk pressure, or memory pressure, while others are not surfaced at all such as accelerator health. A Diagnostic Agent such as the [Node Problem Detector (NPD)](https://github.com/kubernetes/node-problem-detector) offers a way to surface these failures as additional status conditions on the node object.

When a status condition is surfaced through the Node, it indicates that the Node is unhealthy. Karpenter does not currently react to those conditions.

* Mega Issue: https://github.com/kubernetes-sigs/karpenter/issues/750
    * Related (Unreachable): https://github.com/aws/karpenter-provider-aws/issues/2570
    * Related (Remove by taints): https://github.com/aws/karpenter-provider-aws/issues/2544
    * Related (Known resource are not registered) Fixed by v0.28.0: https://github.com/aws/karpenter-provider-aws/issues/3794
    * Related (Stuck on NotReady): https://github.com/aws/karpenter-provider-aws/issues/2439

#### Out of scope

The alpha implementation will not consider these features:

  - Disruption Budgets
  - Customer-Defined Conditions
  - Customer-Defined Remediation Time

The team does not have enough data to determine the right level of configuration that users will utilize. The opinionated mechanism would be responsible for defining unhealthy notes. The advantage of creating the mechanism would be to reduce the configuration burden for customers. **The feature will be gated under an alpha NodeRepair=true feature flag. This will  allow for additional feedback from customers. Additional feedback can support features that were originally considered out of scope for the Alpha stage.**

## Recommendation: CloudProvider-Defined RepairPolicies

```
type RepairStatement struct {
    // Type of unhealthy state that is found on the node
    Type metav1.ConditionType 
    // Status condition of when a node is unhealthy
    Status metav1.ConditionStatus
    // TolerationDuration is the duration the controller will wait
    // before attempting to terminate nodes that are marked for repair.
    TolerationDuration time.Duration
}

type CloudProvider interface {
  ...
    // RepairPolicy is for CloudProviders to define a set Unhealthy condition for Karpenter 
    // to monitor on the node. Customer will need 
    RepairPolicy() []v1.RepairPolicy
  ...
}
```

The RepairPolicy will contain a set of statements that the Karpenter controller will use to watch node conditions. On any given node, multiple node conditions may exist simultaneously. In those cases, we will chose the shortest `TolerationDuration` for a given condition. The cloud provider can define compatibility with any node diagnostic agent, and track a list of node unhealthy condition types. The `TolerationDuration` will wait until a unhealthy state has passed the duration and is considered terminal:

1. A diagnostic agent will add a status condition on a node 
2. Karpenter will reconcile nodes and match unhealthy conditions with repair policy statements
3. Node Health controller will forcefully terminate the the NodeClaim once the node has been in an unhealthy state for the duration specified by the TolerationDuration

### Example

The example will look at the supporting Node Problem Detector for the AWS Karpenter Provider:
```
func (c *CloudProvider) RepairPolicy() []cloudprovider.RepairStatement {
    return cloudprovider.RepairStatement{
        {
            Type: "Ready"
            Status: corev1.ConditionFalse,
            TrolorationDuration: "30m"
        },
        {
            Type: "NetworkUnavailable"
            Status: corev1.ConditionTrue,
            TrolorationDuration: "10m"
        },
        ...
    }
}
```

In the example above, the AWS Karpenter Provider supports monitoring and terminating two node status conditions of the Kubelet Ready condition and the NPD  NetworkUnavailable condition. Below are the two cases of when we will act on nodes:

```
apiVersion: v1
kind: Node
metadata:
  ...
status:
  condition:
    - lastHeartbeatTime: "2024-11-01T16:29:49Z"
      lastTransitionTime: "2024-11-01T15:02:48Z"
      message: no connection
      reason: Network is not available
      status: "False"
      type: NetworkUnavailable
    ...
    
- The Node here will be eligible for node repair after at `2024-11-01T15:12:48Z`    
---
apiVersion: v1
kind: Node
metadata:
  ...
status:
  condition:
    - lastHeartbeatTime: "2024-11-01T16:29:49Z"
      lastTransitionTime: "2024-11-01T15:02:48Z"
      message: kubelet is posting ready status
      reason: KubeletReady
      status: "False"
      type: NetworkUnavailable
    - lastHeartbeatTime: "2024-11-01T16:29:49Z"
      lastTransitionTime: "2024-11-01T15:02:48Z"
      message: kubelet is posting ready status
      reason: KubeletReady
      status: "False"
      type: Ready
    ...
    
- The Node here will be eligible for node repair after at `2024-11-01T15:32:48Z`    
```


## Forceful termination

For a first iteration approach, Karpenter will implement the force termination. Today, the graceful termination in Karpenter will attempt to wait for the pod to be fully drained on a node and all volume attachments to be deleted from a node. This raises the problem that during the graceful termination, the node can be stuck terminating when the pod eviction or volume detachment may be broken. In these cases, users will need to take manual action against the node. **For the Alpha implementation, the recommendation will be to forcefully terminate nodes. Furthermore, unhealthy nodes will not respect the customer configured terminationGracePeriod.**

## Future considerations 

There are additional features we will consider including after the initial iteration. These include:

* Disruption controls (budgets, terminationGracePeriod) for unhealthy nodes 
* Node Reboot (instead of replacement)
* Configuration surface for graceful vs forceful termination 
* Additional consideration for the availability zone resiliency 

