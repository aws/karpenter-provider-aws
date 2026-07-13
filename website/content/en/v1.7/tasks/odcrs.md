---
title: "Utilizing On-Demand Capacity Reservations and Capacity Blocks"
linkTitle: "Utilizing ODCRs and Capacity Blocks"
---

<i class="fa-solid fa-circle-info"></i> <b>Feature State: </b> [Beta]({{<ref "../reference/settings#feature-gates" >}})

Karpenter introduced native support for [EC2 On-Demand Capacity Reservations](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-capacity-reservations.html)  (ODCRs) in [v1.3](https://github.com/aws/karpenter-provider-aws/releases/tag/v1.3.0), enabling users to select upon and prioritize specific capacity reservations.
In [v1.6](https://github.com/aws/karpenter-provider-aws/releases/tag/v1.6.2), this support was expanded to include [EC2 Capacity Blocks for ML](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-capacity-blocks.html).
To enable native ODCR support, ensure the [`ReservedCapacity` feature gate]({{< relref "../reference/settings#feature-gates" >}}) is enabled.

{{% alert title="Note" color="primary" %}}
If you were previously utilizing `open` ODCRs using Karpenter, review the [migration section]({{< relref "#migrating-from-previous-versions" >}}) of this task before enabling this feature.
{{% /alert %}}

## Selecting Capacity Reservations

To configure native ODCR support, you will need to make updates to both your EC2NodeClass and NodePool.
First, you should configure `capacityReservationSelectorTerms` on your EC2NodeClass.
Similar to `amiSelectorTerms`, you can specify a number of terms which are ANDed together to select ODCRs in your AWS account.
The following example demonstrates how to select all capacity reservations tagged with `application: foobar` in addition to `cr-56fac701cc1951b03`:

```yaml
capacityReservationSelectorTerms:
- tags:
    application: foobar
- id: cr-56fac701cc1951b03
```

{{% alert title="Note" color="primary" %}}
Capacity blocks are modeled as on-demand capacity reservations in EC2.
To select capacity blocks, specify them in your `capacityReservationSelectorTerms` in the same way you would for a default ODCR.
{{% /alert %}}

For more information on configuring `capacityReservationSelectorTerms`, see the [NodeClass docs]({{< relref "../concepts/nodeclasses#speccapacityreservationselectorterms" >}}).

Additionally, you will need to update your NodePool to be compatible with ODCRs.
Karpenter doesn't model ODCRs as standard on-demand capacity, and instead uses a dedicated capacity type: `reserved`.
For a NodePool to utilize ODCRs, it must be compatible with `karpenter.sh/capacity-type: reserved`.
The following example demonstrates how to configure a NodePool to prioritize ODCRs and fallback to on-demand capacity:

```yaml
requirements:
- key: karpenter.sh/capacity-type
  operator: In
  values: ['reserved', 'on-demand']
```

Additionaly, Karpenter supports the following scheduling labels:

| Label                                         | Example                       | Description                      |
| --------------------------------------------- | ----------------------------- | -------------------------------- |
| `karpenter.k8s.aws/capacity-reservation-id`   | `cr-56fac701cc1951b03`        | The capacity reservation's ID    |
| `karpenter.k8s.aws/capacity-reservation-type` | `default` or `capacity-block` | The type of capacity reservation |

These labels will only be present on reserved nodes.
They are supported as NodePool requirements and as pod scheduling constaints (e.g. [node affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity)).

{{% alert title="Warning" color="warning" %}}
Karpenter does **not** support open matching for ODCRs.
This means that all ODCRs you wish to utilize, even those with `open` instance eligibility, must be included in your NodeClass' `spec.capacityReservationSelectorTerms`.
{{% /alert %}}

## Prioritization Behavior

NodePools are not limited to a single compatible capacity-type -- they can be compatible with any combination of the available capacity-types (`on-demand`, `spot`, and `reserved`).
Consider the following NodePool requirements:

```yaml
requirements:
- key: karpenter.sh/capacity-type
  operator: In
  values: ['reserved', 'on-demand', 'spot']
```

In this example, the NodePool is compatible with all capacity types.
Karpenter will prioritize ODCRs, but if none are available or none are compatible with the pending workloads it will fallback to spot or on-demand.
Similarly, Karpenter will prioritize reserved capacity during consolidation.
Since ODCRs are pre-paid, Karpenter will model them as free and consolidate spot / on-demand nodes when possible.

## Expiration

An instance launched into an ODCR is not necessarily in that ODCR indefinitely.
The ODCR could expire, be cancelled, or the instance could be manually removed from the ODCR.
If any of these occur, and Karpenter detects that the instance no longer belongs to an ODCR, it will update the `karpenter.sh/capacity-type` label to `on-demand`.

### Capacity Blocks

Unlike default ODCRs, Capacity Blocks must have an end time.
Additionally, instances launched into a capacity block will be terminated by EC2 ahead of the end time, rather than becoming standard on-demand capacity.

From the [AWS docs](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-capacity-blocks.html):

> You can use all the instances you reserved until 30 minutes (for instance types) or 60 minutes (for UltraServer type) before the end time of the Capacity Block.
> With 30 minutes (for instance types) or 60 minutes (for UltraServer types) left in your Capacity Block reservation, we begin terminating any instances that are running in the Capacity Block.
> We use this time to clean up your instances before delivering the Capacity Block to the next customer.

Karpenter will preemptively begin draining nodes launched for capacity blocks 10 minutes before EC2 begins termination, ensuring your workloads can gracefully terminate before reclaimation.

## Migrating From Previous Versions

Although it was not natively supported, it was possible to utilize ODCRs on previous versions of Karpenter.
If a NodeClaim's requirements happened to be compatible with an open ODCR in the target AWS account, it may have launched an instance into that open ODCR.
This could be ensured by constraining a NodePool such that it was only compatible with the desired open ODCR, and limits could be used to enable fallback to a different NodePool once the ODCR was exhausted.
This behavior is no longer supported when native on-demand capacity support is enabled.

If you were relying on this behavior, you should configure your `EC2NodeClasses` to select the desired ODCRs **before** enabling the feature gate.
You should also ensure any NodePools which you wish to use with ODCRs are compatible with `karpenter.sh/capacity-type: reserved`.
Performing these steps before enabling the feature gate will ensure that Karpenter can immediately continue utilizing your reservations, rather than falling back to on-demand.
