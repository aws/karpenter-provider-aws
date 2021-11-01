---
title: "Concepts"
linkTitle: "Concepts"
weight: 35
---

Adding Karpenter to a Kubernetes cluster can dramatically improve the efficiency and cost of running workloads on that cluster.
Karpenter is tightly integrated with Kubernetes features to make sure that the right types and amounts of compute resources are available to pods as they are needed.
Karpenter carries out its duties by:

* Watching for pods that the Kubernetes scheduler has marked as unschedulable
* Evaluating scheduling constraints (resource requests, nodeselectors, affinities, tolerations, and topology spread constraints) requested by the pods
* Provisioning nodes that meet the requirements of the pods
* Scheduling the pods to run on the new nodes
* Removing the nodes when the nodes are no longer needed

In many cases, you can configure an unconstrained Karpenter provisioner when it is first installed and not change it again. 
Other times, you might want to continue to tweak the provisioner or even create multiple provisioners for a cluster where you tailor the requirements of different teams to use specific instance types, zones, or architectures.
Likewise, you might want to constrain the provisioner to use underlying cloud features, such as AWS spot instances.

Application developers can make specific requests for capacity and features you want from the nodes running your pods.
Karpenter is designed to quickly create the best possible nodes to meet those needs.

## Provisioners

Karpenter can be installed onto your Kubernetes cluster's data plane.
Its job is to add nodes to handle unschedulable pods, schedule pods on those nodes, and remove the nodes when they are not needed.
Karpenter's provisioning work is configured through [Provisioner CRDs](/docs/provisioner-crd/).

Here are some things to know about the Karpenter provisioner:

* **Unschedulable pods**: Karpenter only attempts to provision pods that has a status condition Unschedulable=True, which the kube scheduler sets when it fails to schedule the pod to existing capacity.

* **Provisioner CRD**: Provisioner CRDs define how the provisioner controller behaves.
Each provisioner manages a distinct set of nodes.
Each CRD contains constraints that impact the nodes that can be provisioned and attributes of those nodes (such timers for removing nodes).
See [Provisioner CRD](/docs/provisioner-crd/) for a description of settings.
* **Well-known labels**: The CRD can use well-known Kubernetes labels to allow pods to request only certain instance types, architectures, operating systems, or other attributes when creating nodes.
See [Well-Known Labels, Annotations and Taints](https://kubernetes.io/docs/reference/labels-annotations-taints/) for details.
Keep in mind that only a subset of these labels are supported in Karpenter, as described in [Provisioning](/docs/concepts/provisioner.md).
* **Time to live**: A provisioner can also include time-to-live values to indicate when nodes should be decommissioned after a set amount of time from when it was created or after it becomes empty of deployed pods.
* **Multiple provisioner CRDs**: An important feature of Karpenter is that it allows multiple provisioner CRDs on the same cluster.
One possible result is that you could configure different teams on the same cluster to run on completely separate capacity.
One team could run on GPU nodes while another uses storage nodes, with each having a different set of defaults.
Although you can address the needs of multiple teams in one CRD in some ways, having separate ones lets you isolate nodes for billing, use different defaults (such as no GPUs for a team), or use different expiration and scale-down values.
Having multiple CRDs could some day lead Karpenter to allow provisioners with different cloud providers on the same cluster or a control plane on one cloud and data plane on another.

## Scheduling

Besides creating nodes, Karpenter also schedules pods.
To avoid conflicts, Karpenter only schedules pods that the Kubernetes scheduler has marked unschedulable.
Karpenter optimistically creates the Node object and binds the pod.
This stateless approach avoids race conditions and improves performance.
If something is wrong with the launched node, Kubernetes will automatically migrate the pods to a new node.

Once Karpenter brings up a node, that node is available for the Kubernetes scheduler to schedule pods on it as well.
This is useful if there is additional room in the node due to imperfect packing shape or as workloads finish over time.

## Cloud provider
Karpenter makes requests to provision new nodes to the associated cloud provider.
The first supported cloud provider is AWS, although Karpenter is designed to work other cloud providers as well.
Separating Kubernetes and AWS-specific settings allows Karpenter a clean path to integrating with other cloud providers.

While using Kubernetes well-known labels, the provisioner can set some values that are specific to the cloud provider.
So, for example, to include a certain instance type, you could use the Kubernetes label `node.kubernetes.io/instance-type`, but set its value to an AWS instance type (such as `m5.large` or `m5.2xlarge`).

Choosing spot instances is one feature that is specific to AWS.
Cloud providers may choose to implement support for additional vendor specific labels like `node.k8s.aws/capacity-type`.
See [AWS labels](/docs/cloud-providers/aws/) for details.

## Kubernetes cluster autoscaler
Like Karpenter, [Cluster Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) is
designed to add nodes when requests come in to run pods that cannot be met by current capacity.
Cluster autoscaler is part of the Kubenetes project, with implementations by most major Kubernetes cloud providers.
By taking a fresh look at provisioning, Karpenter offers the following improvements over the older cluster autoscaler:

* **Designed to handle the full flexibility of the cloud**:
Cluster autoscaler was not built with the flexibility to handle hundreds of instance types, zones, and purchase options.
Karpenter has the ability to efficiently address the full range of instance types available through AWS.

* **Adds scheduling**: Cluster autoscaler doesn’t bind pods to the nodes it creates.
A node that Karpenter creates knows to run the bound pods immediately when the node comes up.
I doesn't have to wait for the scheduler or for the node to become ready.
It can start pulling the container images it needs immediately.
This can save seconds of performance.

## Constraints
Understanding the concept of layered constraints is key to using Karpenter.
With no constraints defined in provisioners and none requested from pods being deployed, Karpenter on AWS is free to choose from the entire universe of features available from EC2.
Nodes can be created using any instance type and run in any zones.
On-demand or spot instances can be used.

The effects of layering constraints are applied when a pod makes requests for compute resources that cannot be met by current capacity.
A pod can request *affinity* (such as to run in a particular zone or instance type) or a topology spread (to require that a set of pods be run across multiple nodes).
The pod can ask to run only on nodes with a particular label or that have a certain amount of available memory or particular architecture type.

The Kubernetes scheduler then tries to match those constraints with available nodes.
If the node is unschedulable, facilities like the cluster autoscaler or Karpenter come in and try to create compute resources to match each pod's needs.
When Karpenter tries to provision a node that can schedule an unschedulable pode, it compares its own constraints when choosing the node to create.
The Karpenter provisoners configured for the cluster will look to best match the request, using mostly the same well-known labels and affinity features that the pod uses.

For details about how to set constraints when configuring a Karpenter provider, see [Karpenter provisioner and scheduler](/docs/concepts/provisioner.md) for details.
For more on how, as a developer, you can add constraints to your pod spec deployment, see [Karpenter pod developer concepts](/docs/concepts/running-pods.md) for details.

## Deprovisioning nodes

Karpenter not only manages the creation of nodes to add cluster capacity, but also will delete nodes when that capacity is no longer needed.
There are several things you should know about how Karpenter deprovisions nodes:

* **Node Expiry**: If a node expiry time-to-live value (`ttlSecondsUntilExpired`) is reached, that node is drained of pods and deleted (even if it is still running workloads).
* **Empty nodes**: When the last workload pod running on a Karpenter-managed node is gone, a clock starts keeping track of how long the node is empty.
Once that "node empty" time-to-live (`ttlSecondsAfterEmpty`) is reached, that node is deleted.
* **Finalizer**: Karpenter places a finalizer bit on the nodes it creates.
When a request comes in to delete one of those nodes (from an expired TTL or maybe an explicit `kubectl delete node`), Karpenter will drain all the pods and then terminate the EC2 instance.
Karpenter handles all clean-up work needed to properly delete the node.

For more details on how Karpenter deletes nodes, see [Deleting nodes with Karpenter](/docs/concepts/delete-nodes.md) for details.

## Karpenter workflows

To further understand Karpenter concepts and how Karpenter works, refer to concept documentation on the following workflows:

* **Setting up Karpenter**: To get started with Karpenter, an operator needs to deploy Karpenter on a running cluster and provide it with the necessary permissions (IAM Roles for Service Accounts for AWS, in this example) to create and delete nodes on the cloud provider from within Kubernetes.
* **Configuring Provisioners**: Configuring one or more provisioners on a cluster allows an operator to define the constraints by which nodes are created and deleted on the assigned cluster.
* **Deploying pods**: A developer that is configuring pods can request that features like taints and affinity be met by the nodes that are created to run those pods.
Karpenter is made to match those requests to the best available nodes, then schedule the pods on those nodes.
* **Deleting nodes**: Karpenter will drain pods and delete nodes based on direct requests or timer expirations.

Additional Karpenter concepts pages describe what you need to understand Karpenter in different workflows:
