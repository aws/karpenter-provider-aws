---
title: "Upgrade Guide"
linkTitle: "Upgrade Guide"
weight: 10
---

Karpenter is a controller that runs in your cluster, but it is not tied to a specific Kubernetes version, as the Cluster Autoscaler is.
Use your existing upgrade mechanisms to upgrade your core add-ons in Kubernetes and keep Karpenter up to date on bug fixes and new features.

This document describes issues related to upgrading Karpenter, along with steps you can follow to perform an upgrade.

# Compatibility issues

To make upgrading easier, we aim to minimize the introduction of breaking changes with the followings components:

* Provisioner API
* Helm Chart

We try to maintain compatibility with:

* The application itself
* The documentation of the application

When we introduce a breaking change, we do so only as described in this document.

Karpenter follows [Semantic Versioning 2.0.0](https://semver.org/) in its release versions, while in
major version zero (v0.y.z) [anything may change at any time](https://semver.org/#spec-item-4).
However, to further protect customers during this phase we will only introduce breaking changes in minor releases (releases that increment y in x.y.z).
Note this does not mean every minor upgrade has a breaking change as we will also increment the
minor version when we release a new feature.

Users should therefore check to see if there is a breaking change every time they are upgrading to a new minor version.

## How Do We Break Incompatibility?

When there is a breaking change we will:

* Increment the minor version when in major version 0
* Add a permanent separate file named `migrating_to_vx.y.z.md` to our website (linked at the bottom of this page)
  clearly explaining the breaking change and what needs to be done on the customer side to ensure a safe upgrade
* Add the sentence “This is a breaking change, please refer to `migrating_to_x.y.z.md` for upgrade instructions” to the top of the release notes and in all our announcements

## How Do We Find Incompatibilities

Besides the peer review process for all changes to the code base we also do the followings in order to find
incompatibilities:
* (To be implemented) Automated tests for installing, uninstalling, upgrading from an older version, and downgrading to an older version to check the compatibility of the application
* (To be implemented) Turn the commands in our documentation into scripts that we can automatically run to find incompatibilities between the application and documentation

## Nightly Builds

(To be implemented) Every night we will build and release everything that has been checked into the source code.
This allows us to detect problems including breaking changes and potential drifts in our external dependencies sooner than we otherwise would.
It also allows some of advanced users who have their own nightly builds to test the upcoming changes before they are released.

## Release Candidates

(To be implemented) We are considering having release candidates when we are in major version 1.
By adopting this practice we allow our users who are early adopters to test out new releases before they are available to the wider community thereby providing us with early feedback which would result in more stable releases.

## Security Patches

While we are in major version 0 we will not release security patches to older versions.
Rather we provide the patches in the latest versions.
When at major version 1 we will have an EOL (end of life) policy where we provide security patches
for a subset of older versions and deprecate the others.

# Steps

Upgrading your version of Karpenter requires that you:

1. Set proper permissions in the `KarpenterNode IAM Role` and the `KarpenterController IAM Role`.
  To upgrade Karpenter to version `$VERSION`, make sure that the `KarpenterNode IAM Role` and the `KarpenterController IAM Role` have the right permission described in `https://karpenter.sh/$VERSION/getting-started/getting-started-with-eksctl/cloudformation.yaml`.
1. Locate `KarpenterController IAM Role` ARN (i.e., ARN of the resource created in [Create the KarpenterController IAM Role](../getting-started/getting-started-with-eksctl/#create-the-karpentercontroller-iam-role)) and the cluster endpoint.
1. Pass that information to the helm upgrade command
{{% script file="./content/en/preview/getting-started/getting-started-with-eksctl/scripts/step08-apply-helm-chart.sh" language="bash"%}}
