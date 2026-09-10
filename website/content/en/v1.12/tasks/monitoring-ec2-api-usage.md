---
title: "Monitoring Amazon EC2 API Usage"
linkTitle: "Monitoring Amazon EC2 API Usage"
description: >
  Monitor Karpenter's Amazon EC2 API call volume and request throttling, and keep it within your account's request-rate limits
---

AWS throttling can impact Karpenter's ability to manage your cluster. Karpenter calls the Amazon EC2
API to discover infrastructure and to launch and terminate nodes, and Amazon EC2 enforces
per-account, per-Region request-rate limits. When your request rate exceeds a limit, Amazon EC2
rejects the excess requests with the `RequestLimitExceeded` error (HTTP 503).

The volume of these calls is not fixed. It scales with the number of `EC2NodeClass`es and clusters
you run, how often your cluster scales up and down, and the Karpenter version you run, so a large
enough fleet can generate enough requests to be throttled. Because this volume can scale, you should
monitor it. This task describes the Amazon EC2 APIs Karpenter calls, how to observe your call volume
and throttling, how to compare that volume against your account's request-rate limits, and other best
practices.

## What Amazon EC2 APIs Karpenter calls

| API | Category | When Karpenter calls it |
|-----|----------|-------------------------|
| `CreateFleet` | Launch (hot path) | Launching nodes to satisfy pending pods |
| `CreateLaunchTemplate` | Launch (hot path) | Preparing launch configuration for new nodes |
| `RunInstances` | Launch (hot path) | Launching nodes |
| `CreateTags` | Launch (hot path) | Tagging instances, fleets, and launch templates as they are created |
| `TerminateInstances` | Terminate | Removing nodes during consolidation, drift, or expiration |
| `DeleteLaunchTemplate` | Cleanup | Removing launch templates Karpenter manages |
| `DescribeSubnets` | Discovery / refresh | Resolving `subnetSelectorTerms` for each `EC2NodeClass` |
| `DescribeSecurityGroups` | Discovery / refresh | Resolving `securityGroupSelectorTerms` for each `EC2NodeClass` |
| `DescribeImages` | Discovery / refresh | Resolving `amiSelectorTerms` for each `EC2NodeClass` |
| `DescribeCapacityReservations` | Discovery / refresh | Resolving `capacityReservationSelectorTerms` (On-Demand Capacity Reservations) |
| `DescribeInstanceTypes`, `DescribeInstanceTypeOfferings` | Discovery / refresh | Resolving available instance types and their offerings |
| `DescribeSpotPriceHistory` | Discovery / refresh | Determining spot pricing for instance type selection |
| `DescribePlacementGroups` | Discovery | Resolving placement groups referenced by an `EC2NodeClass` |
| `DescribeInstances`, `DescribeInstanceStatus` | Discovery | Reconciling the state of launched instances |
| `DescribeLaunchTemplates` | Discovery | Reconciling the launch templates Karpenter manages |

## How to observe call volume and throttling

Karpenter exposes Prometheus metrics (by default at `:8080/metrics`, configurable via `METRICS_PORT`;
see the [Metrics reference]({{< relref "../reference/metrics" >}})). The AWS SDK request metrics are
the most direct measure of Karpenter's Amazon EC2 call volume and throttling. They are labeled by
`service` (for example, `EC2`), `action` (the API operation, for example `DescribeSubnets` or
`CreateFleet`), and `code` (the HTTP status code — `200` for success, and `503` for the
`RequestLimitExceeded` throttling response):

* `aws_sdk_go_request_total` — total AWS SDK requests, by `service`, `action`, and `code`.
* `aws_sdk_go_request_attempt_total` — total request attempts (a single request may make multiple
  attempts when retried).
* `aws_sdk_go_request_retry_count` — number of retry attempts per request. Sustained retries are an
  early indicator of throttling, because the AWS SDK retries throttled requests before they surface
  as an error.

For example, to graph Karpenter's Amazon EC2 request rate by operation:

```
sum by (action) (rate(aws_sdk_go_request_total{service="EC2"}[5m]))
```

To graph the throttled fraction of Karpenter's Amazon EC2 requests:

```
sum(rate(aws_sdk_go_request_total{service="EC2", code="503"}[5m]))
  / sum(rate(aws_sdk_go_request_total{service="EC2"}[5m]))
```

### See also

* Karpenter sets a User-Agent of `karpenter.sh-<version>` on its AWS SDK clients, so you can attribute
  Amazon EC2 API events to Karpenter in
  [AWS CloudTrail](https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-user-guide.html)
  by filtering `userAgent` for a value that begins with `karpenter.sh-`.

## How to compare against your account's Amazon EC2 API request-rate limits

Amazon EC2 API request-rate limits are enforced **per account, per Region**, and are independent for
different groups of actions (for example, the non-mutating `Describe*` actions are limited separately
from mutating actions). Review your account's applied limits with
[Service Quotas](https://docs.aws.amazon.com/servicequotas/latest/userguide/intro.html) for Amazon
EC2 in each Region where you run Karpenter, compare them against the request rate you observe from the
metrics above, and request an increase if your steady-state request rate is close to, or exceeds, your
limits. Because these limits are per account and per Region, the combined request volume of every
cluster in an account competes for the same limits.

## Best practices

### Use a multi-account architecture where clusters are isolated by account

Because request-rate limits are per account and per Region, concentrating a large number of clusters
in a single account concentrates all of their Amazon EC2 request volume against that one account's
limits. Use a multi-account architecture where clusters are isolated by account to spread the volume
across multiple accounts' limits. See the multi-account guidance in the
[AWS Well-Architected Framework](https://docs.aws.amazon.com/wellarchitected/latest/framework/welcome.html).
