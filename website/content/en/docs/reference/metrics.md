---
title: "Metrics"
linkTitle: "Metrics"
weight: 7

description: >
  Inspect Karpenter Metrics
---
<!-- this document is generated from hack/docs/metrics_gen_docs.go -->
Karpenter makes several metrics available in Prometheus format to allow monitoring cluster provisioning status. These metrics are available by default at `karpenter.karpenter.svc.cluster.local:8080/metrics` configurable via the `METRICS_PORT` environment variable documented [here](../settings)
### `karpenter_build_info`
A metric with a constant '1' value labeled by version from which karpenter was built.
- Stability Level: STABLE

## Nodeclaims Metrics

### `karpenter_nodeclaims_termination_duration_seconds`
Duration of NodeClaim termination in seconds.
- Stability Level: BETA

### `karpenter_nodeclaims_terminated_total`
Number of nodeclaims terminated in total by Karpenter. Labeled by the owning nodepool.
- Stability Level: STABLE

### `karpenter_nodeclaims_instance_termination_duration_seconds`
Duration of CloudProvider Instance termination in seconds.
- Stability Level: BETA

### `karpenter_nodeclaims_disrupted_total`
Number of nodeclaims disrupted in total by Karpenter. Labeled by reason the nodeclaim was disrupted and the owning nodepool.
- Stability Level: ALPHA

### `karpenter_nodeclaims_created_total`
Number of nodeclaims created in total by Karpenter. Labeled by reason the nodeclaim was created and the owning nodepool.
- Stability Level: STABLE

## Nodes Metrics

### `karpenter_nodes_total_pod_requests`
Node total pod requests are the resources requested by pods bound to nodes, including the DaemonSet pods.
- Stability Level: BETA

### `karpenter_nodes_total_pod_limits`
Node total pod limits are the resources specified by pod limits, including the DaemonSet pods.
- Stability Level: BETA

### `karpenter_nodes_total_daemon_requests`
Node total daemon requests are the resource requested by DaemonSet pods bound to nodes.
- Stability Level: BETA

### `karpenter_nodes_total_daemon_limits`
Node total daemon limits are the resources specified by DaemonSet pod limits.
- Stability Level: BETA

### `karpenter_nodes_termination_duration_seconds`
The time taken between a node's deletion request and the removal of its finalizer
- Stability Level: BETA

### `karpenter_nodes_terminated_total`
Number of nodes terminated in total by Karpenter. Labeled by owning nodepool.
- Stability Level: STABLE

### `karpenter_nodes_system_overhead`
Node system daemon overhead are the resources reserved for system overhead, the difference between the node's capacity and allocatable values are reported by the status.
- Stability Level: BETA

### `karpenter_nodes_leases_deleted_total`
Number of deleted leaked leases.
- Stability Level: ALPHA

### `karpenter_nodes_created_total`
Number of nodes created in total by Karpenter. Labeled by owning nodepool.
- Stability Level: STABLE

### `karpenter_nodes_allocatable`
Node allocatable are the resources allocatable by nodes.
- Stability Level: BETA

## Pods Metrics

### `karpenter_pods_state`
Pod state is the current state of pods. This metric can be used several ways as it is labeled by the pod name, namespace, owner, node, nodepool name, zone, architecture, capacity type, instance type and pod phase.
- Stability Level: BETA

### `karpenter_pods_startup_duration_seconds`
The time from pod creation until the pod is running.
- Stability Level: STABLE

## Voluntary Disruption Metrics

### `karpenter_voluntary_disruption_queue_failures_total`
The number of times that an enqueued disruption decision failed. Labeled by disruption method.
- Stability Level: BETA

### `karpenter_voluntary_disruption_eligible_nodes`
Number of nodes eligible for disruption by Karpenter. Labeled by disruption reason.
- Stability Level: BETA

### `karpenter_voluntary_disruption_decisions_total`
Number of disruption decisions performed. Labeled by disruption decision, reason, and consolidation type.
- Stability Level: STABLE

### `karpenter_voluntary_disruption_decision_evaluation_duration_seconds`
Duration of the disruption decision evaluation process in seconds. Labeled by method and consolidation type.
- Stability Level: BETA

### `karpenter_voluntary_disruption_consolidation_timeouts_total`
Number of times the Consolidation algorithm has reached a timeout. Labeled by consolidation type.
- Stability Level: BETA

## Scheduler Metrics

### `karpenter_scheduler_scheduling_duration_seconds`
Duration of scheduling simulations used for deprovisioning and provisioning in seconds.
- Stability Level: STABLE

### `karpenter_scheduler_queue_depth`
The number of pods currently waiting to be scheduled.
- Stability Level: BETA

## Nodepools Metrics

### `karpenter_nodepools_usage`
The amount of resources that have been provisioned for a nodepool. Labeled by nodepool name and resource type.
- Stability Level: ALPHA

### `karpenter_nodepools_limit`
Limits specified on the nodepool that restrict the quantity of resources provisioned. Labeled by nodepool name and resource type.
- Stability Level: ALPHA

### `karpenter_nodepools_allowed_disruptions`
The number of nodes for a given NodePool that can be concurrently disrupting at a point in time. Labeled by NodePool. Note that allowed disruptions can change very rapidly, as new nodes may be created and others may be deleted at any point.
- Stability Level: ALPHA

## Interruption Metrics

### `karpenter_interruption_received_messages_total`
Count of messages received from the SQS queue. Broken down by message type and whether the message was actionable.
- Stability Level: STABLE

### `karpenter_interruption_message_queue_duration_seconds`
Amount of time an interruption message is on the queue before it is processed by karpenter.
- Stability Level: STABLE

### `karpenter_interruption_deleted_messages_total`
Count of messages deleted from the SQS queue.
- Stability Level: STABLE

## Cluster State Metrics

### `karpenter_cluster_state_synced`
Returns 1 if cluster state is synced and 0 otherwise. Synced checks that nodeclaims and nodes that are stored in the APIServer have the same representation as Karpenter's cluster state
- Stability Level: STABLE

### `karpenter_cluster_state_node_count`
Current count of nodes in cluster state
- Stability Level: STABLE

## Cloudprovider Metrics

### `karpenter_cloudprovider_instance_type_offering_price_estimate`
Instance type offering estimated hourly price used when making informed decisions on node cost calculation, based on instance type, capacity type, and zone.
- Stability Level: BETA

### `karpenter_cloudprovider_instance_type_offering_available`
Instance type offering availability, based on instance type, capacity type, and zone
- Stability Level: BETA

### `karpenter_cloudprovider_instance_type_memory_bytes`
Memory, in bytes, for a given instance type.
- Stability Level: BETA

### `karpenter_cloudprovider_instance_type_cpu_cores`
VCPUs cores for a given instance type.
- Stability Level: BETA

### `karpenter_cloudprovider_errors_total`
Total number of errors returned from CloudProvider calls.
- Stability Level: BETA

### `karpenter_cloudprovider_duration_seconds`
Duration of cloud provider method calls. Labeled by the controller, method name and provider.
- Stability Level: BETA

## Cloudprovider Batcher Metrics

### `karpenter_cloudprovider_batcher_batch_time_seconds`
Duration of the batching window per batcher
- Stability Level: BETA

### `karpenter_cloudprovider_batcher_batch_size`
Size of the request batch per batcher
- Stability Level: BETA

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
Current depth of workqueue
- Stability Level: STABLE

### `workqueue_adds_total`
Total number of adds handled by workqueue
- Stability Level: STABLE

## Status Condition Metrics

### `operator_status_condition_transitions_total`
The count of transitions of a given object, type and status.
- Stability Level: DEPRECATED

### `operator_status_condition_transition_seconds`
The amount of time a condition was in a given state before transitioning. e.g. Alarm := P99(Updated=False) > 5 minutes
- Stability Level: DEPRECATED

### `operator_status_condition_current_status_seconds`
The current amount of time in seconds that a status condition has been in a specific state. Alarm := P99(Updated=Unknown) > 5 minutes
- Stability Level: DEPRECATED

### `operator_status_condition_count`
The number of an condition for a given object, type and status. e.g. Alarm := Available=False > 0
- Stability Level: DEPRECATED

## Client Go Metrics

### `client_go_request_total`
Number of HTTP requests, partitioned by status code and method.
- Stability Level: STABLE

### `client_go_request_duration_seconds`
Request latency in seconds. Broken down by verb, group, version, kind, and subresource.
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

