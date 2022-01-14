---
title: "FAQs"
linkTitle: "FAQs"
weight: 90
---
## General

### How does a provisioner decide to manage a particular node?
See [Configuring provisioners]({{< ref "/docs/concepts/#configuring-provisioners" >}}) for information on how Karpenter provisions and manages nodes.

### What cloud providers are supported?
AWS is the first cloud provider supported by Karpenter, although it is designed to be used with other cloud providers as well.
See [[Cloud provider]({{< ref "/docs/concepts/#cloud-provider" >}}) for details.

### Can I write my own cloud provider for Karpenter?
Yes, but there is no documentation yet for it.
Start with Karpenter's GitHub [cloudprovider](https://github.com/aws/karpenter/tree/main/pkg/cloudprovider) documentation to see how the AWS provider is built, but there are other sections of the code that will require changes too.

### What operating system nodes does Karpenter deploy?
By default, Karpenter uses Amazon Linux 2 images.

### Can I provide my own custom operating system images?
Karpenter allows you to create your own AWS AMIs using custom launch templates.
See [Launch Templates and Custom Images]({{< ref "/docs/aws/launch-templates/" >}}) for details.

### Can Karpenter deal with workloads for mixed architecture cluster (arm vs. amd)?
Yes. Build and prepare custom arm images as described in [Launch Templates and Custom Images]({{< ref "/docs/aws/launch-templates/" >}}).
Specify the desired architecture when you deploy workloads.

### What RBAC access is required?
All of the required RBAC rules can be found in the helm chart template.
See the [rbac.yaml](https://github.com/aws/karpenter/blob/main/charts/karpenter/templates/controller/rbac.yaml) file for details.

### Can I run Karpenter outside of a Kubernetes cluster?
Yes, as long as the controller has network and IAM/RBAC access to the Kubernetes API and your provider API.

## Compatibility

### Which versions of Kubernetes does Karpenter support?
Karpenter is tested with Kubernetes v1.19 and later.

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
See [Kubernetes cluster autoscaler]({{< ref "/docs/concepts/#kubernetes-cluster-autoscaler" >}}) for details.
* Kubernetes Scheduler: Karpenter focuses on scheduling pods that the Kubernetes scheduler has marked as unschedulable.
See [Scheduling]({{< ref "/docs/concepts/#scheduling" >}}) for details on how Karpenter interacts with the Kubernetes scheduler.

## Provisioning
### What features does the Karpenter provisioner support?
See [Provisioner API]({{< ref "/docs/provisioner" >}}) for provisioner examples and descriptions of features.

### Can I create multiple (team-based) provisioners on a cluster?
Yes, provisioners can identify multiple teams based on labels.
See [Provisioner API]({{< ref "/docs/provisioner" >}}) for details.

### If multiple provisioners are defined, which will my pod use?

By default, pods will use the rules defined by a provisioner named default.
This is analogous to the default scheduler.
To select an alternative provisioner, use the node selector `karpenter.sh/provisioner-name: alternative-provisioner`.
You must either define a default provisioner or explicitly specify `karpenter.sh/provisioner-name node selector`.

### Can I set total limits of CPU and memory for a provisioner?
Yes, the setting is provider-specific.
See examples in [Accelerators, GPU]({{< ref "/docs/aws/provisioning/#accelerators-gpu" >}}) Karpenter documentation.

### Can I mix spot and on-demand EC2 run types?
Yes, see [Example Provisioner Resource]({{< ref "/docs/provisioner/#example-provisioner-resource" >}}) for an example.

### Can I restrict EC2 instance types?

* Attribute-based requests are currently not possible.
* You can select instances with special hardware, such as gpu.

## Workloads

### How can someone deploying pods take advantage of Karpenter?

See [Application developer]({{< ref "/docs/concepts/#application-developer" >}}) for descriptions of how Karpenter matches nodes with pod requests.

### How do I use Karpenter with the AWS load balancer controller?

* Set the [ALB target type]({{< ref "https://kubernetes-sigs.github.io/aws-load-balancer-controller/v2.3/guide/ingress/annotations/#target-type" >}}) to IP mode for the pods.
Use IP targeting if you want the pods to receive equal weight.
Instance balancing could greatly skew the traffic being sent to a node without also managing host spread of the workload.
* Set [readiness gate]({{< ref "https://kubernetes-sigs.github.io/aws-load-balancer-controller/v2.3/deploy/pod_readiness_gate/" >}}) on the namespace.
The default is round robin at the node level.
For Karpenter, not all nodes are equal.
For example, each node will have different performance characteristics and a different number of pods running on it.
A `t3.small` with three instances should not receive the same amount of traffic as a `m5.4xlarge` with dozens of pods.
If you don't specify a spread at the workload level, or limit what instances should be picked, you could get the same amount of traffic sent to the `t3` and `m5`.

### Can I use Karpenter with EBS disks per availability zone?
Not yet.

### Can I set `--max-pods` on my nodes?
Not yet.

## Deprovisioning
### How does Karpenter deprovision nodes?
See [Deprovisioning nodes]({{< ref "/docs/tasks/deprov-nodes" >}}) for information on how Karpenter deprovisions nodes.

## Upgrading
### How do I upgrade Karpenter?
Karpenter is a controller that runs in your cluster, but it is not tied to a specific Kubernetes version, as the Cluster Autoscaler is.
Use your existing upgrade mechanisms to upgrade your core add-ons in Kubernetes and keep Karpenter up to date on bug fixes and new features.
