---
title: "FAQs"
linkTitle: "FAQs"
weight: 30
---

## General
### How does a Provisioner decide to manage a particular node?
Karpenter will only take action on nodes that it provisions. All nodes launched by Karpenter will be labeled with `karpenter.sh/provisioner-name`.
## Compatibility
### Which Kubernetes versions does Karpenter support?
Karpenter releases on a similar cadence to upstream Kubernetes releases. Currently, Karpenter is compatible with Kubernetes versions v1.19+. However, this may change in the future as Karpenter takes dependencies on new Kubernetes features.
### Can I use Karpenter alongside another node management solution?
Provisioners are designed to work alongside static capacity management solutions like EKS Managed Node Groups and EC2 Auto Scaling Groups. Some users may choose to (1) manage the entirety of their capacity using Provisioners, others may prefer (2) a mixed model with both dynamic and statically managed capacity, some may prefer (3) a fully static approach. We anticipate that most users will fall into bucket (2) in the short term, and (1) in the long term.
### Can I use Karpenter with the Kubernetes Cluster Autoscaler?
Yes, with side effects. Karpenter is a Cluster Autoscaler replacement. Both systems scale up nodes in response to unschedulable pods. If configured together, both systems will race to launch new instances for these pods. Since Karpenter makes binding decisions, Karpenter will typically win the scheduling race. In this case, the Cluster Autoscaler will eventually scale down the unnecessary capacity. If the Cluster Autoscaler is configured with Node Groups that support scheduling constraints that aren’t supported by any Provisioner, its behavior will continue unimpeded.
### Does Karpenter replace the Kube Scheduler?
No. Provisioners work in tandem with the Kube Scheduler. When capacity is unconstrained, the Kube Scheduler will schedule pods as usual. It may schedule pods to nodes managed by Provisioners or other types of capacity in the cluster. Provisioners only attempt to schedule pods when `type=PodScheduled,reason=Unschedulable`. In this case, Karpenter will make a provisioning decision, launch new capacity, and bind pods to the provisioned nodes. Unlike the Cluster Autoscaler, Karpenter does not wait for the Kube Scheduler to make a scheduling decision, as the decision is already made during the provisioning decision. It's possible that a node from another management solution, like the Cluster Autoscaler, could create a race between the `kube-scheduler` and Karpenter. In this case, the first binding call will win, although Karpenter will often win these race conditions due to its performance characteristics. If Karpenter loses this race, the node will eventually be cleaned up.
## Provisioning
### How should I define scheduling constraints?
Karpenter takes a layered approach to scheduling constraints. Karpenter comes with a set of global defaults, which may be overriden by Provisioner-level defaults. Further, these may be overriden by pod scheduling constraints. This model requires minimal configuration for most use cases, and supports diverse workloads using a single Provisioner.
### Does Karpenter support node selectors?
Yes. Node selectors are an opt-in mechanism which allow users to specify the nodes on which a pod can scheduled. Karpenter recognizes [well-known node selectors](https://kubernetes.io/docs/reference/labels-annotations-taints/) on unschedulable pods and uses them to constrain the nodes it provisions. You can read more about the well-known node selectors supported by Karpenter in the [Concepts](/docs/concepts/#well-known-labels) documentation. For example, `node.kubernetes.io/instance-type`, `topology.kubernetes.io/zone`, `kubernetes.io/os`, `kubernetes.io/arch`, `karpenter.sh/capacity-type` are supported, and will ensure that provisioned nodes are constrained accordingly. Additionally, users may specify arbitrary labels, which will be automatically applied to every node launched by the Provisioner.
<!-- todo defaults+overrides -->
### Does Karpenter support taints?
Yes. Taints are an opt-out mechanism which allows users to specify the nodes on which a pod cannot be scheduled. Unlike node selectors, Karpenter does not automatically taint nodes in response to pod tolerations. Similar to node selectors, users may specify taints on their Provisioner, which will be automatically added to every node it provisions. This means that if a Provisioner is configured with taints, any incoming pods will not be scheduled unless the taints are tolerated.
### Does Karpenter support topology spread constraints?
Not yet. Karpenter plans to respect `pod.spec.topologySpreadConstraints` by v0.4.0.
### Does Karpenter support node affinity?
Not yet. Karpenter plans to respect `pod.spec.nodeAffinity` by v0.4.0.
### Does Karpenter support custom resource like accelerators or HPC?
Yes. Support for specific custom resources may be implemented by cloud providers. The AWS Cloud Provider supports `nvidia.com/gpu`, `amd.com/gpu`, `aws.amazon.com/neuron`.
### Does Karpenter support daemonsets?
Yes. Karpenter factors in daemonset overhead into all provisioning calculations. Daemonsets are only included in calculations if their scheduling constraints are applicable to the provisoned node.
### Does Karpenter support multiple Provisioners?
Each Provisioner is capable of defining heterogenous nodes across multiple availability zones, instance types, and capacity types. This flexibility reduces the need for a large number of Provisioners. However, users may find multiple Provisioners to be useful for more advanced use cases, such as defining multiple sets of provisioning defaults in a single cluster.
### If multiple Provisioners are defined, which will my pod use?
By default, pods will use the rules defined by a Provisioner named `default`. This is analogous to the `default` scheduler. To select an alternative provisioner, use the node selector `karpenter.sh/provisioner-name: alternative-provisioner`. You must either define a default provisioner or explicitly specify `karpenter.sh/provisioner-name` node selector.
## Deprovisioning
### How does Karpenter decide which nodes it can terminate?
Karpenter will only terminate nodes that it manages. Nodes will be considered for termination due to expiry or emptiness (see below).
### When does Karpenter terminate empty nodes?
Nodes are considered empty when they do not have any pods scheduled to them. Daemonsets pods and Failed pods are ignored. Karpenter will send a deletion request to the Kubernetes API, and graceful termination will be handled by termination finalizer. Karpenter will wait for the duration of `ttlSecondsAfterUnderutilized` to terminate an empty node. If `ttlSecondsAfterUnderutilized` is unset, **which it is by default**, Karpenter will not terminate nodes once they are empty.
### When does Karpenter terminate expired nodes?
Nodes are considered expired when the current time exceeds their creation time plus `ttlSecondsUntilExpired`. Karpenter will send a deletion request to the Kubernetes API, and graceful termination will be handled by termination finalizer. If `ttlSecondsUntilExpired` is unset, **which it is by default**,  Karpenter will not terminate any nodes due to expiry.
### How does Karpenter terminate nodes?
Karpenter [cordons](https://kubernetes.io/docs/concepts/architecture/nodes/#manual-node-administration) nodes to be terminated and uses the [Kubernetes Eviction API](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/#eviction-api) to evict all non-daemonset pods. After successful eviction of all non-daemonset pods, the node is terminated. If all the pods cannot be evicted, Karpenter won't forcibly terminate them and keep on trying to evict them. Karpenter respects [Pod Disruption Budgets (PDB)](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) by using the Kubernetes Eviction API.
### Does Karpenter support scale to zero?
Yes. Karpenter only launches or terminates nodes as necessary based on aggregate pod resource requests. Karpenter will only retain nodes in your cluster as long as there are pods using them.
