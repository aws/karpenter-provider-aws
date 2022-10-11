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

Karpenter follows [Semantic Versioning 2.0.0](https://semver.org/) in its stable release versions, while in
major version zero (v0.y.z) [anything may change at any time](https://semver.org/#spec-item-4).
However, to further protect users during this phase we will only introduce breaking changes in minor releases (releases that increment y in x.y.z).
Note this does not mean every minor upgrade has a breaking change as we will also increment the
minor version when we release a new feature.

Users should therefore check to see if there is a breaking change every time they are upgrading to a new minor version.

## Custom Resource Definition (CRD) Upgrades

Karpenter ships with a few Custom Resource Definitions (CRDs). These CRDs are part of the helm chart [here](https://github.com/aws/karpenter/blob/main/charts/karpenter/crds). Helm [does not manage the lifecycle of CRDs](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/), the tool will only install the CRD during the first installation of the helm chart. Subsequent chart upgrades will not add or remove CRDs, even if the CRDs have changed. When CRDs are changed, we will make a note in the version's upgrade guide.

In general, you can reapply the CRDs in the `crds` directory of the Karpenter helm chart:

```shell
kubectl replace -f https://raw.githubusercontent.com/aws/karpenter{{< githubRelRef >}}charts/karpenter/crds/karpenter.sh_provisioners.yaml

kubectl replace -f https://raw.githubusercontent.com/aws/karpenter{{< githubRelRef >}}charts/karpenter/crds/karpenter.k8s.aws_awsnodetemplates.yaml
```

## How Do We Break Incompatibility?

When there is a breaking change we will:

* Increment the minor version when in major version 0
* Add a permanent separate section named `upgrading to vx.y.z+` under [released upgrade notes](#released-upgrade-notes)
  clearly explaining the breaking change and what needs to be done on the user side to ensure a safe upgrade
* Add the sentence “This is a breaking change, please refer to the above link for upgrade instructions” to the top of the release notes and in all our announcements

## How Do We Find Incompatibilities

Besides the peer review process for all changes to the code base we also do the followings in order to find
incompatibilities:
* (To be implemented) To check the compatibility of the application, we will automate tests for installing, uninstalling, upgrading from an older version, and downgrading to an older version
* (To be implemented) To check the compatibility of the documentation with the application, we will turn the commands in our documentation into scripts that we can automatically run

## Security Patches

While we are in major version 0 we will not release security patches to older versions.
Rather we provide the patches in the latest versions.
When at major version 1 we will have an EOL (end of life) policy where we provide security patches
for a subset of older versions and deprecate the others.

# Release Types

Karpenter offers four types of releases. This section explains the purpose of each release type and how the images for each release type are tagged in our [public image repository](https://gallery.ecr.aws/karpenter).

## Stable Releases

Stable releases are the most reliable releases that are released with weekly cadence. Stable releases are our only recommended versions for production environments.
Sometimes we skip a stable release because we find instability or problems that need to be fixed before having a stable release.
Stable releases are tagged with Semantic Versioning. For example `v0.13.0`.

## Snapshot Releases

We release a snapshot release for every commit that gets merged into the main repository. This enables our users to immediately try a new feature or fix right after it's merged rather than waiting days or weeks for release.
Snapshot releases are suitable for testing, and troubleshooting but users should exercise great care if they decide to use them in production environments.
Snapshot releases are tagged with the git commit hash prefixed by the Karpenter major version. For example `v0-fc17bfc89ebb30a3b102a86012b3e3992ec08adf`. For more detailed examples on how to use snapshot releases look under "Usage" in [Karpenter Helm Chart](https://gallery.ecr.aws/karpenter/karpenter).

## Nightly Releases

Every night we build and release everything that has been checked into the source code. This enables us to detect problems including breaking changes and potential drifts in our external dependencies sooner than we otherwise would.
It also allows some advanced Karpenter users who have their own nightly builds to test the upcoming changes before they are released. Nightly releases are tagged with date in YYYYMMDD format. For example `20220713`.
For more examples on how to use nightly releases look under "Usage" in [Karpenter Helm Chart](https://gallery.ecr.aws/karpenter/karpenter).

## Release Candidates

We consider having release candidates for major and important minor versions. Our release candidates are tagged like `vx.y.z-rc.0`, `vx.y.z-rc.1`. The release candidate will then graduate to `vx.y.z` as a normal stable release.
By adopting this practice we allow our users who are early adopters to test out new releases before they are available to the wider community, thereby providing us with early feedback resulting in more stable releases.

# Released Upgrade Notes

## Upgrading to v0.18.0+
* v0.18.0 removes the `karpenter_consolidation_nodes_created` and `karpenter_consolidation_nodes_terminated` prometheus metrics in favor of the more generic `karpenter_nodes_created` and `karpenter_nodes_terminated` metrics. You can still see nodes created and terminated by consolidation by checking the `reason` label on the metrics. Check out all the metrics published by Karpenter [here](../tasks/metrics/).

## Upgrading to v0.17.0+
Karpenter's Helm chart package is now stored in [Karpenter's OCI (Open Container Initiative) registry](https://gallery.ecr.aws/karpenter/karpenter). The Helm CLI supports the new format since [v3.8.0+](https://helm.sh/docs/topics/registries/).
With this change [charts.karpenter.sh](https://charts.karpenter.sh/) is no longer updated but preserved to allow using older Karpenter versions. For examples on working with the Karpenter helm charts look at [Install Karpenter Helm Chart]({{< ref "../getting-started/getting-started-with-eksctl/#install-karpenter-helm-chart" >}}).

Users who have scripted the installation or upgrading of Karpenter need to adjust their scripts with the following changes:
1. There is no longer a need to add the Karpenter helm repo to helm
2. The full URL of the Helm chart needs to be present when using the helm commands

## Upgrading to v0.16.2+
* v0.16.2 adds new kubeletConfiguration fields to the `provisioners.karpenter.sh` v1alpha5 CRD.  The CRD will need to be updated to use the new parameters:
```bash
kubectl replace -f https://raw.githubusercontent.com/aws/karpenter/v0.16.2/charts/karpenter/crds/karpenter.sh_provisioners.yaml
```

## Upgrading to v0.16.0+
* v0.16.0 adds a new weight field to the `provisioners.karpenter.sh` v1alpha5 CRD.  The CRD will need to be updated to use the new parameters:
```bash
kubectl replace -f https://raw.githubusercontent.com/aws/karpenter/v0.16.0/charts/karpenter/crds/karpenter.sh_provisioners.yaml
```

## Upgrading to v0.15.0+
* v0.15.0 adds a new consolidation field to the `provisioners.karpenter.sh` v1alpha5 CRD.  The CRD will need to be updated to use the new parameters:
```bash
kubectl replace -f https://raw.githubusercontent.com/aws/karpenter/v0.15.0/charts/karpenter/crds/karpenter.sh_provisioners.yaml
```

## Upgrading to v0.14.0+
* v0.14.0 adds new fields to the `provisioners.karpenter.sh` v1alpha5 and `awsnodetemplates.karpenter.k8s.aws` v1alpha1 CRDs. The CRDs will need to be updated to use the new parameters:

```bash
kubectl replace -f https://raw.githubusercontent.com/aws/karpenter/v0.14.0/charts/karpenter/crds/karpenter.sh_provisioners.yaml

kubectl replace -f https://raw.githubusercontent.com/aws/karpenter/v0.14.0/charts/karpenter/crds/karpenter.k8s.aws_awsnodetemplates.yaml
```

* v0.14.0 changes the way Karpenter discovers its dynamically generated AWS launch templates to use a tag rather than a Name scheme. The previous name scheme was `Karpenter-${CLUSTER_NAME}-*` which could collide with user created launch templates that Karpenter should not manage. The new scheme uses a tag on the launch template `karpenter.k8s.aws/cluster: ${CLUSTER_NAME}`. As a result, Karpenter will not clean-up dynamically generated launch templates using the old name scheme. You can manually clean these up with the following commands:

```bash
## Find launch templates that match the naming pattern and you do not want to keep
aws ec2 describe-launch-templates --filters="Name=launch-template-name,Values=Karpenter-${CLUSTER_NAME}-*"

## Delete launch template(s) that match the name but do not have the "karpenter.k8s.aws/cluster" tag
aws ec2 delete-launch-template --launch-template-id <LAUNCH_TEMPLATE_ID>
```

* v0.14.0 introduces additional instance type filtering if there are no `node.kubernetes.io/instance-type` or `karpenter.k8s.aws/instance-family` or `karpenter.k8s.aws/instance-category` requirements that restrict instance types specified on the provisioner. This prevents Karpenter from launching bare metal and some older non-current generation instance types unless the provisioner has been explicitly configured to allow them. If you specify an instance type or family requirement that supplies a list of instance-types or families, that list will be used regardless of filtering.  The filtering can also be completely eliminated by adding an `Exists` requirement for instance type or family.
```yaml
  - key: node.kubernetes.io/instance-type
    operator: Exists
```

* v0.14.0 introduces support for custom AMIs without the need for an entire launch template. You must add the `ec2:DescribeImages` permission to the Karpenter Controller Role for this feature to work. This permission is needed for Karpenter to discover custom images specified. Read the [Custom AMI documentation here](../aws/provisioning/#amiselector) to get started
* v0.14.0 adds an an additional default toleration (CriticalAddonOnly=Exists) to the Karpenter helm chart. This may cause Karpenter to run on nodes with that use this Taint which previously would not have been schedulable. This can be overridden by using `--set tolerations[0]=null`.

* v0.14.0 deprecates the `AWS_ENI_LIMITED_POD_DENSITY` environment variable in-favor of specifying `spec.kubeletConfiguration.maxPods` on the Provisioner. `AWS_ENI_LIMITED_POD_DENSITY` will continue to work when `maxPods` is not set on the Provisioner. If `maxPods` is set, it will override `AWS_ENI_LIMITED_POD_DENSITY` on that specific Provisioner.

## Upgrading to v0.13.0+
* v0.13.0 introduces a new CRD named `AWSNodeTemplate` which can be used to specify AWS Cloud Provider parameters. Everything that was previously specified under `spec.provider` in the Provisioner resource, can now be specified in the spec of the new resource. The use of `spec.provider` is deprecated but will continue to function to maintain backwards compatibility for the current API version (v1alpha5) of the Provisioner resource. v0.13.0 also introduces support for custom user data that doesn't require the use of a custom launch template. The user data can be specified in-line in the AWSNodeTemplate resource. Read the [UserData documentation here](../aws/operating-systems) to get started.

  If you are upgrading from v0.10.1 - v0.11.1, a new CRD `awsnodetemplate` was added. In v0.12.0, this crd was renamed to `awsnodetemplates`. Since helm does not manage the lifecycle of CRDs, you will need to perform a few manual steps for this CRD upgrade:
  1. Make sure any `awsnodetemplate` manifests are saved somewhere so that they can be reapplied to the cluster.
  2. `kubectl delete crd awsnodetemplate`
  3. `kubectl apply -f https://raw.githubusercontent.com/aws/karpenter/v0.13.2/charts/karpenter/crds/karpenter.k8s.aws_awsnodetemplates.yaml`
  4. Perform the Karpenter upgrade to v0.13.x, which will install the new `awsnodetemplates` CRD.
  5. Reapply the `awsnodetemplate` manifests you saved from step 1, if applicable.
* v0.13.0 also adds EC2/spot price fetching to Karpenter to allow making more accurate decisions regarding node deployments.  Our getting started guide documents this, but if you are upgrading Karpenter you will need to modify your Karpenter controller policy to add the `pricing:GetProducts` and `ec2:DescribeSpotPriceHistory` permissions.


## Upgrading to v0.12.0+
* v0.12.0 adds an OwnerReference to each Node created by a provisioner. Previously, deleting a provisioner would orphan nodes. Now, deleting a provisioner will cause Kubernetes [cascading delete](https://kubernetes.io/docs/concepts/architecture/garbage-collection/#cascading-deletion) logic to gracefully terminate the nodes using the Karpenter node finalizer. You may still orphan nodes by removing the owner reference.
* If you are upgrading from v0.10.1 - v0.11.1, a new CRD `awsnodetemplate` was added. In v0.12.0, this crd was renamed to `awsnodetemplates`. Since helm does not manage the lifecycle of CRDs, you will need to perform a few manual steps for this CRD upgrade:
  1. Make sure any `awsnodetemplate` manifests are saved somewhere so that they can be reapplied to the cluster.
  2. `kubectl delete crd awsnodetemplate`
  3. `kubectl apply -f https://raw.githubusercontent.com/aws/karpenter/v0.12.1/charts/karpenter/crds/karpenter.k8s.aws_awsnodetemplates.yaml`
  4. Perform the Karpenter upgrade to v0.12.x, which will install the new `awsnodetemplates` CRD.
  5. Reapply the `awsnodetemplate` manifests you saved from step 1, if applicable.

## Upgrading to v0.11.0+

v0.11.0 changes the way that the `vpc.amazonaws.com/pod-eni` resource is reported.  Instead of being reported for all nodes that could support the resources regardless of if the cluster is configured to support it, it is now controlled by a command line flag or environment variable. The parameter defaults to false and must be set if your cluster uses [security groups for pods](https://docs.aws.amazon.com/eks/latest/userguide/security-groups-for-pods.html).  This can be enabled by setting the environment variable `AWS_ENABLE_POD_ENI` to true via the helm value `controller.env`.

Other extended resources must be registered on nodes by their respective device plugins which are typically installed as DaemonSets (e.g. the `nvidia.com/gpu` resource will be registered by the [NVIDIA device plugin](https://github.com/NVIDIA/k8s-device-plugin). Previously, Karpenter would register these resources on nodes at creation and they would be zeroed out by `kubelet` at startup.  By allowing the device plugins to register the resources, pods will not bind to the nodes before any device plugin initialization has occurred.

v0.11.0 adds a `providerRef` field in the Provisioner CRD. To use this new field you will need to replace the Provisioner CRD manually:

```shell
kubectl replace -f https://raw.githubusercontent.com/aws/karpenter/v0.11.0/charts/karpenter/crds/karpenter.sh_provisioners.yaml
```

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
