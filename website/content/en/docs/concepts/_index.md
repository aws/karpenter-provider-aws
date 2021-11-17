---
title: "Concepts"
linkTitle: "Concepts"
weight: 35
---

Those who use Karpenter fall under two basic roles: Kubernetes cluster operators and application developers.
This document describes Karpenter concepts through the lens of those two types of users.

## Cluster operator

As a Kubernetes cluster operator, you can engage with Karpenter to:

* Install Karpenter 
* Configure provisioners to set constraints and other features for managing nodes
* Deprovision nodes
* Upgrade nodes

Concepts associated with this role are described below.


### Installing Karpenter

Karpenter is made to be installed on the control plane of a Kubernetes cluster.
As part of the installation process, you need credentials from the underlying cloud provider to allow nodes to be started up and added to the cluster as they are needed.

[Getting Started with Karpenter on AWS](https://karpenter.sh/docs/getting-started/)
describes the process of installing Karpenter on an AWS cloud provider.
Because requests to add and delete nodes and schedule pods are made through Kubernetes, AWS IAM Roles for Service Accounts (IRSA) are needed by your Kubernetes cluster to make privileged requests to AWS.
For example, Karpenter uses AWS IRSA roles to grant permissions needed to run containers, configure networking, and create EC2 instances.

Once privileges are in place, deployment of Karpenter itself is done using Helm charts.

### Configuring provisioners

Karpenter's job is to add nodes to handle unschedulable pods, schedule pods on those nodes, and remove the nodes when they are not needed.
To configure Karpenter, you create *provisioners* to add constraints to how Karpenter manages unschedulable pods and expires nodes.
Here are some things to know about the Karpenter provisioner:

* **Unschedulable pods**: Karpenter only attempts to provision pods that have a status condition `Unschedulable=True`, which the kube scheduler sets when it fails to schedule the pod to existing capacity.

* **Provisioner CR**: Karpenter defines a Custom Resource called a Provisioner to specify provisioning configuration.
Each provisioner manages a distinct set of nodes.
A provisioner contains constraints that impact the nodes that can be provisioned and attributes of those nodes (such timers for removing nodes).
See [Provisioner](/docs/provisioner-crd/) for a description of settings and the [Provisioning](/docs/tasks/provisioner.md) task for of provisioner examples. 

* **Well-known labels**: The provisioner can use well-known Kubernetes labels to allow pods to request only certain instance types, architectures, operating systems, or other attributes when creating nodes.
See [Well-Known Labels, Annotations and Taints](https://kubernetes.io/docs/reference/labels-annotations-taints/) for details.
Keep in mind that only a subset of these labels are supported in Karpenter, as described later.

* **Deprovisioning nodes**: A provisioner can also include time-to-live values to indicate when nodes should be deprovisioned after a set amount of time from when they were created or after they becomes empty of deployed pods.

* **Multiple provisioners**: Multiple provisioners can be configured on the same cluster.
For example, you might want to configure different teams on the same cluster to run on completely separate capacity.
One team could run on GPU nodes while another uses storage nodes, with each having a different set of defaults.

Although some use cases are addressed witha single provisioner for multiple teams, multiple provisioners are useful to isolate nodes for billing, use different defaults (such as no GPUs for a team), or use different deprovisioning settings.

In the future, multiple provisioners would enable to provision nodes from different cloud providers on the same cluster or even a control plane on one cloud and data plane on another.

### Deprovisioning nodes

Karpenter not only manages the creation of nodes to add cluster capacity, but also will delete nodes when that capacity is no longer needed.
There are several things you should know about how Karpenter deprovisions nodes:

* **Node Expiry**: If a node expiry time-to-live value (`ttlSecondsUntilExpired`) is reached, that node is drained of pods and deleted (even if it is still running workloads).
* **Empty nodes**: When the last workload pod running on a Karpenter-managed node is gone, a clock starts keeping track of how long the node is empty.
Once that "node empty" time-to-live (`ttlSecondsAfterEmpty`) is reached, that node is deleted.
* **Finalizer**: Karpenter places a finalizer bit on each node it creates.
When a request comes in to delete one of those nodes (from an expired TTL or maybe an explicit `kubectl delete node`), Karpenter will drain all the pods and then terminate the EC2 instance.
Karpenter handles all clean-up work needed to properly delete the node.

For more details on how Karpenter deletes nodes, see [Deleting nodes with Karpenter](/docs/tasks/delete-nodes.md) for details.

### Upgrading nodes

A straight-forward way to upgrade nodes that are managed by Karpenter is to add new upgraded nodes, move pod workloads from the old nodes to the new ones, and delete the old nodes.
Another way to do upgrades is to set `ttlSecondsUntilExpired` values so old nodes are deleted after a set period of time, so they can then be replaced with newer nodes.
For details on upgrading nodes with Karpenter, see [Upgrading nodes with Karpenter](/docs/tasks/upgrade-nodes.md) for details.

### Cluster operator concepts

Understanding the following concepts will help you in carrying out the tasks just described.

#### Constraints

The concept of layered constraints is key to using Karpenter.
With no constraints defined in provisioners and none requested from pods being deployed, Karpenter on AWS is free to choose from the entire universe of features available from EC2.
Nodes can be created using any instance type and run in any zones.
On-demand or spot instances can be used.

Most of the same constraints a Karpenter cluster operator can set up on a provisioner can be used by an application developer in a podspec to request particular features from a node.
Refer to the description of Karpenter constraints in the Application Developer section below for details.

#### Scheduling

To avoid conflicts, Karpenter only schedules pods that the Kubernetes scheduler has marked unschedulable.
Karpenter optimistically creates the Node object and binds the pod.
This stateless approach helps to avoid race conditions and improves performance.
If something is wrong with the launched node, Kubernetes will automatically migrate the pods to a new node.

Once Karpenter brings up a node, that node is available for the Kubernetes scheduler to schedule pods on it as well.
This is useful if there is additional room in the node due to imperfect packing shape or because workloads finish over time.

#### Cloud provider
Karpenter makes requests to provision new nodes to the associated cloud provider.
The first supported cloud provider is AWS, although Karpenter is designed to eventually work with other cloud providers as well.
Separating Kubernetes and AWS-specific settings allows Karpenter a clean path to integrating with other cloud providers.

While using Kubernetes well-known labels, the provisioner can set some values that are specific to the cloud provider.
So, for example, to include a certain instance type, you could use the Kubernetes label `node.kubernetes.io/instance-type`, but set its value to an AWS instance type (such as `m5.large` or `m5.2xlarge`).

Choosing spot instances is one feature that is currently specific to AWS, but one that could be generalized to other providers.
Furhter, cloud providers may choose to implement support for additional vendor specific labels like `node.k8s.aws/capacity-type`.
See [AWS labels](/docs/cloud-providers/aws/) for details.

#### Kubernetes cluster autoscaler
Like Karpenter, [Kubernetes Cluster Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) is
designed to add nodes when requests come in to run pods that cannot be met by current capacity.
Cluster autoscaler is part of the Kubenetes project, with implementations by most major Kubernetes cloud providers.
By taking a fresh look at provisioning, Karpenter offers the following improvements:

* **Designed to handle the full flexibility of the cloud**:
Karpenter has the ability to efficiently address the full range of instance types available through AWS.
Cluster autoscaler was not originally built with the flexibility to handle hundreds of instance types, zones, and purchase options.

* **Adds scheduling**: Cluster autoscaler doesn’t bind pods to the nodes it creates.
A node that Karpenter creates knows to run the bound pods immediately when the node comes up.
I doesn't have to wait for the scheduler or for the node to become ready.
It can start pulling the container images it needs immediately.
This can save seconds of performance.

## Application developer

As someone deploying pods that might be evaluated by Karpenter, you should know how to request the properties that your pods need of its compute resources.
Karpenter's job is to efficiently assess and choose compute assets based on requests from pod deployments.
These can include basic Kubernetes features or features that are specific to the cloud provider (such as AWS).

Layered *constraints* are applied when a pod makes requests for compute resources that cannot be met by current capacity.
A pod can request *affinity* (to run in a particular zone or instance type) or a topology spread (to cause a set of pods be run across multiple nodes).
The pod can ask to run only on nodes with a particular label or that have a certain amount of available memory or particular architecture type.

The Kubernetes scheduler tries to match those constraints with available nodes.
If the pod is unschedulable, facilities like the Cluster Autoscaler or Karpenter come in and try to create compute resources to match each pod's needs.
When Karpenter tries to provision a node that can schedule an unschedulable pode, it compares its own constraints when choosing the node to create.

As long as the requests are not outside of the provisioner's constraints, 
Karpenter will look to best match the request, comparing mostly the same well-known labels and affinity features that the pod uses.
Note that if the constraints are such that a match is not possible, the pod will not deploy.

So, what constraints can you use as an application developer deploying pods that could be managed by Karpenter?

* **Kubernetes**: Kubernetes features that Karpenters supports for scheduling nodes include node affinity based on [persistant volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#node-affinity) and [nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector).
It also supports [PodDisruptionBudget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) and [topologySpreadConstraints](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/).

  From the Kubernetes [Well-Known Labels, Annotations and Taints](https://kubernetes.io/docs/reference/labels-annotations-taints/) page,
you can see a full list of Kubernetes labels, annotations and taints that determine scheduling.
Only a small set of them are implemented in Karpenter, including:

  * **kubernetes.io/arch**: For example, kubernetes.io/arch=amd64
  * **kubernetes.io/os**: For example, kubernetes.io/os=linux
  * **node.kubernetes.io/instance-type**: For example, node.kubernetes.io/instance-type=m3.medium
  * **topology.kubernetes.io/zone**: For example, topology.kubernetes.io/zone=us-east-1c


* **AWS-specific**: Capacity types (node.k8s.aws/capacity-type) that are specific to AWS in Karpenter include spot and on-demand.
Karpenter also supports AWS-specific accelerators, including:

  * **nvidia.com/gpu**: For Nvidia graphics processing units
  * **amd.com/gpu**: For AMD graphics processing units
  * **aws.amazon.com/neuron**: For AWS Neuron machine learning applications

**NOTE**: Don't use podAffinity and podAntiAffinity to schedule pods on the same or different nodes as other pods.
Kubernetes SIG scalability recommends against these features and Karpenter doesn't support them.

For more on how, as a developer, you can add constraints to your pod deployment, see [Running pods](/docs/tasks/running-pods.md) for details.
