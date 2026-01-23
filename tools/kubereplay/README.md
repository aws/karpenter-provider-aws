# kubereplay

Capture workload events from EKS audit logs and replay them against Karpenter clusters for A/B testing.

## Usage

```bash
# Build
go build -o kubereplay ./cmd

# Capture last hour of workload events
kubereplay capture -o workloads.json

# Replay against test cluster (Ctrl+C to cleanup)
kubereplay replay -f workloads.json

# Replay 24x faster
kubereplay replay -f workloads.json --speed 24

# Generate synthetic test data
kubereplay demo -o demo.json
```

## Prerequisites

- EKS cluster with audit logging enabled
- AWS credentials with CloudWatch Logs read access
- kubectl access to target cluster
