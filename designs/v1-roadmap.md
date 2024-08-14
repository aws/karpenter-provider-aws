# Karpenter v1 Roadmap

_This RFC is an extension of the [v1 Roadmap RFC](https://github.com/kubernetes-sigs/karpenter/blob/main/designs/v1-roadmap.md) that is merged in the [`kubernetes-sigs/karpenter` repo](https://github.com/kubernetes-sigs/karpenter)._

## Overview

Karpenter released the beta version of its APIs and features in October 2023. The intention behind this beta was that we would be able to determine the final set of changes and feature adds that we wanted to add to Karpenter before we considered Karpenter feature-complete. The list below details the features that Karpenter has on its roadmap before Karpenter becomes feature complete and stable at v1.

### Categorization

This list represents the minimal set of changes that are needed to ensure proper operational excellence, feature completeness, and stability by v1. For a change to make it on this list, it must meet one of the following criteria:

1. Breaking: The feature requires changes or removals from the API that would be considered breaking after a bump to v1
2. Stability: The feature ensures proper operational excellence for behavior that is leaky or has race conditions in the beta state
3. Planned Deprecations: The feature cleans-up deprecations that were previously planned the project

## Roadmap

1. [v1 APIs](./v1-api)
2. [Removing Ubuntu AMIFamily](#removing-ubuntu-amifamily)
3. [Change default TopologySpreadConstraint policy for Deployment from `ScheduleAnyways` to `DoNotSchedule`](#change-default-topologyspreadconstraint-policy-for-karpenter-deployment-from-scheduleanyways-to-donotschedule)
4. [Removing Implicit ENI Public IP Configuration](#removing-implicit-eni-public-ip-configuration)

### v1 APIs

**Issue Ref(s):** https://github.com/kubernetes-sigs/karpenter/issues/758, https://github.com/aws/karpenter-provider-aws/issues/5006

**Category:** Breaking, Stability

For Karpenter to be considered v1, the CustomResources that are shipped with an installation of the project also need to be stable at v1. Changes to Karpenter’s API (including labels, annotations, and tags) in v1 are detailed in [Karpenter v1 API](./v1-api.md). The migration path for these changes will ensure that customers will not have to roll their nodes or manually convert their resources as they did at v1beta1. Instead, we will leverage Kubernetes [conversion webhooks](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#webhook-conversion) to automatically convert their resources to the new schema format in code. The API groups and Kind naming will remain unchanged.

### Removing Ubuntu AMIFamily

**Issue Ref(s):** https://github.com/aws/karpenter-provider-aws/issues/5572

**Category:** Breaking

Karpenter has supported the Ubuntu AMIFamily [since the v0.6.2 version of Karpenter](https://github.com/aws/karpenter-provider-aws/pull/1323). EKS does not have formal support for the Ubuntu AMIFamily for MNG or SMNG nodes (it's currently a third-party vendor AMI). As a result, there is no direct line-of-sight between changes in things like supported Kubernetes versions or kernel updates on the image.

Users who still want to use Ubuntu can still use a Custom AMIFamily with amiSelectorTerms pinned to the latest Ubuntu AMI ID. They can reference `bootstrapMode: AL2` to get the same userData configuration they received before.

#### Tasks

- [ ] Drop the Ubuntu AMIFamily from the set of enum values in the v1 CRD
- [ ] Remove the Ubuntu bootstrapping logic from the Karpenter AMIFamily providers
- [ ] Remove the Ubuntu-specific AMIFamily documentation in the karpenter.sh documentation

### Change default TopologySpreadConstraint policy for Deployment from `ScheduleAnyways` to `DoNotSchedule`

**Category:** Stability, Breaking

Karpenter ships by default with multiple replicas and leader election enabled to ensure that it can run in HA (High Availability) mode. This ensures that if a pod goes down due to an outage, the other pod is able to recover quickly by shifting the leader election over.

Karpenter currently uses the `ScheduleAnyways` zonal topologySpreadConstraint to spread its Karpenter deployment across zones. Because this is a preference, this doesn't guarantee that pods will end up in different zones, meaning that, if there is a zonal outage, multiple replicas won't increase resiliency.

```yaml
topologySpreadConstraints:
  - labelSelector:
      matchLabels:
        app.kubernetes.io/instance: karpenter
        app.kubernetes.io/name: karpenter
    maxSkew: 1
    topologyKey: topology.kubernetes.io/zone
    whenUnsatisfiable: ScheduleAnyways
```

As part of v1, we are changing our default from `ScheduleAnyways` to `DoNotSchedule` to enforce stronger best practices by default to ensure that Karpenter can recover quickly in the event of a zonal outage. Users who still want the old behavior can opt back into `ScheduleAnyways` by overriding the default TopologySpreadConstraint.

#### Tasks

- [ ] Update Karpenter's zonal topologySpreadConstraint from `whenUnsatisfiable: ScheduleAnyways` to `whenUnsatisfiable: DoNotSchedule`

### Removing Implicit ENI Public IP Configuration

**Category:** Planned Deprecations, Breaking

Karpenter currently supports checking the subnets that your instance request is attempting to launch into and explicitly configuring that `AssociatePublicIPAddress: false` when you are only launching into private subnets. This feature was supported because users had specifically requested for it in https://github.com/aws/karpenter-provider-aws/issues/3815, where users were writing deny policies on their EC2 instance launches through IRSA policies or SCP for instances that attempted to create network interfaces that associated an IP address. Now with https://github.com/aws/karpenter-provider-aws/pull/5437 merged, we have the ability to set the `associatePublicIPAddress` value explicitly on the EC2NodeClass. Users can directly set this value to `false` and we will no longer need to introspect the subnets when making instance launch requests.

#### Tasks

- [ ] Remove the [`CheckAnyPublicIPAssociations`](https://github.com/aws/karpenter-provider-aws/blob/ea8ea0ecb042f4143e2948d4e299e169671841fe/pkg/providers/subnet/subnet.go#L97) call in our launch template creation at v1