## Versioning

Karpenter follows [Semantic Versioning 2.0.0](https://semver.org/).

- **Major version** increments indicate breaking API changes.
- **Minor version** increments introduce new features or behavioral changes.
- **Patch version** increments contain bug fixes, security patches, and other targeted changes backported to supported releases.

## Release Types

### Regular Releases

Regular minor version releases are the primary vehicle for new features, performance improvements, and enhancements.

Regular releases are supported **only until the next minor release is published**. Users on regular releases are expected to upgrade promptly to stay current.

### Long-Term Support (LTS) Releases

Every **6 months**, a minor release is designated as a Long-Term Support (LTS) release. LTS releases receive extended maintenance and are the recommended choice for users who prioritize stability over access to the latest features.

## Support Timeline

Each LTS release is supported for **12 months** from its release date. At any point in time, at most **2 LTS releases** are concurrently supported. This creates a 6-month overlap window during which users can migrate from the older LTS to the newer one.

### Support Phases

| Phase | Duration | Coverage |
| --- | --- | --- |
| **Active Support** | First 6 months (until the next LTS is released) | Security patches, workload availability fixes, correctness fixes with availability impact |
| **Maintenance Mode** | Last 6 months (overlaps with the newer LTS) | Security patches only |
| **End of Life (EOL)** | After 12 months | No further patches |

### Example Timeline

|      | Aug'24         | Jan'25           | Jul'25           | Aug'25 | Jan'26 | Feb'26           | Jul'26 | 
| ---  | ---            | ---              | ---              | ---    | ---    | ---              | ---    | 
| v1.0 | active support | security patches | -                | EOL    |        |                  |        |  
| v1.2 |                | active support   | security patches | -      | EOL    |                  |        |  
| v1.6 |                |                  | active support   | -      | -      | security patches | EOL    | 
| v1.9 |                |                  |                  |        |        | active support   | -      | 

## What Is Backported to Supported LTS Releases

During the **Active Support** phase, the following categories of fixes are eligible for backport:

### In-Scope

| Category | Description | Examples |
| --- | --- | --- |
| **Security vulnerabilities in Karpenter code** | Fixes for exploitable vulnerabilities identified in Karpenter's own codebase | Security group hashing |
| **Security patches in dependencies** | Updates to consumed libraries (Go modules, container base images) that address known CVEs | CVE in `client-go`, vulnerable base image layer |
| **Workload availability impact** | Bugs that cause running workloads to become unavailable or prevent new workloads from being scheduled | Incorrect node termination, premature drain ignoring disruption budgets, nodes failing to launch for valid workloads |
| **Kubernetes compatibility** | Fixes required to maintain compatibility with Kubernetes minor and patch releases within the LTS version's declared K8s support range | A K8s minor release (e.g., 1.30 -> 1.31) changes behavior that breaks Karpenter's node lifecycle |
| **Observability regressions that mask availability or security issues** | Fixes for bugs where broken metrics, logs, or events prevent users from detecting real availability or security problems | Prometheus metrics for node health silently stop emitting, disruption events no longer surface |

During the **Maintenance Mode** phase, only security vulnerabilities (in Karpenter code and dependencies) are backported.

### Out of Scope

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

## Backport Process and Commitment

Fixes meeting the in-scope criteria above are **eligible** for backport. Maintainers will assess each case considering:

- **Feasibility** — Can the fix be cleanly cherry-picked, or does it require large refactors that increase risk?
- **Risk** — Does the backport introduce regression potential to a stable branch?
- **Severity** — How critical is the issue? (CVSS score for security; blast radius for availability)

Maintainers use best effort to backport qualifying fixes. In cases where a fix is too complex or risky to backport safely, users may be advised to upgrade to a newer release. Such decisions will be communicated transparently on the relevant GitHub issue.

## Kubernetes Version Compatibility

Each LTS release declares a supported Kubernetes version range at release time. The following commitments apply:

- **Kubernetes patch releases** (e.g., 1.30.x -> 1.30.y) within the declared range: Karpenter will maintain compatibility. If a K8s security patch breaks a supported LTS, a fix will be issued.
- **New Kubernetes minor releases** (e.g., 1.30 -> 1.31) published after the LTS: No guarantee of compatibility. Users who need new K8s minors should adopt the latest Karpenter release.

## LTS Release Marking

LTS releases are clearly identified on GitHub through the following conventions:

**Release title:**

```
v1.14.0 (LTS)
```

**Release notes banner** (first line of every LTS release's notes):

```
> 🛡️ **This is a Long-Term Support (LTS) release.**
> Supported until [month year]. See SUPPORT.md for details.
```

## LTS versions table

| Version | Released | EOL      |
| ---     | ---      | ---      |
| v1.14   | Jul 2026 | Jul 2027 |
| v1.9    | Feb 2026 | Feb 2027 |

> **Note:** Regular (non-LTS) releases are supported only until the next minor release is published. Only LTS releases receive backported patches beyond their initial release.

## Upgrade Guidance

- **Regular/LTS -> Regular:** Recommended. Review release notes for behavioral changes.
- **LTS -> LTS:** Upgrade during the 6-month overlap window. Review the cumulative migration guide covering changes between LTS versions.
