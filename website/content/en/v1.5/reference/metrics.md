---
title: "Metrics"
linkTitle: "Metrics"
weight: 7

description: >
  Inspect Karpenter Metrics
---
<!-- this document is generated from hack/docs/metrics_gen_docs.go -->
Karpenter makes several metrics available in Prometheus format to allow monitoring cluster provisioning status. These metrics are available by default at `karpenter.kube-system.svc.cluster.local:8080/metrics` configurable via the `METRICS_PORT` environment variable documented [here](../settings)
## Controller Runtime Metrics

### `controller_runtime_terminal_reconcile_errors_total`
Total number of terminal reconciliation errors per controller
- Stability Level: STABLE

### `controller_runtime_reconcile_total`
Total number of reconciliations per controller
- Stability Level: STABLE

### `controller_runtime_reconcile_time_seconds`
Length of time per reconciliation per controller
- Stability Level: STABLE

### `controller_runtime_reconcile_panics_total`
Total number of reconciliation panics per controller
- Stability Level: STABLE

### `controller_runtime_reconcile_errors_total`
Total number of reconciliation errors per controller
- Stability Level: STABLE

### `controller_runtime_max_concurrent_reconciles`
Maximum number of concurrent reconciles per controller
- Stability Level: STABLE

### `controller_runtime_active_workers`
Number of currently used workers per controller
- Stability Level: STABLE

## Workqueue Metrics

### `workqueue_work_duration_seconds`
How long in seconds processing an item from workqueue takes.
- Stability Level: STABLE

### `workqueue_unfinished_work_seconds`
How many seconds of work has been done that is in progress and hasn't been observed by work_duration. Large values indicate stuck threads. One can deduce the number of stuck threads by observing the rate at which this increases.
- Stability Level: STABLE

### `workqueue_retries_total`
Total number of retries handled by workqueue
- Stability Level: STABLE

### `workqueue_queue_duration_seconds`
How long in seconds an item stays in workqueue before being requested
- Stability Level: STABLE

### `workqueue_longest_running_processor_seconds`
How many seconds has the longest running processor for workqueue been running.
- Stability Level: STABLE

### `workqueue_depth`
Current depth of workqueue by workqueue and priority
- Stability Level: STABLE

### `workqueue_adds_total`
Total number of adds handled by workqueue
- Stability Level: STABLE

## AWS SDK Go Metrics

### `aws_sdk_go_request_total`
The total number of AWS SDK Go requests
- Stability Level: STABLE

### `aws_sdk_go_request_retry_count`
The total number of AWS SDK Go retry attempts per request
- Stability Level: STABLE

### `aws_sdk_go_request_duration_seconds`
Latency of AWS SDK Go requests
- Stability Level: STABLE

### `aws_sdk_go_request_attempt_total`
The total number of AWS SDK Go request attempts
- Stability Level: STABLE

### `aws_sdk_go_request_attempt_duration_seconds`
Latency of AWS SDK Go request attempts
- Stability Level: STABLE

## Leader Election Metrics

### `leader_election_slowpath_total`
Total number of slow path exercised in renewing leader leases. 'name' is the string used to identify the lease. Please make sure to group by name.
- Stability Level: STABLE

### `leader_election_master_status`
Gauge of if the reporting system is master of the relevant lease, 0 indicates backup, 1 indicates master. 'name' is the string used to identify the lease. Please make sure to group by name.
- Stability Level: STABLE

