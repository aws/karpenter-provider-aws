# FAQs
## Allocation
### Does Karpenter replace the Kube Scheduler?
No. Provisioners work in tandem with the Kube Scheduler. When capacity is unconstrained, the Kube Scheduler will schedule pods as usual. It may schedule pods to nodes managed by Provisioner or other types of capacity in the cluster. Provisioners only attempt to schedule pods of when `type=PodScheduled,reason=Unschedulable`. In this case, they will make a provisioning decision, launch new capacity, and bind pods to the provisioned nodes. Provisioning Groups do not wait for the Kube Scheduler to make a scheduling decision in this case, as the decision is already made by nature of making a provisioning decision.
### Does Karpenter support node selectors?
Yes. Node selectors are an opt-in mechanism which allows customers to specify the nodes to which a pod can schedule. Provisioning Groups recognize well known node selectors on incoming pods and use them to constrain the nodes they generate. For example, well known selectors like `node.kubernetes.io/instance-type`, `topology.kubernetes.io/zone`, `kubernetes.io/os`, `kubernetes.io/arch` are supported, and will ensure that provisioned nodes are constrained accordingly. Additionally, customers may specify arbitrary labels, which will be automatically applied to every node launched by the Provisioner.
### Does Karpenter support topology spread constraints?
Yes. Provisioning Groups respect `pod.spec.topologySpreadConstraints`. Allocating pods with these constraints may yield highly fragmented nodes, due to their strict nature and complexity of “online binpacking” algorithms. However, the reallocation pass is able to produce much more efficient packings using “offline binpacking” techniques.
### Does Karpenter support taints?
Yes. Taints are an opt-out mechanism which allows customers to specify the nodes to which a pod cannot schedule. Unlike labels, Provisioning Groups do not automatically taint nodes in response to pod tolerations, since pod tolerations do not require that corresponding taints exist. However, similar to labels, customers may specify taints for their Provisioner, which will automatically be applied to every node in the group. This means that if a Provisioner is configured with taints, any incoming pods will not be provisioned unless they tolerate the taints.
### Does Karpenter support affinity?
No. Karpenter intentionally does not support affinity due the to scalability limitations outlined by SIG Scalability. Do you have a use case for affinity? We're excited to hear about it in our [Working Group](working-group/README.md).
### Does Karpenter support custom resource like accelerators or HPC?
Yes. Support for specific custom resources can be implemented by your cloud provider.
### Does Karpenter support daemonsets?
Yes. Provisioners factor in Daemonset overhead into all allocation and reallocation calculations. It also respects specific Daemonsets scheduling constraints, such as Nvidia’s GPU Driver Installer.
### Does Karpenter support multiple scheduling defaults?
Provisioning Groups are heterogeneous, which means that the nodes they manage are spread across multiple availability zones, instance types, and capacity types. This flexibility reduces the need for a large number of groups. However, customers may find multiple groups to be useful for more advanced use cases. For example, customers can create multiple groups, and then use the node selector `provisioning.karpenter.sh/name` to target specific groups. This enables advanced use cases like resource isolation and sharding.
### What if my pod is schedulable for multiple Provisioners?
It's possible that an unconstrained pods could flexibly schedule in multiple groups. In this case, Provisioners will race to create a scheduling lease for the pod before launching new nodes, which avoids unnecessary scale out.
## Reallocation
TODO
### Does Karpenter support scale to zero?
Yes. Provisioners start at zero and launch or terminate nodes as necessary. We recommend that customers maintain a small amount of static capacity to bootstrap system controllers or run Karpenter outside of their cluster.
## Upgrade
TODO
## Interruption
TODO
## Compatibility
### Which Kubernetes versions does Karpenter support?
Karpenter releases on a similar cadence to upstream Kubernetes releases. Currently, Karpenter is compatible with all Kubernetes versions greater than v1.16. However, this may change in the future as Karpenter takes dependencies on new Kubernetes features.
### Can I use Karpenter another Node management solution?
Provisioners are designed to work alongside static capacity management solutions like EKS Managed Node Groups and EC2 Auto Scaling Groups. Some customers may choose to (1) manage the entirety of their capacity using Provisioner, others may prefer (2) a mixed model with both dynamic and statically managed capacity, some may prefer (3) a fully static approach. We anticipate that most customers will fall into bucket (2) in the short term, and (1) in the long term.
### Can I use Karpenter with the Kubernetes Cluster Autoscaler?
Yes, with side effects. Karpenter is a Cluster Autoscaler replacement. Both systems scale up nodes in response to pending pods. If configured together, both systems will race to launch new instances for incoming pods. Since Provisioners make binding decisions, they will typically win the scheduling race. In this case, the Cluster Autoscaler will eventually scale down the unnecessary capacity. If the Cluster Autoscaler is configured with Node Groups that have constraints that aren’t supported by any Provisioner, its behavior will continue unimpeded.
