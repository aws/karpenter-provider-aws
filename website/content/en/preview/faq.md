---
title: "FAQs"
linkTitle: "FAQs"
weight: 90
description: >
  Review Karpenter Frequently Asked Questions
---
## General

### How does a provisioner decide to manage a particular node?
See [Configuring provisioners]({{< ref "./concepts/#configuring-provisioners" >}}) for information on how Karpenter provisions and manages nodes.

### What cloud providers are supported?
AWS is the first cloud provider supported by Karpenter, although it is designed to be used with other cloud providers as well.

### Can I write my own cloud provider for Karpenter?
Yes, but there is no documentation yet for it.
Start with Karpenter's GitHub [cloudprovider](https://github.com/aws/karpenter-core/tree{{< githubRelRef >}}pkg/cloudprovider) documentation to see how the AWS provider is built, but there are other sections of the code that will require changes too.

### What operating system nodes does Karpenter deploy?
By default, Karpenter uses Amazon Linux 2 images.

### Can I provide my own custom operating system images?
Karpenter has multiple mechanisms for configuring the [operating system]({{< ref "./concepts/operating-systems/" >}}) for your nodes.

### Can Karpenter deal with workloads for mixed architecture cluster (arm vs. amd)?
Karpenter is flexible to multi architecture configurations using [well known labels]({{< ref "./concepts/scheduling/#supported-labels">}}).

### What RBAC access is required?
All of the required RBAC rules can be found in the helm chart template.
See [clusterrolebinding.yaml](https://github.com/aws/karpenter/blob{{< githubRelRef >}}charts/karpenter/templates/clusterrolebinding.yaml), [clusterrole.yaml](https://github.com/aws/karpenter/blob{{< githubRelRef >}}charts/karpenter/templates/clusterrole.yaml), [rolebinding.yaml](https://github.com/aws/karpenter/blob{{< githubRelRef >}}charts/karpenter/templates/rolebinding.yaml), and [role.yaml](https://github.com/aws/karpenter/blob{{< githubRelRef >}}charts/karpenter/templates/role.yaml) files for details.

### Can I run Karpenter outside of a Kubernetes cluster?
Yes, as long as the controller has network and IAM/RBAC access to the Kubernetes API and your provider API.

## Compatibility

### Which versions of Kubernetes does Karpenter support?
Karpenter is tested with Kubernetes v1.20-v1.24.

### What Kubernetes distributions are supported?
Karpenter documents integration with a fresh install of the latest AWS Elastic Kubernetes Service (EKS).
Existing EKS distributions can be used, but this use case has not yet been documented.
Other Kubernetes distributions (KOPs, etc.) can be used, but setting up cloud provider permissions for those distributions has not been documented.

### How does Karpenter interact with AWS node group features?
Provisioners are designed to work alongside static capacity management solutions like EKS Managed Node Groups and EC2 Auto Scaling Groups.
You can manage all capacity using provisioners, use a mixed model with dynamic and statically managed capacity, or use a fully static approach.
We expect most users will use a mixed approach in the near term and provisioner-managed in the long term.


### How does Karpenter interact with Kubernetes features?
* Kubernetes Cluster Autoscaler: Karpenter can work alongside cluster autoscaler.
See [Kubernetes cluster autoscaler]({{< ref "./concepts/overview/#kubernetes-cluster-autoscaler" >}}) for details.
* Kubernetes Scheduler: Karpenter focuses on scheduling pods that the Kubernetes scheduler has marked as unschedulable.
See [Scheduling]({{< ref "./concepts/scheduling" >}}) for details on how Karpenter interacts with the Kubernetes scheduler.

## Provisioning

### What features does the Karpenter provisioner support?
See [Provisioner API]({{< ref "./concepts/provisioning" >}}) for provisioner examples and descriptions of features.

### Can I create multiple (team-based) provisioners on a cluster?
Yes, provisioners can identify multiple teams based on labels.
See [Provisioner API]({{< ref "./concepts/provisioning" >}}) for details.

### If multiple provisioners are defined, which will my pod use?

Pending pods will be handled by any Provisioner that matches the requirements of the pod.
There is no ordering guarantee if multiple provisioners match pod requirements.
We recommend that Provisioners are setup to be mutually exclusive.
Read more about this recommendation in the [EKS Best Practices Guide for Karpenter](https://aws.github.io/aws-eks-best-practices/karpenter/#create-provisioners-that-are-mutually-exclusive).
To select a specific provisioner, use the node selector `karpenter.sh/provisioner-name: my-provisioner`.

### How can I configure Karpenter to only provision pods for a particular namespace?

There is no native support for namespaced based provisioning.
Karpenter can be configured to provision a subset of pods based on a combination of taints/tolerations and node selectors.
This allows Karpenter to work in concert with the `kube-scheduler` in that the same mechanisms that `kube-scheduler` uses to determine if a pod can schedule to an existing node are also used for provisioning new nodes.
This avoids scenarios where pods are bound to nodes that were provisioned by Karpenter which Karpenter would not have bound itself.
If this were to occur, a node could remain non-empty and have its lifetime extended due to a pod that wouldn't have caused the node to be provisioned had the pod been unschedulable.

We recommend using Kubernetes native scheduling constraints to achieve namespace based scheduling segregation. Using native scheduling constraints ensures that Karpenter, `kube-scheduler` and any other scheduling or auto-provisioning mechanism all have an identical understanding of which pods can be scheduled on which nodes.  This can be enforced via policy agents, an example of which can be seen [here](https://blog.mikesir87.io/2022/01/creating-tenant-node-pools-with-karpenter/).

### Can I add SSH keys to a provisioner?

Karpenter does not offer a way to add SSH keys via provisioners or secrets to the nodes it manages.
However, you can use Session Manager (SSM) or EC2 Instance Connect to gain shell access to Karpenter nodes.
See [Node NotReady]({{< ref "./troubleshooting/#node-notready" >}}) troubleshooting for an example of starting an SSM session from the command line or [EC2 Instance Connect](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-connect-set-up.html) documentation to connect to nodes using SSH.

Though not recommended, if you need to access Karpenter-managed nodes without AWS credentials, you can add SSH keys using AWSNodeTemplate.
See [Custom User Data]({{< ref "./concepts/operating-systems/" >}}) for details.

### Can I set total limits of CPU and memory for a provisioner?
Yes, the setting is provider-specific.
See examples in [Accelerators, GPU]({{< ref "./concepts/node-templates/#accelerators-gpu" >}}) Karpenter documentation.

### Can I mix spot and on-demand EC2 run types?
Yes, see [Provisioning]({{< ref "./concepts/provisioning/" >}}) for an example.

### Can I restrict EC2 instance types?

* Attribute-based requests are currently not possible.
* You can select instances with special hardware, such as gpu.

### Can I use Bare Metal instance types?

Yes, Karpenter supports provisioning metal instance types when a Provisioner's `node.kubernetes.io/instance-type` Requirements only include `metal` instance types. If other instance types fulfill pod requirements, then Karpenter will prioritize all non-metal instance types before metal ones are provisioned.

### How does Karpenter dynamically select instance types?

Karpenter batches pending pods and then binpacks them based on CPU, memory, and GPUs required, taking into account node overhead, VPC CNI resources required, and daemonsets that will be packed when bringing up a new node.
By default Karpenter uses C, M, and R >= Gen 3 instance types, but it can be constrained in the provisioner spec with the [instance-type](https://kubernetes.io/docs/reference/labels-annotations-taints/#nodekubernetesioinstance-type) well-known label in the requirements section.
After the pods are binpacked on the most efficient instance type (i.e. the smallest instance type that can fit the pod batch), Karpenter takes 59 other instance types that are larger than the most efficient packing, and passes all 60 instance type options to an API called Amazon EC2 Fleet.
The EC2 fleet API attempts to provision the instance type based on an allocation strategy.
If you are using the on-demand capacity type, then Karpenter uses the `lowest-price` allocation strategy.
So fleet will provision the lowest priced instance type it can get from the 60 instance types Karpenter passed to the EC2 fleet API.
If the instance type is unavailable for some reason, then fleet will move on to the next cheapest instance type.
If you are using the spot capacity type, Karpenter uses the price-capacity-optimized allocation strategy. This tells fleet to find the instance type that EC2 has the most capacity for while also considering price. This allocation strategy will balance cost and decrease the probability of a spot interruption happening in the near term.
See [Choose the appropriate allocation strategy](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-fleet-allocation-strategy.html#ec2-fleet-allocation-use-cases) for information on fleet optimization.

### What if there is no Spot capacity? Will Karpenter use On-Demand?

The best defense against running out of Spot capacity is to allow Karpenter to provision as many different instance types as possible.
Even instance types that have higher specs, e.g. vCPU, memory, etc., than what you need can still be cheaper in the Spot market than using On-Demand instances.
When Spot capacity is constrained, On-Demand capacity can also be constrained since Spot is fundamentally spare On-Demand capacity.
Allowing Karpenter to provision nodes from a large, diverse set of instance types will help you to stay on Spot longer and lower your costs due to Spot’s discounted pricing.
Moreover, if Spot capacity becomes constrained, this diversity will also increase the chances that you’ll be able to continue to launch On-Demand capacity for your workloads.

If your Karpenter Provisioner specifies flexibility to both Spot and On-Demand capacity, Karpenter will attempt to provision On-Demand capacity if there is no Spot capacity available.
However, it’s strongly recommended that you specify at least 20 instance types in your Provisioner (or none and allow Karpenter to pick the best instance types) as our research indicates that this additional diversity increases the chances that your workloads will not need to launch On-Demand capacity at all.
Today, Karpenter will warn you if the number of instances in your Provisioner isn’t sufficiently diverse.

Technically, Karpenter has a concept of an “offering” for each instance type, which is a combination of zone and capacity type (equivalent in the AWS cloud provider to an EC2 purchase option – Spot or On-Demand).
Whenever the Fleet API returns an insufficient capacity error for Spot instances, those particular offerings are temporarily removed from consideration (across the entire provisioner) so that Karpenter can make forward progress with different options.

## Workloads

### How can someone deploying pods take advantage of Karpenter?

See [Application developer]({{< ref "./concepts/overview/#application-developer" >}}) for descriptions of how Karpenter matches nodes with pod requests.

### Can I use Karpenter with EBS disks per availability zone?
Yes.  See [Persistent Volume Topology]({{< ref "./concepts/scheduling#persistent-volume-topology" >}}) for details.

### Can I set `--max-pods` on my nodes?
Not yet.

## Deprovisioning
### How does Karpenter deprovision nodes?
See [Deprovisioning nodes]({{< ref "./concepts/deprovisioning" >}}) for information on how Karpenter deprovisions nodes.

## Upgrading

### How do I upgrade Karpenter?
Karpenter is a controller that runs in your cluster, but it is not tied to a specific Kubernetes version, as the Cluster Autoscaler is.
Use your existing upgrade mechanisms to upgrade your core add-ons in Kubernetes and keep Karpenter up to date on bug fixes and new features.

Karpenter requires proper permissions in the `KarpenterNode IAM Role` and the `KarpenterController IAM Role`.
To upgrade Karpenter to version `$VERSION`, make sure that the `KarpenterNode IAM Role` and the `KarpenterController IAM Role` have the right permission described in `https://karpenter.sh/$VERSION/getting-started/getting-started-with-eksctl/cloudformation.yaml`.
Next, locate `KarpenterController IAM Role` ARN (i.e., ARN of the resource created in [Create the KarpenterController IAM Role](../getting-started/getting-started-with-eksctl/#create-the-karpentercontroller-iam-role)) and the cluster endpoint, and pass them to the helm upgrade command
{{% script file="./content/en/preview/getting-started/getting-started-with-eksctl/scripts/step08-apply-helm-chart.sh" language="bash"%}}

For information on upgrading Karpenter, see the [Upgrade Guide]({{< ref "./upgrade-guide/" >}}).

### Why do I get an `unknown field "startupTaints"` error when creating a provisioner with startupTaints?

```bash
error: error validating "provisioner.yaml": error validating data: ValidationError(Provisioner.spec): unknown field "startupTaints" in sh.karpenter.v1alpha5.Provisioner.spec; if you choose to ignore these errors, turn validation off with --validate=false
```

The `startupTaints` parameter was added in v0.10.0.  Helm upgrades do not upgrade the CRD describing the provisioner, so it must be done manually. For specific details, see the [Upgrade Guide]({{< ref "./upgrade-guide/#upgrading-to-v0100" >}})

## Interruption Handling

### Should I use Karpenter interruption handling alongside Node Termination Handler?
No. We recommend against using Node Termination Handler alongside Karpenter due to conflicts that could occur from the two components handling the same events.

### Why should I migrate from Node Termination Handler?
Karpenter's native interruption handling offers two main benefits over the standalone Node Termination Handler component:
1. You don't have to manage and maintain a separate component to exclusively handle interruption events.
2. Karpenter's native interruption handling coordinates with other deprovisoining so that consolidation, expiration, etc. can be aware of interruption events and vice-versa.

### Why am I receiving QueueNotFound errors when I set `aws.interruptionQueueName`?
Karpenter requires a queue to exist that receives event messages from EC2 and health services in order to handle interruption messages properly for nodes.

Details on the types of events that Karpenter handles can be found in the [Interruption Handling Docs]({{< ref "./concepts/deprovisioning/#interruption" >}}).

Details on provisioning the SQS queue and EventBridge rules can be found in the [Getting Started Guide]({{< ref "./getting-started/getting-started-with-eksctl/#create-the-karpenter-infrastructure-and-iam-roles" >}}).

## Consolidation

### Why do I sometimes see an extra node get launched when updating a deployment that remains empty and is later removed?

Consolidation packs pods tightly onto nodes which can leave little free allocatable CPU/memory on your nodes.  If a deployment uses a deployment strategy with a non-zero `maxSurge`, such as the default 25%, those surge pods may not have anywhere to run. In this case, Karpenter will launch a new node so that the surge pods can run and then remove it soon after if it's not needed.
