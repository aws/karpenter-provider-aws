# In-Place AMI Upgrades for Drift

Let drift resolve an AMI change on an existing EC2 instance by replacing its root volume, instead of terminating and replacing the node.

This is the AWS provider half of [In-Place Drift Remediation](https://github.com/kubernetes-sigs/karpenter/pull/3460), which replaces `IsDrifted` with the provider-agnostic `GetDriftStatus` (`NoOp | Replace | Reboot`) and reuses the reboot lifecycle.
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
  amiDriftPolicy: ReplaceRootVolume   # ReplaceInstance (default) | ReplaceRootVolume
```

- `ReplaceInstance`: today's behavior.
- `ReplaceRootVolume`: prefer an in-place root volume replacement for AMI drift. Falls back to replacing the instance whenever a safety check fails or the swap fails.

Each entry in `status.amis` reports whether that AMI can be swapped in place:

```yaml
status:
  amis:
    - id: ami-0abc
      requirements: [...]
      inPlaceUpgrade:
        supported: true
    - id: ami-0def
      requirements: [...]
      inPlaceUpgrade:
        supported: false
        reason: SnapshotNotReadable
```

An EC2NodeClass can resolve several AMIs (per architecture or variant), and each can pass or fail on its own, so the result lives on the AMI entry rather than in one EC2NodeClass condition.

An AMI cannot be swapped in place, and `reason` names the check, when:

- `ProductCodes`: the AMI carries Marketplace product codes. EC2 requires the new AMI to have the same product code and billing information as the instance, so Karpenter only swaps AMIs without product codes. Karpenter reads this from the image's `ProductCodes` in `DescribeImages`.
- `SnapshotNotReadable`: the node's account cannot create a volume from the AMI's root snapshot (`createVolumePermission`), which is common when AMIs are published from a central account. Karpenter reads this from `DescribeSnapshots` by snapshot ID. It does not pass `RestorableByUserIds=self`, which leaves out public snapshots.

Node-level checks run per NodeClaim and are not reported in `status.amis`, because they compare the AMI with the running instance. They cover every deterministic EC2 [requirement](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/replace-root.html), so a swap EC2 would reject never drains the node.
The AMI family, variant and Kubernetes minor version must be unchanged, which also keeps the architecture and virtualization type EC2 requires, and the instance must be running and not bare metal.
The node is also not swapped, and the event names the check, when:

- `EnaSupport`: the instance has ENA enabled and the AMI does not support it.
- `SriovNetSupport`: the instance has sriov-net enabled and the AMI does not support it.
- `TpmSupport`: the instance has NitroTPM enabled and the AMI does not.
- `BootMode`: the AMI's boot mode is `uefi` or `legacy-bios` and differs from the instance's `currentInstanceBootMode`.

These fields come from the `DescribeInstances` and `DescribeImages` responses Karpenter already fetches, so the checks add no API calls.
Encryption is not a gate: a replacement volume restored from an AMI keeps the original volume's encryption and KMS key.

AMIs with more than one volume are not rejected.
Only the root volume is replaced; volumes already attached to the instance stay attached with their data.
Bottlerocket is the main case: its AMI defines a root volume and a data volume, and the data volume keeps container images and state across the swap.

### How It Works

1. **Keep the field out of the drift hash.** `amiDriftPolicy` is excluded with `hash:"ignore"`. [`EC2NodeClass.Hash`](https://github.com/aws/karpenter-provider-aws/blob/103b2f0a/pkg/apis/v1/ec2nodeclass.go#L553) hashes the whole spec, so without it turning the field on would drift every node on the EC2NodeClass for a non-AMI reason, and they would all be replaced.
2. **Report every drift reason.** [`isNodeClassDrifted`](https://github.com/aws/karpenter-provider-aws/blob/103b2f0a/pkg/cloudprovider/drift.go#L45-L55) returns `AMIDrift` before it checks security groups, subnets, capacity reservations and placement groups. `GetDriftStatus` needs to know AMI is the only drift, so the AMI check moves after the others (or all reasons are computed) when `amiDriftPolicy` is `ReplaceRootVolume`.
3. **`GetDriftStatus`.** Returns `NoOp` when nothing drifted. Returns `Reboot` only when `amiDriftPolicy` is `ReplaceRootVolume`, the reason is `AMIDrift` alone, and every gate below passes. Any other drift returns `Replace`. The AMI it checks is the one the NodeClaim would launch with now: the `status.amis` entry whose requirements match the NodeClaim, the same choice a launch makes. Its `inPlaceUpgrade` result decides the AMI-level gates. While that result is not computed yet, `GetDriftStatus` returns an error and the node is retried on a later pass, so turning the policy on never races into replacements.
4. **Gates.** Every failed gate emits an event naming it.
   - AMI level, reported in `status.amis[].inPlaceUpgrade`: the AMI has no Marketplace product codes, and its root snapshot is readable (`createVolumePermission`) from the node's account. Snapshot sharing and product codes can change outside Karpenter, so these are recomputed on every EC2NodeClass AMI status reconcile (the [AMI controller](https://github.com/aws/karpenter-provider-aws/blob/e9a7e995/pkg/controllers/nodeclass/ami.go#L97) requeues every 5 minutes). They are not checked again before each swap: if something changed outside Karpenter, EC2 rejects the task and the reboot fails.
   - Node level, per NodeClaim: the checks listed under Proposed Spec, against the target AMI and the running instance.
5. **`Reboot`.** When the NodeClaim's `DisruptionReason` is `Drifted`, `Reboot` hands off to a root volume replacement provider. It makes every `CreateReplaceRootVolumeTask` and `DescribeReplaceRootVolumeTasks` call and keeps the latest task per instance. It looks up the instance's tasks with `DescribeReplaceRootVolumeTasks` (filter `instance-id`). If a task is `pending` or `in-progress`, it waits for that task instead of creating another. Otherwise it calls `CreateReplaceRootVolumeTask` with the AMI resolved from the EC2NodeClass, `DeleteReplacedRootVolume=true` so swaps do not leak the original root volume, and a `ClientToken` derived from `operationID`. Core's `operationID` is about 93 characters (`<NodeClaim UID>-<request time in ns>-<boot ID>`) and EC2 allows 64, so the provider passes its SHA-256 hex digest, which is exactly 64 characters and the same on every retry. The task lookup, not the `ClientToken`, is what finds an existing task: EC2 does not always reject a reused token whose parameters changed, for example when the EC2NodeClass AMI drifts again between retries. If EC2 rejects the task, `Reboot` returns `NodeRebootFailedError`, so core fails the reboot right away and deletes the NodeClaim. Any other reboot stays `RebootInstances`.
6. **Keep `Status.ImageID` current.** Core does not read the AMI back after the reboot; it calls `GetDriftStatus` on its next drift pass. A controller watches NodeClaims that are rebooting with `DisruptionReason=Drifted`, or rebooted that way until their task finishes. When the task succeeds, it patches `status.imageID` to the task's AMI, which is also what `DescribeInstances` reports after the swap. The existing AMI drift check, and everything else that reads `status.imageID` (such as [instance type resolution](https://github.com/aws/karpenter-provider-aws/blob/e9a7e995/pkg/providers/instancetype/instancetype.go#L526)), then sees the AMI the node really runs. `GetDriftStatus` reads the same task records. A `failed` or `failed-detached` task returns `Replace`. A task that has not finished returns `Reboot`, so the NodeClaim stays `Drifted`: core does not commit a node that is already rebooting, and a repeated `Reboot` call joins the running task.

### IAM

The controller policy gains one write action and two reads.
The write is scoped like `RebootInstances` in `AllowScopedDeletion`: the instance must carry the cluster and NodePool tags.
The root volume and the task are new resources created by the call, so they cannot carry a tag condition.
The image statement allows any AMI, matching `RunInstances` in `AllowScopedEC2InstanceAccessActions`; operators can narrow it with `ec2:Owner`.

```json
{
  "Sid": "AllowScopedReplaceRootVolume",
  "Effect": "Allow",
  "Resource": "arn:${AWS::Partition}:ec2:${AWS::Region}:*:instance/*",
  "Action": "ec2:CreateReplaceRootVolumeTask",
  "Condition": {
    "StringEquals": {
      "aws:ResourceTag/kubernetes.io/cluster/${ClusterName}": "owned"
    },
    "StringLike": {
      "aws:ResourceTag/karpenter.sh/nodepool": "*"
    }
  }
},
{
  "Sid": "AllowReplaceRootVolumeResources",
  "Effect": "Allow",
  "Resource": [
    "arn:${AWS::Partition}:ec2:${AWS::Region}::image/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}:${AWS::AccountId}:volume/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}:${AWS::AccountId}:replace-root-volume-task/*"
  ],
  "Action": "ec2:CreateReplaceRootVolumeTask"
}
```

`ec2:DescribeReplaceRootVolumeTasks` (confirming the swap) and `ec2:DescribeSnapshots` (the snapshot gate) join `AllowRegionalReadActions`.
Whether `DeleteReplacedRootVolume=true` also needs `ec2:DeleteVolume` is not verified yet. Encrypted root volumes need the KMS permissions EC2 requires to detach them.

### Interaction with Existing Features

- **Instance store and data volumes.** Only the root volume is replaced. Instance-store and attached data EBS volumes are untouched.
- **Capacity reservations.** The instance keeps its reservation; nothing is relaunched.
- **Repair.** Repair reboots keep calling `RebootInstances`, so they never spend the `Unhealthy` budget on drift. A repaired node that still has AMI drift swaps its root volume on a later drift pass, under the `Drifted` budget.
- **Cross-account AMIs.** The snapshot gate makes snapshot sharing a prerequisite for organizations that publish AMIs from a central account. Launch permission on the AMI is not enough.
- **AL2023 with `instanceStorePolicy: RAID0`.** On older AL2023 AMIs the instance-store RAID0 array is not reassembled after the root volume swap, so the node does not rejoin the cluster ([amazon-eks-ami#2867](https://github.com/awslabs/amazon-eks-ami/issues/2867)). [amazon-eks-ami#2869](https://github.com/awslabs/amazon-eks-ami/pull/2869) fixes it by reusing the existing array. This is not a gate. The troubleshooting docs name the first AMI release with the fix and give this user data mitigation for older AMIs:

  ```bash
  #cloud-boothook
  #!/bin/bash
  set -euo pipefail
  MD_CONFIG=/.aws/mdadm.conf
  if mdadm --detail --scan 2>/dev/null | grep -q "ARRAY" && [ ! -s "$MD_CONFIG" ]; then
    mkdir -p "$(dirname "$MD_CONFIG")"
    mdadm --detail --scan > "$MD_CONFIG"
  fi
  ```

  Without it, a node that does not rejoin never becomes `Ready`, and core deletes the NodeClaim when its observation window ends.

### Observability

- `status.amis[].inPlaceUpgrade` for AMI-level eligibility, with the failed gate as `reason`.
- A NodeClaim event when AMI-only drift on a `ReplaceRootVolume` EC2NodeClass falls back to replacing the instance, naming the gate that failed (for example `SnapshotNotReadable`).
- A counter of root volume replacement tasks by outcome (`succeeded`, `failed`, `failed-detached`), so operators can alert on failed swaps.
- Swap duration comes from core's existing reboot histograms, `karpenter_nodes_reboot_duration_seconds` and `karpenter_nodes_reboot_recovery_duration_seconds` ([metrics.go](https://github.com/kubernetes-sigs/karpenter/blob/dbc3490/pkg/controllers/nodeclaim/reboot/metrics.go#L64-L89)), which cover every reboot. [In-Place Drift Remediation](https://github.com/kubernetes-sigs/karpenter/pull/3460) adds a disruption reason label so drift swaps can be told apart from repair reboots, which gives Beta the duration data it needs.

### Edge Cases

- **AMI and subnet drift at once.** AMI is not the only drift, so the node is replaced.
- **Snapshot not shared to the node's account.** The gate fails and the node is replaced. Operators fix sharing and later rollouts go in place.
- **Controller restart mid-swap.** The restarted controller finds the `pending` or `in-progress` task with `DescribeReplaceRootVolumeTasks` and waits for it instead of creating a second one.
- **AMI drifts again during a swap.** The running task finishes on the older AMI and `status.imageID` is updated to it. The next drift pass sees drift to the newer AMI and starts a new reboot with a new `operationID`.
- **Task fails and the instance boots the old AMI.** The reboot completes, the next `GetDriftStatus` sees the `failed` task and returns `Replace`, and core replaces the node through its normal drift path.
- **Task fails before the instance reboots.** If EC2 rejects the task, the reboot fails right away. Otherwise no new boot appears, and core deletes the NodeClaim when its observation window ends.
- **Task ends `failed-detached`.** The instance has no root volume and never comes back, so core deletes the NodeClaim when its observation window ends and the instance is terminated. `DeleteReplacedRootVolume` only deletes the original volume after a successful swap, so volumes a failed task leaves behind are an open question.

## Alternatives Considered

### Stop, update user data and AMI, start

Rejected because stopping an instance discards instance-store data, which is the data this feature exists to keep.

### Operator tooling outside Karpenter

Scripts that call `CreateReplaceRootVolumeTask` directly.
Rejected because they bypass disruption budgets, drains and drift state, and race Karpenter's own drift.

## Backward Compatibility

- `amiDriftPolicy` defaults to `ReplaceInstance` and is excluded from the drift hash. Upgrading changes nothing.
- Opting in needs the IAM above. Without it, `CreateReplaceRootVolumeTask` is denied, the reboot fails, and core deletes the NodeClaim.

## Graduation Criteria

- **Alpha:** opt-in through `amiDriftPolicy` with no feature gate, AWS as the first provider. Proves the gates on real fleets.
- **Beta:** after production data on swap duration across instance types. `amiDriftPolicy` keeps defaulting to `ReplaceInstance`, because launching a replacement before terminating is faster, so in-place stays opt-in per EC2NodeClass for nodes whose instance-store data must survive.

## Open Questions

1. Shape of the per-AMI result: a nested `inPlaceUpgrade` object, or flat `inPlaceUpgradeSupported` and `inPlaceUpgradeReason` fields on each `status.amis` entry.
2. Root volume replacements typically take 2 to 10 minutes. Is core's 20 minute observation window right for them?
3. Which volumes does a `failed` or `failed-detached` task leave behind, and should the provider delete them?
