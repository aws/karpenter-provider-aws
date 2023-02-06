# Native AMI Versioning

Karpenter uses the latest EKS Optimized AMI releases for Bottlerocket, Amazon Linux 2, and Ubuntu as the defaults for AMI Requirements. With [Drift](https://karpenter.sh/preview/concepts/deprovisioning/#drift) implemented, when a new EKS Optimized AMI is released, users who have enabled drift could see the newly released AMIs rolled out to their fleet without warning.

[EKS Optimized AMI Releases](#eks-optimized-ami-releases) are irregular and versioned differently. Without validation, [irregular release cadences](#release-cadences) could introduce unwanted AMIs in production. This doc proposes two functional improvements to AMI selection logic to ease customer pain.

## What could happen when a new EKS Optimized AMI is released?

When a new EKS Optimized AMI is released, Karpenter could begin deprovisioning and replacing nodes. If a user wants to revert back to an older release, they need to find the corresponding AMI IDs and change their AWSNodeTemplate to pin those values by setting their AWSNodeTemplate to the following:

```
spec:
    amiSelector:
        aws-ids: "ami-123,ami-456"
```

Once the user wants to return to the latest release, they’ll have to restore their configurations to the previous default as the following.

```
spec:
    amiFamily: Ubuntu # Or Bottlerocket or AL2
```

While this isn’t a difficult process, it isn't viable to require users to jump through these hoops. They must be aware of new EKS Optimized AMI releases, understand SSM aliases to fetch the AMI IDs, and modify their AWSNodeTemplate values in their CI/CD pipelines multiple times. Some users have [requested these improvements](https://github.com/aws/karpenter/issues/1495). In order to improve the experience for all users, we’ll introduce (1) Bake Time and (2) Release Version Pinning improvements.

## Bake Time

### 🔑 Introduce a minimumAgeDuration field in the amiSelector.

While this hasn’t been asked for explicitly, bake time is a common concept in phasing in new changes. Whether a user has a mandated bake time for production or developers have an estimated time to validate workloads before rolling out to prod, bake time provides a sliding window of leeway. Users can be cautious and set a high value, or leave out the setting for no bake time requirements.

```
apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
metadata:
  name: default
spec:
  amiSelector:
    minimumAgeDuration: 10d  // metav1.Duration
```

### How `minimumAgeDuration` interacts with the other methods of AMI selection

#### *EKS Optimized AMI (no other AMI Selector field)*

1. Karpenter will compare `minimumAgeDuration` to when the SSM Parameter was last modified instead of using the Image Creation Date that's returned in `aws ec2 describe-images`. This is because the SSM Parameter is updated independently of image creation.
2. The newest EKS Optimized AMI that fits the `minimumAgeDuration` selector will be chosen.

#### *Arbitrary tags*

1. An AMI tagged and discovered via the amiSelector tags will not be chosen unless it fits the `minimumAgeDuration` filter.
2. This has the downside of resulting in no AMIs found if users are too restrictive with tags and the `minimumAgeDuration` filter. The upside is that users will know any AMI created before x days ago will not be included.

#### *AMI IDs*

1. When using AMI IDs, other tags specified are not respected. To keep explicit AMI ID usage as an override, using `minimumAgeDuration` and AMI IDs will result in `minimumAgeDuration` not being respected.

### DriftTTL in the ProvisionerSpec

Karpenter could include a drift TTL in the Provisioner. Nodes would have to be drifted for the TTL's duration to be deprovisioned. This can implement bake time, but will also apply for all future methods of Drift, such as zonal and instance permission requirements. Since other Drift methods may need more immediate actions when it occurs, setting a Drift TTL may not be granular as needed.

### 🔑 Suggestion

Implement Bake Time with `minimumAgeDuration`. Users can take advantage of it with EKS Optimized AMIs generally to follow requirements on AMI bake time. This applies only to AMIs, rather than a Drift TTL which would apply broadly to every provisioning requirement. Users can rely on `minimumAgeDuration` to use it as a safeguard against unexpected AMI upgrades, creating more confidence in relying on Karpenter to make the right decisions.

## Release Pinning

*🔑 Expand amiFamily to allow a pinned version for EKS Optimized AMIs.*

Add an option to specify AMI Release Version with AMI Family. A user can pin known AMI Versions, and declaratively upgrade to a different AMI when ready. This could prevent automatic updates in the event of a newer EKS Optimized AMI. Some users [have requested this](https://github.com/aws/karpenter/issues/1495) with 13 up-votes.

### *Option 1*

Use an AMI Family with an appended magic string. This allows users who only know of version strings to modify AWSNodeTemplates easily to their liking.

```
apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
metadata:
  name: default
spec:
  amiFamily: AL2-v20220824 # No version suffix defaults to latest
```

This benefits current users of EKS Optimized AMIs. It extends cleanly into defaults, and allows users an easy override that follows the same [versioning semantics](#versioning-semantics) used for each of the AMI Families, which users would be aware of.

The downside is users that use custom Bootstrapping logic but do not use EKS Optimized AMIs may find this confusing if they find it in documentation or code. Confusion can be mitigated with good documentation as it’s only referring to EKS Optimized AMIs.

### *Option 2*

Include amiVersion as an additional filter *outside* of the amiSelector.

This may be more intuitive to users that are using EKS Optimized AMIs, but has no value add for those who aren’t. This may be confusing to users looking to extend the amiVersion field into other methods of custom AMI selection, especially since it’s a top level field. One alternative is to make this mutually exclusive with AMISelector so that there’s no room for confusion.

```
apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
metadata:
  name: default
spec:
  amiFamily: Bottlerocket
  amiVersion: v1.9.1 # Defaults to latest, requires AMIFamily if specified
```

### 🔑 Suggestion

Implement only Option 1. This keeps configuration within the amiFamily field, does not add new fields to be managed, and works well for users who are aware of EKS Optimized semantics, which is the audience for this feature. Additionally, since this doesn't add another API field, this can be easily moved to Option 2 or another option if that's a better place in the future.

## Appendix

### EKS Optimized AMI Releases

EKS Optimized AMIs are not released on a regular cadence, and do not all use the same versioning semantic.

### Versioning Semantics

AL2 (https://github.com/awslabs/amazon-eks-ami/releases) and Ubuntu (https://cloud-images.ubuntu.com/docs/aws/eks/) append their released AMIs with the date of the release (vYYYYMMDD). Bottlerocket (https://github.com/bottlerocket-os/bottlerocket/releases) uses SemVer (https://semver.org/) for their releases. SemVer has the benefit of indicating when new AMIs are adding breaking changes, but since AL2 and Ubuntu do not use SemVer, it’s tougher to introduce SemVer based versioning in the AMI Selector.

### Release Cadences

For release cadences, since mid December 2022, the most recent AL2 (https://github.com/awslabs/amazon-eks-ami/releases) releases were on 11/15, 11/08, 11/04, 10/28, and 09/29. Ubuntu (https://cloud-images.ubuntu.com/docs/aws/eks/) has recently been released at 12/08, 12/06, 11/11, 11/04, and 10/18. Bottlerocket (https://github.com/bottlerocket-os/bottlerocket/releases) has recently released v1.11.1 (11/30), v1.11.0 (11/16), v1.10.1 (10/19), v1.10.0 (10/13), and v1.9.2 (08/31).
