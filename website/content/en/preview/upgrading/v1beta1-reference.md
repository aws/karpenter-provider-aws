---
title: "v1beta1 Upgrade Reference"
linkTitle: "v1beta1 Upgrade Reference"
weight: 30
description: >
  API information for upgrading Karpenter
---

Significant changes to the Karpenter APIs have been introduced in Karpenter v0.32.x.
In this release, Karpenter APIs have advanced to v1beta1, in preparation for Karpenter v1 in the near future.
The v1beta1 changes are meant to simplify and improve ease of use of those APIs, as well as solidify the APIs for the v1 release.
Use this document as a reference to the changes that were introduced in the current release and as a guide to how you need to update the manifests and other Karpenter objects you created in previous Karpenter releases.

The [Upgrade Guide]({{< relref "upgrade-guide" >}}) steps you through the process of upgrading Karpenter for the latest release.
For a more general understanding of Karpenter's compatibility, see the [Compatibility Document]({{< relref "compatibility" >}}).

## CRD Upgrades

Karpenter ships with a few Custom Resource Definitions (CRDs). Starting with v0.32.0, CRDs representing Provisioners, Machines, and AWS Node Templates are replaced by those for NodePools, NodeClaims, and EC2Nodeclasses, respectively.
You can find these CRDs by visiting the [Karpenter GitHub repository](https://github.com/aws/karpenter/tree{{< githubRelRef >}}pkg/apis/crds).

The [Upgrade Guide]({{< relref "upgrade-guide" >}}) describes how to install the new CRDs.

## Annotations, Labels, and Status Conditions

Karpenter v1beta1 introduces changes to some common labels, annotations, and status conditions that are present in the project. The tables below lists the v1alpha5 values and their v1beta1 equivalent.

| Karpenter Labels                |                             |
|---------------------------------|-----------------------------|
| **v1alpha5**                    | **v1beta1**                 |
| karpenter.sh/provisioner-name   | karpenter.sh/nodepool       |

> **Note**: Previously, you could use the `karpenter.sh/provisioner-name:DoesNotExist` requirement on pods to specify that pods should schedule to nodes unmanaged by Karpenter. With the addition of the `karpenter.sh/nodepool` label key, you now need to specify the `karpenter.sh/nodepool:DoesNotExist` requirement on these pods as well to ensure they don't schedule to nodes provisioned by the new NodePool resources.


| Karpenter Annotations               |                                  |
|-------------------------------------|----------------------------------|
| **v1alpha5**                        | **v1beta1**                      |
| karpenter.sh/provisioner-hash       | karpenter.sh/nodepool-hash       |
| karpenter.k8s.aws/nodetemplate-hash | karpenter.k8s.aws/nodeclass-hash |
| karpenter.sh/do-not-consolidate     | karpenter.sh/do-not-disrupt      |
| karpenter.sh/do-not-evict           | karpenter.sh/do-not-disrupt      |

> **Note**: Karpenter dropped the `karpenter.sh/do-not-consolidate` annotation in favor of the `karpenter.sh/do-not-disrupt` annotation on nodes. This annotation specifies that no voluntary disruption should be performed by Karpenter against this node.

| StatusCondition Types           |                |
|---------------------------------|----------------|
| **v1alpha5**                    | **v1beta1**    |
| MachineLaunched                 | Launched       |
| MachineRegistered               | Registered     |
| MachineInitialized              | Initialized    |
| MachineEmpty                    | Empty          |
| MachineExpired                  | Expired        |
| MachineDrifted                  | Drifted        |

## Provisioner to NodePool

Karpenter v1beta1 moves almost all top-level fields under the `NodePool` template field. Similar to Deployments (which template Pods that are orchestrated by the deployment controller), Karpenter NodePool templates NodeClaims (that are orchestrated by the Karpenter controller). Here is an example of a `Provisioner` (v1alpha5) migrated to a `NodePool` (v1beta1):

Note that:
* The `Limits` and `Weight` fields sit outside of the template section. The `Labels` and `Annotations` fields from the Provisioner are now under the `spec.template.metadata` section. All other fields including requirements, taints, kubelet, and so on, are specified under the `spec.template.spec` section.
* Support for `spec.template.spec.kubelet.containerRuntime` has been dropped. If you are using EKS 1.23 you should upgrade to containerd before using Karpenter v0.32.0, as this field in the kubelet block of the NodePool is not supported. EKS 1.24+ only supports containerd as a supported runtime.

**Provisioner example (v1alpha)**

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
 ...
spec:
  providerRef:
    name: default
  annotations:
    custom-annotation: custom-value
  labels:
    team: team-a
    custom-label: custom-value
  requirements:
  - key: karpenter.k8s.aws/instance-generation
    operator: Gt
    values: ["3"]
  - key: karpenter.k8s.aws/instance-category
    operator: In
    values: ["c", "m", "r"]
  - key: karpenter.sh/capacity-type
    operator: In
    values: ["spot"]
  taints:
  - key: example.com/special-taint
    value: "true"
    effect: NoSchedule
  startupTaints:
  - key: example.com/another-taint
    value: "true"
    effect: NoSchedule
  kubelet:
    systemReserved:
      cpu: 100m
      memory: 100Mi
      ephemeral-storage: 1Gi
    maxPods: 20
  limits:
    resources:
      cpu: 1000
      memory: 1000Gi
  weight: 50
```

**NodePool example (v1beta1)**

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
...
spec:
  template:
    metadata:
      annotations:
        custom-annotation: custom-value
      labels:
        team: team-a
        custom-label: custom-value
    spec:
      requirements:
      - key: karpenter.k8s.aws/instance-generation
        operator: Gt
        values: ["3"]
      - key: karpenter.k8s.aws/instance-category
        operator: In
        values: ["c", "m", "r"]
      - key: karpenter.sh/capacity-type
        operator: In
        values: ["spot"]
      taints:
      - key: example.com/special-taint
        value: "true"
        effect: NoSchedule
      startupTaints:
      - key: example.com/another-taint
        value: "true"
        effect: NoSchedule
      kubelet:
        systemReserved:
          cpu: 100m
          memory: 100Mi
          ephemeral-storage: 1Gi
        maxPods: 20
  limits:
    cpu: 1000
    memory: 1000Gi
  weight: 50
```

### Provider

The Karpenter `spec.provider` field has been deprecated since version v0.7.0 and is now removed in the new `NodePool` resource. Any of the fields that you could specify within the `spec.provider` field are now available in the separate `NodeClass` resource.

**Provider example (v1alpha)**

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
...
spec:
  provider:
    amiFamily: Bottlerocket
    tags:
      test-tag: test-value  
```

**Nodepool example (v1beta1)**

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
...
nodeClassRef:
  name: default
```

**EC2NodeClass example (v1beta1)**

```yaml
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
metadata:
  name: default
spec:
  amiFamily: Bottlerocket
  tags:
    test-tag: test-value
```

### TTLSecondsAfterEmpty

The Karpenter `spec.ttlSecondsAfterEmpty` field has been removed in favor of a `consolidationPolicy` and `consolidateAfter` field. 

As part of the v1beta1 migration, Karpenter has chosen to collapse the concepts of emptiness and underutilization into a single concept: `consolidation`.
You can now define the types of consolidation that you want to support in your `consolidationPolicy` field.
The current values for this field are `WhenEmpty` or `WhenUnderutilized` (defaulting to `WhenUnderutilized` if not specified).
If specifying `WhenEmpty`, you can define how long you wish to wait for consolidation to act on your empty nodes by tuning the `consolidateAfter` parameter.
This field works the same as the `ttlSecondsAfterEmpty` field except this field now accepts either of the following values:

* `Never`: This disables empty consolidation.
* Duration String (e.g. “10m”, “1s”): This enables empty consolidation for the time specified.

**ttlSecondsAfterEmpty example (v1alpha)**

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
...
spec:
  ttlSecondsAfterEmpty: 120
```

**consolidationPolicy and consolidateAfter examples (v1beta1)**

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
...
spec:
  disruption:
    consolidationPolicy: WhenEmpty
    consolidateAfter: 2m

```

### Consolidation

The Karpenter `spec.consolidation` block has also been shifted under `consolidationPolicy`. If you were previously enabling Karpenter’s consolidation feature for underutilized nodes using the `consolidation.enabled` flag, you now enable consolidation through the `consolidationPolicy`.

**consolidation enabled example (v1alpha)**

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
...
spec:
  consolidation:
    enabled: true
```

**consolidationPolicy WhenUnderutilized example (v1beta1)**

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
...
spec:
  disruption:
    consolidationPolicy: WhenUnderutilized
```

> Note: You currently can’t set the `consolidateAfter` field when specifying `consolidationPolicy: WhenUnderutilized`. Karpenter will use a 15s `consolidateAfter` runtime default.

### TTLSecondsUntilExpired

The Karpenter `spec.ttlSecondsUntilExpired` field has been removed in favor of the `expireAfter` field inside of the `disruption` block. This field works the same as it did before except this field now accepts either of the following values:

* `Never`: This disables expiration.
* Duration String (e.g. “10m”, “1s”): This enables expiration for the time specified.

**consolidation ttlSecondsUntilExpired example (v1alpha)**

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
...
spec:
  ttlSecondsUntilExpired: 2592000 # 30 Days = 60 * 60 * 24 * 30 Seconds
```

**consolidationPolicy WhenUnderutilized example (v1beta1)**

```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
...
spec:
  disruption:
    expireAfter: 720h # 30 days = 30 * 24 Hours

```

### Defaults

Karpenter now statically defaults some fields in the v1beta1 if they are not specified when applying the `NodePool` configuration. The following fields are defaulted if unspecified.

| Field                                | Default                                                          |
|--------------------------------------|------------------------------------------------------------------|
| spec.disruption                      | {"consolidationPolicy: WhenUnderutilized", expireAfter: "720h"}  |
| spec.disruption.consolidationPolicy  | WhenUnderutilized                                                |
| spec.disruption.expireAfter          | 720h                                                             |


### spec.template.spec.requirements Defaults Dropped

Karpenter v1beta1 drops the defaulting logic for the node requirements that were shipped by default with Provisioners in v1alpha5. Previously, Karpenter would create dynamic defaulting in the following cases. If multiple of these cases were satisfied, those default requirements would be combined:

* If you didn't specify any instance type requirement:

   ```yaml
   spec:
     requirements:
     - key: karpenter.k8s.aws/instance-category
       operator: In
       values: ["c", "m", "r"]
     - key: karpenter.k8s.aws/instance-generation
       operator: In
       values: ["2"]
   ```

* If you didn’t specify any capacity type requirement (`karpenter.sh/capacity-type`):

   ```yaml
   spec:
     requirements:
     - key: karpenter.sh/capacity-type
       operator: In
       values: ["on-demand"]
   ```

* If you didn’t specify any OS requirement (`kubernetes.io/os`):
   ```yaml
   spec:
     requirements:
     - key: kubernetes.io/os
       operator: In
       values: ["linux"]
   ```

* If you didn’t specify any architecture requirement (`kubernetes.io/arch`):
   ```yaml
   spec:
     requirements:
     - key: kubernetes.io/arch
       operator: In
       values: ["amd64"]
   ```

If you were previously relying on this defaulting logic, you will now need to explicitly specify these requirements in your `NodePool`.

## AWSNodeTemplate to EC2NodeClass

To configure AWS-specific settings, AWSNodeTemplate (v1alpha) is being changed to EC2NodeClass (v1beta1). Below are ways in which you can update your manifests for the new version.

{{% alert title="Note" color="warning" %}}
Tag-based [AMI Selection](https://karpenter.sh/v0.31/concepts/node-templates/#ami-selection) is no longer supported for the new v1beta1 `EC2NodeClass` API. That feature allowed users to tag their AMIs using EC2 tags to express “In” requirements on selected. We recommend using different NodePools with different EC2NodeClasses with your various AMI requirement constraints to appropriately constrain your AMIs based on the instance types you’ve selected for a given NodePool.
{{% /alert %}}

{{% alert title="Note" color="primary" %}} 
Karpenter will now tag EC2 instances with their node name instead of with `karpenter.sh/provisioner-name: $PROVISIONER_NAME`. This makes it easier to map nodes to instances if you are not currently using [resource-based naming](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-naming.html).
{{% /alert %}}


### InstanceProfile

The Karpenter `spec.instanceProfile` field has been removed from the EC2NodeClass in favor of the `spec.role` field. Karpenter is also removing support for the `defaultInstanceProfile` specified globally in the karpenter-global-settings, making the `spec.role` field required for all EC2NodeClasses.

Karpenter will now auto-generate the instance profile in your EC2NodeClass, given the role that you specify. To find the role, type:

```bash
export INSTANCE_PROFILE_NAME=KarpenterNodeInstanceProfile-bob-karpenter-demo
aws iam get-instance-profile --instance-profile-name $INSTANCE_PROFILE_NAME --query "InstanceProfile.Roles[0].RoleName"
KarpenterNodeRole-bob-karpenter-demo
```

**instanceProfile example (v1alpha)**

```yaml
apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
...
spec:
  instanceProfile: KarpenterNodeInstanceProfile-karpenter-demo 
```

**role example (v1beta1)**

```yaml
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
...
spec:
  role: KarpenterNodeRole-karpenter-demo
```

### SubnetSelector, SecurityGroupSelector, and AMISelector

Karpenter’s `spec.subnetSelector`, `spec.securityGroupSelector`, and `spec.amiSelector` fields have been modified to support multiple terms and to first-class keys like id and name. If using comma-delimited strings in your `tag`, `id`, or `name` values, you may need to create separate terms for the new fields.


**subnetSelector example (v1alpha)**

```yaml
apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
...
spec:
  subnetSelector:
    karpenter.sh/discovery: karpenter-demo
```

**SubnetSelectorTerms.tags example (v1beta1)**

```yaml
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
...
spec:
  subnetSelectorTerms:
  - tags:
      karpenter.sh/discovery: karpenter-demo
```

**subnetSelector example (v1alpha)**

```yaml
apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
...
spec:
  subnetSelector:
    aws::ids: subnet-123,subnet-456
```

**subnetSelectorTerms.id example (v1beta1)**

```yaml
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
...
spec:
  subnetSelectorTerms:
  - id: subnet-123
  - id: subnet-456
```

**securityGroupSelector example (v1alpha)**

```yaml
apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
...
spec:
  securityGroupSelector:
    karpenter.sh/discovery: karpenter-demo
```

**securityGroupSelectorTerms.tags example (v1beta1)**

```yaml
apiVersion: compute.k8s.aws/v1beta1
kind: EC2NodeClass
...
spec:
  securityGroupSelectorTerms:
  - tags:
      karpenter.sh/discovery: karpenter-demo
```

**securityGroupSelector example (v1alpha)**

```yaml
apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
...
spec:
  securityGroupSelector:
    aws::ids: sg-123, sg-456
```


**securityGroupSelectorTerms.id example (v1beta1)**

```yaml
apiVersion: compute.k8s.aws/v1beta1
kind: EC2NodeClass
...
spec:
  securityGroupSelectorTerms:
  - id: sg-123
  - id: sg-456
```


**amiSelector example (v1alpha)**

```yaml
apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
...
spec:
  amiSelector:
    karpenter.sh/discovery: karpenter-demo
```


**amiSelectorTerms.tags example (v1beta1)**

```yaml
apiVersion: compute.k8s.aws/v1beta1
kind: EC2NodeClass
...
spec:
  amiSelectorTerms:
  - tags:
      karpenter.sh/discovery: karpenter-demo
```

**amiSelector example (v1alpha)**

```yaml
apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
...
spec:
  amiSelector:
    aws::ids: ami-123,ami-456
```

**amiSelectorTerms example (v1beta1)**

```yaml
apiVersion: compute.k8s.aws/v1beta1
kind: EC2NodeClass
...
spec:
  amiSelectorTerms:
  - id: ami-123
  - id: ami-456
```


**amiSelector example (v1alpha)**

```yaml
apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
...
spec:
  amiSelector:
    aws::name: my-name1,my-name2
    aws::owners: 123456789,amazon
```

**amiSelectorTerms.name example (v1beta1)**

```yaml
apiVersion: compute.k8s.aws/v1beta1
kind: EC2NodeClass
...
spec:
  amiSelectorTerms:
  - name: my-name1
    owner: 123456789
  - name: my-name2
    owner: 123456789
  - name: my-name1
    owner: amazon
  - name: my-name2
    owner: amazon
```

### LaunchTemplateName

The `spec.launchTemplateName` field for referencing unmanaged launch templates within Karpenter has been removed. 
Find a discussion of the decision to remove `spec.launchTemplateName`, see [RFC: Unmanaged LaunchTemplate Removal](https://github.com/aws/karpenter/blob/main/designs/unmanaged-launch-template-removal.md).

### AMIFamily

The AMIFamily field is now required. If you were previously not specifying the AMIFamily field, having Karpenter default the AMIFamily to AL2, you will now have to specify AL2 explicitly.

## Metrics

The following table shows v1alpha5 metrics and the v1beta1 version of each metric. All metrics on this table will exist simultaneously, while both v1alpha5 and v1beta1 are supported within the same version.

| **v1alpha5 Metric Name**                                              | **v1beta1 Metric Name**                                         |
|-----------------------------------------------------------------------|-----------------------------------------------------------------|
| karpenter_machines_created                                            | karpenter_nodeclaims_created                                    |
| karpenter_machines_disrupted                                          | karpenter_nodeclaims_disrupted                                  |
| karpenter_machines_drifted                                            | karpenter_nodeclaims_drifted                                    |
| karpenter_machines_initialized                                        | karpenter_nodeclaims_initialized                                |
| karpenter_machines_launched                                           | karpenter_nodeclaims_launched                                   |
| karpenter_machines_registered                                         | karpenter_nodeclaims_registered                                 |
| karpenter_machines_terminated                                         | karpenter_nodeclaims_terminated                                 |
| karpenter_provisioners_limit                                          | karpenter_nodepools_limit                                       |
| karpenter_provisioners_usage                                          | karpenter_nodepools_usage                                       |
| karpenter_deprovisioning_evaluation_duration_seconds                  | karpenter_disruption_evaluation_duration_seconds                |
| karpenter_deprovisioning_eligible_machines                            | karpenter_disruption_eligible_nodeclaims                        |
| karpenter_deprovisioning_replacement_machine_initialized_seconds      | karpenter_disruption_replacement_nodeclaims_initialized_seconds |
| karpenter_deprovisioning_replacement_machine_launch_failure_counter   | karpenter_disruption_replacement_nodeclaims_launch_failed_total |
| karpenter_deprovisioning_actions_performed                            | karpenter_disruption_actions_performed_total                    |
| karpenter_deprovisioning_consolidation_timeouts                       | karpenter_disruption_consolidation_timeouts_total               |
| karpenter_nodes_leases_deleted                                        | karpenter_leases_deleted                                        |
| karpenter_provisioners_usage_pct                                      | **Dropped**                                                     |

In addition to these metrics, the MachineNotFound error returned by the `karpenter_cloudprovider_errors_total` values in the error label has been changed to `NodeClaimNotFound`. This is agnostic to the version of the API (Machine or NodeClaim) that actually owns the instance.

## Global Settings

The v1beta1 specification removes the `karpenter-global-settings` ConfigMap in favor of setting all Karpenter configuration using environment variables. Along, with this change, Karpenter has chosen to remove certain global variables that can be configured with more specificity in the EC2NodeClass . These values are marked as removed below.


| **`karpenter-global-settings` ConfigMap Key** | **Environment Variable**   | **CLI Argument**             |
|-----------------------------------------------|----------------------------|------------------------------|
| batchMaxDuration                              | BATCH_MAX_DURATION         | --batch-max-duration         |
| batchIdleDuration                             | BATCH_IDLE_DURATION        | --batch-idle-duration        |
| assumeRoleARN                                 | ASSUME_ROLE_ARN            | --assume-role-arn            |
| assumeRoleDuration                            | ASSUME_ROLE_DURATION       | --assume-role-duration       |
| clusterCABundle                               | CLUSTER_CA_BUNDLE          | --cluster-ca-bundle          |
| clusterName                                   | CLUSTER_NAME               | --cluster-name               |
| clusterEndpoint                               | CLUSTER_ENDPOINT           | --cluster-endpoint           |
| isolatedVPC                                   | ISOLATED_VPC               | --isolated-vpc               |
| vmMemoryOverheadPercent                       | VM_MEMORY_OVERHEAD_PERCENT | --vm-memory-overhead-percent |
| interruptionQueueName                         | INTERRUPTION_QUEUE         | --interruption-queue         |
| reservedENIs                                  | RESERVED_ENIS              | --reserved-enis              |
| featureGates.driftEnabled                     | FEATURE_GATE="Drift=true"  | --feature-gates Drift=true   |
| defaultInstanceProfile                        | **Dropped**                | **Dropped**                  |
| enablePodENI                                  | **Dropped**                | **Dropped**                  |
| enableENILimitedPodDensity                    | **Dropped**                | **Dropped**                  |

## Helm Values

The v1beta1 helm chart comes with a number of changes to the values that were previously used in v0.31.x. Your older helm values will continue to work throughout v0.32.x but any values no longer specified in the chart will no longer be supported starting in v0.33.0.

| < v0.32.x Key                           | >= v0.32.x Key                   |
|-----------------------------------------|----------------------------------|
| controller.outputPaths                  | logConfig.outputPaths            |
| controller.errorOutputPaths             | logConfig.errorOutputPaths       |
| controller.logLevel                     | logConfig.logLevel.controller    |
| webhook.logLevel                        | logConfig.logLevel.webhook       |
| logEncoding                             | logConfig.logEncoding            |
| settings.aws.assumeRoleARN              | settings.assumeRoleARN           |
| settings.aws.assumeRoleDuration         | settings.assumeRoleDuration      |
| settings.aws.clusterCABundle            | settings.clusterCABundle         |
| settings.aws.clusterName                | settings.clusterName             |
| settings.aws.clusterEndpoint            | settings.clusterEndpoint         |
| settings.aws.isolatedVPC                | settings.isolatedVPC             |
| settings.aws.vmMemoryOverheadPercent    | settings.vmMemoryOverheadPercent |
| settings.aws.interruptionQueueName      | settings.interruptionQueue       |
| settings.aws.reservedENIs               | settings.reservedENIs            |
| settings.featureGates.driftEnabled      | settings.featureGates.drift      |
| settings.aws.defaultInstanceProfile     | **Dropped**                      |
| settings.aws.enablePodENI               | **Dropped**                      |
| settings.aws.enableENILimitedPodDensity | **Dropped**                      |
| settings.aws.tags                       | **Dropped**                      |

## Drift Enabled by Default

The drift feature will now be enabled by default starting from v0.33.0. If you don’t specify the Drift featureGate, the feature will be assumed to be enabled. You can disable the drift feature by specifying --feature-gates Drift=false. This feature gate is expected to be dropped when core APIs (NodePool, NodeClaim) are bumped to v1.

## Logging Configuration is No Longer Dynamic

As part of this deprecation, Karpenter will no longer call out to the APIServer to discover the ConfigMap. Instead, Karpenter will expect the ConfigMap to be mounted on the filesystem at `/etc/karpenter/logging/zap-logger-config`. You can also still choose to override the individual log level of components of the system (webhook and controller) at the paths `/etc/karpenter/logging/loglevel.webhook` and `/etc/karpenter/logging/loglevel.controller`. 

What you do to upgrade this feature depends on how you install Karpenter:

* If you are using the helm chart to install Karpenter, you won’t need to make any changes for Karpenter to begin using this new mechanism for loading the config.

* If you are manually configuring the deployment for Karpenter, you will need to add the following sections to your deployment:

   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   spec:
     template:
       spec:
       ...
         containers:
         - name: controller
           volumeMounts:
           - name: config-logging
             mountPath: /etc/karpenter/logging
         volumes:
         - name: config-logging
           configMap:
             name: config-logging
   ```

Karpenter will drop support for ConfigMap discovery through the APIServer starting in v0.33.0, meaning that you will need to ensure that you are mounting the config file on the expected filepath by that version.

## Webhook Support Deprecated in Favor of CEL

Karpenter v1beta1 APIs now support Common Expression Language (CEL) for validaiton directly through the APIServer. This change means that Karpenter’s validating webhooks are no longer needed to ensure that Karpenter’s NodePools and EC2NodeClasses are configured correctly. 

As a result, Karpenter will now disable webhooks by default by setting the `DISABLE_WEBHOOK` environment variable to `true` starting in v0.33.0. If you are currently on a version of Kubernetes < less than 1.25, CEL validation for Custom Resources is not enabled. We recommend that you enable the webhooks on these versions with `DISABLE_WEBHOOK=false` to get proper validation support for any Karpenter configuration.