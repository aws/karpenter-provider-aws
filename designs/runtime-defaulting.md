# Parameter Defaulting  
(IP Family, Bare Metal, and Instance Types)

## Goals

* A consistent or understood approach to defaulting provisioner parameters
* A path forward for Karpenter IPv6 support
* A path forward for Karpenter Bare Metal support
* Agree on Instance Types overrides


**NOTE**: When you see 👀 , there’s a decision or discussion point. 

## Background

There have been several occurrences of adding a new field to the Provisioner where we have questioned whether the parameter should be defaulted at runtime (not mutated by our defaulting webhook) or if it should have a persisted default (mutated on provisioner create/update by the defaulting webhook). Based on previous parameter implementations such as `AMIFamily`,  `BlockDeviceMappings` and `MetadataOptions` in the AWS Cloud Provider, we have chosen runtime defaulting.  Outside of the AWS Cloud Provider spec, the Requirements section of the Provisioner Spec uses a mixture of runtime defaulting and persisted defaults. Both `[kubernetes.io/arch](http://kubernetes.io/arch)` and `[kubernetes.io/capacity-type](http://kubernetes.io/capacity-type)` requirements persist the defaults with amd64 and on-demand, respectively. The `node.kubernetes.io/instance-type` is runtime defaulted using an opinionated filter in the cloud provider that is currently not overridable. 

## Defaulting Examples

If a user does not provide an `AMIFamily` in their Provisioner, then Karpenter will default to using `AL2` without writing that default to the Provisioner. Likewise, if `MetadataOptions` is not specified in a Provisioner, then Karpenter will apply a set of default options that is not visible to the user in the Provisioner.  

**Actual Spec:**

```
spec:
   provider:
     apiVersion: extensions.karpenter.sh/v1alpha1
     instanceProfile: KarpenterNodeInstanceProfile-karpenter-demo
     kind: AWS
     securityGroupSelector:
       kubernetes.io/cluster/karpenter-demo: '*'
     subnetSelector:
       kubernetes.io/cluster/karpenter-demo: '*'
```

**Inferred Spec (not persisted):**

```
spec:
   provider:
 **amiFamily: AL2
     metadataOptions:
       httpEndpoint: enabled
       httpProtocolIPv6: disabled
       httpPutResponseHopLimit: 2
       httpTokens: required**
     apiVersion: extensions.karpenter.sh/v1alpha1
     instanceProfile: KarpenterNodeInstanceProfile-karpenter-demo
     kind: AWS
     securityGroupSelector:
       kubernetes.io/cluster/karpenter-demo: '*'
     subnetSelector:
       kubernetes.io/cluster/karpenter-demo: '*'
```


In a more complex example, `BlockDeviceMappings` are runtime defaulted but a different default is applied depending on which `AMIFamily` is specified. If the `AL2` `AMIFamily` is specified in the Provisioner, then one block device mapping is defaulted. If the `Bottlerocket` `AMIFamily` is specified, then two block device mappings are defaulted.  If a user would elect to override Karpenter’s runtime defaults, they can specify `BlockDeviceMappings` in the Provisioner spec. However, once `BlockDeviceMappings` are explicitly provided, Karpenter will respect the exact mappings provided even if they would result in an invalid instance configuration.   

**Actual Spec (AL2):**

```
spec:
   provider:
     amiFamily: AL2
     apiVersion: extensions.karpenter.sh/v1alpha1
     instanceProfile: KarpenterNodeInstanceProfile-karpenter-demo
     kind: AWS
     securityGroupSelector:
       kubernetes.io/cluster/karpenter-demo: '*'
     subnetSelector:
       kubernetes.io/cluster/karpenter-demo: '*'
```

**Inferred Spec (AL2):**

```
spec:
   provider:
     amiFamily: AL2
 **blockDeviceMappings:
     - deviceName: /dev/xvda
       ebs:
         volumeSize: 20G
         volumeType: gp3
         encrypted: true**
     apiVersion: extensions.karpenter.sh/v1alpha1
     instanceProfile: KarpenterNodeInstanceProfile-karpenter-demo
     kind: AWS
     securityGroupSelector:
       kubernetes.io/cluster/karpenter-demo: '*'
     subnetSelector:
       kubernetes.io/cluster/karpenter-demo: '*'
```

**Actual Spec (Bottlerocket):**

```
spec:
   provider:
     amiFamily: Bottlerocket
     apiVersion: extensions.karpenter.sh/v1alpha1
     instanceProfile: KarpenterNodeInstanceProfile-karpenter-demo
     kind: AWS
     securityGroupSelector:
       kubernetes.io/cluster/karpenter-demo: '*'
     subnetSelector:
       kubernetes.io/cluster/karpenter-demo: '*'
```

**Inferred Spec (Bottlerocket):**

```
spec:
   provider:
     amiFamily: Bottlerocket
**     blockDeviceMappings****:**
**     ****-**** deviceName****:**** ****/dev/****xvda**
**       ebs****:**
**         volumeSize****:**** 4****G**
**         volumeType****:**** gp3**
**         encrypted****:**** ****true
     - deviceName: /dev/xvdb
       ebs:
         volumeSize: 20G
         volumeType: gp3
         encrypted: true**
     apiVersion: extensions.karpenter.sh/v1alpha1
     instanceProfile: KarpenterNodeInstanceProfile-karpenter-demo
     kind: AWS
     securityGroupSelector:
       kubernetes.io/cluster/karpenter-demo: '*'
     subnetSelector:
       kubernetes.io/cluster/karpenter-demo: '*'
```

## Considerations:

There are several factors to consider when deciding on runtime defaulting and persistent defaults through defaulting webhooks.

### 1.  Compatibility Between Karpenter Versions

Since runtime defaults are reevaluated when Karpenter encounters a `Provisioner` during processing, the defaults could change between versions of the Karpenter binary. For example, today we default `AMIFamily` to `AL2` when it is not explicitly specified in the `Provisioner`. There is the possibility that a new version of Karpenter would change that default to `Bottlerocket`. A user may be surprised that the same `Provisioner` starts launching `Bottlerocket` nodes after the upgrade. 

If persistent defaulting was used instead, Karpenter’s defaulting webhook would apply the default for the version of Karpenter that was currently running when the Provisioner was created. In this case, if we changed the default `AMIFamily` to `Bottlerocket` in a new version of Karpenter, any provisioners created before the upgrade would retain the `AL2` `AMIFamily` since it would not run the defaulting webhook because a value is already present for the field.

👀  If we decide to change a runtime defaulted parameter, it may make sense to increment the `Provisioner` CRD version and migrate the `Provisioner` with a persisted default value that matches the previous runtime default.

### 2. Provisioner Verbosity 

Runtime defaulting has the advantage that it keeps the `Provisioner` spec fairly clean looking. This is an important consideration for Karpenter since one of the value propositions with Karpenter is its ease of use. For users that don’t need to modify `MetadataOptions` or `BlockDeviceMappings` (or are not even aware of what those concepts are), the Provisioner spec is simple, readable, and just works ™️.   

On the other hand, if you are a user that needs to edit those parameters, it is difficult to figure out what the runtime defaults are since they are not written to the `Provisioner` spec. For example, if a user onboards to Karpenter with a simple use-case that does not require customizing a `Provisioner` parameter like `BlockDeviceMappings`, but then wants to onboard another ML workload that requires adjusting the `BlockDeviceMappings`, the modification is more difficult since you would need to understand how to configure `BlockDeviceMappings` for the specific AMI that you are using. If the defaults were persisted to the `Provisioner`, you could simply adjust the parameters you needed such as `volumeSize` and not need to worry about the `volumeType`. 

👀  A potential solution to making it easier to find the runtime defaults Karpenter is using is to show the full inferred parameters in the `Provisioner` status field. The other obvious solution is to ensure our documentation is up-to-date.   

There are limitations to how useful persisting defaults would actually be for users. If a user is bringing their own AMI, then they would need to understand how to configure the `BlockDeviceMappings` or any other AMI specific parameter since our persisted defaults would be specific to the AMI Families we offer. Also, since we are planning to expand the number of Launch Template parameters Karpenter supports natively, persistent defaulting could have serious implications on the readability of the `Provisioner` spec. 

## IP Family 

There is currently a [PR](https://github.com/aws/karpenter/pull/1232) out to support IPv6 within Karpenter. Rather than adding a parameter to the Provisioner that would expose `IPFamily`, the PR attempts to auto-discover the IP Family by looking up the kube-dns service IP and checking if it is an IPv4 or IPv6 address. For EKS and kOps, this seems to work well since both k8s providers do not support dual stack clusters. The EKS bootstrap.sh script and the Bottlerocket configuration file only accept 1 cluster DNS IP that would definitively make the node an IPv4 node or an IPv6 node.  

👀  **Options**:

1. We can proceed with a completely auto-discovered approach where there is no `IPFamily` parameter in the Provisioner.
2. An `IPFamily` field could be added to the `Provisioner` spec and we could auto-discover for runtime defaulting purposes. Users could then override the parameter if they wanted to force a certain IP configuration.
3. An `IPFamily` field could be added to the Provisioner that we default to a static value and force users to override for the IPv6 case. 

**Recommendation:** 
I would lean towards option 2 where we do the right thing the vast majority of the time (auto-discover), but if for some reason we are not accounting for a specific setup and discover incorrectly, the user is empowered to manually override it. 

### Related Parameter Defaulting 

If a cluster is discovered to be IPv6 by the mechanism mentioned previously, then some of Karpenter’s runtime defaulted parameters would not produce a valid node configuration. One parameter is the default instance types list. Only Nitro instance types (>= Gen 5) support IPv6 prefix delegation which limits the provisioner’s default instance types to bin-pack onto. Another parameter that would need to be adjusted is the `MetadataOptions`. Karpenter runtime defaults `MetadataOptions` to disable the IPv6 EC2 Instance Metadata Service (IMDS) because the IPv6 endpoint is only available on Nitro instance types and causes a CreateFleet API error if enabled for non-nitro instance types. 

👀  **Options**:

1. We do nothing and expect the user to provide a compatible list of instance types with proper `MetadataOptions` when using IPv6. We could always help in docs and recommend tooling like the ec2-instance-selector for proper node selection.
2. We dynamically adjust the runtime defaults based on the other parameters. In the IPv6 case, we would default `MetadataOptions` to enable the IPv6 endpoint and we would filter the instance types to only use Nitro. The options would be overridden if specified directly in the Provisioner spec.

**Recommendation:** 
I would lean towards option 2 where runtime defaults are adjusted based on other parameters to present a “just works™️” experience. The thinking behind this recommendation is that users that are not setting parameters like `MetadataOptions` and `InstanceTypes` do not want to deal with the nitty gritty details of configuring dependent systems to fulfill their goal of an IPv6 cluster. If they have already conveyed opinions on these dependent parameters, then they assume full responsibility over them (i.e. setting instance type requirements).

## Instance Types - Bare Metal

Another runtime defaulted parameter is the Instance Types Requirements field. Karpenter applies an opinionated filter (mostly M*, C*, R*, A*, T>2, and I* but not metal) over the universe of all instance types. Bare Metal instance types allow users to run their own hypervisors which non-metal instance types do not support. There is no difference between hardware specs and cost for bare-metal instance types compared to the largest instance type of an instance family (i.e. m5.24xlarge == m5.metal). However, bare-metal instance types boot significantly slower. 

When Karpenter provisions capacity with EC2 Fleet, it uses a “lowest-price” allocation strategy for on-demand instances and “capacity-optimized-prioritized” for spot instances. Since metal instance types are the same price as the largest non-metal instance type in the family, Fleet does not prefer the non-metal and may randomly choose which one to provision. In the “capacity-optimized-prioritized” case, Fleet could choose either one depending on how much EC2 capacity is available for each. 

Users have expressed interest in using Firecracker VMs within k8s to strengthen the pod security boundary for multi-tenant PaaS use-cases. In order for Karpenter to support bare metal, we would need to allow instance types outside of our default filter. Currently, even if a user of Karpenter specified a metal instance type in the `Requirements` for instance-types, we will not use it. 

👀  Requirements operate differently than the other parameters discussed in this doc since operators like `In` and `NotIn` are supported. Since we runtime default Instance Types to the list provided by the Cloud Provider, a user would expect that adding a `NotIn` to exclude an instance type from the defaulted list would not add additional instance types (i.e. override the opinionated filter). On the other hand, if an `In` is specified, we should give control over the universe of instance types available even if it expands the defaulted, opinionated filter. 

👀  **Options****:**


1. An Explicit Parameter on the Provisioner or Pod:
    Karpenter could support a parameter that signals to provisioning that bare-metal instance types are required and would adjust the runtime default to use only bare metal instance types when packing. The parameter would need to be Karpenter specific similar to our `karpenter.sh/capacity-type` for `spot` and `on-demand`, since Kubernetes does not have a well-known label denoting a bare metal requirement.
    
2. Explicitly List Bare Metal Instance Types Requirements:
    Karpenter could require a user to explicitly list the bare metal instance types in the provisioner’s requirements to override the runtime default that excludes bare metal instance types. This option is more in-line with how the other runtime defaulting parameters work today and discussed in previous sections of this document. 

**Recommendation:**
I would lean towards option 2 and force the user to override the instance types in order to use bare metal. I think this use-case is special enough to warrant more work from the user to configure. There are other steps a user would need to take like labeling the nodes with a bare metal label and using a node selector on their pods in addition to setting up containerd-firecracker or some other special software to setup the hypervisor on the Karpenter provisioned nodes.


## Discussion Notes and Action Items

* Show runtime defaults in Status field of the Provisioner
* Proceed w/ IPv6 auto-discovery w/o a parameter in the Provisioner (CP) 
    * This is a 2-way door, we can always add the field later
* Bare Metal support by de-prioritizing in the CP (similar to GPU handling)
* Allow for expansion of Instance Types via “IN” Requirements
* Follow-up discussion on parameter default changes 

