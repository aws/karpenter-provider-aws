# Graceful Shutdown for Stateful Workloads (Solving `FailedAttachVolume` Delays)

Workloads should start on new nodes in seconds not minutes. So why does it take minutes for disrupted stateful workloads to run on a new node?

Ideally, once a StatefulSet pod terminates, its persistent volume gets unmounted & detached from its current node, attached & mounted on its new node + pod, and the new pod should start `Running` all within 10-20 seconds[^1].

However, with the default configurations of Karpenter `v0.37.0` and the [EBS](https://aws.amazon.com/ebs/) [CSI](https://kubernetes.io/blog/2019/01/15/container-storage-interface-ga/) Driver `v1.31.0`, disrupted statefulset pods may experience minutes of `FailedAttachVolume` delays before `Running` on their new node.

This document will review the desired flow for disrupting of stateful workloads, describe the **two separate** race conditions that cause `FailedAttachVolume` delays, and recommend solutions to these problems.


- [Disruption of Stateful Workloads Background](#disruption-of-stateful-workloads-background)
  - [Karpenter Graceful Shutdown for Stateless Workloads](#karpenter-graceful-shutdown-for-stateless-workloads)
  - [Stateful Workloads Overview](#stateful-workloads-overview)
  - [Ideal Graceful Shutdown for Stateful Workloads](#ideal-graceful-shutdown-for-stateful-workloads)
  - [Problems](#problems)
  - [Solutions](#solutions)
- [Problem A. Preventing 6+ minute delays](#problem-a-preventing-6-minute-delays)
  - [When does this happen?](#when-does-this-happen)
  - [Solutions:](#solutions-1)
    - [A1: Fix race at Kubelet level](#a1-fix-race-at-kubelet-level)
    - [A2: Taint node as `out-of-service` after termination](#a2-taint-node-as-out-of-service-after-termination)
    - [Alternatives Considered](#alternatives-considered)
- [Problem B. Preventing Delayed Detachments](#problem-b-preventing-delayed-detachments)
  - [Solution B1: Wait for unmounts in Karpenter cloudProvider.Delete](#solution-b1-wait-for-unmounts-in-karpenter-cloudproviderdelete)
  - [Alternatives Considered](#alternatives-considered-1)
- [Appendix](#appendix)
  - [Z. Document TODOs](#z-document-todos)
  - [A. Further Reading](#a-further-reading)
  - [B. Terminology](#b-terminology)
  - [C. Issue Timeline](#c-issue-timeline)
  - [D. Footnotes](#d-footnotes)
  - [E. Reproduction Manifests](#e-reproduction-manifests)
  - [F: Sequence Diagrams](#f-sequence-diagrams)

## Disruption of Stateful Workloads Background

### Karpenter Graceful Shutdown for Stateless Workloads

From [Karpenter: Disruption](https://karpenter.sh/docs/concepts/disruption/):

"Karpenter sets a Kubernetes finalizer on each node and node claim it provisions. **The finalizer blocks deletion of the node object while the Termination Controller taints and drains the node**, before removing the underlying NodeClaim. Disruption is triggered by the Disruption Controller, by the user through manual disruption, or through an external system that sends a delete request to the node object."

For the scope of this document, we will focus on Karpenter's [Node Termination Controller](https://github.com/kubernetes-sigs/karpenter/blob/38b4c32043724f6dafce910669777877295d89ec/pkg/controllers/node/termination/controller.go#L77) and its interactions with the terminating node.

See the following diagram for the relevant sequence of events in the case of only stateless pods:

![Stateless_Workload_Karpenter_Termination](https://github.com/AndrewSirenko/karpenter-provider-aws/assets/68304519/e629f41a-5b1e-49a2-b7a4-29a2db172394)

*Note: These diagrams abstract away parts of Karpenter/Kubernetes/EC2 in order to remain approachable. For example, we exclude the K8s API Server and EC2 API. `Terminating Node` represents both the node object and underlying EC2 Instance. For an example of what other distinctions are missing see the footnotes.*[^2]

### Stateful Workloads Overview

Persistent Storage in Kubernetes involves many moving parts, most of which may not be relevant for the decision at hand. 

For the purpose of this document, you should know that:
- The **Container Storage Interface** (CSI) is a standard way for Container Orchestrators to provision persistent volumes from storage providers and expose block and file storage systems to containers.
- The **AttachDetach Controller** watches for stateful workloads that are waiting on their storage, and ensures that their volumes are attached to the right node. Also watches for attached volumes that are no longer in use, and ensures they are detached. 
- The **CSI Controller** attaches/detaches volumes to nodes. (I.e. Calls EC2 AttachVolume)
- The **CSI Node Service** mounts[^3] volumes to make them available for use by pods. Unmounts volumes after pods terminate to ensure they are no longer in use. Runs on each node. 

<details open>
<summary>If you want to dive one level deeper, open the dropdown to see the following diagram of what happens between pod eviction and volume detachment</summary>
<img src="https://github.com/AndrewSirenko/karpenter-provider-aws/assets/68304519/fc577f2c-2de0-4aee-8daa-110a1eb0b990" alt="Stateful Pod Termination" class="inline"/>
</details>

### Ideal Graceful Shutdown for Stateful Workloads

In order for a stateful pods to smoothly migrate from the terminating node to another node, the following steps must occur in order:

0. Node marked for deletion
1. Stateful pods must terminate
2. Volumes must be unmounted (By CSI Node Service)
3. Volumes must be detached from instance (By AttachDetach & CSI Controllers)
4. Karpenter terminates EC2 Instance
5. Karpenter deletes finalizer on Node

See the following diagram for a more detailed sequence of events.  

![ideal](https://github.com/AndrewSirenko/karpenter-provider-aws/assets/68304519/ae5267e1-fe2d-455f-9199-60cee772f0ae)

### Problems

Today, with customers with default Karpenter `v0.37.0` and EBS CSI Driver `v1.31.0` configurations may experience two different kinds of delays once their disrupted stateful workloads are scheduled on a new node. 

**Problem A. If step 2 doesn't happen, there will be a 6+ minute delay.**

If volumes are not unmounted *by CSI Node Service*, Kubernetes cannot confirm volumes are not in use and will wait 6 minutes before treating the volume as unmounted. See [EBS CSI 6-minute delay FAQ](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/faq.md#6-minute-delays-in-attaching-volumes) 

Relevant PVC Event (Note the 6+ minute delay): 

```
Warning  FailedAttachVolume      6m51s              attachdetach-controller  Multi-Attach error for volume "pvc-123" Volume is already exclusively attached to one node and can't be attached to another
```

**Problem B. If step 3 doesn't happen before step 4, there will be a 1+ minute delay**

If karpenter calls EC2 TerminateInstance **before** EC2 DetachVolume calls finish, then the volumes won't be detached **until the old instance terminates**. This delay depends on the instance type. 1 minute for `m5a.large`, up to 15 minutes for certain GPU/Windows instances.
 
Relevant PVC Events (Note the 1-min delay between `Multi-Attach error` AND `AttachVolume.Attach failed`):

```
Warning  FailedAttachVolume      102s               attachdetach-controller  Multi-Attach error...
Warning  FailedAttachVolume      40s                attachdetach-controller  AttachVolume.Attach failed for volume "pvc-" : rpc error: code = Internal desc = Could not attach volume "vol-" to node "i-"... VolumeInUse: vol- is already attached to an instance                    
Normal   SuccessfulAttachVolume  33s                attachdetach-controller  AttachVolume.Attach succeeded for volume "pvc"                                                                   
```

Customers can determine which delay they are suffering from based off of whether or not `AttachVolume.Attach` is in the `FailedAttachVolume` event. 

### Solutions

**A1: To solve A long-term, Kubernetes should ensure volumes are unmounted before CSI Driver Node pods are terminated.**

**A2: To solve A today, Karpenter should confirm and communicate that volumes are not in use before deleting the node's finalizer.**

**B1: To solve B today, Karpenter should wait for volumes to detach before terminating the node.**

See [WIP Kubernetes 1.31 solution in PR #125070](https://github.com/kubernetes/kubernetes/pull/125070)

See [a proof-of-concept implementation of A2 & B1 in PR #1294](https://github.com/kubernetes-sigs/karpenter/pull/1294)

Finally we should add the following EBS x Karpenter end-to-end test in karpenter-provider-aws to catch regressions between releases of Karpenter or EBS CSI Driver:
1. Deploy statefulset with 1 replica
2. Consolidate Node
3. Confirm replica migrated
4. Confirm replica running within x minutes (where x is high enough to prevent flakes)

## Problem A. Preventing 6+ minute delays 

If volumes are not unmounted *by CSI Node Service*, Kubernetes cannot confirm volumes are not in use and will wait 6 minutes[^4] before treating the volume as unmounted and moving forward with a volume detach. See [EBS CSI 6-minute delay FAQ](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/faq.md#6-minute-delays-in-attaching-volumes) 

Cluster operator will see a `FailedAttachVolume` event with `Multi-Attach error`

### When does this happen? 

This delay happens when the EBS CSI Node Service is killed before it can unmount all volumes. One potential sequence of events is the following:

![6min](https://github.com/AndrewSirenko/karpenter-provider-aws/assets/68304519/0a0fbe16-fe81-428d-85fb-0716c733c9c0)

Karpenter's terminator cannot drain pods that tolerate its `Disrupting` taint. Therefore, once it drains the drainable pods, it calls EC2 TerminateInstance on a node. If [Graceful Shutdown](https://kubernetes.io/docs/concepts/cluster-administration/node-shutdown/#graceful-node-shutdown) is configured on the node, the Kubelet's Node Shutdown Manager will start killing normal, then critical pods. Because all normal pods have been killed, the Kubelet will kill the EBS CSI Driver. 

However, the the shutdown manager does not wait for all volumes to be unmounted, just pod termination. This leads to a race condition where the CSI Driver Node Pod is killed before all unmounts are completed. See @msau42's diagram:

![kubelet_shutdown_race](https://github.com/AndrewSirenko/karpenter-provider-aws/assets/68304519/9ccb6c94-ef02-40af-84bf-9471827f3303)

Today, the EBS CSI Driver attempts to workaround this race by utilizing a [PreStop hook](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/faq.md#what-steps-can-be-taken-to-mitigate-this-issue) that tries to keep the Node Service alive until all volumes are unmounted. We will explore the shortcomings of this solution later.  

*Note: In addition to the Kubelet race, this delay can happen if stateful pods out-live the CSI Node Pod. E.g. Operator has a statefulset that tolerates all taints and a longer terminationGracePeriod than EBS CSI Driver.*

### Solutions:

We should:
- A1: Fix Kubelet race condition upstream for future Kubernetes versions
- A2: Have Karpenter taint terminated nodes as `out-of-service` before removing its finalizer. 

#### A1: Fix race at Kubelet level

The Kubelet Shutdown Manager should not kill CSI Driver Pods before volumes are unmounted. 

See: [PR #125070](https://github.com/kubernetes/kubernetes/pull/125070) for more information. 

**Pros:** 
- Other cluster autoscalers will not face this race condition.  
- Ideal long-term solution
- Reduces pod migration times
- Reduces risk of data corruption
**Cons:**
- Unavailable until Kubernetes `v1.31`

#### A2: Taint node as `out-of-service` after termination

While this race should be fixed at the Kubelet level long-term, we still need a solution for earlier versions of Kubernetes.

One solution is to mark terminated nodes as `out-of-service`. 

In `v1.26`, Kubernetes enabled the [Non-graceful node shutdown handling feature](https://kubernetes.io/docs/concepts/cluster-administration/node-shutdown/#non-graceful-node-shutdown) by default. This introduces the `node.kubernetes.io/out-of-service` taint, which can be used to mark a node as terminated. 

Once Karpenter confirms that an instance is terminated, adding this taint to the node object will allow the Attach/Detach Controller to treat the volume as not in use, preventing the 6+ minute delay. 

With this taint and wait, the following sequence will occur:

![taint_solution](https://github.com/AndrewSirenko/karpenter-provider-aws/assets/68304519/d1bc5704-7592-4aa1-a72e-83d33f824b8b)

See [this commit](https://github.com/kubernetes-sigs/karpenter/pull/1294/commits/88134e00ad02863d5a7268bba0b639e21f3f5398) for a proof-of-concept implementation. 

**Pros:**
- Solves 6+ minute delays by default
- No additional latency before starting instance termination. (In my tests, a 6 second wait was sufficient for AttachDetach Controller to recognize the out-of-service taint and allow for volume detach)
- If Kubernetes makes this 6 minute ForceDetach timer infinite by default (As currently planned in Kubernetes v1.32), the out-of-service taint will be the only ensure workload starts on other node 

**Cons:**
- Unavailable until Kubernetes `v1.26`. Customers running Karpenter on Kubernetes ≤ `v1.25` will require solution B1 to be implemented. 
- Requires Karpenter-Provider-AWS to not treat `Shutting Down` as terminated (Though Karpenter had already planned on this via [PR #5979](https://github.com/aws/karpenter-provider-aws/pull/5979))
- Problem B's delay still occurs because we must wait until instance terminates. 

#### Alternatives Considered

**Customer configuration**

Customers can mitigate 6+ minute delays by configuring their nodes and pod taint tolerations. See [EBS CSI Driver FAQ: Mitigating 6+ minute delays](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/faq.md#what-steps-can-be-taken-to-mitigate-this-issue)

A quick overview:
- Configure Kubelet for Graceful Node Shutdown
- Enable Karpenter Spot Instance interruption handling
- Use EBS CSI Driver ≥ `v1.x` in order use the [PreStop Lifecycle Hook](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/faq.md#what-is-the-prestop-lifecycle-hook)
- Set `.node.tolerateAllTaints=false` when deploying the EBS CSI Driver
[](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/faq.md#what-steps-can-be-taken-to-mitigate-this-issue)
- Confirm that your stateful workloads do not tolerate Karpenter's `disrupting` taint, and any that any system-critical stateful daemonsets have a lower terminationGracePeriod than the EBS CSI Driver.

**Pros:**
- No code change required in Karpenter. 
**Cons:**
- Requiring configuration is a poor customer experience. Troubleshooting this configuration is difficult. (Hence why issues are still being raised on Karpenter and EBS CSI Driver Projects)
- Stateful workloads that tolerate Karpenter's `disrupting` taint, or any system-critical stateful daemonsets with a higher terminationGracePeriod than the EBS CSI Driver will still see migration delays. 

## Problem B. Preventing Delayed Detachments

Even if we solve the 6+ minute volume-in-use delay, AWS customers may suffer from a second type of delay due to behavior specific to EC2.  

If Karpenter calls EC2 TerminateInstance **before** EC2 DetachVolume calls finish, then the volumes won't be detached **until the old instance terminates**. This delay depends on the instance type. 1 minute for `m5a.large`, up to 15 minutes for certain GPU/Windows instances (especially painful for ML/AI customers).

Operators will see `FailedAttachVolume` events with `Multi-Attach error` and then `AttachVolume.Attach failed` errors. 

### Solution B1: Wait for unmounts in Karpenter cloudProvider.Delete

Wait for volumes to unmount before terminating the instance. 

We can do this by waiting for all volumes of drain-able nodes to be marked as not be in use before terminating the node in c.cloudProvider.Delete (until a maximum of 20 seconds). 

This means that our sequence of events will match the ideal diagram from section [### Ideal Graceful Shutdown for Stateful Workloads]

We can use similar logic to [today's proof-of-concept implementation](https://github.com/kubernetes-sigs/karpenter/pull/1294), but move it to karpenter-provider-aws and check for `node.Status.VolumesInUse` instead of listing volumeattachment objects. A 20 second max wait was sufficient to prevent delays with m5a instance type, but further testing is needed to ensure it is enough for Windows/GPU instance types.

**Pros:**
- Leaves decision to each cloud/storage provider 
- Can opt-in to this behavior for specific CSI Drivers (Perhaps via Helm parameter)
- Only delays termination of nodes with stateful workloads.
**Cons:**
- Delays node termination and finalizer deletion by a worst-case of 20 seconds. (We can skip waiting on the volumes of non-drainable pods to make the average case lower)
- Other CSI Drivers must opt-in

### Alternatives Considered

**Implement B1 in `kubernetes-sigs/karpenter`**

Instead of solving this inside the [`c.cloudProvider.Delete`](https://github.com/aws/karpenter-provider-aws/blob/9cef47b6df77ec9e0a39dc6f4a4ecd1aab504ae3/pkg/cloudprovider/cloudprovider.go#L179), solve this inside [termination controller's reconciler loop](https://github.com/kubernetes-sigs/karpenter/blob/94d5b41d4711b1c21fc0264f1b29db6a64b95caf/pkg/controllers/node/termination/controller.go#L77) (as is done in [today's proof-of-concept implementation](https://github.com/kubernetes-sigs/karpenter/pull/1294))

**Pros:**
- Karpenter-provider-aws does not need to know about Kubernetes volume lifecycle
**Cons:**
- [Open Question] EBS may be the only storage provider where detaching volumes before terminating the instance matters. If this is the case, the delay before instance termination is not worth it for customers of other storage/cloud providers.
- Should not hardcode cloud-provider specific CSI Drivers in upstream project. 

**Karpenter Polls EC2 for volume detachments before calling EC2 TerminateInstance**

Karpenter-provider-aws can poll EC2 DescribeVolumes before making an EC2 TerminateInstance call. 

**Pros:**
- Solves issue
**Cons:**
- If volumes are not unmounted due to problem A, volumes cannot be detached anyway. Termination is the fastest way forward.
- Karpenter has to worry about EBS volume lifecycle. 

## Appendix 

### Z. Document TODOs

- Mention that volumes with Multi-Attach enabled do not have this problem
- Move images to gists before PR merge
- Increase resolution of pngs. 
- Expand Appendix terminology + timeline sections
- Prove what potential data corruption issues we are talking about. 

### A. Further Reading

- [EBS CSI: What causes 6-minute delays in attaching volumes?](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/faq.md#6-minute-delays-in-attaching-volumes)
- Kubernetes:
  - [Node Shutdowns](https://kubernetes.io/docs/concepts/cluster-administration/node-shutdown/)
  - [API-initiated Eviction](https://kubernetes.io/docs/concepts/scheduling-eviction/api-eviction/#how-api-initiated-eviction-works)


### B. Terminology
Daemonset: Pod scheduled on every Node. 
EBS: Elastic Block Store. An Amazon Web Service that 
Kubelet: 
StatefulSet: Manages the deployment and scaling of a set of Pods, and provides guarantees about the ordering and uniqueness of these Pods. These uniqueness gauruntees are valuable when your workload needs persistent storage. 

### C. Issue Timeline

### D. Footnotes

[^1]: From my testing via the EBS CSI Driver, EBS volumes typically take 10 seconds to detach and 5 seconds to attach. But there is no Attach/Detach SLA. 

[^2]: [Complicated stateless diagram](https://github.com/AndrewSirenko/karpenter-provider-aws/assets/68304519/f8fdc38d-be65-4de1-9837-827f65707fa5).

[^3]: EBS CSI Node service is called by the node's Kubelet's Volume Manager twice after a volume attachment. Once to format the block device and mount filesystem on a node global directory, and a second time to mount on the pod's directory.   

[^4]: For certain storage providers, this delay in pod restart can prevent potential data corruption due to unclean mounts. (Is this true for EBS? I'm skeptical that these data corruption issues exist for non-multi-attach EBS volumes. EC2 does not allow mounted volumes to be detached, and most Linux distributions unmount all filesystems/volumes during shut down. Finally when the volume is attached to the new node, I believe CSI Node pods run e2fsck before formatting volumes and *never* forcefully reformat volumes)

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
