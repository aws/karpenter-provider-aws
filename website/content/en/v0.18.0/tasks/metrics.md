---
title: "Metrics"
linkTitle: "Metrics"
weight: 100

description: >
  Inspect Karpenter Metrics
---
<!-- this document is generated from hack/docs/metrics_gen_docs.go -->
Karpenter makes several metrics available in Prometheus format to allow monitoring cluster provisioning status. These metrics are available by default at `karpenter.karpenter.svc.cluster.local:8080/metrics` configurable via the `METRICS_PORT` environment variable documented [here](../configuration)
## Consolidation Metrics

### `karpenter_consolidation_actions_performed`
Number of consolidation actions performed. Labeled by action.

### `karpenter_consolidation_evaluation_duration_seconds`
Duration of the consolidation evaluation process in seconds.

### `karpenter_consolidation_replacement_node_initialized_seconds`
Amount of time required for a replacement node to become initialized.

## Provisioner Metrics

### `karpenter_provisioner_limit`
The Provisioner Limits are the limits specified on the provisioner that restrict the quantity of resources provisioned. Labeled by provisioner name and resource type.

### `karpenter_provisioner_usage`
The Provisioner Usage is the amount of resources that have been provisioned by a particular provisioner. Labeled by provisioner name and resource type.

### `karpenter_provisioner_usage_pct`
The Provisioner Usage Percentage is the percentage of each resource used based on the resources provisioned and the limits that have been configured in the range [0,100].  Labeled by provisioner name and resource type.

## Nodes Metrics

### `karpenter_nodes_allocatable`
Node allocatable are the resources allocatable by nodes.

### `karpenter_nodes_created`
Number of nodes created in total by Karpenter. Labeled by reason the node was created.

### `karpenter_nodes_system_overhead`
Node system daemon overhead are the resources reserved for system overhead, the difference between the node's capacity and allocatable values are reported by the status.

### `karpenter_nodes_terminated`
Number of nodes terminated in total by Karpenter. Labeled by reason the node was terminated.

### `karpenter_nodes_termination_time_seconds`
The time taken between a node's deletion request and the removal of its finalizer

### `karpenter_nodes_total_daemon_limits`
Node total daemon requests are the resource requested by DaemonSet pods bound to nodes.

### `karpenter_nodes_total_daemon_requests`
Node total daemon limits are the resources specified by DaemonSet pod limits.

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

## Allocation_controller Metrics

### `karpenter_allocation_controller_scheduling_duration_seconds`
Duration of scheduling process in seconds. Broken down by provisioner and error.

