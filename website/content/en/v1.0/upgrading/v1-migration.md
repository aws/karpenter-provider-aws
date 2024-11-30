---
title: "v1 Migration"
linkTitle: "v1 Migration"
weight: 30
description: >
  Migrating to Karpenter `v1.0`
---

This migration guide is designed to help you migrate to Karpenter `v1.0.x` from `v0.33.x` through `v0.37.x`.
Use this document as a reference to the changes that were introduced in this release and as a guide to how you need to update the manifests and other Karpenter objects you created in previous Karpenter releases.

Before continuing with this guide, you should know that Karpenter `v1.0.x` only supports Karpenter `v1` and `v1beta1` APIs.
Earlier `Provisioner`, `AWSNodeTemplate`, and `Machine` APIs are not supported.
Do not upgrade to `v1.0.x` without first [upgrading to `v0.32.x`]({{<ref "upgrade-guide#upgrading-to-0320" >}}) and then upgrading to `v0.33+`.

Additionaly, validate that you are running at least Kubernetes `1.25`.
Use the [compatibility matrix]({{<ref "compatibility#compatibility-matrix">}}) to confirm you are on a supported Kubernetes version.

## Before You Start

Karpenter `v1.0` is a major release and contains a number of breaking changes.
The following section will highlight some of the major breaking changes, but you should review the full [changelog]({{<ref "#changelog">}}) before proceeding with the upgrade.

#### Deprecated Annotations, Labels, and Tags Removed

The following annotations, labels, and tags have been removed in `v1.0.0`:

|Key|Type|
|-|-|
|`karpenter.sh/do-not-consolidate`|annotation|
|`karpenter.sh/do-not-evict`|annotation|
|`karpenter.sh/managed-by`|tag|

Both the `karpenter.sh/do-not-consolidate` and the `karpenter.sh/do-not-evict` annotations were [deprecated in `v0.32.0`]({{<ref "../../v0.32/upgrading/v1beta1-migration/#annotations-labels-and-status-conditions">}}).
They have now been dropped in-favor of their replacement, `karpenter.sh/do-not-disrupt`.

The `karpenter.sh/managed-by`, which currently stores the cluster name in its value, is replaced by `eks:eks-cluster-name`, to allow
for [EKS Pod Identity ABAC policies](https://docs.aws.amazon.com/eks/latest/userguide/pod-id-abac.html).

#### Zap Logging Config Removed

Support for setting the Zap logging config was [deprecated in `v0.32.0`]({{<ref "../../v0.32/upgrading/v1beta1-migration#logging-configuration-is-no-longer-dynamic" >}}) and has been been removed in `v1.0.0`.
The following environment variables are now available to configure logging:

* `LOG_LEVEL`
* `LOG_OUTPUT_PATHS`
* `LOG_ERROR_OUTPUT_PATHS`.

Refer to [Settings]({{<ref "../reference/settings.md">}}) for more details.

#### New MetadataOptions Defaults

The default value for `httpPutResponseHopLimit` has been reduced from `2` to `1`.
This prevents pods that are not using `hostNetworking` from accessing IMDS by default.
If you have pods which rely on access to IMDS, and are not using `hostNetworking`, you will need to either update the pod's networking config or configure `httpPutResponseHopLimit` on your `EC2NodeClass`.
This change aligns Karpenter's defaults with [EKS' Best Practices](https://aws.github.io/aws-eks-best-practices/security/docs/iam/#restrict-access-to-the-instance-profile-assigned-to-the-worker-node).

#### Ubuntu AMIFamily Removed

Support for automatic AMI selection and UserData generation for Ubuntu has been dropped in Karpenter `v1.0.0`.
To continue using Ubuntu AMIs you will need to specify an AMI using [`amiSelectorTerms`]({{<ref "../concepts/nodeclasses#specamiselectorterms">}}).

UserData generation can be achieved using `amiFamily: AL2`, which has an identical UserData format.
However, compatibility is not guaranteed long-term and changes to either AL2 or Ubuntu's UserData format may introduce incompatibilities.
If this occurs, `amiFamily: Custom` should be used for Ubuntu AMIs and UserData will need to be entirely maintained by the user.

If you are upgrading to `v1.0.x` and already have `v1beta1` Ubuntu `EC2NodeClasses`, all you need to do is specify `amiSelectorTerms` and Karpenter will translate your `EC2NodeClasses` to the `v1` equivalent (as shown below).
Failure to specify `amiSelectorTerms` will result in the `EC2NodeClass` and all referencing `NodePools` to become `NotReady`.
These `NodePools` and `EC2NodeClasses` would then be ignored for provisioning and drift.

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

#### New Registration Taint

`EC2NodeClasses` using `amiFamily: Custom` must configure the kubelet to register with the `karpenter.sh/unregistered:NoExecute` taint.
For example, to achieve this with an AL2023 AMI you would use the following UserData:

```yaml
version: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
spec:
  amiFamily: Custom
  amiSelectorTerms:
    - id: ami-custom-al2023-ami
  userData: |
    apiVersion: node.eks.aws/v1alpha1
    kind: NodeConfig
    spec:
      # ...
      kubelet:
        config:
          # ...
          registerWithTaints:
            - key: karpenter.sh/unregistered
              effect: NoExecute
```

If you are using one of Karpenter's managed AMI families, this will be handled for you by Karpenter's [generated UserData]({{<ref "../concepts/nodeclasses.md#specuserdata">}}).

## Upgrading

Before proceeding with the upgrade, be sure to review the [changelog]({{<ref "#changelog">}}) and review the [upgrade procedure]({{<ref "#upgrade-procedure">}}) in its entirety.
The procedure can be split into two sections:

* Steps 1 through 6 will upgrade you to the latest patch release on your current minor version.
* Steps 7 through 11 will then upgrade you to the latest `v1.0` release.

While it is possible to upgrade directly from any patch version on versions `v0.33` through `v0.37`, rollback from `v1.0.x` is only supported on the latest patch releases.
Upgrading directly may leave you unable to rollback.
For more information on the rollback procedure, refer to the [downgrading section]({{<ref "#downgrading">}}).

{{% alert title="Note" color="primary" %}}
The examples provided in the [upgrade procedure]({{<ref "#upgrade-procedure">}}) assume you've installed Karpenter following the [getting started guide]({{<ref "../getting-started/getting-started-with-karpenter/_index.md">}}).
If you are using IaC / GitOps, you may need to adapt the procedure to fit your specific infrastructure solution.
You should still review the upgrade procedure; the sequence of operations remains the same regardless of the solution used to roll out the changes.
{{% /alert %}}


#### Upgrade Procedure

1. Configure environment variables for the cluster you're upgrading:

    ```bash
    export AWS_PARTITION="aws" # if you are not using standard partitions, you may need to configure to aws-cn / aws-us-gov
    export CLUSTER_NAME="${USER}-karpenter-demo"
    export AWS_REGION="us-west-2"
    export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
    export KARPENTER_NAMESPACE=kube-system
    export KARPENTER_IAM_ROLE_ARN="arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
    ```

2. Determine your current Karpenter version:
   ```bash
   kubectl get deployment -A -l app.kubernetes.io/name=karpenter -ojsonpath="{.items[0].metadata.labels['app\.kubernetes\.io/version']}{'\n'}"
   ```

   To upgrade to v1, you must be running a Karpenter version between `v0.33` and `v0.37`.
   If you are on an older version, you must upgrade before continuing with this guide.

3. Before upgrading to v1, we're going to upgrade to a patch release that supports rollback.
   Set the `KARPENTER_VERSION` environment variable to the latest patch release for your current minor version.
   The following releases are the current latest:

   * `0.37.6`
   * `0.36.8`
   * `0.35.11`
   * `v0.34.12`
   * `v0.33.11`

   ```bash
   # Note: v0.33.x and v0.34.x include the v prefix, omit it for versions v0.35+
   export KARPENTER_VERSION="0.37.5" # Replace with your minor version
   ```

4. Upgrade Karpenter to the latest patch release for your current minor version.
   Note that webhooks must be enabled.

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

5. Apply the latest patch version of your current minor version's Custom Resource Definitions (CRDs).
   Applying this version of the CRDs will enable the use of both the `v1` and `v1beta1` APIs on this version via the conversion webhooks.
   Note that this is only for rollback purposes, and new features available with the `v1` APIs will not work on your minor version.

   ```bash
   helm upgrade --install karpenter-crd oci://public.ecr.aws/karpenter/karpenter-crd --version "${KARPENTER_VERSION}" --namespace "${KARPENTER_NAMESPACE}" --create-namespace \
       --set webhook.enabled=true \
       --set webhook.serviceName="karpenter" \
       --set webhook.port=8443
   ```
   {{% alert title="Note" color="primary" %}}
   To properly template the `conversion` field in the CRD, the `karpenter-crd` chart must be used.
   If you're using a GitOps solution to manage your Karpenter installation, you should use this chart to manage your CRDs.
   You should set `skipCrds` to true for the main `karpenter` chart (e.g. [Argo CD](https://argo-cd.readthedocs.io/en/latest/user-guide/helm/#helm-skip-crds)).

   Alternatively, you can install the CRDs with the main chart and apply the following patches. However, we strongly recommend using the dedicated CRD chart.

   ```bash
   SERVICE_NAME="karpenter"
   SERVICE_NAMESPACE="kube-system"
   SERVICE_PORT="8443"
   CRDS=("nodepools.karpenter.sh" "nodeclaims.karpenter.sh" "ec2nodeclasses.karpenter.k8s.aws")
   for crd in ${CRDS[@]}; do
       kubectl patch customresourcedefinitions ${crd} --patch-file=/dev/stdin <<-EOF
   spec:
     conversion:
       webhook:
         clientConfig:
           service:
             name: "${SERVICE_NAME}"
             namespace: "${SERVICE_NAMESPACE}"
             port: ${SERVICE_PORT}
   EOF
   done
   ```
   {{% /alert %}}

   {{% alert title="Note" color="primary" %}}
   Helm uses annotations on resources it provisions to track ownership.
   Switching to the new chart may result in Helm failing to install the chart due to `invalid ownership metadata`.
   If you encounter errors at this step, consult this [troubleshooting entry]({{<ref "../troubleshooting/#helm-error-when-installing-the-karpenter-crd-chart" >}}) to resolve.
   {{% /alert %}}


6. Validate that Karpenter is operating as expected on this patch release.
   If you need to rollback after upgrading to `v1`, this is the version you will need to rollback to.

   {{% alert title="Note" color="primary" %}}
   The conversion webhooks must be able to communicate with the API server to operate correctly.
   If you see errors related to the conversion webhooks, ensure that your security groups and network policies allow traffic between the webhooks and the API server.
   {{% /alert %}}

7. We're now ready to begin the upgrade to `v1`. Set the `KARPENTER_VERSION` environment variable to the latest `v1.0.x` release.

    ```bash
    export KARPENTER_VERSION="1.0.8"
    ```

8. Attach the v1 policy to your existing NodeRole.
   Notable Changes to the IAM Policy include additional tag-scoping for the `eks:eks-cluster-name` tag for instances and instance profiles.
   We will remove this additional policy later once the controller has been migrated to v1 and we've updated the Karpenter cloudformation stack.

   ```bash
   POLICY_DOCUMENT=$(mktemp)
   curl -fsSL https://raw.githubusercontent.com/aws/karpenter-provider-aws/13d6fc014ea59019b1c3b1953184efc41809df11/website/content/en/v1.0/upgrading/get-controller-policy.sh | sh | envsubst > ${POLICY_DOCUMENT}
   POLICY_NAME="KarpenterControllerPolicy-${CLUSTER_NAME}-v1"
   ROLE_NAME="${CLUSTER_NAME}-karpenter"
   POLICY_ARN="$(aws iam create-policy --policy-name "${POLICY_NAME}" --policy-document "file://${POLICY_DOCUMENT}" | jq -r .Policy.Arn)"
   aws iam attach-role-policy --role-name "${ROLE_NAME}" --policy-arn "${POLICY_ARN}"
   ```

9. Apply the `v1` Custom Resource Definitions (CRDs):

    ```bash
    helm upgrade --install karpenter-crd oci://public.ecr.aws/karpenter/karpenter-crd --version "${KARPENTER_VERSION}" --namespace "${KARPENTER_NAMESPACE}" --create-namespace \
        --set webhook.enabled=true \
        --set webhook.serviceName="karpenter" \
        --set webhook.port=8443
    ```

10. Upgrade Karpenter to the latest `v1.0.x` release.

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
    {{% alert title="Note" color="primary" %}}
<!-- Note: don't indent this line to match the indenting of the alert box. Hugo will create a code block. -->
Karpenter has deprecated and moved a number of Helm values as part of the v1 release. Ensure that you upgrade to the newer version of these helm values during your migration to v1. You can find detail for all the settings that were moved in the [v1 Upgrade Reference]({{<ref "#helm-values" >}}).
    {{% /alert %}}

11. Upgrade your cloudformation stack and remove the temporary `v1` controller policy.

    ```bash
    TEMPOUT=$(mktemp)
    curl -fsSL https://raw.githubusercontent.com/aws/karpenter-provider-aws/v"${KARPENTER_VERSION}"/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml > "${TEMPOUT}"
    aws cloudformation deploy \
      --stack-name "Karpenter-${CLUSTER_NAME}" \
      --template-file "${TEMPOUT}" \
      --capabilities CAPABILITY_NAMED_IAM \
      --parameter-overrides "ClusterName=${CLUSTER_NAME}"

    ROLE_NAME="${CLUSTER_NAME}-karpenter"
    POLICY_NAME="KarpenterControllerPolicy-${CLUSTER_NAME}-v1"
    POLICY_ARN=$(aws iam list-policies --query "Policies[?PolicyName=='${POLICY_NAME}'].Arn" --output text)
    aws iam detach-role-policy --role-name "${ROLE_NAME}" --policy-arn "${POLICY_ARN}"
    aws iam delete-policy --policy-arn "${POLICY_ARN}"
    ```

## Downgrading

Once you upgrade to Karpenter `v1.0.x`, both `v1` and `v1beta1` resources may be stored in ETCD.
Due to this, you may only rollback to a version of Karpenter with the conversion webhooks.
The following releases should be used as rollback targets:

* `v0.37.6`
* `v0.36.8`
* `v0.35.12`
* `v0.34.12`
* `v0.33.11`

{{% alert title="Warning" color="warning" %}}
When rolling back from `v1`, Karpenter will not retain data that was only valid in the `v1` APIs.
For instance, if you upgraded from `v0.33.5` to `v1.0.x`, updated the `NodePool.Spec.Disruption.Budgets` field, and then rolled back to `v0.33.6`, Karpenter would not retain the `NodePool.Spec.Disruption.Budgets` field, as that was introduced in `v0.34.0`.

If you have configured the `kubelet` field on your `EC2NodeClass` and have removed the `compatibility.karpenter.sh/v1beta1-kubelet-conversion` annotation from your `NodePools`, you must re-add the annotation before downgrading.
For more information, refer to [kubelet configuration migration]({{<ref "#kubelet-configuration-migration">}}).
{{% /alert %}}

{{% alert title="Note" color="primary" %}}
Since both `v1beta1` and `v1` will be served, `kubectl` will default to returning the `v1` version of your CRs.
To interact with the v1beta1 version of your CRs, you'll need to add the full resource path (including api version) into `kubectl` calls.
For example: `kubectl get nodepoll.v1beta1.karpenter.sh`.
{{% /alert %}}

#### Downgrade Procedure

1. Configure environment variables for the cluster you're downgrading:

   ```bash
   export AWS_PARTITION="aws" # if you are not using standard partitions, you may need to configure to aws-cn / aws-us-gov
   export CLUSTER_NAME="${USER}-karpenter-demo"
   export AWS_REGION="us-west-2"
   export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
   export KARPENTER_NAMESPACE=kube-system
   export KARPENTER_IAM_ROLE_ARN="arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
   ```

2. Configure your target Karpenter version. You should select one of the following versions:
   * `0.37.5`
   * `0.36.7`
   * `0.35.10`
   * `v0.34.11`
   * `v0.33.10`

   ```bash
   # Note: v0.33.x and v0.34.x include the v prefix, omit it for versions v0.35+
   export KARPENTER_VERSION="0.37.5" # Replace with your minor version
   ```

3. Attach the `v1beta1` policy from your target version to your existing NodeRole.

   ```bash
   POLICY_DOCUMENT=$(mktemp)
   curl -fsSL https://raw.githubusercontent.com/aws/karpenter-provider-aws/website/docs/v1.0/upgrading/get-controller-policy.sh | sh | envsubst > ${POLICY_DOCUMENT}
   POLICY_NAME="KarpenterControllerPolicy-${CLUSTER_NAME}-${KARPENTER_VERSION}"
   ROLE_NAME="${CLUSTER_NAME}-karpenter"
   POLICY_ARN="$(aws iam create-policy --policy-name "${POLICY_NAME}" --policy-document "file://${POLICY_DOCUMENT}" | jq -r .Policy.Arn)"
   aws iam attach-role-policy --role-name "${ROLE_NAME}" --policy-arn "${POLICY_ARN}"
   ```

4. Rollback the Karpenter Controller:
   Note that webhooks must be **enabled** to rollback.
   Without enabling the webhooks, Karpenter will be unable to correctly operate on `v1` versions of the resources already stored in ETCD.

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

5. Rollback the CRDs.

   ```bash
   helm upgrade --install karpenter-crd oci://public.ecr.aws/karpenter/karpenter-crd --version "${KARPENTER_VERSION}" --namespace "${KARPENTER_NAMESPACE}" --create-namespace \
     --set webhook.enabled=true \
     --set webhook.serviceName=karpenter \
     --set webhook.port=8443
   ```

6. Rollback your cloudformation stack and remove the temporary `v1beta1` controller policy.

   ```bash
   TEMPOUT=$(mktemp)
   VERSION_TAG=$([[ ${KARPENTER_VERSION} == v* ]] && echo "${KARPENTER_VERSION}" || echo "v${KARPENTER_VERSION}")
   curl -fsSL https://raw.githubusercontent.com/aws/karpenter-provider-aws/${VERSION_TAG}/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml > "${TEMPOUT}"
   aws cloudformation deploy \
     --stack-name "Karpenter-${CLUSTER_NAME}" \
     --template-file "${TEMPOUT}" \
     --capabilities CAPABILITY_NAMED_IAM \
     --parameter-overrides "ClusterName=${CLUSTER_NAME}"

   ROLE_NAME="${CLUSTER_NAME}-karpenter"
   POLICY_NAME="KarpenterControllerPolicy-${CLUSTER_NAME}-${KARPENTER_VERSION}"
   POLICY_ARN=$(aws iam list-policies --query "Policies[?PolicyName=='${POLICY_NAME}'].Arn" --output text)
   aws iam detach-role-policy --role-name "${ROLE_NAME}" --policy-arn "${POLICY_ARN}"
   aws iam delete-policy --policy-arn "${POLICY_ARN}"
   ```

## Before Upgrading to `v1.1.0`

You've successfully upgraded to `v1.0`, but more than likely your manifests are still `v1beta1`.
You can continue to apply these `v1beta1` manifests on `v1.0`, but support will be dropped in `v1.1`.
Before upgrading to `v1.1+`, you will need to migrate your manifests over to `v1`.

#### Manifest Migration

You can manually migrate your manifests by referring to the [changelog]({{<ref "#changelog">}}) and the updated API docs ([NodePool]({{<ref "../concepts/nodepools.md">}}), [EC2NodeClass]({{<ref "../concepts/nodeclasses.md">}})).
Alternatively, you can take advantage of the conversion webhooks.
Performing a `get` using `kubectl` will return the `v1` version of the resource, even if it was applied with a `v1beta1` manifest.

For example, applying the following `v1beta1` manifest and performing a `get` will return the `v1` equivalent:
```bash
cat <<EOF | kubectl apply -f -
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: default
spec:
  template:
    spec:
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values: ["amd64"]
        - key: kubernetes.io/os
          operator: In
          values: ["linux"]
        - key: karpenter.sh/capacity-type
          operator: In
          values: ["on-demand"]
        - key: karpenter.k8s.aws/instance-category
          operator: In
          values: ["c", "m", "r"]
        - key: karpenter.k8s.aws/instance-generation
          operator: Gt
          values: ["2"]
      nodeClassRef:
        apiVersion: karpenter.k8s.aws/v1beta1
        kind: EC2NodeClass
        name: default
  limits:
    cpu: 1000
  disruption:
    consolidationPolicy: WhenUnderutilized
    expireAfter: 720h # 30 * 24h = 720h
EOF
kubectl get nodepools default -o yaml > v1-nodepool.yaml
```

{{% alert title="Note" color="primary" %}}
Due to the many-to-one relation between `NodePools` and `EC2NodeClasses`, the `kubelet` field is **not** automtatically migrated by the conversion webhooks.
When updating your manifests, make sure you are migrating the `kubelet` field from your `NodePools` to your `EC2NodeClasses`.
For more information, refer to [kubelet configuration migration]({{<ref "#kubelet-configuration-migration">}}).
{{% /alert %}}

#### Kubelet Configuration Migration

One of the changes made to the `NodePool` and `EC2NodeClass` schemas for `v1` was the migration of the `kubelet` field from the `NodePool` to the `EC2NodeClass`.
This change is difficult to properly handle with conversion webhooks due to the many-to-one relation between `NodePools` and `EC2NodeClasses`.
To facilitate this, Karpenter adds the `compatibility.karpenter.sh/v1beta1-kubelet-conversion` annotation to converted `NodePools`.
If this annotation is present, it will take precedence over the `kubelet` field in the `EC2NodeClass`.

This annotation is only meant to support migration, and support will be dropped in `v1.1`.
Before upgrading to `v1.1+`, you must migrate your kubelet configuration to your `EC2NodeClasses`, and remove the compatibility annotation from your `NodePools`.

{{% alert title="Warning" color="warning" %}}
Do not remove the compatibility annotation until you have updated your `EC2NodeClass` with the matching `kubelet` field.
Once the annotations is removed, the `EC2NodeClass` will be used as the source of truth for your kubelet configuration.
If the field doesn't match, this will result in Nodes drifting.

If you need to rollback to a pre-`v1.0` version after removing the compatibility annotation, you must re-add it before rolling back.
{{% /alert %}}

If you have multiple `NodePools` that refer to the same `EC2NodeClass`, but have varying kubelet configurations, you will need to create a separate `EC2NodeClass` for unique set of kubelet configurations.

For example, consider the following `v1beta1` manifests:
```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: nodepool-a
spec:
  template:
    spec:
      kubelet:
        maxPods: 10
      nodeClassRef:
        apiVersion: karpenter.k8s.aws/v1beta1
        kind: EC2NodeClass
        name: nodeclass
---
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: nodepool-b
spec:
  template:
    spec:
      kubelet:
        maxPods: 20
      nodeClassRef:
        apiVersion: karpenter.k8s.aws/v1beta1
        kind: EC2NodeClass
        name: nodeclass
---
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
metadata:
  name: nodeclass
```

In this example, we have two `NodePools` with different `kubelet` values, but they refer to the same `EC2NodeClass`.
The conversion webhook will annotate the `NodePools` with the `compatibility.karpenter.sh/v1beta1-kubelet-conversion` annotation.
This is the result of that conversion:

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: nodepool-a
  annotations:
    compatibility.karpenter.sh/v1beta1-kubelet-conversion: "{\"maxPods\": 10}"
spec:
  template:
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: nodeclass
---
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: nodepool-b
  annotations:
    compatibility.karpenter.sh/v1beta1-kubelet-conversion: "{\"maxPods\": 20}"
spec:
  template:
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: nodeclass
---
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: nodeclass
```

Before upgrading to `v1.1`, you must update your `NodePools` to refer to separate `EC2NodeClasses` to retain this behavior.
Note that this will drift the Nodes associated with these NodePools due to the updated `nodeClassRef`.

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: nodepool-a
spec:
  template:
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: nodeclass-a
---
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: nodepool-b
spec:
  template:
    spec:
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: nodeclass-b
---
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: nodeclass-a
spec:
  kubelet:
    maxPods: 10
---
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: nodeclass-b
spec:
  kubelet:
    maxPods: 20
```

#### Stored Version Migration

Once you have upgraded all of your manifests, you need to ensure that all existing resources are stored as `v1` in ETCD.
Karpenter `v1.0.6`+ includes a controller to automatically migrate all stored resources to `v1`.
To validate that the migration was successful, you should check the stored versions for Karpenter's CRDs:

```bash
for crd in "nodepools.karpenter.sh" "nodeclaims.karpenter.sh" "ec2nodeclasses.karpenter.k8s.aws"; do
    kubectl get crd ${crd} -ojsonpath="{.status.storedVersions}{'\n'}"
done
```

For more details on this migration process, refer to the [kubernetes docs](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#upgrade-existing-objects-to-a-new-stored-version).

{{% alert title="Note" color="primary" %}}
If the `v1beta1` stored version persists, ensure that you are on Karpenter `v1.0.6+`.
Additionally, ensure that the storage version on the CRD in question is set to `v1`.

```bash
kubectl get crd ${crd} -ojsonpath="{.spec.versions[?(.storage==true)].name}{'\n'}"
```

If it is not, this indicates an issue upgrading the CRD when upgrading Karpenter to `v1.0.x`.
Revisit step 9 of the [upgrade procedure]({{< ref "#upgrading" >}}) and ensure the CRD was updated correctly.
{{% /alert %}}

## Changelog

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
  * Karpenter will no longer set `associatePublicIPAddress` to false in private subnets by default. Users with IAM policies / SCPs that require this field to be set explicitly should configure this through their `EC2NodeClass` ([ref]({{<ref "../concepts/nodeclasses/#specassociatepublicipaddress">}})).
* API Moves:
  * ExpireAfter has moved from the `NodePool.Spec.Disruption` block to `NodePool.Spec.Template.Spec`, and is now a drift-able field.
  * `Kubelet` was moved to the EC2NodeClass from the NodePool.
* RBAC changes: added `delete pods` | added `get, patch crds` | added `update nodes` | removed `create nodes`
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

The following changes have been made to Karpenter's metrics in `v1.0.0`.

#### Renamed Metrics

| Type | Original Name | New Name |
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

#### Dropped Metrics

| Type | Name |
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
Karpenter now waits for the underlying instance to be completely terminated before deleting a node and orchestrates this by emitting `NodeClaimNotFoundError`.
With this change we expect to see an increase in the `NodeClaimNotFoundError`.
Customers can filter out this error by label in order to get accurate values for `karpenter_cloudprovider_errors_total` metric.
Use this Prometheus filter expression - `({controller!="node.termination"} or {controller!="nodeclaim.termination"}) and {error!="NodeClaimNotFoundError"}`.
{{% /alert %}}
