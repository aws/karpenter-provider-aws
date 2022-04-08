## Karpenter and AWS LaunchTemplates


This document focuses on how to evolve Karpenter’s support for AWS LaunchTemplates.

### **What is the problem?**

Karpenter currently lets customers control several properties of the worker node that are being provisioned via the Provisioner. While this includes most core, required parameters like the instanceType, subnets, securityGroups, not all EC2 level parameters are supported.

If additional configuration is required (say to use a specific AMI), we ask that a complete LaunchTemplate is provided to the Karpenter provisioner. This route has had a few advantages -

1. Users can fully customize their worker node and use any available EC2 feature that’s exposed via launch templates.
2. Partially filled out launch templates are difficult to interpret. Anyone specifying a custom AMI to use will also need to explicitly specify how it needs to be bootstrapped via UserData. This is important because Karpenter cannot make guesses on how to start the kubelet on an unknown AMI.

The main, critical downside of this approach has been that a user needs to maintain a fully-formed launch template even when they’re only looking to customize a single property of their worker nodes. Consider these examples -

1. In order to override [the root volume](https://github.com/aws/karpenter/issues/885), you’d need to author an entire launch template containing security groups, userData etc.
2. In order to just have all statically provisioned [volumes be encrypted](https://github.com/aws/karpenter/issues/933), you need to author an entire launch template.

While some users do like complete control over the launch template since they customize everything (kOps), most only need to override a few settings and would therefore like those to be present as knobs on the provisioner itself, and for all other settings to be figured out by Karpenter.


### **Which parameters should we support in the providerSpec?**

The reason this question is important is because we don't want to bloat the AWS providerSpec by adding every EC2 property in there. Having said that, we’ve already made some changes to the v1alpha5 Provisioner API to support scenarios that could’ve been addressed by a custom launch template. This includes [overriding instanceProfiles](https://github.com/aws/karpenter/pull/914), flexibility to [change maxPods](https://github.com/aws/karpenter/pull/1032), configure [IMDS settings](https://github.com/aws/karpenter/commit/5bb3c3ab4ec840de15f05090761bc5f0733bda46), specify [clusterDNS](https://github.com/aws/karpenter/pull/1013), supporting [BlockDeviceMappings](https://github.com/aws/karpenter/pull/1420) and more.

The rationale we’ve stuck to so far is

1. We only support basic use cases that should apply to most users. Categorizing a use case as basic has been somewhat arbitrary, but we’ve used upvotes on open issues and PRs to get that signal.
2. At the minimum, functionality that other AWS provisioners like ManagedNodegroups give you without having to use a launch template should be available through Karpenter as well. Most of the parameters we’ve recently added fit this tenet.

### How do we evolve LT usage?

We’ve considered a few different approaches for this so far.

**Approach 1 - Continue as is. Inline fields as necessary and enforce complete launch templates otherwise.**

With this approach, we’ll continue to add overrides in our providerSpec based on user appetite, accepting PRs that solve a basic / common use case and asking all others to specify an entire launch template.


*Pros*
* Fastest path forward - we let users define which parameters are most important to them and let them unblock themselves with a PR if it should be beneficial to others.
* Consider this as an example - EC2 has only added new fields twice in the last two years. Once for nitro enclaves, and the other for network index cards. Both of these aren't what we call a common use case, and even so it's not a lot to maintain. Network cards don't even apply to Karpenter (we tell customers to let the CNI configure network interfaces).
  * Once we overcome the initial hump of missing fields, we shouldn't frequently encounter new fields that are needed for a base case.

*Cons*
* The Provider API starts to have too many configuration knobs and becomes difficult to document and use.
* Difficult messaging for users - how do we define what is and isn't supported on the provisioner? Forcing a user to define an entire launch template as an alternative is a lot of friction.
* Validating the provisionerSpec will become complex. Consider these examples -
  * If a user specifies `provider.ImageId`, they'd necessarily also need to provide `provider.UserData` because Karpenter cannot guess how to bootstrap a worker node using an unknown AMI.
  * If a user specifies both `provisionerSpec.kubeletConfiguration.clusterDNS` and `provider.UserData`, we'd have to fail the provisioner update because we can't honor the former unless we have complete control over the latter.


**Approach 2 - Accept partial launch templates and fill in the rest.**

This approach is inspired by Managed Nodegroups  - we let a customer only specify a certain set of properties in their launch template and let Karpenter automatically figure out the rest. That way a customer needing to only update the sizes of their root volumes needs to specify a much smaller and easier to maintain launch template like -

```
aws ec2 describe-launch-template-versions --launch-template-id lt-123
{
    "LaunchTemplateVersions": [
        {
            "LaunchTemplateId": "lt-123",
            "LaunchTemplateName": "karpenter-lt-example",
            "VersionNumber": 1,
            "CreateTime": "2022-02-02T23:41:05+00:00",
            "CreatedBy": "arn:aws:sts::123:assumed-role/me",
            "DefaultVersion": true,
            "LaunchTemplateData": {
                "BlockDeviceMappings": [
                    {
                        "DeviceName": "/dev/xvdb",
                        "Ebs": {
                            "Iops": 3000,
                            "VolumeSize": 30,
                            "VolumeType": "gp3"
                        }
                    }
                ]
            }
        }
    ]
}
```

If a field that Karpenter normally specifies in the launch template is already filled in by the customer, we ignore it. If it hasn’t been filled in, Karpenter determines what value to use as normal (looking at defaults / providerSpec etc).

We can potentially expose a parameter like autofill which dictates whether Karpenter should hydrate the remaining fields of your launch template or use your launch template as is. That way we maintain backwards compatibility and customers don’t get surprised if Karpenter suddenly starts filling in a field that it would previously never touch.
```
kind: Provisioner
spec:
  provider:
    launchTemplate: CustomKarpenterLaunchTemplateDemo
     autofill: true
```

*Pros*
* A smaller launch template is much easier to maintain for the user.
* Our API shape remains clean and easier to use. This paradigm is well known across other AWS capacity providers like MNG, Batch etc.
* If someone wants to customize the entire LaunchTemplate, they can continue to do so and specify `autofill: false`

*Cons*
* Our validation logic will start to look very messy due to odd scenarios.
Example - if someone gives us a partial launch template with a custom AMI, the UserData cannot be autofilled by Karpenter.
Our API shape remains clean and easier to use. This paradigm is well known across other AWS capacity providers like MNG, Batch etc.
* The user still needs to maintain a launch template. For some, the pain of maintaining a launch template is invariant to how many fields they have to fill out.
* Karpenter will need to periodically reconcile the provided LaunchTemplate with the Karpenter managed version, if the provided launch template is specified using a version alias like `$LATEST` rather than a fixed number. This can lead to eventual consistency issues - i.e a short time lag between Karpenter picking up that the `$LATEST` version of the provided LT has changed and making the corresponding updates to the Karpenter managed LT.


**Approach 3 - Provide a more generic way to specify launch template contents in the Provider**

The Provider today is strongly typed and we only support a few fields. We can introduce a new field like launchTemplateOverride where we accept bytes and attempt to do deserialize it into a launch template field at runtime.

Effectively, a customer writes the provider as such -

```
kind: Provisioner
spec:
  provider:
    launchTemplateOverride:
        BlockDeviceMappings:
        - DeviceName: "/dev/xvdb"
        Ebs:
            Iops: 3000
            VolumeSize: 30
            VolumeType: gp3
```

and everything under launchTemplateOverride we try and deserialize [to the LaunchTemplateData](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RequestLaunchTemplateData.html) object using some form of reflection.


*Pros*
* You get the full ability of EC2 launch template right within your Provider
* You don't need to maintain a launch template at all. Any new EC2 feature will work out of the box as long as the deserialization logic uses the latest AWS SDK i.e keep the controller up to date.
* It most directly addresses what every customer is asking - let them configure EC2 properties from within the provisioner.

*Cons*
* APIs should really never accept weakly typed input. It's going to be very easy for a customer to get something wrong what went wrong break the deserialization. When that happens, it might be equally difficult to debug YAML.
* If customers have to fill out the whole launch templates, it's going to make the provisionerSpec verbose. If they only fill it out partially, then you have the same cons as approach 2.


**Approach 4 - Let all infrastructure configuration be expressed as a pod requirement**

The essence of this approach, is to represent each EC2 instance level feature as a well known label that the Karpenter provisioner recognizes. This is what we currently do with instanceTypes with the `node.kubernetes.io/instance-type` label. Using this approach, the provisioner can read off the podSpec to determine which ImageId it needs to use, which additional volumes need to be statically provisioned and so on.

*Pros*
* Karpenter supports a kube-native way to define node level configuration that is agnostic of the cloud provider.
* Since each application is unique and may have unique infrastructure requirements, users will be otherwise forced to define multiple provisioners where each provisioner uses a distinct launch template. By moving the configuration to the pod level, we can continue to only necessitate a single / fewer provisioners.


*Cons*
* Labels aren't expressive enough for free-form fields like UserData or even complex fields like say volumes that have multiple nested attributes.
* Karpenter uses layered constraints - is it possible to define a bound for UserData?
  * Along the same lines, should the application developer always have complete control over their infrastructure? Some configuration like ImageIds might need to be enforced by the cluster administrator, so it would be beneficial to never expose ImageIds as a knob to the app developer at all.
* By moving instance configuration from the provider label to the provisioner label, each separate cloud provider implementation will need to support that label which introduces unneeded complexity. The feature set across all cloud providers is unlikely to ever be homogenous.


### Recommendation

I recommend we continue with Approach 1, and give everyone the ability to customize a few select, commonly used fields in the provisionerSpec and ask them to configure all others via a complete LaunchTemplate. I think this keeps our API easy to use. The fields I expect us to add to the Provider in the short term are _ImageId and UserData._ The use cases for other fields either don’t apply to Karpenter, or they’re quite niche where it probably makes sense for the customer to manage all other aspects of the launch template in any case.

* For example, I don’t expect Karpenter customers to need [any of these launch template options](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RequestLaunchTemplateData.html) - CapacityReservationSpecification, CpuOptions, CreditSpecification, DisableApiTermination, ElasticGpuSpecifications, HibernationOptions, InstanceInitiatedShutdownBehavior, InstanceMarketOptions, InstanceRequirements, InstanceType, NetworkInterfaces, PrivateDnsNameOptions so we shouldn’t get too concerned of ever increasing knobs in our Provider API.