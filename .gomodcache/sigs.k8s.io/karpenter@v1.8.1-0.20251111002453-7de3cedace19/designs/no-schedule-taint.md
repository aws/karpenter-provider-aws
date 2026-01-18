# Karpenter NoSchedule Taint [v0.32.0+]

## Overview

Karpenter uses the `node.kubernetes.io/unschedulable:NoSchedule` taint to prevent pods from scheduling to nodes being deprovisioned. The `node.kubernetes.io/unschedulable` taint is well-known in Kubernetes and may be leveraged or relied on by other controllers or applications.

Karpenter cordons nodes when it launching replacements for a deprovisioning action. When Karpenter crashes or restarts during a deprovisioning action, nodes can be left cordoned until manual intervention or deprovisioning by Karpenter in the future. Karpenter can't distinguish between nodes that it has cordoned and nodes a user/another controller has cordoned. **Since Karpenter cannot assume it’s the only agent in the cluster managing this taint, Karpenter is unable to recover from crashes where nodes are left cordoned.**

Karpenter is [increasing the parallelism of deprovisioning](https://github.com/aws/karpenter-core/pull/542), allowing Karpenter to execute many actions simultaneously. Once merged, Karpenter could have many actions in memory, making it likely for a Karpenter crash to have more widespread and noticeable effects. At the worst, a cluster could have all nodes being deprovisioned simultaneously, where a crash would result in all nodes being left cordoned, requiring new nodes for any new pods.

## Proposal

Karpenter should taint nodes with a `karpenter.sh/disruption=disrupting:NoSchedule` taint rather than relying on the upstream unschedulable taint.

Since Karpenter is currently migrating to v1beta1, this behavior change will only be present with v1beta1 APIs.

PR: https://github.com/aws/karpenter-core/pull/508

## Considerations

### Daemonsets

The [Daemonset Controller](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/#create-a-daemonset) adds [default tolerations to every pod](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/#taints-and-tolerations), including the `node.kubernetes.io/unschedulable` taint. As part of termination, Karpenter [will not](https://github.com/aws/karpenter-core/blob/main/pkg/controllers/termination/terminator/terminator.go#L79-L81) evict pods that tolerate the `node.kubernetes.io/unschedulable` taint, as they would immediately reschedule. After this change, Karpenter will not use the `node.kubernetes.io/unschedulable` taint, resulting in draining for daemonsets during termination. If users don’t want their daemonsets to be evicted by Karpenter at termination, they’ll need to add a special toleration, not currently added by default. **This will be called out in the v1beta1 migration.**

Cluster Autoscaler [uses their own `NoSchedule` taint](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/utils/taints/taints.go#L39-L42) to ensure that pods will not schedule to the nodes being deprovisioned. [By default](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/main.go#L216-L217), CAS will evict daemonset pods for non-empty nodes and will also respect a pod [annotation](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#how-can-i-enabledisable-eviction-for-a-specific-daemonset) on each daemonset pod to know if Karpenter should evict the pod.

🔑 Karpenter determines if it should evict a pod through its tolerations. Karpenter could consider adding in a configuration surface similar to CAS to allow evicting pods owned by a daemonset. I recommend that we make this taint change during v1beta1 with ample documentation, and add the configuration surface in as a follow up if requested.

### SIG-Autoscaling Alignment

Karpenter is currently in process of [aligning with SIG-Autoscaling](https://docs.google.com/document/d/1_KCCr5CzxmurFX_6TGLav6iMwCxPOj0P/edit) for a common experience. As part of alignment, all API groups for both projects are proposed to be renamed to `node-lifecycle.kubernetes.io`.  After this change, users migrating to `v1beta1` will need to change their daemonsets to tolerate the new Karpenter taint if they don't want their daemonsets evicted. If the API group is changed as part of alignment, these API groups will change once more, requiring users to change their tolerations on their workloads once again, introducing more churn.

🔑 There is ample ambiguity on the shared direction of Karpenter, CAS, and the entirety of upstream Kubernetes aligning around these concepts. It's unclear how this conversation will evolve and how long this conversation will take to resolve. As a result, we are choosing not to block on this conversation and, instead, taking an opinionated stance in Karpenter that we can choose to evolve later.

### How severe is this issue currently?

At the moment, no user has reported this issue, likely since its effect and recovery are mitigated by how Karpenter deprovisions serially, executing one action at a time. If Karpenter crashes/restarts while executing a command, only one command’s worth of candidate nodes will be left cordoned. Since expiration, drift, and consolidation all have a consistent ordering, it’s then likely that each cordoned node will get chosen for a deprovisioning action soon after recovery.
