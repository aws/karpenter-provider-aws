# v1beta11

This document formalizes the [v1beta1 laundry list](https://github.com/aws/karpenter/issues/1327) into a release strategy and concrete set of proposed changes.

### Migration path from v1alpha5 to v1beta1

Customers will be able to migrate from v1alpha5 to v1beta1 in a single cluster using a single Karpenter version.

Kubernetes custom resources have built in support for API version compatibility. CRDs with multiple versions must define a “storage version”, which controls the data stored in etcd. Other versions are views onto this data and converted using [conversion webhooks](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#webhook-conversion). However, there is a fundamental limitation that all versions must be safely [round-tripable through each other](https://book.kubebuilder.io/multiversion-tutorial/api-changes.html). This means that it must be possible to define a function that converts a v1alpha5 Provisioner into a v1beta1 Provisioner and vise versa.

Unfortunately, multiple proposed changes in v1beta1 are not round-trippable. Below, we propose two deprecations of legacy fields in favor more modern mechanisms that have seen broad adoption in v1alpha5. These changes remove sharp edges that regularly cause customer surprise and production pain.

To work around this limitation, we have three options:

1. [Recommended] Rename Provisioner to something like NodeProvisioner, to avoid being subject to round-trippable requirements
2. Require that users delete the existing v1alpha5 Provisioner CRD and then install the v1beta1 Provisioner CRD. This will result in all capacity being shut down, and cannot be done in place if the customer has already launched nodes.
3. Keep the legacy fields in our API forever

Option 2 is untenable and easily discarded. We must provide a migration path for existing customers. Option 3 minimizes immediate impact to customers, but results in long term customer pain. There are other benefits to renaming Provisioner, so while it does cause some churn, it results in long term value.

Following option #1, customers would upgrade as follows:

1. Install Karpenter version 0.x.0, this version supports both v1alpha5.Provisioner and v1beta1.NodeProvisioner
    1. We may choose to support both APIs in more than one Karpenter version at the cost of developer velocity
2. Create a v1beta1.NodeProvisioner, and add a taint to the old v1alpha5.Provisioner
3. Depending on disruption tolerance, trigger rolling restarts to all deployments, delete the provisioner, or simply wait.

### Implementation Plan

The Karpenter codebase currently references v1alpha5.Provisioner in most of its controllers. While v1alpha5 and v1beta1 are not roundtrippable, we can restore the broken relationship between v1alpha5.Provisioner.spec.provider and v1beta1.AWSNodeTemplate in memory. This can be achieved by json encoding provisioner.spec.provider into an annotation karpenter.sh/compatibility/provider. Any area of code that needs to maintain compatibility will be required to parse out these values. Luckily, this is contained to the AWS cloud provider, as spec.provider predates prototyping efforts from other vendors.

Alternatively, we can choose to replicate existing v1alpha5 controllers into a new folder controllers/v1beta1/... where each set of versioned controllers operates independently on Provisioners of different API version. This will result in a much larger volume of code in the short term, but keep the complexity isolated. Developers will need to be aware of this duplication as long as it exists, and backport fixes to both places where necessary. Because of this, we will limit the number of Karpenter releases that support both versions and eventually deprecate support for v1alpha5.

Either approach impacts the velocity of the project — we will need to decide when is the best time to make these changes.

## Proposed Changes

### Rename: Provisioner → NodeProvisioner

This change avoids the roundtrippable problem mentioned above. Further, Provisioner is an overloaded term, colliding with storage provisioning, control plane provisioning, network provisioning. The name NodeProvisioner has nice alignment with NodeTemplate, and potentially in the future NodeGroup. NodeProvisioner and NodeGroup would each be responsible for managing a set of nodes, but with differing rules for when capacity is launched. They can be thought of as dynamic and static capacity.

Naming confusion:

* https://kubernetes.slack.com/archives/C02SFFZSA2K/p1669605517500229?thread_ts=1669289308.636989&cid=C02SFFZSA2K

### Rename: Move provisioner.spec to provisioner.spec.template

Currently fields that control node properties, such as Labels, Taints, StartupTaints, Requirements, Kubelet, ProviderRef, are top level members of provisioner.spec. These are siblings of fields that control provisioning behavior, such as Limits, TTLSecondsUntilExpired, and Weight.

Instead, we should separate out properties of the node in the same way that pod properties are separated from their parent controllers. This has a nice effect of simultaneously creating a place for users to define node metadata. This design mirrors that of kubernetes objects that contain podtemplatespecs, like Deployment, StatefulSet, Job, and CronJob.
```
spec:
  ttlSecondsUntilExpired: ...
  weight: ...
  limits: ...
  template:
    metadata:
      labels: ...
      annotations: ...
    spec:
      taints: ...
      startupTaints: ...
      requirements: ...
      kubelet: ...
      providerRef: ...
```

### Rename: kubeletConfiguration → kubelet

This is a minor change that favors a less verbose naming style, while conveying the same information to the user.

### Deprecate: provisioner.spec.provider

We’ve recommended that customers leverage spec.providerRef in favor of spec.provider since Q2 2022. Documentation for this feature has been removed since Q3 2022. We will take the opportunity to remove the feature entirely to minimize code bugs/complexity and user confusion.

### Deprecate: awsnodetemplate.spec.launchTemplate

Direct launch template support is problematic for many reasons, outlined in the [design](https://github.com/aws/karpenter/blob/main/designs/aws/aws-launch-templates-v2.md). Customers continue to run into issues when directly using launch templates. From many conversations with customers, our AWSNodeTemplate design has achieved feature parity with launch templates. The only gap is for users who maintain external workflows for launch template management. This requirement is in direct conflict with users who run into this sharp edge.

This change simply removes legacy support for launch templates in favor of the AWSNodeTemplate design.

### Require awsnodetemplate.instanceProfile

InstanceProfile, SubnetSelector, and SecurityGroup are all required information to launch nodes. Currently InstanceProfile is set in default settings, but subnetSelector and securityGroupSelector aren't. This is somewhat awkward, and doesn't provide a consistent experience for users.

Customer confusion:
* https://github.com/aws/karpenter/issues/2973

Options:
1. Make InstanceProfile required in all AWSNodeTemplates
2. [Recommended] Make SubnetSelector and SecurityGroupSelector able to be set in the global-settings configmap

The global-settings configmap is largely for enabling/disabling features and settings that apply globally and cannot be easily specified in one of the CRDs. Further, this reduces the configuration surface of where identities can be configured, reducing attack surface.
