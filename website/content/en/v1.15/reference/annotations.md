---
title: "Well-Known Annotations"
linkTitle: "Well-Known Annotations"
weight: 8

description: >
  Annotations that Karpenter reads or writes
---
Karpenter reads and writes the annotations below, in the style of the Kubernetes [Well-Known Labels, Annotations and Taints](https://kubernetes.io/docs/reference/labels-annotations-taints/) reference. Each annotation lists an example, the kinds of object it is used on, and its stability level.

[comment]: <> (the content below is generated from hack/docs/annotations_gen/main.go)

## User-Facing Annotations

### `karpenter.sh/disruption-cost`

Type: Annotation

Example: `karpenter.sh/disruption-cost: "100"`

Used on: Pod

Stability Level: ALPHA

Users set this to an int32 cost of evicting the pod. Consolidation prefers to disrupt nodes whose pods cost less to evict, so a higher cost makes the pod's node less likely to be consolidated. When unset and the PodDeletionCostManagement feature gate is disabled, Karpenter reads controller.kubernetes.io/pod-deletion-cost instead; when enabled, Karpenter writes controller.kubernetes.io/pod-deletion-cost itself.

### `karpenter.sh/do-not-disrupt`

Type: Annotation

Example: `karpenter.sh/do-not-disrupt: "true"`

Used on: Pod, Node, NodeClaim

Stability Level: GA

Users set this to block voluntary disruption. On a Node or NodeClaim, it blocks consolidation and drift. On a Pod, it blocks consolidation of the Pod's node, and blocks drift unless the NodeClaim sets a terminationGracePeriod. On a Pod, the value may also be a duration (e.g. `5m`) after the Pod's start time at which the protection ends. It does not block expiration, repair, or forceful termination once the terminationGracePeriod elapses.

Values:
- `true` — Disruption is blocked.

### `karpenter.sh/do-not-repair`

Type: Annotation

Example: `karpenter.sh/do-not-repair: "true"`

Used on: Node, NodeClaim

Stability Level: ALPHA

Users set this to block node repair, independently of karpenter.sh/do-not-disrupt.

Values:
- `true` — Repair is blocked.

## Internal Annotations

Karpenter sets these annotations itself. They are documented for visibility only and may change at any time; never set them, never read them.

### `compatibility.karpenter.k8s.aws/cluster-name-tagged`

Type: Annotation

Example: `compatibility.karpenter.k8s.aws/cluster-name-tagged: "true"`

Used on: NodeClaim

Stability Level: ALPHA

Karpenter sets this once it has tagged the NodeClaim's EC2 instance with the eks:eks-cluster-name tag, so instances tagged before that tag was introduced are tagged again.

Values:
- `true` — The instance has the eks:eks-cluster-name tag.

### `karpenter.k8s.aws/ec2nodeclass-hash`

Type: Annotation

Example: `karpenter.k8s.aws/ec2nodeclass-hash: "5763643673275251833"`

Used on: EC2NodeClass, NodeClaim

Stability Level: ALPHA

Karpenter sets this to a hash of the EC2NodeClass, on the EC2NodeClass and on each NodeClaim it launches. A NodeClaim whose hash differs from its EC2NodeClass's is drifted.

### `karpenter.k8s.aws/ec2nodeclass-hash-version`

Type: Annotation

Example: `karpenter.k8s.aws/ec2nodeclass-hash-version: "v6"`

Used on: EC2NodeClass, NodeClaim

Stability Level: ALPHA

Karpenter sets this to the version of the karpenter.k8s.aws/ec2nodeclass-hash algorithm. Hashes are only compared when versions match; when the version changes, Karpenter rehashes NodeClaims instead of drifting them.

Values:
- `v6` — The current hash version.

### `karpenter.k8s.aws/instance-profile-name`

Type: Annotation

Example: `karpenter.k8s.aws/instance-profile-name: "my-cluster_15263850463527461230"`

Used on: NodeClaim

Stability Level: ALPHA

Karpenter sets this to the name of the instance profile the NodeClaim's instance was launched with. Instance profiles still referenced by a NodeClaim are not garbage collected, so they outlive a change to the EC2NodeClass's role.

### `karpenter.k8s.aws/tagged`

Type: Annotation

Example: `karpenter.k8s.aws/tagged: "true"`

Used on: NodeClaim

Stability Level: ALPHA

Karpenter sets this once it has tagged the NodeClaim's EC2 instance with its Name, karpenter.sh/nodeclaim, and eks:eks-cluster-name tags.

Values:
- `true` — The instance is tagged.

### `karpenter.sh/nodeclaim-min-values-relaxed`

Type: Annotation

Example: `karpenter.sh/nodeclaim-min-values-relaxed: "false"`

Used on: NodeClaim

Stability Level: ALPHA

Karpenter sets this to whether scheduling relaxed the NodePool's minValues requirements to launch the NodeClaim.

Values:
- `true` — minValues was relaxed.
- `false` — minValues was satisfied.

### `karpenter.sh/nodeclaim-termination-timestamp`

Type: Annotation

Example: `karpenter.sh/nodeclaim-termination-timestamp: "2026-10-01T22:00:00Z"`

Used on: NodeClaim

Stability Level: ALPHA

Karpenter sets this to the RFC3339 time by which the node must finish draining, from the NodeClaim's terminationGracePeriod when it is deleted. Pods are deleted early enough to complete their own terminationGracePeriodSeconds by then, bypassing PDBs and karpenter.sh/do-not-disrupt.

### `karpenter.sh/nodepool-hash`

Type: Annotation

Example: `karpenter.sh/nodepool-hash: "6821555240594823858"`

Used on: NodePool, NodeClaim

Stability Level: ALPHA

Karpenter sets this to a hash of the NodePool's template, on the NodePool and on each NodeClaim it launches. A NodeClaim whose hash differs from its NodePool's is drifted.

### `karpenter.sh/nodepool-hash-version`

Type: Annotation

Example: `karpenter.sh/nodepool-hash-version: "v3"`

Used on: NodePool, NodeClaim

Stability Level: ALPHA

Karpenter sets this to the version of the karpenter.sh/nodepool-hash algorithm. Hashes are only compared when versions match; when the version changes, Karpenter rehashes NodeClaims instead of drifting them.

Values:
- `v3` — The current hash version.

### `karpenter.sh/reboot-pre-boot-id`

Type: Annotation

Example: `karpenter.sh/reboot-pre-boot-id: "2b6c2f9e-3a8d-4f4e-9c1a-7d5e8b0f6a12"`

Used on: NodeClaim

Stability Level: ALPHA

Karpenter sets this to the node's boot ID before it issues a reboot. A changed boot ID means the reboot happened, so it is not issued again. Removed when the reboot completes.

### `karpenter.sh/reboot-termination-grace-period`

Type: Annotation

Example: `karpenter.sh/reboot-termination-grace-period: "10m"`

Used on: NodeClaim

Stability Level: ALPHA

Karpenter sets this to a duration bounding the drain before a reboot. When unset, the drain is unbounded; `0` drains forcefully.

### `karpenter.sh/requested-dra-drivers`

Type: Annotation

Example: `karpenter.sh/requested-dra-drivers: "gpu.nvidia.com"`

Used on: NodeClaim

Stability Level: ALPHA

Karpenter sets this to a comma-separated list of the DRA drivers whose devices were allocated to pods scheduled to the NodeClaim. The node is not initialized until each driver has published its ResourceSlices.

[comment]: <> (end docs generated content from hack/docs/annotations_gen/main.go)
