---
title: "FAQs"
linkTitle: "FAQs"
weight: 60
description: >
  Review Karpenter Frequently Asked Questions
---
## General

### Is Karpenter safe for production use?
Karpenter v1 is the first stable Karpenter API. Any future incompatible API changes will require a v2 version.

### How does a NodePool decide to manage a particular node?
See [Configuring NodePools]({{< ref "./concepts/#configuring-nodepools" >}}) for information on how Karpenter configures and manages nodes.

### What cloud providers are supported?
AWS is the first cloud provider supported by Karpenter, although it is designed to be used with other cloud providers as well.

### Can I write my own cloud provider for Karpenter?
Yes, but there is no documentation yet for it. Start with Karpenter's GitHub [cloudprovider](https://github.com/aws/karpenter-provider-aws/tree{{< githubRelRef >}}pkg/cloudprovider) documentation to see how the AWS provider is built, but there are other sections of the code that will require changes too.

### What operating system nodes does Karpenter deploy?
Karpenter uses the OS defined by the [AMI Family in your EC2NodeClass]({{< ref "./concepts/nodeclasses#specamifamily" >}}).

### Can I provide my own custom operating system images?
Karpenter has multiple mechanisms for configuring the [operating system]({{< ref "./concepts/nodeclasses/#specamiselectorterms" >}}) for your nodes.

### Can Karpenter deal with workloads for mixed architecture cluster (arm vs. amd)?
Karpenter is flexible to multi-architecture configurations using [well known labels]({{< ref "./concepts/scheduling/#supported-labels">}}).

### What RBAC access is required?
All the required RBAC rules can be found in the Helm chart template. See [clusterrole-core.yaml](https://github.com/aws/karpenter/blob{{< githubRelRef >}}charts/karpenter/templates/clusterrole-core.yaml), [clusterrole.yaml](https://github.com/aws/karpenter/blob{{< githubRelRef >}}charts/karpenter/templates/clusterrole.yaml), [rolebinding.yaml](https://github.com/aws/karpenter/blob{{< githubRelRef >}}charts/karpenter/templates/rolebinding.yaml), and [role.yaml](https://github.com/aws/karpenter/blob{{< githubRelRef >}}charts/karpenter/templates/role.yaml) files for details.

### Can I run Karpenter outside of a Kubernetes cluster?
Yes, as long as the controller has network and IAM/RBAC access to the Kubernetes API and your provider API.

### What do I do if I encounter a security issue with Karpenter?
Refer to [Reporting Security Issues](https://github.com/aws/karpenter/security/policy) for information on how to report Karpenter security issues. Do not create a public GitHub issue.

## Compatibility

### Which versions of Kubernetes does Karpenter support?
See the [Compatibility Matrix in the Upgrade Section]({{< ref "./upgrading/compatibility#compatibility-matrix" >}}) to view the supported Kubernetes versions per Karpenter released version.

### What Kubernetes distributions are supported?
Karpenter documents integration with a fresh or existing installation of the latest AWS Elastic Kubernetes Service (EKS). Other Kubernetes distributions (KOPs, etc.) can be used, but setting up cloud provider permissions for those distributions has not been documented.

### How does Karpenter interact with AWS node group features?
NodePools are designed to work alongside static capacity management solutions like EKS Managed Node Groups and EC2 Auto Scaling Groups. You can manage all capacity using NodePools, use a mixed model with dynamic and statically managed capacity, or use a fully static approach. We expect most users will use a mixed approach in the near term and NodePool-managed in the long term.


### How does Karpenter interact with Kubernetes features?
* Kubernetes Cluster Autoscaler: Karpenter can work alongside Cluster Autoscaler. See [Kubernetes Cluster Autoscaler]({{< ref "./concepts/#kubernetes-cluster-autoscaler" >}}) for details.
* Kubernetes Scheduler: Karpenter focuses on scheduling pods that the Kubernetes scheduler has marked as unschedulable. See [Scheduling]({{< ref "./concepts/scheduling" >}}) for details on how Karpenter interacts with the Kubernetes scheduler.

## Provisioning

### What features does the Karpenter NodePool support?
See the [NodePool API docs]({{< ref "./concepts/nodepools" >}}) for NodePool examples and descriptions of features.

### Can I create multiple (team-based) NodePools on a cluster?
Yes, NodePools can identify multiple teams based on labels. See the [NodePool API docs]({{< ref "./concepts/nodepools" >}}) for details.

### If multiple NodePools are defined, which will my pod use?

Pending pods will be handled by any NodePools that matches the requirements of the pod. There is no ordering guarantee if multiple NodePools match pod requirements. We recommend that NodePools are set-up to be mutually exclusive. To select a specific NodePool, use the node selector `karpenter.sh/nodepool: my-nodepool`.

### How can I configure Karpenter to only provision pods for a particular namespace?

There is no native support for namespaced-based provisioning. Karpenter can be configured to provision a subset of pods based on a combination of taints/tolerations and node selectors. This allows Karpenter to work in concert with the `kube-scheduler` using the same mechanisms to determine if a pod can schedule to an existing node are also used for provisioning new nodes. This avoids scenarios where pods are bound to nodes that were provisioned by Karpenter which Karpenter would not have bound itself. If this were to occur, a node could remain non-empty and have its lifetime extended due to a pod that wouldn't have caused the node to be provisioned had the pod been unschedulable.

We recommend using Kubernetes native scheduling constraints to achieve namespace-based scheduling segregation. Using native scheduling constraints ensures that Karpenter, `kube-scheduler` and any other scheduling or auto-provisioning mechanism all have an identical understanding of which pods can be scheduled on which nodes.  This can be enforced via policy agents, an example of which can be seen [here](https://blog.mikesir87.io/2022/01/creating-tenant-node-pools-with-karpenter/).

### Can I add SSH keys to a NodePool?

Karpenter does not offer a way to add SSH keys via NodePools or secrets to the nodes it manages.
However, you can use Session Manager (SSM) or EC2 Instance Connect to gain shell access to Karpenter nodes. See [Node NotReady]({{< ref "./troubleshooting/#node-notready" >}}) troubleshooting for an example of starting an SSM session from the command line or [EC2 Instance Connect](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-connect-set-up.html) documentation to connect to nodes using SSH.

Though not recommended, if you need to access Karpenter-managed nodes without AWS credentials, you can add SSH keys using EC2NodeClass User Data. See the [User Data section in the EC2NodeClass documentation]({{< ref "./concepts/nodeclasses/#specuserdata" >}}) for details.

### Can I set limits of CPU and memory for a NodePool?
Yes. View the [NodePool API docs]({{< ref "./concepts/nodepools#speclimits" >}}) for NodePool examples and descriptions of how to configure limits.

### Can I mix spot and on-demand EC2 run types?
Yes, see the [NodePool API docs]({{< ref "./concepts/nodepools#examples" >}}) for an example.

### Can I restrict EC2 instance types?

* Attribute-based requests are currently not possible.
* You can select instances with special hardware, such as gpu.

### Can I use Bare Metal instance types?

Yes, Karpenter supports provisioning metal instance types when a NodePool's `node.kubernetes.io/instance-type` Requirements only include `metal` instance types. If other instance types fulfill pod requirements, then Karpenter will prioritize all non-metal instance types before metal ones are provisioned.

### How does Karpenter dynamically select instance types?

Karpenter batches pending pods and then binpacks them based on CPU, memory, and GPUs required, taking into account node overhead, VPC CNI resources required, and daemonsets that will be packed when bringing up a new node. Karpenter [recommends the use of C, M, and R >= Gen 3 instance types]({{< ref "./concepts/nodepools#spectemplatespecrequirements" >}}) for most generic workloads, but it can be constrained in the NodePool spec with the [instance-type](https://kubernetes.io/docs/reference/labels-annotations-taints/#nodekubernetesioinstance-type) well-known label in the requirements section.

After the pods are binpacked on the most efficient instance type (i.e. the smallest instance type that can fit the pod batch), Karpenter takes 59 other instance types that are larger than the most efficient packing, and passes all 60 instance type options to an API called Amazon EC2 Fleet.


The EC2 fleet API attempts to provision the instance type based on the [Price Capacity Optimized allocation strategy](https://aws.amazon.com/blogs/compute/introducing-price-capacity-optimized-allocation-strategy-for-ec2-spot-instances/). For the on-demand capacity type, this is effectively equivalent to the `lowest-price` allocation strategy. For the spot capacity type, Fleet will determine an instance type that has both the lowest price combined with the lowest chance of being interrupted. Note that this may not give you the instance type with the strictly lowest price for spot.

### How does Karpenter calculate the resource usage of Daemonsets when simulating scheduling?

Karpenter currently calculates the applicable daemonsets at the NodePool level with label selectors/taints, etc. It does not look to see if there are requirements on the daemonsets that would exclude it from running on particular instances that the NodePool could or couldn't launch.
The recommendation for now is to use multiple NodePools with taints/tolerations or label selectors to limit daemonsets to only nodes launched from specific NodePoools.

### What if there is no Spot capacity?

The best defense against running out of Spot capacity is to allow Karpenter to provision as many distinct instance types as possible. Even instance types that have higher specs (e.g. vCPU, memory, etc.) than what you need can still be cheaper in the Spot market than using On-Demand instances. When Spot capacity is constrained, On-Demand capacity can also be constrained since Spot is fundamentally spare On-Demand capacity.

Allowing Karpenter to provision nodes from a large, diverse set of instance types will help you to stay on Spot longer and lower your costs due to Spot’s discounted pricing. Moreover, if Spot capacity becomes constrained, this instance type diversity will also increase the chances that you’ll be able to continue to launch On-Demand capacity for your workloads.

Karpenter has a concept of an “offering” for each instance type, which is a combination of zone and capacity type. Whenever the Fleet API returns an insufficient capacity error for Spot instances, those particular offerings are temporarily removed from consideration (across the entire NodePool) so that Karpenter can make forward progress with different options.

### Does Karpenter support IPv6?

Yes! Karpenter dynamically discovers if you are running in an IPv6 cluster by checking the kube-dns service's cluster-ip. When using an AMI Family such as `AL2`, Karpenter will automatically configure the EKS Bootstrap script for IPv6. Some EC2 instance types do not support IPv6 and the Amazon VPC CNI only supports instance types that run on the Nitro hypervisor. It's best to add a requirement to your NodePool to only allow Nitro instance types:

```
apiVersion: karpenter.sh/v1
kind: NodePool
...
spec:
  template:
    spec:
      requirements:
        - key: karpenter.k8s.aws/instance-hypervisor
          operator: In
          values:
            - nitro
```

For more documentation on enabling IPv6 with the Amazon VPC CNI, see the [docs](https://docs.aws.amazon.com/eks/latest/userguide/cni-ipv6.html).

{{% alert title="Windows Support Notice" color="warning" %}}
Windows nodes do not support IPv6.
{{% /alert %}}

### Why do I see extra nodes get launched to schedule pending pods that remain empty and are later removed?

You might have a daemonset, userData configuration, or some other workload that applies a taint after a node is provisioned. After the taint is applied, Karpenter will detect that the pod cannot be scheduled to this new node due to the added taint. As a result, Karpenter will provision yet another node. Typically, the original node has the taint removed and the pod schedules to it, leaving the extra new node unused and reaped by emptiness/consolidation. If the taint is not removed quickly enough, Karpenter may remove the original node before the pod can be scheduled via emptiness consolidation. This could result in an infinite loop of nodes being provisioned and consolidated without the pending pod ever scheduling.

The solution is to configure [startupTaints]({{<ref "./concepts/nodepools/#cilium-startup-taint" >}}) to make Karpenter aware of any temporary taints that are needed to ensure that pods do not schedule on nodes that are not yet ready to receive them.

Here's an example for Cilium's startup taint.
```
apiVersion: karpenter.sh/v1
kind: NodePool
...
spec:
  template:
    spec:
      startupTaints:
        - key: node.cilium.io/agent-not-ready
          effect: NoSchedule
```

## Scheduling

### When using preferred scheduling constraints, Karpenter launches the correct number of nodes at first.  Why do they then sometimes get consolidated immediately?

`kube-scheduler` is responsible for the scheduling of pods, while Karpenter launches the capacity. When using any sort of preferred scheduling constraint, `kube-scheduler` will schedule pods to nodes anytime it is possible.

As an example, suppose you scale up a deployment with a preferred zonal topology spread and none of the newly created pods can run on your existing cluster.  Karpenter will then launch multiple nodes to satisfy that preference.  If a) one of the nodes becomes ready slightly faster than other nodes and b) has enough capacity for multiple pods, `kube-scheduler` will schedule as many pods as possible to the single ready node, so they won't remain unschedulable. It doesn't consider the in-flight capacity that will be ready in a few seconds.  If all the pods fit on the single node, the remaining nodes that Karpenter has launched aren't needed when they become ready and consolidation will delete them.

### When deploying an additional DaemonSet to my cluster, why does Karpenter not scale-up my nodes to support the extra DaemonSet?

Karpenter will not scale-up more capacity for an additional DaemonSet on its own. This is due to the fact that the only pod that would schedule to that new node would be the DaemonSet pod, which is consuming additional capacity with no benefit. Therefore, Karpenter only considers DaemonSets when doing overhead calculations for scale-ups to workload pods.

To avoid new DaemonSets failing to schedule to existing Nodes, you should [set a high priority on your DaemonSet pods with a `preemptionPolicy: PreemptLowerPriority`](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#example-priorityclass) so that DaemonSet pods will be guaranteed to schedule on all existing and new Nodes. For existing Nodes, this will cause some pods with lower priority to get [preempted](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#preemption), replaced by the DaemonSet and re-scheduled onto new capacity that Karpenter will launch in response to the new pending pods.

The Karpenter maintainer team is also discussing a consolidation mechanism [in this Github issue](https://github.com/aws/karpenter/issues/3256) that would allow existing capacity to be rolled when a new DaemonSet is deployed without having to set [priority or preemption](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/) on the pods.


### Why aren’t my Topology Spread Constraints spreading pods across zones?

Karpenter will provision nodes according to `topologySpreadConstraints`. However, the Kubernetes scheduler may schedule pods to nodes that do not fulfill zonal spread constraints if the `minDomains` field is not set. If Karpenter launches nodes that can handle more than the required number of pods, and the newly launched nodes initialize at different times, then the Kubernetes scheduler may place more than the desired number of pods on the first node that is Ready.

The preferred solution is to use the [`minDomains` field in `topologySpreadConstraints`](https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/#topologyspreadconstraints-field), which is enabled by default starting in Kubernetes 1.27.

Before `minDomains` was available, another workaround has been to launch a lower [Priority](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/) pause container to each zone before launching the pods that you want to spread across the zones. The lower Priority on these pause pods would mean that they would be preempted when your desired pods are scheduled.

## Workloads

### How can someone deploying pods take advantage of Karpenter?

See [Application developer]({{< ref "./concepts/#application-developer" >}}) for descriptions of how Karpenter matches nodes with pod requests.

### Can I use Karpenter with EBS disks per availability zone?
Yes.  See [Persistent Volume Topology]({{< ref "./concepts/scheduling#persistent-volume-topology" >}}) for details.

### Can I set `--max-pods` on my nodes?
Yes, see the [KubeletConfiguration Section in the NodePool docs]({{<ref "./concepts/nodepools#spectemplatespeckubelet" >}}) to learn more.

### Why do the Windows2019, Windows2022 and Windows2025 AMI families only support Windows Server Core?
The difference between the Core and Full variants is that Core is a minimal OS with less components and no graphic user interface (GUI) or desktop experience.
`Windows2019`, `Windows2022` and `Windows2025` AMI families use the Windows Server Core option for simplicity, but if required, you can specify a custom AMI to run Windows Server Full.

You can specify the [Amazon EKS optimized AMI](https://docs.aws.amazon.com/eks/latest/userguide/eks-optimized-windows-ami.html) with Windows Server 2022 Full for Kubernetes {{< param "latest_k8s_version" >}} by configuring an `amiSelector` that references the AMI name.
```yaml
amiSelectorTerms:
    - name: Windows_Server-2022-English-Full-EKS_Optimized-{{< param "latest_k8s_version" >}}*
```

### Can I use Karpenter to scale my workload's pods?
Karpenter is a node autoscaler which will create new nodes in response to unschedulable pods. Scaling the pods themselves is outside of its scope.
This is the realm of pod autoscalers such as the [Vertical Pod Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler) (for scaling an individual pod's resources) or the [Horizontal Pod Autoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) (for scaling replicas).
We also recommend taking a look at [Keda](https://keda.sh/) if you're looking for more advanced autoscaling capabilities for pods.

## Deprovisioning
### How does Karpenter deprovision nodes?
See [Deprovisioning nodes]({{< ref "./concepts/disruption" >}}) for information on how Karpenter deprovisions nodes.

## Upgrading Karpenter

### How do I upgrade Karpenter?
Karpenter is a controller that runs in your cluster, but it is not tied to a specific Kubernetes version, as the Cluster Autoscaler is.
Use your existing upgrade mechanisms to upgrade your core add-ons in Kubernetes and keep Karpenter up to date on bug fixes and new features.

Karpenter requires proper permissions in the `KarpenterNode IAM Role` and the `KarpenterController IAM Role`.
To upgrade Karpenter to version `$VERSION`, make sure that the `KarpenterNode IAM Role` and the `KarpenterController IAM Role` have the right permission described in `https://karpenter.sh/$VERSION/getting-started/getting-started-with-karpenter/cloudformation.yaml`.
Next, locate `KarpenterController IAM Role` ARN (i.e., ARN of the resource created in [Create the KarpenterController IAM Role](../getting-started/getting-started-with-karpenter/#create-the-karpentercontroller-iam-role)) and pass them to the Helm upgrade command.
{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-karpenter/scripts/step08-apply-helm-chart.sh" language="bash"%}}

For information on upgrading Karpenter, see the [Upgrade Guide]({{< ref "./upgrading/upgrade-guide/" >}}).

## Upgrading Kubernetes Cluster

### How do I upgrade an EKS Cluster with Karpenter?

{{% alert title="Note" color="primary" %}}
Karpenter recommends that you always validate AMIs in your lower environments before using them in production environments. Read [Managing AMIs]({{<ref "./tasks/managing-amis" >}}) to understand best practices about upgrading your AMIs.

If using a custom AMI, you will need to trigger the rollout of new worker node images through the publication of a new AMI with tags matching the [`amiSelector`]({{<ref "./concepts/nodeclasses#specamiselectorterms" >}}), or a change to the [`amiSelector`]({{<ref "./concepts/nodeclasses#specamiselectorterms" >}}) field.
{{% /alert %}}

Karpenter's default behavior will upgrade your nodes when you've upgraded your Amazon EKS Cluster. Karpenter will [drift]({{<ref "./concepts/disruption#drift" >}}) nodes to stay in-sync with the EKS control plane version. Drift is enabled by default starting in `v0.33`. This means that as soon as your cluster is upgraded, Karpenter will auto-discover the new AMIs for that version.

Start by [upgrading the EKS Cluster control plane](https://docs.aws.amazon.com/eks/latest/userguide/update-cluster.html). After the EKS Cluster upgrade completes, Karpenter will Drift and disrupt the Karpenter-provisioned nodes using EKS Optimized AMIs for the previous cluster version by first spinning up replacement nodes. Karpenter respects [Pod Disruption Budgets](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/) (PDB), and automatically [replaces, cordons, and drains those nodes]({{<ref "./concepts/disruption#control-flow" >}}). To best support pods moving to new nodes, follow Kubernetes best practices by setting appropriate pod [Resource Quotas](https://kubernetes.io/docs/concepts/policy/resource-quotas/) and using PDBs.

## Interruption Handling

### Should I use Karpenter interruption handling alongside Node Termination Handler?
No. We recommend against using Node Termination Handler alongside Karpenter due to conflicts that could occur from the two components handling the same events.

### Why should I migrate from Node Termination Handler?
Karpenter's native interruption handling offers two main benefits over the standalone Node Termination Handler component:
1. You don't have to manage and maintain a separate component to exclusively handle interruption events.
2. Karpenter's native interruption handling coordinates with other deprovisioning so that consolidation, expiration, etc. can be aware of interruption events and vice-versa.

### Why am I receiving QueueNotFound errors when I set `--interruption-queue`?
Karpenter requires a queue to exist that receives event messages from EC2 and health services in order to handle interruption messages properly for nodes.

Details on the types of events that Karpenter handles can be found in the [Interruption Handling Docs]({{< ref "./concepts/disruption/#interruption" >}}).

Details on provisioning the SQS queue and EventBridge rules can be found in the [Getting Started Guide]({{< ref "./getting-started/getting-started-with-karpenter/#create-the-karpenter-infrastructure-and-iam-roles" >}}).

## Consolidation

### Why do I sometimes see an extra node get launched when updating a deployment that remains empty and is later removed?

Consolidation packs pods tightly onto nodes which can leave little free allocatable CPU/memory on your nodes.  If a deployment uses a deployment strategy with a non-zero `maxSurge`, such as the default 25%, those surge pods may not have anywhere to run. In this case, Karpenter will launch a new node so that the surge pods can run and then remove it soon after if it's not needed.

## Logging

### How do I customize or configure the log output?

Karpenter uses [uber-go/zap](https://github.com/uber-go/zap) for logging. You can customize or configure the log messages by editing the [configmap-logging.yaml](https://github.com/aws/karpenter/blob/main/charts/karpenter/templates/configmap-logging.yaml)
`ConfigMap`'s [data.zap-logger-config](https://github.com/aws/karpenter/blob/main/charts/karpenter/templates/configmap-logging.yaml#L26) field.
The available configuration options are specified in the [zap.Config godocs](https://pkg.go.dev/go.uber.org/zap#Config).
