---
title: "Settings"
linkTitle: "Settings"
weight: 5
description: >
  Configure Karpenter
---

Karpenter surfaces environment variables and CLI parameters to allow you to configure certain global settings on the controllers. These settings are described below.

[comment]: <> (the content below is generated from hack/docs/configuration_gen_docs.go)

| Environment Variable | CLI Flag | Description |
|--|--|--|
| DISABLE_WEBHOOK | \-\-disable-webhook | Disable the admission and validation webhooks (default = false)|
| ENABLE_PROFILING | \-\-enable-profiling | Enable the profiling on the metric endpoint (default = false)|
| HEALTH_PROBE_PORT | \-\-health-probe-port | The port the health probe endpoint binds to for reporting controller health (default = 8081)|
| KARPENTER_SERVICE | \-\-karpenter-service | The Karpenter Service name for the dynamic webhook certificate|
| KUBE_CLIENT_BURST | \-\-kube-client-burst | The maximum allowed burst of queries to the kube-apiserver (default = 300)|
| KUBE_CLIENT_QPS | \-\-kube-client-qps | The smoothed rate of qps to kube-apiserver (default = 200)|
| LEADER_ELECT | \-\-leader-elect | Start leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability. (default = true)|
| LOG_LEVEL | \-\-log-level | Log verbosity level. Can be one of 'debug', 'info', or 'error'|
| MEMORY_LIMIT | \-\-memory-limit | Memory limit on the container running the controller. The GC soft memory limit is set to 90% of this value. (default = -1)|
| METRICS_PORT | \-\-metrics-port | The port the metric endpoint binds to for operating metrics about the controller itself (default = 8000)|
| WEBHOOK_METRICS_PORT | \-\-webhook-metrics-port | The port the webhook metric endpoing binds to for operating metrics about the webhook (default = 8001)|
| WEBHOOK_PORT | \-\-webhook-port | The port the webhook endpoint binds to for validation and mutation of resources (default = 8443)|

[comment]: <> (end docs generated content from hack/docs/configuration_gen_docs.go)

### Feature Gates

Karpenter uses [feature gates](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/#feature-gates-for-alpha-or-beta-features) You can enable the feature gates through the `--feature-gates` CLI environment variable or the `FEATURE_GATES` environment variable in the Karpenter deployment. For example, you can configure drift by setting the following CLI argument: `--feature-gates Drift=true`.

| Feature | Default | Stage | Since   | Until   |
|---------|---------|-------|---------|---------|
| Drift   | false   | Alpha | v0.21.x | v0.32.x |
| Drift   | true    | Beta  | v0.33.x |         |

### Batching Parameters

The batching parameters control how Karpenter batches an incoming stream of pending pods.  Reducing these values may trade off a slightly faster time from pending pod to node launch, in exchange for launching smaller nodes.  Increasing the values can do the inverse.  Karpenter provides reasonable defaults for these values, but if you have specific knowledge about your workloads you can tweak these parameters to match the expected rate of incoming pods.

For a standard deployment scale-up, the pods arrive at the QPS setting of the `kube-controller-manager`, and the default values are typically fine.  These settings are intended for use cases where other systems may create large numbers of pods over a period of many seconds or minutes and there is a desire to batch them together.

#### Batch Idle Duration

The batch idle duration duration is the period of time that a new pending pod extends the current batching window. This can be increased to handle scenarios where pods arrive slower than one second part, but it would be preferable if they were batched together onto a single larger node.

This value is expressed as a string value like `10s`, `1m` or `2h45m`. The valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.

#### Batch Max Duration

The batch max duration is the maximum period of time a batching window can be extended to. Increasing this value will allow the maximum batch window size to increase to collect more pending pods into a single batch at the expense of a longer delay from when the first pending pod was created.

This value is expressed as a string value like `10s`, `1m` or `2h45m`. The valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.