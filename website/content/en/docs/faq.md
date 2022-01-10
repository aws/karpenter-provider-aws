---
title: "FAQs"
linkTitle: "FAQs"
weight: 90
---
## General

### How does a provisioner decide to manage a particular node?
Karpenter only provisions nodes for pods with a status condition `Unschedulable=True`
Karpenter will only take action on nodes that it provisions.
All nodes launched by Karpenter will be labeled with `karpenter.sh/provisioner-name`.

### What cloud providers are supported?
AWS is the first cloud provider supported by Karpenter, although it is designed to be used with other cloud providers as well.
See [[Cloud provider]({{< ref "/docs/concepts/#cloud-provider" >}}) for details.

### What deployment methods are supported by Karpenter?
To deploy Karpenter manually, see [[Getting Started]({{< ref "/docs/getting-started/" >}}).
To deploy using Terraform, see [[Getting Started with Terraform]({{< ref "/docs/getting-started-with-terraform/" >}}).
To deploy using kOps, see [[Getting Started with Terraform]({{< ref "/docs/getting-started-with-kops/" >}}).

### Can I write my own cloud provider for Karpenter?
!!! NEEDS INFO !!!

### What operating system nodes does Karpenter deploy?
By default, Karpenter uses Amazon Linux 2 images.

### Can I provide my own custom operating system images?
Karpenter allows you to create your own AWS AMIs using custom launch templates.
See [Launch Templates and Custom Images]({{< ref "/docs/aws/launch-templates/" >}}) for details.

### Can Karpenter deal with workloads for mixed architecture cluster (arm vs. amd)?
Yes. Build and prepare custom arm images as described in [Launch Templates and Custom Images]({{< ref "/docs/aws/launch-templates/" >}}).
Specify the desired architecture when you deploy workloads.

### What RBAC access is required?
!!! NEEDS INFO !!!

### Can I run Karpenter outside of a Kubernetes cluster?
!!! NEEDS INFO !!!

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
!!! NEEDS INFO !!!

### Can I mix spot and on-demand EC2 run types?
Yes, see [Example Provisioner Resource]({{< ref "/docs/provisioner/#example-provisioner-resource" >}}) for an example.

### Can I restrict EC2 instance types?

* Attribute-based requests are currently not possible.
* You can select instances with special hardware, such as gpu.

### Can I identify EC2 instances using common labels?
!!! NEEDS INFO !!!

## Workloads

### How can someone deploying pods take advantage of Karpenter?

See [Application developer]({{< ref "/docs/concepts/#application-developer" >}}) for descriptions of how Karpenter matches nodes with pod requests.

### How do I use Karpenter with the AWS load balancer controller?

* Set the [ALB target type]({{< ref "https://kubernetes-sigs.github.io/aws-load-balancer-controller/v2.3/guide/ingress/annotations/#target-type" >}}) to IP mode for the pods 
* Set [readiness gate]({{< ref "https://kubernetes-sigs.github.io/aws-load-balancer-controller/v2.3/deploy/pod_readiness_gate/" >}}) on the namespace.
The default is round robin at node level.
For Karpenter, not all nodes are equal.

### Can I use Karpenter with ELB disks per availability zone?
Not yet.

### Can I set --max-pods on my nodes?
Not yet.

## Deprovisioning
### How does Karpenter deprovision nodes?
See [Deprovisioning nodes]({{< ref "/docs/tasks/deprov-nodes" >}}) for information on how Karpenter deprovisions nodes.

## Upgrading
### How do I upgrade Karpenter?
!!! NEEDS INFO !!!

### How do I upgrade Kubernetes?
!!! NEEDS INFO !!!
