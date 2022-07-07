---
title: "Scheduling"
linkTitle: "Scheduling"
weight: 15
description: >
  Learn about scheduling workloads with Karpenter
---

If your pods have no requirements for how or where to run, you can let Karpenter choose nodes from the full range of available cloud provider resources.
However, by taking advantage of Karpenter's model of layered constraints, you can be sure that the precise type and amount of resources needed are available to your pods.
Reasons for constraining where your pods run could include:

* Needing to run in zones where dependent applications or storage are available
* Requiring certain kinds of processors or other hardware
* Wanting to use techniques like topology spread to help ensure high availability

Your Cloud Provider defines the first layer of constraints, including all instance types, architectures, zones, and purchase types available to its cloud.
The cluster administrator adds the next layer of constraints by creating one or more provisioners.
The final layer comes from you adding specifications to your Kubernetes pod deployments.
Pod scheduling constraints must fall within a provisioner's constraints or the pods will not deploy.
For example, if the provisioner sets limits that allow only a particular zone to be used, and a pod asks for a different zone, it will not be scheduled.

Constraints you can request include:

* **Resource requests**: Request that certain amount of memory or CPU be available.
* **Node selection**: Choose to run on a node that is has a particular label (`nodeSelector`).
* **Node affinity**: Draws a pod to run on nodes with particular attributes (affinity).
* **Topology spread**: Use topology spread to help ensure availability of the application.
* **Pod affinity/anti-affinity**: Draws pods towards or away from topology domains based on the scheduling of other pods.

Karpenter supports standard Kubernetes scheduling constraints.
This allows you to define a single set of rules that apply to both existing and provisioned capacity.

{{% alert title="Note" color="primary" %}}
Karpenter supports specific [Well-Known Labels, Annotations and Taints](https://kubernetes.io/docs/reference/labels-annotations-taints/) that are useful for scheduling.
{{% /alert %}}

## Resource requests

Within a Pod spec, you can both make requests and set limits on resources a pod needs, such as CPU and memory.
For example:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: myapp
spec:
  containers:
  - name: app
    image: myimage
    resources:
      requests:
        memory: "128Mi"
        cpu: "500m"
      limits:
        memory: "256Mi"
        cpu: "1000m"
```
In this example, the container is requesting 128MiB of memory and .5 CPU.
Its limits are set to 256MiB of memory and 1 CPU.
Instance type selection math only uses `requests`, but `limits` may be configured to enable resource oversubscription.


See [Managing Resources for Containers](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/) for details on resource types supported by Kubernetes, [Specify a memory request and a memory limit](https://kubernetes.io/docs/tasks/configure-pod-container/assign-memory-resource/#specify-a-memory-request-and-a-memory-limit) for examples of memory requests, and [Provisioning Configuration](../../aws/provisioning/) for a list of supported resources.


## Selecting nodes

With `nodeSelector` you can ask for a node that matches selected key-value pairs.
This can include well-known labels or custom labels you create yourself.

While `nodeSelector` is like node affinity, it doesn't have the same "and/or" matchExpressions that affinity has.
So all key-value pairs must match if you use `nodeSelector`.
Also, `nodeSelector` can do only do inclusions, while `affinity` can do inclusions and exclusions (`In` and `NotIn`).

### Supported Labels
The following labels are supported by Karpenter. They may be specified as provisioner requirements or pod scheduling constraints.

| Label                                       | Example    | Description                                                                                                                                 |
| ------------------------------------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| topology.kubernetes.io/zone                 | us-west-2a | Zones are defined by your cloud provider ([aws](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html)) |
| node.kubernetes.io/instance-type            | p3.8xlarge | Instance types are defined by your cloud provider ([aws](https://aws.amazon.com/ec2/instance-types/))                                       |
| kubernetes.io/os                            | linux      | Operating systems are defined by [GOOS values](https://github.com/golang/go/blob/master/src/go/build/syslist.go#L10) on the instance        |
| kubernetes.io/arch                          | amd64      | Architectures are defined by [GOARCH values](https://github.com/golang/go/blob/master/src/go/build/syslist.go#L50) on the instance          |
| karpenter.sh/capacity-type                  | spot       | Capacity types include `spot`, `on-demand`                                                                                                  |
| karpenter.k8s.aws/instance-family           | p3         | [AWS Specific] Instance types of similar properties but different resource quantities                                                       |
| karpenter.k8s.aws/instance-size             | 8xlarge    | [AWS Specific] Instance types of similar resource quantities but different properties                                                       |
| karpenter.k8s.aws/instance-cpu              | 32         | [AWS Specific] Number of CPUs on the instance                                                                                               |
| karpenter.k8s.aws/instance-memory           | 249856     | [AWS Specific] Number of mebibytes of memory on the instance                                                                                |
| karpenter.k8s.aws/instance-gpu-name         | v100       | [AWS Specific] Name of the GPU on the instance, if available                                                                                |
| karpenter.k8s.aws/instance-gpu-manufacturer | nvidia     | [AWS Specific] Name of the GPU manufacturer                                                                                                 |
| karpenter.k8s.aws/instance-gpu-count        | 4          | [AWS Specific] Number of GPUs on the instance                                                                                               |
| karpenter.k8s.aws/instance-gpu-memory       | 16384      | [AWS Specific] Number of mebibytes of memory on the GPU                                                                                     |

### Node selectors

Here is an example of a `nodeSelector` for selecting nodes:

```yaml
nodeSelector:
  topology.kubernetes.io/zone: us-west-2a
  karpenter.sh/capacity-type: spot
```
This example features a well-known label (`topology.kubernetes.io/zone`) and a label that is well known to Karpenter (`karpenter.sh/capacity-type`).

If you want to create a custom label, you should do that at the provisioner level.
Then the pod can declare that custom label.


See [nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector) in the Kubernetes documentation for details.

### Node affinity

Examples below illustrate how to use Node affinity to include (`In`) and exclude (`NotIn`) objects.
See [Node affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) for details.
When setting rules, the following Node affinity types define how hard or soft each rule is:

* **requiredDuringSchedulingIgnoredDuringExecution**: This is a hard rule that must be met.
* **preferredDuringSchedulingIgnoredDuringExecution**: This is a preference, but the pod can run on a node where it is not guaranteed.

The `IgnoredDuringExecution` part of each tells the pod to keep running, even if conditions change on the node so the rules no longer matched.
You can think of these concepts as `required` and `preferred`, since Kubernetes never implemented other variants of these rules.

All examples below assume that the provisioner doesn't have constraints to prevent those zones from being used.
The first constraint says you could use `us-west-2a` or `us-west-2b`, the second constraint makes it so only `us-west-2b` can be used.

```yaml
 affinity:
   nodeAffinity:
     requiredDuringSchedulingIgnoredDuringExecution:
       nodeSelectorTerms:
         - matchExpressions:
           - key: "topology.kubernetes.io/zone"
             operator: "In"
             values: ["us-west-2a, us-west-2b"]
           - key: "topology.kubernetes.io/zone"
             operator: "In"
             values: ["us-west-2b"]
```

Changing the second operator to `NotIn` would allow the pod to run in `us-west-2a` only:

```yaml
           - key: "topology.kubernetes.io/zone"
             operator: "In"
             values: ["us-west-2a, us-west-2b"]
           - key: "topology.kubernetes.io/zone"
             operator: "NotIn"
             values: ["us-west-2b"]
```

Continuing to add to the example, `nodeAffinity` lets you define terms so if one term doesn't work it goes to the next one.
Here, if `us-west-2a` is not available, the second term will cause the pod to run on a spot instance in `us-west-2d`.


```yaml
 affinity:
   nodeAffinity:
     requiredDuringSchedulingIgnoredDuringExecution:
       nodeSelectorTerms:
         - matchExpressions: # OR
           - key: "topology.kubernetes.io/zone" # AND
             operator: "In"
             values: ["us-west-2a, us-west-2b"]
           - key: "topology.kubernetes.io/zone" # AND
             operator: "NotIn"
             values: ["us-west-2b"]
         - matchExpressions: # OR
           - key: "karpenter.sh/capacity-type" # AND
             operator: "In"
             values: ["spot"]
           - key: "topology.kubernetes.io/zone" # AND
             operator: "In"
             values: ["us-west-2d"]
```
In general, Karpenter will go through each of the `nodeSelectorTerms` in order and take the first one that works.
However, if Karpenter fails to provision on the first `nodeSelectorTerms`, it will try again using the second one.
If they all fail, Karpenter will fail to provision the pod.
Karpenter will backoff and retry over time.
So if capacity becomes available, it will schedule the pod without user intervention.

## Taints and tolerations

Taints are the opposite of affinity.
Setting a taint on a node tells the scheduler to not run a pod on it unless the pod has explicitly said it can tolerate that taint.
This example shows a Provisioner that was set up with a taint for only running pods that require a GPU, such as the following:


```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: gpu
spec:
  requirements:
  - key: karpenter.k8s.aws/instance-family
    operator: In
    values:
      - p3
  taints:
  - key: nvidia.com/gpu
    value: true
    effect: "NoSchedule"
```

For a pod to request to run on a node that has provisioner, it could set a toleration as follows:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: mygpupod
spec:
  containers:
  - name: gpuapp
    resources:
      requests:
        nvidia.com/gpu: 1
      limits:
        nvidia.com/gpu: 1
    image: mygpucontainer
  tolerations:
  - key: "nvidia.com/gpu"
    operator: "Exists"
    effect: "NoSchedule"
```
See [Taints and Tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) in the Kubernetes documentation for details.

## Topology Spread

By using the Kubernetes `topologySpreadConstraints` you can ask the provisioner to have pods push away from each other to limit the blast radius of an outage.
Think of it as the Kubernetes evolution for pod affinity: it lets you relate pods with respect to nodes while still allowing spread.
For example:

```yaml
spec:
  topologySpreadConstraints:
    - maxSkew: 1
      topologyKey: "topology.kubernetes.io/zone"
      whenUnsatisfiable: ScheduleAnyway
      labelSelector:
        matchLabels:
          dev: jjones
    - maxSkew: 1
      topologyKey: "kubernetes.io/hostname"
      whenUnsatisfiable: ScheduleAnyway
      labelSelector:
        matchLabels:
          dev: jjones
    - maxSkew: 1
      topologyKey: "karpenter.sh/capacity-type"
      whenUnsatisfiable: ScheduleAnyway
      labelSelector:
        matchLabels:
          dev: jjones

```
Adding this to your podspec would result in:

* Pods being spread across zones, hosts, and capacity-type (`topologyKey`).
* The `dev` `labelSelector` will include all pods with the label of `dev=jjones` in topology calculations. It is recommended to use a selector to match all pods in a deployment.
* No more than one pod difference in the number of pods on each host (`maxSkew`).
For example, if there were three nodes and five pods the pods could be spread 1, 2, 2 or 2, 1, 2 and so on.
If instead the spread were 5, pods could be 5, 0, 0 or 3, 2, 0, or 2, 1, 2 and so on.

The three supported `topologyKey` values that Karpenter supports are:
- `topology.kubernetes.io/zone`
- `kubernetes.io/hostname`
- `karpenter.sh/capacity-type`


See [Pod Topology Spread Constraints](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/) for details.

## Pod affinity/anti-affinity

By using the `podAffinity` and `podAntiAffinity` configuration on a pod spec, you can inform the provisioner of your desire for pods to schedule together or apart with respect to different topology domains. For example:

```yaml
spec:
  affinity:
    podAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchExpressions:
          - key: system
            operator: In
            values:
            - backend
        topologyKey: topology.kubernetes.io/zone
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            app: inflate
        topologyKey: kubernetes.io/hostname
```

The above pod affinity rule would cause the pod to only schedule in zones where a pod with the label `system=backend` is already running.

The anti-affinity rule would cause it to avoid running on any node with a pod labeled `app=inflate`.  If this anti-affinity term was on a deployment pod spec along with a matching `app=inflate` label, it would prevent more than one pod from the deployment from running on any single node.

See [Inter-pod affinity and anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity) in the Kubernetes documentation for details.

## Persistent Volume Topology

Karpenter automatically detects storage scheduling requirements and includes them in node launch decisions.

In the following example, the `StorageClass` defines zonal topologies for `us-west-2a` and `us-west-2b` and [binding mode `WaitForFirstConsumer`](https://kubernetes.io/docs/concepts/storage/storage-classes/#volume-binding-mode).
When the pod is created, Karpenter follows references from the `Pod` to `PersistentVolumeClaim` to `StorageClass` and identifies that this pod requires storage in `us-west-2a` and `us-west-2b`.
It randomly selects `us-west-2a`, provisions a node in that zone, and waits for kube-scheduler to bind the pod to the node.
The CSI driver creates a `PersistentVolume` according to the `PersistentVolumeClaim` and gives it a node affinity rule for `us-west-2a`.

Later on, the pod is deleted and a new pod is created that requests the same claim. This time, Karpenter identifies that a `PersistentVolume` already exists for the `PersistentVolumeClaim`, and includes its zone `us-west-2a` in the pod's scheduling requirements.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: app
spec:
  containers: ...
  volumes:
    - name: storage
      persistentVolumeClaim:
        claimName: ebs-claim
---
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: ebs
provisioner: ebs.csi.aws.com
volumeBindingMode: WaitForFirstConsumer
allowedTopologies:
- matchLabelExpressions:
  - key: topology.ebs.csi.aws.com/zone
    values: ["us-west-2a", "us-west-2b"]
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ebs-claim
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: ebs
  resources:
    requests:
      storage: 4Gi
```

{{% alert title="Note" color="primary" %}}
☁️ AWS Specific

The EBS CSI driver uses `topology.ebs.csi.aws.com/zone` instead of the standard `topology.kubernetes.io/zone` label. Karpenter is aware of label aliasing and translates this label into `topology.kubernetes.io/zone` in memory. When configuring a `StorageClass` for the EBS CSI Driver, you must use `topology.ebs.csi.aws.com/zone`.
{{% /alert %}}

{{% alert title="Note" color="primary" %}}
The topology key `topology.kubernetes.io/region` is not supported. Legacy in-tree CSI providers specify this label. Instead, install an out-of-tree CSI provider. [Learn more about moving to CSI providers.](https://kubernetes.io/blog/2021/12/10/storage-in-tree-to-csi-migration-status-update/#quick-recap-what-is-csi-migration-and-why-migrate)
{{% /alert %}}
