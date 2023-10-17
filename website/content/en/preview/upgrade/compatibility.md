---
title: "Compatibility"
linkTitle: "Karpenter Compatibility"
weight: 30
description: >
  Compatibility issues for Karpenter
---

# Compatibility 

To make upgrading easier we aim to minimize the introduction of breaking changes.
Before you begin upgrading Karpenter, consider Karpenter compatibility issues related to Kubernetes, the NodePool API (previously Provisioner), and Kubernetes Custom Resource Definitions (CRDs) applied through Helm Charts.

## Compatibility Matrix 

[comment]: <> (the content below is generated from hack/docs/compatibilitymetrix_gen_docs.go)

| KUBERNETES |  1.23   |  1.24   |  1.25   |  1.26   |  1.27   |  1.28   |
|------------|---------|---------|---------|---------|---------|---------|
| karpenter  | 0.21.x+ | 0.21.x+ | 0.25.x+ | 0.28.x+ | 0.28.x+ | 0.28.x+ |

[comment]: <> (end docs generated content from hack/docs/compatibilitymetrix_gen_docs.go)

{{% alert title="Note" color="warning" %}}
Karpenter currently does not support the following [new `topologySpreadConstraints` keys](https://kubernetes.io/blog/2023/04/17/fine-grained-pod-topology-spread-features-beta/), promoted to beta in Kubernetes 1.27:
- `matchLabelKeys`
- `nodeAffinityPolicy`
- `nodeTaintsPolicy`

For more information on Karpenter's support for these keys, view [this tracking issue](https://github.com/aws/karpenter-core/issues/430).
{{% /alert %}}

## Compatibility issues

When we introduce a breaking change, we do so only as described in this document.

Karpenter follows [Semantic Versioning 2.0.0](https://semver.org/) in its stable release versions, while in
major version zero (v0.y.z) [anything may change at any time](https://semver.org/#spec-item-4).
However, to further protect users during this phase we will only introduce breaking changes in minor releases (releases that increment y in x.y.z).
Note this does not mean every minor upgrade has a breaking change as we will also increment the
minor version when we release a new feature.

Users should therefore check to see if there is a breaking change every time they are upgrading to a new minor version.

### Custom Resource Definition (CRD) Upgrades

Karpenter ships with a few Custom Resource Definitions (CRDs). These CRDs are published:
* As an independent helm chart [karpenter-crd](https://gallery.ecr.aws/karpenter/karpenter-crd) - [source](https://github.com/aws/karpenter/blob/main/charts/karpenter-crd) that can be used by Helm to manage the lifecycle of these CRDs.
  * To upgrade or install `karpenter-crd` run:
    ```
    helm upgrade --install karpenter-crd oci://public.ecr.aws/karpenter/karpenter-crd --version vx.y.z --namespace karpenter --create-namespace
    ```

{{% alert title="Note" color="warning" %}}
< If you get the error `invalid ownership metadata; label validation error:` while installing the `karpenter-crd` chart from an older version of Karpenter, follow the [Troubleshooting Guide]({{<ref "../troubleshooting#helm-error-when-installing-the-karpenter-crd-chart" >}}) for details on how to resolve these errors.
{{% /alert %}}

* As part of the helm chart [karpenter](https://gallery.ecr.aws/karpenter/karpenter) - [source](https://github.com/aws/karpenter/blob/main/charts/karpenter/crds). Helm [does not manage the lifecycle of CRDs using this method](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/), the tool will only install the CRD during the first installation of the helm chart. Subsequent chart upgrades will not add or remove CRDs, even if the CRDs have changed. When CRDs are changed, we will make a note in the version's upgrade guide.

In general, you can reapply the CRDs in the `crds` directory of the Karpenter helm chart:

```shell
kubectl apply -f https://raw.githubusercontent.com/aws/karpenter{{< githubRelRef >}}pkg/apis/crds/karpenter.sh_nodepools.yaml
kubectl apply -f https://raw.githubusercontent.com/aws/karpenter{{< githubRelRef >}}pkg/apis/crds/karpenter.sh_nodeclaims.yaml
kubectl apply -f https://raw.githubusercontent.com/aws/karpenter{{< githubRelRef >}}pkg/apis/crds/karpenter.k8s.aws_ec2nodeclasses.yaml
```

### How Do We Break Incompatibility?

When there is a breaking change we will:

* Increment the minor version when in major version 0
* Add a permanent separate section named `upgrading to vx.y.z+` under [release upgrade notes](#release-upgrade-notes)
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
Stable releases are tagged with Semantic Versioning. For example `v0.13.0`.

### Release Candidates

We consider having release candidates for major and important minor versions. Our release candidates are tagged like `vx.y.z-rc.0`, `vx.y.z-rc.1`. The release candidate will then graduate to `vx.y.z` as a normal stable release.
By adopting this practice we allow our users who are early adopters to test out new releases before they are available to the wider community, thereby providing us with early feedback resulting in more stable releases.

### Snapshot Releases

We release a snapshot release for every commit that gets merged into the main repository. This enables our users to immediately try a new feature or fix right after it's merged rather than waiting days or weeks for release.
Snapshot releases are suitable for testing, and troubleshooting but users should exercise great care if they decide to use them in production environments.
Snapshot releases are tagged with the git commit hash prefixed by the Karpenter major version. For example `v0-fc17bfc89ebb30a3b102a86012b3e3992ec08adf`. For more detailed examples on how to use snapshot releases look under "Usage" in [Karpenter Helm Chart](https://gallery.ecr.aws/karpenter/karpenter).
