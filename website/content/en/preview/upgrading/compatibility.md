---
title: "Compatibility"
linkTitle: "Compatibility"
weight: 20
description: >
  Compatibility issues for Karpenter
---

# Compatibility

To make upgrading easier we aim to minimize the introduction of breaking changes.
Before you begin upgrading Karpenter, consider Karpenter compatibility issues related to Kubernetes and the NodePool API (previously Provisioner).

## Compatibility Matrix

[comment]: <> (the content below is generated from hack/docs/compatibilitymatrix/main.go)

| KUBERNETES |   1.24   |   1.25   |   1.26   |   1.27   |   1.28   |   1.29   |  1.30  |
|------------|----------|----------|----------|----------|----------|----------|--------|
| karpenter  | \>= 0.21 | \>= 0.25 | \>= 0.28 | \>= 0.28 | \>= 0.31 | \>= 0.34 | 0.37.6 |

[comment]: <> (end docs generated content from hack/docs/compatibilitymatrix/main.go)

{{% alert title="Note" color="warning" %}}
Karpenter currently does not support the following [new `topologySpreadConstraints` keys](https://kubernetes.io/blog/2023/04/17/fine-grained-pod-topology-spread-features-beta/), promoted to beta in Kubernetes 1.27:
- `matchLabelKeys`
- `nodeAffinityPolicy`
- `nodeTaintsPolicy`

For more information on Karpenter's support for these keys, view [this tracking issue](https://github.com/aws/karpenter-core/issues/430).
{{% /alert %}}

{{% alert title="Note" color="warning" %}}
Karpenter supports using [Kubernetes Common Expression Language](https://kubernetes.io/docs/reference/using-api/cel/) for validating its Custom Resource Definitions out-of-the-box; however, this feature is not supported on versions of Kubernetes < 1.25. If you are running an earlier version of Kubernetes, you will need to use the Karpenter admission webhooks for validation instead. You can enable these webhooks with `--set webhook.enabled=true` when applying the Karpenter Helm chart.
{{% /alert %}}

## Compatibility issues

When we introduce a breaking change, we do so only as described in this document.

Karpenter follows [Semantic Versioning 2.0.0](https://semver.org/) in its stable release versions, while in
major version zero (`0.y.z`) [anything may change at any time](https://semver.org/#spec-item-4).
However, to further protect users during this phase we will only introduce breaking changes in minor releases (releases that increment y in x.y.z).
Note this does not mean every minor upgrade has a breaking change as we will also increment the
minor version when we release a new feature.

Users should therefore check to see if there is a breaking change every time they are upgrading to a new minor version.

### How Do We Break Incompatibility?

When there is a breaking change we will:

* Increment the minor version when in major version 0
* Add a permanent separate section named `upgrading to x.y.z+` under [release upgrade notes](#release-upgrade-notes)
  clearly explaining the breaking change and what needs to be done on the user side to ensure a safe upgrade
* Add the sentence “This is a breaking change, please refer to the above link for upgrade instructions” to the top of the release notes and in all our announcements

### How Do We Find Incompatibilities?

Besides the peer review process for all changes to the code base we also do the followings in order to find
incompatibilities:
* (To be implemented) To check the compatibility of the application, we will automate tests for installing, uninstalling, upgrading from an older version, and downgrading to an older version
* (To be implemented) To check the compatibility of the documentation with the application, we will turn the commands in our documentation into scripts that we can automatically run

### Security Patches

While we are in major version 0 we will not release security patches to older versions.
Rather we provide the patches in the latest versions.
When at major version 1 we will have an EOL (end of life) policy where we provide security patches
for a subset of older versions and deprecate the others.

## Release Types

Karpenter offers three types of releases. This section explains the purpose of each release type and how the images for each release type are tagged in our [public image repository](https://gallery.ecr.aws/karpenter).

### Stable Releases

Stable releases are the most reliable releases that are released with weekly cadence. Stable releases are our only recommended versions for production environments.
Sometimes we skip a stable release because we find instability or problems that need to be fixed before having a stable release.
Stable releases are tagged with a semantic version prefixed by a `v`. For example `v0.13.0`.

### Release Candidates

We consider having release candidates for major and important minor versions. Our release candidates are tagged like `vx.y.z-rc.0`, `vx.y.z-rc.1`. The release candidate will then graduate to `vx.y.z` as a normal stable release.
By adopting this practice we allow our users who are early adopters to test out new releases before they are available to the wider community, thereby providing us with early feedback resulting in more stable releases.

### Snapshot Releases

We release a snapshot release for every commit that gets merged into [`aws/karpenter-provider-aws`](https://www.github.com/aws/karpenter-provider-aws). This enables users to immediately try a new feature or fix right after it's merged rather than waiting days or weeks for release.

Snapshot releases are not made available in the same public ECR repository as other release types, they are instead published to a separate private ECR repository.
Helm charts are published to `oci://{{< param "snapshot_repo.account_id" >}}.dkr.ecr.{{< param "snapshot_repo.region" >}}.amazonaws.com/karpenter/snapshot/karpenter` and are tagged with the git commit hash prefixed by the Karpenter major version (e.g. `0-fc17bfc89ebb30a3b102a86012b3e3992ec08adf`).
Anyone with an AWS account can pull from this repository, but must first authenticate:

```bash
aws ecr get-login-password --region {{< param "snapshot_repo.region" >}} | docker login --username AWS --password-stdin {{< param "snapshot_repo.account_id" >}}.dkr.ecr.{{< param "snapshot_repo.region" >}}.amazonaws.com
```

{{% alert title="Note" color="warning" %}}
Snapshot releases are suitable for testing, and troubleshooting but they should not be used in production environments. Snapshot releases are ephemeral and will be removed 90 days after they were published.
{{% /alert %}}
