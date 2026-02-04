The goal of this document is to describe how custom user data and AMIs will be supported within Karpenter.

### Current scenario

As of v0.8.1, Karpenter supports a limited set of instance configuration within the ProvisionerSpec. The most notable omissions being the lack of support for [UserData](https://github.com/aws/karpenter/issues/885) and [AMI](https://github.com/aws/karpenter/issues/1008). The prescribed workaround is a launchTemplate override within the provisioner’s definition, which will be used as-is to create worker nodes.

While this pattern has given users the ability to fully customize their worker nodes, there have been several sharp edges.

1. If a user is looking to make a small UserData edit, say to install a specific binary for security compliance, they now need to provide a fully filled out launch template complete with bootstrapping info, security groups, instanceProfiles etc.
2. It forces users to continue to manage their infrastructure via some tooling like Terraform or CDK, and prevents them from going kube-native for everything except their EKS cluster.
3. Node properties that Karpenter should own (labels, taints, maxPods, nodeAllocatable) are now dictated by the user. This can lead to surprises such as nodes being orphaned because of missing provisioner labels, inaccurate binpacking leading to evictions and so on.

Our current API is versioned and we’re on `v1alpha5`. This gives us the flexibility to redefine the UX to better suit everyone, even if it were to be backwards incompatible with our GA experience.


Before we determine how the Provisioner API needs to change, we need to first agree on what the final user experience should be. As part of the [first draft](https://github.com/aws/karpenter/pull/1270) of the launch templates UX redefinition, we’ve already decided that we will not support partial launch templates, not express instance configuration requirements via the podSpec, and rather focus on strongly typing any required instance configuration within the provisionerSpec itself. This document now attempts to further dive into what the UserData, AMI and other EC2 property inline support will look like.

------
------


### Supporting UserData

We have 16* upvotes for [this request](https://github.com/aws/karpenter/issues/885) currently and several more reachouts on Slack. Most userData customizations generally fall into the following two buckets:

* _CASE 1_ - Installing some binaries on worker nodes. These are generally custom tooling, security daemons etc.
* _CASE 2_ - Modifying the kubelet, container-runtime configuration or some system level property like ulimits.


Once we give users the ability to override the UserData, we need to determine how to merge their UserData contents with the contents that Karpenter wants to set. Look at this section in the [appendix](#appendix) for more information on why *it’s critical* that Karpenter always has control over the UserData and therefore merging is necessary.

The merging logic for UserData for AL2 AMIs and Bottlerocket AMIs is different. This is because the former uses cloud-init that executes UserData and the latter uses TOML which works in a different way.

**For Bottlerocket AMFamily**

Bottlerocket AMIs rely on UserData being defined [as TOML](https://github.com/toml-lang/toml) which is a markup language that is a set of unique key-value pairs. Since TOML is essentially a large dictionary / hashTable, we can accept TOML and merge contents with whatever Karpenter wants to add in. For example, Karpenter will add whatever label  and taints it thinks are accurate to ensure that there is no invalid scheduling performed. Consider the following example:

A user's BR Data -
```toml
[settings.kubernetes.eviction-hard]
"memory.available" = "15%"

[settings.kubernetes.node-labels]
'karpenter.sh/provisioner-name' = 'my-prov'
'foo' = 'bar'
```

Final BR Data after Karpenter merges things in -

```toml
[settings]
[settings.kubernetes]
api-server = 'https://7EBA…'
cluster-certificate = 'LS0tLS…'
cluster-name = 'my-cluster'
settings.kubernetes.api-server = 'apiServerEndpoint'

[settings.kubernetes.eviction-hard]
"memory.available" = "15%"

[settings.kubernetes.node-labels]
'karpenter.sh/capacity-type' = 'on-demand'
'karpenter.sh/provisioner-name' = 'default'
'foo' = 'bar'
```

From the output of the merged output, you can see that Karpenter will override any fields that it considers necessary (`node-labels` based on what needs to be present as per the incoming pending pod, `cluster-certificate` etc). All other fields will be copied over as is from the user's TOML content.

* You can address `CASE2` via this mode. `CASE1` isn’t very applicable to Bottlerocket nodes.
* ManagedNodegroups performs a similar merge so there is precedence and is feasible.
    * The merge is necessary because TOML doesn’t support [duplicate keys](https://github.com/toml-lang/toml/issues/697) as part of its spec, so you can’t just concatenate two TOML files.
* To ensure that we don’t perform incorrect bin-packing, Karpenter might need to parse the UserData and extract `settings.kubernetes.system-reserved` and `settings.kubernetes.kube-reserved` which is needed to better estimate nodeAllocatable. This might mean Karpenter maintains state / a cache per Karpenter provisioner.
    * Alternatively, we can introduce a new field to our provisioner that helps hint how to binpack. This is further discussed in the following section since this limitation applies to the EKS AL2 AMIs too.


**For EKS-optimized AL2 AMIFamily**

AL2 / Linux AMIs is more popularly used. UserData is executed by [cloud-init](https://cloudinit.readthedocs.io/en/latest/) and the user data is expected to be shell scripts / cloud-init directives. Merging shell scripts can be messy, so the default pattern used by other AWS services is to leverage MIME multi-part data.

There are different options we have here -

*Option 1 - MIME parts are run in the order -  `UserManaged` → `Karpenter Managed`*

```
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="==MYBOUNDARY=="

--==MYBOUNDARY==
Content-Type: text/x-shellscript; charset="us-ascii"

#!/bin/bash
echo "Running custom user data script that was in the ProvisionerSpec"

--==MYBOUNDARY==
Content-Type: text/x-shellscript; charset="us-ascii"

#!/bin/bash
set -ex
B64_CLUSTER_CA=base64data
API_SERVER_URL=apiServerEndpoint
K8S_CLUSTER_DNS_IP=k8sClusterDnsIp
/etc/eks/bootstrap.sh clusterName
  --kubelet-extra-args '--node-labels=karpenter.sh/provisioner-name=default,karpenter.sh/capacity-type=on-demand' \
  --b64-cluster-ca $B64_CLUSTER_CA \
  --apiserver-endpoint $API_SERVER_URL \
  --dns-cluster-ip $K8S_CLUSTER_DNS_IP

--==MYBOUNDARY==--
```

* Users can address CASE1 very well through this mode. They can let Karpenter figure out how to bootstrap the worker node and focus on installing whatever they need on the worker node.
* Users cannot address CASE2 via this mode. This is because of how the bootstrap worker node is currently architected - it creates and seeds the kubeletConfiguration at runtime based on the parameters to the script.
    * If a user were to bootstrap the worker node themselves in their userdata MIME part, the worker node may join with the wrong labels that violates the bin-packing + scheduling logic Karpenter has used.
* This is currently how [Managed Nodegroups operates](https://docs.aws.amazon.com/eks/latest/userguide/launch-templates.html#launch-template-user-data).
* As part of this, we can also remove `spec.kubeletConfiguration`.

*Option 2 - MIME parts are run in the order -  `UserManaged` → `Karpenter Managed` but `spec.kubeletConfiguration` will be honored*

This is just a variation of Option 1.

* In this mode, we keep the [kubeletConfiguration field in the provisionerSpec](https://karpenter.sh/v0.8.1/provisioner/#speckubeletconfiguration) and you can set any required configuration here. When Karpenter generates UserData, it’ll make sure to include anything set in the spec.
    * This helps with the bin-packing logic since it will probably be impossible to parse the user-data as a bash script.
* Users can then address CASE2 for most cases. But this still leaves the door open for everyone to try and add more kubelet and container configuration within the provisioner which will bloat our API surface and make the provisioner more challenging to maintain.

*[Recommended] Option 3 - MIME parts are run in the order - `Karpenter Managed` → `UserManaged` → `Karpenter Managed`*

```
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="==MYBOUNDARY=="

--==MYBOUNDARY==
Content-Type: text/x-shellscript; charset="us-ascii"

#!/bin/bash
set -ex
B64_CLUSTER_CA=base64data
API_SERVER_URL=apiServerEndpoint
K8S_CLUSTER_DNS_IP=k8sClusterDnsIp
/etc/eks/bootstrap.sh clusterName
  --kubelet-extra-args '--node-labels=karpenter.sh/provisioner-name=default,karpenter.sh/capacity-type=on-demand' \
  --b64-cluster-ca $B64_CLUSTER_CA \
  --apiserver-endpoint $API_SERVER_URL \
  --dns-cluster-ip $K8S_CLUSTER_DNS_IP \
  --kubelet-disabled

--==MYBOUNDARY==
Content-Type: text/x-shellscript; charset="us-ascii"
#!/bin/bash

# Running custom user data script that was in the ProvisionerSpec
# At this point, the kubelet hasn't started so as a user I can make mutations
# to any of the kubeletConfig / containerRuntime as needed.

# To edit kubelet extra-args - /etc/systemd/system/kubelet.service.d/30-kubelet-extra-args.conf
# To edit other kubelet args - /etc/systemd/system/kubelet.service.d/
# To edit kubelet-config - /etc/kubernetes/kubelet/kubelet-config.json


--==MYBOUNDARY==
Content-Type: text/x-shellscript; charset="us-ascii"

#!/bin/bash
systemctl enable kubelet
systemctl restart kubelet

--==MYBOUNDARY==--
```

* In this mode, Karpenter passes a new parameter to the bootstrap script called `kubelet-disabled`. The bootstrap script will function normally, except the kubelet systemd service will not be started. This final hook gives Karpenter users time to make any modifications they need to make.
* Making modifications via bash in the UserData is *not easy*. Using jq _*is painful*_ because it doesn’t support in-place updates / json merges etc.
    * Updating the `.conf` files that has the kubelet args is difficult since you'd need to sed / awk your way through.
* Your options are now limitless - you have full control over the worker node's contents and can potentially make any changes you'd like. You can also see what Karpenter was planning to start the kubelet as.
* We're establishing a runtime-contract that the kubelet configurations will always be in-place at specific file locations. We can’t change this runtime contract over time for backwards compatibility.
* We can create variations of this approach. For example, we can establish a runtime contract where we tell users that they can give us a kubelet-config-custom.json in a specific directory, and do the merging for them as part of the final MIME part.
* With this option, we give users the ability to do CASE1 and CASE2 bringing parity to the AL2 and BR experience.
* There’s shared responsibility in the success of the kubelet. If users completely remove the labels Karpenter applies, or modify nodeAllocatable etc, then they're on the hook to debug failures. This is difficult to define outside of documentation.


Since we're leaking the abstraction that the bootstrap script currently provides on EKS-optimized AMIs, we can consider two paths to make the UX easier while keeping the flexibility this option provides -
1. Expose new tiny helper scripts which makes it easy for everyone to make modifications to kubelet configuration. Something like `set-container-runtime.sh`, `max-pods-calculator.sh` (this already exists), `merge-kubelet-params.sh` and so on.
2. We continue to maintain `spec.kubeletConfiguration` in the Provisioner. For use cases where Karpenter cares about the kubelet config (node allocatable, max pods etc), you can specify it directly within the kubelet config. For everything else, you can modify the configuration directly via a UserData script.

I'm leaning towards implementing *both* of the above.

------
------


### Supporting Custom AMIs

When someone passes in an AMI of their choice, it’s difficult for Karpenter to know how to bootstrap the worker node correctly. Different options we can consider are -

_[Recommended] Option 1 - If you provide a custom AMI, you must provide UserData too which will be used as-is._

This experience mirrors the experience that Karpenter users have today. Essentially this will look like -

```
spec:
  provider:
    apiVersion: extensions.karpenter.sh/v1alpha1
    kind: AWS
    securityGroupSelector:
      karpenter.sh/discovery: karp-cluster
    subnetSelector:
      karpenter.sh/discovery: karp-cluster
    ami: ami-123456
    userData: "ba123bc.." #base64 encoded
```

* The main problem with this approach is “How will Karpenter apply labels, taints on the worker nodes?”
    * We’ll continue our existing approach where in Karpenter applies them on the node object that is created by Karpenter. Any labels specified via kubelet args, will just be merged in.
* Since we also don’t control the rest of the kubeletConfig, we might need to potentially introduce a provisioner field to specify what the kubeReserved and sysReserved values are so Karpenter accurately bin packs. Karpenter isn’t concerned with any other kubelet parameter.  So something like this -
    *
    ```
      spec:
          binpacking:
            kubeReserved:
              memory:
              cpu:
            systemReserved:
              memory:
              cpu:
    or...

      spec:
        kubeletConfiguration:
          kubeReserved:
            memory:
            cpu:
   ```

* The main advantage of this approach is that we don’t need to try and determine what kind of OS that AMI has and then try and predict what kind of UserData might work. That’s extremely brittle.
* Users are already doing this today in Karpenter (and in all nodegroups options too) and while there have been some sharp edges [see this](https://github.com/aws/karpenter/issues/1380), for the most part this does work and is simplest to reason about.


_Option 2 - Let users indicate what bootstrapping logic to use by defining AMI class_

In this option, users tell Karpenter to use whatever bootstrapping logic they’d normally use for other AMIFamilies. Our APIs would evolve as -


spec:
  provider:
    apiVersion: extensions.karpenter.sh/v1alpha1
    kind: AWS
    ami: ami-123456
    amiFamily: AL2
    userData: "ba123bc.." #base64 encoded

* In such cases, Karpenter would operate the exact same way it does when the amiFamily is AL2. It will attempt to bootstrap the worker node as if it were an EKS optimized AL2 AMI.
* This is helpful for users that build their AMIs using the packer scripts we’ve got (https://github.com/awslabs/amazon-eks-ami) and only perform minimal modifications.
* This will not cover all use cases. It’s difficult for us to know the extent of customizations that users are currently doing, but we’re probably going to be excluding a set of use cases.
* We’d have to be extremely careful modifying how Karpenter bootstraps worker nodes. While our new logic might work for new EKS AMIs, we may end up breaking custom AMIs.

------
------

### Supporting other LaunchTemplate Parameters

We’ve heard of other instance parameters that users want to control, which Karpenter does not need any control over and can wire through completely.

1. [Capacity Reservations](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_LaunchTemplateCapacityReservationSpecificationRequest.html) - RIs for higher instance availabilities where you’d want to wire these values down to the EC2 Fleet request.
2. [Cpu Options](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_LaunchTemplateCpuOptionsRequest.html) - If you wanted to control the ratio of mem / CPU on every instance type that you get back. This can be useful if you want to force a certain mem / CPU ratio per instance.

We could potentially also add `ElasticInferenceAccelerators`, `ElasticGPUSpecifications` and some others if a use case arises.

------
------


### Appendix

Why will Karpenter always assume responsibility to bootstrap worker nodes?

Outside of custom launch templates, Karpenter today assumes full control over the contents of the UserData and therefore dictates the configuration of the container runtime and kubelet. When using EKS-optimized AMIs (AL2 or BR), Karpenter tends to rely on the AMI defaults for most config, while overriding a few others.

Currently, only [two kubeletConfiguration parameters](https://karpenter.sh/v0.8.0/provisioner/#speckubeletconfiguration) are configurable by users (clusterDNS and maxPods) and Karpenter dictates the remaining. The reason Karpenter needs to maintain tight control over the kubelet is because -

1. It has to ensure that the labels and taints specified in the provisionerSpec are actually applied on the worker nodes, otherwise any binding decisions it takes may lead to an immediate eviction.
2. It needs to control kube-reserved and system-reserved since those directly influence the amount of allocatable space a worker node has, which in turn affects Karpenter’s ability to bin pack accurately.

Given Karpenter’s needs to control some kubelet parameters, the natural question that occurs is - “how does launch templates with Karpenter even work today given users dictate everything?”. The reason that this integration mostly works today from a scheduling perspective, is because we create a node object with the expected labels, taints, maxPods and nodeAllocatable right after we launch the instance. When the kubelet comes alive, any labels or taints that the kubelet advertises will get merged. This pattern has led to a bunch of problems -

1. If for some reason Karpenter fails to create the node object itself, then the node may not have the provisioner’s labels and the node will be orphaned. See this as [an example](https://github.com/aws/karpenter/issues/1380).
2. Some users modified kube-reserved. This meant that the node allocatable on the worker node was different than what Karpenter calculates. As a result, the bin packing we did was inaccurate and led to evictions on node ready.

