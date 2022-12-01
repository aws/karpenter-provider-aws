---
title: "Global Settings"
linkTitle: "Global Settings"
weight: 13
description: >
  Configure Karpenter
---

There are two main configuration mechanisms that can be used to configure Karpenter: Environment Variables / CLI parameters to the controller and webhook binaries and the `karpenter-global-settings` config-map.

## Environment Variables / CLI Flags

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
| MEMORY_LIMIT | \-\-memory-limit | Memory limit on the container running the controller. The GC soft memory limit is set to 90% of this value. (default = -1)|
| METRICS_PORT | \-\-metrics-port | The port the metric endpoint binds to for operating metrics about the controller itself (default = 8080)|
| WEBHOOK_PORT | \-\-webhook-port | The port the webhook endpoint binds to for validation and mutation of resources (default = 8443)|

[comment]: <> (end docs generated content from hack/docs/configuration_gen_docs.go)

## ConfigMap

Karpenter installs a default configuration via its Helm chart that should work for most.  Additional configuration can be performed by editing the `karpenter-global-settings` configmap within the namespace that Karpenter was installed in.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: karpenter-global-settings
  namespace: karpenter
data:
  # The maximum length of a batch window. The longer this is, the more pods we can consider for provisioning at one
  # time which usually results in fewer but larger nodes.
  batchMaxDuration: 10s
  # The maximum amount of time with no new ending pods that if exceeded ends the current batching window. If pods arrive
  # faster than this time, the batching window will be extended up to the maxDuration. If they arrive slower, the pods
  # will be batched separately.
  batchIdleDuration: 1s
  # [REQUIRED] The kubernetes cluster name for resource discovery
  aws.clusterName: karpenter-cluster
  # [REQUIRED] The external kubernetes cluster endpoint for new nodes to connect with
  aws.clusterEndpoint: https://00000000000000000000000000000000.gr7.us-west-2.eks.amazonaws.com
  # The default instance profile to use when provisioning nodes
  aws.defaultInstanceProfile: karpenter-instance-profile
  # If true, then instances that support pod ENI will report a vpc.amazonaws.com/pod-eni resource
  aws.enablePodENI: "false"
  # Indicates whether new nodes should use ENI-based pod density. DEPRECATED: Use `.spec.kubeletConfiguration.maxPods` to set pod density on a per-provisioner basis
  aws.enableENILimitedPodDensity: "true"
  # If true, then assume we can't reach AWS services which don't have a VPC endpoint
  # This also has the effect of disabling look-ups to the AWS pricing endpoint
  aws.isolatedVPC: "false"
  # The node naming convention (either "ip-name" or "resource-name")
  aws.nodeNameConvention: ip-name
  # The VM memory overhead as a percent that will be subtracted
  # from the total memory for all instance types
  aws.vmMemoryOverheadPercent: "0.075"
  # Interruption Handling is currently in ALPHA and is disabled by default. Enabling interruption handling may
  # require additional permissions on the controller service account. Additional permissions are outlined in the docs
  aws.interruptionQueueName: karpenter-cluster
  # Any global tag value can be specified by including the "aws.tags.<tag-key>" prefix
  # associated with the value in the key-value tag pair
  aws.tags.custom-tag: custom-tag-value
  aws.tags.custom-tag2: custom-tag-value
```

### Batching Parameters

The batching parameters control how Karpenter batches an incoming stream of pending pods.  Reducing these values may trade off a slightly faster time from pending pod to node launch, in exchange for launching smaller nodes.  Increasing the values can do the inverse.  Karpenter provides reasonable defaults for these values, but if you have specific knowledge about your workloads you can tweak these parameters to match the expected rate of incoming pods.

For a standard deployment scale-up, the pods arrive at the QPS setting of the `kube-controller-manager`, and the default values are typically fine.  These settings are intended for use cases where other systems may create large numbers of pods over a period of many seconds or minutes and there is a desire to batch them together.

#### `batchIdleDuration`

The `batchIdleDuration` is the period of time that a new pending pod extends the current batching window. This can be increased to handle scenarios where pods arrive slower than one second part, but it would be preferable if they were batched together onto a single larger node.

This value is expressed as a string value like `10s`, `1m` or `2h45m`. The valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.

#### `batchMaxDuration`

The `batchMaxDuration` is the maximum period of time a batching window can be extended to. Increasing this value will allow the maximum batch window size to increase to collect more pending pods into a single batch at the expense of a longer delay from when the first pending pod was created.

This value is expressed as a string value like `10s`, `1m` or `2h45m`. The valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.

### AWS Parameters

#### `aws.tags.<tag-key>`

Global tags are applied to __all__ AWS infrastructure resources deployed by Karpenter. These resources include:

- Launch Templates
- Volumes
- Instances

{{% alert title="Note" color="primary" %}}
Since you can specify tags at the global level and in the `AWSNodeTemplate` resource, if a key is specified in both locations, the `AWSNodeTemplate` tag value will override the global tag.
{{% /alert %}}
