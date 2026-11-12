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

[comment]: <> (the content below is generated from hack/docs/compatibilitymatrix_gen/main.go)

| Kubernetes |   1.31    |  1.32   |  1.33   |  1.34   |  1.35   |   1.36   |  1.37  |
|------------|-----------|---------|---------|---------|---------|----------|--------|
| karpenter  | \>= 1.0.5 | \>= 1.2 | \>= 1.5 | \>= 1.6 | \>= 1.9 | \>= 1.13 | 1.15.x |

[comment]: <> (end docs generated content from hack/docs/compatibilitymatrix_gen/main.go)

## Support Policy

Patch releases contain bug fixes, security patches, and other targeted changes backported to supported releases.

### Regular Releases

Regular minor version releases are the primary vehicle for new features, performance improvements, and enhancements.

Regular releases are supported **only until the next minor release is published**. Users on regular releases are expected to upgrade promptly to stay current.

### Long-Term Support (LTS) Releases

Every **6 months**, a minor release is designated as a Long-Term Support (LTS) release. LTS releases receive extended maintenance and are the recommended choice for users who prioritize stability over access to the latest features.

LTS releases are identified on GitHub by an `(LTS)` suffix in the release title (e.g. `v1.14.0 (LTS)`) and a banner at the top of the release notes.

| Version | Released | EOL      |
| ---     | ---      | ---      |
| v1.14   | Jul 2026 | Jul 2027 |
| v1.9    | Feb 2026 | Feb 2027 |

### Support Timeline

Each LTS release is supported for **12 months** from its release date. At any point in time, at most **2 LTS releases** are concurrently supported. This creates a 6-month overlap window during which users can migrate from the older LTS to the newer one.

| Phase | Duration | Coverage |
| --- | --- | --- |
| **Active Support** | First 6 months (until the next LTS is released) | Security patches, workload availability fixes, correctness fixes with availability impact |
| **Maintenance Mode** | Last 6 months (overlaps with the newer LTS) | Security patches only |
| **End of Life (EOL)** | After 12 months | No further patches |

For example:

|      | Aug'24         | Jan'25           | Jul'25           | Aug'25 | Jan'26 | Feb'26           | Jul'26 |
| ---  | ---            | ---              | ---              | ---    | ---    | ---              | ---    |
| v1.0 | active support | security patches | -                | EOL    |        |                  |        |
| v1.2 |                | active support   | security patches | -      | EOL    |                  |        |
| v1.6 |                |                  | active support   | -      | -      | security patches | EOL    |
| v1.9 |                |                  |                  |        |        | active support   | -      |

### Backports

During the **Active Support** phase, the following categories of fixes are eligible for backport:

| Category | Description | Examples |
| --- | --- | --- |
| **Security vulnerabilities in Karpenter code** | Fixes for exploitable vulnerabilities identified in Karpenter's own codebase | Security group hashing |
| **Security patches in dependencies** | Updates to consumed libraries (Go modules, container base images) that address known CVEs | CVE in `client-go`, vulnerable base image layer |
| **Workload availability impact** | Bugs that cause running workloads to become unavailable or prevent new workloads from being scheduled | Incorrect node termination, premature drain ignoring disruption budgets, nodes failing to launch for valid workloads |
| **Kubernetes compatibility** | Fixes required to maintain compatibility with Kubernetes minor and patch releases within the LTS version's declared K8s support range | A K8s minor release (e.g., 1.30 -> 1.31) changes behavior that breaks Karpenter's node lifecycle |
| **Observability regressions that mask availability or security issues** | Fixes for bugs where broken metrics, logs, or events prevent users from detecting real availability or security problems | Prometheus metrics for node health silently stop emitting, disruption events no longer surface |

During the **Maintenance Mode** phase, only security vulnerabilities (in Karpenter code and dependencies) are backported.

The following are **not backported** to supported LTS releases. Users seeking these improvements should upgrade to the latest release:

| Category | Examples |
| --- | --- |
| **Performance improvements** | Scheduling latency, memory usage, batching efficiency, controller throughput |
| **Cost optimization** | Improved bin-packing, better consolidation heuristics, smarter instance selection |
| **New features and API additions** | New capabilities are delivered in new minor releases |
| **Support for new cloud provider capabilities** | New instance types, new availability zones, new cloud APIs |
| **General observability enhancements** | New metrics, improved log formatting, additional events (unless masking availability/security issues) |
| **Helm chart improvements** | Chart structure, default values, templating enhancements |
| **Documentation-only fixes** | Corrections or improvements to documentation |
| **Cost-only regressions** | Bugs where the symptom is increased cost but workloads remain available and healthy |
| **Forward compatibility with new Kubernetes minor versions** | LTS versions are not updated to support new features in K8s minors released after the LTS |

Fixes meeting the in-scope criteria above are **eligible** for backport. Maintainers will assess each case considering:

- **Feasibility** — Can the fix be cleanly cherry-picked, or does it require large refactors that increase risk?
- **Risk** — Does the backport introduce regression potential to a stable branch?
- **Severity** — How critical is the issue? (CVSS score for security; blast radius for availability)

Maintainers use best effort to backport qualifying fixes. In cases where a fix is too complex or risky to backport safely, users may be advised to upgrade to a newer release. Such decisions will be communicated transparently on the relevant GitHub issue.

### Kubernetes Version Support

Each LTS release declares a supported Kubernetes version range at release time. The following commitments apply:

- **Kubernetes patch releases** (e.g., 1.30.x -> 1.30.y) within the declared range: Karpenter will maintain compatibility. If a K8s security patch breaks a supported LTS, a fix will be issued.
- **New Kubernetes minor releases** (e.g., 1.30 -> 1.31) published after the LTS: No guarantee of compatibility. Users who need new K8s minors should adopt the latest Karpenter release.

### Upgrade Guidance

- **Regular/LTS -> Regular:** Recommended. Review release notes for behavioral changes.
- **LTS -> LTS:** Upgrade during the 6-month overlap window. Review the cumulative migration guide covering changes between LTS versions.

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

## Release Types

Karpenter offers three types of releases. This section explains the purpose of each release type and how the images for each release type are tagged in our [public image repository](https://gallery.ecr.aws/karpenter).

### Stable Releases

Stable releases are the only recommended versions for production environments. Stable releases are tagged with a semantic version (e.g. `0.35.0`). Note that stable releases prior to `0.35.0` are prefixed with a `v` (e.g. `v0.34.0`). See the [Support Policy](#support-policy) for how long each stable release is supported.

### Release Candidates

We consider having release candidates for major and important minor versions. Our release candidates are tagged like `x.y.z-rc.0`, `x.y.z-rc.1`. The release candidate will then graduate to `x.y.z` as a stable release.
By adopting this practice we allow our users who are early adopters to test out new releases before they are available to the wider community, thereby providing us with early feedback resulting in more stable releases.
Note that, like the stable releases, release candidates prior to `0.35.0` are prefixed with a `v`.

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
