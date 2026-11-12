# E2E suites

Each directory under `test/suites` is one Ginkgo suite that runs against a real cluster. This file explains how the suites are laid out and what to follow when adding or changing a spec.

## Why specs live in non-test files

Specs are written in ordinary `.go` files (`drift/drift.go`, `integration/tags.go`, ...) in a package named after the suite, not in `_test.go` files. Go can't import `_test.go` files, so keeping specs out of them lets other repos import a suite and run it against their own cluster setup without copying the specs. A copy drifts from upstream the day it's made; an import picks up every new spec on the next version bump.

Keep it that way:

- Put new specs in a non-test file in the suite package, never in `suite_test.go`.
- Only `suite_test.go` is a `_test.go` file. It holds the Ginkgo hooks and nothing else.
- Read test data with `//go:embed`, not a relative `os.ReadFile`, since an importer's working directory isn't this repo.
- Lint runs on these files like any other code (`make verify`), so int conversions need `//nolint:gosec` and long registration functions need `//nolint:gocyclo`, as in `scheduling/scheduling.go`.

## Layout of a suite

```
test/suites/drift/
  drift.go        # package drift: Env, NodeClass, NodePool vars and RegisterTests()
  suite_test.go   # package drift_test: TestDrift, BeforeSuite/AfterSuite, BeforeEach/AfterEach, calls drift.RegisterTests()
```

The suite package exports `Env`, `NodeClass` and `NodePool`. `suite_test.go` sets them in `BeforeSuite`/`BeforeEach` and calls `RegisterTests()` once at package init. An importer writes its own `suite_test.go` that does the same with its own environment, then calls the same `RegisterTests()`.

Bigger suites split specs across files. Each file has an unexported `registerX()` function and `RegisterTests()` calls them all (see `integration/integration.go`). Add a new file's register function there, or its specs never run.

A suite can be registered more than once with different parameters. `scheduling.RegisterTests(policy)` is run by both `scheduling-strict` and `scheduling-besteffort`, each with its own `suite_test.go`.

## Where helpers live

- `test/pkg/environment/common`: provider-neutral environment. Kubernetes clients, `Expect*`/`EventuallyExpect*` helpers, cleanup, controller settings, and the `Require*` capability checks.
- `test/pkg/environment/aws`: AWS environment on top of common. AWS SDK clients, default EC2NodeClass and NodePool, subnet and security group discovery, AMI lookups, FIS, EBS CSI, IPv6.
- `test/pkg/debug`: node, pod and event monitors that log during a run. Label a spec `debug.NoWatch` or `debug.NoEvents` to turn them off for large scale specs.

Before writing a new helper, look for one in these packages. A helper that waits on cluster state belongs in the environment, not in a suite file, so every suite and every importer gets it.

## Writing specs that run on more than one setup

Importers run these specs on clusters that differ from the upstream CI cluster: Karpenter may run outside the cluster, some add-ons may be missing, and some controller settings may be fixed. Specs stay portable when they follow these rules.

- **Go through the environment for controller details.** Use `Env.ExpectSettingsOverridden` and friends to change settings, never patch the karpenter Deployment directly. Call `Env.RequireInClusterController(reason)` before reading controller pods, logs, leases or metrics. With `Env.ExternalController` set, those helpers skip or no-op instead of failing.
- **Check capabilities, then skip with a tagged reason.** Use the `Require*` helpers (`RequireCRD`, `RequireVPCCNI`, `RequireIPv6Cluster`, `RequireEBSCSIDriver`, ...). They call `Skip("[unsupported:<capability>] <why>")`. Reports group skips by the bracketed tag, so a new skip should use one of these helpers or the same `[unsupported:<capability>]` format.
- **Label static requirements with `needs:<capability>`.** If a spec always needs something the default cluster doesn't have (`needs:zonal-shift`, `needs:capacity-reservation`, `needs:windows`, `needs:cost-budget`, ...), add a Ginkgo `Label`, so runners can filter with `--ginkgo.label-filter` before the spec starts.
- **Label specs that change shared resources.** A spec that changes something other clusters in the account may use, such as the controller's IAM role or the subnets, gets a `needs:dedicated-<resource>` label (`needs:dedicated-controller-role`, `needs:dedicated-subnets`), so runners in a shared test account can skip it.
- **Don't assume discovery tags.** Get subnets and security groups from `Env.DiscoveredSubnets`/`Env.DiscoveredSecurityGroups`, which honor `Env.SubnetIDs`/`Env.SecurityGroupIDs` when set and fall back to `karpenter.sh/discovery` tags otherwise.
- **Wait on state, not time.** Prefer `Eventually` on an object, condition or event over `time.Sleep`. A sleep encodes how fast one controller is and fails on any other.
- **Clean up what the spec creates outside the cluster.** IAM policies, zonal shifts, capacity reservations and similar resources need a `DeferCleanup` so an interrupted run doesn't leak them.
- **Make Ordered containers fail usefully.** An `Ordered` container skips every later spec after the first failure. Add `ContinueOnFailure` when the specs don't depend on each other.
- **Don't hardcode a size that only fits one AMI.** Derive requests from the instance's allocatable, not from a number that happens to fit AL2023.

## Adding a spec

1. Find the suite whose `BeforeEach` gives you the right default NodeClass and NodePool, and the file in it that matches the feature.
2. Add the `It` inside that file's `Describe`, or add a new file with a `registerX()` and call it from `RegisterTests()`.
3. Use `Env`, `NodeClass` and `NodePool` from the suite package, and the helpers above.
4. Add `needs:` labels and `Require*` checks for anything the default cluster doesn't have.
5. Build the suite and dry-run it to check the spec registers: `go test -c ./test/suites/<suite> && ./<suite>.test --ginkgo.dry-run -v`.

## Adding a suite

Copy an existing small suite (`localzone` or `nodeclaim`). Create the package with `Env`/`NodeClass`/`NodePool` and `RegisterTests()`, a `suite_test.go` with the standard hooks, and add the suite name to the lists in `.github/workflows/e2e.yaml` and `.github/workflows/e2e-matrix.yaml`.

## Running

`make e2etests` runs a suite against the cluster in your kubeconfig. Set `CLUSTER_NAME`, `CLUSTER_ENDPOINT` and `TEST_SUITE`, and optionally `FOCUS`. See `test/README.md` for the GitHub Actions setup.
