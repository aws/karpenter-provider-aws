# Open ODCR Usage Strategy

This document proposes adding opt-in support for EC2 Fleet's `use-capacity-reservations-first` usage strategy to allow Karpenter to automatically consume open (non-targeted) On-Demand Capacity Reservations when launching on-demand instances.

- [Open ODCR Usage Strategy](#open-odcr-usage-strategy)
    * [Overview](#overview)
    * [Goals](#goals)
    * [Non-Goals](#non-goals)
    * [Background: Open vs. Targeted ODCRs](#background-open-vs-targeted-odcrs)
    * [API Changes](#api-changes)
        + [EC2NodeClass API](#ec2nodeclass-api)
    * [Implementation](#implementation)
        + [CreateFleet Changes](#createfleet-changes)
        + [LaunchTemplate Changes](#launchtemplate-changes)
    * [Interaction with `capacityReservationSelectorTerms`](#interaction-with-capacityreservationselectorterms)
    * [Scheduling Representation](#scheduling-representation)
    * [Drift Considerations](#drift-considerations)
    * [Pricing and Consolidation](#pricing-and-consolidation)
    * [Appendix: CreateFleet Behavior Matrix](#appendix-createfleet-behavior-matrix)

## Overview

The [ODCR design](odcr.md) introduced explicit capacity reservation selection through `capacityReservationSelectorTerms` in `EC2NodeClass`. That design intentionally excluded open ODCR handling and `use-capacity-reservations-first` due to interactions with Karpenter's drift mechanisms.

This RFC proposes addressing the open ODCR case with a separate, narrower opt-in. Many users hold open On-Demand Capacity Reservations and want their on-demand workloads to consume that reserved capacity — but they do not need Karpenter's full ODCR tracking, the `reserved` capacity type, consolidation-into-reservation behavior, or per-ODCR scheduling representation. They simply want EC2 Fleet to fill matching open ODCRs when they are available and fall back to standard on-demand when they are not.

EC2 Fleet supports exactly this behavior through `OnDemandOptions.CapacityReservationOptions.UsageStrategy = use-capacity-reservations-first`. When set, Fleet prioritizes matching open ODCRs before launching regular on-demand instances. Once all matching open ODCR capacity is exhausted, Fleet falls back to standard on-demand capacity automatically.

This RFC proposes an opt-in field on `EC2NodeClass.spec` — `preferOpenCapacityReservations` — that enables this strategy. When set, Karpenter passes `use-capacity-reservations-first` in `CreateFleet` for on-demand launches from that node class. Karpenter does not otherwise change how it tracks, prices, or schedules these instances: they remain visible as `on-demand` capacity type throughout the system.

## Goals

1. Allow users with open ODCRs to have Karpenter automatically consume that capacity on on-demand launches without requiring explicit reservation IDs or the full ODCR tracking feature.
2. Require no scheduling changes — instances launched into open ODCRs are still treated as on-demand throughout Karpenter's scheduling, consolidation, and drift logic.
3. Ensure the feature is opt-in and scoped per `EC2NodeClass`, so users who do not have open ODCRs pay no cost.
4. Avoid drift: instances that happen to land in open ODCRs must not trigger Karpenter's ODCR drift logic.
5. Be safe to combine with standard on-demand NodePools — if no matching open ODCR capacity exists, Fleet falls back to standard on-demand with no user action required.

## Non-Goals

1. Tracking or exposing which open ODCR an instance landed in.
2. Scheduling pods preferentially into open-ODCR instances vs. standard on-demand instances.
3. Consolidation from standard on-demand into open ODCR instances.
4. Pricing open-ODCR instances differently from standard on-demand (the instances remain billed at the on-demand rate; the reservation benefit is on the AWS account side).
5. Supporting `use-capacity-reservations-first` alongside `capacityReservationSelectorTerms` — the two features target different use cases and the combination is disallowed (see [Interaction with `capacityReservationSelectorTerms`](#interaction-with-capacityreservationselectorterms)).

## Background: Open vs. Targeted ODCRs

A capacity reservation's `instanceMatchCriteria` determines which instances may consume it:

- **Targeted**: only instances that explicitly target the reservation by ID (or via a Capacity Reservation Group) are placed into it. Karpenter's `capacityReservationSelectorTerms` feature handles this case.
- **Open**: any instance whose attributes (instance type, platform, AZ) match the reservation is automatically placed into it, without the instance needing to specify a target. This is what `use-capacity-reservations-first` makes Fleet prefer.

The two criteria have very different interactions with Karpenter. For targeted ODCRs, Karpenter tracks the reservation, models its count in offerings, and controls whether an instance ends up in it. For open ODCRs with this proposal, Karpenter delegates the placement decision entirely to EC2 Fleet and treats the resulting instance as plain on-demand.

## API Changes

### EC2NodeClass API

Add a new boolean field `preferOpenCapacityReservations` to `EC2NodeClass.spec`:

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: example-node-class
spec:
  # preferOpenCapacityReservations, when true, instructs Karpenter to pass
  # use-capacity-reservations-first to EC2 Fleet when launching on-demand instances
  # from this node class. Fleet will prefer any matching open ODCRs before falling
  # back to standard on-demand capacity. Instances that land in open ODCRs are still
  # reported as on-demand by Karpenter; no ODCR tracking or drift logic is applied.
  #
  # This field is mutually exclusive with capacityReservationSelectorTerms.
  # Defaults to false.
  preferOpenCapacityReservations: true | false
```

This field:
- Defaults to `false` (no change from current behavior).
- Is a `spec` field because it controls launch behavior.
- Is mutually exclusive with `capacityReservationSelectorTerms` — a CEL validation rule on `EC2NodeClassSpec` rejects the combination at admission time.
- Requires no `status` changes — Karpenter does not discover or track open ODCRs.
- Has no effect on spot launches (EC2 Fleet ignores `CapacityReservationOptions` for spot).
- Has no effect on `reserved` capacity type launches (those use targeted ODCRs via the launch template).

## Implementation

Implementing this feature requires changes in two places: the `CreateFleetInput` builder and the launch template builder. Both must cooperate — the Fleet-level strategy alone is not sufficient when the `ReservedCapacity` feature gate is enabled.

### CreateFleet Changes

When `preferOpenCapacityReservations: true` is set on an `EC2NodeClass` and the resolved capacity type for a `NodeClaim` is `on-demand`, Karpenter sets `CapacityReservationOptions` in `OnDemandOptions`:

```go
input.OnDemandOptions = &ec2types.OnDemandOptionsRequest{
    AllocationStrategy: ...,  // unchanged (lowest-price or prioritized)
    CapacityReservationOptions: &ec2types.CapacityReservationOptionsRequest{
        UsageStrategy: ec2types.FleetCapacityReservationUsageStrategyUseCapacityReservationsFirst,
    },
}
```

### LaunchTemplate Changes

When the `ReservedCapacity` feature gate is enabled (`LaunchModeTargeted`), Karpenter currently sets `CapacityReservationPreference: none` on all on-demand and spot launch templates. This is intentional: it prevents on-demand instances from accidentally landing in open ODCRs and triggering drift against `capacityReservationSelectorTerms`.

However, `CapacityReservationPreference: none` in a launch template takes precedence over `use-capacity-reservations-first` in CreateFleet options. If left unchanged, `preferOpenCapacityReservations` would be silently ineffective whenever the `ReservedCapacity` feature gate is on — Fleet would dutifully find matching open ODCRs and then be blocked from using them by the `none` preference in the launch template.

To fix this, the launch template builder must be made aware of `preferOpenCapacityReservations`. When this field is `true` on the EC2NodeClass, on-demand launch templates should omit `CapacityReservationPreference` entirely (falling back to the EC2 default of `open`) rather than setting it to `none`. This allows Fleet's `use-capacity-reservations-first` strategy to take effect.

Concretely, the existing guard in `CreateLaunchTemplateInputBuilder.Build()`:

```go
if b.LaunchMode(ctx) == LaunchModeTargeted {
    lt.LaunchTemplateData.CapacityReservationSpecification = &ec2types.LaunchTemplateCapacityReservationSpecificationRequest{
        CapacityReservationPreference: lo.Ternary(
            b.options.CapacityType == karpv1.CapacityTypeReserved,
            ec2types.CapacityReservationPreferenceCapacityReservationsOnly,
            ec2types.CapacityReservationPreferenceNone,
        ),
        ...
    }
}
```

needs to additionally skip setting `preference: none` for on-demand when `preferOpenCapacityReservations` is `true`:

```go
if b.LaunchMode(ctx) == LaunchModeTargeted {
    if b.options.CapacityType == karpv1.CapacityTypeReserved {
        lt.LaunchTemplateData.CapacityReservationSpecification = &ec2types.LaunchTemplateCapacityReservationSpecificationRequest{
            CapacityReservationPreference: ec2types.CapacityReservationPreferenceCapacityReservationsOnly,
            CapacityReservationTarget: &ec2types.CapacityReservationTarget{
                CapacityReservationId: &b.options.CapacityReservationID,
            },
        }
    } else if !b.options.PreferOpenCapacityReservations {
        // Only opt out of open ODCRs when we're not explicitly trying to use them.
        lt.LaunchTemplateData.CapacityReservationSpecification = &ec2types.LaunchTemplateCapacityReservationSpecificationRequest{
            CapacityReservationPreference: ec2types.CapacityReservationPreferenceNone,
        }
    }
}
```

This preserves the original behavior (opt-out of open ODCRs on OD/spot) unless the user has opted in to `preferOpenCapacityReservations`, in which case the `none` preference is omitted and EC2 Fleet's `use-capacity-reservations-first` strategy can take effect.

## Interaction with `capacityReservationSelectorTerms`

`preferOpenCapacityReservations` and `capacityReservationSelectorTerms` must not be used together on the same `EC2NodeClass`. There are two independent failure modes when this combination is attempted:

**Failure mode 1 — Fleet silently ignores targeted launch templates.** When a CreateFleet request includes both targeted ODCR launch templates (from `capacityReservationSelectorTerms`) and `use-capacity-reservations-first`, Fleet ignores the targeted launch templates entirely and only looks for open ODCRs. The targeted reservations are silently skipped with no error. This is documented in the appendix of [odcr.md](odcr.md).

**Failure mode 2 — `CapacityReservationPreference: none` overrides the usage strategy.** When `capacityReservationSelectorTerms` is set and the `ReservedCapacity` feature gate is enabled, Karpenter generates on-demand launch templates with `CapacityReservationPreference: none` to prevent accidental open ODCR assignment (which would cause drift). This `none` preference in the launch template takes precedence over `use-capacity-reservations-first` in the CreateFleet options, making `preferOpenCapacityReservations` silently ineffective.

Neither failure mode produces an error at launch time. Both produce quietly wrong behavior. Karpenter enforces mutual exclusivity as a CEL validation rule on `EC2NodeClassSpec`, rejecting the combination at admission time with a clear error message.

## Scheduling Representation

No scheduling changes are required. Karpenter does not add any new offerings, change pricing, or add new capacity types for this feature. Instances launched into open ODCRs via `use-capacity-reservations-first` have a `CapacityReservationPreference` of `open` (the EC2 default) and will not have `capacity-reservations-only` set; consequently, `capacityTypeFromInstance` will continue to classify them as `on-demand`.

This means the feature is transparent to the scheduler: from Karpenter's perspective, whether or not a launched instance ended up in an open ODCR is invisible. The AWS account accrues the reservation benefit, but Karpenter's scheduling, consolidation, and drift behavior are unaffected.

## Drift Considerations

Because Karpenter treats instances launched with `preferOpenCapacityReservations` as plain on-demand, no new drift conditions are introduced:

- The instance's `karpenter.sh/capacity-type` label will be `on-demand`.
- Karpenter will not inspect `CapacityReservationId` on these instances during drift checks.
- If the open ODCR that the instance landed in expires or is cancelled, EC2 detaches the instance from the reservation and bills it at the standard on-demand rate. Karpenter is unaffected: the instance was always `on-demand` from its perspective.
- There is no "drift to ODCR / drift away from ODCR" cycle of the kind described in the existing ODCR design, because Karpenter never tracks the ODCR assignment in the first place.

This is the key difference from the open ODCR concerns in `odcr.md`: that drift problem arose because `capacityReservationSelectorTerms` caused Karpenter to track and enforce ODCR membership. With `preferOpenCapacityReservations`, Karpenter deliberately does not track membership, so there is nothing to drift against.

## Pricing and Consolidation

No pricing changes. Open-ODCR instances are priced identically to standard on-demand instances within Karpenter's model. The on-demand hourly rate is used for scheduling and consolidation decisions.

Consolidation behavior is unchanged. Karpenter will consolidate open-ODCR instances the same way it consolidates standard on-demand instances. Since Karpenter does not know which instances are in open ODCRs, it may consolidate away an instance that is in a reservation. This is acceptable — the ODCR capacity remains available for the next on-demand launch, which will also prefer it via `use-capacity-reservations-first`.

## Appendix: CreateFleet Behavior Matrix

The following table (extended from `odcr.md`) shows how `preferOpenCapacityReservations` interacts with other configuration. The `CapacityReservationPreference` column reflects what Karpenter sets in the launch template; "EC2 default (`open`)" means Karpenter does not set the field.

| `capacityReservationSelectorTerms` | `preferOpenCapacityReservations` | `usageStrategy` sent to Fleet | `CapacityReservationPreference` in LT | Result |
|---|---|---|---|---|
| Not set | `false` (default) | none | EC2 default (`open`) — or `none` if `ReservedCapacity` gate is on | Current behavior. Fleet may incidentally land in open ODCRs but does not prefer them. When the `ReservedCapacity` gate is on, instances actively opt out of open ODCRs. |
| Not set | `true` | `use-capacity-reservations-first` | EC2 default (`open`) | **This proposal.** Fleet prefers matching open ODCRs before standard on-demand. Instances remain `on-demand` in Karpenter. |
| Set | `false` | none | `capacity-reservations-only` (reserved LTs) / `none` (OD/spot LTs) | Existing targeted ODCR behavior. Instances are tracked as `reserved`. |
| Set | `true` | — | — | **Validation error.** Mutually exclusive. |
