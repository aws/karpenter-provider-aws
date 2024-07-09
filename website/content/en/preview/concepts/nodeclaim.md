---
title: "NodeClaims"
linkTitle: "NodeClaims"
weight: 30
description: >
  Understand NodeClaims
---

Karpenter uses NodeClaims to perform node autoscaling based on the requests of unschedulable pods and the
requirements of existing [NodePool]({{<ref "./nodepools" >}}) and associated [NodeClasses]({{<ref "./nodeclasses" >}}).
While NodeClaims require no direct user input, as a Karpenter user you can monitor NodeClaims to keep track of
the status of your nodes in cases where something goes wrong or a node drifts from its intended state.

Karpenter uses NodeClaims as a merged representation of the cloud provider instance and the node on the cluster.
Karpenter creates NodeClaims in response to provisioning and disruption needs (pre-spin). Whenever Karpenter
creates a NodeClaim, it asks the cloud provider to create the instance (launch), register the created node
with the node claim (registration), and wait for the node and its resources to be ready (initialization).

This page describes how NodeClaims relate to other components of Karpenter and the related cloud provider.

If you want to learn more about the nodes being managed by Karpenter, depending on what you are interested in,
you can either look directly at the NodeClaim or at the nodes they are associated with:

* Checking NodeClaims: If something goes wrong in the process of creating a node, you can look at the NodeClaim
to see where the node creation process might have stalled. Using `kubectl get nodeclaims` you can see the NodeClaims
for the cluster and using `kubectl describe nodeclaim <nodeclaim>` you can see the status of a particular NodeClaim.
For example, if the node is not available, you might see statuses indicating that the node has expired, is empty, or has drifted.

* Checking nodes: Use commands such as `kubectl get node` and  `kubectl describe node <nodename>` to see the actual resources,
labels, and other attributes associated with a particular node.

## NodeClaim roles in node creation

NodeClaims provide a critical role in the Karpenter workflow for creating instances and registering them as nodes, then later in responding to node disruptions.

The following diagram illustrates how NodeClaims interact with other components during Karpenter-driven node creation.

![nodeclaim-node-creation](/nodeclaims.png)

{{% alert title="Note" color="primary" %}}
Assuming that Karpenter is running in the default `kube-system` namespace, if you want to follow along with the Karpenter logs in your cluster, do the following:

```bash
export KARPENTER_NAMESPACE="kube-system"
kubectl logs -f -n "${KARPENTER_NAMESPACE}" \
   -l app.kubernetes.io/name=karpenter -c controller
```
In a separate terminal, start some pods that would require Karpenter to create nodes to handle those pods.
For example, start up some inflate pods as described in [Scale up deployment]({{< ref "../getting-started/getting-started-with-karpenter/#6-scale-up-deployment" >}}).
{{% /alert %}}

As illustrated in the previous diagram, Karpenter interacts with NodeClaims and related components when creating a node:

1. Watches for pods and monitors NodePools and NodeClasses:
    * Checks what the pod needs, such as requests for CPU, memory, architecture, and so on.
    * Checks the constraints imposed by existing NodePools and NodeClasses, such as allowing pods to only run in specific zones, on certain architectures, or on particular operating systems.
   Example of log messages at this stage:
    ```
    {"level":"INFO","time":"2024-06-22T02:24:16.114Z","logger":"controller","message":
       "found provisionable pod(s)","commit":"490ef94","controller":"provisioner",
       "Pods":"default/inflate-66fb68585c-xvs86, default/inflate-66fb68585c-hpcdz,
       default/inflate-66fb68585c-8xztf, default/inflate-66fb68585c-t29d8,
       default/inflate-66fb68585c-nxflz","duration":"100.761702ms"}
    ```

2. Asks the Kubernetes API server to create a NodeClaim object to satisfy the pod and NodePool needs.
   Example of log messages at this stage:
    ```
    {"level":"INFO","time":"2024-06-22T02:24:16.114Z","logger":"controller","message":
       "computed new nodeclaim(s) to fit pod(s)","commit":"490ef94","controller":
       "provisioner","nodeclaims":1,"pods":5}
    ```
3. Finds the new NodeClaim and checks its requirements (pre-spin)
    ```
    {"level":"INFO","time":"2024-06-22T02:24:16.128Z","logger":"controller","message":   "created nodeclaim","commit":"490ef94","controller":"provisioner","NodePool":
       {"name":"default"},"NodeClaim":{"name":"default-sfpsl"},"requests":
       {"cpu":"5150m","pods":"8"},"instance-types":"c3.2xlarge, c4.2xlarge, c4.4xlarge,
       c5.2xlarge, c5.4xlarge and 55 other(s)"}
    ```
4. Based on the NodeClaim’s requirements, directs the cloud provider to create an instance that meets those requirements (launch):
   Example of log messages at this stage:
    ```
    {"level":"INFO","time":"2024-06-22T02:24:19.028Z","logger":"controller","message":   "launched nodeclaim","commit":"490ef94","controller":"nodeclaim.lifecycle",
       "controllerGroup":"karpenter.sh","controllerKind":"NodeClaim","NodeClaim":
       {"name":"default-sfpsl"},"namespace":"","name":"default-sfpsl","reconcileID":
       "9c9dbc80-3f0f-43ab-b01d-faac6c29e979","provider-id":
       "aws:///us-west-2b/i-08a3bf1cadb205c7e","instance-type":"c3.2xlarge","zone":
       "us-west-2b","capacity-type":"spot","allocatable":{"cpu":"7910m",
       "ephemeral-storage":"17Gi","memory":"13215Mi","pods":"58"}}
    ```
 
5. Gathers necessary metadata from the NodeClaim to check and setup the node.
6. Checks that the Node exists, and prepares it for use.  This includes:
    * Waiting for the instance to return a provider ID, instance type, zone, capacity type,
      and an allocatable status, indicating that the instance is ready.
    * Checking to see if the node has been synced, adding a finalizer to the node (this provides the same
      termination guarantees that all Karpenter nodes have), passing in labels, and updating the node owner references.
    * Checking if the node has been synced, adding a finalizer to the node (providing Karpenter termination guarantees), passing in labels, and updating the node owner references.
    * Making sure that the Node is ready to use. This includes such things as seeing if resources are registered and start-up taints are removed.
    * Checking the nodes for liveliness.
7. Registers the instance as a node in the Kubernetes cluster (registered). Example of log message at this stage:
    ```
    {"level":"INFO","time":"2024-06-22T02:24:39.998Z","logger":"controller","message":
       "registered nodeclaim","commit":"490ef94","controller":"nodeclaim.lifecycle",
       "controllerGroup":"karpenter.sh","controllerKind":"NodeClaim","NodeClaim":
       {"name":"default-sfpsl"},"namespace":"","name":"default-sfpsl","reconcileID":
       "4ae2c003-883c-4655-98b9-45871223a6a0",
       "provider-id":"aws:///us-west-2b/i-08a3bf1cadb205c7e",
       "Node":{"name":"ip-192-168-170-220.us-west-2.compute.internal"}}
    ```
    Finally, you can see the Node is now ready for use.
    ```
    {"level":"INFO","time":"2024-06-22T02:24:52.642Z","logger":"controller","message":
       "initialized nodeclaim","commit":"490ef94","controller":
       "nodeclaim.lifecycle","controllerGroup":"karpenter.sh","controllerKind":
       "NodeClaim","NodeClaim":{"name":"default-sfpsl"},"namespace":
       "","name":"default-sfpsl","reconcileID":
       "7e7a671d-887f-428d-bd79-ddf603290f0a",
       "provider-id":"aws:///us-west-2b/i-08a3bf1cadb205c7e",
       "Node":{"name":"ip-192-168-170-220.us-west-2.compute.internal"},
       "allocatable":{"cpu":"7910m","ephemeral-storage":"18242267924",
       "hugepages-2Mi":"0","memory":"14320468Ki","pods":"58"}}
    ```
    At this point, the node is considered ready to go.

If a node doesn’t appear as registered after 15 minutes since it was first created, the NodeClaim is deleted.
Karpenter assumes there is a problem that isn’t going to be fixed.
No pods will be deployed there. After the node is deleted, Karpenter will try to create a new NodeClaim.

## NodeClaim drift and disruption

Although NodeClaims play a role in replacing nodes that drift or have been disrupted,
as someone using Karpenter, there is nothing particular you need to do with NodeClaims
in relation to those activities.
Just know that, if a Node become unusable and a new NodeClaim needs to be created,
it follows the same node creation process just decribed.

For details on Karpenter disruption, see [Disruption]({{< ref "./disruption" >}}).
