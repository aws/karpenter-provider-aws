---
title: "Metrics"
linkTitle: "Metrics"
weight: 7

description: >
  Inspect Karpenter Metrics
---
<!-- this document is generated from hack/docs/metrics_gen_docs.go -->
Karpenter makes several metrics available in Prometheus format to allow monitoring cluster provisioning status. These metrics are available by default at `karpenter.karpenter.svc.cluster.local:8000/metrics` configurable via the `METRICS_PORT` environment variable documented [here](../settings)
## Interruption Metrics

### `karpenter_interruption_actions_performed`
Number of notification actions performed. Labeled by action

### `karpenter_interruption_deleted_messages`
Count of messages deleted from the SQS queue.

### `karpenter_interruption_message_latency_time_seconds`
Length of time between message creation in queue and an action taken on the message by the controller.

### `karpenter_interruption_received_messages`
Count of messages received from the SQS queue. Broken down by message type and whether the message was actionable.

## Cloudprovider Metrics

### `karpenter_cloudprovider_instance_type_cpu_cores`
VCPUs cores for a given instance type.

### `karpenter_cloudprovider_instance_type_memory_bytes`
Memory, in bytes, for a given instance type.

### `karpenter_cloudprovider_instance_type_price_estimate`
Estimated hourly price used when making informed decisions on node cost calculation. This is updated once on startup and then every 12 hours.

## Cloudprovider Batcher Metrics

### `karpenter_cloudprovider_batcher_batch_size`
Size of the request batch per batcher

### `karpenter_cloudprovider_batcher_batch_time_seconds`
Duration of the batching window per batcher

