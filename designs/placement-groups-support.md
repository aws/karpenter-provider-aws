# Placement Groups Support

This document proposes supporting placement groups in Karpenter

- [Placement Groups Support](#placement-groups-support)
    * [Overview](#overview)
        + [Placement Groups](#placement-groups)
        + [Placement Group Placement Strategies](#placement-group-placement-strategies)
    * [Customer Use Cases](#customer-use-cases)
        + [ML Training with EFA (Cluster Placement Group)](#ml-training-with-efa-cluster-placement-group)
        + [Kafka with Partition Isolation (Partition Placement Group)](#kafka-with-partition-isolation-partition-placement-group)
        + [ETCD with Hardware Spread (Spread Placement Group)](#etcd-with-hardware-spread-spread-placement-group)
    * [Goals](#goals)
    * [Non-Goals](#non-goals)
    * [Placement Group Selection](#placement-group-selection)
        + [EC2NodeClass API](#ec2nodeclass-api)
        + [Labels](#labels)
        + [NodePool API](#nodepool-api)
    * [Scheduling and Launch Behavior](#scheduling-and-launch-behavior)
        + [Strategy-Specific Behavior](#strategy-specific-behavior)
        + [Partition Label Assignment via Registration Hook](#partition-label-assignment-via-registration-hook)
        + [Placement Groups as Constraints on Instance Type Offerings](#placement-groups-as-constraints-on-instance-type-offerings)
        + [Interaction with Capacity Reservations (ODCRs)](#interaction-with-capacity-reservations-odcrs)
    * [Placement Group-Aware ICE Cache](#placement-group-aware-ice-cache)
    * [Pricing/Consolidation](#pricingconsolidation)
    * [Drift](#drift)
    * [Spread Placement Group Disruption Limitations](#spread-placement-group-disruption-limitations)
    * [Appendix](#appendix)
        + [Input/Output for CreateFleet with Placement Groups](#inputoutput-for-createfleet-with-placement-groups)
        + [Strategy-Specific Limitations](#strategy-specific-limitations)

## Overview

In AWS, [Placement Groups](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/placement-groups.html) allow users to launch a group of interdependent EC2 instances with an influence on placement that the workload requires.

This RFC outlines the proposed API and implementation of support for Placement Groups within Karpenter. This feature will allow users to select a single placement group through `placementGroupSelector` in their EC2NodeClass. Karpenter will then discover and use this placement group during scheduling and disruption (including consolidation) simulations to ensure instances are launched respecting the placement constraints.

**Key design principle:** Each EC2NodeClass maps to exactly one placement group. All instances launched from that EC2NodeClass go into the resolved placement group — this is not conditional on application topology requirements.

### Placement Groups

Each [Placement Group](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ec2/types#PlacementGroup) is defined with:

- The placement strategy for which to launch instances
  - [Cluster](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/placement-strategies.html#placement-groups-cluster) -- logical grouping of instances within a single Availability Zone that enjoy a higher per-flow throughput limit for TCP/IP traffic and are placed in the same high-bisection bandwidth segment of the network
  - [Partition](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/placement-strategies.html#placement-groups-partition) -- multiple logical groupings of instances across one or more Availability Zones called partitions where no two partitions within the placement group share the same racks, allowing you to isolate the impact of hardware failure within your application
  - [Spread](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/placement-strategies.html#placement-groups-spread) -- logical grouping of instances across a single Region with access to distinct hardware to minimize the risk of simultaneous failures that might occur when instances share the same equipment
- The partition count for partition strategy placement groups
- The spread level for spread strategy placement groups

Placement Groups also have these limitations:

- You can't launch Dedicated Hosts in placement groups.
- You can't launch a Spot Instance that is configured to stop or hibernate on interruption in a placement group. Since Karpenter always uses the `terminate` interruption behavior, spot instances are fully compatible with all placement group strategies.

### Placement Group Placement Strategies

Currently, [Placement Groups](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/placement-groups.html) supports three placement strategies -- cluster, partition, and spread. Each strategy has its own sets of limitations that are relevant for scheduling. See also [Strategy-Specific Limitations](#strategy-specific-limitations) in the Appendix for a summary table.

- [Cluster Placement Groups](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/placement-strategies.html#placement-groups-cluster)
  - A cluster placement group can't span multiple Availability Zones
  - Recommended to use a single launch request to launch the number of instances that you need in the placement group and to use the same instance type for all instances in the placement group. (Note: this limitation can be mitigated by creating an On-Demand Capacity Reservation linked to the cluster placement group, which reserves capacity and avoids incremental ICE risk when adding instances over time.)
  - If you try to add more instances to the placement group later, or if you try to launch more than one instance type in the placement group, you increase your chances of getting an insufficient capacity error.
  - If you receive a capacity error when launching an instance in a placement group that already has running instances, stop and start all of the instances in the placement group, and try the launch again. Starting the instances may migrate them to hardware that has capacity for all of the requested instances.
  - There is an instance type restriction: only the following are supported — previous generation instances (A1, C3, C4, I2, M4, R3, and R4) and current generation instances, except for [burstable performance](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/burstable-performance-instances.html) instances (for example, T2), [Mac1](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-mac-instances.html) instances, and M7i-flex instances.
- [Partition Placement Groups](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/placement-strategies.html#placement-groups-partition)
  - A partition placement group can have a maximum of seven partitions per Availability Zone.
  - When instances are launched into a partition placement group, Amazon EC2 tries to evenly distribute the instances across all partitions. Amazon EC2 doesn't guarantee an even distribution of instances across all partitions.
  - A partition placement group with Dedicated Instances can have a maximum of two partitions.
  - Capacity Reservations do not reserve capacity in a partition placement group.
- [Spread Placement Groups](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/placement-strategies.html#placement-groups-spread)
  - There are two types of spread placement groups: rack-level spread placement groups (available in AWS Regions and on AWS Outposts) and host-level spread placement groups (available on AWS Outposts only).
  - A rack-level spread placement group can span multiple Availability Zones in the same Region.
  - In a Region, a rack-level spread placement group can have a maximum of seven running instances per Availability Zone per group.
  - If you need more than seven instances in an Availability Zone, we recommend that you use multiple spread placement groups.
  - Using multiple spread placement groups does not provide guarantees about the spread of instances between groups, but it does help ensure the spread for each group, thus limiting the impact from certain classes of failures.
  - Spread placement groups are not supported for Dedicated Instances.
  - Capacity Reservations do not reserve capacity in a spread placement group.

## Customer Use Cases

The following use cases motivate the design decisions in this RFC.

### ML Training with EFA (Cluster Placement Group)

Distributed ML training workloads require low-latency, high-throughput networking between GPU nodes. A cluster placement group ensures all training nodes are physically colocated in the same AZ on the same network segment, which is critical for EFA (Elastic Fabric Adapter) performance.

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: ml-training
spec:
  placementGroupSelector:
    name: "ml-training-pg"
  role: "KarpenterNodeRole-my-cluster"
  amiSelectorTerms:
    - alias: al2023@latest
  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: "my-cluster"
  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: "my-cluster"
---
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: ml-training
spec:
  template:
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: ml-training
      requirements:
        - key: node.kubernetes.io/instance-type
          operator: In
          values: ["p5.48xlarge", "p5e.48xlarge"]
        - key: karpenter.sh/capacity-type
          operator: In
          values: ["on-demand"]
        - key: topology.kubernetes.io/zone
          operator: In
          values: ["us-east-1a"]
  disruption:
    consolidationPolicy: WhenEmpty
    consolidateAfter: 1h
  limits:
    cpu: "1000"
    nvidia.com/gpu: "64"
```

### Kafka with Partition Isolation (Partition Placement Group)

Kafka brokers benefit from partition placement groups to achieve hardware fault isolation between brokers. By using TSCs with the `karpenter.k8s.aws/placement-group-partition` label, brokers are spread across different partitions, ensuring no two brokers share the same underlying rack.

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: kafka-brokers
spec:
  placementGroupSelector:
    name: "kafka-partitioned-pg"
  ...
  blockDeviceMappings:
    - deviceName: /dev/xvda
      ebs:
        volumeSize: 100Gi
        volumeType: gp3
---
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: kafka-brokers
spec:
  template:
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: kafka-brokers
      requirements:
        - key: node.kubernetes.io/instance-type
          operator: In
          values: ["i4i.8xlarge", "i8g.8xlarge"]
        - key: karpenter.sh/capacity-type
          operator: In
          values: ["on-demand"]
      taints:
        - key: workload-type
          value: kafka
          effect: NoSchedule
  disruption:
    consolidationPolicy: WhenEmpty
    consolidateAfter: Never
  limits:
    cpu: "200"
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: kafka
spec:
  replicas: 7
  template:
    spec:
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: karpenter.k8s.aws/placement-group-partition
          whenUnsatisfiable: DoNotSchedule
          labelSelector:
            matchLabels:
              app: kafka
      tolerations:
        - key: workload-type
          value: kafka
          effect: NoSchedule
```

### ETCD with Hardware Spread (Spread Placement Group)

ETCD clusters require high availability with each member on distinct hardware to minimize the risk of correlated failures. A spread placement group ensures each ETCD node is placed on a separate rack, providing hardware-level fault isolation.

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: etcd
spec:
  placementGroupSelector:
    name: "etcd-spread-pg"
  ...
---
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: etcd
spec:
  template:
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: etcd
      requirements:
        - key: node.kubernetes.io/instance-type
          operator: In
          values: ["m7g.xlarge", "m7g.2xlarge"]
        - key: karpenter.sh/capacity-type
          operator: In
          values: ["on-demand"]
  disruption:
    consolidationPolicy: WhenEmpty
    consolidateAfter: Never
  limits:
    cpu: "40"
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: etcd
spec:
  replicas: 5
  template:
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels:
                  app: etcd
              topologyKey: kubernetes.io/hostname
```

Since the spread placement group guarantees each instance is on distinct hardware, the `podAntiAffinity` on `kubernetes.io/hostname` ensures one ETCD member per node, and the placement group ensures those nodes are on separate racks.

## Goals

1. Allow selection of cluster Placement Groups with Karpenter
2. Allow selection of partition, rack-level spread, and host-level spread Placement Groups with Karpenter
3. Ensure Karpenter launches capacity into a Placement Group respecting strategy-specific constraints (e.g., single AZ for cluster, 7 instances per AZ for spread)
4. Ensure Karpenter consolidates respecting Placement Group constraints expressed through offerings and pod scheduling rules
5. Allow users to constrain a NodePool to only launch into a specified Placement Group

## Non-Goals

Below lists the non-goals for _this RFC design._ Each of these items represents potential follow-ups for the initial implementation and are features we will consider based on feature requests.

1. Create/Manage/Delete Placement Groups based on application topology requirements

## Placement Group Selection

### EC2NodeClass API

- Add a new struct under `spec` for `placementGroupSelector` to `EC2NodeClass` for defining which Placement Group to be used for a specific `EC2NodeClass`
  - Each EC2NodeClass maps to exactly one Placement Group
  - The struct accepts either a placement group name or id as a string value
- The resolved placement group details are stored **in-memory** by the placement group provider

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: example-node-class
spec:
  # placementGroupSelector specifies a placement group name or id to identify
  # a single Placement Group via the EC2 DescribePlacementGroups API.
  placementGroupSelector:
    name: String | None
    id: String | None
  # CEL validation ensures placementGroupSelector is not empty and is either
  # a placement group name or a placement group id (pg-* prefix) is populated
status:
  conditions:
    - # PlacementGroupReady indicates whether the placement group specified
      # by spec.placementGroupSelector has been successfully resolved.
      # The EC2NodeClass is not ready if this condition is False,
      # blocking all launches from this NodeClass.
      type: PlacementGroupReady
```

The placement group provider resolves placement groups from the EC2 `DescribePlacementGroups` API and stores the result in-memory, keyed by nodeclass name. The resolved placement group contains the following fields:

- **ID**: Placement group ID (e.g., `"pg-0123456789abcdef0"`)
- **Name**: Placement group name
- **PartitionCount**: Number of partitions (partition strategy only)
- **SpreadLevel**: Spread level (`"rack"` or `"host"`, spread strategy only)
- **Strategy**: Placement group strategy (`"cluster"`, `"partition"`, or `"spread"`)

All consumers (drift detection, offering resolution, launch template creation, etc.) read from the provider's in-memory store. The nodeclass reconciler is responsible for calling the provider to resolve and store the placement group on each reconciliation loop, ensuring the in-memory state is always fresh.

### Labels

When Karpenter launches an instance into a placement group, it will apply the following well-known labels to the Node/NodeClaim:

| Label | Values | Description |
|-------|--------|-------------|
| `karpenter.k8s.aws/placement-group-id` | Placement group ID (e.g., `"pg-0123456789abcdef0"`) | Uniquely identifies the placement group the node belongs to. Used as the primary key for offering requirements and ICE cache scoping. |
| `karpenter.k8s.aws/placement-group-partition` | Partition number as a string (e.g., `"2"`) | The partition number (**partition strategy only**) |

These labels serve multiple purposes:

1. **Pod Scheduling** -- Applications can use node selectors or node affinities on these labels to ensure pods land on nodes in specific placement groups or partitions
2. **Drift Detection** -- Karpenter uses these labels to detect when a node's placement group membership has changed relative to the EC2NodeClass's `placementGroupSelector`

Example Node labels for an instance in a partition placement group:

```yaml
metadata:
  labels:
    karpenter.k8s.aws/placement-group-id: "pg-0123456789abcdef0"
    karpenter.k8s.aws/placement-group-partition: "2"       # partition strategy only
```

### NodePool API

The EC2NodeClass determines the placement group; the NodePool expresses additional constraints via `requirements` using the labels defined above (e.g., constraining to a specific partition).

**Cluster** — pin the AZ since cluster PGs are single-AZ:
```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: cluster-placement
spec:
  template:
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: cluster-placement-node-class
```

**Partition** — constrain instance types; use TSCs in workloads to spread across partitions:
```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: partition-placement
spec:
  template:
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: partition-placement-node-class
      requirements:
        - key: node.kubernetes.io/instance-type
          operator: In
          values: ["i4i.8xlarge", "i8g.8xlarge"]
        - key: karpenter.sh/capacity-type
          operator: In
          values: ["on-demand"]
```

An application can then use TSCs to spread across partitions:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: kafka
spec:
  replicas: 7
  template:
    spec:
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: karpenter.k8s.aws/placement-group-partition
          whenUnsatisfiable: DoNotSchedule
          labelSelector:
            matchLabels:
              app: kafka
```

**Spread** — no additional requirements needed; EC2 enforces the 7-instance-per-AZ limit and Karpenter handles the resulting errors:
```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: spread-placement
spec:
  template:
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: spread-placement-node-class
      # No additional requirements needed -- EC2 enforces
      # the 7-instance-per-AZ limit; Karpenter handles the errors
      requirements: []
```

## Scheduling and Launch Behavior

Since Placement Groups are an AWS-specific concept, there needs to be a mechanism to pass down these placement constraints for the scheduler to reason about. The placement group ID is specified in the launch template's `Placement` block when calling `CreateFleet`. Karpenter will use the `lowest-price` allocation strategy for on-demand instances and the `price-capacity-optimized` allocation strategy for spot instances, consistent with current behavior. The placement group constraint is applied via the launch template rather than through the allocation strategy.

### Strategy-Specific Behavior

Each placement group strategy constrains the scheduler and `CreateFleet` call differently:

| Strategy | AZ Behavior | Targeting | Capacity Limit | ICE Handling |
|----------|------------|-----------|----------------|--------------|
| **Cluster** | Single AZ. If PG is empty and NodePool allows multiple AZs, Karpenter passes all valid subnet overrides (across AZs) in the CreateFleet request; EC2's allocation strategy (`lowest-price` for OD, `price-capacity-optimized` for Spot) selects the AZ based on availability and price. If PG already has instances, Karpenter determines the existing AZ via `DescribeInstances` filtered by placement group (to account for instances launched outside of Karpenter) and constrains subsequent launches to that AZ. | N/A | No hard limit, but ICE risk increases when adding instances incrementally. Karpenter retries with alternative instance types. | ICE scoped to `(PG ID, instance type, zone)` — does not block non-PG launches of the same instance type + zone |
| **Partition** | Multi-AZ | Karpenter does not target a specific partition unless pod scheduling constraints require it (via node selector/affinity on `karpenter.k8s.aws/placement-group-partition`). EC2 auto-assigns partitions. The partition label is populated during node registration via a `RegistrationHook` (see [Partition Label Assignment via Registration Hook](#partition-label-assignment-via-registration-hook)). | 7 partitions per AZ. Capacity Reservations do not reserve capacity. | ICE scoped to `(PG ID, [partition,] instance type, zone)` — when a specific partition is targeted, only that partition is marked unavailable; other partitions remain eligible |
| **Spread** | Multi-AZ | N/A | **Hard limit: 7 instances per AZ per group**, enforced by EC2. When a `CreateFleet` call exceeds the limit, EC2 returns an `InsufficientInstanceCapacity` error (error code `UnfulfillableCapacity`) with the message `"You've reached the limit of instances in this spread placement group. A spread placement group can have up to seven instances per Availability Zone."` Karpenter parses this error message to distinguish it from genuine capacity shortages and marks the AZ as unavailable for that placement group in the ICE cache. Capacity Reservations do not reserve capacity. | Spread limit: all instance types in the AZ marked unavailable for this PG. Genuine capacity error: scoped to `(PG ID, instance type, zone)` |

**Note on cluster PG launches:** A cluster placement group is confined to a single AZ. Karpenter does not proactively discover the AZ of a non-empty cluster PG — instead, it relies on EC2 to enforce the single-AZ constraint. When a cluster PG is empty, all AZs are eligible and Karpenter passes all valid subnet overrides in the `CreateFleet` request; EC2 selects the AZ. During the initial scale-up, parallel `CreateFleet` calls may target different AZs. The first successful call pins the PG to an AZ, and calls targeting other AZs will fail. When the PG already has instances, EC2 automatically constrains new launches to the pinned AZ and returns `InsufficientInstanceCapacity` for overrides targeting other AZs. These failures are expected and handled as typical ICE — Karpenter retries with alternative instance types on subsequent scheduling loops. Users can avoid these transient failures by pinning the AZ in their NodePool requirements.

### Partition Label Assignment via Registration Hook

For partition placement groups, the `karpenter.k8s.aws/placement-group-partition` label cannot be determined until after launch, since EC2 auto-assigns the partition and the assignment is only discoverable via `DescribeInstances`. To ensure TopologySpreadConstraints using `karpenter.k8s.aws/placement-group-partition` as the topology key always see accurate partition data, Karpenter uses a `RegistrationHook` to gate node registration until the partition label is populated.

**How it works:**

1. When a NodeClaim is created for an EC2NodeClass with a partition placement group, the instance is launched via `CreateFleet` without specifying a `PartitionNumber`, allowing EC2 to auto-assign the partition.
2. During node registration, before the `karpenter.sh/unregistered` taint is removed, the `PlacementGroupRegistrationHook` runs as part of the NodeClaim lifecycle controller's registration phase.
3. The hook resolves the EC2NodeClass from the NodeClaim's `spec.nodeClassRef` and queries the placement group provider's in-memory store to determine if this is a partition placement group. If not, the hook passes through immediately.
4. For partition placement groups, the hook calls `DescribeInstances` to discover the EC2-assigned partition number from `Placement.PartitionNumber`.
5. Once the partition number is available, the hook sets the `karpenter.k8s.aws/placement-group-partition` label on the NodeClaim and allows registration to proceed. The label is then synced to the Node as part of the normal registration sync.
6. If the partition number is not yet available (e.g., the instance is still initializing), the hook returns `false`, causing the lifecycle controller to requeue after 1 second and retry.

This approach leverages the existing `karpenter.sh/unregistered` taint to block pod scheduling until the partition label is set, without requiring any additional startup taints. The hook is registered via `WithRegistrationHook()` in `cmd/controller/main.go` and is evaluated alongside any other registration hooks before the unregistered taint is removed.

### Placement Groups as Constraints on Instance Type Offerings

Unlike ODCRs which add additional offerings, placement groups primarily _constrain_ existing offerings. When an EC2NodeClass resolves a placement group, Karpenter filters instance type offerings based on the strategy and adds placement group labels as requirements on offerings so the scheduler can match them against NodePool/pod constraints. This is analogous to how `karpenter.sh/capacity-type: reserved` is used in the ODCR design.

**Strategy-specific filtering:**

- **Cluster**: Filter out unsupported instance types (burstable, Mac1, M7i-flex). If the PG already has instances, filter offerings to only the existing AZ.
- **Partition**: No AZ or instance type filtering. Offerings are expanded into per-partition variants (one per partition count) to support TopologySpreadConstraints on `karpenter.k8s.aws/placement-group-partition`. When a specific partition is targeted (via nodeSelector/affinity), the scheduler picks only offerings for that partition, and the partition number is passed through to the launch template's `Placement.PartitionNumber`.
- **Spread**: No proactive filtering during offering resolution. The 7-instance-per-AZ limit is enforced reactively by EC2 — when the limit is exceeded, EC2 returns an `InsufficientInstanceCapacity` error with a spread-specific message, and Karpenter marks the AZ as unavailable for that placement group in the ICE cache.

Example offering for a `p5.48xlarge` in a placement group:

```yaml
name: p5.48xlarge
offerings:
  - price: 98.32
    available: 4294967295
    requirements:
      - key: karpenter.sh/capacity-type
        operator: In
        values: ["on-demand"]
      - key: topology.kubernetes.io/zone
        operator: In
        values: ["us-east-1a"]
      - key: karpenter.k8s.aws/placement-group-id
        operator: In
        values: ["pg-dsf832nr1232"]
```

### Interaction with Capacity Reservations (ODCRs)

Placement groups and capacity reservations are orthogonal constraints applied at the EC2 launch template level — placement groups control physical placement via `Placement.GroupId`, while capacity reservations control capacity sourcing via `CapacityReservationSpecification`. Both can be specified simultaneously on a single launch.

When an EC2NodeClass specifies both `placementGroupSelector` and `capacityReservationSelectorTerms`, Karpenter's existing launch ordering naturally handles the interaction:

1. **Reserved capacity first**: The scheduler considers reserved offerings (ODCR ∩ NodePool requirements). If a match is found, CreateFleet targets both the capacity reservation and the placement group. The instance consumes the reserved capacity and is physically placed within the placement group.
2. **Fallback to on-demand/spot**: If no reserved capacity is available, the scheduler falls back to on-demand or spot offerings. The instance is launched into the placement group without a capacity reservation, with a broader set of instance types to choose from.

This behavior applies uniformly to all placement group strategies (cluster, partition, and spread). Note that while EC2 supports creating capacity reservations *within* a cluster placement group, capacity reservations cannot be scoped to partition or spread placement groups. However, standalone capacity reservations (not associated with any placement group) can still be consumed by instances launched into any placement group strategy.

**Spread placement group limit and capacity reservations:** When a reserved launch fails due to the spread placement group 7-instance-per-AZ limit, Karpenter does not mark the capacity reservation as unavailable. The spread limit is a placement constraint failure, not a capacity reservation failure — the CR still has available capacity. This ensures that when a PG slot frees up (e.g., after a node is deleted), subsequent launches can still target the reserved capacity rather than falling back to on-demand.

## Placement Group-Aware ICE Cache

Karpenter's existing ICE (Insufficient Capacity Error) cache tracks instance type + AZ combinations that have recently failed. This cache must be extended to be placement group-aware so that an ICE from a placement group launch does not incorrectly prevent launches of the same instance type + AZ combination outside of that placement group.

When an ICE occurs, Karpenter caches at the most granular failure domain that was targeted. The placement group ID (`pg-*`) is used as the cache key rather than the placement group name, since IDs are immutable and globally unique:

| Strategy | ICE Cache Key | What remains unaffected |
|----------|--------------|------------------------|
| **Cluster** | `(placement group ID, instance type)` | Same instance type + AZ without this PG |
| **Partition** | `(placement group ID, [partition,] instance type)` | Other partitions, non-PG launches in same AZ |
| **Spread** | `(placement group ID, AZ, instance type)` for genuine capacity errors; `(placement group ID, AZ)` when the 7-instance limit is reached. Karpenter distinguishes these by parsing the EC2 error message — if it contains `"spread placement group"`, it's the 7-instance limit and the entire AZ is marked unavailable for that PG; otherwise, it's a genuine capacity error scoped to the instance type. | Other PGs, non-PG launches in same AZ; for genuine capacity errors, other instance types in the same AZ within this PG |

**Example:** NodePool A targets cluster PG `pg-0123456789abcdef0` (`ml-training-pg`) in `us-east-1a`. NodePool B has no placement group. If `p5.48xlarge` in `us-east-1a` returns ICE when launching into `pg-0123456789abcdef0`, the cache entry is `(pg-0123456789abcdef0, p5.48xlarge)`. NodePool B can still launch `p5.48xlarge` in `us-east-1a`.

## Pricing/Consolidation

Placement groups do not directly affect pricing, but they constrain the set of valid instance types and availability zones, which indirectly affects cost optimization.

**No additional consolidation logic is added for placement groups.** Karpenter's standard consolidation behavior applies — consolidation works through scheduling simulation against available offerings. Since the placement group ID is a requirement on the offering (set by the EC2NodeClass), placement group membership is naturally preserved when a pod is re-scheduled onto a replacement node within the same NodePool/NodeClass.

**Cross-placement-group consolidation:** With this implementation, placement group membership is a NodeClass-level constraint, not an application-level constraint. If a pod has no placement-group-related scheduling constraints (no `nodeSelector`, `nodeAffinity`, or `podAffinity` on placement group labels), and multiple NodePools can serve it (one with a PG, one without, or two with different PGs), Karpenter's consolidation may move the pod to a different placement group or out of a placement group entirely if it finds a cheaper option. Applications that require placement group membership should express this via pod-level constraints (e.g., `nodeSelector` on `karpenter.k8s.aws/placement-group-id`) to ensure consolidation respects their placement requirements.

**Within-placement-group consolidation:** When consolidating within the same NodePool/NodeClass, the replacement node naturally launches into the same placement group because the offering requirements include the placement group ID. EC2 strategy-specific limits (single AZ for cluster, 7 instances per AZ for spread) are enforced by EC2 at launch time — if a replacement launch violates these limits, it fails with an ICE and Karpenter handles it through the standard ICE cache mechanism.

## Drift

Nodes are marked as drifted when their placement group ID label (`karpenter.k8s.aws/placement-group-id`) no longer matches the EC2NodeClass's resolved placement group ID stored in the placement group provider's in-memory store. This is checked explicitly in the `isPlacementGroupDrifted` function, which compares the NodeClaim's `placement-group-id` label against the resolved PG ID from the provider.

| Scenario | Detection | Recovery |
|----------|-----------|----------|
| `placementGroupSelector` added to an EC2NodeClass that previously had none | Existing nodes lack `placement-group-id` label | Nodes drifted, replaced into the placement group |
| `placementGroupSelector` removed | Existing nodes have a `placement-group-id` label that no longer matches | Nodes drifted, replaced without placement group |
| `placementGroupSelector` changed to a different placement group | `placement-group-id` label value differs from resolved PG ID | Nodes drifted, replaced into new placement group |
| Placement group deleted externally | `PlacementGroupReady` condition → `False`; EC2NodeClass becomes not ready, blocking all launches | Since a placement group cannot be deleted while it still contains instances, this scenario implies all instances have already been terminated and there are no existing nodes to drift. The only effect is that the EC2NodeClass becomes not ready, blocking future launches until `placementGroupSelector` is updated to a valid placement group. |

Karpenter detects a deleted placement group via `DescribePlacementGroups` returning no results. Note that EC2 prevents deletion of a placement group that still contains running instances, so this scenario only arises after all instances in the group have been terminated.

## Spread Placement Group Disruption Limitations

Spread placement groups impose a hard limit of 7 running instances per AZ per group, enforced by EC2. This limit creates a known limitation for Karpenter's disruption (drift, consolidation) workflows:

**Replace-then-delete is blocked at capacity:** Karpenter's disruption model launches a replacement node before terminating the old one. When a spread PG is at 7 instances in an AZ, launching an 8th instance in that AZ fails with an `InsufficientInstanceCapacity` error. If all AZs are at capacity (7 × number of AZs), no replacement can be scheduled and disruption is blocked entirely — drifted or consolidation-candidate nodes remain running until capacity frees up.

**No fallback to non-placement-group launches:** The current implementation does not attempt to launch replacement instances outside the placement group. All offerings from an EC2NodeClass with a placement group unconditionally include the placement group ID as a scheduling requirement, and all launch templates include the placement group in the `Placement` block.

Users who need consolidation to function at spread PG capacity limits should use `WhenEmpty` consolidation (`spec.disruption.consolidationPolicy: WhenEmpty` with a `consolidateAfter` duration). With `WhenEmpty`, Karpenter deletes a node only after all non-daemonset pods have been drained from it — freeing a placement group slot without requiring a replacement launch first. Users are responsible for ensuring pods are moved off the node (e.g., by scaling down the workload or using pod disruption budgets), at which point Karpenter detects the node as empty, waits the `consolidateAfter` duration, and deletes it. Note that `WhenEmpty` only applies to consolidation — drift always uses replace-then-delete regardless of the consolidation policy, so drift remains blocked when the spread PG is at capacity.

## Appendix

### Input/Output for CreateFleet with Placement Groups

The following table documents the `CreateFleet` API behavior when specifying placement groups through the launch template's `Placement` block:

| Scenario | Placement Configuration | Result |
|----------|------------------------|--------|
| Single placement group targeting | `Placement.GroupId` set in launch template | Instances launched into the specified placement group |
| Empty cluster PG, multiple AZs allowed | `Placement.GroupId` set, no AZ constraint | EC2 selects AZ to maximize instance type availability |
| Non-empty cluster PG | `Placement.GroupId` set, PG already has instances | Instances launched in the same AZ as existing instances; ICE if no capacity |
| Cluster PG, instance type not available | `Placement.GroupId` set, specific instance type | `InsufficientInstanceCapacity` error; Karpenter retries with alternative instance types |
| Partition PG, no partition specified | `Placement.GroupId` set, no `PartitionNumber` | EC2 auto-assigns partition, distributing instances across partitions |
| Partition PG, specific partition | `Placement.GroupId` + `Placement.PartitionNumber` set | Instance launched into the specified partition |
| Spread PG at capacity | `Placement.GroupId` set, 7 instances already in AZ | `InsufficientInstanceCapacity` error |

### Strategy-Specific Limitations

| Strategy | AZ Constraint | Instance Limit | Instance Type Restrictions | Capacity Reservation Support | Spot Support |
|----------|--------------|----------------|---------------------------|------------------------------|-------------|
| Cluster | Single AZ | No hard limit (ICE risk increases) | No burstable, Mac1, M7i-flex | Yes | Yes (terminate only) |
| Partition | Per-AZ partitions | 7 partitions per AZ | None | No | Yes (terminate only) |
| Spread (rack) | Multi-AZ | 7 instances per AZ per group | None | No | Yes (terminate only) |
| Spread (host) | Single AZ (Outposts only) | 1 instance per host | None | No | Yes (terminate only) |
