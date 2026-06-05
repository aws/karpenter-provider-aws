# Zonal Shift RFC

## Background

Occasionally, zones in cloud providers can experience temporary outages. These outages can be partial failures or complete outages of any number of dependencies that clusters require including networking, compute, authentication, and more. During these events, Karpenter's actions do not improve its cluster's availability posture and can sometimes exacerbate the scenario.

While detecting these outages is outside the scope of this RFC, Karpenter should provide the ability to integrate with solutions that do and modify its behavior to ensure that it does not exacerbate any zonal outages.

## Technical Requirements 

1. Stop provisioning capacity in the **impaired** AZ
2. Stop performing voluntary disruption in the **impaired** AZ.
3. Stop performing voluntary disruption in the **unimpaired** AZs if the disruption relies on scheduling pods to the **impaired** AZ.
4. Pods with strict scheduling requirements that require capacity in the impaired AZ such as volume requirements or node affinities **should not** result in launch attempts
5. If an option is set, pods with TSCs that require capacity in the impaired AZ should instead have capacity launched into unimpaired AZs while still maintaining skew between the remaining unimpaired AZs.

# Recommended Option: Provider-only Implementation

Because the EKS Zonal Shift button already taints nodes in the impaired AZ, a Karpenter or Auto managed cluster that has been Zonally Shifted will already meet the technical requirement for `3`, because the nodes cannot have pods scheduled to them due to the aforementioned taint.

This option does not meet requirement `5`. kube-scheduler changes are necessary to meet requirement 5. See https://docs.google.com/document/d/1elP211dNvUXCtAn5alW4qGzGnY0s4K8_e4X6p640-5E/edit?tab=t.0#heading=h.8l5g85o4cda3

## Mechanism

To meet requirements `1` , `2`, and  `4` during a zonal shift the aws and auto providers will set all of the offerings in the impaired zone to Unavailable while a zonal shift is active.

```
type Offering struct {
    Requirements        scheduling.Requirements
    Price               float64
    Available           bool // set to false during a zonal event
    ReservationCapacity int
    priceOverlayApplied bool
}
```

## Observability

### Metrics

Karpenter will emit metrics that indicate which zones have been marked as impaired, and will log when the state of zonal behavior changes. It will not log each time it decides to not take an action to prevent spamming the log with entries during an event. 

Karpenter will emit a new metric, `karpenter_cloudprovider_zonal_shift_duration` that will indicates how long a zonal shift has been in progress. This metric will be dimensioned with the zone in question and if the shift is manual or automatic so users are able to understand overlapping zonal shifts in multiple zones.

### Events

Karpenter could event against nodepools that allow instances in the impaired AZ to indicate that new nodes cannot be provisioned in a given AZ. This is not required for an initial release, but could be a nice follow up.

## Enablement

Zonal Shift Support will be disabled by default with an opt in flag for alpha release. Users who choose to configure this behavior will pass in an [environment variable or CLI flag](https://karpenter.sh/docs/reference/settings/) to the Karpenter binary that indicates if they wish to enable Zonal Shift.

The flag will be called `ENABLE_ZONAL_SHIFT` or `--enable-zonal-shift` , and will accept a boolean value.

The downside to this approach is that customers who wish to quickly disable this behavior during an event will need to restart their Karpenter process to do so.

The decision to not enable this feature at the NodePool or NodeClass level is purposeful. It simplifies Karpenter’s behavior considerably to have the modifications be uniform across the cluster. Unless there is a strong use case for failing away some nodepools but not others, this should be kept as a cluster level setting. 

## Source of Truth for Zonal Shifts

Karpenter will need to detect when a ZonalShift is activated, deactivated, or expires.

### Option 1: GetManagedResource Now, EventBridge Later (recommended)

Karpenter relies on GetManagedResource now to build a simple and operationally sound interface, then later we can perform the additional work to support EventBridge events if users experience TPS issues.

### Option  2: GetManagedResource  only

A Zonal Shift Provider will be created. The provider will be responsible for tracking zonal shifts in ARC, and will be used by the Offerings Cache to determine offering availability.

The Zonal Shift Provider will regularly exercise the ARC [GetManagedResource API](https://docs.aws.amazon.com/arc-zonal-shift/latest/api/API_ListZonalShifts.html)with the resource arn of the EKS cluster and maintain an in-memory store of the state of zonal shifts, as well as an aggregated state of the list of impaired zones. 

When a new Zonal Shift is returned from the API, the provider will verify that the ShiftType is correct and that the shift applies to the EKS cluster that Karpenter manages. If the Zonal Shift passes validation, it will be added to the in memory store of the state of zonal shifts, and the aggregated state will be re-computed.

When a Zonal Shift expires as per its ExpiryTime, it will be evicted from the in memory store and the aggregated state will be re-computed using the in memory store.

If a subsequent response of GetManagedResource updates the Expiry or cancels a zonal shift, Karpenter will update it's in memory store to match the state of the world.

When the provider’s GetInstanceTypes() function is exercised, the availability of offerings will be updated with zonal shift information.

#### Modifications to Permissions

Karpenter will need to be given permissions to GetManagedResource. Users will need to update their [ControllerRole](https://karpenter.sh/docs/reference/cloudformation/).

### Option 3: EventBridge\SQS only

Karpenter is already made aware of some EventBridge events via an SQS queue, notably spot interruption events. This queue could be supplemented to also consume Zonal Shift events. The SQS provider can be supplemented to update the Offering Cache.

https://docs.aws.amazon.com/eventbridge/latest/ref/events-ref-arc-zonal-shift.html
https://docs.aws.amazon.com/r53recovery/latest/dg/eventbridge-zonal-autoshift.html

These events return the zone-id, which we can translate to zone using the subnet data the same way we do for offerings.

#### Benefits:

This allows Karpenter to call GetManagedResource less frequently

#### Drawbacks:

EventBridge events are best effort, which means that Karpenter may miss some events, or get some events late. EventBridge does not have any SLAs on event delivery time. 
