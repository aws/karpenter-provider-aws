# Workload Replay

A tool for capturing and replaying workload events (Deployments, Jobs) from EKS audit logs against Karpenter clusters.

## Why Workloads Instead of Pods?

Pods are ephemeral artifacts managed by higher-level controllers. Replaying raw pod events conflates two concerns:

1. **Workload intent** - what the user actually deployed (Deployments, Jobs)
2. **Node management artifacts** - pods recreated after node disruption/eviction

This tool captures **workload intent** by tracking Deployments and Jobs, enabling valid A/B comparisons between different Karpenter versions. The same workload pattern can be replayed against different configurations while letting each Karpenter version make its own provisioning decisions.

## Installation

```bash
cd tools/kubereplay
go build -o kubereplay ./cmd
```

## Quick Start

### 1. Capture Workload Events

```bash
kubereplay capture -o replay.json
```

### 2. Replay Against Test Cluster

```bash
kubereplay replay -f replay.json
```

Press Ctrl+C when done to cleanup workloads.

## Commands

### `capture`

Captures workload events (Deployments, Jobs, scale changes) from EKS audit logs.

```bash
# Capture last 1 hour (default)
kubereplay capture -o replay.json

# Capture last 24 hours
kubereplay capture --duration 24h -o replay.json
```

**Flags:**
- `--output, -o` - Output file path (default: replay.json)
- `--duration, -d` - Duration to capture (default: 1h)

**Events captured:**
- Deployment creates
- Deployment scale changes (from autoscalers like HPA)
- Job creates

### `replay`

Replays captured workloads against a cluster with time-based scheduling. Workloads are created at their original relative timing (with optional speed adjustment), then the tool waits for stabilization before cleanup.

```bash
# Real-time replay (events applied at original timing)
kubereplay replay -f replay.json

# 24x faster replay (24h capture replays in 1h)
kubereplay replay -f replay.json --speed 24

# Preview timing without creating workloads
kubereplay replay -f replay.json --dry-run

# Fast preview (compress 1h to 1 second)
kubereplay replay -f replay.json --dry-run --speed 3600
```

**Flags:**
- `--file, -f` - Replay log file (required)
- `--speed` - Time dilation factor (default: 1.0). Higher = faster replay
- `--dry-run` - Simulate replay with timing output, no workloads created

### `demo`

Generates synthetic replay data for testing with simulated timestamps.

```bash
# Generate 20 deployments and 10 jobs spread across 1 hour
kubereplay demo -o demo.json

# Custom counts and duration
kubereplay demo -o demo.json --deployments 50 --jobs 20 --duration 24h
```

**Flags:**
- `--output, -o` - Output file path (default: demo.json)
- `--deployments` - Number of deployments to generate (default: 20)
- `--jobs` - Number of jobs to generate (default: 10)
- `--duration` - Time span to spread events across (default: 1h)

## Sanitization

Captured workloads are sanitized while preserving scheduling-relevant properties:

**Preserved:**
- Replica counts
- Pod template scheduling constraints:
  - Node selectors, affinity/anti-affinity rules
  - Tolerations, topology spread constraints
  - Resource requests/limits
  - Priority class
- Karpenter annotations (e.g., do-not-disrupt)

**Removed:**
- UIDs, resource versions, owner references
- Service accounts, volumes, secrets
- Container images (replaced with `pause:3.9`)

## Replay Log Format

```json
{
  "cluster": "my-cluster",
  "captured": "2024-01-15T10:00:00Z",
  "events": [
    {
      "type": "create",
      "kind": "Deployment",
      "key": "default/my-app",
      "deployment": { ... },
      "timestamp": "2024-01-15T09:00:00Z"
    },
    {
      "type": "scale",
      "kind": "Deployment",
      "key": "default/my-app",
      "replicas": 10,
      "timestamp": "2024-01-15T09:30:00Z"
    },
    {
      "type": "create",
      "kind": "Job",
      "key": "default/batch-job",
      "job": { ... },
      "timestamp": "2024-01-15T09:45:00Z"
    }
  ]
}
```

Each event includes its original timestamp, enabling time-based replay of workload patterns.

## Prerequisites

- AWS credentials with CloudWatch Logs read access
- EKS cluster with audit logging enabled
- `kubectl` access to target cluster (for replay)
