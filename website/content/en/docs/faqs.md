---
title: "FAQs"
linkTitle: "FAQs"
weight: 30
---

## General
### How does a Provisioner decide to manage a particular node?
<<<<<<< HEAD:website/content/en/docs/faq.md
Each node will have a set of predetermined Karpenter labels. Provisioners will use the `name` and `namespace` labels to distinguish between Provisioners. Furthermore, a Provisioner will only take action on a node based on the label that details what phase a node is in, e.g. a Provisioner will only consider a node for termination if its phase label says `"underutilized"`.
## Provisioning
=======
Each node will have a set of predetermined Karpenter labels. Provisioners will use a `name` label to distinguish between Provisioners. Furthermore, a Provisioner will only take action on a node based on the label that details what phase a node is in. e.g. a provisioner will only consider a node for termination if its phase label says "underutilized".
## Allocation
>>>>>>> eaa48cf (update filenames and get started guide):website/content/en/docs/faqs.md
### How should I define scheduling constraints?
Karpenter takes a layered approach to scheduling constraints. Each Cloud Provider has its own set of global defaults, which are overriden by defaults specified in the Provisioner, which are overridden by Pod scheduling constraints. This model requires minimal configuration for most use cases, and supports diverse workloads using a single Provisioner.
### Does Karpenter replace the Kube Scheduler?
No. Provisioners work in tandem with the Kube Scheduler. When capacity is unconstrained, the Kube Scheduler will schedule pods as usual. It may schedule pods to nodes managed by Provisioners or other types of capacity in the cluster. Provisioners only attempt to schedule pods when `type=PodScheduled,reason=Unschedulable`. In this case, they will make a provisioning decision, launch new capacity, and bind pods to the provisioned nodes. Provisioners do not wait for the Kube Scheduler to make a scheduling decision in this case, as the decision is already made by nature of making a provisioning decision. It's possible that a node from another management solution, like the Cluster Autoscaler, could create a race between the `kube-scheduler` and Karpenter. In this case, the first binding call will win, although Karpenter will often win these race conditions due to its performance characteristics. If Karpenter loses this race, the node will eventually be cleaned up.
### Does Karpenter support node selectors?
Yes. Node selectors are an opt-in mechanism which allow customers to specify the nodes on which a pod can scheduled. Provisioners recognize well-known node selectors on incoming pods and use them to constrain the nodes they generate. You can read more about the well-known node selectors Karpenter supports in the [Concepts](/docs/concepts/#well-known-labels) documentation. For example, well known selectors like `node.kubernetes.io/instance-type`, `topology.kubernetes.io/zone`, `kubernetes.io/os`, `kubernetes.io/arch` are supported, and will ensure that provisioned nodes are constrained accordingly. Additionally, customers may specify arbitrary labels, which will be automatically applied to every node launched by the Provisioner.
### Does Karpenter support taints?
Yes. Taints are an opt-out mechanism which allows customers to specify the nodes on which a pod cannot be scheduled. Unlike labels, Provisioners do not automatically taint nodes in response to pod tolerations, since pod tolerations do not require that corresponding taints exist. However, similar to labels, customers may specify taints for their Provisioner, which will automatically be applied to every node in the group. This means that if a Provisioner is configured with taints, any incoming pods will not be provisioned unless they tolerate the taints.
### Does Karpenter support topology spread constraints?
Yes. Provisioners respect `pod.spec.topologySpreadConstraints`. Allocating pods with these constraints may yield highly fragmented nodes, due to their strict nature and complexity of “online binpacking” algorithms.
### Does Karpenter support affinity?
No. Karpenter intentionally does not support affinity due the to [scalability limitations](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity) outlined by SIG Scalability. Instead, we recommend using node selectors or taints instead of node affinity and pod topology spread instead of pod affinity. Do you have a use case for affinity that we're missing? Open an issue in our [GitHub repo](https://github.com/awslabs/karpenter/issues/new/choose) and tell us about it!
### Does Karpenter support custom resource like accelerators or HPC?
Yes. Support for specific custom resources can be implemented by your cloud provider.
### Does Karpenter support daemonsets?
Yes. Provisioners factor in daemonset overhead into all allocation calculations. They also respect daemonset scheduling constraints, such as Nvidia’s GPU Driver Installer.
### Does Karpenter support multiple scheduling defaults?
Provisioners are heterogeneous, which means that the nodes they manage are spread across multiple availability zones, instance types, and capacity types. This flexibility reduces the need for a large number of groups. However, customers may find multiple groups to be useful for more advanced use cases. For example, customers can create multiple groups, and then use the node selector `karpenter.sh/provisioner-name` to target specific groups. This enables advanced use cases like resource isolation and sharding.
### What if my pod is schedulable for multiple Provisioners?
It's possible that unconstrained pods could flexibly schedule in multiple groups. In this case, Provisioners will race to create a scheduling lease for the pod before launching new nodes, which avoids unnecessary scale out.
## Deprovisioning
### How does Karpenter decide which nodes it can terminate? 
A provisioner will only take action on nodes that it manages. This means that a node will only be considered for termination if it is labeled underutilized by the provisioner that manages it.
### How do I know if a node is underutilized?
Nodes are labeled underutilized if they have 0 non-daemonset pods scheduled. We plan to include more use cases in the future. A node needs to be underutilized for a period of time before being considered for termination.
### How does Karpenter terminate nodes?
Karpenter annotates nodes that are underutilized with a time to live (TTL). If the node remains underutilized after the TTL expires, Karpenter then [cordons](https://kubernetes.io/docs/concepts/architecture/nodes/#manual-node-administration) the node and uses the [Kubernetes Eviction API](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/#eviction-api) to evict all non-daemonset pods. Once the node is empty, the node is terminated.
### Does Karpenter support Pod Disruption Budgets?
Yes. The Kubernetes Eviction API will not delete pods that violate a [Pod Disruption Budget (PDB)](https://kubernetes.io/docs/tasks/run-application/configure-pdb/). It also disallows eviction of any pod covered by multiple PDBs, so most users will want to avoid overlapping selectors. See [this](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/#pod-disruption-budgets) for more.
### Does Karpenter support scale to zero?
Yes. Provisioners start at zero and launch or terminate nodes as necessary. We recommend that customers maintain a small amount of static capacity to bootstrap system controllers or run Karpenter outside of their cluster.
## Compatibility
### Which Kubernetes versions does Karpenter support?
Karpenter releases on a similar cadence to upstream Kubernetes releases. Currently, Karpenter is compatible with all Kubernetes versions greater than v1.16. However, this may change in the future as Karpenter takes dependencies on new Kubernetes features.
### Can I use Karpenter alongside another node management solution?
Provisioners are designed to work alongside static capacity management solutions like EKS Managed Node Groups and EC2 Auto Scaling Groups. Some customers may choose to (1) manage the entirety of their capacity using Provisioner, others may prefer (2) a mixed model with both dynamic and statically managed capacity, some may prefer (3) a fully static approach. We anticipate that most customers will fall into bucket (2) in the short term, and (1) in the long term.
### Can I use Karpenter with the Kubernetes Cluster Autoscaler?
Yes, with side effects. Karpenter is a Cluster Autoscaler replacement. Both systems scale up nodes in response to unschedulable pods. If configured together, both systems will race to launch new instances for these pods. Since Provisioners make binding decisions, Karpenter will typically win the scheduling race. In this case, the Cluster Autoscaler will eventually scale down the unnecessary capacity. If the Cluster Autoscaler is configured with Node Groups that have constraints that aren’t supported by any Provisioner, its behavior will continue unimpeded.
