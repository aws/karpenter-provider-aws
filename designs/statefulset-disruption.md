# Graceful Shutdown for Stateful Workloads (Solving `FailedAttachVolume` Delays)

Workloads should start on new nodes in seconds not minutes. So why [can it take minutes](#c-related-issues) for disrupted stateful workloads to run on a new node?

Ideally, once a StatefulSet pod terminates, its persistent volume gets unmounted & detached from its current node, attached & mounted on its new node + pod, and the new pod should start `Running` all within 10-20 seconds[^1].

However, with the default configurations of Karpenter `v0.37.0` and the [EBS](https://aws.amazon.com/ebs/) [CSI](https://kubernetes.io/blog/2019/01/15/container-storage-interface-ga/) Driver `v1.31.0`, disrupted statefulset pods may experience minutes of `FailedAttachVolume` delays before `Running` on their new node.

This document reviews the desired flow for disrupting of stateful workloads, describes the **two separate** race conditions that cause `FailedAttachVolume` delays for stateful workloads attempting to run on new nodes, and recommends solutions to these problems.

- [Graceful Shutdown for Stateful Workloads (Solving `FailedAttachVolume` Delays)](#graceful-shutdown-for-stateful-workloads-solving-failedattachvolume-delays)
  - [Disruption of Stateful Workloads Background](#disruption-of-stateful-workloads-background)
    - [Karpenter Graceful Shutdown for Stateless Workloads](#karpenter-graceful-shutdown-for-stateless-workloads)
    - [Stateful Workloads Overview](#stateful-workloads-overview)
    - [Ideal Disruption Flow for Stateful Workloads](#ideal-disruption-flow-for-stateful-workloads)
    - [Problems](#problems)
    - [Solutions](#solutions)
  - [Problem A. Preventing 6+ minute delays](#problem-a-preventing-6-minute-delays)
    - [When does this happen?](#when-does-this-happen)
    - [Solutions:](#solutions-1)
      - [A1: Fix race at Kubelet level](#a1-fix-race-at-kubelet-level)
      - [A2: Taint node as `out-of-service` after termination](#a2-taint-node-as-out-of-service-after-termination)
      - [Alternatives Considered](#alternatives-considered)
  - [Problem B. Preventing Delayed Detachments](#problem-b-preventing-delayed-detachments)
    - [When does this happen?](#when-does-this-happen-1)
    - [Solution B1: Wait for detach in Karpenter cloudProvider.Delete](#solution-b1-wait-for-detach-in-karpenter-cloudproviderdelete)
    - [Alternatives Considered](#alternatives-considered-1)
  - [Appendix](#appendix)
    - [Z. Document TODOs](#z-document-todos)
    - [T. Latency Numbers](#t-latency-numbers)
      - [Pod termination -\> volumes cleaned up](#pod-termination---volumes-cleaned-up)
      - [Instance stopped/terminated times](#instance-stoppedterminated-times)
    - [A. Further Reading](#a-further-reading)
    - [B. Terminology](#b-terminology)
    - [C. Related Issues](#c-related-issues)
    - [D. Additional Context](#d-additional-context)
      - [D1. EC2 Termination + EC2 DetachVolume relationship additional context](#d1-ec2-termination--ec2-detachvolume-relationship-additional-context)
      - [D2. Non-Graceful Shutdown + out-of-service taint additional context](#d2-non-graceful-shutdown--out-of-service-taint-additional-context)
        - [When was out-of-service taint added?](#when-was-out-of-service-taint-added)
        - [Where is out-of-service taint used in k/k?](#where-is-out-of-service-taint-used-in-kk)
        - [Is the out-of-service taint safe to use?](#is-the-out-of-service-taint-safe-to-use)
        - [What changes in Kubernetes due to this Node Ungraceful Shutdown feature?](#what-changes-in-kubernetes-due-to-this-node-ungraceful-shutdown-feature)
      - [D3: WaitForVolumeDetachments Implementation Details](#d3-waitforvolumedetachments-implementation-details)
    - [E. Reproduction Manifests](#e-reproduction-manifests)
    - [F: Sequence Diagrams](#f-sequence-diagrams)
    - [G: Footnotes](#g-footnotes)

## Disruption of Stateful Workloads Background

### Karpenter Graceful Shutdown for Stateless Workloads

From [Karpenter: Disruption](https://karpenter.sh/docs/concepts/disruption/):

"Karpenter sets a Kubernetes finalizer on each node and node claim it provisions. **The finalizer blocks deletion of the node object while the Termination Controller taints and drains the node**, before removing the underlying NodeClaim. Disruption is triggered by the Disruption Controller, by the user through manual disruption, or through an external system that sends a delete request to the node object."

For the scope of this document, we will focus on Karpenter's [Node Termination Controller](https://github.com/kubernetes-sigs/karpenter/blob/38b4c32043724f6dafce910669777877295d89ec/pkg/controllers/node/termination/controller.go#L77) and its interactions with the terminating node.

See the following diagram for the relevant sequence of events in the case of only stateless workloads:

![Stateless_Workload_Karpenter_Termination](https://github.com/AndrewSirenko/karpenter-provider-aws/assets/68304519/e629f41a-5b1e-49a2-b7a4-29a2db172394)

*Note: These diagrams abstract away parts of Karpenter/Kubernetes/EC2 in order to remain approachable. For example, we exclude the K8s API Server and EC2 API. `Terminating Node` represents both the node object and underlying EC2 Instance. For an example of what other distinctions are missing see the footnotes.*[^2]

### Stateful Workloads Overview

Persistent Storage in Kubernetes involves many moving parts, most of which may not be relevant for the decision at hand. 

For the purpose of this document, you should know that:
- The **Container Storage Interface** (CSI) is a standard way for Container Orchestrators to provision persistent volumes from storage providers and expose block and file storage systems to containers.
- The **AttachDetach Controller** watches for stateful workloads that are waiting on their storage, and ensures that their volumes are attached to the right node. Also watches for attached volumes that are no longer in use, and ensures they are detached. 
- The **CSI Controller** attaches/detaches volumes to nodes with workloads that require Persistent Volumes. (I.e. Calls EC2 AttachVolume). [^5]
- The **CSI Node Service** mounts[^3] volumes to make them available for use by workloads. Unmounts volumes after workload terminates to ensure they are no longer in use. Runs on each node. The Kubelet's Volume Manager watches for stateful workloads and calls csi node service.
- `Mounted != Attached`. Attached EBS Volume is visible as a block device by privileged user on node at `/dev/<device-path>`. Mounted volume is visible by workload containers at specified `mountPath`s. See [this StackOverflow post](https://stackoverflow.com/questions/24429949/device-vs-partition-vs-file-system-vs-volume-how-do-these-concepts-relate-to-ea)
- The [CSI Specification](https://github.com/container-storage-interface/spec/blob/master/spec.md) states that the container orchestrator must interact with CSI  Plugin through the following flow of Remote Procedure Calls when a workload requires persistent storage: ControllerPublishVolume (i.e. attach volume to node) -> NodeStageVolume (Mount volume to global node mount-point) -> NodePublishVolume (Mount volume to pod's mountpoint) (and when volume no longer in use: NodeUnbpublishVolume -> NodeUnstageVolume -> ControllerUnpublishVolume) 

For the purpose of this document, assume volumes have already been created and will never be deleted.

<details open>
<summary>If you want to dive one level deeper, open the dropdown to see the following diagram of what happens between pod eviction and volume detachment</summary>
<img src="https://github.com/AndrewSirenko/karpenter-provider-aws/assets/68304519/fc577f2c-2de0-4aee-8daa-110a1eb0b990" alt="Stateful Pod Termination" class="inline"/>
</details>

### Ideal Disruption Flow for Stateful Workloads

In order for a stateful pods to smoothly migrate from the terminating node to another node, the following steps must occur in order:

0. Node marked for deletion
1. Stateful pods must enter `terminated` state
2. Volumes must be confirmed as unmounted (By CSI Node Service)
3. Volumes must be confirmed as detached from instance (By AttachDetach & CSI Controllers)
4. Karpenter terminates EC2 Instance
5. Karpenter deletes finalizer on Node

See the following diagram for a more detailed sequence of events.  

![ideal](https://github.com/AndrewSirenko/karpenter-provider-aws/assets/68304519/ae5267e1-fe2d-455f-9199-60cee772f0ae)

### Problems

Today, with customers with default Karpenter `v0.37.0` and EBS CSI Driver `v1.31.0` configurations may experience two different kinds of delays once their disrupted stateful workloads are scheduled on a new node. 

**Problem A. If step 2 doesn't happen, there will be a 6+ minute delay.**

If volumes are not *confirmed* as unmounted *by CSI Node Service*, Kubernetes cannot confirm volumes are not in use and will wait a hard-coded [6 minute MaxWaitForUnmountDuration](https://github.com/kubernetes/kubernetes/blob/8b727956214818a3a5846bca060426a13a578348/pkg/controller/volume/attachdetach/attach_detach_controller.go#L94) and confirm node is unhealthy before treating the volume as unmounted. See [EBS CSI 6-minute delay FAQ](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/faq.md#6-minute-delays-in-attaching-volumes) for more context. [^6] 

Customers will see the following event on pod object (Note the 6+ minute delay): 

```
Warning  FailedAttachVolume      6m51s              attachdetach-controller  Multi-Attach error for volume "pvc-123" Volume is already exclusively attached to one node and can't be attached to another
```

**Problem B. If step 3 doesn't happen before step 4, there will be a 1+ minute delay**

If Karpenter calls EC2 TerminateInstance **before** EC2 DetachVolume calls from EBS CSI Driver Controller pod finish, then the volumes won't be detached **until the old instance terminates**.This delay depends on how long it takes the underlying instance to enter the `terminated` state, which depends on the instance type. Typically 1 minute for `m5a.large`,  up to 10 minutes for certain metals instances. See [appendix D1](#d1-ec2-termination--ec2-detachvolume-relationship-) and [instance termination latency measurements](#t-latency-numbers) for more context. 
 
Customers will see the following events (Note the 1-min delay between `Multi-Attach error` AND `AttachVolume.Attach failed`):

```
Warning  FailedAttachVolume      102s               attachdetach-controller  Multi-Attach error...
Warning  FailedAttachVolume      40s                attachdetach-controller  AttachVolume.Attach failed for volume "pvc-" : rpc error: code = Internal desc = Could not attach volume "vol-" to node "i-"... VolumeInUse: vol- is already attached to an instance                    
Normal   SuccessfulAttachVolume  33s                attachdetach-controller  AttachVolume.Attach succeeded for volume "pvc"                                                                   
```

Customers can determine which delay they are suffering from based off of whether `AttachVolume.Attach` is in the `FailedAttachVolume` event. 

### Solutions

**A1: To solve A long-term, Kubernetes should ensure volumes are unmounted before critical pods like CSI Driver Node pod are terminated.**

**A2: To solve A today, Karpenter should confirm that volumes are not in use and confirm AttachDetach Controller knows this before deleting the node's finalizer.**

**B1: To solve B today, Karpenter should wait for volumes to detach by watching volumeattachment objects before terminating the node.**

See [WIP Kubernetes 1.31/1.32 A1 solution in PR #125070](https://github.com/kubernetes/kubernetes/pull/125070)

See [a proof-of-concept implementation of A2 & B1 in PR #1294](https://github.com/kubernetes-sigs/karpenter/pull/1294)

Finally, we should add the following EBS x Karpenter end-to-end test in karpenter-provider-aws to catch regressions between releases of Karpenter or EBS CSI Driver:
1. Deploy statefulset with 1 replica
2. Consolidate Node
3. Confirm replica migrated
4. Confirm replica running within x minutes (where x is high enough to prevent flakes)

## Problem A. Preventing 6+ minute delays 

If `ReadWriteOnce` volumes are not unmounted *by CSI Node Service*, Kubernetes cannot confirm volumes are not in use and safe to attach to new node. Kubernetes will wait 6 minutes[^4] and ensure Node is `unhealthy` before treating the volume as unmounted and moving forward with a volume detach. See [EBS CSI 6-minute delay FAQ](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/faq.md#6-minute-delays-in-attaching-volumes) 

Cluster operator will see a `FailedAttachVolume` event on pod object with `Multi-Attach error`

### When does this happen? 

This delay happens when the EBS CSI Node pod is killed before it can unmount all volumes of `terminated` pods. Note that a pod's volumes can only be unmounted **after** the pod enters the `terminated` state. 

The EBS CSI Node pod can be killed in two places, depending on whether it tolerates the `karpenter.sh/disruption=disrupting` taint: If the EBS CSI Node does not tolerate the taint, it will be killed during Karpenter [Terminator's draining process](https://github.com/kubernetes-sigs/karpenter/blob/e58d48e24c051e217b1c0c119bd19b7b29519532/pkg/controllers/node/termination/controller.go#L90) after all pods that are not system-critical Daemonsets enter the `terminated` state.  If the EBS CSI Node Pod does tolerate this Karpenter taint, Karpenter's Terminator will call EC2 TerminateInstances when all intolerant pods are `terminated`. In this case, if [Graceful Shutdown](https://kubernetes.io/docs/concepts/cluster-administration/node-shutdown/#graceful-node-shutdown) is configured on the node, the Kubelet's [Node Shutdown Manager will attempt to kill EBS CSI Node Pod](https://github.com/kubernetes/kubernetes/blob/b3db54ea72a4f7441260982b4d2941f856401c9a/pkg/kubelet/nodeshutdown/nodeshutdown_manager_linux.go#L322-L417) after all non-critical pods have entered `terminated` state.  

As of EBS CSI Driver `v1.31.0`, the EBS CSI Node Pod tolerates all taints be default, therefore we will focus on this second type of race for the following diagram:

![6min](https://github.com/AndrewSirenko/karpenter-provider-aws/assets/68304519/0a0fbe16-fe81-428d-85fb-0716c733c9c0)

Karpenter's terminator cannot drain pods that tolerate its `Disrupting` taint. Therefore, once it drains the drainable pods, it calls EC2 TerminateInstance on a node. 

However, the shutdown manager does not wait for all volumes to be unmounted, just pod termination. This leads to a race condition where the CSI Driver Node Pod is killed before all unmounts are completed. See @msau42's diagram:

![kubelet_shutdown_race](https://github.com/AndrewSirenko/karpenter-provider-aws/assets/68304519/9ccb6c94-ef02-40af-84bf-9471827f3303)

Today, the EBS CSI Driver attempts to work around these races by utilizing a [PreStop hook](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/faq.md#what-steps-can-be-taken-to-mitigate-this-issue) that tries to keep the Node Service alive for an additional [terminationGracePeriodSeconds](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/73b43995a70b8fea51655577e4b475613ab400f1/charts/aws-ebs-csi-driver/values.yaml#L373) until all volumes are unmounted. We will explore the shortcomings of this solution later in [problem A alternative solutions](#alternatives-considered).  

*Note: In addition to the Kubelet race, this delay can happen if stateful pods out-live the CSI Node Pod. E.g. Operator has a statefulset that tolerates all taints and has a longer terminationGracePeriod than EBS CSI Driver.*

### Solutions:

We should:
- A1: Fix Kubelet race condition upstream for future Kubernetes versions
- A2: Have Karpenter taint terminated nodes as `out-of-service` before removing its finalizer. 

#### A1: Fix race at Kubelet level

The Kubelet Shutdown Manager should not kill CSI Driver Pods before volumes are unmounted. This change must be made at `kubernetes/kubernetes` level. 

Because this solution does not rely on changes in Karpenter, please see [Active PR #125070](https://github.com/kubernetes/kubernetes/pull/125070) for more information on this solution. 

**Pros:** 

- Other cluster autoscalers will not face this race condition. 
- Reduces pod migration times
- Reduces risk of data corruption because relevant CSI Drivers will perform unmount operations required  

**Cons:**

- Unavailable until merged in a version of Kubernetes (Likely Kubernetes `v1.31` or `v1.32`) (Possibly able to be backported)
- If gracefulShutdown period is up BEFORE volumes are unmounted by CSI Node pod, then kubernetes cannot confirm volume was unmounted, and volume will still have delay. (E.g. unmount takes more than 1 minute, which is longer than gracefulShutdown period of 45 seconds.)

#### A2: Taint node as `out-of-service` after termination

While this race should be fixed at the Kubelet level long-term, we still need a solution for earlier versions of Kubernetes.

One solution is to mark terminated nodes via `out-of-service` taint. 

In `v1.26`, Kubernetes enabled the [Non-graceful node shutdown handling feature](https://kubernetes.io/docs/concepts/cluster-administration/node-shutdown/#non-graceful-node-shutdown) by default. This introduces the `node.kubernetes.io/out-of-service` taint, which can be used to mark a node as permanently shut down. See more context in [appendix D2]() 

Once Karpenter confirms that an instance is terminated, adding this taint to the node object will allow the Attach/Detach Controller to treat the volume as not in use, preventing the 6+ minute delay. 

By modifying Karpenter to apply this taint and wait until volumes are marked not in use on node object (~5 seconds), the following sequence will occur:

![taint_solution](https://github.com/AndrewSirenko/karpenter-provider-aws/assets/68304519/d1bc5704-7592-4aa1-a72e-83d33f824b8b)

See [this commit](https://github.com/kubernetes-sigs/karpenter/pull/1294/commits/88134e00ad02863d5a7268bba0b639e21f3f5398) for a proof-of-concept implementation. 

**Pros:**

- Solves 6+ minute delays by default
- No additional latency before Karpenter's terminator can start instance termination. 
- Minor latency in deleting Node's finalizer IF karpenter does not treat `shutting down` instance as terminated. (In my tests, a 5 second wait was sufficient for AttachDetach Controller to recognize the out-of-service taint and allow for volume detach)
- If Kubernetes makes this 6 minute ForceDetach timer infinite by default (As currently planned in Kubernetes v1.32), and the EBS CSI Node Pod is unable to unmount all volumes before improved Node Shutdown Manager times out, the out-of-service taint will be the only way to ensure workload starts on other node 

**Cons:**

- Only available in Kubernetes ≥ `v1.26`.
- Requires Terminator to ensure instance is `terminated` before applying taint.
- Problem B's delay still occurs because volumes will not be detached until consolidated instance terminates. 

#### Alternatives Considered

**Customer configuration**

Customers can mitigate 6+ minute delays by configuring their nodes and pod taint tolerations. See [EBS CSI Driver FAQ: Mitigating 6+ minute delays](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/faq.md#what-steps-can-be-taken-to-mitigate-this-issue) for an updating list of configuration requirements.

A quick overview:
- Configure Kubelet for Graceful Node Shutdown
- Enable Karpenter Spot Instance interruption handling
- Use EBS CSI Driver ≥ `v1.28` in order use the [PreStop Lifecycle Hook](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/faq.md#what-is-the-prestop-lifecycle-hook)
- Use Karpenter ≥ `v1.0.0`

**Pros:**

- No code change required in Karpenter. 

**Cons:**

- Requiring configuration is a poor customer experience because many customers will not be aware of special requirements for EBS-backed workloads with Karpenter. Troubleshooting this configuration is difficult due to the two separate attachment delay issues. (Hence why issues are still being raised on Karpenter and EBS CSI Driver Projects)
- Only fixes problem A, the 6+ minute delay. 
- Stateful workloads that tolerate Karpenter's `disrupting` taint, or any system-critical stateful daemonsets with a higher terminationGracePeriod than the EBS CSI Driver will still see migration delays. 

## Problem B. Preventing Delayed Detachments

Even if we solve the 6+ minute volume-in-use delay, AWS customers may suffer from a second type of delay due to behavior specific to EC2.  

### When does this happen?

If Karpenter calls EC2 TerminateInstance **before** EC2 DetachVolume calls finish, then the volumes won't be detached **until the old instance terminates**. This delay depends on the instance type. 1 minute for `m5a.large`, 2 minutes for large GPU instances like `g4ad.16xlarge`, and 10+ minutes for certain Metal instances like `m7i.metal-48xl`. For more context see [Appendix D1](#d1-ec2-termination--ec2-detachvolume-relationship-additional-context) and [instance termination latencies](#t-latency-numbers)

Operators will see `FailedAttachVolume` events on pod object with `Multi-Attach error` and then `AttachVolume.Attach failed` errors. 

### Solution B1: Wait for detach in Karpenter cloudProvider.Delete

Wait for volumes to detach before terminating the instance.

We can do this by waiting for all volumes of drain-able nodes to be marked as not be in use nor attached before terminating the node in c.cloudProvider.Delete (until a maximum of 20 seconds). See [Appendix D3 for the implementation details of this wait](#d) 

We can detect a volume is detached by ensuring that `volumeattachment` objects associated to relevant PVs are deleted. This also implies that the volume was safely unmounted by the CSI Node pod. 

This means that our sequence of events will match the ideal diagram from section [Ideal Graceful Shutdown for Stateful Workloads][#ideal-graceful-shutdown-for-stateful-workloads]

We can use similar logic to [today's proof-of-concept implementation](https://github.com/kubernetes-sigs/karpenter/pull/1294), but move it to karpenter-provider-aws and check for `node.Status.VolumesInUse` instead of listing volumeattachment objects. A 20 second max wait was sufficient to prevent delays with m5a instance type, but further testing is needed to ensure it is enough for Windows/GPU instance types.

**Pros:**

- Leaves decision to each cloud/storage provider 
- Can opt-in to this behavior for specific CSI Drivers (Perhaps via Helm parameter)
- Only delays termination of nodes with stateful workloads.
- Implicitly solves problem A for EBS-backed stateful workloads, if volumeattachment object is deleted before instance is terminated.

**Cons:**

- Delays node termination and finalizer deletion by a worst-case of 20 seconds. (We can skip waiting on the volumes of non-drainable pods to make the average case lower)
- Other CSI Drivers must opt-in

### Alternatives Considered

**Implement B1 in `kubernetes-sigs/karpenter`**

Instead of solving this inside the [`c.cloudProvider.Delete`](https://github.com/aws/karpenter-provider-aws/blob/9cef47b6df77ec9e0a39dc6f4a4ecd1aab504ae3/pkg/cloudprovider/cloudprovider.go#L179), solve this inside [termination controller's reconciler loop](https://github.com/kubernetes-sigs/karpenter/blob/94d5b41d4711b1c21fc0264f1b29db6a64b95caf/pkg/controllers/node/termination/controller.go#L77) (as is done in [today's proof-of-concept implementation](https://github.com/kubernetes-sigs/karpenter/pull/1294))

**Pros:**

- Karpenter-provider-aws does not need to know about Kubernetes volume lifecycle
- Implicitly solves problem A for EBS-backed stateful workloads, if volumeattachment object is deleted before instance is terminated.

**Cons:**

- [Open Question] EBS may be the only storage provider where detaching volumes before terminating the instance matters. If this is the case, the delay before instance termination is not worth it for customers of other storage/cloud providers.
- Should not hardcode cloud-provider specific CSI Drivers in upstream project. Therefore this delay must be agreed upon by multiple drivers and cloud providers.

**Karpenter Polls EC2 for volume detachments before calling EC2 TerminateInstance**

Karpenter-provider-aws can poll EC2 DescribeVolumes before making an EC2 TerminateInstance call. 

**Pros:**

- Solves problem B
- Implicitly solves problem A for EBS-backed stateful workloads, if volumeattachment object is deleted before instance is terminated.

**Cons:**

- If volumes are not unmounted due to problem A, volumes cannot be detached anyway. Termination is the fastest way forward.
- Karpenter has to worry about EBS volume lifecycle. 

## Appendix 

### Z. Document TODOs

- Expand Appendix terminology and further reading sections
- Prove what potential data corruption issues we are talking about.
- List open questions + decision log after design review.

### T. Latency Numbers

#### Pod termination -> volumes cleaned up
These timings come from a few manual tests. Treat them as ball-park numbers to guide our conversation, not facts. 

Pod terminated -> volumes unmounted: Typically <1 second. Can be longer if volume very large (Terabytes).  

unmount -> EC2 DetatchVolume called: Typically <1 second

unmount -> Volume actually Detached from linux instance: Typically 5-10 seconds. Can take longer because no SLA from EBS

pod termination -> Karpenter can safely call EC2 TerminateInstances: ~10 seconds. (If EC2 DetatchVolume is made enough ahead of EC2TerminateInstances, we are fine for many instance types. It's only if they're within a few seconds of each other that we run into the problem B race.)

#### Instance stopped/terminated times

These are manual tests measured by polling `EC2 DescribeInstances` performed in June 2024 in `us-west-2`. Treat them as ball-park numbers to guide our conversation, not facts.   

Times are in Minutes:Seconds

m5.large
stopped ~40
terminated ~55 

c5.12xlarge -- Windows 2022 AMI
Stopped ~30
Terminated ~1:15

c5.metal
stopped ~10:55
terminated ~10:54

g4ad.xlarge
stopped ~53 
terminated ~57

g4ad.4xlarge
stopped ~53 
terminated ~1:37

g4ad.16xlarge
stopped ~2:00
terminated ~2:10

Windows instances with elastic GPUs are reported to have slow termination times, but this has yet to be tested. 


### A. Further Reading

- [EBS CSI: What causes 6-minute delays in attaching volumes?](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/faq.md#6-minute-delays-in-attaching-volumes)
- Kubernetes:
  - [Node Shutdowns](https://kubernetes.io/docs/concepts/cluster-administration/node-shutdown/)
  - [API-initiated Eviction](https://kubernetes.io/docs/concepts/scheduling-eviction/api-eviction/#how-api-initiated-eviction-works)


### B. Terminology

- Daemonset: Pod scheduled on every Node. 
- EBS: Elastic Block Store. 
- Kubelet: Primary 'node agent' that runs on each node. Volume Manager service in Kubelet makes gRPC to EBS CSI Node pod.
- StatefulSet: Manages the deployment and scaling of a set of Pods, and provides gauruntees about the ordering and uniqueness of these Pods. These uniqueness gauruntees are valuable when your workload needs persistent storage. 

### C. Related Issues

* [Volume still hang on Karpenter Node Consolidation/Termination](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/1955)
* [Volume hang on Karpenter Node Consolidation/Termination](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/1665)
* [VolumeAttachment takes too long to remove](https://github.com/kubernetes-csi/external-attacher/issues/463)
* [PVC attaching takes much time](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/issues/1302)

### D. Additional Context

#### D1. EC2 Termination + EC2 DetachVolume relationship additional context

If EC2 API reacts to an EC2 TerminateInstances call before EC2 DetachVolumes, the following may occur: 

1. Karpenter invokes TerminateInstances
2. EC2 notifies the guest OS that it needs to shut down.
3. The guest OS can take a long time to complete shutting down.
4. In the meantime, the CSI driver was informed of volume no longer in use and attempts to detach the volumes.
5. The detach workflow is blocked because the OS is shutting down.
6. Once the guest OS finally finishes shutting down, AWS EC2 cleans up instance.
7. Then the detach workflows are unblocked and are no-ops because instance is already terminated.
8. EBS CSI Controller is able to attach volume to new instance

#### D2. Non-Graceful Shutdown + out-of-service taint additional context

##### When was out-of-service taint added?

Added as part of `Non-graceful node shutdown handling` feature, default-on in Kubernetes v1.26, stable in v1.28.

See
- [Kubernetes documentation](https://kubernetes.io/docs/concepts/cluster-administration/node-shutdown/#non-graceful-node-shutdown)
- [Enhancements Issue](https://github.com/kubernetes/enhancements/issues/2268)
- [Final KEP-2268](https://github.com/kubernetes/enhancements/tree/master/keps/sig-storage/2268-non-graceful-shutdown)

From documentation:

```
When a node is shutdown but not detected by kubelet's Node Shutdown Manager, the pods that are part of a StatefulSet will be stuck in terminating status on the shutdown node and cannot move to a new running node. This is because kubelet on the shutdown node is not available to delete the pods so the StatefulSet cannot create a new pod with the same name. If there are volumes used by the pods, the VolumeAttachments will not be deleted from the original shutdown node so the volumes used by these pods cannot be attached to a new running node. As a result, the application running on the StatefulSet cannot function properly. If the original shutdown node comes up, the pods will be deleted by kubelet and new pods will be created on a different running node. If the original shutdown node does not come up, these pods will be stuck in terminating status on the shutdown node forever.

To mitigate the above situation, a user can manually add the taint node.kubernetes.io/out-of-service with either NoExecute or NoSchedule effect to a Node marking it out-of-service. If the NodeOutOfServiceVolumeDetachfeature gate is enabled on kube-controller-manager, and a Node is marked out-of-service with this taint, the pods on the node will be forcefully deleted if there are no matching tolerations on it and volume detach operations for the pods terminating on the node will happen immediately. This allows the Pods on the out-of-service node to recover quickly on a different node.

During a non-graceful shutdown, Pods are terminated in the two phases:

1. Force delete the Pods that do not have matching out-of-service tolerations.
2. Immediately perform detach volume operation for such pods.

Note:
- Before adding the taint node.kubernetes.io/out-of-service, it should be verified that the node is already in shutdown or power off state (not in the middle of restarting).
- The user is required to manually remove the out-of-service taint after the pods are moved to a new node and the user has checked that the shutdown node has been recovered since the user was the one who originally added the taint.
```

##### Where is out-of-service taint used in k/k?

Searching `Kubernetes/Kubernetes`, I found the out-of-service taint referenced in the following places:

- AttachDetach controller will trigger volume detach even if state of Kubernetes thinks volume mounted by node. Seen [here](https://github.com/kubernetes/kubernetes/blob/3f9b79fc119d064d00939f91567b48d9ada7dc43/pkg/controller/volume/attachdetach/reconciler/reconciler.go#L234-L236). (As of Kubernetes 1.30 detach trigger will happen after a 6 min forceDetach timer. There are plans to turn off this timer by default in Kubernetes 1.32, which will mean volumes will never be detached without a successful CSI NodeUnstage / NodeUnpublish)

- Pod Garbage Collection Controller garbage collect pods that are terminating on not-ready node with out-of-service taint (will add pods on nodes to `terminatingPods` list) [here](https://github.com/kubernetes/kubernetes/blob/3f9b79fc119d064d00939f91567b48d9ada7dc43/pkg/controller/podgc/gc_controller.go#L171)

- Various metrics like [PodGCReasonTerminated](https://github.com/kubernetes/kubernetes/blob/3f9b79fc119d064d00939f91567b48d9ada7dc43/pkg/controller/podgc/gc_controller.go#L219)

- GCE has upstream e2e tests on this feature [here](https://github.com/kubernetes/kubernetes/blob/3f9b79fc119d064d00939f91567b48d9ada7dc43/test/e2e/storage/non_graceful_node_shutdown.go#L43-L61)

##### Is the out-of-service taint safe to use? 

The out-of-service taint is confirmed safe to use with EBS-backed stateful workloads. This is because even if AttachDetach Controller issues a forceDetach, the EBS CSI Controller's `EC2 DetachVolume` call cannot detach a mounted volume in the case instance is still running, and by the time instance is terminated volume is already in detached without an `EC2 DetachVolume` call. 

Open Question: However, this may not be true for all CSI Drivers. There may be certain CSI Drivers that expect NodeUnstage and NodeUnpublish to be called before ControllerUnpublish, because they perform additional logic outside of typical io flushes and unmount syscalls.

We can perhaps consider a version of this solution that lives in karpenter-provider-aws AND only applies taint if all volumeattachment objects left on node are associated with EBS CSI Driver. 

##### What changes in Kubernetes due to this Node Ungraceful Shutdown feature? 

From KEP 2268, proposed pre-KEP and post-KEP logic:

```
Existing logic:

1. When a node is not reachable from the control plane, the health check in Node lifecycle controller, part of kube-controller-manager, sets Node v1.NodeReady Condition to False or Unknown (unreachable) if lease is not renewed for a specific grace period. Node Status becomes NotReady.

2. After 300 seconds (default), the Taint Manager tries to delete Pods on the Node after detecting that the Node is NotReady. The Pods will be stuck in terminating status.

Proposed logic change:

1. [Proposed change] This proposal requires a user to apply a out-of-service taint on a node when the user has confirmed that this node is shutdown or in a non-recoverable state due to the hardware failure or broken OS. Note that user should only add this taint if the node is not coming back at least for some time. If the node is in the middle of restarting, this taint should not be used.

2. [Proposed change] In the Pod GC Controller, part of the kube-controller-manager, add a new function called gcTerminating. This function would need to go through all the Pods in terminating state, verify that the node the pod scheduled on is NotReady. If so, do the following:

3. Upon seeing the out-of-service taint, the Pod GC Controller will forcefully delete the pods on the node if there are no matching tolation on the pods. This new out-of-service taint has NoExecute effect, meaning the pod will be evicted and a new pod will not schedule on the shutdown node unless it has a matching toleration. For example, node.kubernetes.io/out-of-service=out-of-service=nodeshutdown:NoExecute or node.kubernetes.io/out-of-service=out-of-service=hardwarefailure:NoExecute. We suggest to use NoExecute effect in taint to make sure pods will be evicted (deleted) and fail over to other nodes.

4. We'll follow taint and toleration policy. If a pod is set to tolerate all taints and effects, that means user does NOT want to evict pods when node is not ready. So GC controller will filter out those pods and only forcefully delete pods that do not have a matching toleration. If your pod tolerates the out-of-service taint, then it will not be terminated by the taint logic, therefore none of this applies.

5. [Proposed change] Once pods are selected and forcefully deleted, the attachdetach reconciler should check the out-of-service taint on the node. If the taint is present, the attachdetach reconciler will not wait for 6 minutes to do force detach. Instead it will force detach right away and allow volumeAttachment to be deleted.

6. This would trigger the deletion of the volumeAttachment objects. For CSI drivers, this would allow ControllerUnpublishVolume to happen without NodeUnpublishVolume and/or NodeUnstageVolume being called first. Note that there is no additional code changes required for this step. This happens automatically after the Proposed change in the previous step to force detach right away.

7. When the external-attacher detects the volumeAttachment object is being deleted, it calls CSI driver's ControllerUnpublishVolume.
```

#### D3: WaitForVolumeDetachments Implementation Details

A proof-of-concept implementation can be seen in [PR 1294](https://github.com/kubernetes-sigs/karpenter/pull/1294)

WaitForVolumeDetachments can cause reconciler requeues until either all detachable EBS-managed volumeattachment objects are deleted, or a max timeout has been reached. As jmdeal@ suggested, we can either:

- Add an annotation to the node to indicate when the drain attempt to begin. Continue to reconcile until an upper time limit has been hit.
- Do the same thing but with an in-memory map from node to timestamp.
  
An in-memory map would mean no new annotations on the node, but would not persist across Karpenter controller restarts. 

### E. Reproduction Manifests

Deploy Karpenter `v0.37.0` and EBS CSI Driver `1.31.0` to your cluster

<details closed>
<summary>Apply the following manifest to have a stateful pod migrate from an expiring node every 3 minutes. </summary>
<br>

```
---
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: general-purpose
  annotations:
    kubernetes.io/description: "General purpose NodePool for generic workloads"
spec:
  template:
    spec:
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values: ["amd64"]
        - key: kubernetes.io/os
          operator: In
          values: ["linux"]
        - key: karpenter.sh/capacity-type
          operator: In
          values: ["on-demand"]
        - key: karpenter.k8s.aws/instance-category
          operator: In
          values: ["c", "m", "r"]
        - key: karpenter.k8s.aws/instance-generation
          operator: Gt
          values: ["2"]
      nodeClassRef:
        apiVersion: karpenter.k8s.aws/v1beta1
        kind: EC2NodeClass
        name: default
  disruption:
    consolidationPolicy: WhenUnderutilized
    expireAfter: 3m
---
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
metadata:
  name: default
  annotations:
    kubernetes.io/description: "General purpose EC2NodeClass for running Amazon Linux 2 nodes"
spec:
  amiFamily: AL2 # Amazon Linux 2
  role: "KarpenterNodeRole-karpenter-demo" # replace with your cluster name
  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: "karpenter-demo" # replace with your cluster name
  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: "karpenter-demo" # replace with your cluster name
  userData: |
    MIME-Version: 1.0
    Content-Type: multipart/mixed; boundary="BOUNDARY"

    --BOUNDARY
    Content-Type: text/x-shellscript; charset="us-ascii"

    #!/bin/bash
    echo -e "InhibitDelayMaxSec=45\n" >> /etc/systemd/logind.conf
    systemctl restart systemd-logind
    echo "$(jq ".shutdownGracePeriod=\"45s\"" /etc/kubernetes/kubelet/kubelet-config.json)" > /etc/kubernetes/kubelet/kubelet-config.json
    echo "$(jq ".shutdownGracePeriodCriticalPods=\"15s\"" /etc/kubernetes/kubelet/kubelet-config.json)" > /etc/kubernetes/kubelet/kubelet-config.json
    --BOUNDARY--
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ebs-sc
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: ebs.csi.aws.com
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
apiVersion: v1
kind: Service
metadata:
  name: nginx
  labels:
    app: nginx
spec:
  ports:
  - port: 80
    name: web
  clusterIP: None
  selector:
    app: nginx
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: web
spec:
  serviceName: "nginx"
  replicas: 1
  podManagementPolicy: Parallel
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: registry.k8s.io/nginx-slim:0.8
        ports:
        - containerPort: 80
          name: web
        volumeMounts:
        - name: www
          mountPath: /usr/share/nginx/html
        resources:
          limits:
            memory: "128Mi"
            cpu: "500m"
      nodeSelector:
        karpenter.sh/nodepool: general-purpose
      topologySpreadConstraints:
      - maxSkew: 1
        topologyKey: "topology.kubernetes.io/zone"
        whenUnsatisfiable: ScheduleAnyway
        labelSelector:
          matchLabels:
            app: nginx
  volumeClaimTemplates:
  - metadata:
      name: www
      labels:
        roar: www
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 1Gi
  persistentVolumeClaimRetentionPolicy:
    whenDeleted: Delete
```

</details>

### F: Sequence Diagrams



<details closed>
<summary>Raw code for sequence diagrams</summary>
<br>

Simplified Stateless Termination
```
sequenceDiagram
    participant Karp as Karpenter Terminator
    participant Old as Consolidating Node (VM)
    
    Karp->>+Old: Drain
    Old->>-Karp: Pods Terminated.
    Karp->>+Old: EC2 TerminateInstance
    Old->>Old: Shutting Down
    Old->>-Karp: Terminated
    Karp-->>Old: Remove Finalizer.
```

Complicated Stateless Disruption
```
sequenceDiagram
    participant Old as Old Node (VM)
    participant Kub as Kubernetes CP
    participant Karp as Karpenter Terminator
    participant EC2 as EC2 API
    
    Kub->>+Karp: Old Node marked for deletion
    Karp->>Karp: Old terminator start
    Karp->>Kub: Taint Old `Disrupting:NoSchedule`
    Karp->>+Kub: Drain Old Node
    Kub->>+Old: Terminate Pods
    Old->>-Kub: Pods Terminated
    Kub->>-Karp: Drain Done
    Karp->>+EC2: Terminate Old Node
    EC2->>+Old: Shut Down
    EC2->>-Karp: Old ShuttingDown
    Karp->>-Kub: Remove Old Node Finalizer
    Kub-->Old: Lost Communication
    Old->-EC2: Terminated
    destroy Old
```

Consolidation Event: Stateful Today (Default EBS CSI Driver configuration)
```
participant EN as CSI Node Pod
    participant Old as Old Node (VM)
    participant Kub as Kubernetes CP
    participant Karp as Karpenter Terminator
    participant EC2 as EC2 API
    
    Kub->>+Karp: Old Node marked for deletion
    Karp->>Karp: Old terminator start
    Karp->>+Kub: Drain Old Node
    Kub->>+Old: Terminate Stateful Pod
    Old->>Kub: Stateful Pod Terminated
    Kub->>-Karp: Drain Done
    Old->>+EN: Unmount Volume
    EN->>-Old: Unmounted Volume
    Old->>-Kub: Safe to detach Volume
    Karp->>+EC2: Terminate Old Node
    EC2->>+Old: Shut Down
    EC2->>-Karp: Old ShuttingDown
    Karp->>Kub: Remove Old Node Finalizer
    Kub-->Old: Lost Communication
    Old->-EC2: Terminated
```

Stateful Workload Termination–Ideal Case:
```
sequenceDiagram
    participant Karp as Karpenter Terminator
    participant Old as Consolidating Node
    participant CN as CSI Node Service
    participant AT as AttachDetach Controller
    participant NN as New Node
    
    note left of Karp: 0. Deletion Marked
    Karp->>+Old: Drain
    Old->>Old: Intolerant Stateful Pod Terminated
    note left of Karp: 1. Pods Terminated
    Old-->NN: Pod Rescheduled
    Old->>-Karp: Drain Complete
    NN->>+NN: Stateful Pod ContainerCreating
    NN->>+AT: Where is my Volume?
    AT->>AT: Volume Still In Use
    note left of Karp: 2. Volumes unmount
    Old->>+CN: Unmount Volume
    CN->>-Old: Unmounted Volume
    Old->>+AT: Volume Not In Use
    note left of Karp: 3. Volumes detached
    AT->>-Old: EC2 Detach Volume
    Old->>AT: EC2 Detached Volume
    Note right of Karp: Waited for volume detach
    note left of Karp: 4. Terminate Instance
    Karp->>+Old: EC2 TerminateInstance
    Note right of NN: ~15s delay (EC2 detach + attach)
    AT->>NN: EC2 Attach Volume
    NN->>AT: EC2 Attached Volume
    AT->>-NN: Your Volume is Ready
    NN->>-NN: Pod Running
    Old->>Old: Shutting Down
    destroy CN
    Old->>CN: Kubelet Kills
    Old->>-Karp: EC2 Terminated
    note left of Karp: 5. Remove Finalizer
    Karp-->>Old: Remove Finalizer
```

Delay until instance terminated (1 min delay)
```
sequenceDiagram
    participant Karp as Karpenter Terminator
    participant Old as Consolidating Node
    participant AT as AttachDetach Controller
    participant NN as New Node
    
    Note left of Karp: 0. Deletion Marked
    Karp->>+Old: Drain
    Old->>Old: Intolerant Stateful Pod Terminated
    Note left of Karp: 1. Pods Terminated
    Old-->NN: Pod Rescheduled
    NN->+NN: Stateful Pod ContainerCreating
    NN->>+AT: Where is my Volume?
    AT->>AT: Volume Still In Use
    Old->>Karp: Drain Complete
    Old->>Old: CSI Node Pod Unmounted Volume
    Note left of Karp: 2. Unmount Volume
    Note right of Karp: Karpenter waits for unmount
    Note left of Karp: 4. Terminate Instance
    par
        Karp->>+Old: EC2 TerminateInstance
    and 
        Old->>AT: Volume Not In Use
        AT->>Old: EC2 Detach Volume
    end
    Old->>Old: Shutting Down
    Note right of Old: Volume detach delayed until terminated.    
    Old->>AT: Termination Detached Volume
    Note right of NN: ~1m delay (EC2 Termination)*
    Old->>-Karp: EC2 Terminated
    AT->>NN: EC2 Attach Volume
    NN->>AT: EC2 Attached Volume
    AT->>-NN: Your Volume is Ready
    NN->>-NN: Pod Running
    Note left of Karp: 5. Remove Finalizer
    Karp-->>Old: Remove Finalizer
```

force detach 6 min delay 
```
sequenceDiagram
    participant Karp as Karpenter Terminator
    participant Old as Consolidating Node
    participant CN as CSI Node Service
    participant AT as AttachDetach Controller
    participant NN as New Node
    
    Note left of Karp: 0. Deletion Marked
    Karp->>+Old: Drain
    Old->>Old: All intolerant pods terminated
    Old->>-Karp: Drain Complete
    Note left of Karp: 4. Terminate Instance
    Karp->>+Old: EC2 TerminateInstance
    Old->>Old: Shutting Down
    Old->>Karp: EC2 Shutting Down
    par
    destroy CN 
    Old->>CN: Killed
    and
    Old->>Old: Kill Tolerant Pods
    end
    Old--xCN: Unknown if Volume Unmounted
    Old-->NN: Pod Rescheduled
    
    Note left of Karp: 5. Remove Finalizer
    destroy Old
    Karp-->>Old: Remove Finalizer
    NN->>+NN: Stateful Pod ContainerCreating
    NN->>+AT: Where is my Volume?
    AT--xOld: Unknown if Volume in use
    AT-->AT: Wait 6 Min ForceDetach Timer
    Old->Old: Shutdown Manager unmounts
    Old->Old: Instance Terminated
    AT->>AT: Volume Force Detached
    Note right of NN: 6+ min delay (K8s ForceDetach Timer)
    AT->>NN: EC2 Attach Volume
    NN->>AT: EC2 Attached Volume
    AT->>-NN: Your Volume is Ready
    NN->>-NN: Pod Running
```

Taint post shutdown 2 min delay
```
sequenceDiagram
    participant Karp as Karpenter Terminator
    participant Old as Consolidating Node
    participant CN as CSI Node Service
    participant AT as AttachDetach Controller
    participant NN as New Node
    
    Note left of Karp: 0. Mark Deletion
    Karp->>+Old: Drain
    par
        destroy CN 
        Old->>CN: Terminated
        Old--xCN: Unknown if Volume Unmounted
    and
        Old->>Old: All Intolerant Pods Terminated
    end
    Note left of Karp: 1. Pods Terminated
    Old->>-Karp: Drain Complete
    Old-->NN: Pod Rescheduled
    NN->>+NN: Stateful Pod ContainerCreating
    NN->>+AT: Where is my Volume?
    AT-->Old: Unknown if Volume not in use
    Note left of Karp: 4. Terminate Instance
    Karp->>+Old: EC2 TerminateInstance
    Old->>Old: Shutting Down / ShutdownManager Unmounts
    Note right of NN:  ~1m delay (EC2 Termination)*
    Old->>-Karp: EC2 Terminated
    Note left of Karp: Solution A2
    Karp->>Old: Taint out-of-service
    Old->>AT: Taint confirms Volume not in use
    Karp->>Karp: Wait until taint seen (~5s)
    destroy Old
    Note left of Karp: 5. Delete Finalizer
    Karp-->>Old: Remove Finalizer
    AT->>NN: EC2 Attach Volume
    NN->>AT: EC2 Attached Volume
    AT->>-NN: Your Volume is Ready
    NN->>-NN: Pod Running
    
```

Alt Taint post shutdown delay
```
sequenceDiagram
    participant Karp as Karpenter Terminator
    participant Old as Consolidating Node
    participant CN as CSI Node Service
    participant AT as AttachDetach Controller
    participant NN as New Node
    
    Note left of Karp: 0. Mark Deletion
    Karp->>+Old: Drain
    Old->>Old: All Intolerant Pods Terminated

    Old->>-Karp: Drain Complete
    
    
    Note left of Karp: 4. Terminate Instance
    Karp->>+Old: EC2 TerminateInstance
    Old->>Old: Shutting Down 
    par
        destroy CN 
        Old->>CN: Terminated
        Old--xCN: Unknown if Volume Unmounted
    and
       Old->>Old: Tolerant Pods Killed
    end
    Old-->NN: Pod Rescheduled
    NN->>+NN: Stateful Pod ContainerCreating
    NN->>+AT: Where is my Volume?
    AT-->Old: Unknown if Volume not in use
    Old->>Old: Volume Unmounted/Detached by ShutDown
    Old->>-Karp: EC2 Terminated
    Note left of Karp: Solution A2
    Karp->>Old: Taint out-of-service
    Old->>AT: Taint confirms Volume not in use
    Karp->>Karp: Wait until taint seen (~5s)
    destroy Old
    
    Note right of NN:  ~1m delay (EC2 Termination)*
    Note left of Karp: 5. Delete Finalizer
    par 
    
    Karp-->>Old: Remove Finalizer
    and
    
    AT->>NN: EC2 Attach Volume
    NN->>AT: EC2 Attached Volume
    AT->>-NN: Your Volume is Ready
    NN->>-NN: Pod Running
    end
```

</details>

### G: Footnotes

[^1]: From my testing via the EBS CSI Driver, EBS volumes typically take 10 seconds to detach and 5 seconds to attach. But there is no Attach/Detach SLA.

[^2]: [Complicated stateless diagram](https://github.com/AndrewSirenko/karpenter-provider-aws/assets/68304519/f8fdc38d-be65-4de1-9837-827f65707fa5).

[^3]: EBS CSI Node service is called by the node's Kubelet's Volume Manager twice after a volume attachment. Once to format the block device and mount filesystem on a node global directory, and a second time to mount on the pod's directory.

[^4]: For certain storage providers, this delay in pod restart can prevent potential data corruption due to unclean mounts. (Is this true for EBS? I'm skeptical that these data corruption issues exist for non-multi-attach EBS volumes. EC2 does not allow mounted volumes to be detached, and most Linux distributions unmount all filesystems/volumes during shut down. Finally, when the volume is attached to the new node, I believe CSI Node pods run e2fsck before formatting volumes and *never* forcefully reformat volumes)

[^5]: The CSI controller pod actually consists of multiple containers. Kubernetes-maintained 'sidecar' controllers and the actual `csi-plugin` that cloud-storage-providers maintain. Relevant to this document are the `external-attacher` and `ebs-plugin` containers. The `external-attacher` watches kubernetes `volumeattachment` and makes remote procedural calls `ebs-plugin` to interact with EC2 backend to make sure volumes get attached.

[^6]: Note, this is a hard-coded "forced-detach" delay in the KCM AttachDetach Controller, which can be disabled. If disabled this delay is infinite, and Kubernetes will never call ControllerUnpublishVolume, requiring customer to manually delete volumeattachment object. As of June 2024, SIG-Storage wants to disable this timer by default in Kubernetes v1.32. 
