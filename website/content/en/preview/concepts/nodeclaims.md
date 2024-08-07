---
title: "NodeClaims"
linkTitle: "NodeClaims"
weight: 30
description: >
  Understand NodeClaims
---

Karpenter uses NodeClaims to manage the lifecycle of Kubernetes Nodes with the underlying cloud provider.
Karpenter's algorithm creates and deletes NodeClaims in response to the demands of the Pods in the cluster.
NodeClaims are created from the requirements of a [NodePool]({{<ref "./nodepools" >}}) and associated
[NodeClasses]({{<ref "./nodeclasses" >}}).
While you shouldn't modify the NodeClaim, as a Karpenter user, you can monitor NodeClaims to keep track of
the status of your nodes to understand how Karpenter is reasoning about your nodes.

Karpenter uses NodeClaims to request capacity with the cloud provider and is a running representation of that capacity.
Karpenter creates NodeClaims in response to provisioning and disruption needs (pre-spin). Whenever Karpenter
creates a NodeClaim, it asks the cloud provider to create the instance (launch), register and link the created node
with the NodeClaim (registration), and wait for the node and its resources to be ready (initialization).

This page describes how NodeClaims integrate throughout Karpenter and the cloud provider implementation.

If you want to learn more about the nodes being managed by Karpenter,
you can either look directly at the NodeClaim or at the nodes they are associated with:

* Checking NodeClaims: 
If something goes wrong in the process of creating a node, you can look at the NodeClaim
to see where the node creation process might have failed. `kubectl get nodeclaims` will show you the NodeClaims
for the cluster, and its linked nodes. Using `kubectl describe nodeclaim <nodeclaim>` will show the status of a particular NodeClaim.
For example, if the node is NotReady, you might see statuses indicating that the NodeClaim failed to launch, register, or initialize.
There will be logs emitted by the Karpenter controller to indicate this too.

* Checking nodes: 
Use commands such as `kubectl get node` and  `kubectl describe node <nodename>` to see the actual resources,
labels, and other attributes associated with a particular node.

## NodeClaim roles in node creation

NodeClaims provide a critical role in the Karpenter workflow for provisioning capacity, and in node disruptions.

The following diagram illustrates how NodeClaims interact with other components during Karpenter-driven node creation.

![nodeclaim-node-creation](/nodeclaims.png)

{{% alert title="Note" color="primary" %}}
Configure the `KARPENTER_NAMESPACE` environment variable to the namespace where you've installed Karpenter (`kube-system` is the default). Follow along with the Karpenter logs in your cluster and do the following:

```bash
export KARPENTER_NAMESPACE="kube-system"
kubectl logs -f -n "${KARPENTER_NAMESPACE}" \
   -l app.kubernetes.io/name=karpenter
```
In a separate terminal, start some pods that would require Karpenter to create nodes to handle those pods.
For example, start up some inflate pods as described in [Scale up deployment]({{< ref "../getting-started/getting-started-with-karpenter/#6-scale-up-deployment" >}}).
{{% /alert %}}

As illustrated in the previous diagram, Karpenter interacts with NodeClaims and related components when creating a node:

1. Watches for pods and monitors NodePools and NodeClasses:
    * Checks the pod scheduling constraints and resource requests.
    * Cross-references the requirements with the existing NodePools and NodeClasses, (e.g. zones, arch, os)
   Example log:
    ```
    {"level":"INFO","time":"2024-06-22T02:24:16.114Z","logger":"controller","message":
       "found provisionable pod(s)","commit":"490ef94","controller":"provisioner",
       "Pods":"default/inflate-66fb68585c-xvs86, default/inflate-66fb68585c-hpcdz,
       default/inflate-66fb68585c-8xztf, default/inflate-66fb68585c-t29d8,
       default/inflate-66fb68585c-nxflz","duration":"100.761702ms"}
    ```

2. Computes the shape and size of a NodeClaim to create in the cluster to fit the set of pods from step 1.
   Example log:
    ```
    {"level":"INFO","time":"2024-06-22T02:24:16.114Z","logger":"controller","message":
       "computed new nodeclaim(s) to fit pod(s)","commit":"490ef94","controller":
       "provisioner","nodeclaims":1,"pods":5}
    ```
3. Creates the NodeClaim object in the cluster.
   Example log:
    ```
    {"level":"INFO","time":"2024-06-22T02:24:16.128Z","logger":"controller","message":   "created nodeclaim","commit":"490ef94","controller":"provisioner","NodePool":
       {"name":"default"},"NodeClaim":{"name":"default-sfpsl"},"requests":
       {"cpu":"5150m","pods":"8"},"instance-types":"c3.2xlarge, c4.2xlarge, c4.4xlarge,
       c5.2xlarge, c5.4xlarge and 55 other(s)"}
    ```
4. Finds the new NodeClaim and translates it into an API call to create a cloud provider instance, logging
   the response of the API call. 

   If the API response is an unrecoverable error, such as an Insufficient Capacity, Karpenter will delete the NodeClaim, mark that instance type as temporarily unavailable, and create another NodeClaim if necessary.
   Example log:
    ```
    {"level":"INFO","time":"2024-06-22T02:24:19.028Z","logger":"controller","message":"launched nodeclaim","commit":"490ef94","controller":"nodeclaim.lifecycle",
       "controllerGroup":"karpenter.sh","controllerKind":"NodeClaim","NodeClaim":
       {"name":"default-sfpsl"},"namespace":"","name":"default-sfpsl","reconcileID":
       "9c9dbc80-3f0f-43ab-b01d-faac6c29e979","provider-id":
       "aws:///us-west-2b/i-01234567adb205c7e","instance-type":"c3.2xlarge","zone":
       "us-west-2b","capacity-type":"spot","allocatable":{"cpu":"7910m",
       "ephemeral-storage":"17Gi","memory":"13215Mi","pods":"58"}}
    ```
5. Karpenter watches for the instance to register itself with the cluster as a node, and updates the node's 
   labels, annotations and taints to match what was defined in the NodePool and NodeClaim. Once this step is 
   completed, Karpenter will remove the `karpenter.sh/unregistered` taint from the Node.

   If this fails to succeed within 15 minutes, Karpenter will remove the NodeClaim from the cluster and delete
   the underlying instance, creating another NodeClaim if necessary.
   Example log: 
   ```
   {"level":"INFO","time":"2024-06-22T02:26:19.028Z","logger":"controller","message":"registered nodeclaim","commit":"525136f-dirty","controller":"nodeclaim.lifecycle",
    "controllerGroup":"karpenter.sh","controllerKind":"NodeClaim",
    "NodeClaim":{"name":"default-xmbww"},"namespace":"","name":"default-xmbww","reconcileID":"1f1a350b-6b3a-4927-9632-074ee4a095da","provider-id":"aws:///us-west-2c/i-01234567adb205c7e","Node":{"name":"ip-xxx-xxx-xx-xxx.us-west-1.compute.internal"}}
   ```
6. Karpenter continues to watch the node, waiting until the node becomes ready, has all its startup taints removed, 
   and has all requested resources registered on the node. 
   Example log:
    ```
    {"level":"INFO","time":"2024-06-22T02:24:52.642Z","logger":"controller","message":
        "initialized nodeclaim","commit":"490ef94","controller":
        "nodeclaim.lifecycle","controllerGroup":"karpenter.sh","controllerKind":
        "NodeClaim","NodeClaim":{"name":"default-sfpsl"},"namespace":
        "","name":"default-sfpsl","reconcileID":
        "7e7a671d-887f-428d-bd79-ddf603290f0a",
        "provider-id":"aws:///us-west-2b/i-01234567adb205c7e",
        "Node":{"name":"ip-xxx-xxx-xx-xxx.us-west-1.compute.internal"},
        "allocatable":{"cpu":"7910m","ephemeral-storage":"18242267924",
        "hugepages-2Mi":"0","memory":"14320468Ki","pods":"58"}}
    ```

As they are booting up, NodeClaims are included in the scheduling simulation for both launch and termination flows.

## NodeClaim drift and disruption

NodeClaims are immutable and cannot be modified, but are used to compare to changes made to the NodePool. When NodePools are modified, differing in spec from a created NodeClaim, Karpenter will mark the NodeClaim as drifted.

For details on Karpenter disruption, see [Disruption]({{< ref "./disruption" >}}).

## NodeClaim example
The following is an example of a NodeClaim. Keep in mind that you cannot modify a NodeClaim.
To see the contents of a NodeClaim, get the name of your NodeClaim, then run `kubectl describe` to see its contents:

```
kubectl get nodeclaim
NAME            TYPE               ZONE         NODE                                           READY   AGE
default-m6pzn   c7i-flex.2xlarge   us-west-1a   ip-xxx-xxx-xx-xxx.us-west-1.compute.internal   True    7m50s

kubectl describe nodeclaim default-m6pzn
```
Starting at the bottom of this example, here are some highlights of what the NodeClaim contains:

* The Node Name (ip-xxx-xxx-xx-xxx.us-west-1.compute.internal) and Provider IDi (aws:///us-west-1a/i-xxxxxxxxxxxxxxxxx) identify the instance that is fulfilling this NodeClaim.
* Image ID (ami-0ccbbed159cce4e37) represents the operating system image running on the node.
* Status shows the resources that are available on the node (CPU, memory, and so on) as well as the conditions associated with the node. The conditions show the status of the node, including whether the node is launched, registered, and healthy. This is particularly useful if Pods are not deploying to the node and you want to determine the cause.
* The Spec sections show the values of different NodeClaim objects. For example, you can see the type of operating system running (linux), the instance type and category, the NodePool that was used for the NodeClaim, the node's architecture (amd64), and resources such as CPU and number of Pods currently running on the node.
* Metadata show information about how the NodeClaim was created.
* Other information shows labels and annotations on the node, the API version, and Kind (NodeClaim).

```
Name:         default-m6pzn
Namespace:
Labels:       karpenter.k8s.aws/instance-category=c
              karpenter.k8s.aws/instance-cpu=8
              karpenter.k8s.aws/instance-cpu-manufacturer=intel
              karpenter.k8s.aws/instance-ebs-bandwidth=10000
              karpenter.k8s.aws/instance-encryption-in-transit-supported=true
              karpenter.k8s.aws/instance-family=c7i-flex
              karpenter.k8s.aws/instance-generation=7
              karpenter.k8s.aws/instance-hypervisor=nitro
              karpenter.k8s.aws/instance-memory=16384
              karpenter.k8s.aws/instance-network-bandwidth=1562
              karpenter.k8s.aws/instance-size=2xlarge
              karpenter.sh/capacity-type=spot
              karpenter.sh/nodepool=default
              kubernetes.io/arch=amd64
              kubernetes.io/os=linux
              node.kubernetes.io/instance-type=c7i-flex.2xlarge
              topology.k8s.aws/zone-id=usw1-az3
              topology.kubernetes.io/region=us-west-1
              topology.kubernetes.io/zone=us-west-1a
Annotations:  karpenter.k8s.aws/ec2nodeclass-hash: 164893570827491067
              karpenter.k8s.aws/ec2nodeclass-hash-version: v2
              karpenter.k8s.aws/tagged: true
              karpenter.sh/nodepool-hash: 15093649574832938182
              karpenter.sh/nodepool-hash-version: v2
API Version:  karpenter.sh/v1beta1
Kind:         NodeClaim
Metadata:
  Creation Timestamp:  2024-08-06T15:33:27Z
  Finalizers:
    karpenter.sh/termination
  Generate Name:  default-
  Generation:     1
  Owner References:
    API Version:           karpenter.sh/v1beta1
    Block Owner Deletion:  true
    Kind:                  NodePool
    Name:                  default
    UID:                   fd25d0e6-1ab3-4ac8-a377-a46cbd0d9b03
  Resource Version:        486466
  UID:                     6d4ead04-979f-42a3-ab7c-d697b10155f8
Spec:
  Node Class Ref:
    API Version:  karpenter.k8s.aws/v1beta1
    Kind:         EC2NodeClass
    Name:         default
  Requirements:
    Key:       kubernetes.io/os
    Operator:  In
    Values:
      linux
    Key:       karpenter.sh/capacity-type
    Operator:  In
    Values:
      spot
    Key:       node.kubernetes.io/instance-type
    Operator:  In
    Values:
      c3.2xlarge
      c3.4xlarge
      c3.8xlarge
      ...
    Key:       karpenter.k8s.aws/instance-category
    Operator:  In
    Values:
      c
      m
      r
    Key:       karpenter.k8s.aws/instance-generation
    Operator:  Gt
    Values:
      2
    Key:       karpenter.sh/nodepool
    Operator:  In
    Values:
      default
    Key:       kubernetes.io/arch
    Operator:  In
    Values:
      amd64
  Resources:
    Requests:
      Cpu:   5150m
      Pods:  8
Status:
  Allocatable:
    Cpu:                  7910m
    Ephemeral - Storage:  17Gi
    Memory:               14162Mi
    Pods:                 58
  Capacity:
    Cpu:                  8
    Ephemeral - Storage:  20Gi
    Memory:               15155Mi
    Pods:                 58
  Conditions:
    Last Transition Time:  2024-08-06T15:34:01Z
    Message:               KnownEphemeralTaint "node.kubernetes.io/not-ready:NoSchedule" still exists
    Reason:                KnownEphemeralTaintsExist
    Status:                False
    Type:                  Initialized
    Last Transition Time:  2024-08-06T15:33:30Z
    Message:
    Reason:                Launched
    Status:                True
    Type:                  Launched
    Last Transition Time:  2024-08-06T15:33:51Z
    Message:               Initialized=False
    Reason:                UnhealthyDependents
    Status:                False
    Type:                  Ready
    Last Transition Time:  2024-08-06T15:33:51Z
    Message:
    Reason:                Registered
    Status:                True
    Type:                  Registered
  Image ID:                ami-0ccbbed159cce4e37
  Node Name:               ip-xxx-xxx-xx-xxx.us-west-1.compute.internal
  Provider ID:             aws:///us-west-1a/i-xxxxxxxxxxxxxxxxx
Events:          
```
