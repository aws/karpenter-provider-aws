# RFC: Unmanaged LaunchTemplate Removal

## Overview

EC2 fleet requires a launch template to be created and referenced in the CreateFleet call prior to the launch of an instance. Karpenter supports two methods for the user to reference this launch template:

1. Managed - Provisioned, maintained, and deleted by the Karpenter controllers
2. Unmanaged (currently deprecated) - Provisioned, maintained, and deleted by the user

Karpenter will shortly be promoting its api surface to v1beta1, which will result in a number of breaking changes that involve a user’s manual interaction with Karpenter’s configuration.  Because users are already having to go through a manual breaking change process with the API version promotion, it’s a natural time to look at the deprecation or full removal of fields within the current Karpenter Provisioner and AWSNodeTemplate. One of these fields is the `spec.launchTemplateName`, which allows users to reference their own, unmanaged launch templates.

Unmanaged launch template support has been present in the project since the beginning of the project, but it proves to have sharp edges for some users as well as various incompatibilities with other AWSNodeTemplate API surface. These limitations are documented in the [Limitations](#limitations) section below.

## Limitations

1. 👎 [incompatibility] AMISelection logic doesn’t work, implying that AMI drift is not supported as well
    1. [mitigation] CreateFleet supports AMI overrides so we could potentially workaround this limitation long-term
2. 👎 [sharp edge] Multi-architecture support doesn’t work, meaning that it’s the responsibility of the user to enforce architecture compatibility on the Provisioner requirements manually
    1. [mitigation] CreateFleet supports AMI overrides so we could potentially workaround this limitation long-term
3. 👎 [incompatibility] SecurityGroup selection logic doesn’t work, implying that security group drift is also not supported
4. 👎 [incompatibility] InstanceProfile isn’t known to Karpenter, meaning that the passed instance profile can’t be known by Karpenter and that the PassRole permission on the InstanceProfileRole has to be managed by the user
5. 👎👎 [sharp edge] Ephemeral Storage blockDeviceMappings are currently not discovered or known, meaning that Karpenter can’t know how much ephemeral storage exists on the instance once it launches, implying that pod scheduling may be way off: [#3579](https://github.com/aws/karpenter/issues/3579)
6. 👎👎 [incompatibility] Dynamic ephemeral storage won’t work with the current `launchTemplateName` implementation for the same reason as described in #5
7. 👎 [incompatibility] Automatic userData injection doesn’t work, meaning that the user has to manage the userData as it changes between K8s versions
8. 👎 [incompatibility] Reserved ENIs support doesn’t work to auto-reduce maxPods based on user-defined reserved ENIs for custom networking

## Options

We need to come to consensus on whether we should keep, enhance, or remove the `spec.launchTemplateName` field in the AWSNodeTemplate. Below lays out the three options, including the considerations for pursuing each one.

### Option 1: Keep `.spec.launchTemplateName` with no change

#### Considerations

1. 👎👎 The current externally-referenced `launchTemplateName` comes with inherent [Limitations](#limitations) as listed above
2. 👎 The `spec.launchTemplateName` value has already been marked as deprecated (or supported for backwards compatibility) since Karpenter version `v0.7.0 `(released on May 14, 2022) so users have been told to get off of this field
3. 👎 Drift for Launch Template details would have to be achieved through either changing the launch template name. Karpenter could potentially discover the launch template version, allowing there to be launch template version drift; however, this is not currently implemented as a feature.
4. 👍 Users who are used to launch templates or have security organizations who have already approved and vetted their launch templates don’t have additional work to do

### Option 2: Keep `.spec.launchTemplateName` and support discovery

Karpenter could continue to support `.spec.launchTemplateName` while supporting discovery of some of the details that are currently listed in the [Limitations](#limitations) section above. Discoverable details would potentially include: AMIs used with architecture, security groups, launch template version, ephemeral block device mappings. Designing the details of how this would occur is out of scope of this document; however, this option should be mentioned as it would effectively cut down on some of the limitations listed above.

#### Considerations

1. 👎👎 The design for discovery of launch template details is not currently scoped for Karpenter v1. Adding this into scope would extend existing timelines to launch v1beta1
2. 👎 Some aspects of Karpenter feature support are still not achievable, even with launch templates and discovery, such as dynamic ephemeral storage support
3. 👍 Users who are used to launch templates or have security organizations who have already approved and vetted their launch templates don’t have additional work to do

### [Recommended] Option 3: Remove `.spec.launchTemplateName`

#### Considerations

1. 👍👍 Removing `spec.launchTemplateName` allows us to add `launchTemplateName` back at a later time without making a breaking change. Keeping `launchTemplateName` in the spec causes us to have to make a breaking change to remove the field if we find we don't want to support this down the line
2. 👍 Dropping `spec.launchTemplateName` down the line would require a bump to Karpenter CRDs `v2` and a breaking change
3. 👍 Supporting `spec.launchTemplateName` as a deprecated field while not documenting it is a non-viable solution. This is because any API surface that we include in v1 should be considered supported.
4. 👎 Some users are still asking for launch template support, due to existing infrastructure and automation built around the EC2 launch templates ([#3369](https://github.com/aws/karpenter/issues/3369)). These users would have to do the migration work to migrate off of these templates to the NodeTemplate
    1. [mitigation] We could provide a tool that converts a launch template, referenced by name/id to be converted to the NodeTemplate representation of the same

