---
title: "CapacityBuffers"
linkTitle: "CapacityBuffers"
weight: 50
description: >
  Understand CapacityBuffers and how they enable pre-provisioned spare capacity for reduced pod scheduling latency.
---

<i class="fa-solid fa-circle-info"></i> <b>Feature State: </b> [Alpha]({{<ref "../reference/settings#feature-gates" >}})

You can use CapacityBuffers to pre-provision spare node capacity so that your workloads can schedule instantly without waiting for new nodes to launch.
CapacityBuffers implement the Kubernetes SIG Autoscaling [CapacityBuffer API](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler/apis/capacitybuffer) (`autoscaling.x-k8s.io/v1alpha1`), providing compatibility with Cluster Autoscaler's buffer semantics.

A CapacityBuffer defines virtual placeholder pods that exist only in Karpenter's scheduling simulation — they are never created as real Kubernetes pods.
These virtual pods drive node provisioning with spare capacity, participate in every scheduling cycle to maintain the buffer, and automatically refill when real workloads consume the pre-provisioned capacity.

## CapacityBuffer Configuration

```yaml
apiVersion: autoscaling.x-k8s.io/v1alpha1
kind: CapacityBuffer
metadata:
  name: web-app-buffer
  namespace: default
spec:
  provisioningStrategy: "buffer.x-k8s.io/active-capacity"
  podTemplateRef:
    name: web-buffer-template
  replicas: 5
  limits:
    cpu: "20"
    memory: "40Gi"
```

## spec.provisioningStrategy

Defines how the buffer operates. Currently only `buffer.x-k8s.io/active-capacity` is supported, which continuously maintains spare capacity and reacts to workload changes. This is the default value if not specified.

## spec.podTemplateRef

Reference to a PodTemplate resource in the same namespace that defines the shape of a single buffer chunk. The PodTemplate's spec (containers, resource requests, nodeSelector, tolerations, affinity) is used to construct virtual pods for the scheduling simulation.

```yaml
apiVersion: v1
kind: PodTemplate
metadata:
  name: web-buffer-template
  namespace: default
template:
  spec:
    containers:
      - name: placeholder
        image: public.ecr.aws/eks-distro/kubernetes/pause:3.2
        resources:
          requests:
            cpu: "2"
            memory: "4Gi"
    nodeSelector:
      karpenter.sh/capacity-type: on-demand
    tolerations:
      - key: workload-type
        value: web
        effect: NoSchedule
```

{{% alert title="Note" color="primary" %}}
PVC-backed and ephemeral volumes in the PodTemplate are automatically stripped from virtual pods since no real PVC exists for topology resolution.
{{% /alert %}}

## spec.scalableRef

Reference to a workload with a scale subresource. Mutually exclusive with `podTemplateRef`. When set, the buffer uses the workload's pod template spec directly and can scale proportionally using `percentage`.

Supported kinds:
- `apps/v1/Deployment`
- `apps/v1/StatefulSet`
- `apps/v1/ReplicaSet`

```yaml
spec:
  scalableRef:
    apiGroup: apps
    kind: Deployment
    name: api-service
  percentage: 20
```

{{% alert title="Note" color="primary" %}}
The buffer reads the workload's `spec.template.spec` directly — no running pods are required for initialization. Changes to the workload spec are picked up within 30 seconds.
{{% /alert %}}

## spec.replicas

Fixed number of buffer chunks to provision. When used with `percentage` or `limits`, the minimum is taken.

## spec.percentage

Percentage of the `scalableRef`'s current replicas to maintain as buffer capacity. Only applicable when `scalableRef` is set. The absolute number is calculated by rounding up to a minimum of 1 when both the percentage and scalable replicas are greater than zero.

For example, if a Deployment has 10 replicas and `percentage` is 20, the buffer maintains 2 chunks.

## spec.limits

Resource constraints that cap the number of buffer chunks based on total resource requests. If no other constraints are set (`replicas` and `percentage` are both absent), limits alone determines how many chunks are created. When combined with other constraints, the minimum across all is used.

```yaml
spec:
  podTemplateRef:
    name: worker-template
  limits:
    cpu: "20"
    memory: "40Gi"
```

If each pod requests 2 CPU and 4Gi memory, this produces min(20/2, 40/4) = 10 buffer chunks.

## Replica Calculation

When multiple constraints are specified, the minimum across all of them is used:

| Configuration | Result |
|---|---|
| `replicas: 5` only | 5 |
| `percentage: 20` with 10-replica Deployment | 2 |
| `replicas: 10` + `limits: {cpu: 3}` (1 CPU per pod) | 3 (min of 10, 3) |
| `replicas: 5` + `percentage: 20` (10-replica Deployment) | 2 (min of 5, 2) |
| `limits: {cpu: 5}` only (1 CPU per pod) | 5 |

## Integration with Disruption

CapacityBuffers integrate with Karpenter's disruption system to ensure your buffer capacity is preserved:

* **Empty consolidation** is blocked for nodes hosting buffer capacity. Even though these nodes have no real pods, the virtual buffer pods prevent them from being treated as empty. The node is marked unconsolidatable with the reason `"Node has buffer pods"`.
* All other disruption methods (underutilized consolidation, drift, expiry) are allowed — replacement nodes must still fit virtual buffer pods, so the buffer is automatically maintained.

## Status and Observability

CapacityBuffers include status conditions to help you understand their current state and troubleshoot issues.

### Status Conditions

* **ReadyForProvisioning=True**: Pod template was successfully resolved and target replicas were calculated
* **ReadyForProvisioning=False**: Resolution failed — check the reason for details (PodTemplateNotFound, ScalableRefNotFound, ResolutionFailed)
* **Provisioning=True**: All virtual pods fit on existing cluster capacity without requiring new nodes
* **Provisioning=False**: Virtual pods require new capacity — provisioning is in progress or constrained by NodePool limits

### Example Status

```yaml
status:
  conditions:
    - type: ReadyForProvisioning
      status: "True"
      reason: Resolved
      message: "Pod template resolved successfully"
    - type: Provisioning
      status: "True"
      reason: FitsExistingCapacity
      message: "All 5 virtual pods fit on existing capacity"
  replicas: 5
  podTemplateRef:
    name: web-buffer-template
  podTemplateGeneration: 3
  provisioningStrategy: "buffer.x-k8s.io/active-capacity"
```

### Monitoring

The buffer controller reconciles every 30 seconds. Status conditions are updated on each reconcile, giving you visibility into:
- Whether the referenced PodTemplate or workload exists and is valid
- Whether capacity is currently satisfied or new nodes are being provisioned
- The current target replica count

## Limitations and Considerations

* **Volume stripping**: PVC-backed and ephemeral volumes are stripped from virtual pods since no real PVC exists for topology resolution
* **Polling for scalableRef changes**: The buffer controller requeues every 30 seconds — changes to a referenced Deployment's replica count take up to 30s to reflect in buffer status
* **NodePool limits**: Buffer virtual pods are subject to NodePool resource limits. If the NodePool's CPU or memory limit is exhausted, your buffer capacity cannot be fulfilled
* **Alpha status**: CapacityBuffers are currently in alpha (`autoscaling.x-k8s.io/v1alpha1`) and the API may change in future versions
