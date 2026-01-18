# Deprovisioning - Drift

This will be a living document, where Karpenter details the key design choices for Drift, Karpenter's mechanism to assert created machines match the corresponding CRDs. At its inception, this doc only included AWS fields. Future cloud providers should add to this document to add to the table below, as well as in their respective website.

## Problem (v0.21.0 - v0.28.x)

Provisioners and Provider-specific CRDs (AWSNodeTemplates in AWS) are [declarative APIs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#declarative-apis) that dictate the desired state of machines. User’s requirements for their machines as reflected in these CRDs can change over time. For example, users can add or remove labels or taints from their nodes, modify their machine requirements in their Provisioner, or modify the discovered networking resources in their providerRef. To enforce that requirements set in CRDs are applied to their fleet, users must manually terminate all out-of-spec machines to rely on Karpenter to provision in-spec replacements.

Karpenter’s drift feature automates this process by automatically (1) detecting machines that have drifted and (2) safely replacing them with the [standard deprovisioning flow](https://karpenter.sh/preview/concepts/deprovisioning/#control-flow).

## Recommended Solution

Karpenter automatically detects when machines no longer match their corresponding specification in the Provisioner and ProviderRef. When this occurs, Karpenter triggers the standard deprovisioning workflow on the machine. Karpenter Drift will not add API, it will only build on top of existing APIs, and be enabled by default. More on this decision below in [API Choices](#🔑-api-choices).

Karpenter Drift will classify each CRD field as a (1) Static, (2) Dynamic, or (3) Behavioral field and will treat them differently. Static Drift will be a one-way reconciliation, triggered only by CRD changes. Dynamic Drift will be a two-way reconciliation, triggered by machine/node/cloud provider changes and CRD changes.

(1) For Static Fields, values in the CRDs are reflected in the machine in the same way that they’re set. A machine will be detected as drifted if the values in the CRDs do not match the values in the machine. Yet, since some external controllers directly edit the associated cloud provider machine or node, this is expanded to be more flexible below in [Static Field Flexibility](#🔑-in-place-drift).

(2) Dynamic Fields can correspond to multiple values and must be handled differently. Dynamic fields can create cases where drift occurs without changes to CRDs, or where CRD changes do not result in drift.

For example, if a machine has `node.kubernetes.io/instance-type: m5.large`, and requirements change from `node.kubernetes.io/instance-type In [m5.large]` to `node.kubernetes.io/instance-type In [m5.large, m5.2xlarge]`, the machine will not be drifted because it's value is still compatible with the new requirements. Conversely, for an AWS Installation, if a machine is using a machine image `ami: ami-abc`, but a new image is published, Karpenter's `AWSNodeTemplate.amiSelector` will discover that the new correct value is `ami: ami-xyz`, and detect the machine as drifted.

(3) Behavioral Fields are treated as over-arching settings on the Provisioner to dictate how Karpenter behaves. These fields don’t correspond to settings on the machine or instance. They’re set by the user to control Karpenter’s Provisioning and Deprovisioning logic. Since these don’t map to a desired state of machines, these fields will not be considered for Drift.

Each of the currently defined fields in Karpenter's existing CRDs will be classified as follows (AWS Fields included here as well):

```
|                            | Static | Dynamic | Behavioral |
|----------------------------|--------|---------|------------|
| - Provisioner Fields -     |   ---  |   ---   |     ---    |
| Startup Taints             |    x   |         |            |
| Taints                     |    x   |         |            |
| Labels                     |    x   |         |            |
| Annotations                |    x   |         |            |
| Node Requirements          |        |    x    |            |
| Kubelet Configuration      |    x   |         |            |
| Weight                     |        |         |      x     |
| Limits                     |        |         |      x     |
| Consolidation              |        |         |      x     |
| TTLSecondsUntilExpired     |        |         |      x     |
| TTLSecondsAfterEmpty       |        |         |      x     |
| - AWS Fields -             |   ---  |   ---   |     ---    |
| Subnet Selector            |        |    x    |            |
| Security Group Selector    |        |    x    |            |
| Instance Profile           |    x   |         |            |
| AMI Family/AMI Selector    |        |    x    |            |
| UserData                   |    x   |         |            |
| Tags                       |    x   |         |            |
| Metadata Options           |    x   |         |            |
| Block Device Mappings      |    x   |         |            |
| Detailed Monitoring        |    x   |         |            |
|                            |        |         |            |
```

Design Questions

## 🔑 API Choices

Drift is an existing mechanism in Karpenter and relies on standard Provisioning CRDs in Karpenter - Provisioners and AWSNodeTemplates. Drift is toggled through a feature gate in the `karpenter-global-settings` ConfigMap.

Once Drift is implemented as described above, the absence of the feature gate will be enabled Drift by default. To allow users to opt-out, users will be able to configure their `karpenter-global-settings` ConfigMap to disable this. There will be no API additions.

In the future, users may want Karpenter to not Drift certain fields. This promotes users to not rely on CRDs as declarative sources of truth, breaking the contract that Drift defines. If a user request to opt out a field from Drift is validated, Karpenter should first rely on other Deprovisioning control mechanisms to enable this. Otherwise, Karpenter could consider adding a setting in the `karpenter-global-settings` ConfigMap to selectively toggle this.

## 🔑 Static Field Flexibility

### Problem

Users that run other controllers in their clusters that edit machines directly like Cilium and NVIDIA GPU Operator may find Drift too unstable for Static Fields, since Karpenter will drift machines that don’t exactly match the CRDs. If a user has an external controller that edit labels on the machine, or edit tags on the instance, Karpenter will drift those machines any time this external controller makes changes. Karpenter should work in tandem with external systems that users run in their clusters.

For instance, if an external controller adds a label to a node for cost monitoring purposes on startup, without static field flexibility, Karpenter would see that the label map on the node is not equal to what's defined in its respective Provisioner, marking the node as drifted. Karpenter would deprovision the node and spin up a replacement, only to be labeled by the controller again. This loop would continue until the controller stops, creating crippling node churn, making users unable to use external controllers that modify nodes in the same cluster as Karpenter.

### Solution

Karpenter will only consider Static Drift on machines when the respective CRDs change. External systems and users can edit the machines and instances as they please, as long as they don’t edit Dynamic Fields. This allows external systems to modify nodes and instances, but keep the drift automation of rolling out changes to settings to their fleet. Furthermore, triggering Drift for Static Fields only on CRD changes allows users to safely control when their fleet will be refreshed.

As a reference, this is how Kubernetes Deployments work. A user configures a `PodTemplateSpec` in the `DeploymentSpec` to reflect a desired state of the pods it manages. A user can edit a pod’s metadata without invoking Deployment drift. As a result, Deployment drift will only occur if a user edits the Deployment’s `PodTemplateSpec`.

## 🔑 In-place Drift

Some machine settings that are generated from the fields in the Provisioner and ProviderRef could be drifted in-place.

For example, Kubernetes supports editing the metadata of nodes (e.g. labels, annotations) without having to delete and recreate it. If a user adds a new distinct key-value pair to their Provisioner labels, Karpenter could simply add the label in-place, without having to rely on the [standard deprovisioning flow](https://karpenter.sh/preview/concepts/deprovisioning/#control-flow). Yet, modifying labels after nodes and pods after joined the cluster may have unexpected interactions with scheduling constraints, since previous scheduling decisions were not based on the added/removed labels.

The same would be possible if a user adds a new distinct key-value pair to their AWSNodeTemplate tags. Yet, in-place tagging may fail as there are tag limits on AWS EC2 instances.

Since each CRD field comes with different use-cases, and the standard deprovisioning flow is to replace machines that have drifted, any special cases of in-place drift mechanism will need to be separately designed and implemented as special deprovisioning logic. For initial design and implementation, Drift will only replace machines, and in-place drift can be designed in the future.
