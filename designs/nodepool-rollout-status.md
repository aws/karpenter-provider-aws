# NodePool and EC2NodeClass rollout status

This is the AWS provider half of [NodePool and NodeClass rollout status (`kubernetes-sigs/karpenter#3216`)](https://github.com/kubernetes-sigs/karpenter/pull/3216). Karpenter core defines the NodeClaim and NodePool API shapes. Here we cover the `EC2NodeClass` status changes, the `NodeRolloutStatus` feature gate, AWS drift evaluation, and the docs updates.

## Motivation

See [NodePool and NodeClass rollout status#Motivation](https://github.com/kubernetes-sigs/karpenter/blob/3e934e80c52a3fc8472081ab3734f9559fac6401/designs/nodepool-rollout-status.md#motivation)

### Use cases

1. AMI and launch template rollouts in argo. A team pins a new AMI in `EC2NodeClass.spec.amiSelectorTerms` or edits `spec.userData`. An argo health check on `karpenter.k8s.aws/v1` `EC2NodeClass` reports `Progressing` while Karpenter replaces nodes across referencing `NodePool`s, `Degraded` if the `EC2NodeClass` fails validation or selector discovery (`RolloutBlocked`), and `Healthy` once the rollout crosses the configured threshold.
2. Shared `EC2NodeClass` across multiple `NodePool`s. Several `NodePool`s point to one `EC2NodeClass`. Updating the `EC2NodeClass` rolls nodes in every referencing `NodePool`, and `EC2NodeClass.status` reports aggregate progress on the object that changed.

### Non-goals

- Counting out-of-band AWS changes as spec rollouts (`@latest` aliases resolving to a new AMI, SSM parameter updates in AWS Systems Manager, or EC2 tag edits outside kubernetes). None of these bump `EC2NodeClass.metadata.generation`. Status tracks whether the applied `EC2NodeClass` spec has rolled out, while prometheus metrics cover fleet-wide drift across all causes
- Changing AWS drift detection or disruption behavior

## Proposal

### Feature gate

Rollout status is behind the alpha core feature gate `NodeRolloutStatus` (default `false`), enabled via `--feature-gates NodeRolloutStatus=true` or `FEATURE_GATES=NodeRolloutStatus=true`.

When `NodeRolloutStatus=true`:

- Core writes the `NodeClaim` generation stamps and reconciles `NodeRolloutStatus` + `NodesUpToDate` on both `NodePool` and `EC2NodeClass`
- `NodePool.status.nodes` and `EC2NodeClass.status.nodes` count all associated `NodeClaim`s, including unlaunched and terminating `NodeClaim`s

### `EC2NodeClass` status API

`EC2NodeClassStatus` in `pkg/apis/v1/ec2nodeclass_status.go` embeds `karpv1.NodeRolloutStatus` with `json:",inline"` and implements `NodeClassWithRolloutStatus`.

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: default
  generation: 5
status:
  observedGeneration: 5           # NEW: the EC2NodeClass generation the counts below were computed against
  nodes: 21                       # NEW: NodeClaims whose spec.nodeClassRef points to this EC2NodeClass
  upToDateNodes: 14               # NEW: of those, compatibleWithNodeClassGeneration == metadata.generation
  readyNodes: 18                  # NEW: of those, whose NodeClaim Ready condition is True
  upToDateAndReadyNodes: 12       # NEW: of those, counted by both upToDateNodes and readyNodes
  amis: [...]
  subnets: [...]
  securityGroups: [...]
  instanceProfile: "..."
  conditions:
    - type: NodesUpToDate         # NEW: informational, not part of the Ready condition set
      status: "False"
      reason: RolloutInProgress   # or RolloutBlocked when EC2NodeClass Ready is False
      message: 14/21 nodes are up to date
      observedGeneration: 5
...
```

When `upToDateNodes < nodes` and `EC2NodeClass` `Ready` is `False`, `nodeclass.counter` sets `NodesUpToDate=False` w/ reason `RolloutBlocked` and copies the readiness failure message. This covers cases like `AMIsReady=False` (when an AMI selector matches nothing) or `ValidationSucceeded=False` (when a kubelet setting fails validation).

### Disruption and `EC2NodeClass` drift evaluation

When `NodeRolloutStatus` is enabled, `nodeclaim.disruption` calls `cloudProvider.IsDrifted(ctx, nodeClaim)` on every reconcile pass, even if the `NodeClaim` is already drifted on the `NodePool` axis. If `cloudProvider.IsDrifted` returns `""`, `nodeclaim.disruption` sets `status.compatibleWithNodeClassGeneration` on the `NodeClaim` to `EC2NodeClass.metadata.generation`. Otherwise (a non-empty `DriftReason` or an error), it leaves `compatibleWithNodeClassGeneration` alone.

In `pkg/cloudprovider/drift.go`, `isNodeClassDrifted` runs six checks against `EC2NodeClass`:

1. `areStaticFieldsDrifted` -> `NodeClassDrift` when the `karpenter.k8s.aws/ec2nodeclass-hash` annotation on the `NodeClaim` differs from the `EC2NodeClass` hash (`userData`, `kubelet`, `tags`, `blockDeviceMappings`, `metadataOptions`, resolved `amiFamily`, `role`, `instanceProfile`, `instanceStorePolicy`, `networkInterfaces`, `detailedMonitoring`, `associatePublicIPAddress`, `enclaveOptions`, `connectionTracking`, `cpuOptions`, `placementGroupSelector`, and `context`)
2. `isAMIDrifted` -> `AMIDrift` when `nodeClaim.Status.ImageID` is not in the `nodeClass.Status.AMIs` entries matching the node's instance type
3. `areSecurityGroupsDrifted` -> `SecurityGroupDrift` when the EC2 instance's security group IDs do not match `nodeClass.Status.SecurityGroups`
4. `isSubnetDrifted` -> `SubnetDrift` when the EC2 instance's `SubnetID` is not in `nodeClass.Status.Subnets`
5. `isCapacityReservationDrifted` -> `CapacityReservationDrift` when an instance in a capacity reservation has a reservation ID that is not in `nodeClass.Status.CapacityReservations`
6. `isPlacementGroupDrifted` -> `PlacementGroupDrift` when `nodeClaim.Labels["karpenter.k8s.aws/placement-group-id"]` does not match the ID resolved from `nodeClass.Spec.PlacementGroupSelector`

How each `EC2NodeClass` change or AWS drift event affects `EC2NodeClass.metadata.generation`, `compatibleWithNodeClassGeneration`, and the rollout gate:

| Change or event | Drift reason from `IsDrifted` | `EC2NodeClass.metadata.generation` bumps? | `compatibleWithNodeClassGeneration` advances? | `EC2NodeClass` rollout gate waits? |
|---|---|---|---|---|
| Static `EC2NodeClass.spec` edit (`userData`, `kubelet`, `tags`, `blockDeviceMappings`, `metadataOptions`, `role`, etc) | `NodeClassDrift` | Yes (`C` -> `C+1`) | No (stays at `C`) | Yes, until replacement nodes launch w/ `C+1` |
| `amiSelectorTerms` edit that selects a new AMI for the node's instance type | `AMIDrift` | Yes (`C` -> `C+1`) | No (stays at `C`) | Yes |
| `amiSelectorTerms` edit where the node's `status.imageID` stays selected | `""` | Yes (`C` -> `C+1`) | Yes (`C+1`) | No, existing node still matches `C+1` |
| Out-of-band AMI update (`alias: al2023@latest`, `ssmParameter`, or EC2 tag change w/ no `EC2NodeClass.spec` edit) | `AMIDrift` | No (stays at `C`) | No, but stamp is already `C` | No (`upToDateNodes == nodes`, `Drifted` condition + metrics still report `AMIDrift`) |
| `subnetSelectorTerms` or `capacityReservationSelectorTerms` widened so the instance's subnet or reservation stays selected | `""` | Yes (`C` -> `C+1`) | Yes (`C+1`) | No |
| `subnetSelectorTerms` or `capacityReservationSelectorTerms` narrowed so the instance's subnet or reservation is removed | `SubnetDrift` or `CapacityReservationDrift` | Yes (`C` -> `C+1`) | No (stays at `C`) | Yes |
| `securityGroupSelectorTerms` edit that changes resolved `status.securityGroups` | `SecurityGroupDrift` | Yes (`C` -> `C+1`) | No (stays at `C`) | Yes |
| `securityGroupSelectorTerms` edit that resolves to the same security group IDs | `""` | Yes (`C` -> `C+1`) | Yes (`C+1`) | No |
| `placementGroupSelector` edit that changes the resolved placement group | `NodeClassDrift` / `PlacementGroupDrift` | Yes (`C` -> `C+1`) | No (stays at `C`) | Yes |
| Invalid `EC2NodeClass.spec` edit (`AMIsReady=False`, `SubnetsReady=False`, `ValidationSucceeded=False`, etc) | Error or drift reason | Yes (`C` -> `C+1`) | No (stays at `C` for affected nodes) | Yes, `NodesUpToDate=False` w/ reason `RolloutBlocked` |

### Website documentation updates

The implementation PR will update `website/content/en/preview/`:

1. `reference/settings.md` - run `make docgen` to add `NodeRolloutStatus` to `FEATURE_GATES` / `--feature-gates`, and add a row to the Feature Gates table (`Default: false`, `Stage: Alpha`, `Since: v1.16.x`)
2. `concepts/disruption.md` - add a `Rollout Status` subsection under `### Drift`:
   - How to enable the alpha `NodeRolloutStatus` feature gate (`--feature-gates NodeRolloutStatus=true` or helm `settings.featureGates.nodeRolloutStatus: true`)
   - How `status.compatibleWithNodePoolGeneration` and `status.compatibleWithNodeClassGeneration` on `NodeClaim`s drive `observedGeneration`, `nodes`, `upToDateNodes`, `readyNodes`, `upToDateAndReadyNodes`, and the `NodesUpToDate` condition (`RolloutInProgress` / `RolloutBlocked`) on `NodePool` and `EC2NodeClass`
   - Which `NodePool` and `EC2NodeClass` changes keep `upToDateNodes < nodes`, which advance the generation stamp without replacement, and why out-of-band AMI updates (e.g `al2023@latest`) do not change `EC2NodeClass` rollout counts when `metadata.generation` is unchanged
   - Why `status.nodes` includes unlaunched and terminating `NodeClaim`s when `NodeRolloutStatus` is enabled
   - The argo Lua health check for `karpenter.sh/v1` `NodePool` and `karpenter.k8s.aws/v1` `EC2NodeClass`
3. `concepts/nodepools.md`, `concepts/nodeclasses.md`, and `concepts/nodeclaims.md` - document the new status fields and `NodesUpToDate` condition
4. `upgrading/upgrade-guide.md` - note the opt-in alpha `NodeRolloutStatus` gate, the CRD upgrade requirement, and the `NodePool.status.nodes` change when enabled

## Alternatives considered

### Separate AWS-specific feature gate (`--aws-feature-gates`)

`NodeClaim` stamps, `nodepool.counter`, and `nodeclass.counter` all live in core. Using a single `NodeRolloutStatus` gate in `--feature-gates` turns the feature on or off across `NodeClaim`, `NodePool`, and `EC2NodeClass` without making users set two flags.

## Backward compatibility

All new fields on `EC2NodeClass.status` (`observedGeneration`, `nodes`, `upToDateNodes`, `readyNodes`, `upToDateAndReadyNodes`) and the `NodesUpToDate` condition are additive and read-only. Existing `EC2NodeClass` manifests do not need changes, though users must apply the updated `ec2nodeclasses.karpenter.k8s.aws` CRD when upgrading.

When `NodeRolloutStatus=false` (the default in Alpha), Karpenter does not write the rollout fields or condition, and `NodePool.status.nodes` keeps its existing cluster-state definition.

## Graduation criteria

- Alpha (`NodeRolloutStatus=false` by default): ships `NodeRolloutStatus` in `EC2NodeClassStatus`, `NodeClassWithRolloutStatus` on `EC2NodeClass`, the `IsDrifted` generation checks, helm chart support (`settings.featureGates.nodeRolloutStatus`), and docs in `concepts/disruption.md` + `reference/settings.md`
- Beta (`NodeRolloutStatus=true` by default): requires scale testing of API server status-patch load during `EC2NodeClass` AMI rollouts and feedback on the `status.nodes` denominator change
- GA (gate removed): follows core's `NodeRolloutStatus` graduation after at least one minor release in Beta

## References

- Core RFC: [kubernetes-sigs/karpenter#3216](https://github.com/kubernetes-sigs/karpenter/pull/3216)
- Tracking issue: [kubernetes-sigs/karpenter#3071](https://github.com/kubernetes-sigs/karpenter/issues/3071)
- AWS drift implementation: [`pkg/cloudprovider/drift.go`](../pkg/cloudprovider/drift.go)
- `EC2NodeClass` status definition: [`pkg/apis/v1/ec2nodeclass_status.go`](../pkg/apis/v1/ec2nodeclass_status.go)
