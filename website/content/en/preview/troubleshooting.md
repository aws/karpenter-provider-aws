---
title: "Troubleshooting"
linkTitle: "Troubleshooting"
weight: 70
description: >
  Troubleshoot Karpenter problems
---

## Controller

### Enable debug logging

This can be done by updating the env variable `LOG_LEVEL` Karpenter deployment and then restarting the Karpenter deployment.

You can also enable debug logging during installation with Helm by setting the option `logLevel`.

```
helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter \
  --set logLevel=debug \
  ...
```

## Installation

### Missing Service Linked Role

Unless your AWS account has already onboarded to EC2 Spot, you will need to create the service linked role to avoid `ServiceLinkedRoleCreationNotPermitted`.

```
AuthFailure.ServiceLinkedRoleCreationNotPermitted: The provided credentials do not have permission to create the service-linked role for EC2 Spot Instances
```

This can be resolved by creating the [Service Linked Role](https://docs.aws.amazon.com/batch/latest/userguide/spot_fleet_IAM_role.html).

```
aws iam create-service-linked-role --aws-service-name spot.amazonaws.com
```

### Failed Resolving STS Credentials with I/O Timeout

```bash
Checking EC2 API connectivity, WebIdentityErr: failed to retrieve credentials\ncaused by: RequestError: send request failed\ncaused by: Post \"https://sts.us-east-1.amazonaws.com/\": dial tcp: lookup sts.us-east-1.amazonaws.com: i/o timeout
```

If you see the error above when you attempt to install Karpenter, this indicates that Karpenter is unable to reach out to the STS endpoint due to failed DNS resolution. This can happen when Karpenter is running with `dnsPolicy: ClusterFirst` and your in-cluster DNS service is not yet running.

You have two mitigations to resolve this error:
1. Let Karpenter manage your in-cluster DNS service - You can let Karpenter manage your DNS application pods' capacity by changing Karpenter's `dnsPolicy` to be `Default` (run `--set dnsPolicy=Default` with a Helm installation). This ensures that Karpenter reaches out to the [VPC DNS service](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-dns.html) when running its controllers, allowing Karpenter to start-up without the DNS application pods running, enabling Karpenter to manage the capacity for these pods.
2. Let MNG/Fargate manage your in-cluster DNS service - If running a cluster with MNG, ensure that your group has enough capacity to support the DNS application pods and ensure that the application has the correct tolerations to schedule against the capacity. If running a cluster with Fargate, ensure that you have a [fargate profile](https://docs.aws.amazon.com/eks/latest/userguide/fargate-profile.html) that selects against your DNS application pods.

### Karpenter Role names exceeding 64-character limit

If you use a tool such as AWS CDK to generate your Kubernetes cluster name, when you add Karpenter to your cluster you could end up with a cluster name that is too long to incorporate into your KarpenterNodeRole name (which is limited to 64 characters).

Node role names for Karpenter are created in the form `KarpenterNodeRole-${Cluster_Name}` in the [Create the KarpenterNode IAM Role]({{<ref "./getting-started/getting-started-with-karpenter/#create-the-karpenternode-iam-role" >}}) section of the getting started guide.
If a long cluster name causes the Karpenter node role name to exceed 64 characters, creating that object will fail.

Keep in mind that `KarpenterNodeRole-` is just a recommendation from the getting started guide.
Instead of using the eksctl role, you can shorten the name to anything you like, as long as it has the right permissions.

### Unknown field in NodePool or EC2NodeClass spec

If you are upgrading from an older version of Karpenter, there may have been changes in the CRD between versions. Attempting to utilize newer functionality which is surfaced in newer versions of the CRD may result in the following error message:

```
Error from server (BadRequest): error when creating "STDIN": NodePool in version "v1" cannot be handled as a NodePool: strict decoding error: unknown field "spec.template.spec.nodeClassRef.foo"
```

If you see this error, you can solve the problem by following the [Custom Resource Definition Upgrade Guidance](../upgrade-guide/#custom-resource-definition-crd-upgrades).

Info on whether there has been a change to the CRD between versions of Karpenter can be found in the [Release Notes](../upgrade-guide/#released-upgrade-notes)

### Unable to schedule pod due to insufficient node group instances

`0.16.0` changed the default replicas from 1 to 2.

Karpenter won't launch capacity to run itself (log related to the `karpenter.sh/nodepool DoesNotExist requirement`)
so it can't provision for the second Karpenter pod.

To solve this you can either reduce the replicas back from 2 to 1, or ensure there is enough capacity that isn't being managed by Karpenter to run both pods.

To do so on AWS increase the `minimum` and `desired` parameters on the node group autoscaling group to launch at lease 2 instances.

### Helm Error When Pulling the Chart

If Helm is showing an error when trying to install Karpenter Helm charts:

- Ensure you are using a newer Helm version, Helm started supporting OCI images since `3.8.0`.
- Helm does not have an `helm repo add` concept in OCI, so to install Karpenter you no longer need this.
- If you get an error like `Error: public.ecr.aws/karpenter/karpenter:0.34.0: not found` make sure you're adding a `v` prefix for Karpenter versions between `0.17.0` & `0.34.x`.
- Verify that the image you are trying to pull actually exists in [gallery.ecr.aws/karpenter](https://gallery.ecr.aws/karpenter/karpenter)
- Sometimes Helm generates a generic error, you can add the --debug switch to any of the Helm commands in this doc for more verbose error messages
- If you are getting a 403 forbidden error, you can try `docker logout public.ecr.aws` as explained [here](https://docs.aws.amazon.com/AmazonECR/latest/public/public-troubleshooting.html).

### Helm Error when installing the `karpenter-crd` chart

Karpenter `0.26.1` introduced the `karpenter-crd` Helm chart. When installing this chart on your cluster, if you have previously added the Karpenter CRDs to your cluster through the `karpenter` controller chart or through `kubectl replace`, Helm will reject the install of the chart due to `invalid ownership metadata`.

- In the case of `invalid ownership metadata; label validation error: missing key "app.kubernetes.io/managed-by": must be set to "Helm"` run:

```shell
kubectl label crd ec2nodeclasses.karpenter.k8s.aws nodepools.karpenter.sh nodeclaims.karpenter.sh app.kubernetes.io/managed-by=Helm --overwrite
```

- In the case of `annotation validation error: missing key "meta.helm.sh/release-namespace": must be set to "karpenter"` run:

```shell
KARPENTER_NAMESPACE=kube-system
kubectl annotate crd ec2nodeclasses.karpenter.k8s.aws nodepools.karpenter.sh nodeclaims.karpenter.sh meta.helm.sh/release-name=karpenter-crd --overwrite
kubectl annotate crd ec2nodeclasses.karpenter.k8s.aws nodepools.karpenter.sh nodeclaims.karpenter.sh meta.helm.sh/release-namespace="${KARPENTER_NAMESPACE}" --overwrite
```

## Upgrade

### Karpenter upgrade impairs cascade deletion of Kubernetes resources

This error happens because of an upgrade between API versions of the Karpenter CRD (e.j. v1beta1 to v1). This issue can lead to child pods not being deleted when deleting the owner resource. For example, the deletion of a deployment does not delete the child pods afterwards.
The kube-controller-manager logs will show the following message:

```text
conversion webhook for karpenter.k8s.aws/v1beta1, Kind=EC2NodeClass failed: Post "https://karpenter.kube-system.svc:8443/conversion/karpenter.k8s.aws?timeout=30s": no endpoints available for service "karpenter"
```

To fix the issue, delete all Karpenter CRDs from the cluster and perform a clean install of Karpenter. Make sure to backup any NodePool and NodeClass objects in the cluster. Follow the [Karpenter upgrade guide](https://karpenter.sh/docs/upgrading/upgrade-guide/) to avoid this from happening.

## Uninstallation

### Unable to delete nodes after uninstalling Karpenter

Karpenter adds a [finalizer](https://github.com/aws/karpenter/pull/466) to nodes that it provisions to support graceful node termination. If Karpenter is uninstalled, these finalizers will cause the API Server to block deletion until the finalizers are removed.

You can fix this by patching the node objects:

- `kubectl edit node <node_name>` and remove the line that says `karpenter.sh/termination` in the finalizers field.
- Run the following script that gets all nodes with the finalizer and removes all the finalizers from those nodes.
  - NOTE: this will remove ALL finalizers from nodes with the karpenter finalizer.

```bash
kubectl get nodes -ojsonpath='{range .items[*].metadata}{@.name}:{@.finalizers}{"\n"}' | grep "karpenter.sh/termination" | cut -d ':' -f 1 | xargs kubectl patch node --type='json' -p='[{"op": "remove", "path": "/metadata/finalizers"}]'
```

## Provisioning

### Instances with swap volumes fail to register with control plane

Some instance types (c1.medium and m1.small) are given limited amount of memory (see [Instance Store swap volumes](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-store-swap-volumes.html)). They are subsequently configured to use a swap volume, which will cause the kubelet to fail on launch. The following error can be seen in the systemd logs:

```bash
"command failed" err="failed to run Kubelet: running with swap on is not supported, please disable swap!..."
```

##### Solutions
Disabling swap will allow kubelet to join the cluster successfully, however users should be mindful of performance, and consider adjusting the NodePool requirements to use larger instance types.

### DaemonSets can result in deployment failures

For Karpenter versions `0.5.3` and earlier, DaemonSets were not properly considered when provisioning nodes.
This sometimes caused nodes to be deployed that could not meet the needs of the requested DaemonSets and workloads.
This issue no longer occurs after Karpenter version `0.5.3` (see [PR #1155](https://github.com/aws/karpenter/pull/1155)).

If you are using a pre `0.5.3` version of Karpenter, one workaround is to set your NodePool to only use larger instance types that you know will be big enough for the DaemonSet and the workload.
For more information, see [Issue #1084](https://github.com/aws/karpenter/issues/1084).
Examples of this behavior are included in [Issue #1180](https://github.com/aws/karpenter/issues/1180).

### Unspecified resource requests cause scheduling/bin-pack failures

Not using the Kubernetes [LimitRanges](https://kubernetes.io/docs/concepts/policy/limit-range/) feature to enforce minimum resource request sizes will allow pods with very low or non-existent resource requests to be scheduled.
This can cause issues as Karpenter bin-packs pods based on the resource requests.

If the resource requests do not reflect the actual resource usage of the pod, Karpenter will place too many of these pods onto the same node resulting in the pods getting CPU throttled or terminated due to the OOM killer.
This behavior is not unique to Karpenter and can also occur with the standard `kube-scheduler` with pods that don't have accurate resource requests.

To prevent this, you can set LimitRanges on pod deployments on a per-namespace basis.
See the Karpenter [Best Practices Guide](https://aws.github.io/aws-eks-best-practices/karpenter/#use-limitranges-to-configure-defaults-for-resource-requests-and-limits) for further information on the use of LimitRanges.

### Pods using Security Groups for Pods stuck in "ContainerCreating" state for up to 30 minutes before transitioning to "Running"

When leveraging [Security Groups for Pods](https://docs.aws.amazon.com/eks/latest/userguide/security-groups-for-pods.html), Karpenter will launch nodes as expected but pods will be stuck in "ContainerCreating" state for up to 30 minutes before transitioning to "Running".
This is related to an interaction between Karpenter and the [amazon-vpc-resource-controller](https://github.com/aws/amazon-vpc-resource-controller-k8s) when a pod requests `vpc.amazonaws.com/pod-eni` resources.
More info can be found in [issue #1252](https://github.com/aws/karpenter/issues/1252).

To workaround this problem, add the `vpc.amazonaws.com/has-trunk-attached: "false"` label in your Karpenter NodePool spec and ensure instance-type requirements include [instance-types which support ENI trunking](https://github.com/aws/amazon-vpc-resource-controller-k8s/blob/master/pkg/aws/vpc/limits.go).

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: default
spec:
  template
    metadata:
      labels:
        vpc.amazonaws.com/has-trunk-attached: "false"
```

### Pods using PVCs can hit volume limits and fail to scale-up

When attempting to schedule a large number of pods with PersistentVolumes, it's possible that these pods will co-locate on the same node. Pods will report the following errors in their events using a `kubectl describe pod` call

```bash
Warning   FailedAttachVolume    pod/example-pod                      AttachVolume.Attach failed for volume "***" : rpc error: code = Internal desc = Could not attach volume "***" to node "***": attachment of disk "***" failed, expected device to be attached but was attaching
Warning   FailedMount           pod/example-pod                      Unable to attach or mount volumes: unmounted volumes=[***], unattached volumes=[***]: timed out waiting for the condition
```

In this case, Karpenter may fail to scale-up your nodes due to these pods due to one of the following reasons:

#### Pods were not scheduled but Karpenter couldn't discover limits

Karpenter does not support [in-tree storage plugins](https://kubernetes.io/blog/2021/12/10/storage-in-tree-to-csi-migration-status-update/) to provision PersistentVolumes, since nearly all of the in-tree plugins have been deprecated in upstream Kubernetes. This means that, if you are using a statically-provisioned PersistentVolume that references a volume source like `AWSElasticBlockStore` or a dynamically-provisioned PersistentVolume that references a StorageClass with a in-tree storage plugin provisioner like `kubernetes.io/aws-ebs`, Karpenter will fail to discover the maxiumum volume attachments for the node. Instead, Karpenter may think the node still has more schedulable space due to memory and cpu constraints when there is really no more schedulable space on the node due to volume limits. When Karpenter sees you are using an in-tree storage plugin on your pod volumes, it will print the following error message into the logs. If you see this message, upgrade your StorageClasses and statically-provisioned PersistentVolumes to use the latest CSI drivers for your cloud provider.

```bash
2023-04-05T23:56:53.363Z        ERROR   controller.node_state   PersistentVolume source 'AWSElasticBlockStore' uses an in-tree storage plugin which is unsupported by Karpenter and is deprecated by Kubernetes. Scale-ups may fail because Karpenter will not discover driver limits. Use a PersistentVolume that references the 'CSI' volume source for Karpenter auto-scaling support.       {"commit": "b2af562", "node": "ip-192-168-36-137.us-west-2.compute.internal", "pod": "inflate0-6c4bdb8b75-7qmfd", "volume": "mypd", "persistent-volume": "pvc-11db7489-3c6e-46f3-a958-91f9d5009d41"}
2023-04-05T23:56:53.464Z        ERROR   controller.node_state   StorageClass .spec.provisioner uses an in-tree storage plugin which is unsupported by Karpenter and is deprecated by Kubernetes. Scale-ups may fail because Karpenter will not discover driver limits. Create a new StorageClass with a .spec.provisioner referencing the CSI driver plugin name 'ebs.csi.aws.com'.     {"commit": "b2af562", "node": "ip-192-168-36-137.us-west-2.compute.internal", "pod": "inflate0-6c4bdb8b75-7qmfd", "volume": "mypd", "storage-class": "gp2", "provisioner": "kubernetes.io/aws-ebs"}
```

#### Pods were scheduled due to a race condition in Kubernetes

Due to [this race condition in Kubernetes](https://github.com/kubernetes/kubernetes/issues/95911), it's possible that the scheduler and the CSINode can race during node registration such that the scheduler assumes that a node can mount more volumes than the node attachments support. There is currently no universal solve for this problem other than enforcing `topologySpreadConstraints` and `podAntiAffinity` on your workloads that use PVCs such that you attempt to reduce the number of PVCs that schedule to a given node.

The following is a list of known CSI drivers which support a startupTaint to eliminate this issue:
- [aws-ebs-csi-driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/docs/install.md#configure-node-startup-taint)
- [aws-efs-csi-driver](https://github.com/kubernetes-sigs/aws-efs-csi-driver/tree/master/docs#configure-node-startup-taint)

These taints should be configured via `startupTaints` on your `NodePool`. For example, to enable this for EBS, add the following to your `NodePool`:
```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
spec:
  template:
    spec:
      startupTaints:
        - key: ebs.csi.aws.com/agent-not-ready
          effect: NoExecute
```

### CNI is unable to allocate IPs to pods

_Note: This troubleshooting guidance is specific to the VPC CNI that is shipped by default with EKS clusters. If you are using a custom CNI, some of this guidance may not apply to your cluster._

Whenever a new pod is assigned to a node, the CNI will assign an IP address to that pod (assuming it isn't using host networking), allowing it to communicate with other pods on the cluster. It's possible for this IP allocation and assignment process to fail for a number of reasons. If this process fails, you may see an error similar to the one below.

```bash
time=2023-06-12T19:18:15Z type=Warning reason=FailedCreatePodSandBox from=kubelet message=Failed to create pod sandbox: rpc error: code = Unknown desc = failed to setup network for sandbox "0f46f3f1289eed7afab81b6945c49336ef556861fe5bb09a902a00772848b7cc": plugin type="aws-cni" name="aws-cni" failed (add): add cmd: failed to assign an IP address to container
```

#### `maxPods` is greater than the node's supported pod density

By default, the number of pods on a node is limited by both the number of networking interfaces (ENIs) that may be attached to an instance type and the number of IP addresses that can be assigned to each ENI.  See [IP addresses per network interface per instance type](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-eni.html#AvailableIpPerENI) for a more detailed information on these instance types' limits.

If the max-pods (configured through your EC2NodeClass [`kubeletConfiguration`]({{<ref "./concepts/nodeclasses/#speckubelet" >}})) is greater than the number of supported IPs for a given instance type, the CNI will fail to assign an IP to the pod and your pod will be left in a `ContainerCreating` state.

If you've enabled [Security Groups per Pod](https://aws.github.io/aws-eks-best-practices/networking/sgpp/), one of the instance's ENIs is reserved as the trunk interface and uses branch interfaces off of that trunk interface to assign different security groups.
If you do not have any `SecurityGroupPolicies` configured for your pods, they will be unable to utilize branch interfaces attached to the trunk interface, and IPs will only be available from the non-trunk ENIs.
This effectively reduces the max-pods value by the number of IPs that would have been available from the trunk ENI.
Note that Karpenter is not aware if [Security Groups per Pod](https://aws.github.io/aws-eks-best-practices/networking/sgpp/) is enabled, and will continue to compute max-pods assuming all ENIs on the instance can be utilized.

##### Solutions

To avoid this discrepancy between `maxPods` and the supported pod density of the EC2 instance based on ENIs and allocatable IPs, you can perform one of the following actions on your cluster:

1. Enable [Prefix Delegation](https://www.eksworkshop.com/docs/networking/prefix/) to increase the number of allocatable IPs for the ENIs on each instance type
2. Reduce your `maxPods` value to be under the maximum pod density for the instance types assigned to your NodePods
3. Remove the `maxPods` value from your [`kubeletConfiguration`]({{<ref "./concepts/nodeclasses#speckubeletconfiguration" >}}) if you no longer need it and instead rely on the defaulted values from Karpenter and EKS AMIs.
4. Set [RESERVED_ENIS]({{<ref "./reference/settings" >}})=1 in your Karpenter configuration to account for the reserved ENI when using Security Groups for Pods.

For more information on pod density, view the [Pod Density Section in the NodePools doc]({{<ref "./concepts/nodeclasses#pod-density" >}}).

#### IP exhaustion in a subnet

When a node is launched by Karpenter, it is assigned to a subnet within your VPC based on the [`subnetSelector`]({{<ref "./concepts/nodeclasses#specsubnetselector" >}}) value in your [`EC2NodeClass`]({{<ref "./concepts/nodeclasses" >}})). When a subnet becomes IP address constrained, EC2 may think that it can successfully launch an instance in the subnet; however, when the CNI tries to assign IPs to the pods, there are none remaining. In this case, your pod will stay in a `ContainerCreating` state until an IP address is freed in the subnet and the CNI can assign one to the pod.

##### Solutions

1. Use `topologySpreadConstraints` on `topology.kubernetes.io/zone` to spread your pods and nodes more evenly across zones
2. Increase the IP address space (CIDR) for the subnets selected by your `EC2NodeClass`
3. Use [custom networking](https://www.eksworkshop.com/docs/networking/custom-networking/) to assign separate IP address spaces to your pods and your nodes
4. [Run your EKS cluster on IPv6](https://aws.github.io/aws-eks-best-practices/networking/ipv6/) (Note: IPv6 clusters have some known limitations which should be well-understood before choosing to use one)

For more troubleshooting information on why your pod may have a `FailedCreateSandbox` error, view the [EKS CreatePodSandbox Knowledge Center Post](https://repost.aws/knowledge-center/eks-failed-create-pod-sandbox).

### Windows pods are failing with `FailedCreatedPodSandbox`

The following solution(s) may resolve your issue if you are seeing an error similar to the following when attempting to launch Windows pods. This error typically occurs if you have not enabled Windows support.

```
Failed to create pod sandbox: rpc error: code = Unknown desc = failed to setup network for sandbox "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx": plugin type="vpc-bridge" name="vpc" failed (add): failed to parse Kubernetes args: pod does not have label vpc.amazonaws.com/PrivateIPv4Address
```

#### Solutions
1. See [Enabling Windows support](https://docs.aws.amazon.com/eks/latest/userguide/windows-support.html#enable-windows-support) for instructions on how to enable Windows support.

### Windows pods fail to launch with image pull error

The following solution(s) may resolve your issue if you are seeing an error similar to the following when attempting to launch Windows pods.

```
Failed to pull image "mcr.microsoft.com/windows/servercore:xxx": rpc error: code = NotFound desc = failed to pull and unpack image "mcr.microsoft.com/windows/servercore:xxx": no match for platform in manifest: not found
```

This error typically occurs in a scenario whereby a pod with a given container OS version attempts to be scheduled on an incompatible Windows host OS version.
Windows requires the host OS version to match the container OS version.

#### Solutions

1. Define your pod's `nodeSelector` to ensure that your containers are scheduled on a compatible OS host version. To learn more, see [Windows container version compatibility](https://learn.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/version-compatibility).

### Windows pods unable to resolve DNS
Causes for DNS resolution failure may vary, but in the case where DNS resolution is working for Linux pods but not for Windows pods,
then the following solution(s) may resolve your issue.

#### Solution(s)
1. Verify that the instance role of the Windows node includes the RBAC permission group `eks:kube-proxy-windows` as shown below.
   This group is required for Windows nodes because in Windows, `kube-proxy` runs as a process on the node, and as such, the node requires the necessary RBAC cluster permissions to allow access to the resources required by `kube-proxy`.
   For more information, see https://docs.aws.amazon.com/eks/latest/userguide/windows-support.html.
```yaml
...
  username: system:node:{{EC2PrivateDNSName}}
  groups:
    - system:bootstrappers
    - system:nodes
    - eks:kube-proxy-windows # This is required for Windows DNS resolution to work
...
```

### Karpenter incorrectly computes available resources for a node

When creating nodes, the allocatable resources Karpenter computed (as seen in logs and `nodeClaim.status.allocatable`) do not always match the allocatable resources on the created node (`node.status.allocatable`) due to some amount of memory being reserved for the hypervisor and underlying OS.
Karpenter uses the results from `ec2:DescribeInstanceTypes` along with a cache for tracking observed memory capacity to determine the resources available on a node launched with a given instance type.
The following computation is used to determine allocatable CPU, memory, and ephemeral storage based on the results returned from `ec2:DescribeInstanceTypes`.

```
### cpu
nodeClaim.allocatable.cpu = instance.cpu - kubeReserved.cpu - systemReserved.cpu

### memory
# If first time launching this instance-type + AMI pair
nodeClaim.allocatable.memory = (instance.memory  * (1.0 - VM_MEMORY_OVERHEAD_PERCENT)) - kubeReserved.memory - systemReserved.memory - max(evictionSoft.memory.available, evictionHard.memory.available)
# For subsequent nodes where cached instance-type capacity is available
nodeClaim.allocatable.memory = ( cached.instance.memory - kubeReserved.memory - systemReserved.memory - max(evictionSoft.memory.available, evictionHard.memory.available)

### ephemeral-storage
nodeClaim.allocatable.ephemeralStorage = instance.storage - kubeReserved.ephemeralStorage - systemReserved.ephemeralStorage - max(evictionSoft.nodefs.available, evictionHard.nodefs.available)
```

Most of these factors directly model user configuration (i.e. the KubeletConfiguration options).
On the other hand, `VM_MEMORY_OVERHEAD_PERCENT` models an implicit reduction of available memory that varies by instance type and AMI.
However, once a node is created, the actual memory capacity on that node (node.status.capacity.memory) is checked by the controller. The controller caches the observed memory for any subsequent nodes launched with the same AMI and instance type pair, improving accuracy for future nodes.
For new combinations of AMI and instance type (i.e., when this pair is launched for the first time), Karpenter will still use the VM_MEMORY_OVERHEAD_PERCENT value as a fallback for estimating allocatable memory.
This fallback is necessary because Karpenter can't compute the exact value being modeled ahead of time, so `VM_MEMORY_OVERHEAD_PERCENT` is a [global setting]({{< ref "./reference/settings.md" >}}) used across all instance type and AMI combinations.
The default value (`7.5%`) has been tuned to closely match reality for the majority of instance types while not overestimating.
As a result, Karpenter will typically underestimate the memory available on a node for a given instance type.
If you know the real `VM_MEMORY_OVERHEAD_PERCENT` for the specific instances you're provisioning in your cluster, you can tune this value to tighten the bound.
However, this should be done with caution.
A `VM_MEMORY_OVERHEAD_PERCENT` which results in Karpenter overestimating the memory available on a node can result in Karpenter launching nodes which are too small for your workload.

To detect instances of Karpenter overestimating resource availability, the following status condition can be monitored:

```bash
$ kg nodeclaim $NODECLAIM_NAME -o jsonpath='{.status.conditions[?(@.type=="ConsistentStateFound")]}'
```

```json
{
    "type": "ConsistentStateFound",
    "status": "False",
    "reason": "ConsistencyCheckFailed",
    "message": "Consistency Check Failed",
    "lastTransitionTime": "2024-08-19T20:02:16Z"
}
```

This can be spot checked like shown above, or monitored via the following metric:

```
operator_status_condition_count{type="ConsistentStateFound",kind="NodeClaim",status="False"}
```

### Karpenter Is Unable to Satisfy Topology Spread Constraint

When scheduling pods with TopologySpreadConstraints, Karpenter will attempt to spread the pods across all eligible domains.
Eligible domains are determined based on the pod's requirements, e.g. node affinity terms.
However, pod's do not inherit the requirements of compatible NodePools.

For example, consider the following NodePool and Deployment specs:

```yaml
appVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: default
spec:
  template:
    spec:
      requirements:
      - key: topology.kubernetes.io/zone
        operator: Exists
---
appVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: np-zonal-constraint
  labels:
    project: zone-specific-project
spec:
  template:
    spec:
      requirements:
      - key: topology.kubernetes.io/zone
        operator: In
        values: ['us-east-1a', 'us-east-1b']
      # ...
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: inflate
spec:
  replicas: 3
  selector:
    matchLabels:
      app: inflate
  template:
    metadata:
      labels:
        app: inflate
    spec:
      nodeSelector:
        project: zone-specific-project
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: topology.kubernetes.io/zone
          whenUnsatisfiable: DoNotSchedule
          labelSelector:
            matchLabels:
              app: inflate
```

This cluster has subnets in three availability zones: `us-east-1a`, `us-east-1b`, and `us-east-1c`.
NodePool `default` can launch instance types in all three zones, but `np-zonal-constraint` is constrained to two.
Since Karpenter uses the pod's requirements to derive eligible domains, and the pod does not have any zonal constraints, all three availability zones are considered eligible domains.
However, the only NodePool compatible with the pod's requirements is `np-zonal-constraints`, which can only create instances in two of the three eligible domains.
Karpenter will succeed to launch the first two instances, for the first two replicas, but will fail to provision capacity for subsequent replicas since it can't provision capacity in the third domain.

In order to prevent these scenarios, you should ensure that all eligible domains for a pod can be provisioned by compatible NodePools, or constrain the pod such that it's eligble domains match those of the NodePools.
To resolve this specific issue, zonal constraints should be added to the pod spec to match the requirements of `np-zonal-constraint`:
```yaml
nodeAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
    nodeSelectorTerms:
      - matchExpressions:
          - key: topology.kubernetes.io/zone
            operator: In
            values: ['us-east-1a', 'us-east-1b']
```

### Log message of `no instance type met the scheduling requirements or had a required offering` is reported

This error suggests that there is no instance type available that meets the pod's scheduling requirements. A pod may have resource requests that necessitate a minimum instance size. If the pod is confined to a Node Pool with a specific instance family and size, it might not find an instance type that aligns with its resource needs. Additionally, resource requests from daemonsets are considered when determining if an instance type is compatible with the pod.


The phrase `had a required offering` pertains to the availability of an instance type in a specific location, such as an availability zone. This error can occur if a pod is restricted to a particular availability zone. For instance, consider a pod in a stateful set that previously had an EBS volume attached. If the subnet where the pod is scheduled changes, the pod might end up in a different availability zone than the EBS volume it needs to attach to. This mismatch in availability zones can lead to an error related to the required offering.

## Deprovisioning

### Nodes not deprovisioned

There are a few cases where requesting to deprovision a Karpenter node will fail or will never be attempted. These cases are outlined below in detail.

#### Initialization

Karpenter determines the nodes that it can begin to consider for deprovisioning by looking at the `karpenter.sh/initialized` node label. If this node label is not set on a Node, Karpenter will not consider it for any automatic deprovisioning. For more details on what may be preventing nodes from being initialized, see [Nodes not initialized]({{<ref "#nodes-not-initialized" >}}).

#### Disruption budgets

Karpenter respects Pod Disruption Budgets (PDBs) by using a backoff retry eviction strategy. Pods will never be forcibly deleted, so pods that fail to shut down will prevent a node from deprovisioning.
Kubernetes PDBs let you specify how much of a Deployment, ReplicationController, ReplicaSet, or StatefulSet must be protected from disruptions when pod eviction requests are made.

PDBs can be used to strike a balance by protecting the application's availability while still allowing a cluster administrator to manage the cluster.
Here is an example where the pods matching the label `myapp` will block node termination if evicting the pod would reduce the number of available pods below 4.

```yaml
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

You can set `minAvailable` or `maxUnavailable` as integers or as a percentage.
Review what [disruptions are](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/), and [how to configure them](https://kubernetes.io/docs/tasks/run-application/configure-pdb/).

#### `karpenter.sh/do-not-disrupt` Annotation

If a pod exists with the annotation `karpenter.sh/do-not-disrupt: true` on a node, and a request is made to delete the node, Karpenter will not drain any pods from that node or otherwise try to delete the node. Nodes that have pods with a `do-not-disrupt` annotation are not considered for consolidation, though their unused capacity is considered for the purposes of running pods from other nodes which can be consolidated.

If you want to terminate a node with a `do-not-disrupt` pod, you can simply remove the annotation and the deprovisioning process will continue.

#### Scheduling Constraints (Consolidation Only)

Consolidation will be unable to consolidate a node if, as a result of its scheduling simulation, it determines that the pods on a node cannot run on other nodes due to inter-pod affinity/anti-affinity, topology spread constraints, or some other scheduling restriction that couldn't be fulfilled.

## Node Launch/Readiness

### Node not created

In some circumstances, Karpenter controller can fail to start up a node.
For example, providing the wrong block storage device name in a custom launch template can result in a failure to start the node and an error similar to:

```bash
2022-01-19T18:22:23.366Z ERROR controller.provisioning Could not launch node, launching instances, with fleet error(s), InvalidBlockDeviceMapping: Invalid device name /dev/xvda; ...
```

You can see errors like this by viewing Karpenter controller logs:

```bash
kubectl get pods -A | grep karpenter
```

```bash
karpenter     karpenter-XXXX   2/2     Running   2          21d
```

```bash
kubectl logs karpenter-XXXX -c controller -n karpenter | less
```

### Nodes not initialized

Karpenter uses node initialization to understand when to begin using the real node capacity and allocatable details for scheduling. It also utilizes initialization to determine when it can being consolidating nodes managed by Karpenter.

Karpenter determines node initialization using three factors:

1. Node readiness
2. Expected resources are registered
3. NodePool startup taints are removed

#### Node Readiness

Karpenter checks the `Ready` condition type and expects it to be `True`.

To see troubleshooting around what might be preventing nodes from becoming ready, see [Node NotReady]({{<ref "#node-notready" >}})

#### Expected resources are registered

Karpenter pull instance type information, including all expected resources that should register to your node. It then expects all these resources to properly register to a non-zero quantity in node `.status.allocatable`.

Common resources that don't register and leave nodes in a non-initialized state:

1. `nvidia.com/gpu` (or any gpu-based resource): A GPU instance type that supports the `nvidia.com/gpu` resource is launched but the daemon/daemonset to register the resource on the node doesn't exist
2. `vpc.amazonaws.com/pod-eni`: An instance type is launched by the `ENABLE_POD_ENI` value is set to `false` in the `vpc-cni` plugin. Karpenter will expect that the `vpc.amazonaws.com/pod-eni` will be registered, but it never will.

#### NodePool startup taints are removed

Karpenter expects all startup taints specified in `.spec.template.spec.startupTaints` of the NodePool to be completely removed from node `.spec.taints` before it will consider the node initialized.

### Node NotReady

There are cases where the node starts, but fails to join the cluster and is marked "Node NotReady".
Reasons that a node can fail to join the cluster include:

- Permissions
- Security Groups
- Networking

The easiest way to start debugging is to connect to the instance and get the Kubelet logs.  For an AL2 based node:

```bash
# List the nodes managed by Karpenter
kubectl get node -l karpenter.sh/nodepool
# Extract the instance ID (replace <node-name> with a node name from the above listing)
INSTANCE_ID=$(kubectl get node <node-name> -ojson | jq -r ".spec.providerID" | cut -d \/ -f5)
# Connect to the instance
aws ssm start-session --target $INSTANCE_ID
# Check Kubelet logs
sudo journalctl -u kubelet
```

For Bottlerocket, you'll need to get access to the root filesystem:

```bash
# List the nodes managed by Karpenter
kubectl get node -l karpenter.sh/nodepool
# Extract the instance ID (replace <node-name> with a node name from the above listing)
INSTANCE_ID=$(kubectl get node <node-name> -ojson | jq -r ".spec.providerID" | cut -d \/ -f5)
# Connect to the instance
aws ssm start-session --target $INSTANCE_ID
# Enter the admin container
enter-admin-container
# Check Kubelet logs
journalctl -D /.bottlerocket/rootfs/var/log/journal -u kubelet.service
```

Here are examples of errors from Node NotReady issues that you might see from `journalctl`:

- The runtime network not being ready can reflect a problem with IAM role permissions:

  ```
  KubeletNotReady runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: cni plugin not initialized
    ```

  See [Amazon EKS node IAM role](https://docs.aws.amazon.com/eks/latest/userguide/create-node-role.html) for details. If you’re using `eksctl`, the VPC CNI pods may be given permissions through IRSA instead. Verify that this set up is working as intended. You can also look at the logs for your CNI plugin from the `aws-node` pod:

  ```bash
  kubectl get pods -n kube-system | grep aws-node
  ```

  ```
  aws-node-?????             1/1     Running   2          20d
  ```

  ```bash
  kubectl logs aws-node-????? -n kube-system
  ```

- Not being able to register the node with the Kubernetes API server indicates an error condition like the following:

  ```
  Attempting to register node" node="ip-192-168-67-130.ec2.internal"
  Unable to register node with API server" err="Unauthorized" node="ip-192-168-67-130.ec2.internal"
  Error getting node" err="node \"ip-192-168-67-130.ec2.internal\" not found
  Failed to contact API server when waiting for CSINode publishing: Unauthorized
  ```

  Check the ConfigMap to check whether or not the correct node role is there. For example:

  ```bash
  kubectl get configmaps -n kube-system aws-auth -o yaml
  ```

  ```yaml
  apiVersion: v1
  data:
  mapRoles: |
     - groups:
        - system:bootstrappers
        - system:nodes
        rolearn: arn:aws:iam::973227887653:role/eksctl-johnw-karpenter-demo-NodeInstanceRole-72CV61KQNOYS
        username: system:node:{{EC2PrivateDNSName}}
     - groups:
        - system:bootstrappers
        - system:nodes
        rolearn: arn:aws:iam::973227887653:role/KarpenterNodeRole-johnw-karpenter-demo
        username: system:node:{{EC2PrivateDNSName}}
  mapUsers: |
      []
  kind: ConfigMap
  ...
    ```

If you are not able to resolve the Node NotReady issue on your own, run the [EKS Logs Collector](https://github.com/awslabs/amazon-eks-ami/blob/master/log-collector-script/linux/README.md) (if it’s an EKS optimized AMI) and look in the following places in the log:

- Your UserData (in `/var_log/cloud-init-output.log` and `/var_log/cloud-init.log`)
- Your kubelets (`/kubelet/kubelet.log`)
- Your networking pod logs (`/var_log/aws-node`)

Reach out to the Karpenter team on [Slack](https://kubernetes.slack.com/archives/C02SFFZSA2K) or [GitHub](https://github.com/aws/karpenter/) if you are still stuck.

### Nodes stuck in pending and not running the kubelet due to outdated CNI

If you have an EC2 instance get launched that is stuck in pending and ultimately not running the kubelet, you may see a message like this in your `/var/log/user-data.log`:

> No entry for c6i.xlarge in /etc/eks/eni-max-pods.txt

This means that your CNI plugin is out of date. You can find instructions on how to update your plugin [here](https://docs.aws.amazon.com/eks/latest/userguide/managing-vpc-cni.html).

### Node terminates before ready on failed encrypted EBS volume

If you are using a custom launch template and an encrypted EBS volume, the IAM principal launching the node may not have sufficient permissions to use the KMS customer managed key (CMK) for the EC2 EBS root volume.
This issue also applies to [Block Device Mappings]({{<ref "./concepts/nodeclasses/#block-device-mappings" >}}) specified in the EC2NodeClass.
In either case, this results in the node terminating almost immediately upon creation.

Keep in mind that it is possible that EBS Encryption can be enabled without your knowledge.
EBS encryption could have been enabled by an account administrator or by default on a per region basis.
See [Encryption by default](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSEncryption.html#encryption-by-default) for details.

To correct the problem if it occurs, you can use the approach that AWS EBS uses, which avoids adding particular roles to the KMS policy. Below is an example of a policy applied to the KMS key:

```json
[
    {
        "Sid": "Allow access through EBS for all principals in the account that are authorized to use EBS",
        "Effect": "Allow",
        "Principal": {
            "AWS": "*"
        },
        "Action": [
            "kms:Encrypt",
            "kms:Decrypt",
            "kms:ReEncrypt*",
            "kms:GenerateDataKey*",
            "kms:CreateGrant",
            "kms:DescribeKey"
        ],
        "Resource": "*",
        "Condition": {
            "StringEquals": {
            "kms:ViaService": "ec2.${AWS_REGION}.amazonaws.com",
            "kms:CallerAccount": "${AWS_ACCOUNT_ID}"
            }
        }
    },
    {
        "Sid": "Allow direct access to key metadata to the account",
        "Effect": "Allow",
        "Principal": {
            "AWS": "arn:aws:iam::${AWS_ACCOUNT_ID}:root"
        },
        "Action": [
            "kms:Describe*",
            "kms:Get*",
            "kms:List*",
            "kms:RevokeGrant"
        ],
        "Resource": "*"
    }
]
```

### Node is not deleted, even though `ttlSecondsUntilExpired` is set or the node is empty

This typically occurs when the node has not been considered fully initialized for some reason.  If you look at the logs, you may see something related to an `Inflight check failed for node...` that gives more information about why the node is not considered initialized.

### Log message of `inflight check failed for node, Expected resource "vpc.amazonaws.com/pod-eni" didn't register on the node` is reported

This error indicates that the `vpc.amazonaws.com/pod-eni` resource was never reported on the node. You will need to make the corresponding change to the VPC CNI to enable [security groups for pods](https://docs.aws.amazon.com/eks/latest/userguide/security-groups-for-pods.html) which will cause the resource to be registered.

### AWS Node Termination Handler (NTH) interactions
Karpenter [doesn't currently support draining and terminating on spot rebalance recommendations]({{< ref "concepts/disruption#interruption" >}}). Users who want support for both drain and terminate on spot interruption as well as drain and termination on spot rebalance recommendations may install Node Termination Handler (NTH) on their clusters to support this behavior.

These two components do not share information between each other, meaning if you have drain and terminate functionality enabled on NTH, NTH may remove a node for a spot rebalance recommendation. Karpenter will replace the node to fulfill the pod capacity that was being fulfilled by the old node; however, Karpenter won't be aware of the reason that that node was terminated. This means that Karpenter may launch the same instance type that was just deprovisioned, causing a spot rebalance recommendation to be sent again. This can result in very short-lived instances where NTH continually removes nodes and Karpeneter re-launches the same instance type over and over again.

Karpenter doesn't recommend reacting to spot rebalance recommendations when running Karpenter with spot nodes; however, if you absolutely require this functionality, note that the above scenario is possible.
Spot instances are time limited and, therefore, interruptible. When a signal is sent by AWS, it triggers actions from NTH and Karpenter, where the former signals a shutdown and the later provisions, creating a recursive situation.
This can be mitigated by either completely removing NTH or by setting the following values:

* enableSpotInterruptionDraining: If false, do not drain nodes when the spot interruption termination notice is received. Only used in IMDS mode.
enableSpotInterruptionDraining: false

* enableRebalanceDrainin: If true, drain nodes when the rebalance recommendation notice is received. Only used in IMDS mode.
enableRebalanceDraining: false

## Pricing

### Stale pricing data on isolated subnet

The following pricing-related error occurs if you are running Karpenter in an isolated private subnet (no Internet egress via IGW or NAT gateways):

```text
ERROR   controller.aws.pricing  updating on-demand pricing, RequestError: send request failed
caused by: Post "https://api.pricing.us-east-1.amazonaws.com/": dial tcp 52.94.231.236:443: i/o timeout; RequestError: send request failed
caused by: Post "https://api.pricing.us-east-1.amazonaws.com/": dial tcp 52.94.231.236:443: i/o timeout, using existing pricing data from 2022-08-17T00:19:52Z  {"commit": "4b5f953"}
```

This network timeout occurs because there is no VPC endpoint available for the [Price List Query API.](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/using-pelong.html).
To workaround this issue, Karpenter ships updated on-demand pricing data as part of the Karpenter binary; however, this means that pricing data will only be updated on Karpenter version upgrades.
To disable pricing lookups and avoid the error messages, set the `AWS_ISOLATED_VPC` environment variable (or the `--aws-isolated-vpc` option) to true.
See [Environment Variables / CLI Flags]({{<ref "./reference/settings#environment-variables--cli-flags" >}}) for details.
