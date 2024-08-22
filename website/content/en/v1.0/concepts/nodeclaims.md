---
title: "NodeClaims"
linkTitle: "NodeClaims"
weight: 30
description: >
  Understand NodeClaims
---

Karpenter uses NodeClaims to manage the lifecycle of Kubernetes Nodes with the underlying cloud provider.
Karpenter will create and delete NodeClaims in response to the demands of Pods in the cluster.
It does this by evaluating the requirements of pending pods, finding a compatible [NodePool]({{< ref "./nodepools" >}}) and [NodeClass]({{< ref "./nodeclasses" >}}) pair, and creating a NodeClaim which meets both sets of requirements.
Although NodeClaims are immutable resources managed by Karpenter, you can monitor NodeClaims to keep track of the status of your Nodes.

In addition to tracking the lifecycle of Nodes, NodeClaims serve as requests for capacity.
Karpenter creates NodeClaims in response to provisioning and disruption needs (pre-spin). Whenever Karpenter
creates a NodeClaim, it asks the cloud provider to create the instance (launch), register and link the created node
with the NodeClaim (registration), and wait for the node and its resources to be ready (initialization).

This page describes how NodeClaims integrate throughout Karpenter and the cloud provider implementation.

If you want to learn more about the nodes being managed by Karpenter, you can either look directly at the NodeClaim or at the nodes they are associated with:

* Checking NodeClaims:
If something goes wrong in the process of creating a node, you can look at the NodeClaim
to see where the node creation process might have failed. `kubectl get nodeclaims` will show you the NodeClaims
for the cluster, and its linked node. Using `kubectl describe nodeclaim <nodeclaim>` will show the status of a particular NodeClaim.
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

   **Example log:**
   ```json
   {
       "level": "INFO",
       "time": "2024-06-22T02:24:16.114Z",
       "message": "found provisionable pod(s)",
       "commit": "490ef94",
       "Pods": "default/inflate-66fb68585c-xvs86, default/inflate-66fb68585c-hpcdz, default/inflate-66fb68585c-8xztf,01234567adb205c7e default/inflate-66fb68585c-t29d8, default/inflate-66fb68585c-nxflz",
       "duration": "100.761702ms"
   }
   ```

2. Computes the shape and size of a NodeClaim (or NodeClaims) to create in the cluster to fit the set of pods from step 1.

   **Example log:**
   ```json
   {
       "level": "INFO",
       "time": "2024-06-22T02:24:16.114Z",
       "message": "computed new nodeclaim(s) to fit pod(s)",
       "controller": "provisioner",
       "nodeclaims": 1,
       "pods": 5
   }
   ```

3. Creates the NodeClaim object in the cluster.

   **Example log:**
   ```json
   {
       "level": "INFO",
       "time": "2024-06-22T02:24:16.128Z",
       "message": "created nodeclaim",
       "controller": "provisioner",
       "NodePool": {
           "name":"default"
       },
       "NodeClaim": {
           "name":"default-sfpsl"
       },
       "requests": {
           "cpu":"5150m",
           "pods":"8"
       },
       "instance-types": "c3.2xlarge, c4.2xlarge, c4.4xlarge, c5.2xlarge, c5.4xlarge and 55 other(s)"
   }
   ```

4. Finds the new NodeClaim and translates it into an API call to create a cloud provider instance, logging
   the response of the API call.

   If the API response is an unrecoverable error, such as an Insufficient Capacity Error, Karpenter will delete the NodeClaim, mark that instance type as temporarily unavailable, and create another NodeClaim if necessary.

   **Example log:**
   ```json
   {
       "level": "INFO",
       "time": "2024-06-22T02:24:19.028Z",
       "message": "launched nodeclaim",
       "controller": "nodeclaim.lifecycle",
       "NodeClaim": {
           "name": "default-sfpsl"
       },
       "provider-id": "aws:///us-west-2b/i-01234567adb205c7e",
       "instance-type": "c3.2xlarge",
       "zone": "us-west-2b",
       "capacity-type": "spot",
       "allocatable": {
         "cpu": "7910m",
         "ephemeral-storage": "17Gi",
         "memory": "13215Mi",
         "pods": "58"
       }
   }
   ```

5. Karpenter watches for the instance to register itself with the cluster as a node, and updates the node's
   labels, annotations, taints, owner refs, and finalizer to match what was defined in the NodePool and NodeClaim. Once this step is
   completed, Karpenter will remove the `karpenter.sh/unregistered` taint from the Node.

   If this fails to succeed within 15 minutes, Karpenter will remove the NodeClaim from the cluster and delete
   the underlying instance, creating another NodeClaim if necessary.

   **Example log:**
   ```json
   {
     "level": "INFO",
     "time": "2024-06-22T02:26:19.028Z",
     "message": "registered nodeclaim",
     "controller": "nodeclaim.lifecycle",
     "NodeClaim": {
       "name": "default-sfpsl"
     },
     "provider-id": "aws:///us-west-2b/i-01234567adb205c7e",
     "Node": {
       "name": "ip-xxx-xxx-xx-xxx.us-west-2.compute.internal"
     }
   }
   ```

6. Karpenter continues to watch the node, waiting until the node becomes ready, has all its startup taints removed,
   and has all requested resources registered on the node.

   **Example log:**
   ```json
   {
     "level": "INFO",
     "time": "2024-06-22T02:24:52.642Z",
     "message": "initialized nodeclaim",
     "controller": "nodeclaim.lifecycle",
     "NodeClaim": {
       "name": "default-sfpsl"
     },
     "provider-id": "aws:///us-west-2b/i-01234567adb205c7e",
     "Node": {
       "name": "ip-xxx-xxx-xx-xxx.us-west-2.compute.internal"
     },
     "allocatable": {
       "cpu": "7910m",
       "ephemeral-storage": "18242267924",
       "hugepages-2Mi": "0",
       "memory": "14320468Ki",
       "pods": "58"
     }
   }
   ```

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

* The Node Name (ip-xxx-xxx-xx-xxx.us-west-1.compute.internal) and Provider ID (aws:///us-west-1a/i-xxxxxxxxxxxxxxxxx) identify the instance that is fulfilling this NodeClaim.
* Image ID (ami-0ccbbed159cce4e37) represents the operating system image running on the node.
* Status shows the resources that are available on the node (CPU, memory, and so on) as well as the conditions associated with the node. The conditions show the status of the node, including whether the node is launched, registered, and initialized. This is particularly useful if Pods are not deploying to the node and you want to determine the cause.
* Spec contains the metadata required for Karpenter to launch and manage an instance. This includes any scheduling requirements, resource requirements, the NodeClass reference, taints, and immutable disruption fields (expireAfter and terminationGracePeriod).
* Additional information includes annotations and labels which should be synced to the Node, creation metadata, the termination finalizer, and the owner reference.

```
Name:         default-x9wxq
Namespace:
Labels:       karpenter.k8s.aws/instance-category=c
              karpenter.k8s.aws/instance-cpu=8
              karpenter.k8s.aws/instance-cpu-manufacturer=amd
              karpenter.k8s.aws/instance-ebs-bandwidth=3170
              karpenter.k8s.aws/instance-encryption-in-transit-supported=true
              karpenter.k8s.aws/instance-family=c5a
              karpenter.k8s.aws/instance-generation=5
              karpenter.k8s.aws/instance-hypervisor=nitro
              karpenter.k8s.aws/instance-memory=16384
              karpenter.k8s.aws/instance-network-bandwidth=2500
              karpenter.k8s.aws/instance-size=2xlarge
              karpenter.sh/capacity-type=spot
              karpenter.sh/nodepool=default
              kubernetes.io/arch=amd64
              kubernetes.io/os=linux
              node.kubernetes.io/instance-type=c5a.2xlarge
              topology.k8s.aws/zone-id=usw2-az3
              topology.kubernetes.io/region=us-west-2
              topology.kubernetes.io/zone=us-west-2c
Annotations:  compatibility.karpenter.k8s.aws/cluster-name-tagged: true
              compatibility.karpenter.k8s.aws/kubelet-drift-hash: 15379597991425564585
              karpenter.k8s.aws/ec2nodeclass-hash: 5763643673275251833
              karpenter.k8s.aws/ec2nodeclass-hash-version: v3
              karpenter.k8s.aws/tagged: true
              karpenter.sh/nodepool-hash: 377058807571762610
              karpenter.sh/nodepool-hash-version: v3
API Version:  karpenter.sh/v1
Kind:         NodeClaim
Metadata:
  Creation Timestamp:  2024-08-07T05:37:30Z
  Finalizers:
    karpenter.sh/termination
  Generate Name:  default-
  Generation:     1
  Owner References:
    API Version:           karpenter.sh/v1
    Block Owner Deletion:  true
    Kind:                  NodePool
    Name:                  default
    UID:                   6b9c6781-ac05-4a4c-ad6a-7551a07b2ce7
  Resource Version:        19600526
  UID:                     98a2ba32-232d-45c4-b7c0-b183cfb13d93
Spec:
  Expire After:  720h0m0s
  Node Class Ref:
    Group:
    Kind:   EC2NodeClass
    Name:   default
  Requirements:
    Key:       kubernetes.io/arch
    Operator:  In
    Values:
      amd64
    Key:       kubernetes.io/os
    Operator:  In
    Values:
      linux
    Key:       karpenter.sh/capacity-type
    Operator:  In
    Values:
      spot
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
    Key:       node.kubernetes.io/instance-type
    Operator:  In
    Values:
      c3.xlarge
      c4.xlarge
      c5.2xlarge
      c5.xlarge
      c5a.xlarge
      c5ad.2xlarge
      c5ad.xlarge
      c5d.2xlarge
  Resources:
    Requests:
      Cpu:   3150m
      Pods:  6
  Startup Taints:
    Effect:  NoSchedule
    Key:     app.dev/example-startup
  Taints:
    Effect:                  NoSchedule
    Key:                     app.dev/example
  Termination Grace Period:  1h0m0s
Status:
  Allocatable:
    Cpu:                        7910m
    Ephemeral - Storage:        17Gi
    Memory:                     14162Mi
    Pods:                       58
    vpc.amazonaws.com/pod-eni:  38
  Capacity:
    Cpu:                        8
    Ephemeral - Storage:        20Gi
    Memory:                     15155Mi
    Pods:                       58
    vpc.amazonaws.com/pod-eni:  38
  Conditions:
    Last Transition Time:  2024-08-07T05:38:08Z
    Message:
    Reason:                Consolidatable
    Status:                True
    Type:                  Consolidatable
    Last Transition Time:  2024-08-07T05:38:07Z
    Message:
    Reason:                Initialized
    Status:                True
    Type:                  Initialized
    Last Transition Time:  2024-08-07T05:37:33Z
    Message:
    Reason:                Launched
    Status:                True
    Type:                  Launched
    Last Transition Time:  2024-08-07T05:38:07Z
    Message:
    Reason:                Ready
    Status:                True
    Type:                  Ready
    Last Transition Time:  2024-08-07T05:37:55Z
    Message:
    Reason:                Registered
    Status:                True
    Type:                  Registered
  Image ID:                ami-08946d4d49fc3f27b
  Node Name:               ip-xxx-xxx-xxx-xxx.us-west-2.compute.internal
  Provider ID:             aws:///us-west-2c/i-01234567890123
Events:
  Type    Reason             Age   From       Message
  ----    ------             ----  ----       -------
  Normal  Launched           70s   karpenter  Status condition transitioned, Type: Launched, Status: Unknown -> True, Reason: Launched
  Normal  DisruptionBlocked  70s   karpenter  Cannot disrupt NodeClaim: state node doesn't contain both a node and a nodeclaim
  Normal  Registered         48s   karpenter  Status condition transitioned, Type: Registered, Status: Unknown -> True, Reason: Registered
  Normal  Initialized        36s   karpenter  Status condition transitioned, Type: Initialized, Status: Unknown -> True, Reason: Initialized
  Normal  Ready              36s   karpenter  Status condition transitioned, Type: Ready, Status: Unknown -> True, Reason: Ready
```
