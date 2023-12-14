---
title: "Metrics"
linkTitle: "Metrics"
weight: 7

description: >
  Inspect Karpenter Metrics
---
<!-- this document is generated from hack/docs/metrics_gen_docs.go -->
Karpenter makes several metrics available in Prometheus format to allow monitoring cluster provisioning status. These metrics are available by default at `karpenter.karpenter.svc.cluster.local:8000/metrics` configurable via the `METRICS_PORT` environment variable documented [here](../settings)
## Controller Runtime Metrics

### `controller_runtime_active_workers`
Number of currently used workers per controller

### `controller_runtime_max_concurrent_reconciles`
Maximum number of concurrent reconciles per controller

### `controller_runtime_reconcile_errors_total`
Total number of reconciliation errors per controller

### `controller_runtime_reconcile_time_seconds`
Length of time per reconciliation per controller

### `controller_runtime_reconcile_total`
Total number of reconciliations per controller

## Consistency Metrics

### `karpenter_consistency_errors`
Number of consistency checks that have failed.

## Deprovisioning Metrics

### `karpenter_deprovisioning_actions_performed`
Number of deprovisioning actions performed. Labeled by deprovisioner.

### `karpenter_deprovisioning_consolidation_timeouts`
Number of times the Consolidation algorithm has reached a timeout. Labeled by consolidation type.

### `karpenter_deprovisioning_eligible_machines`
Number of machines eligible for deprovisioning by Karpenter. Labeled by deprovisioner

### `karpenter_deprovisioning_evaluation_duration_seconds`
Duration of the deprovisioning evaluation process in seconds.

### `karpenter_deprovisioning_replacement_machine_initialized_seconds`
Amount of time required for a replacement machine to become initialized.

### `karpenter_deprovisioning_replacement_machine_launch_failure_counter`
The number of times that Karpenter failed to launch a replacement node for deprovisioning. Labeled by deprovisioner.

## Disruption Metrics

### `karpenter_disruption_actions_performed_total`
Number of disruption actions performed. Labeled by disruption method.

### `karpenter_disruption_consolidation_timeouts_total`
Number of times the Consolidation algorithm has reached a timeout. Labeled by consolidation type.

### `karpenter_disruption_eligible_nodes`
Number of nodes eligible for disruption by Karpenter. Labeled by disruption method.

### `karpenter_disruption_evaluation_duration_seconds`
Duration of the disruption evaluation process in seconds.

### `karpenter_disruption_replacement_nodeclaim_failures_total`
The number of times that Karpenter failed to launch a replacement node for disruption. Labeled by disruption method.

### `karpenter_disruption_replacement_nodeclaim_initialized_seconds`
Amount of time required for a replacement nodeclaim to become initialized.

## Interruption Metrics

### `karpenter_interruption_actions_performed`
Number of notification actions performed. Labeled by action

### `karpenter_interruption_deleted_messages`
Count of messages deleted from the SQS queue.

### `karpenter_interruption_message_latency_time_seconds`
Length of time between message creation in queue and an action taken on the message by the controller.

### `karpenter_interruption_received_messages`
Count of messages received from the SQS queue. Broken down by message type and whether the message was actionable.

## Machines Metrics

### `karpenter_machines_created`
Number of machines created in total by Karpenter. Labeled by reason the machine was created and the owning provisioner.

### `karpenter_machines_disrupted`
Number of machines disrupted in total by Karpenter. Labeled by disruption type of the machine and the owning provisioner.

### `karpenter_machines_drifted`
Number of machine drifted reasons in total by Karpenter. Labeled by drift type of the machine and the owning provisioner..

### `karpenter_machines_initialized`
Number of machines initialized in total by Karpenter. Labeled by the owning provisioner.

### `karpenter_machines_launched`
Number of machines launched in total by Karpenter. Labeled by the owning provisioner.

### `karpenter_machines_registered`
Number of machines registered in total by Karpenter. Labeled by the owning provisioner.

### `karpenter_machines_terminated`
Number of machines terminated in total by Karpenter. Labeled by reason the machine was terminated and the owning provisioner.

## Nodeclaims Metrics

### `karpenter_nodeclaims_created`
Number of nodeclaims created in total by Karpenter. Labeled by reason the nodeclaim was created and the owning nodepool.

### `karpenter_nodeclaims_disrupted`
Number of nodeclaims disrupted in total by Karpenter. Labeled by disruption type of the nodeclaim and the owning nodepool.

### `karpenter_nodeclaims_drifted`
Number of nodeclaims drifted reasons in total by Karpenter. Labeled by drift type of the nodeclaim and the owning nodepool.

### `karpenter_nodeclaims_initialized`
Number of nodeclaims initialized in total by Karpenter. Labeled by the owning nodepool.

### `karpenter_nodeclaims_launched`
Number of nodeclaims launched in total by Karpenter. Labeled by the owning nodepool.

### `karpenter_nodeclaims_registered`
Number of nodeclaims registered in total by Karpenter. Labeled by the owning nodepool.

### `karpenter_nodeclaims_terminated`
Number of nodeclaims terminated in total by Karpenter. Labeled by reason the nodeclaim was terminated and the owning nodepool.

## Nodepool Metrics

### `karpenter_nodepool_limit`
The nodepool limits are the limits specified on the provisioner that restrict the quantity of resources provisioned. Labeled by nodepool name and resource type.

### `karpenter_nodepool_usage`
The nodepool usage is the amount of resources that have been provisioned by a particular nodepool. Labeled by nodepool name and resource type.

## Provisioner Metrics

### `karpenter_provisioner_limit`
The Provisioner Limits are the limits specified on the provisioner that restrict the quantity of resources provisioned. Labeled by provisioner name and resource type.

### `karpenter_provisioner_scheduling_duration_seconds`
Duration of scheduling process in seconds.

### `karpenter_provisioner_scheduling_simulation_duration_seconds`
Duration of scheduling simulations used for deprovisioning and provisioning in seconds.

### `karpenter_provisioner_usage`
The Provisioner Usage is the amount of resources that have been provisioned by a particular provisioner. Labeled by provisioner name and resource type.

### `karpenter_provisioner_usage_pct`
The Provisioner Usage Percentage is the percentage of each resource used based on the resources provisioned and the limits that have been configured in the range [0,100].  Labeled by provisioner name and resource type.

## Nodes Metrics

### `karpenter_nodes_allocatable`
Node allocatable are the resources allocatable by nodes.

### `karpenter_nodes_created`
Number of nodes created in total by Karpenter. Labeled by owning provisioner.

### `karpenter_nodes_leases_deleted`
Number of deleted leaked leases.

### `karpenter_nodes_system_overhead`
Node system daemon overhead are the resources reserved for system overhead, the difference between the node's capacity and allocatable values are reported by the status.

### `karpenter_nodes_terminated`
Number of nodes terminated in total by Karpenter. Labeled by owning provisioner.

### `karpenter_nodes_termination_time_seconds`
The time taken between a node's deletion request and the removal of its finalizer

### `karpenter_nodes_total_daemon_limits`
Node total daemon limits are the resources specified by DaemonSet pod limits.

### `karpenter_nodes_total_daemon_requests`
Node total daemon requests are the resource requested by DaemonSet pods bound to nodes.

### `karpenter_nodes_total_pod_limits`
Node total pod limits are the resources specified by non-DaemonSet pod limits.

### `karpenter_nodes_total_pod_requests`
Node total pod requests are the resources requested by non-DaemonSet pods bound to nodes.

## Pods Metrics

### `karpenter_pods_startup_time_seconds`
The time from pod creation until the pod is running.

### `karpenter_pods_state`
Pod state is the current state of pods. This metric can be used several ways as it is labeled by the pod name, namespace, owner, node, provisioner name, zone, architecture, capacity type, instance type and pod phase.

## Cloudprovider Metrics

### `karpenter_cloudprovider_duration_seconds`
Duration of cloud provider method calls. Labeled by the controller, method name and provider.

### `karpenter_cloudprovider_errors_total`
Total number of errors returned from CloudProvider calls.

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

