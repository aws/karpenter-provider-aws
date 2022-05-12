---
title: "Upgrade Guide"
linkTitle: "Upgrade Guide"
weight: 10
description: >
  Learn about upgrading Karpenter
---

Karpenter is a controller that runs in your cluster, but it is not tied to a specific Kubernetes version, as the Cluster Autoscaler is.
Use your existing upgrade mechanisms to upgrade your core add-ons in Kubernetes and keep Karpenter up to date on bug fixes and new features.

To make upgrading easier we aim to minimize introduction of breaking changes with the followings:


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
However, to further protect users during this phase we will only introduce breaking changes in minor releases (releases that increment y in x.y.z).
Note this does not mean every minor upgrade has a breaking change as we will also increment the
minor version when we release a new feature.

Users should therefore check to see if there is a breaking change every time they are upgrading to a new minor version.

## How Do We Break Incompatibility?

When there is a breaking change we will:

* Increment the minor version when in major version 0
* Add a permanent separate file named `migrating_to_vx.y.z.md` to our website (linked at the bottom of this page)
  clearly explaining the breaking change and what needs to be done on the user side to ensure a safe upgrade
* Add the sentence “This is a breaking change, please refer to `migrating_to_x.y.z.md` for upgrade instructions” to the top of the release notes and in all our announcements

## How Do We Find Incompatibilities

Besides the peer review process for all changes to the code base we also do the followings in order to find
incompatibilities:
* (To be implemented) To check the compatibility of the application, we ill automate tests for installing, uninstalling, upgrading from an older version, and downgrading to an older version 
* (To be implemented) To check the compatibility of the documentation with the application, we will turn the commands in our documentation into scripts that we can automatically run

## Nightly Builds

(To be implemented) Every night we will build and release everything that has been checked into the source code.
This enables us to detect problems including breaking changes and potential drifts in our external dependencies sooner than we otherwise would.
It also allows some advanced Karpenter users who have their own nightly builds to test the upcoming changes before they are released.

## Release Candidates

(To be implemented) We are considering having release candidates when we are in major version 1.
By adopting this practice we allow our users who are early adopters to test out new releases before they are available to the wider community, thereby providing us with early feedback resulting in more stable releases.

## Security Patches

While we are in major version 0 we will not release security patches to older versions.
Rather we provide the patches in the latest versions.
When at major version 1 we will have an EOL (end of life) policy where we provide security patches
for a subset of older versions and deprecate the others.

# Released Upgrade Notes

## Upgrading to v0.10.0+

v0.10.0 adds a new field, `startupTaints` to the provisioner spec.  Standard Helm upgrades [do not upgrade CRDs](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations) so the  field will not be available unless the CRD is manually updated.  This can be performed prior to the standard upgrade by applying the new CRD manually:

```shell
kubectl replace -f https://raw.githubusercontent.com/aws/karpenter/v0.10.0/charts/karpenter/crds/karpenter.sh_provisioners.yaml
```

📝 If you don't perform this manual CRD update, Karpenter will work correctly except for rejecting the creation/update of provisioners that use `startupTaints`.

## Upgrading to v0.6.2+

If using Helm, the variable names have changed for the cluster's name and endpoint. You may need to update any configuration
that sets the old variable names.

- `controller.clusterName` is now `clusterName`
- `controller.clusterEndpoint` is now `clusterEndpoint`
