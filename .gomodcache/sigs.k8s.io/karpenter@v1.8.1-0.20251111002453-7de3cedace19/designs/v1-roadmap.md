# Karpenter v1 Roadmap

## Overview

Karpenter released the beta version of its APIs and features in October 2023. The intention behind this beta was that we would be able to determine the final set of changes and feature adds that we wanted to add to Karpenter before we considered Karpenter feature-complete. The list below details the features that Karpenter has on its roadmap before Karpenter becomes feature complete and stable at v1.

### Categorization

This list represents the minimal set of changes that are needed to ensure proper operational excellence, feature completeness, and stability by v1. For a change to make it on this list, it must meet one of the following criteria:

1. Breaking: The feature requires changes or removals from the API that would be considered breaking after a bump to v1
2. Stability: The feature ensures proper operational excellence for behavior that is leaky or has race conditions in the beta state
3. Planned Deprecations: The feature cleans-up deprecations that were previously planned the project

## Roadmap

1. [v1 APIs](./v1-api.md)
2. [Update `karpenter.sh/disruption=disrupting:NoSchedule` taint to `karpenter.sh/disrupted:NoSchedule`](#update-karpentershdisruptiondisruptingnoschedule-taint-to-karpentershdisruptednoschedule)
3. [Add the `karpenter.sh/unregistered:NoExecute` taint on registration](#add-the-karpentershunregisterednoexecute-taint-on-registration)
4. [Wait for Instance Termination on NodeClaim/Node Deletion](#wait-for-instance-termination-on-nodeclaimnode-deletion)
5. [Drift Hash Breaking Change Handling](#drift-hash-breaking-change-handling)
6. [Introduce ConsolidateAfter for Consolidation Controls](#introduce-consolidateafter-for-consolidation-controls)
7. [Change Expiration from Graceful to Forceful](#change-expiration-from-graceful-to-forceful)
8. [Define SemVer Versioning Policy for `kubernetes-sigs/karpenter` Library](#define-semver-versioning-policy-for-kubernetes-sigskarpenter-library)
9. [Stabilize Observability (metrics, status, eventing)](#stabilize-observability-metrics-status-eventing)
10. [NodeClaim Conceptual Documentation](#nodeclaim-conceptual-documentation)
11. [Drop Knative References from the Code](#drop-knative-references-from-the-code)
12. [Migrate Knative Webhook away from Karpenter](#drop-knative-webhook-from-karpenter)
13. [Karpenter Global Logging Configuration Changes](#karpenter-global-logging-configuration-changes)
13. [Promoting Drift Feature to Stable](#promoting-drift-feature-to-stable)

### v1 APIs

**Issue Ref(s):** https://github.com/kubernetes-sigs/karpenter/issues/758, https://github.com/aws/karpenter-provider-aws/issues/5006

**Category:** Breaking, Stability

For Karpenter to be considered v1, the CustomResources that are shipped with an installation of the project also need to be stable at v1. Changes to Karpenter’s API (including labels, annotations, and tags) in v1 are detailed in [Karpenter v1 API](./v1-api.md). The migration path for these changes will ensure that customers will not have to roll their nodes or manually convert their resources as they did at v1beta1. Instead, we will leverage Kubernetes [conversion webhooks](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#webhook-conversion) to automatically convert their resources to the new schema format in code. The API groups and Kind naming will remain unchanged.

### Update `karpenter.sh/disruption=disrupting:NoSchedule` taint to `karpenter.sh/disrupted:NoSchedule`

**Issue Ref:** https://github.com/kubernetes-sigs/karpenter/issues/624, https://github.com/kubernetes-sigs/karpenter/issues/1049

**Category:** Breaking

Karpenter currently uses the `karpenter.sh/disruption` key with the `disrupting` value to signal when nodes are actively being disrupted by the disruption controller. This key was originally added with a value because it was assumed that we would expand upon this taint with additional values over time. As part of the v1 stability, we now realize that this behavior is inconsistent with the usage of taint keys in Kubernetes generally. Rather than keeping the `karpenter.sh/disruption` key and differentiating based on values, we will be creating different taint keys for each unique action that Karpenter takes (e.g. `karpenter.sh/disrupted` and `karpenter.sh/unregistered`) This update also changes the taint key from `karpenter.sh/disrupting` to `karpenter.sh/disrupted`. 

#### Tasks

- [ ] Update the `karpenter.sh/disruption=disrupting:NoSchedule` taint to be `karpenter.sh/disrupted:NoSchedule`

### Add the `karpenter.sh/unregistered:NoExecute` taint on registration

**Issue Ref:** https://github.com/kubernetes-sigs/karpenter/issues/624, https://github.com/kubernetes-sigs/karpenter/issues/1049

**Category:** Breaking

Karpenter currently adds taints, labels, and annotations to nodes from the NodeClaim when nodes register with the cluster. This is done without restricting pods from scheduling to the node which means that for any labels or taints that aren't present on the node on startup, this process or registration can break scheduling compatability.

For instance, a pod may not tolerate a specific taint that was configured in the NodeClaim template, but the taint did not appear on the node immediately on startup, meaning that the pod was able to successfully schedule when it shouldn't have been able to.

To mitigate this, Karpenter is introducing the `karpenter.sh/unregistered:NoExecute` taint at v1. Nodes registering with Karpenter will be required to have this taint present on their nodes when the node first joins. Karpenter will propagate lables, annotations, and taints, and remove this `karpenter.sh/unregistered` taint simultaneously, allowing Karpenter to ensure that pods will not schedule against the node before it has had a chance to add configuration.

#### Tasks

- [ ] Add `karpenter.sh/unregistered:NoExecute` taint into the cloudprovider bootstrap config (Kubelet's `--register-with-taints` parameter)
- [ ] Remove the `karpenter.sh/unregistered:NoExecute` taint as part of the NodeClaim lifecycle registration flow
- [ ] Validate the `karpenter.sh/unregistered:NoExecute` taint exists for nodes owned by Karpenter

### Wait for Instance Termination on NodeClaim/Node Deletion

**Issue Ref(s):** https://github.com/kubernetes-sigs/karpenter/issues/655, https://github.com/kubernetes-sigs/karpenter/issues/947

**Category:** Stability

Karpenter currently leaks DaemonSet pods and leases when it is terminating instances. This occurs because Karpenter currently initiates a Delete() operation once on the CloudProvider but does not continually check that the instance is fully terminated. For AWS and Azure, terminating the instance simply starts a shutdown procedure for the instance, meaning that kubelet can continue to reach out to the node until the instance is fully shutdown and terminated.

Because Karpenter is not waiting, the node fails to properly deregister, leaking [daemonsets](https://github.com/kubernetes-sigs/karpenter/issues/655) and [node leases](https://github.com/aws/karpenter-provider-aws/issues/4363) onto the cluster. By waiting for instance termination before fully deleting the node, we are allowing the node to go through its graceful shutdown process.

#### Tasks

- [ ] Implement a retry mechanism to ensure that the instance is fully terminated before removing the Node and the NodeClaim from the cluster
- [ ] Validate and remove the current lease garbage collection controller from `kubernetes-sigs/karpenter`, removing the permission on node leases in the `kube-node-lease` namespace

### Drift Hash Breaking Change Handling

**Issue Ref(s):** https://github.com/kubernetes-sigs/karpenter/issues/957

**Category:** Stability

Karpenter currently relies on a hash to determine whether certain fields in Karpenter’s NodeClaims have drifted from their owning NodePool and owning EC2NodeClass. Today, this is determined by hashing a set of fields on the NodePool or EC2NodeClass and then validating that this hash still matches the NodeClaim’s hash.

This hashing mechanism works well for additive changes to the API, but does not work well when adding fields to the hashing function that already have a set value on a customer’s cluster. In particular, we have a need to make breaking changes to this hash scheme from these two issues: https://github.com/kubernetes-sigs/karpenter/issues/909 and https://github.com/aws/karpenter-provider-aws/issues/5447.

We need to implement a common way for handing breaking changes to our hashing logic ahead of v1 so we can make the requisite changes that we need to make to the v1 APIs at v1 as well as handle breaking changes through defaults to our drift hashing API moving forward post-v1 e.g. we introduce an alpha field that affects our static hashing and then drop it later.

#### Tasks

- [ ] Design and implement a hash version-style implementation that allows us to make breaking changes to the hash versioning scheme

### Introduce ConsolidateAfter for Consolidation Controls

**Issue Ref(s):** https://github.com/kubernetes-sigs/karpenter/issues/735

**Category**: Breaking, Stability

Karpenter currently evaluates all forms of disruption in a synchronous loop, starting with expiration, drift, emptiness, and considering consolidation and multi-node consolidation *only* if the other conditions are not satisfied. Consolidation performs scheduling simulations on the cluster and evaluates if there are any opportunities to save money by removing nodes or replacing nodes on the cluster.

The current consolidateAfter behavior creates a consolidation decision, waits for 15s synchronously inside of the disruption loop, and then re-validates that the same decision is still valid. This was intended to address the concern from [users that consolidation was acting too aggressively](https://github.com/aws/karpenter-provider-aws/issues/2370), and consolidating nodes that, if kept around for a little longer, would be valid for pods to schedule to. Adding the synchronous wait had the desired effect, but it caused us to have to keep this value low since it blocks *all* forms of disruption while it is waiting.

Users have asked that we make this a configurable field so that they can tweak whether we keep nodes around for longer when they are underutilized before we make a consolidation decision to terminate them. To do this, we will have to change our synchronous waiting mechanism to some other mechanism that will allow us to perform longer waits.

#### Tasks

- [ ] Design and implement a `spec.consolidateAfter` field for the v1 NodePool API, reworking our synchronous wait to ensure that waiting for nodes that haven’t reached the end of their `consolidateAfter` timeframe doesn’t block other disruption evaluation

### Change Expiration from Graceful to Forceful

**Issue/RFC Ref:** https://github.com/kubernetes-sigs/karpenter/pull/1303

**Category:** Breaking

Users care about the removal of their nodes within a certain timeframe due to security requirements. Budgets, `karpenter.sh/do-not-disrupt` pods and blocking PDBs should not stop us from starting the drain of the node after it exceeds its expireAfter value. Once a node has begun evicting, it will remain on the cluster up to its `terminationGracePeriod` duration (assuming there are blocking PDBs or pods that are running with the `karpenter.sh/do-not-disrupt` annotation) and then be removed.

#### Tasks

- [ ] Remove expiration from the graceful disruption controller
- [ ] Perform a direct NodeClaim deletion when a Node has exceeded its expireAfter lifetime value

### Define SemVer Versioning Policy for `kubernetes-sigs/karpenter` Library

**Category:** Stability

Karpenter currently deeply couples the versioning of the `kubernetes-sigs/karpenter` library releases with the versioning of the `aws/karpenter-provider-aws` image and chart releases. This means that when we release a v0.33.2 version of the Karpenter AWS image and chart, we also tag the `kubernetes-sigs/karpenter` library with the same version. Realistically, this is not sustainable long-term since other contributors and cloudproviders will begin to take a heavier reliance on this project’s libraries.

We need to define a mechanism for communicating breaking changes to the library to projects that rely on it. This library is similar to something like `controller-runtime` , where the library is versioned independent of the projects that rely on it. Starting in v1, we should adopt a versioning scheme similar to this, that decouples the AWS release version from the neutral library version.

#### Tasks

- [ ] Create a design doc for defining a versioning strategy for `kubernetes-sigs/karpenter` and cloud provider repos. Begin adhering to this strategy starting in v1

### Stabilize Observability (metrics, status, eventing)

**Issue Ref:** https://github.com/kubernetes-sigs/karpenter/issues/1051

**Category:** Breaking, Stability

Karpenter needs to stabalize a set of metrics, status conditions, and events that can be relied-upon for monitoring. The design for these metrics, status conditions and events will be added in a separate RFC.

### NodeClaim Conceptual Documentation

**Issue Ref:** https://github.com/aws/karpenter-provider-aws/issues/5144

**Category:** Stability

Karpenter currently has no conceptual documentation around NodeClaims. NodeClaims have become a fundamental part of how Karpenter launches and manages nodes. There is critical observability information that is stored inside of the NodeClaim that can help users understand when certain disruption conditions are met (Expired, Empty, Drifted) or why the NodeClaim fails to launch.

For Karpenter’s feature completeness at v1, we need to accurately describe to users what the purpose of Karpenter’s NodeClaims are and how to leverage the information that is stored within the NodeClaim to troubleshoot Karpenter’s decision-making.

#### Tasks

- [ ] Add a NodeClaim doc to the “Concepts” section of the documentation

### Drop Knative References from the Code

**Issue Ref:** https://github.com/kubernetes-sigs/karpenter/issues/332

**Category:** Stability

Karpenter has [used knative](https://github.com/knative/pkg) from the beginning of the project. knative’s pkg libraries were only intended for their own use and were not intended to be used widely by the community. Because knative relies on and generates so much of the upstream API, attempting to bump to a newer version of client-go (or any other upstream k8s package) without having knative pkg pinned to that same version causes incompatibilities. Practically, this means that we can be bottlenecked on older versions of k8s libraries while we are waiting on knative to update its own dependencies. [Knative has a slower release cycle than Karpenter](https://github.com/knative/community/blob/main/mechanics/RELEASE-SCHEDULE.md#upcoming-releases) so we need to avoid these bottlenecks while we have the opportunity to make breaking changes to the API.

#### Tasks

- [ ] Remove the knative logger and replace with the controller-runtime logger
- [ ] Update the status condition schema for Karpenter CustomResources to use the [metav1 status condition schema](https://github.com/kubernetes/apimachinery/blob/f14778da5523847e4c07346e3161a4b4f6c9186e/pkg/apis/meta/v1/types.go#L1523)

### Drop Knative Webhook from Karpenter

**Issue Ref:** https://github.com/kubernetes-sigs/karpenter/issues/332

**Category:** Stability

As part of Karpenter completely removing its dependency on Knative, Karpenter needs to remove its coupling on the knative webhook certificate reconciliation logic. Currently, Karpenter leverages knative’s webhook reconciler to reconcile a certificate needed to enable the TLS webhook traffic. As part of removing this dependency that `kubernetes-sigs/karpenter` has on the knative webhook reconciliation logic, Karpenter will be dropping webhooks entirely.

 Because K8s versions 1.25+ have support for CustomResourceValidations driven through Common Expression Language, Karpenter is dropping support for the webhook container entirely.

[`Azure/karpenter-provider-azure`](https://github.com/Azure/karpenter) dropped support for Kubernetes 1.25 since AKS [dropped support for this version in 2023.](https://learn.microsoft.com/en-us/azure/aks/supported-kubernetes-versions?tabs=azure-cli#aks-kubernetes-release-calendar) [`aws/karpenter-provider-aws`](https://github.com/aws/karpenter-provider-aws) still supports versions [all the way back to Kubernetes 1.23 due to extended support](https://docs.aws.amazon.com/eks/latest/userguide/kubernetes-versions.html). The AWS provider will continue to support versions prior to 1.25 until January 31, 2025. At this point, if not other providers support versions prior to 1.25, we will drop support for webhooks in [`kubernetes-sigs/karpenter`](https://github.com/kubernetes-sigs/karpenter).

#### Tasks

- [ ] Drop the webhook, removing the `knative/pkg` dependency from the Karpenter controller package

### Karpenter Global Logging Configuration Changes

**Issue Ref(s):** https://github.com/aws/karpenter-provider-aws/issues/5352

**Category:** Planned Deprecations, Breaking

Dropping our global logging configuration was a planned deprecation at v1beta1 and we will continue by fully dropping support for the ConfigMap-based configuration for our logging at v1.

#### Tasks

- [ ] Remove logging configuration (only allow LOG_LEVEL, potentially LOG_ENCODING if users request it)

### Promoting Drift Feature to Stable

**Category:** Stability

Karpenter supported drift in its alpha state from v0.21-v0.32. During alpha, we worked to build out features and promoted drift to beta after releasing full drift support for all Karpenter configuration. Since releasing drift in beta, we’ve received no feedback that would lead us to believe the feature is unstable or not the right direction for the declarative state of Karpenter.

Since the feature is such a fundamental part to how the declarative state of Karpenter functions, we will promote drift to stable at Karpenter v1.

#### Tasks

- [ ] Remove `Drift` as a feature flag, always enabling it in Karpenter

