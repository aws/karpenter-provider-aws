---
title: "v1 Migration"
linkTitle: "v1 Migration"
weight: 30
description: >
  Upgrade information for migrating to v1
---

This migration guide is designed to help you migrate Karpenter from v1beta1 APIs to v1 (v0.33-v0.37).
Use this document as a reference to the changes that were introduced in this release and as a guide to how you need to update the manifests and other Karpenter objects you created in previous Karpenter releases.

Before you begin upgrading to `v1.0`, you should know that:

* Every Karpenter upgrade from pre-v1.0 versions must upgrade to minor version `v1.0`.
* You must be upgrading to `v1.0` from a version of Karpenter that only supports v1beta1 APIs, e.g. NodePools, NodeClaims, and NodeClasses (v0.33+).
* Karpenter `v1.0`+ supports Karpenter v1 and v1beta1 APIs and will not work with earlier Provisioner, AWSNodeTemplate or Machine v1alpha1 APIs. Do not upgrade to `v1.0`+ without first [upgrading to `0.32.x`]({{<ref "upgrade-guide#upgrading-to-0320" >}}) or later and then upgrading to v0.33.
* Version `v1.0` adds [conversion webhooks](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#webhook-conversion) to automatically pull the v1 API version of previously applied v1beta1 NodePools, EC2NodeClasses, and NodeClaims. Karpenter will stop serving the v1beta1 API version at v1.1 and will drop the conversion webhooks at that time. You will need to migrate all stored manifests to v1 API versions on Karpenter v1.0+. Keep in mind that this is a conversion and not dual support, which means that resources are updated in-place rather than migrated over from the previous version.
* If you need to rollback the upgrade to v1, you need to upgrade to a special patch version of the minor version you came from. For instance, if you came from v0.33.5, you'll need to downgrade back to v0.33.6. More details on how to do this in [Downgrading]({{<ref "#downgrading" >}}).
* Validate that you are running at least Kubernetes 1.25. Use the [compatibility matrix]({{<ref "compatibility#compatibility-matrix">}}) to confirm you are on a supported Kubernetes version.
*  Due to Node expiration, you may observe an increased number of pods in the "Scheduling" state.

See the [Changelog]({{<ref "#changelog" >}}) for details about actions you should take before upgrading to v1.0 or v1.1.

## Upgrade Procedure

Please read through the entire procedure before beginning the upgrade. There are major changes in this upgrade, so please evaluate the list of breaking changes before continuing.

{{% alert title="Note" color="warning" %}}
The upgrade guide will first require upgrading to your latest patch version prior to upgrade to v1.0. This will be to allow the conversion webhooks to operate and minimize downtime of the Karpenter controller when requesting the Karpenter custom resources.
{{% /alert %}}

1. Set environment variables for your cluster to upgrade to the latest patch version of the current Karpenter version you're running on:

    ```bash
    export AWS_PARTITION="aws" # if you are not using standard partitions, you may need to configure to aws-cn / aws-us-gov
    export CLUSTER_NAME="${USER}-karpenter-demo"
    export AWS_REGION="us-west-2"
    export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
    export KARPENTER_NAMESPACE=kube-system
    export KARPENTER_IAM_ROLE_ARN="arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
    ```


2. Determine the current Karpenter version:
   ```bash
   kubectl get pod -A | grep karpenter
   kubectl describe pod -n "${KARPENTER_NAMESPACE}" karpenter-xxxxxxxxxx-xxxxx | grep Image:
   ```
   Sample output:
   ```bash
   Image: public.ecr.aws/karpenter/controller:0.37.1@sha256:157f478f5db1fe999f5e2d27badcc742bf51cc470508b3cebe78224d0947674f
   ```

   The Karpenter version you are running must be between minor version `v0.33` and `v0.37`. To be able to roll back from Karpenter v1, you must rollback to on the following patch release versions for your minor version, which will include the conversion webhooks for a smooth rollback:

   * v0.37.6
   * v0.36.8
   * v0.35.11
   * v0.34.12
   * v0.33.11

3. Review for breaking changes between v0.33 and v0.37: If you are already running Karpenter v0.37.x, you can skip this step. If you are running an earlier Karpenter version, you need to review the [Upgrade Guide]({{<ref "upgrade-guide#upgrading-to-0320" >}}) for each minor release.

4. Set environment variables for upgrading to the latest patch version. Note that `v0.33` and `v0.34` both need to include the v prefix, whereas `v0.35+` should not.

  ```bash
  export KARPENTER_VERSION=<latest patch version of your current v1beta1 minor version>
  ```

5. Apply the latest patch version of your current minor version's Custom Resource Definitions (CRDs):

   ```bash
   helm upgrade --install karpenter-crd oci://public.ecr.aws/karpenter/karpenter-crd --version "${KARPENTER_VERSION}" --namespace "${KARPENTER_NAMESPACE}" --create-namespace \
        --set webhook.enabled=true \
        --set webhook.serviceName="karpenter" \
        --set webhook.port=8443
    ```

{{% alert title="Note" color="warning" %}}
If you receive a `label validation error` or `annotation validation error` consult the [troubleshooting guide]({{<ref "../troubleshooting/#helm-error-when-installing-the-karpenter-crd-chart" >}}) for steps to resolve.
{{% /alert %}}

{{% alert title="Note" color="warning" %}}

As an alternative approach to updating the Karpenter CRDs conversion webhook configuration, you can patch the CRDs as follows:

```bash
export SERVICE_NAME=<karpenter webhook service name>
export SERVICE_NAMESPACE=<karpenter webhook service namespace>
export SERVICE_PORT=<karpenter webhook service port>
# NodePools
kubectl patch customresourcedefinitions nodepools.karpenter.sh -p "{\"spec\":{\"conversion\":{\"webhook\":{\"clientConfig\":{\"service\": {\"name\": \"${SERVICE_NAME}\", \"namespace\": \"${SERVICE_NAMESPACE}\", \"port\":${SERVICE_PORT}}}}}}}"
# NodeClaims
kubectl patch customresourcedefinitions nodeclaims.karpenter.sh -p "{\"spec\":{\"conversion\":{\"webhook\":{\"clientConfig\":{\"service\": {\"name\": \"${SERVICE_NAME}\", \"namespace\": \"${SERVICE_NAMESPACE}\", \"port\":${SERVICE_PORT}}}}}}}"
# EC2NodeClass
kubectl patch customresourcedefinitions ec2nodeclasses.karpenter.k8s.aws -p "{\"spec\":{\"conversion\":{\"webhook\":{\"clientConfig\":{\"service\": {\"name\": \"${SERVICE_NAME}\", \"namespace\": \"${SERVICE_NAMESPACE}\", \"port\":${SERVICE_PORT}}}}}}}"
```
{{% /alert %}}

6. Upgrade Karpenter to the latest patch version of your current minor version's. At the end of this step, conversion webhooks will run but will not convert any version.

    ```bash
    # Service account annotation can be dropped when using pod identity
    helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter --version ${KARPENTER_VERSION} --namespace "${KARPENTER_NAMESPACE}" --create-namespace \
      --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=${KARPENTER_IAM_ROLE_ARN} \
      --set settings.clusterName=${CLUSTER_NAME} \
      --set settings.interruptionQueue=${CLUSTER_NAME} \
      --set controller.resources.requests.cpu=1 \
      --set controller.resources.requests.memory=1Gi \
      --set controller.resources.limits.cpu=1 \
      --set controller.resources.limits.memory=1Gi \
      --set webhook.enabled=true \
      --set webhook.port=8443 \
      --wait
    ```

7. Set environment variables for first upgrading to v1.0.8

    ```bash
    export KARPENTER_VERSION=1.0.8
    ```


8. Update your existing policy using the following to the v1.0.8 controller policy:
   Notable Changes to the IAM Policy include additional tag-scoping for the `eks:eks-cluster-name` tag for instances and instance profiles.

    ```bash
    export TEMPOUT=$(mktemp)
    curl -fsSL https://raw.githubusercontent.com/aws/karpenter-provider-aws/v"${KARPENTER_VERSION}"/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml > ${TEMPOUT} \
        && aws cloudformation deploy \
        --stack-name "Karpenter-${CLUSTER_NAME}" \
        --template-file "${TEMPOUT}" \
        --capabilities CAPABILITY_NAMED_IAM \
        --parameter-overrides "ClusterName=${CLUSTER_NAME}"
    ```

9. Apply the v1.0.8 Custom Resource Definitions (CRDs):

    ```bash
    helm upgrade --install karpenter-crd oci://public.ecr.aws/karpenter/karpenter-crd --version "${KARPENTER_VERSION}" --namespace "${KARPENTER_NAMESPACE}" --create-namespace \
        --set webhook.enabled=true \
        --set webhook.serviceName="karpenter" \
        --set webhook.port=8443
    ```

{{% alert title="Note" color="warning" %}}
If you receive a `label validation error` or `annotation validation error` consult the [troubleshooting guide]({{<ref "../troubleshooting/#helm-error-when-installing-the-karpenter-crd-chart" >}}) for steps to resolve.
{{% /alert %}}

{{% alert title="Note" color="warning" %}}

As an alternative approach to updating the Karpenter CRDs conversion webhook configuration, you can patch the CRDs as follows:

```bash
export SERVICE_NAME=<karpenter webhook service name>
export SERVICE_NAMESPACE=<karpenter webhook service namespace>
export SERVICE_PORT=<karpenter webhook service port>
# NodePools
kubectl patch customresourcedefinitions nodepools.karpenter.sh -p "{\"spec\":{\"conversion\":{\"webhook\":{\"clientConfig\":{\"service\": {\"name\": \"${SERVICE_NAME}\", \"namespace\": \"${SERVICE_NAMESPACE}\", \"port\":${SERVICE_PORT}}}}}}}"
# NodeClaims
kubectl patch customresourcedefinitions nodeclaims.karpenter.sh -p "{\"spec\":{\"conversion\":{\"webhook\":{\"clientConfig\":{\"service\": {\"name\": \"${SERVICE_NAME}\", \"namespace\": \"${SERVICE_NAMESPACE}\", \"port\":${SERVICE_PORT}}}}}}}"
# EC2NodeClass
kubectl patch customresourcedefinitions ec2nodeclasses.karpenter.k8s.aws -p "{\"spec\":{\"conversion\":{\"webhook\":{\"clientConfig\":{\"service\": {\"name\": \"${SERVICE_NAME}\", \"namespace\": \"${SERVICE_NAMESPACE}\", \"port\":${SERVICE_PORT}}}}}}}"
```
{{% /alert %}}

10. Upgrade Karpenter to the new version. At the end of this step, conversion webhooks run to convert the Karpenter CRDs to v1.

    ```bash
    # Service account annotion can be dropped when using pod identity
    helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter --version ${KARPENTER_VERSION} --namespace "${KARPENTER_NAMESPACE}" --create-namespace \
        --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=${KARPENTER_IAM_ROLE_ARN} \
      --set settings.clusterName=${CLUSTER_NAME} \
      --set settings.interruptionQueue=${CLUSTER_NAME} \
      --set controller.resources.requests.cpu=1 \
      --set controller.resources.requests.memory=1Gi \
      --set controller.resources.limits.cpu=1 \
      --set controller.resources.limits.memory=1Gi \
      --wait
    ```

   {{% alert title="Note" color="warning" %}}
   Karpenter has deprecated and moved a number of Helm values as part of the v1 release. Ensure that you upgrade to the newer version of these helm values during your migration to v1. You can find detail for all the settings that were moved in the [v1 Upgrade Reference]({{<ref "#helm-values" >}}).
   {{% /alert %}}

11. Once upgraded, you won't need to roll your nodes to be compatible with v1.1, except if you have multiple NodePools with different `kubelet`s that are referencing the same EC2NodeClass. Karpenter has moved the `kubelet` to the EC2NodeClass in v1. NodePools with different `kubelet` referencing the same EC2NodeClass will be compatible with v1.0, but will not be in v1.1.

When you have completed the migration to `1.0` CRDs, Karpenter will be able to serve both the `v1beta1` versions and the `v1` versions of NodePools, NodeClaims, and EC2NodeClasses.
The results of upgrading these CRDs include the following:

* The storage version of these resources change to v1. After the upgrade, Karpenter starts converting these resources to v1 storage versions in real time.  Users should experience no differences from this change.
* You are still able to GET and make updates using the v1beta1 versions, by for example doing `kubectl get nodepools.v1beta1.karpenter.sh`.


## Post upgrade considerations

Your NodePool and EC2NodeClass objects are auto-converted to the new v1 storage version during the upgrade. Consider getting the latest versions of those objects to update any stored manifests where you were previously applying the v1beta1 version.

   * [NodePools]({{<ref "../concepts/nodepools" >}}): Get the latest copy of your NodePool (`kubectl get nodepool default -o yaml > nodepool.yaml`) and review the [Changelog]({{<ref "#changelog" >}}) for changes to NodePool objects. Make modifications as needed.
   * [EC2NodeClasses]({{<ref "../concepts/nodeclasses" >}}): Get the latest copy of your EC2NodeClass (`kubectl get ec2nodeclass default -o yaml > ec2nodeclass.yaml`) and review the [Changelog]({{<ref "#changelog" >}}) for changes to EC2NodeClass objects. Make modifications as needed.

When you are satisfied with your NodePool and EC2NodeClass files, apply them as follows:

```bash
kubectl apply -f nodepool.yaml
kubectl apply -f ec2nodeclass.yaml
```

## Changelog
Refer to the [Full Changelog]({{<ref "#full-changelog" >}}) for more.

Because Karpenter `v1.0` will run both `v1` and `v1beta1` versions of NodePools and EC2NodeClasses, you don't immediately have to upgrade the stored manifests that you have to v1.
However, in preparation for later Karpenter upgrades (which will not support `v1beta1`, review the following changes from v1beta1 to v1.

Karpenter `v1.0` changes are divided into two different categories: those you must do before `1.0` upgrades and those you must do before `1.1` upgrades.

### Changes required before upgrading to `v1.0`

Apply the following changes to your NodePools and EC2NodeClasses, as appropriate, before upgrading them to v1.

* **Deprecated annotations, labels and tags are removed for v1.0**: For v1, `karpenter.sh/do-not-consolidate` (annotation), `karpenter.sh/do-not-evict
(annotation)`, and `karpenter.sh/managed-by` (tag) all have support removed.
The `karpenter.sh/managed-by`, which currently stores the cluster name in its value, is replaced by `eks:eks-cluster-name`, to allow
for [EKS Pod Identity ABAC policies](https://docs.aws.amazon.com/eks/latest/userguide/pod-id-abac.html). `karpenter.sh/do-not-consolidate` and `karpenter.sh/do-not-evict` are both replaced by `karpenter.sh/do-not-disrupt`.

* **Zap logging config removed**: Support for setting the Zap logging config was deprecated in beta and is now removed for v1. View the [Logging Configuration Section of the v1beta1 Migration Guide]({{<ref "../../v0.32/upgrading/v1beta1-migration#logging-configuration-is-no-longer-dynamic" >}}) for more details.

* **metadataOptions could break workloads**: If you have workload pods that are not using `hostNetworking`, the updated default `metadataOptions` could cause your containers to break when you apply new EC2NodeClasses on v1.

* **Ubuntu AMIFamily Removed**:

   Support for automatic AMI selection and UserData generation for Ubuntu has been dropped with Karpenter `v1.0`.
   To continue using Ubuntu AMIs you will need to specify an AMI using `amiSelectorTerms`.

   UserData generation can be achieved using the AL2 AMIFamily which has an identical UserData format.
   However, compatibility is not guaranteed long-term and changes to either AL2 or Ubuntu's UserData format may introduce incompatibilities.
   If this occurs, the Custom AMIFamily should be used for Ubuntu and UserData will need to be entirely maintained by the user.

   If you are upgrading to `v1.0` and already have v1beta1 Ubuntu EC2NodeClasses, all you need to do is specify `amiSelectorTerms` and Karpenter will translate your NodeClasses to the v1 equivalent (as shown below).
   Failure to specify `amiSelectorTerms` will result in the EC2NodeClass and all referencing NodePools to show as NotReady, causing Karpenter to ignore these NodePools and EC2NodeClasses for Provisioning and Drift.

   ```yaml
   # Original v1beta1 EC2NodeClass
   version: karpenter.k8s.aws/v1beta1
   kind: EC2NodeClass
   spec:
     amiFamily: Ubuntu
     amiSelectorTerms:
     - id: ami-foo
   ---
   # Conversion Webhook Output
   version: karpenter.k8s.aws/v1
   kind: EC2NodeClass
   metadata:
     annotations:
       compatibility.karpenter.k8s.aws/v1beta1-ubuntu: amiFamily,blockDeviceMappings
   spec:
     amiFamily: AL2
     amiSelectorTerms:
     - id: ami-foo
     blockDeviceMappings:
     - deviceName: '/dev/sda1'
       rootVolume: true
       ebs:
         encrypted: true
         volumeType: gp3
         volumeSize: 20Gi
   ```

* **amiSelectorTerms and amiFamily**: For v1, `amiFamily` is no longer required if you instead specify an `alias` in `amiSelectorTerms` in your `EC2NodeClass`. You need to update your `amiSelectorTerms` and `amiFamily` if you are using:
   * A Custom amiFamily. You must ensure that the node you add the `karpenter.sh/unregistered:NoExecute` taint in your UserData.
   * An Ubuntu AMI, as described earlier.

### Before upgrading to `v1.1`

Apply the following changes to your NodePools and EC2NodeClasses, as appropriate, before upgrading them to `v1.1` (though okay to make these changes for `1.0`)

* **v1beta1 support gone**: In `v1.1`, v1beta1 is not supported. So you need to:
   * Migrate all Karpenter yaml files [NodePools]({{<ref "../concepts/nodepools" >}}), [EC2NodeClasses]({{<ref "../concepts/nodeclasses" >}}) to v1.
   * Know that all resources in the cluster also need to be on v1. It's possible (although unlikely) that some resources still may be stored as v1beta1 in ETCD if no writes had been made to them since the v1 upgrade.  You could use a tool such as [kube-storage-version-migrator](https://github.com/kubernetes-sigs/kube-storage-version-migrator) to handle this.
   * Know that you cannot rollback to v1beta1 once you have upgraded to `v1.1`.

* **Kubelet Configuration**: If you have multiple NodePools pointing to the same EC2NodeClass that have different kubeletConfigurations,
then you have to manually add more EC2NodeClasses and point their NodePools to them. This will induce drift and you will have to roll your cluster.
If you have multiple NodePools pointing to the same EC2NodeClass, but they have the same configuration, then you can proceed with the migration
without having drift or having any additional NodePools or EC2NodeClasses configured.

* **Remove kubelet annotation from NodePools**: During the upgrade process Karpenter will rely on the `compatibility.karpenter.sh/v1beta1-kubelet-conversion` annotation to determine whether to use the v1beta1 NodePool kubelet configuration or the v1 EC2NodeClass kubelet configuration. The   `compatibility.karpenter.sh/v1beta1-kubelet-conversion` NodePool annotation takes precedence over the EC2NodeClass Kubelet configuration when launching nodes. Remove the kubelet-configuration annotation (`compatibility.karpenter.sh/v1beta1-kubelet-conversion`) from your NodePools once you have migrated kubelet from the NodePool to the EC2NodeClass.

Keep in mind that rollback, without replacing the Karpenter nodes, will not be supported to an earlier version of Karpenter once the annotation is removed. This annotation is only used to support the kubelet configuration migration path, but will not be supported in v1.1.

### Downgrading

Once the Karpenter CRDs are upgraded to v1, conversion webhooks are needed to help convert APIs that are stored in etcd from v1 to v1beta1. Also changes to the CRDs will need to at least include the latest version of the CRD in this case being v1. The patch versions of the v1beta1 Karpenter controller that include the conversion wehooks include:

* v0.37.6
* v0.36.8
* v0.35.11
* v0.34.12
* v0.33.11

{{% alert title="Note" color="warning" %}}
When rolling back from v1, Karpenter will not retain data that was only valid in v1 APIs. For instance, if you were upgrading from v0.33.5 to v1, updated the `NodePool.Spec.Disruption.Budgets` field and then rolled back to v0.33.6, Karpenter would not retain the `NodePool.Spec.Disruption.Budgets` field, as that was introduced in v0.34.x. If you are configuring the kubelet field, and have removed the `compatibility.karpenter.sh/v1beta1-kubelet-conversion` annotation, rollback is not supported without replacing your nodes between EC2NodeClass and NodePool.
{{% /alert %}}

{{% alert title="Note" color="warning" %}}
Since both v1beta1 and v1 will be served, `kubectl` will default to returning the `v1` version of your CRDs. To interact with the v1beta1 version of your CRDs, you'll need to add the full resource path (including api version) into `kubectl` calls. For example: `k get nodeclaim.v1beta1.karpenter.sh`
{{% /alert %}}

1. Set environment variables

```bash
export AWS_PARTITION="aws" # if you are not using standard partitions, you may need to configure to aws-cn / aws-us-gov
export CLUSTER_NAME="${USER}-karpenter-demo"
export AWS_REGION="us-west-2"
export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
export KARPENTER_NAMESPACE=kube-system
export KARPENTER_IAM_ROLE_ARN="arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
```

2. Set Karpenter Version

```bash
# Note: v0.33.9 and v0.34.10 include the v prefix, omit it for versions v0.35+
export KARPENTER_VERSION="<rollback version of karpenter>"
```

{{% alert title="Warning" color="warning" %}}
If you open a new shell to run steps in this procedure, you need to set some or all of the environment variables again.
To remind yourself of these values, type:

```bash
echo "${KARPENTER_NAMESPACE}" "${KARPENTER_VERSION}" "${CLUSTER_NAME}" "${TEMPOUT}"
```

{{% /alert %}}

3. Rollback the Karpenter Policy

**v0.33 and v0.34:**
```bash
export TEMPOUT=$(mktemp)
curl -fsSL https://raw.githubusercontent.com/aws/karpenter-provider-aws/"${KARPENTER_VERSION}"/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml > ${TEMPOUT} \
    && aws cloudformation deploy \
    --stack-name "Karpenter-${CLUSTER_NAME}" \
    --template-file "${TEMPOUT}" \
    --capabilities CAPABILITY_NAMED_IAM \
    --parameter-overrides "ClusterName=${CLUSTER_NAME}"
```

**v0.35+:**
```bash
export TEMPOUT=$(mktemp)
curl -fsSL https://raw.githubusercontent.com/aws/karpenter-provider-aws/v"${KARPENTER_VERSION}"/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml > ${TEMPOUT} \
    && aws cloudformation deploy \
    --stack-name "Karpenter-${CLUSTER_NAME}" \
    --template-file "${TEMPOUT}" \
    --capabilities CAPABILITY_NAMED_IAM \
    --parameter-overrides "ClusterName=${CLUSTER_NAME}"
```

4. Rollback the CRDs

```bash
helm upgrade --install karpenter-crd oci://public.ecr.aws/karpenter/karpenter-crd --version "${KARPENTER_VERSION}" --namespace "${KARPENTER_NAMESPACE}" --create-namespace \
  --set webhook.enabled=true \
  --set webhook.serviceName=karpenter \
  --set webhook.port=8443
```

{{% alert title="Note" color="warning" %}}

As an alternative approach to updating the Karpenter CRDs conversion webhook configuration, you can patch the CRDs as follows:

```bash
export SERVICE_NAME=<karpenter webhook service name>
export SERVICE_NAMESPACE=<karpenter webhook service namespace>
export SERVICE_PORT=<karpenter webhook service port>
# NodePools
kubectl patch customresourcedefinitions nodepools.karpenter.sh -p "{\"spec\":{\"conversion\":{\"webhook\":{\"clientConfig\":{\"service\": {\"name\": \"${SERVICE_NAME}\", \"namespace\": \"${SERVICE_NAMESPACE}\", \"port\":${SERVICE_PORT}}}}}}}"
# NodeClaims
kubectl patch customresourcedefinitions nodeclaims.karpenter.sh -p "{\"spec\":{\"conversion\":{\"webhook\":{\"clientConfig\":{\"service\": {\"name\": \"${SERVICE_NAME}\", \"namespace\": \"${SERVICE_NAMESPACE}\", \"port\":${SERVICE_PORT}}}}}}}"
# EC2NodeClass
kubectl patch customresourcedefinitions ec2nodeclasses.karpenter.k8s.aws -p "{\"spec\":{\"conversion\":{\"webhook\":{\"clientConfig\":{\"service\": {\"name\": \"${SERVICE_NAME}\", \"namespace\": \"${SERVICE_NAMESPACE}\", \"port\":${SERVICE_PORT}}}}}}}"
```
{{% /alert %}}

5. Rollback the Karpenter Controller

```bash
# Service account annotation can be dropped when using pod identity
helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter --version ${KARPENTER_VERSION} --namespace "${KARPENTER_NAMESPACE}" --create-namespace \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"=${KARPENTER_IAM_ROLE_ARN} \
  --set settings.clusterName=${CLUSTER_NAME} \
  --set settings.interruptionQueue=${CLUSTER_NAME} \
  --set controller.resources.requests.cpu=1 \
  --set controller.resources.requests.memory=1Gi \
  --set controller.resources.limits.cpu=1 \
  --set controller.resources.limits.memory=1Gi \
  --set webhook.enabled=true \
  --set webhook.port=8443 \
  --wait
```

Karpenter should now be pulling and operating against the v1beta1 APIVersion as it was prior to the upgrade

## Full Changelog
* Features:
  * AMI Selector Terms has a new Alias field which can only be set by itself in `EC2NodeClass.Spec.AMISelectorTerms`
  * Disruption Budgets by Reason was added to `NodePool.Spec.Disruption.Budgets`
  * TerminationGracePeriod was added to `NodePool.Spec.Template.Spec`.
  * LOG_OUTPUT_PATHS and LOG_ERROR_OUTPUT_PATHS environment variables added
* API Rename: NodePool’s ConsolidationPolicy `WhenUnderutilized` is now renamed to `WhenEmptyOrUnderutilized`
* Behavior Changes:
  * Expiration is now forceful and begins draining as soon as it’s expired. Karpenter does not wait for replacement capacity to be available before draining, but will start provisioning a replacement as soon as the node is expired and begins draining.
  * Karpenter's generated NodeConfig now takes precedence when generating UserData with the AL2023 `amiFamily`. If you're setting any values managed by Karpenter in your AL2023 UserData, configure these through Karpenter natively (e.g. kubelet configuration fields).
  * Karpenter now adds a `karpenter.sh/unregistered:NoExecute` taint to nodes in injected UserData when using alias in AMISelectorTerms or non-Custom AMIFamily. When using `amiFamily: Custom`, users will need to add this taint into their UserData, where Karpenter will automatically remove it when provisioning nodes.
  * Discovered standard AL2023 AMIs will no longer be considered compatible with GPU / accelerator workloads. If you're using an AL2023 EC2NodeClass (without AMISelectorTerms) for these workloads, you will need to select your AMI via AMISelectorTerms (non-alias).
  * Karpenter now waits for underlying instances to be completely terminated before removing the associated nodes. This means it may take longer for nodes to be deleted and for nodeclaims to get cleaned up.
  * NodePools now have [status conditions]({{< relref "../concepts/nodepools/#statusconditions" >}}) that indicate if they are ready. If not, then they will not be considered during scheduling.
  * NodeClasses now have [status conditions]({{< relref "../concepts/nodeclasses/#statusconditions" >}}) that indicate if they are ready. If they are not ready, NodePools that reference them through their `nodeClassRef` will not be considered during scheduling.
* API Moves:
  * ExpireAfter has moved from the `NodePool.Spec.Disruption` block to `NodePool.Spec.Template.Spec`, and is now a drift-able field.
  * `Kubelet` was moved to the EC2NodeClass from the NodePool.
* RBAC changes: added `delete pods` | added `get, patch crds` | added `get, patch crd status` | added `update nodes` | removed `create nodes`
* Breaking API (Manual Migration Needed):
  * Ubuntu is dropped as a first class supported AMI Family
  * `karpenter.sh/do-not-consolidate` (annotation), `karpenter.sh/do-not-evict` (annotation), and `karpenter.sh/managed-by` (tag) are all removed. `karpenter.sh/managed-by`, which currently stores the cluster name in its value, will be replaced by eks:eks-cluster-name. `karpenter.sh/do-not-consolidate` and `karpenter.sh/do-not-evict` are both replaced by `karpenter.sh/do-not-disrupt`.
  * The taint used to mark nodes for disruption and termination changed from `karpenter.sh/disruption=disrupting:NoSchedule` to `karpenter.sh/disrupted:NoSchedule`. It is not recommended to tolerate this taint, however, if you were tolerating it in your applications, you'll need to adjust your taints to reflect this.
* Environment Variable Changes:
  * Environment Variable Changes
  * LOGGING_CONFIG, ASSUME_ROLE_ARN, ASSUME_ROLE_DURATION Dropped
  * LEADER_ELECT renamed to DISABLE_LEADER_ELECTION
  * `FEATURE_GATES.DRIFT=true` was dropped and promoted to Stable, and cannot be disabled.
      * Users currently opting out of drift, disabling the drift feature flag will no longer be able to do so.
* Defaults changed:
  * API: Karpenter will drop support for IMDS access from containers by default on new EC2NodeClasses by updating the default of `httpPutResponseHopLimit` from 2 to 1.
  * API: ConsolidateAfter is required. Users couldn’t set this before with ConsolidationPolicy: WhenUnderutilized, where this is now required. Users can set it to 0 to have the same behavior as in v1beta1.
  * API: All `NodeClassRef` fields are now all required, and apiVersion has been renamed to group
  * API: AMISelectorTerms are required. Setting an Alias cannot be done with any other type of term, and must match the AMI Family that's set or be Custom.
  * Helm: Deployment spec TopologySpreadConstraint to have required zonal spread over preferred. Users who had one node running their Karpenter deployments need to either:
    * Have two nodes in different zones to ensure both Karpenter replicas schedule
    * Scale down their Karpenter replicas from 2 to 1 in the helm chart
    * Edit and relax the topology spread constraint in their helm chart from DoNotSchedule to ScheduleAnyway
  * Helm/Binary: `controller.METRICS_PORT` default changed back to 8080

### Updated metrics

Changes to Karpenter metrics from v1beta1 to v1 are shown in the following tables.

This table shows metrics names that changed from v1beta1 to v1:

| Metric type | v1beta1 metrics name | new v1 metrics name |
|--|--|--|
| Node | karpenter_nodes_termination_time_seconds | karpenter_nodes_termination_duration_seconds |
| Node | karpenter_nodes_terminated | karpenter_nodes_terminated_total |
| Node | karpenter_nodes_leases_deleted | karpenter_nodes_leases_deleted_total |
| Node | karpenter_nodes_created | karpenter_nodes_created_total |
| Pod | karpenter_pods_startup_time_seconds | karpenter_pods_startup_duration_seconds |
| Disruption | karpenter_disruption_replacement_nodeclaim_failures_total | karpenter_voluntary_disruption_queue_failures_total |
| Disruption | karpenter_disruption_evaluation_duration_seconds | karpenter_voluntary_disruption_decision_evaluation_duration_seconds |
| Disruption | karpenter_disruption_eligible_nodes | karpenter_voluntary_disruption_eligible_nodes |
| Disruption | karpenter_disruption_consolidation_timeouts_total | karpenter_voluntary_disruption_consolidation_timeouts_total |
| Disruption | karpenter_disruption_budgets_allowed_disruptions | karpenter_nodepools_allowed_disruptions |
| Disruption | karpenter_disruption_actions_performed_total | karpenter_voluntary_disruption_decisions_total |
| Provisioner | karpenter_provisioner_scheduling_simulation_duration_seconds | karpenter_scheduler_scheduling_duration_seconds |
| Provisioner | karpenter_provisioner_scheduling_queue_depth | karpenter_scheduler_queue_depth |
| Interruption | karpenter_interruption_received_messages | karpenter_interruption_received_messages_total |
| Interruption | karpenter_interruption_deleted_messages | karpenter_interruption_deleted_messages_total |
| Interruption | karpenter_interruption_message_latency_time_seconds | karpenter_interruption_message_queue_duration_seconds |
| NodePool     | karpenter_nodepool_usage | karpenter_nodepools_usage |
| NodePool     | karpenter_nodepool_limit | karpenter_nodepools_limit |
| NodeClaim    | karpenter_nodeclaims_terminated | karpenter_nodeclaims_terminated_total |
| NodeClaim    | karpenter_nodeclaims_disrupted | karpenter_nodeclaims_disrupted_total |
| NodeClaim    | karpenter_nodeclaims_created | karpenter_nodeclaims_created_total |

This table shows v1beta1 metrics that were dropped for v1:

| Metric type | Metric dropped for v1 |
|--|--|
| Disruption  | karpenter_disruption_replacement_nodeclaim_initialized_seconds |
| Disruption  | karpenter_disruption_queue_depth |
| Disruption  | karpenter_disruption_pods_disrupted_total |
|             | karpenter_consistency_errors |
| NodeClaim   | karpenter_nodeclaims_registered |
| NodeClaim   | karpenter_nodeclaims_launched |
| NodeClaim   | karpenter_nodeclaims_initialized |
| NodeClaim   | karpenter_nodeclaims_drifted |
| Provisioner | karpenter_provisioner_scheduling_duration_seconds |
| Interruption | karpenter_interruption_actions_performed |

{{% alert title="Note" color="warning" %}}
Karpenter now waits for the underlying instance to be completely terminated before deleting a node and orchestrates this by emitting `NodeClaimNotFoundError`. With this change we expect to see an increase in the `NodeClaimNotFoundError`. Customers can filter out this error by label in order to get accurate values for `karpenter_cloudprovider_errors_total` metric. Use this Prometheus filter expression - `({controller!="node.termination"} or {controller!="nodeclaim.termination"}) and {error!="NodeClaimNotFoundError"}`.
{{% /alert %}}
