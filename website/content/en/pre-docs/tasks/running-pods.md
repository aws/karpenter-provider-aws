---
title: "Running pods"
linkTitle: "Running pods"
weight: 10
---

If your pods have no requirements for how or where to run, you can let Karpenter choose nodes from the full range of available cloud provider resources.
However, by taking advantage of Karpenter's model of layered constraints, you can be sure that the precise type and amount of resources needed are available to your pods.
Reasons for constraining where your pods run could include:

<<<<<<< HEAD
=======
* Saving money by running in ways that are more efficient
>>>>>>> b05c290 (Add Running Pods task to docs)
* Needing to run in zones where dependent applications or storage are available
* Requiring certain kinds of processors or other hardware
* Wanting to use techniques like topology spread to help insure high availability

<<<<<<< HEAD
Your Cloud Provider defines the first layer of constraints, including all instance types, architectures, zones, and purchase types available to its cloud.
The cluster operator adds the next layer of constraints by creating one or more provisioners.
The final layer comes from you adding specifications to your Kubernetes pod deployments.
Pod scheduling constraints must fall within a provisioner's constraints or the pods will not deploy.
For example, if the provisioner sets limits that allow only a particular zone to be used, and a pod asks for a different zone, it will not be scheduled.
=======
Your Kubernetes cluster operator adds the first layer of constraints by creating one or more provisioners.
The next layer comes from you adding specifications to your Kubernetes pod deployments.
When you request constraints, those constraints must fall within the provisioner constraints or the pods will not deploy.
For example, if the provisioner sets limits that allow only a particular zone to be used, and a pod asks for a different zone, deployment will fail.
>>>>>>> b05c290 (Add Running Pods task to docs)

Constraints you can request include:

* **Resource requests**: Request that certain amount of memory or CPU be available.
<<<<<<< HEAD
* **Node selection**: Choose to run on a node that is has a particular label (`nodeSelector`).
* **Node affinity**: Draws a pod to run on nodes with particular attributes (affinity).
* **Topology spread**: Use topology spread to help insure availability of the application.

Karpenter supports standard Kubernetes scheduling constraints.
This allows you to define a single set of rules that apply to both existing and provisioned capacity.
Pod affinity is a key exception to this rule.

{{% alert title="Note" color="primary" %}}
Karpenter supports specific [Well-Known Labels, Annotations and Taints](Well-Known Labels, Annotations and Taints) that are useful for scheduling.
=======
* **Disruption budget**: Limit the disruptions that an application can experience.
* **Node selection**: Choose to run on a node that is has a particular label (`nodeSelector`) or name (`nodeName`).
* **Topology spread**: Use topology spread to help insure availability of the application.
* **Node affinity**: Draws a pod to run on nodes with particular attributes (affinity) or that don't have those attributes (antiaffinity).
* **Persistent volumes**: Insures the availability of selected persistent volumes.

The constraints that Karpenter supports are based mostly on features built into Kubernetes and a few that are specific to the cloud provider.
In fact, you the exact same constraints can be used on clusters by Cluster Autoscaler or even just by the Kubernetes scheduler to pre-provision capacity.
Keep in mind, however, that not all Kubernetes constraints are supported or recommended with Karpenter.
This section steps through examples of those that are supported.

{{% alert title="Note" color="primary" %}}
The constraints you identify through affinity and node selection include a subset of Kubernetes [Well-Known Labels, Annotations and Taints](Well-Known Labels, Annotations and Taints).
See [Specifying Values to Control AWS Provisioning](/docs/cloud-providers/aws/aws-spec-fields) for a listing and descriptions of those values.
>>>>>>> b05c290 (Add Running Pods task to docs)
{{% /alert %}}

## Resource requests (`resources`)

Within a Pod spec, you can both make requests and set limits on resources a pod needs, such as CPU and memory.
For example:

```
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
<<<<<<< HEAD
Instance type selection math only uses `requests`, but `limits` may be configured to enable resource oversubscription.
=======
>>>>>>> b05c290 (Add Running Pods task to docs)


See [Managing Resources for Containers](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/) for details on resource types supported by Kubernetes, [Specify a memory request and a memory limit](https://kubernetes.io/docs/tasks/configure-pod-container/assign-memory-resource/#specify-a-memory-request-and-a-memory-limit) for examples of memory requests, and [Specifying Values to Control AWS Provisioning](/docs/cloud-providers/aws/aws-spec-fields) for a list of supported resources.

<<<<<<< HEAD
## Selecting nodes (`nodeSelector` and `nodeAffinity`)
=======
## Disruption budget (`PodDisruptionBudget`)

Disruption budgets let you specify how much of a Deployment, ReplicationController, ReplicaSet, and StatefulSet must be protected from disruptions when pod eviction requests are made.
This feature can be used to strike a balance between protecting the application's availability while still allowing a cluster operator to manage the cluster.
For example:

```
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: myapp-pdb
spec:
  minAvailable: 4
  selector:
    matchLabels:
      app: myapp
```
In this example, the `myapp` pod is required to have at least 4 pods available after eviction.
Instead of setting a number, you could set a percentage.
So in this case if there were 10 pods, to get the same `minAvailable` you would set it to 40%.

See [Specifying a Disruption Budget for your Application](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) for details.

## Node selection (`nodeSelector` and `nodeName`)
>>>>>>> b05c290 (Add Running Pods task to docs)

With `nodeSelector` you can ask for a node that matches selected key-value pairs.
This can include well-known labels or custom labels you create yourself.

While `nodeSelector` is like node affinity, it doesn't have the same "and/or" matchExpressions that affinity has.
So all key-value pairs must match if you use `nodeSelector`.
<<<<<<< HEAD
Also, `nodeSelector` can do only do inclusions, while `affinity` can do inclusions and exclusions (`In` and `NotIn`).

### Node selector (`nodeSelector`)

Here is an example of a `nodeSelector` for selecting nodes:
=======
Also, `nodeSelector` can do only do inclusions, while `affinity can do inclusions and exclusions (`In` and `NotIn`).
Here is an example:
>>>>>>> b05c290 (Add Running Pods task to docs)

```
nodeSelector:
  topology.kubernetes.io/zone: us-west-2a
<<<<<<< HEAD
  karpenter.sh/capacity-type: spot
```
This example features a well-known label (`topology.kubernetes.io/zone`) and a label that is well known to Karpenter (`karpenter.sh/capacity-type`).

If you want to create a custom label, you should do that at the provisioner level.
Then the pod can declare that custom label.


See [nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector) in the Kubernetes documentation for details.

### Node affinity (`nodeAffinity`)

Examples below illustrate how to use Node affinity to include (`In`) and exclude (`NotIn`) objects.
=======
  node.k8.aws/capacity-type: spot
  jones.dev: john
```
This example features a well-known label (`topology.kubernetes.io/zone`), a label that is specific to the AWS cloud provider (`node.k8.aws/capacity-type`), and a custom label (`jonees.dev`).
Use a command like the following to set a custom label:

```
kubectl label nodes <node_name> <label_key>=<label_value>
```

See [nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector) in the Kubernetes documentation for details.

To select a node by name, use the `nodeName` object:

```
apiVersion: v1
kind: Pod
metadata:
  name: myapp
spec:
  containers:
  - name: myapp
    image: myapp
  nodeName: my-node
```
## Topology spread (`topologySpreadConstraints`)

By using the Kubernetes `topologySpreadConstraints` you can ask the provisioner to have pods push away from each other for high availability reasons.
Think of it as the Kubernetes evolution for pod affinity: it lets you relate pods with respect to nodes while still allowing spread.
For example:

```
topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: "topology.kubernetes.io/zone"
    topologyKey: "topology.kubernetes.io/hostname"
    whenUnsatisfiable: ScheduleAnyway
```
Adding this to your podspec would result in:

* Pods being spread across both zones and hosts (`topologyKey`)
* No more than one pod difference in the number of pods on each host (`maxSkew`).
For example, if there were three nodes and five pods the pods could be spread 1, 2, 2 or 2, 1, 2 and so on.
If instead the spread were 5, pods could be 5, 0, 0 or 3, 2, 0, or 2, 1, 2 and so on.
* If the skew cannot be met, schedule the pods anyway (`whenUnsatisfiable`).

See [Pod Topology Spread Constraints](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/) for details.

## Node affinity and anti-affinity (`nodeAffinity`)

Examples below illustrate how to use Node affinity (`In`) and anti-affinity (`NotIn`).
>>>>>>> b05c290 (Add Running Pods task to docs)
See [Node affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) for details.
When setting rules, the following Node affinity types define how hard or soft each rule is:

* **requiredDuringSchedulingIgnoredDuringExecution**: This is a hard rule that must be met.
* **preferredDuringSchedulingIgnoredDuringExecution**: This is a preference, but the pod can run on a node where it is not guaranteed.

The `IgnoredDuringExecution` part of each tells the pod to keep running, even if conditions change on the node so the rules no longer matched.
<<<<<<< HEAD
You can think of these concepts as `required` and `preferred`, since Kubernetes never implemented other variants of these rules.
=======
>>>>>>> b05c290 (Add Running Pods task to docs)

All examples below assume that the provisioner doesn't have constraints to prevent those zones from being used.
The first constraint says you could use `us-west-2a` or `us-west-2b`, the second constraint makes it so only `us-west-2b` can be used.

```
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

<<<<<<< HEAD
Changing the second operator to `NotIn` would allow the pod to run in `us-west-2a` only:
=======
Changing the second operator to anti-affinity (`NotIn`) would allow the pod to run in `us-west-2a` only:
>>>>>>> b05c290 (Add Running Pods task to docs)

```
           - key: "topology.kubernetes.io/zone"
             operator: "In"
             values: ["us-west-2a, us-west-2b"]
           - key: "topology.kubernetes.io/zone"
             operator: "NotIn"
             values: ["us-west-2b"]
```

<<<<<<< HEAD
Continuing to add to the example, `nodeAffinity` lets you define terms so if one term doesn't work it goes to the next one.
Here, if `us-west-2a` is not available, the second term will cause the pod to run on a spot instance in us-west-2d.
=======
Continuing to add to the example, nodeAffinity lets you match expressions so if one key-value pair doesn't work, you can go on to the next one.
Here, if `us-west-2a` is not available, the pod can go to the East zone and run on a spot instance (notice the AWS-specific key).
>>>>>>> b05c290 (Add Running Pods task to docs)


```
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
<<<<<<< HEAD
           - key: "karpenter.sh/capacity-type" # AND
=======
           - key: "node.k8.aws/capacity-type" # AND
>>>>>>> b05c290 (Add Running Pods task to docs)
             operator: "In"
             values: ["spot"]
           - key: "topology.kubernetes.io/zone" # AND
             operator: "In"
<<<<<<< HEAD
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


```
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: gpu
spec:
  requirements: 
  - key: node.kubernetes.io/instance-type
    operator: In
    values:
      - p3.2xlarge
      - p3.8xlarge
      - p3.16xlarge
  taints:
  - key: nvidia.com/gpu
    value: true
    effect: “NoSchedule”
```

For a pod to request to run on a node that has provisioner, it could set a toleration as follows:

```
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
See Taints and Tolerations (https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) in the Kubernetes documentation for details.

## Topology spread (`topologySpreadConstraints`)

By using the Kubernetes `topologySpreadConstraints` you can ask the provisioner to have pods push away from each other to limit the blast radius of an outage.
Think of it as the Kubernetes evolution for pod affinity: it lets you relate pods with respect to nodes while still allowing spread.
For example:

```
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

```
Adding this to your podspec would result in:

* Pods being spread across both zones and hosts (`topologyKey`).
* The `dev` `labelSelector` will include all pods with the label of `dev=jjones` in topology calculations. It is recommended to use a selector to match all pods in a deployment.
* No more than one pod difference in the number of pods on each host (`maxSkew`).
For example, if there were three nodes and five pods the pods could be spread 1, 2, 2 or 2, 1, 2 and so on.
If instead the spread were 5, pods could be 5, 0, 0 or 3, 2, 0, or 2, 1, 2 and so on.
* Karpenter is always able to improve skew by launching new nodes in the right zones. Therefore, `whenUnsatisfiable` does not change provisioning behavior.

See [Pod Topology Spread Constraints](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/) for details.
=======
             values: ["us-east-2a"]
```
Karpenter will go through each of the matchExpressions in order and take the first one that works.
However, if Karpenter fails to provision on the first matchExpression, it will delete it and try to use the second one.
If they all fail, Karpenter will fail to provision for and deploy the pod.

## Persistent volumes (`VolumeNodeAffinity`)

How does a pod request to run on a node that a particular persistent volume present?

>>>>>>> b05c290 (Add Running Pods task to docs)
