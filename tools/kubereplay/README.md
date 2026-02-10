# kubereplay

Capture and replay workload events from EKS audit logs for Karpenter A/B testing.

```bash
go build -o kubereplay ./cmd

kubereplay capture            # capture last hour to replay.json
kubereplay replay             # replay from replay.json
kubereplay replay --speed 24  # replay 24x faster
kubereplay demo               # generate synthetic test data
```

Requires EKS audit logging, AWS credentials, and kubectl access.
