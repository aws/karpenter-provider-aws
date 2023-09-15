# Karpenter v1beta1 Full Change List

This document formalizes the [v1beta1 laundry list](https://github.com/aws/karpenter/issues/1327) into a full change list of items that are coming in the migration from the `v1alpha5` APIs of Karpenter to `v1beta1`. This document purely describes the necessary changes and the rationale behind the changes. For the high-level overview of the API specs, view the [Karpenter v1beta1 Graduation](./v1beta1-api.md) design doc.

### Update Kind/Group Naming

As part of the bump to v1beta1, to allow the v1alpha5 APIs to exist alongside the v1beta1 APIs while users go through a migration process, the following kind names are being proposed:

 1. `Provisioner` → `NodePool`
 2. `Machine` -> `NodeClaim`
 3. `AWSNodeTemplate` → `EC2NodeClass`

We see the renames as opportunities to better align our API groups and kinds with upstream concepts as well as reducing confusion between other Kubernetes API concepts. Specifically, the word `Provisioner` (on its own) has become overloaded in Kubernetes, [particularly in the area of storage provisioning](https://kubernetes.io/docs/concepts/storage/storage-classes/#the-storageclass-resource). We want to get completely away from this naming, while also prefixing all of our kinds that apply to nodes with `Node` for better alignment and clarity across the project. 

This gives the following naming to API types within the Karpenter project

1. `karpenter.sh/NodePool`
2. `karpenter.sh/NodeClaim`
3. `karpenter.k8s.aws/EC2NodeClass`

### Remove Validation/Mutating Webhooks in favor of CEL (Common Expression Language)

The Karpenter maintainer team has seen an increase in the number of issues related to its webhooks ([#4415](https://github.com/aws/karpenter/issues/4415), [#3598](https://github.com/aws/karpenter/issues/3598), [#2902](https://github.com/aws/karpenter/issues/2902), [#4154](https://github.com/aws/karpenter/issues/4154), [#4106](https://github.com/aws/karpenter/issues/4016), [#3224](https://github.com/aws/karpenter/issues/3224), [#1729](https://github.com/aws/karpenter/issues/1729), ...) which lead us to believe that we should look for alternatives/ways to remove these webhooks from the project.

Kubernetes 1.23 introduced the `CustomResourceValidationExpressions` in alpha, followed by graduating the feature to beta in 1.25. This feature introduces the ability to write CRD validation expressions directly in the CRD OpenAPISpec without any need for validating webhooks to do custom validation. EKS supports CEL starting in Kubernetes version 1.25.

Karpenter v1beta1 will introduce CEL into the CRD OpenAPISpec while maintaining the webhooks until support for EKS versions <= 1.24 is dropped. At this point, we will drop support for the webhooks and rely solely on CEL for validation.

### Label Changes

### `karpenter.sh/do-not-evict` →  `karpenter.sh/do-not-disrupt`

Karpenter validates disruption across NodeClaims and determines which NodeClaims/Nodes it is allowed to disrupt as part of the disruption flow. While eviction is part of the termination process, it’s more accurate to say that the `karpenter.sh/do-not-evict` annotation actually prevents Karpenter’s disruption of the NodeClaim/Node rather than the eviction of it.

###  `karpenter.sh/do-not-consolidate`  → `karpenter.sh/do-not-disrupt`

Karpenter currently surfaces the `karpenter.sh/do-not-consolidate` annotation to block consolidation actions against individual nodes without having to make changes to the owning provisioner. We have found this is useful for users that have one-off scenarios for blocking consolidation, including debugging failures on nodes. 

While this feature is useful for consolidation, it should be expanded out to all disruption mechanisms, so that we have both pod-level and node-level control to block disruption using the `karpenter.sh/do-not-disrupt` annotation.

### `NodePool` Changes

####  `spec` → `spec.template`

Currently fields that control node properties, such as `Labels`, `Taints`, `StartupTaints`, `Requirements`, `KubeletConfiguration`, `ProviderRef,` are top level members of `provisioner.spec`. We can draw a nice line between:

1. Behavior-based fields that dictate how Karpenter should act on nodes
2. Configuration-based fields that dictate how NodeClaims/Nodes should look

In this case, behavior-based fields will live in the top-level of the `spec` of the `NodePool` and configuration-based fields live within the `spec.template`.

On top of this, this interface is very similar to the Deployment/StatefulSet/Job relationship, where a top-level object spawns templatized versions of lower-level objects. In our case, this top-level object is the `NodePool` and the lower-level object is the `NodeClaim` (with the `Node` joining the cluster as a side-effect of the `NodeClaim`).

```
spec:
  weight: ...
  limits: ...
  template:
    metadata:
      labels: ...
      annotations: ...
    spec:
      taints: ...
      startupTaints: ...
      requirements: ...
      providerRef: ...
  disruption:
    expireAfter: ...
    consolidateAfter: ...
    consolidationPolicy: ...
```

#### `spec.ttl...` → `spec.disruption...`

Karpenter plans to expand the amount of control that it gives users over both the aggressiveness of disruption and when disruption can take place. As part of these upcoming changes, more fields within the `NodePool` API will begin to pertain to the disruption configuration.

We can better delineate the fields that specifically pertain to this configuration from the other fields in the `spec` (global behavior-based fields, provisioning-specific fields, node static configuration fields) by moving these fields inside a `disruption` block. This will make it clearer to users which configuration options specifically pertain to scale-down when they are configuring their `NodePool` CRs.

#### `spec.ttlSecondsAfterEmpty` → `spec.disruption.consolidationPolicy`

Currently, Karpenter has two mutually exclusive ways to deprovision nodes based on emptiness: `ttlSecondsAfterEmpty` and `consolidation`. If users are using `ttlSecondsAfterEmpty`, we have generally seen that users are configuring this field in one of two ways:

1. `ttlSecondsAfterEmpty=0` → Users want to delete nodes as soon as they go empty and Karpenter sees that they are empty
2. `ttlSecondsAfterEmpty >> 0` → Users want to delete nodes that are empty but want to reduce the amount of node churn as a result of high pod churn on a larger cluster

We anticipate that both of these scenarios can be captured through the consolidation disruption mechanism; however, we understand that there are use-cases where a user may want to reduce the aggressiveness of Karpenter disruption and only disrupt empty nodes. In this case, a user can configure the `consolidationPolicy` to be `WhenEmpty` which will tell the consolidation disruption mechanism to only deprovision empty nodes through consolidation. Alternatively, you can specify a `consolidationPolicy` of `WhenUnderutilized` which will allow consolidation to deprovision both empty and underutilized nodes.

If `consolidationPolicy` is not set, Karpenter will implicitly default to `WhenUnderutilized`.

#### `spec.ttlSecondsAfterEmpty` → `spec.disruption.consolidateAfter`

While the `consolidationPolicy` offers one mechanism for users to control the aggressiveness of disruption, users that enable a `consolidationPolicy` of `WhenEmpty` or `WhenUnderutilized` may still want to dictate the speed at which nodes are deemed underutilized. This is particularly true on clusters that are large and have a large amount of pod churn. To support this, Karpenter will surface a `consolidateAfter` field which will allow users to define a per-node TTL to define the time that Karpenter can begin disrupting the node after first seeing that the node is eligible for consolidation.

#### `spec.ttlSecondsUntilExpired` → `spec.disruption.expireAfter`

Karpenter will change the `ttlSecondsUntilExipred` field to `expireAfter` to align with the `consolidateAfter` field in the `disruption` block.

#### Remove `spec.provider`

We’ve recommended that customers leverage `spec.providerRef` in favor of `spec.provider` since Q2 2022. Documentation for this feature has been removed since Q3 2022. We will take the opportunity to remove the feature entirely to minimize code bugs/complexity and user confusion.

### `EC2NodeClass` Changes

#### Update `spec.amiSelector`

The alpha API `amiSelector` has two primary limitations that restrict user’s ability to specify the AMIs that they want Karpenter to use:

1. Users can only specify “ANDed” together requirements, meaning that if a user has an orthogonal set of tags that they want to match their images to, they have to specify them by `aws::ids` directly, since there is no way with the current tag-selection logic to specify those values
2. Users want more flexibility to do things like specify a name/owner combination for images. Users have generally been asking Karpenter to more closely adhere to the EC2 APIs in our amiSelector design so that users can use more built-in filtering for AMIs, instead of having to use custom tagging to achieve the same outcome
    1. To support *some* of these use-cases, Karpenter has begun effectively creating “system-tags” i.e. (`aws::ids`, `aws::owners`, `aws::name`). These are special-cased version of the standard user custom-tags that allow users to achieve the scenarios described in #2; however, they are not easily discoverable or understood and if we are beginning to support special-cases like this, it makes sense that we should begin to structure these fields.

```
amiSelectorTerms:
- name: foo
  id: abc-123
  owner: amazon
  tags: 
    key: value
# Selector Terms are ORed
- name: foo
  id: abc-123
  owner: self
  tags: 
    key: value
```

#### Update `spec.subnetSelector`

`subnetSelectorTerms` should have a similar parity to the `amiSelectorTerms` in its design to improve the ease-of-use for users. As a result, we should design the `subnetSelectorTerms` in the same spirit as the `amiSelectorTerms` such that you can also specify multiple selectors through `tags` and `ids` that can be ORed together to produce the ultimate set of items that you want to use.

```
subnetSelectorTerms:
- id: abc-123
  tags:
    key: value
# Selector Terms are ORed
- id: abc-123
  tags:
    key: value
```

#### Update `spec.securityGroupSelector`

The same logic for `subnetSelectorTerms` applies to `securityGroupSelectorTerms`. We should have a similar parity to the `amiSelectorTerms` to improve the ease-of-use around this selector.

```
securityGroupSelectorTerms:
- id: abc-123
  tags:
    use: private-subnet
# Selector Terms are ORed
- name: custom-security-group-b # not the same as the "Name" tag
  tags:
    use: private-subnet
- tags:
    use: private-subnet
    Name: custom-security-group-c # not the same as the "name" field
```

#### Remove `spec.launchTemplate`

Direct launch template support is problematic for many reasons, outlined in the design [Unmanaged LaunchTemplate Support for Karpenter](./unmanaged-launch-template-removal.md). Customers continue to run into issues when directly using launch templates. Rather than continue to maintain these sharp edges and give users a half-baked experience of Karpenter, we should remove this field, considering that we can always add it back later if there is enough ask from users to do so.

#### `spec.instanceProfile` → `spec.role`

Currently, Karpenter uses an `instanceProfile` in the `AWSNodeTemplate` that is referenced to determine the profile that the EC2 node should launch with. Instance profiles are IAM entities that are specific to EC2 and do not have a lot of detail built around them (including console support); users are generally more familiar with the concept of IAM roles. As a result, we can support a `role` in the new `EC2NodeClass` and allow Karpenter to provision the instance profile `ad-hoc` with the `role`  specified attached to it.

#### Remove tag-based AMI Requirements

[Tag-based AMI requirements](https://karpenter.sh/docs/concepts/node-templates/#ami-selection) allowed users to tag their AMIs using EC2 tags to express “In” requirements on the images they selected on. This would allow a user to specify that a given AMI should be used *only* for a given instance type, instance size, etc. The downside of this feature is that there is no way to represent “NotIn”-based requirements in the current state, which means that there is no way to *exclude* an instance type, size, etc. from using a different AMI.

#### Example

Take the following example with AMI “a” and AMI “b”:

1. AMI "a"
    1. Tagged with `node.kubernetes.io/instance-type: c5.large`
2. AMI “b”
    1. No tags

If Karpenter were to launch a “c5.xlarge” in this example, I would be guaranteed to get AMI “b”, since AMI “a” does not satisfy the compatability requirement for the instance type; however, if Karpenter were to launch a “c5.large”, this instance type satisfies both AMI “a” and AMI “b”, meaning that which AMI it chooses could fluctuate based on the creation dates of the selected AMIs.

This functionality of Karpenter hasn’t been surfaced widely at this point in time and the **current state of the feature is effectively unusable and not well-tested**. We should remove this feature and consider adding a `requirements` key as part of the `spec.amiSelector` logic at some time in the future if users require this kind of requirement-based logic.

### `karpenter-global-settings` Changes

#### Deprecate `defaultInstanceProfile` in `karpenter-global-settings`

InstanceProfile, SubnetSelector, and SecurityGroup are all required information to launch nodes. Currently InstanceProfile is set in default settings, but subnetSelector and securityGroupSelector aren't. This is awkward and [doesn't provide a consistent experience for users](https://github.com/aws/karpenter/issues/2973). We should align all of our configuration at the `EC2NodeClass` and `Provisioner` -level for users to streamline their experience.

#### Deprecate `tags` from `karpenter-global-settings` in favor of `nodeClass.spec.tags`

Having `tags` inside of the `karpenter-global-settings` makes it difficult to detect drift when these tag values are changed. Since the primary reason this field exists inside the `karpenter-global-settings` is for ease-of-use, and there is a simple workaround for customers (setting consistent tags inside each `EC2NodeClass`), it makes natural sense to remove this from the `karpenter-global-settings` .

#### Remove `aws.enablePodENI` from `karpenter-global-settings`

This value has no meaning anymore now that our initialization logic does not rely on it. This can be pulled out of the `karpenter-global-settings` without causing impact to users.

#### Deprecate `aws.enableENILimitedPodDensity`  in `karpenter-global-settings`

Setting static pod density is available through the `nodePool.spec.kubeletConfiguration.maxPods` so there is no need for this setting to be configured at a global level anymore.
