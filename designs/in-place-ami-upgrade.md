# In-Place AMI Upgrades for Drift

Let drift resolve an AMI change on an existing EC2 instance by replacing its root volume, instead of terminating and replacing the node.

This is the AWS provider half of [In-Place Drift Remediation](https://github.com/kubernetes-sigs/karpenter/pull/3460), which defines the provider-agnostic core hooks (`DriftRemediation`, `RebootStatus`) and the reuse of the reboot lifecycle.
This document covers the EC2NodeClass opt-in and everything EC2-specific.

## Motivation

When an EC2NodeClass resolves to a new AMI, every node on the old AMI drifts and is replaced.
Terminating the instance discards its instance-store volumes and releases its capacity.
EC2 already supports the alternative: [`CreateReplaceRootVolumeTask`](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/replace-root.html) swaps the root volume for one built from a new AMI on a running instance, through a reboot, and preserves instance-store volumes.
Tracking issue: [#8735](https://github.com/aws/karpenter-provider-aws/issues/8735).

Pinterest validated the mechanism on `i4i.2xlarge`: a canary file on instance store survived `CreateReplaceRootVolumeTask` with a different AMI, and did not survive Karpenter drift.

### Use Cases

- **Instance-store stateful workloads.** Shuffle data, KV stores, search indices and model caches on local NVMe take hours to rebuild after a routine AMI rollout.
- **Capacity reservations and scarce instance types.** Replacing the instance may not get equivalent capacity back.
- **Patch compliance on long-lived nodes.** Teams that block drift on stateful pools can take AMI patches without giving up the data.

### Non-Goals

- **User data changes.** Updating user data requires a stopped instance, and [stopping discards instance-store data](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-store-lifetime.html), so it cannot meet the goal. User data drift keeps replacing.
- **Drift other than AMI.** Subnet, security group, capacity reservation, placement group and static EC2NodeClass drift keep replacing.

## Proposal

### Proposed Spec

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
spec:
  amiDriftPolicy: ReplaceRootVolume   # Replace (default) | ReplaceRootVolume
```

- `Replace`: today's behavior.
- `ReplaceRootVolume`: prefer an in-place root volume replacement for AMI drift. Falls back to `Replace` whenever a safety check fails or the swap fails.
- The field is excluded from the EC2NodeClass drift hash (`hash:"ignore"`). [`EC2NodeClass.Hash`](https://github.com/aws/karpenter-provider-aws/blob/103b2f0a/pkg/apis/v1/ec2nodeclass.go#L553) hashes the whole spec, so without it turning the field on would drift every node on the EC2NodeClass for a non-AMI reason, and they would all be replaced.

### How It Works

1. **Report every drift reason.** [`isNodeClassDrifted`](https://github.com/aws/karpenter-provider-aws/blob/103b2f0a/pkg/cloudprovider/drift.go#L45-L55) returns `AMIDrift` before it checks security groups, subnets, capacity reservations and placement groups. `DriftRemediation` needs to know AMI is the only drift, so the AMI check moves after the others (or all reasons are computed) when `amiDriftPolicy` is `ReplaceRootVolume`.
2. **`DriftRemediation`.** Returns `RebootNode` only when `amiDriftPolicy` is `ReplaceRootVolume`, the reason is `AMIDrift` alone, and every gate below passes. Otherwise `ReplaceNode`.
3. **Gates.** Every failed gate emits an event naming it.
   - AMI level, computed once per EC2NodeClass and exposed as status conditions: the AMI has a single root volume, no Marketplace product codes, is not AL2023 with RAID0 instance store, and its backing snapshot is readable (`createVolumePermission`) from the node's account.
   - Node level, per NodeClaim: AMI family, variant and Kubernetes minor version are unchanged, and the instance is running.
4. **`Reboot`.** When the NodeClaim's `DisruptionReason` is `Drifted`, the provider re-runs the gates and calls `CreateReplaceRootVolumeTask` with the AMI resolved from the EC2NodeClass and `operationID` as the `ClientToken`. Any other reason stays `RebootInstances`.
5. **`RebootStatus`.** Reads the task with `DescribeReplaceRootVolumeTasks`: `pending` and `in-progress` map to `InProgress`, `succeeded` to `Succeeded`, and `failing`, `failed` and `failed-detached` to `Failed`.
6. **`Get`.** Returns the instance's current AMI, so core refreshes `Status.ImageID` and drift clears.

### Interaction with Existing Features

- **Instance store and data volumes.** Only the root volume is replaced. Instance-store and attached data EBS volumes are untouched.
- **Capacity reservations.** The instance keeps its reservation; nothing is relaunched.
- **Cross-account AMIs.** The snapshot gate makes snapshot sharing a prerequisite for organizations that publish AMIs from a central account. Launch permission on the AMI is not enough.

### Observability

- EC2NodeClass status conditions for AMI-level eligibility.
- An event on every `ReplaceNode` decision naming the gate that failed.

### Edge Cases

- **AMI and subnet drift at once.** AMI is not the only drift, so the node is replaced.
- **Snapshot not shared to the node's account.** The gate fails and the node is replaced. Operators fix sharing and later rollouts go in place.
- **Controller restart mid-swap.** The re-issued call carries the same `ClientToken`, so it joins the existing task.
- **Task fails after the drain.** `RebootStatus` reports `Failed` and core replaces the node.

## Alternatives Considered

### Stop, update user data and AMI, start

Rejected because stopping an instance discards instance-store data, which is the data this feature exists to keep.

### Operator tooling outside Karpenter

Scripts that call `CreateReplaceRootVolumeTask` directly.
Rejected because they bypass disruption budgets, drains and drift state, and race Karpenter's own drift.

## Backward Compatibility

- `amiDriftPolicy` defaults to `Replace` and is excluded from the drift hash. Upgrading changes nothing.

## Graduation Criteria

- **Alpha:** behind the core feature gate, AWS as the first provider. Proves the gates on real fleets.
- **Beta:** enabled by default after production data on swap duration across instance types.

## Open Questions

1. Names and shape of the EC2NodeClass status conditions for AMI-level eligibility.
2. Root volume replacements typically take 2 to 10 minutes. Is core's 20 minute observation window right for them?
