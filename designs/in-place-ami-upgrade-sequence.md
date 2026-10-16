# In-Place AMI Upgrade: core / provider sequencing

```mermaid
sequenceDiagram
    autonumber
    participant Core as Core (drift method + reboot controller)
    participant CP as Provider (AWS)
    participant EC2
    Core->>CP: IsDrifted(nc)
    CP-->>Core: AMIDrift
    Core->>CP: DriftRemediation(nc, AMIDrift)
    CP->>EC2: CRRVT DryRun with SnapshotId (cached)
    CP-->>Core: RebootNode
    Core->>Core: commitReboot (Rebooting=Requested, DisruptionReason=Drifted)
    Core->>Core: fence + drain
    Core->>CP: Reboot(nc, opID)
    CP->>EC2: CreateReplaceRootVolumeTask(ImageId, ClientToken)
    loop every 15s within 20m window
        Core->>CP: RebootStatus(nc, opID)
        CP->>EC2: DescribeReplaceRootVolumeTasks, then DescribeInstances (fresh)
        CP-->>Core: InProgress / Failed / Succeeded
    end
    Core->>CP: Get(providerID)
    CP-->>Core: observed ImageID
    Core->>Core: RebootSucceeded + ImageID, hold budget until Initialized
    Core->>CP: IsDrifted(nc)
    CP-->>Core: not drifted, Drifted cleared
```

## More in depth

### 1. DriftRemediation
Core only sees "AMIDrift". It can't know whether an in-place swap is safe for this node: hidden SG drift, a family or version change, a RAID0 layout, or an unreadable snapshot can all make it unsafe. It has to decide before committing, because committing drains the node. So the provider answers that question up front, and anything unsafe falls back to today's replace.

### 2. Reboot
`Reboot()` carries no "which action" field, so the provider works it out from the NodeClaim it's handed. `DisruptionReason` says who committed the reboot: `Drifted` (drift) or `Unhealthy` (repair). The `Drifted` condition and the NodeClass say whether this node is AMI-drifted and opted in. The provider then re-runs the same gates `DriftRemediation` used:
- `Drifted` and the gates pass: `CreateReplaceRootVolumeTask`.
- `Unhealthy` but also AMI-drifted and eligible: `CreateReplaceRootVolumeTask`, which fixes the fault and the drift in one go.
- Anything else: `RebootInstances`.

This works because `DisruptionReason` stays on the NodeClaim for the whole reboot, so no new annotation or interface field is needed.

### 3. RebootStatus loop
A new boot ID plus Ready only proves the node restarted, not that the swap took effect. A failed task that reboots on the old root looks identical, and so does EC2 not having caught up yet. Only the provider's task state can tell these apart, so core waits for the provider to confirm, and fails fast when the provider reports a failure instead of waiting out 20 minutes.

### 4. Get
Core writes `Status.ImageID` only at launch, so without a refresh the node stays drifted forever. Reading it back from the provider after confirmation records the image the node is actually running, not the one we intended. That's what lets drift clear honestly and avoids false patch compliance.
