---
title: "Upgrade Guide"
linkTitle: "Upgrade Guide"
weight: 10
description: >
  Learn about upgrading Karpenter
---

Karpenter is a controller that runs in your cluster, but it is not tied to a specific Kubernetes version, as the Cluster Autoscaler is.
Use your existing upgrade mechanisms to upgrade your core add-ons in Kubernetes and keep Karpenter up to date on bug fixes and new features.
This guide contains information needed to upgrade to the latest release of Karpenter, along with compatibility issues you need to be aware of when upgrading from earlier Karpenter versions.

{{% alert title="Warning" color="warning" %}}
With the release of Karpenter v1.0.0, the Karpenter team has dropped support for karpenter versions v0.36 and below. We recommend upgrading to the latest version of Karpenter and keeping Karpenter up-to-date for bug fixes and new features.
{{% /alert %}}

When upgrading Karpenter in production environments, implementing a robust CI/CD pipeline approach is crucial. Improper upgrades can lead to significant disruptions including failed node provisioning, orphaned nodes, interrupted workloads, and potential cost implications from unmanaged scaling. Given Karpenter's critical role in cluster scaling and workload management, untested upgrades could result in production outages or resource allocation issues that directly impact application availability and performance. Therefore, we recommend following these structured steps:

#### Pre-upgrade Validation

- Validate all required IAM permissions (node role, controller role)
- Check webhook configurations
- Back up existing NodePool and NodeClass configurations
- Document current version and settings

#### Staging Environment Setup

- Create or verify staging environment
- Update version tags in Helm values or manifests
- Configure automated validation tests

#### Staging Deployment

- Deploy to staging environment
- Run comprehensive tests including node provisioning
- Verify controller health
- Test NodePool and NodeClass functionality
- Monitor system behavior

#### Production Approval and Deployment

- Require manual approval/review
- Schedule maintenance window if needed
- Execute production deployment
- Monitor deployment progress
- Verify all components are functioning

#### Post-Deployment

- Monitor system health
- Verify node provisioning
- Keep rollback configurations accessible
- Update documentation

Here are few recommended CI/CD Pipeline Options:

- GitHub Actions - Excellent for GitHub-hosted repositories with built-in Kubernetes support
- GitLab CI - Strong container-native pipeline with integrated Kubernetes functionality
- ArgoCD - Specialized for GitOps workflows with Kubernetes
- AWS CodePipeline - Native integration with EKS and AWS services
- Flux - Open-source GitOps tool for Kubernetes with automatic deployment capabilities

Each pipeline tool can be configured to handle the Karpenter upgrade workflow, but choose based on your existing infrastructure, team expertise, and specific requirements for automation and integration.


### CRD Upgrades

Karpenter ships with a few Custom Resource Definitions (CRDs). These CRDs are published:
* As an independent Helm chart [karpenter-crd](https://gallery.ecr.aws/karpenter/karpenter-crd) ([source](https://github.com/aws/karpenter/blob/main/charts/karpenter-crd)) that can be used by Helm to manage the lifecycle of these CRDs. To upgrade or install `karpenter-crd` run:
  ```bash
  KARPENTER_NAMESPACE=kube-system
  helm upgrade --install karpenter-crd oci://public.ecr.aws/karpenter/karpenter-crd --version x.y.z --namespace "${KARPENTER_NAMESPACE}" --create-namespace
  ```
* As part of the helm chart [karpenter](https://gallery.ecr.aws/karpenter/karpenter) ([source](https://github.com/aws/karpenter/blob/main/charts/karpenter/crds)).
  Helm [does not manage the lifecycle of CRDs using this method](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/) - the tool will only install the CRD during the first installation of the Helm chart.
  Subsequent chart upgrades will not add or remove CRDs, even if the CRDs have changed.

CRDs are coupled to the version of Karpenter, and should be updated along with Karpenter.
For this reason, we recommend using the independent `karpenter-crd` chart to manage CRDs.

{{% alert title="Note" color="warning" %}}
If you get the error `invalid ownership metadata; label validation error:` while installing the `karpenter-crd` chart from an older version of Karpenter, follow the [Troubleshooting Guide]({{<ref "../troubleshooting/#helm-error-when-installing-the-karpenter-crd-chart" >}}) for details on how to resolve these errors.
{{% /alert %}}

<!--
WHEN CREATING A NEW SECTION OF THE UPGRADE GUIDANCE FOR NEWER VERSIONS, ENSURE THAT YOU COPY THE BETA API ALERT SECTION FROM THE LAST RELEASE TO PROPERLY WARN USERS OF THE RISK OF UPGRADING WITHOUT GOING TO 0.32.x FIRST
-->

### Upgrading to `1.6.0`+

{{% alert title="Warning" color="warning" %}}
Karpenter `1.1.0` drops the support for `v1beta1` APIs.
**Do not** upgrade to `1.1.0`+ without following the [Migration Guide]({{<ref "../../v1.0/upgrading/v1-migration.md#before-upgrading-to-v110">}}).
{{% /alert %}}

* Native ODCR support has graduated to beta and is enabled by default.
  If you were previously using open ODCRs with Karpenter and have not already migrated to native ODCR support, review the [native ODCR support guide]({{< relref "../tasks/odcrs" >}}) before upgrading.
* Support a new configuration option `MinValuesPolicy` which controls how the Karpenter scheduler treats min values. Options include 'Strict' (fails scheduling when min values can't be met) and 'BestEffort' (relaxes min values when they can't be met). Default is 'Strict' to preserve existing behavior.
* Support a new configuration option `DisableDryRun` which disables the dry run calls made during EC2NodeClass validation.


Full Changelog:
* https://github.com/aws/karpenter-provider-aws/releases/tag/v1.6.2
* https://github.com/kubernetes-sigs/karpenter/releases/tag/v1.6.1

### Upgrading to `1.5.0`+

{{% alert title="Warning" color="warning" %}}
Karpenter `1.1.0` drops the support for `v1beta1` APIs.
**Do not** upgrade to `1.1.0`+ without following the [Migration Guide]({{<ref "../../v1.0/upgrading/v1-migration.md#before-upgrading-to-v110">}}).
{{% /alert %}}

* No breaking changes 🎉

Full Changelog:
* https://github.com/aws/karpenter-provider-aws/releases/tag/v1.5.0
* https://github.com/kubernetes-sigs/karpenter/releases/tag/v1.5.0

### Upgrading to `1.4.0`+

{{% alert title="Warning" color="warning" %}}
Karpenter `1.1.0` drops the support for `v1beta1` APIs.
**Do not** upgrade to `1.1.0`+ without following the [Migration Guide]({{<ref "../../v1.0/upgrading/v1-migration.md#before-upgrading-to-v110">}}).
{{% /alert %}}

* No breaking changes 🎉

Full Changelog:
* https://github.com/aws/karpenter-provider-aws/releases/tag/v1.4.0
* https://github.com/kubernetes-sigs/karpenter/releases/tag/v1.4.0

### Upgrading to `1.3.0`+

{{% alert title="Warning" color="warning" %}}
Karpenter `1.1.0` drops the support for `v1beta1` APIs.
**Do not** upgrade to `1.1.0`+ without following the [Migration Guide]({{<ref "../../v1.0/upgrading/v1-migration.md#before-upgrading-to-v110">}}).
{{% /alert %}}

* `karpenter_ignored_pod_count` alpha metric had its name changed to `karpenter_scheduler_ignored_pod_count`
* With the `ReservedCapacity` feature flag, Karpenter introduces a new `karpenter.sh/capacity-type` value (`reserved`). This means any applications that explicitly select on `on-demand` with a `nodeSelector` and want to utilize ODCR capacity may need to update their requirements to use `nodeAffinity` to opt-in to using both `reserved` and `on-demand` capacity.

### Upgrading to `1.2.0`+

{{% alert title="Warning" color="warning" %}}
Karpenter `1.1.0` drops the support for `v1beta1` APIs.
**Do not** upgrade to `1.1.0`+ without following the [Migration Guide]({{<ref "../../v1.0/upgrading/v1-migration.md#before-upgrading-to-v110">}}).
{{% /alert %}}

* We have recently updated our labels on `karpenter_voluntary_disruption_queue_failures_total` and `karpenter_nodeclaims_disrupted_total` reason label from camille case to snake case. Therefore these reason labels values on those metrics have now been update as such:
  - Drifted -> drifted
  - Empty -> empty
  - Expired -> expired
  - Underutilized -> underutilized
* Nodeclass status and termination controllers have been merged into a single `nodeclass` controller. If you are relying on logs or metrics for `nodeclass.termination` or `nodeclass.status` controllers, please make sure that you update them to reference the new `nodeclass` controller.

### Upgrading to `1.1.0`+

{{% alert title="Warning" color="warning" %}}
Karpenter `1.1.0` drops the support for `v1beta1` APIs.
**Do not** upgrade to `1.1.0`+ without following the [Migration Guide]({{<ref "../../v1.0/upgrading/v1-migration.md#before-upgrading-to-v110">}}).
{{% /alert %}}

* Support for the `v1beta1` compatiblity annotations have been dropped. Ensure you have completed migration before upgrading to `v1.1.0`. Refer to the [migration guide]({{<ref "../../v1.0/upgrading/v1-migration.md#kubelet-configuration-migration">}}) for more details.
* `nodeClassRef.group` and `nodeClassRef.kind` are strictly required. Ensure these values are set for all `NodePools` / `NodeClaims` before upgrading.
* Bottlerocket AMIFamily now supports `instanceStorePolicy: RAID0`. This means that Karpenter will auto-generate userData to RAID0 your instance store volumes (similar to AL2 and AL2023) when specifying this value.
  * Note: This userData configuration is _only_ valid on Bottlerocket v1.22.0+. If you are using an earlier version of a Bottlerocket image (< v1.22.0) with `amiFamily: Bottlerocket` and `instanceStorePolicy: RAID0`, nodes will fail to join the cluster.
* The AWS Neuron accelerator well known name label (`karpenter.k8s.aws/instance-accelerator-name`) values now reflect their correct names of `trainium`, `inferentia`, and `inferentia2`. Previously, all Neuron accelerators were assigned the label name of `inferentia`.
* Karpenter drops the internal `karpenter.k8s.aws/cluster` tag used for launch template management in favor of `eks:eks-cluster-name` and consistency with other Karpenter-provisioned resources
* Generic operator metrics have been have been deprecated and replaced by resource-specific metrics.

### Upgrading to `1.0.0`+

{{% alert title="Warning" color="warning" %}}
Karpenter `1.0.0` introduces the `v1` APIs and uses [conversion webhooks](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#webhook-conversion) to support existing `v1beta1` APIs.
**Do not** upgrade to `1.0.0`+ without following the [`v1` Migration Guide]({{<ref "../../v1.0/upgrading/v1-migration.md">}}).
{{% /alert %}}

Refer to the `v1` Migration Guide for the [full changelog]({{<ref "../../v1.0/upgrading/v1-migration.md#changelog">}}).

### Upgrading to `0.37.0`+

{{% alert title="Warning" color="warning" %}}
`0.33.0`+ _only_ supports Karpenter v1beta1 APIs and will not work with existing Provisioner, AWSNodeTemplate or Machine alpha APIs. Do not upgrade to `0.37.0`+ without first [upgrading to `0.32.x`]({{<ref "#upgrading-to-0320" >}}). This version supports both the alpha and beta APIs, allowing you to migrate all of your existing APIs to beta APIs without experiencing downtime.
{{% /alert %}}

{{% alert title="Note" color="primary" %}}
Webhooks have been re-enabled by default starting in `0.37.3` to faciliate migration to `v1.0`.
If your cluster has network policies that block Ingress then ports `8000`, `8001`, `8081`, `8443` will need to be allowlisted.
You may still choose to disable webhooks through the helm chart.
{{% /alert %}}


* Karpenter now adds a readiness status condition to the EC2NodeClass. Make sure to upgrade your Custom Resource Definitions before proceeding with the upgrade. Failure to do so will result in Karpenter being unable to provision new nodes.
* Karpenter no longer updates the logger name when creating controller loggers. We now adhere to the controller-runtime standard, where the logger name will be set as `"logger": "controller"` always and the controller name will be stored in the structured value `"controller"`
* Karpenter updated the NodeClass controller naming in the following way: `nodeclass` -> `nodeclass.status`, `nodeclass.hash`, `nodeclass.termination`
* Karpenter's NodeClaim status conditions no longer include the `severity` field

### Upgrading to `0.36.0`+

{{% alert title="Warning" color="warning" %}}
`0.33.0`+ _only_ supports Karpenter v1beta1 APIs and will not work with existing Provisioner, AWSNodeTemplate or Machine alpha APIs. Do not upgrade to `0.36.0`+ without first [upgrading to `0.32.x`]({{<ref "#upgrading-to-0320" >}}). This version supports both the alpha and beta APIs, allowing you to migrate all of your existing APIs to beta APIs without experiencing downtime.
{{% /alert %}}

{{% alert title="Warning" color="warning" %}}
v0.36.x introduces update to drift that restricts rollback. When rolling back from >=v0.36.0, note that v0.32.9+, v0.33.4+, v0.34.5+, v0.35.4+ are the patch versions that support rollback. If Karpenter is rolled back to an older patch version, Karpenter can potentially drift all the nodes in the cluster.
{{% /alert %}}

{{% alert title="Note" color="primary" %}}
Webhooks have been re-enabled by default starting in `0.36.5` to faciliate migration to `v1.0`.
If your cluster has network policies that block Ingress then ports `8000`, `8001`, `8081`, `8443` will need to be allowlisted.
You may still choose to disable webhooks through the helm chart.
{{% /alert %}}

* Karpenter changed the name of the `karpenter_cloudprovider_instance_type_price_estimate` metric to `karpenter_cloudprovider_instance_type_offering_price_estimate` to align with the new `karpenter_cloudprovider_instance_type_offering_available` metric. The `region` label was also dropped from the metric, since this can be inferred from the environment that Karpenter is running in.

### Upgrading to `0.35.0`+

{{% alert title="Warning" color="warning" %}}
`0.33.0`+ _only_ supports Karpenter v1beta1 APIs and will not work with existing Provisioner, AWSNodeTemplate or Machine alpha APIs. Do not upgrade to `0.35.0`+ without first [upgrading to `0.32.x`]({{<ref "#upgrading-to-0320" >}}). This version supports both the alpha and beta APIs, allowing you to migrate all of your existing APIs to beta APIs without experiencing downtime.
{{% /alert %}}

{{% alert title="Note" color="primary" %}}
Webhooks have been re-enabled by default starting in `0.35.8` to faciliate migration to `v1.0`.
If your cluster has network policies that block Ingress then ports `8000`, `8001`, `8081`, `8443` will need to be allowlisted.
You may still choose to disable webhooks through the helm chart.
{{% /alert %}}

* Karpenter OCI tags and Helm chart version are now valid semantic versions, meaning that the `v` prefix from the git tag has been removed and they now follow the `x.y.z` pattern.

### Upgrading to `0.34.0`+

{{% alert title="Warning" color="warning" %}}
`0.33.0`+ _only_ supports Karpenter v1beta1 APIs and will not work with existing Provisioner, AWSNodeTemplate or Machine alpha APIs. Do not upgrade to `0.34.0`+ without first [upgrading to `0.32.x`]({{<ref "#upgrading-to-0320" >}}). This version supports both the alpha and beta APIs, allowing you to migrate all of your existing APIs to beta APIs without experiencing downtime.
{{% /alert %}}

{{% alert title="Warning" color="warning" %}}
The Ubuntu EKS optimized AMI has moved from 20.04 to 22.04 for Kubernetes 1.29+. This new AMI version is __not currently__ supported for users relying on AMI auto-discovery with the Ubuntu AMI family. More details can be found in this [GitHub issue](https://github.com/aws/karpenter-provider-aws/issues/5572). Please review this issue before upgrading to Kubernetes 1.29 if you are using the Ubuntu AMI family. Upgrading to 1.29 without making any changes to your EC2NodeClass will result in Karpenter being unable to create new nodes.
{{% /alert %}}

{{% alert title="Note" color="primary" %}}
Webhooks have been re-enabled by default starting in `0.34.9` to faciliate migration to `v1.0`.
If your cluster has network policies that block Ingress then ports `8000`, `8001`, `8081`, `8443` will need to be allowlisted.
You may still choose to disable webhooks through the helm chart.
{{% /alert %}}

* Karpenter now supports `nodepool.spec.disruption.budgets`, which allows users to control the speed of disruption in the cluster. Since this requires an update to the Custom Resource, before upgrading, you should re-apply the new updates to the CRDs. Check out [Disruption Budgets]({{<ref "../concepts/disruption#disruption-budgets" >}}) for more.
* With Disruption Budgets, Karpenter will disrupt multiple batches of nodes simultaneously, which can result in overall quicker scale-down of your cluster. Before `0.34.0`, Karpenter had a hard-coded parallelism limit for each type of disruption. In `0.34.0`+, Karpenter will now disrupt at most 10% of nodes for a given NodePool. There is no setting that will be perfectly equivalent with the behavior prior to `0.34.0`. When considering how to configure your budgets, please refer to the following limits for versions prior to `0.34.0`:
  * `Empty Expiration / Empty Drift / Empty Consolidation`: infinite parallelism
  * `Non-Empty Expiration / Non-Empty Drift / Single-Node Consolidation`: one node at a time
  * `Multi-Node Consolidation`: max 100 nodes
* To support Disruption Budgets, `0.34.0`+ includes critical changes to Karpenter's core controllers, which allows Karpenter to consider multiple batches of disrupting nodes simultaneously. This increases Karpenter's performance with the potential downside of higher CPU and memory utilization from the Karpenter pod. While the magnitude of this difference varies on a case-by-case basis, when upgrading to Karpenter `0.34.0`+, please note that you may need to increase the resources allocated to the Karpenter controller pods.
* Karpenter now adds a default `podSecurityContext` that configures the `fsgroup: 65536` of volumes in the pod. If you are using sidecar containers, you should review if this configuration is compatible for them. You can disable this default `podSecurityContext` through helm by performing `--set podSecurityContext=null` when installing/upgrading the chart.
* The `dnsPolicy` for the Karpenter controller pod has been changed back to the Kubernetes cluster default of `ClusterFirst`. Setting our `dnsPolicy` to `Default` (confusingly, this is not the Kubernetes cluster default) caused more confusion for any users running IPv6 clusters with dual-stack nodes or anyone running Karpenter with dependencies on cluster services (like clusters running service meshes). This change may be breaking for any users on Fargate or MNG who were allowing Karpenter to manage their in-cluster DNS service (`core-dns` on most clusters). If you still want the old behavior here, you can change the `dnsPolicy` to point to use `Default` by setting the helm value on install/upgrade with `--set dnsPolicy=Default`. More details on this issue can be found in the following Github issues: [#2186](https://github.com/aws/karpenter-provider-aws/issues/2186) and [#4947](https://github.com/aws/karpenter-provider-aws/issues/4947).
* Karpenter now disallows `nodepool.spec.template.spec.resources` to be set. The webhook validation never allowed `nodepool.spec.template.spec.resources`. We are now ensuring that CEL validation also disallows `nodepool.spec.template.spec.resources` to be set. If you were previously setting the resources field on your NodePool, ensure that you remove this field before upgrading to the newest version of Karpenter or else updates to the resource may fail on the new version.

### Upgrading to `0.33.0`+

{{% alert title="Warning" color="warning" %}}
`0.33.0`+ _only_ supports Karpenter v1beta1 APIs and will not work with existing Provisioner, AWSNodeTemplate or Machine alpha APIs. **Do not** upgrade to `0.33.0`+ without first [upgrading to `0.32.x`]({{<ref "#upgrading-to-0320" >}}). This version supports both the alpha and beta APIs, allowing you to migrate all of your existing APIs to beta APIs without experiencing downtime.
{{% /alert %}}

{{% alert title="Note" color="primary" %}}
Webhooks have been re-enabled by default starting in `0.33.8` to faciliate migration to `v1.0`.
If your cluster has network policies that block Ingress then ports `8000`, `8001`, `8081`, `8443` will need to be allowlisted.
You may still choose to disable webhooks through the helm chart.
{{% /alert %}}

* Karpenter no longer supports using the `karpenter.sh/provisioner-name` label in NodePool labels and requirements or in application node selectors, affinities, or topologySpreadConstraints. If you were previously using this label to target applications to specific Provisioners, you should update your applications to use the `karpenter.sh/nodepool` label instead before upgrading. If you upgrade without changing these labels, you may begin to see pod scheduling failures for these applications.
* Karpenter now tags `spot-instances-request` with the same tags that it tags instances, volumes, and primary ENIs. This means that you will now need to add `ec2:CreateTags` permission for `spot-instances-request`. You can also further scope your controller policy for the `ec2:RunInstances` action to require that it launches the `spot-instances-request` with these specific tags. You can view an example of scoping these actions in the [Getting Started Guide's default CloudFormation controller policy](https://github.com/aws/karpenter/blob/v0.33.0/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml#L61).
* We now recommend that you set the installation namespace for your Karpenter controllers to `kube-system` to denote Karpenter as a critical cluster component. This ensures that requests from the Karpenter controllers are treated with higher priority by assigning them to a different [PriorityLevelConfiguration](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/#prioritylevelconfiguration) than generic requests from other namespaces. For more details on API Priority and Fairness, read the [Kubernetes API Priority and Fairness Conceptual Docs](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/). Note: Changing the namespace for your Karpenter release will cause the service account namespace to change. If you are using IRSA for authentication with AWS, you will need to change scoping set in the controller's trust policy from `karpenter:karpenter` to `kube-system:karpenter`.
* ~~`0.33.0` disables mutating and validating webhooks by default in favor of using [Common Expression Language for CRD validation](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation). The Common Expression Language Validation Feature [is enabled by default on EKS 1.25](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules). If you are using Kubernetes version >= 1.25, no further action is required. If you are using a Kubernetes version below 1.25, you now need to set `DISABLE_WEBHOOK=false` in your container environment variables or `--set webhook.enabled=true` if using Helm. View the [Webhook Support Deprecated in Favor of CEL Section of the v1beta1 Migration Guide]({{<ref "../../v1.0/upgrading/v1beta1-migration#webhook-support-deprecated-in-favor-of-cel" >}}).~~
* `0.33.0` drops support for passing settings through the `karpenter-global-settings` ConfigMap. You should pass settings through the container environment variables in the Karpenter deployment manifest. View the [Global Settings Section of the v1beta1 Migration Guide]({{<ref "../../v1.0/upgrading/v1beta1-migration#global-settings" >}}) for more details.
* `0.33.0` enables `Drift=true` by default in the `FEATURE_GATES`. If you previously didn't enable the feature gate, Karpenter will now check if there is a difference between the desired state of your nodes declared in your NodePool and the actual state of your nodes. View the [Drift Section of Disruption Conceptual Docs]({{<ref "../concepts/disruption#drift" >}}) for more details.
* `0.33.0` drops looking up the `zap-logger-config` through ConfigMap discovery. Instead, Karpenter now expects the logging config to be mounted on the filesystem if you are using this to configure Zap logging. This is not enabled by default, but can be enabled through `--set logConfig.enabled=true` in the Helm values. If you are setting any values in the `logConfig` from the `0.32.x` upgrade, such as `logConfig.logEncoding`, note that you will have to explicitly set `logConfig.enabled=true` alongside it. Also, note that setting the Zap logging config is a deprecated feature in beta and is planned to be dropped at v1. View the [Logging Configuration Section of the v1beta1 Migration Guide]({{<ref "../../v1.0/upgrading/v1beta1-migration#logging-configuration-is-no-longer-dynamic" >}}) for more details.
* `0.33.0` change the default `LOG_LEVEL` from `debug` to `info` by default. If you are still enabling logging configuration through the `zap-logger-config`, no action is required.
* `0.33.0` drops support for comma delimited lists on tags for `SubnetSelectorTerm`, `SecurityGroupsSelectorTerm`, and `AMISelectorTerm`. Karpenter now supports multiple terms for each of the selectors which means that we can specify a more explicit OR-based constraint through separate terms rather than a comma-delimited list of values.

### Upgrading to `0.32.0`+

{{% alert title="Warning" color="warning" %}}
Karpenter `0.32.0` introduces v1beta1 APIs, including _significant_ changes to the API and installation procedures for the Karpenter controllers. **Do not** upgrade to `0.32.0`+ without referencing the [v1beta1 Migration Upgrade Procedure]({{<ref "../../v1.0/upgrading/v1beta1-migration#upgrade-procedure" >}}).

This version includes **dual support** for both alpha and beta APIs to ensure that you can slowly migrate your existing Provisioner, AWSNodeTemplate, and Machine alpha APIs to the newer NodePool, EC2NodeClass, and NodeClaim beta APIs.

Note that if you are rolling back after upgrading to `0.32.0`, note that __only__ versions `0.31.4` support handling rollback after you have deployed the v1beta1 APIs to your cluster.
{{% /alert %}}

* Karpenter now uses `settings.InterruptionQueue` instead of `settings.aws.InterruptionQueueName` in its helm chart. The CLI argument also changed to `--interruption-queue`.
* Karpenter now serves the webhook prometheus metrics server on port `8001`. If this port is already in-use on the pod or you are running in `hostNetworking` mode, you may need to change this port value. You can configure this port value through the `WEBHOOK_METRICS_PORT` environment variable or the `webhook.metrics.port` value if installing via Helm.
* Karpenter now exposes the ability to disable webhooks through the `webhook.enabled=false` value. This value will disable the webhook server and will prevent any permissions, mutating or validating webhook configurations from being deployed to the cluster.
* Karpenter now moves all logging configuration for the Zap logger into the `logConfig` values block. Configuring Karpenter logging with this mechanism _is_ deprecated and will be dropped at v1. Karpenter now only surfaces logLevel through the `logLevel` helm value. If you need more advanced configuration due to log parsing constraints, we recommend configuring your log parser to handle Karpenter's Zap JSON logging.
* The default log encoding changed from `console` to `json`. If you were previously not setting the type of log encoding, this default will change with the Helm chart. If you were setting the value through `logEncoding`, this value will continue to work until `0.33.x` but it is deprecated in favor of `logConfig.logEncoding`
* Karpenter now uses the `karpenter.sh/disruption:NoSchedule=disrupting` taint instead of the upstream `node.kubernetes.io/unschedulable` taint for nodes spawned with a NodePool to prevent pods from scheduling to nodes being disrupted. Pods that previously tolerated the `node.kubernetes.io/unschedulable` taint that previously weren't evicted during termination will now be evicted. This most notably affects DaemonSets, which have the `node.kubernetes.io/unschedulable` toleration by default, where Karpenter will now remove these pods during termination. If you want your specific pods to not be evicted when nodes are scaled down, you should add a toleration to the pods with the following: `Key=karpenter.sh/disruption, Effect=NoSchedule, Operator=Equals, Values=disrupting`.
  * Note: Karpenter will continue to use the old `node.kubernetes.io/unschedulable` taint for nodes spawned with a Provisioner.

### Upgrading to `0.31.0`+

* Karpenter moved its `securityContext` constraints from pod-wide to only applying to the Karpenter container exclusively. If you were previously relying on the pod-wide `securityContext` for your sidecar containers, you will now need to set these values explicitly in your sidecar container configuration.

### Upgrading to `0.30.0`+

* Karpenter will now [statically drift]({{<ref "../concepts/disruption#drift" >}}) on both Provisioner and AWSNodeTemplate Fields. For Provisioner Static Drift, the `karpenter.sh/provisioner-hash` annotation must be present on both the Provisioner and Machine. For AWSNodeTemplate drift, the `karpenter.k8s.aws/nodetemplate-hash` annotation must be present on the AWSNodeTemplate and Machine. Karpenter will not add these annotations to pre-existing nodes, so each of these nodes will need to be recycled one time for the annotations to be added.
* Karpenter will now fail validation on AWSNodeTemplates and Provisioner `spec.provider` that have `amiSelectors`, `subnetSelectors`, or `securityGroupSelectors` set with a combination of id selectors (`aws-ids`, `aws::ids`) and other selectors.
* Karpenter now statically sets the `securityContext` at both the pod and container-levels and doesn't allow override values to be passed through the Helm chart. This change was made to adhere to [Restricted Pod Security Standard](https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted), which follows pod hardening best practices.

{{% alert title="Note" color="primary" %}}
If you have sidecar containers configured to run alongside Karpenter that cannot tolerate the [pod-wide `securityContext` constraints](https://github.com/aws/karpenter/blob/v0.30.0/charts/karpenter/templates/deployment.yaml#L40), you will need to specify overrides to the sidecar `securityContext` in your deployment.
{{% /alert %}}

### Upgrading to `0.29.0`+

{{% alert title="Warning" color="warning" %}}
Karpenter `0.29.1` contains a [file descriptor and memory leak bug](https://github.com/aws/karpenter/issues/4296) that leads to Karpenter getting OOMKilled and restarting at the point that it hits its memory or file descriptor limit. Karpenter `0.29.2`+ fixes this leak.
{{% /alert %}}

* Karpenter has changed the default metrics service port from 8080 to 8000 and the default webhook service port from 443 to 8443. In `0.28.0`, the Karpenter pod port was changed to 8000, but referenced the service by name, allowing users to scrape the service at port 8080 for metrics. `0.29.0` aligns the two ports so that service and pod metrics ports are the same. These ports are set by the `controller.metrics.port` and `webhook.port` Helm chart values, so if you have previously set these to non-default values, you may need to update your Prometheus scraper to match these new values.

* Karpenter will now reconcile nodes that are drifted due to their Security Groups or their Subnets. If your AWSNodeTemplate's Security Groups differ from the Security Groups used for an instance, Karpenter will consider it drifted. If the Subnet used by an instance is not contained in the allowed list of Subnets for an AWSNodeTemplate, Karpenter will also consider it drifted.
  * Since Karpenter uses tags for discovery of Subnets and SecurityGroups, check the [Threat Model]({{<ref "../reference/threat-model.md#threat-using-ec2-createtagdeletetag-permissions-to-orchestrate-machine-creationdeletion" >}}) to see how to manage this IAM Permission.

### Upgrading to `0.28.0`+

{{% alert title="Warning" color="warning" %}}
Karpenter `0.28.0` is incompatible with Kubernetes version 1.26+, which can result in additional node scale outs when using `--cloudprovider=external`, which is the default for the EKS Optimized AMI. See: https://github.com/aws/karpenter-core/pull/375. Karpenter `0.28.1`+ fixes this issue and is compatible with Kubernetes version 1.26+.
{{% /alert %}}

* The `extraObjects` value is now removed from the Helm chart. Having this value in the chart proved to not work in the majority of Karpenter installs and often led to anti-patterns, where the Karpenter resources installed to manage Karpenter's capacity were directly tied to the install of the Karpenter controller deployments. The Karpenter team recommends that, if you want to install Karpenter manifests alongside the Karpenter Helm chart, to do so by creating a separate chart for the manifests, creating a dependency on the controller chart.
* The `aws.nodeNameConvention` setting is now removed from the [`karpenter-global-settings`]({{<ref "../reference/settings#configmap" >}}) ConfigMap. Because Karpenter is now driving its orchestration of capacity through Machines, it no longer needs to know the node name, making this setting obsolete. Karpenter ignores configuration that it doesn't recognize in the [`karpenter-global-settings`]({{<ref "../reference/settings#configmap" >}}) ConfigMap, so leaving the `aws.nodeNameConvention` in the ConfigMap will simply cause this setting to be ignored.
* Karpenter now defines a set of "restricted tags" which can't be overridden with custom tagging in the AWSNodeTemplate or in the [`karpenter-global-settings`]({{<ref "../reference/settings#configmap" >}}) ConfigMap. If you are currently using any of these tag overrides when tagging your instances, webhook validation will now fail. These tags include:

  * `karpenter.sh/managed-by`
  * `karpenter.sh/provisioner-name`
  * `kubernetes.io/cluster/${CLUSTER_NAME}`

* The following metrics changed their meaning, based on the introduction of the Machine resource:
  * `karpenter_nodes_terminated`: Use `karpenter_machines_terminated` if you are interested in the reason why a Karpenter machine was deleted. `karpenter_nodes_terminated` now only tracks the count of terminated nodes without any additional labels.
  * `karpenter_nodes_created`: Use `karpenter_machines_created` if you are interested in the reason why a Karpenter machine was created. `karpenter_nodes_created` now only tracks the count of created nodes without any additional labels.
  * `karpenter_deprovisioning_replacement_node_initialized_seconds`: This metric has been replaced in favor of `karpenter_deprovisioning_replacement_machine_initialized_seconds`.
* `0.28.0` introduces the Machine CustomResource into the `karpenter.sh` API Group and requires this CustomResourceDefinition to run properly. Karpenter now orchestrates its CloudProvider capacity through these in-cluster Machine CustomResources. When performing a scheduling decision, Karpenter will create a Machine, resulting in launching CloudProvider capacity. The kubelet running on the new capacity will then register the node to the cluster shortly after launch.
  * If you are using Helm to upgrade between versions of Karpenter, note that [Helm does not automate the process of upgrading or install the new CRDs into your cluster](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations). To install or upgrade the existing CRDs, follow the guidance under the [Custom Resource Definition (CRD) Upgrades]({{< relref "#custom-resource-definition-crd-upgrades" >}}) section of the upgrade guide.
  * Karpenter will hydrate Machines on startup for existing capacity managed by Karpenter into the cluster. Existing capacity launched by an older version of Karpenter is discovered by finding CloudProvider capacity with the `karpenter.sh/provisioner-name` tag or the `karpenter.sh/provisioner-name` label on nodes.
* The metrics port for the Karpenter deployment was changed from 8080 to 8000. Users who scrape the pod directly for metrics rather than the service will need to adjust the commands they use to reference port 8000. Any users who scrape metrics from the service should be unaffected.

{{% alert title="Warning" color="warning" %}}
Karpenter creates a mapping between CloudProvider machines and CustomResources in the cluster for capacity tracking. To ensure this mapping is consistent, Karpenter utilizes the following tag keys:

* `karpenter.sh/managed-by`
* `karpenter.sh/provisioner-name`
* `kubernetes.io/cluster/${CLUSTER_NAME}`

Because Karpenter takes this dependency, any user that has the ability to Create/Delete these tags on CloudProvider machines will have the ability to orchestrate Karpenter to Create/Delete CloudProvider machines as a side effect. Check the [Threat Model]({{<ref "../reference/threat-model.md#threat-using-ec2-createtagdeletetag-permissions-to-orchestrate-machine-creationdeletion" >}}) to see how this might affect you, and ways to mitigate this.
{{% /alert %}}

{{% alert title="Rolling Back" color="warning" %}}
If, after upgrading to `0.28.0`+, a rollback to an older version of Karpenter needs to be performed, Karpenter will continue to function normally, though you will still have the Machine CustomResources on your cluster. You will need to manually delete the Machines and patch out the finalizers to fully complete the rollback.

Karpenter marks CloudProvider capacity as "managed by" a Machine using the `karpenter-sh/managed-by` tag on the CloudProvider machine. It uses this tag to ensure that the Machine CustomResources in the cluster match the CloudProvider capacity managed by Karpenter. If these states don't match, Karpenter will garbage collect the capacity. Because of this, if performing an upgrade, followed by a rollback, followed by another upgrade to `0.28.0`+, ensure you remove the `karpenter.sh/managed-by` tags from existing capacity; otherwise, Karpenter will deprovision the capacity without a Machine CR counterpart.
{{% /alert %}}

### Upgrading to `0.27.3`+

* The `defaulting.webhook.karpenter.sh` mutating webhook was removed in `0.27.3`. If you are coming from an older version of Karpenter where this webhook existed and the webhook was not managed by Helm, you may need to delete the stale webhook.

```bash
kubectl delete mutatingwebhookconfigurations defaulting.webhook.karpenter.sh
```

### Upgrading to `0.27.0`+

* The Karpenter controller pods now deploy with `kubernetes.io/hostname` self anti-affinity by default. If you are running Karpenter in HA (high-availability) mode and you do not have enough nodes to match the number of pod replicas you are deploying with, you will need to scale-out your nodes for Karpenter.
* The following controller metrics changed and moved under the `controller_runtime` metrics namespace:
  * `karpenter_metricscraper_...`
  * `karpenter_deprovisioning_...`
  * `karpenter_provisioner_...`
  * `karpenter_interruption_...`
* The following controller metric names changed, affecting the `controller` label value under `controller_runtime_...` metrics. These metrics include:
  * `podmetrics` -> `pod_metrics`
  * `provisionermetrics` -> `provisioner_metrics`
  * `metricscraper` -> `metric_scraper`
  * `provisioning` -> `provisioner_trigger`
  * `node-state` -> `node_state`
  * `pod-state` -> `pod_state`
  * `provisioner-state` -> `provisioner_state`
* The `karpenter_allocation_controller_scheduling_duration_seconds` metric name changed to `karpenter_provisioner_scheduling_duration_seconds`

### Upgrading to `0.26.0`+

* The `karpenter.sh/do-not-evict` annotation no longer blocks node termination when running `kubectl delete node`. This annotation on pods will only block automatic deprovisioning that is considered "voluntary," that is, disruptions that can be avoided. Disruptions that Karpenter deems as "involuntary" and will ignore the `karpenter.sh/do-not-evict` annotation include spot interruption and manual deletion of the node. See [Disabling Deprovisioning]({{<ref "../concepts/disruption#disabling-deprovisioning" >}}) for more details.
* Default resources `requests` and `limits` are removed from the Karpenter's controller deployment through the Helm chart. If you have not set custom resource `requests` or `limits` in your Helm values and are using Karpenter's defaults, you will now need to set these values in your Helm chart deployment.
* The `controller.image` value in the Helm chart has been broken out to a map consisting of `controller.image.repository`, `controller.image.tag`, and `controller.image.digest`. If manually overriding the `controller.image`, you will need to update your values to the new design.

### Upgrading to `0.25.0`+

* Cluster Endpoint can now be automatically discovered. If you are using Amazon Elastic Kubernetes Service (EKS), you can now omit the `clusterEndpoint` field in your configuration. In order to allow the resolving, you have to add the permission `eks:DescribeCluster` to the Karpenter Controller IAM role.

### Upgrading to `0.24.0`+

* Settings are no longer updated dynamically while Karpenter is running. If you manually make a change to the [`karpenter-global-settings`]({{<ref "../reference/settings#configmap" >}}) ConfigMap, you will need to reload the containers by restarting the deployment with `kubectl rollout restart -n karpenter deploy/karpenter`
* Karpenter no longer filters out instance types internally. Previously, `g2` (not supported by the NVIDIA device plugin) and FPGA instance types were filtered. The only way to filter instance types now is to set requirements on your provisioner or pods using well-known node labels described [here]({{<ref "../concepts/scheduling#selecting-nodes" >}}). If you are currently using overly broad requirements that allows all of the `g` instance-category, you will want to tighten the requirement, or add an instance-generation requirement.
* `aws.tags` in [`karpenter-global-settings`]({{<ref "../reference/settings#configmap" >}}) ConfigMap is now a top-level field and expects the value associated with this key to be a JSON object of string to string. This is change from previous versions where keys were given implicitly by providing the key-value pair `aws.tags.<key>: value` in the ConfigMap.

### Upgrading to `0.22.0`+

* Do not upgrade to this version unless you are on Kubernetes >= v1.21. Karpenter no longer supports Kubernetes v1.20, but now supports Kubernetes v1.25. This change is due to the v1 PDB API, which was introduced in K8s v1.20 and subsequent removal of the v1beta1 API in K8s v1.25.

### Upgrading to `0.20.0`+

* Prior to `0.20.0`, Karpenter would prioritize certain instance type categories absent of any requirements in the Provisioner. `0.20.0`+ removes prioritizing these instance type categories ("m", "c", "r", "a", "t", "i") in code. Bare Metal and GPU instance types are still deprioritized and only used if no other instance types are compatible with the node requirements. Since Karpenter does not prioritize any instance types, if you do not want exotic instance types and are not using the runtime Provisioner defaults, you will need to specify this in the Provisioner.

### Upgrading to `0.19.0`+

* The karpenter webhook and controller containers are combined into a single binary, which requires changes to the Helm chart. If your Karpenter installation (Helm or otherwise) currently customizes the karpenter webhook, your deployment tooling may require minor changes.
* Karpenter now supports native interruption handling. If you were previously using Node Termination Handler for spot interruption handling and health events, you will need to remove the component from your cluster before enabling `aws.interruptionQueueName`. For more details on Karpenter's interruption handling, see the [Interruption Handling Docs]({{< ref "../concepts/disruption/#interruption" >}}).
* Instance category defaults are now explicitly persisted in the Provisioner, rather than handled implicitly in memory. By default, Provisioners will limit instance category to c,m,r. If any instance type constraints are applied, it will override this default. If you have created Provisioners in the past with unconstrained instance type, family, or category, Karpenter will now more flexibly use instance types than before. If you would like to apply these constraints, they must be included in the Provisioner CRD.
* Karpenter CRD raw YAML URLs have migrated from `https://raw.githubusercontent.com/aws/karpenter-provider-aws/v0.19.3/charts/karpenter/crds/...` to `https://raw.githubusercontent.com/aws/karpenter-provider-aws/v0.19.3/pkg/apis/crds/...`. If you reference static Karpenter CRDs or rely on `kubectl replace -f` to apply these CRDs from their remote location, you will need to migrate to the new location.
* Pods without an ownerRef (also called "controllerless" or "naked" pods) will now be evicted by default during node termination and consolidation.  Users can prevent controllerless pods from being voluntarily disrupted by applying the `karpenter.sh/do-not-evict: "true"` annotation to the pods in question.
* The following CLI options/environment variables are now removed and replaced in favor of pulling settings dynamically from the [`karpenter-global-settings`]({{<ref "../reference/settings#configmap" >}}) ConfigMap. See the [Settings docs]({{<ref "../reference/settings/#environment-variables--cli-flags" >}}) for more details on configuring the new values in the ConfigMap.

  * `CLUSTER_NAME` -> `settings.aws.clusterName`
  * `CLUSTER_ENDPOINT` -> `settings.aws.clusterEndpoint`
  * `AWS_DEFAULT_INSTANCE_PROFILE` -> `settings.aws.defaultInstanceProfile`
  * `AWS_ENABLE_POD_ENI` -> `settings.aws.enablePodENI`
  * `AWS_ENI_LIMITED_POD_DENSITY` -> `settings.aws.enableENILimitedPodDensity`
  * `AWS_ISOLATED_VPC` -> `settings.aws.isolatedVPC`
  * `AWS_NODE_NAME_CONVENTION` -> `settings.aws.nodeNameConvention`
  * `VM_MEMORY_OVERHEAD` -> `settings.aws.vmMemoryOverheadPercent`

### Upgrading to `0.18.0`+

* `0.18.0` removes the `karpenter_consolidation_nodes_created` and `karpenter_consolidation_nodes_terminated` prometheus metrics in favor of the more generic `karpenter_nodes_created` and `karpenter_nodes_terminated` metrics. You can still see nodes created and terminated by consolidation by checking the `reason` label on the metrics. Check out all the metrics published by Karpenter [here]({{<ref "../reference/metrics" >}}).

### Upgrading to `0.17.0`+

Karpenter's Helm chart package is now stored in [Karpenter's OCI (Open Container Initiative) registry](https://gallery.ecr.aws/karpenter/karpenter). The Helm CLI supports the new format since [v3.8.0+](https://helm.sh/docs/topics/registries/).
With this change [charts.karpenter.sh](https://charts.karpenter.sh/) is no longer updated but preserved to allow using older Karpenter versions. For examples on working with the Karpenter Helm charts look at [Install Karpenter Helm Chart]({{< ref "../getting-started/getting-started-with-karpenter/#install-karpenter-helm-chart" >}}).

Users who have scripted the installation or upgrading of Karpenter need to adjust their scripts with the following changes:
1. There is no longer a need to add the Karpenter Helm repo with `helm repo add`
2. The full URL of the Helm chart needs to be present when using the `helm` CLI
3. If you were not prepending a `v` to the version (i.e. `0.17.0`), you will need to do so with the OCI chart  (i.e `v0.17.0`).

### Upgrading to `0.16.2`+

* `0.16.2` adds new kubeletConfiguration fields to the `provisioners.karpenter.sh` v1alpha5 CRD.  The CRD will need to be updated to use the new parameters:
```bash
kubectl replace -f https://raw.githubusercontent.com/aws/karpenter-provider-aws/v0.16.2/charts/karpenter/crds/karpenter.sh_provisioners.yaml
```

### Upgrading to `0.16.0`+

* `0.16.0` adds a new weight field to the `provisioners.karpenter.sh` v1alpha5 CRD.  The CRD will need to be updated to use the new parameters:
```bash
kubectl replace -f https://raw.githubusercontent.com/aws/karpenter-provider-aws/v0.16.0/charts/karpenter/crds/karpenter.sh_provisioners.yaml
```

### Upgrading to `0.15.0`+

* `0.15.0` adds a new consolidation field to the `provisioners.karpenter.sh` v1alpha5 CRD.  The CRD will need to be updated to use the new parameters:
```bash
kubectl replace -f https://raw.githubusercontent.com/aws/karpenter-provider-aws/v0.15.0/charts/karpenter/crds/karpenter.sh_provisioners.yaml
```

### Upgrading to `0.14.0`+

* `0.14.0` adds new fields to the `provisioners.karpenter.sh` v1alpha5 and `awsnodetemplates.karpenter.k8s.aws` v1alpha1 CRDs. The CRDs will need to be updated to use the new parameters:

```bash
kubectl replace -f https://raw.githubusercontent.com/aws/karpenter-provider-aws/v0.14.0/charts/karpenter/crds/karpenter.sh_provisioners.yaml

kubectl replace -f https://raw.githubusercontent.com/aws/karpenter-provider-aws/v0.14.0/charts/karpenter/crds/karpenter.k8s.aws_awsnodetemplates.yaml
```

* `0.14.0` changes the way Karpenter discovers its dynamically generated AWS launch templates to use a tag rather than a Name scheme. The previous name scheme was `Karpenter-${CLUSTER_NAME}-*` which could collide with user created launch templates that Karpenter should not manage. The new scheme uses a tag on the launch template `karpenter.k8s.aws/cluster: ${CLUSTER_NAME}`. As a result, Karpenter will not clean-up dynamically generated launch templates using the old name scheme. You can manually clean these up with the following commands:

```bash
## Find launch templates that match the naming pattern and you do not want to keep
aws ec2 describe-launch-templates --filters="Name=launch-template-name,Values=Karpenter-${CLUSTER_NAME}-*"

## Delete launch template(s) that match the name but do not have the "karpenter.k8s.aws/cluster" tag
aws ec2 delete-launch-template --launch-template-id <LAUNCH_TEMPLATE_ID>
```

* `0.14.0` introduces additional instance type filtering if there are no `node.kubernetes.io/instance-type` or `karpenter.k8s.aws/instance-family` or `karpenter.k8s.aws/instance-category` requirements that restrict instance types specified on the provisioner. This prevents Karpenter from launching bare metal and some older non-current generation instance types unless the provisioner has been explicitly configured to allow them. If you specify an instance type or family requirement that supplies a list of instance-types or families, that list will be used regardless of filtering.  The filtering can also be completely eliminated by adding an `Exists` requirement for instance type or family.
```yaml
  - key: node.kubernetes.io/instance-type
    operator: Exists
```

* `0.14.0` introduces support for custom AMIs without the need for an entire launch template. You must add the `ec2:DescribeImages` permission to the Karpenter Controller Role for this feature to work. This permission is needed for Karpenter to discover custom images specified. Read the [Custom AMI documentation here]({{<ref "../concepts/nodepools#spec-amiselector" >}}) to get started
* `0.14.0` adds an an additional default toleration (CriticalAddonOnly=Exists) to the Karpenter Helm chart. This may cause Karpenter to run on nodes with that use this Taint which previously would not have been schedulable. This can be overridden by using `--set tolerations[0]=null`.

* `0.14.0` deprecates the `AWS_ENI_LIMITED_POD_DENSITY` environment variable in-favor of specifying `spec.kubeletConfiguration.maxPods` on the Provisioner. `AWS_ENI_LIMITED_POD_DENSITY` will continue to work when `maxPods` is not set on the Provisioner. If `maxPods` is set, it will override `AWS_ENI_LIMITED_POD_DENSITY` on that specific Provisioner.

### Upgrading to `0.13.0`+

* `0.13.0` introduces a new CRD named `AWSNodeTemplate` which can be used to specify AWS Cloud Provider parameters. Everything that was previously specified under `spec.provider` in the Provisioner resource, can now be specified in the spec of the new resource. The use of `spec.provider` is deprecated but will continue to function to maintain backwards compatibility for the current API version (v1alpha5) of the Provisioner resource. `0.13.0` also introduces support for custom user data that doesn't require the use of a custom launch template. The user data can be specified in-line in the AWSNodeTemplate resource.

  If you are upgrading from `0.10.1` - `0.11.1`, a new CRD `awsnodetemplate` was added. In `0.12.0`, this crd was renamed to `awsnodetemplates`. Since Helm does not manage the lifecycle of CRDs, you will need to perform a few manual steps for this CRD upgrade:
  1. Make sure any `awsnodetemplate` manifests are saved somewhere so that they can be reapplied to the cluster.
  2. `kubectl delete crd awsnodetemplate`
  3. `kubectl apply -f https://raw.githubusercontent.com/aws/karpenter-provider-aws/v0.13.2/charts/karpenter/crds/karpenter.k8s.aws_awsnodetemplates.yaml`
  4. Perform the Karpenter upgrade to `0.13.0`+, which will install the new `awsnodetemplates` CRD.
  5. Reapply the `awsnodetemplate` manifests you saved from step 1, if applicable.
* `0.13.0` also adds EC2/spot price fetching to Karpenter to allow making more accurate decisions regarding node deployments.  Our [getting started guide]({{< ref "../getting-started/getting-started-with-karpenter" >}}) documents this, but if you are upgrading Karpenter you will need to modify your Karpenter controller policy to add the `pricing:GetProducts` and `ec2:DescribeSpotPriceHistory` permissions.

### Upgrading to `0.12.0`+

* `0.12.0` adds an OwnerReference to each Node created by a provisioner. Previously, deleting a provisioner would orphan nodes. Now, deleting a provisioner will cause Kubernetes [cascading delete](https://kubernetes.io/docs/concepts/architecture/garbage-collection/#cascading-deletion) logic to gracefully terminate the nodes using the Karpenter node finalizer. You may still orphan nodes by removing the owner reference.
* If you are upgrading from `0.10.1` - `0.11.1`, a new CRD `awsnodetemplate` was added. In `0.12.0`, this crd was renamed to `awsnodetemplates`. Since Helm does not manage the lifecycle of CRDs, you will need to perform a few manual steps for this CRD upgrade:
  1. Make sure any `awsnodetemplate` manifests are saved somewhere so that they can be reapplied to the cluster.
  2. `kubectl delete crd awsnodetemplate`
  3. `kubectl apply -f https://raw.githubusercontent.com/aws/karpenter-provider-aws/v0.12.1/charts/karpenter/crds/karpenter.k8s.aws_awsnodetemplates.yaml`
  4. Perform the Karpenter upgrade to `0.12.0`+, which will install the new `awsnodetemplates` CRD.
  5. Reapply the `awsnodetemplate` manifests you saved from step 1, if applicable.

### Upgrading to `0.11.0`+

`0.11.0` changes the way that the `vpc.amazonaws.com/pod-eni` resource is reported.  Instead of being reported for all nodes that could support the resources regardless of if the cluster is configured to support it, it is now controlled by a command line flag or environment variable. The parameter defaults to false and must be set if your cluster uses [security groups for pods](https://docs.aws.amazon.com/eks/latest/userguide/security-groups-for-pods.html).  This can be enabled by setting the environment variable `AWS_ENABLE_POD_ENI` to true via the helm value `controller.env`.

Other extended resources must be registered on nodes by their respective device plugins which are typically installed as DaemonSets (e.g. the `nvidia.com/gpu` resource will be registered by the [NVIDIA device plugin](https://github.com/NVIDIA/k8s-device-plugin). Previously, Karpenter would register these resources on nodes at creation and they would be zeroed out by `kubelet` at startup.  By allowing the device plugins to register the resources, pods will not bind to the nodes before any device plugin initialization has occurred.

`0.11.0` adds a `providerRef` field in the Provisioner CRD. To use this new field you will need to replace the Provisioner CRD manually:

```shell
kubectl replace -f https://raw.githubusercontent.com/aws/karpenter-provider-aws/v0.11.0/charts/karpenter/crds/karpenter.sh_provisioners.yaml
```

### Upgrading to `0.10.0`+

`0.10.0` adds a new field, `startupTaints` to the provisioner spec.  Standard Helm upgrades [do not upgrade CRDs](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations) so the  field will not be available unless the CRD is manually updated.  This can be performed prior to the standard upgrade by applying the new CRD manually:

```shell
kubectl replace -f https://raw.githubusercontent.com/aws/karpenter-provider-aws/v0.10.0/charts/karpenter/crds/karpenter.sh_provisioners.yaml
```

📝 If you don't perform this manual CRD update, Karpenter will work correctly except for rejecting the creation/update of provisioners that use `startupTaints`.

### Upgrading to `0.6.2`+

If using Helm, the variable names have changed for the cluster's name and endpoint. You may need to update any configuration
that sets the old variable names.

- `controller.clusterName` is now `clusterName`
- `controller.clusterEndpoint` is now `clusterEndpoint`
