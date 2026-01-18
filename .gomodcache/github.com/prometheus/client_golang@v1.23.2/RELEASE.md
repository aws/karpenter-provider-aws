# Release

The Prometheus Go client library does not follow a strict release schedule. Releases are made based on necessity and the current state of the project.

## Branch Management

We use [Semantic Versioning](https://semver.org/).

- Maintain separate `release-<major>.<minor>` branches
- Branch protection enabled automatically for `release-*` branches
- Bug fixes go to the latest release branch, then merge to main
- Features and changes go to main branch
- Non-latest minor release branches are maintained (e.g. bug and security fixes) on the best-effort basis

## Pre-Release Preparations

1. Review main branch state:
   - Expedite critical bug fixes
   - Don't rush on risky changes, consider them for the next release if not ready
   - Update dependencies via Dependabot PRs or manually if needed
   - Check for security alerts

## Cutting a Minor Release

1. Create release branch:

   ```bash
   git checkout -b release-<major>.<minor> main
   git push origin release-<major>.<minor>
   ```

2. Create a new branch on top of `release-<major>.<minor>`:

   ```bash
   git checkout -b <yourname>/cut-<major>.<minor>.0 release-<major>.<minor>
   ```

3. Update version and documentation:
   - Update `VERSION` file
   - Update `CHANGELOG.md` (user-impacting changes)
      - Each release documents the minimum required Go version
   - Order: [SECURITY], [CHANGE], [FEATURE], [ENHANCEMENT], [BUGFIX]
   - For RCs, append `-rc.0`

4. Create PR and get review

5. After merge, create tags:

   ```bash
   tag="v$(< VERSION)"
   git tag -s "${tag}" -m "${tag}"
   git push origin "${tag}"
   ```

6. Create a draft release.
  - Copy Changelog section.
  - You can also generate automatic changelog and put the `What's changed` section under `<summary>` HTML tag. This will render all contributors nicely as in the [example](https://github.com/prometheus/client_golang/releases/tag/v1.21.0-rc.0) release.

7a. For Release Candidates:
   - Release RC GitHub release with "pre-release" box checked
   - Click "Publish release"!
   - Create PR against [prometheus/prometheus](https://github.com/prometheus/prometheus) using RC version (e.g. https://github.com/prometheus/prometheus/pull/15851)
   - Create PR against [kubernetes/kubernetes](https://github.com/kubernetes/kubernetes) using RC version (e.g. https://github.com/kubernetes/kubernetes/pull/129752)
     - Kubernetes uses scripts to update deps  e.g.: 
 
```bash
./hack/pin-dependency.sh github.com/prometheus/client_golang v1.21.0-rc.0
./hack/update-vendor.sh
# If indirect dependencies changed, following check will fail on the CI:
./hack/verify-vendor.sh
# You will need to modify hack/unwanted-dependencies.json manually as the check suggests.
```
     
   - Make sure the CI is green for the PRs
   - Allow 1-2 days for downstream testing
   - Fix any issues found before final release
   - Use `-rc.1`, `-rc.2` etc. for additional fixes
  
7b. For Final Release:
   - Release GitHub release with "latest" box checked (default).
   - Click "Publish release"!

8. Announce release:
   - <prometheus-announce@googlegroups.com>
   - Slack
   - x.com/BlueSky

9. Merge release branch to main:

   ```bash
   git checkout main
   git merge --no-ff release-<major>.<minor>
   ```

## Cutting a Patch Release

1. Create branch from release branch:

   ```bash
   git checkout -b <yourname>/cut-<major>.<minor>.<patch> release-<major>.<minor>
   ```

2. Apply fixes:
   - Commit the required fixes; avoid refactoring or otherwise risky changes (preferred)
   - Cherry-pick from main if fix was already merged there: `git cherry-pick <commit>`

3. Follow steps 3-9 from minor release process

## Handling Merge Conflicts

If conflicts occur merging to main:

1. Create branch: `<yourname>/resolve-conflicts`
2. Fix conflicts there
3. PR into main
4. Leave release branch unchanged

## Note on Versioning

## Compatibility Guarantees

### Supported Go Versions

- Support provided only for the two most recent major Go releases
- While the library may work with older Go versions, support and fixes are best-effort for those.
- Each release documents the minimum required Go version

### API Stability

The Prometheus Go client library aims to maintain backward compatibility within minor versions, similar to [Go 1 compatibility promises](https://golang.org/doc/go1compat):


## Minor Version Changes
- API signatures are `stable` within a **minor** version
- No breaking changes are introduced
- Methods may be added, but not removed
   - Arguments may NOT be removed or added (unless varargs)
   - Return types may NOT be changed
- Types may be modified or relocated
- Default behaviors might be altered (unfortunately, this has happened in the past)

## Major Version Changes
- API signatures may change between **major** versions
- Types may be modified or relocated
- Default behaviors might be altered
- Feature removal/deprecation can occur with minor version bump

### Compatibility Testing

Before each release:

1. **Internal Testing**:
   - Full test suite must pass
   - Integration tests with latest Prometheus server
   - (optional) Benchmark comparisons with previous version
   > There is no facility for running benchmarks in CI, so this is best-effort.

2. **External Validation**:

Test against bigger users, especially looking for broken tests or builds. This will give us awareness of a potential accidental breaking changes, or if there were intentional ones, the potential damage radius of them.

   - Testing with [prometheus/prometheus](https://github.com/prometheus/prometheus) `main` branch
   - Testing with [kubernetes/kubernetes](https://github.com/kubernetes/kubernetes) `main` branch
   - Breaking changes must be documented in CHANGELOG.md

### Version Policy

- Bug fixes increment patch version (e.g., v0.9.1)
- New features increment minor version (e.g., v0.10.0)
- Breaking changes increment minor version with clear documentation

### Deprecation Policy

1. Features may be deprecated in any minor release
2. Deprecated features:
   - Will be documented in CHANGELOG.md
   - Will emit warnings when used (when possible)

