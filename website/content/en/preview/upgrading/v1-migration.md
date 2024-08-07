---
title: "v1 Migration"
linkTitle: "v1 Migration"
weight: 30
description: >
  Upgrade information for migrating to v1
---

This migration guide is designed to help you migrate Karpenter from a pre-v1 version (v1beta1) to Karpenter v1.
Use this document as a reference to the changes that were introduced in this release and as a guide to how you need to update the manifests and other Karpenter objects you created in previous Karpenter releases.

Before you begin upgrading to `1.0.0`, you should know that:

* Every Karpenter upgrade from pre-v1.0.0 versions must go through an upgrade to minor version `v1.0.0`.
* You must be upgrading to `v1.0.0` from a version of Karpenter that supports NodePools, NodeClaims, and NodeClasses (`0.32.0`+ Karpenter versions support v1beta1 APIs).
* Karpenter `1.0.0`+ supports Karpenter v1 and v1beta1 APIs and will not work with earlier Provisioner, AWSNodeTemplate or Machine alpha APIs. Do not upgrade to `1.0.0`+ without first [upgrading to `0.32.x`]({{<ref "upgrade-guide#upgrading-to-0320" >}}) or later.
* Version `1.0.0` adds [conversion webhooks](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#webhook-conversion) to automatically pull the v1 API version of previously applied v1beta1 NodePools, EC2NodeClasses, and NodeClaims. Karpenter will stop serving the v1beta1 API version at v1.1.0 and will drop the conversion webhooks at that time. You will need to migrate all stored manifests to v1 API versions on Karpenter v1.0+. Keep in mind that this is a conversion and not dual support, which means that resources are updated in-place rather than migrated over from the previous version.

See the [Changelog]({{<ref "#changelog" >}}) for details about actions you should take before upgrading to v1.0 or v1.1.

## Upgrade Procedure

Please read through the entire procedure before beginning the upgrade. There are major changes in this upgrade, so you should carefully evaluate your cluster and workloads before proceeding.

1. Determine the current cluster version: Run the following to make check your Karpenter version:
   ```bash
   kubectl get pod -A | grep karpenter
   kubectl describe pod -n karpenter karpenter-xxxxxxxxxx-xxxxx | grep Image: 
   ```
   Sample output:
   ```bash
   Image: public.ecr.aws/karpenter/controller:0.37.1@sha256:157f478f5db1fe999f5e2d27badcc742bf51cc470508b3cebe78224d0947674f
   ```

   The Karpenter version you are running must be between minor version `v0.33` and `v0.37`. To be able to roll back from Karpenter v1, you must be on at least the following patch release versions for your minor version, which will include the conversion webhooks for a smooth rollback:

   * v0.37.1
   * v0.36.3
   * v0.35.6
   * v0.34.7
   * v0.33.6

2. Review for breaking changes: If you are already running Karpenter v0.37.x, you can skip this step. If you are running an earlier Karpenter version, you need to review the [Upgrade Guide]({{<ref "upgrade-guide#upgrading-to-0320" >}}) for each minor release.

3. Set environment variables for your cluster:

    ```bash
    export KARPENTER_NAMESPACE=kube-system
    export KARPENTER_VERSION=1.0.0
    export AWS_PARTITION="aws" # if you are not using standard partitions, you may need to configure to aws-cn / aws-us-gov
    export CLUSTER_NAME="${USER}-karpenter-demo"
    export AWS_REGION="us-west-2"
    export AWS_ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
    export KARPENTER_IAM_ROLE_ARN="arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter"
    export CLUSTER_ENDPOINT="$(aws eks describe-cluster --name ${CLUSTER_NAME} --query "cluster.endpoint" --output text)"
    ```

4. Update your existing policy using the following:

    ```bash
    TEMPOUT=$(mktemp)

    curl -fsSL https://raw.githubusercontent.com/aws/karpenter-provider-aws/v"${KARPENTER_VERSION}"/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml > ${TEMPOUT} \
        && aws cloudformation deploy \
        --stack-name "Karpenter-${CLUSTER_NAME}" \
        --template-file "${TEMPOUT}" \
        --capabilities CAPABILITY_NAMED_IAM \
        --parameter-overrides "ClusterName=${CLUSTER_NAME}"
    ```

5. Apply the v1.0.0 Custom Resource Definitions (CRDs):

   ```bash
   KARPENTER_NAMESPACE=kube-system 
   helm upgrade --install karpenter-crd oci://public.ecr.aws/karpenter/karpenter-crd --version "${KARPENTER_VERSION}" --namespace "${KARPENTER_NAMESPACE}" --create-namespace \
        --set webhook.enabled=true \
        --set webhook.serviceName=karpenter \
        --set webhook.serviceNamespace="${KARPENTER_NAMESPACE}" \
        --set webhook.port=8443
    ```

6. Upgrade Karpenter to the new version. At the end of this process, conversion webhooks run to convert the Karpenter CRDs to v1.

    ```bash
    helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter --version ${KARPENTER_VERSION} --namespace "${KARPENTER_NAMESPACE}" --create-namespace \
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

   Your upgraded Karpenter controller is now installed.

7. Rolling over nodes: There is no need to roll over nodes in most cases. One case where you will need to roll nodes is if you have multiple NodePools with different `kubeletConfiguration`s that are referencing the same EC2NodeClass.

When you have completed the migration to `1.0.0` CRDs, Karpenter will be able to serve both the `v1beta1` versions and the `v1` versions of NodePools, NodeClaims, and EC2NodeClasses.
The results of upgrading these CRDs include the following:

* The storage version of these resources change to v1. After the upgrade, Karpenter starts converting these resources to v1 storage versions in real time.  Users should experience no differences from this change.
* You are still able to GET and make updates using the v1beta1 versions.


## Post upgrade considerations

Your NodePool and EC2NodeClass objects are auto-converted to the new v1 storage version during the upgrade. Consider getting the latest versions of those objects to store separately and possibly update.

   * ([NodePools]({{<ref "../concepts/nodepools" >}}): Get the latest copy of your NodePool (`kubectl describe nodepool default > nodepool.yaml`) and review the [Changelog]({{<ref "#changelog" >}}) for changes to NodePool objects. Make modifications as needed.
   * [EC2NodeClasses]({{<ref "../concepts/nodeclasses" >}}): Get the latest copy of your EC2NodeClass (`kubectl describe ec2nodeclass default > ec2nodeclass.yaml`) and review the [Changelog]({{<ref "#changelog" >}}) for changes to EC2NodeClass objects. Make modifications as needed.

When you are satisfied with your NodePool and EC2NodeClass files, apply them as follows:

    ```bash
    kubectl apply -f nodepool.yaml
    kubectl apply -f ec2nodeclass.yaml
    ```

## Changelog

Because Karpenter `v1.0.0` will run both `v1` and `v1beta1` versions of NodePools and EC2NodeClasses, you don't immediately have to upgrade them to v1.
However, in preparation for later Karpenter upgrades (which will not support `v1beta1`, review the following changes from v1beta1 to v1.

Karpenter `v1.0.0` changes are divided into two different categories: those you must do before `1.0.0` upgrades and those you must do before `1.1.0` upgrades.

### Before upgrading to `1.0.0`

Apply the following changes to your NodePools and EC2NodeClasses, as appropriate, before upgrading them to v1.

* **Deprecated annotations, labels and tags are removed for v1.0.0**: For v1, `karpenter.sh/do-not-consolidate` (annotation), `karpenter.sh/do-not-evict
(annotation)`, and `karpenter.sh/managed-by` (tag) all have support removed.
The `karpenter.sh/managed-by`, which currently stores the cluster name in its value, is replaced by `eks:eks-cluster-name`, to allow
for [EKS Pod Identity ABAC policies](https://docs.aws.amazon.com/eks/latest/userguide/pod-id-abac.html).

* **Zap logging removed**: Support for setting the Zap logging config was deprecated in beta and is now removed for v1. View the [Logging Configuration Section of the v1beta1 Migration Guide]({{<ref "../../v0.32/upgrading/v1beta1-migration#logging-configuration-is-no-longer-dynamic" >}}) for more details.

* **metadataOptions could break workloads**: If you have workload pods that are not using `hostNetworking`, the updated default `metadataOptions` could cause your containers to break when you apply new EC2NodeClasses on v1.

* **Support for native Ubuntu AMI selection is dropped**: If you are using Ubuntu, be aware that Karpenter v1 no longer natively supports Ubuntu. To continue using Ubuntu in v1, you can update `amiSelectorTerms` in your EC2NodeClasses to identify Ubuntu as the AMI you want to use. See [reimplement amiFamily](https://github.com/aws/karpenter-provider-aws/pull/6569) for an example. Once you have done that, you can leave the amiFamily as Ubuntu and proceed to upgrading to v1. This will result in the following Transformation:
   ```yaml
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
     - deviceName: 'dev/sda1'
       rootVolume: true
       ebs:
         encrypted: true
         volumeType: gp3
         volumeSize: 20Gi
   ```
   **NOTE**: If you do not specify `amiSelectorTerms` before upgrading with an Ubuntu AMI, the conversion webhook will fail. If you do this, downgrade to Karpenter `v0.37.1 and add amiSelectorTerms before upgrading again.

* **Migrating userData**: Consider migrating your userData ahead of the v1.0.0 upgrade. See the description of `amiSelectorTerms` below.

### Before upgrading to `1.1.0`

Apply the following changes to your NodePools and EC2NodeClasses, as appropriate, before upgrading them to `v1.1.0` (though okay to make these changes for `1.0.0`)

* **v1beta1 support gone**: In `1.1.0`, v1beta1 is not supported. So you need to:
   * Migrate all Karpenter yaml files ([NodePools]({{<ref "../concepts/nodepools" >}}), [EC2NodeClasses]({{<ref "../concepts/nodeclasses" >}}), and so on) to v1.
   * Know that all resources in the cluster also need to be on v1. It's possible (although unlikely) that some resources still may be stored as v1beta1 in ETCD if no writes had been made to them since the v1 upgrade.  You could use a tool such as [kube-storage-version-migrator](https://github.com/kubernetes-sigs/kube-storage-version-migrator) to handle this.
   * Know that you cannot rollback to v1beta1 once you have upgraded to `v1.1.0`.

* **Remove kubelet annotation from NodePools**: Check that NodePools no longer contain the kubelet-configuration annotation (`compatibility.karpenter.sh/v1beta1-kubelet-conversion` annotation).
Karpenter will crash if NodePool resources contain this annotation.

* **Remove BootstrapMode annotation**: Karpenter will crash if NodePool resources contain the `BootstrapMode` annotation.
This annotation is no longer being added. If you are migrating an Ubuntu NodeClass to v1, you need to remove the `karpenter.k8s.aws/v1beta1-ubuntu` annotation.

* **KubeletConfiguration**: If you have multiple NodePools pointing to the same EC2NodeClass that have different kubeletConfigurations,
then you have to manually intervene to add more EC2NodeClasses and point their NodePools to them.
Otherwise, this will induce drift and you will have to roll your cluster.
If you have multiple NodePools pointing to the same EC2NodeClass, but they have the same configuration, then you can proceed with the migration
without having drift or having any additional NodePools or EC2NodeClasses configured.

* **amiSelectorTerms and amiFamily**: For v1, `amiFamily` is no longer required if you instead specify an `alias` in `amiSelectorTerms` in your `EC2NodeClass`. However, you need to update those settings if you are using:
   * A Custom amiFamily. You must ensure that the node is registered with the `karpenter.sh/unregistered` taint.
   * An Ubuntu AMI, as described earlier.

### Update metrics

Changes to Karpenter metrics from v1beta1 to v1 are shown in the following tables.

This table shows metrics names that changed from v1beta1 to v1:

| Metric type | v1beta1 metrics name | new v1 metrics name| 
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
