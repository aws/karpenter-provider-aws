
---
title: "Getting Started with Karpenter"
linkTitle: "Getting Started with Karpenter"
weight: 10
description: >
  Set up a cluster and add Karpenter
---

Karpenter automatically provisions new nodes in response to unschedulable pods. Karpenter does this by observing events within the Kubernetes cluster, and then sending commands to the underlying cloud provider.

This guide shows how to get started with Karpenter by creating a Kubernetes cluster and installing Karpenter.
To use Karpenter, you must be running a supported Kubernetes cluster on a supported cloud provider.

The guide below explains how to utilize the [Karpenter provider for AWS](https://github.com/aws/karpenter-provider-aws) with EKS.

See the [AKS Node autoprovisioning article](https://learn.microsoft.com/azure/aks/node-autoprovision) on how to use Karpenter on Azure's AKS or go to the [Karpenter provider for Azure open source repository](https://github.com/Azure/karpenter-provider-azure) for self-hosting on Azure and additional information.

## Create a cluster and add Karpenter

This guide uses `eksctl` to create the cluster.
It should take less than 1 hour to complete, and cost less than $0.25.
Follow the clean-up instructions to reduce any charges.

### 1. Install utilities

Karpenter is installed in clusters with a Helm chart.

Karpenter requires cloud provider permissions to provision nodes, for AWS IAM
Roles for Service Accounts (IRSA) should be used. IRSA permits Karpenter
(within the cluster) to make privileged requests to AWS (as the cloud provider)
via a ServiceAccount.

Install these tools before proceeding:

1. [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2-linux.html)
2. `kubectl` - [the Kubernetes CLI](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/)
3. `eksctl` (>= v0.191.0) - [the CLI for AWS EKS](https://eksctl.io/installation)
4. `helm` - [the package manager for Kubernetes](https://helm.sh/docs/intro/install/)

[Configure the AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html)
with a user that has sufficient privileges to create an EKS cluster. Verify that the CLI can
authenticate properly by running `aws sts get-caller-identity`.

### 2. Set environment variables

After setting up the tools, set the Karpenter and Kubernetes version:

```bash
export KARPENTER_NAMESPACE="kube-system"
export KARPENTER_VERSION="1.0.10"
export K8S_VERSION="1.31"
```

Then set the following environment variable:

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-karpenter/scripts/step01-config.sh" language="bash"%}}

{{% alert title="Warning" color="warning" %}}
If you open a new shell to run steps in this procedure, you need to set some or all of the environment variables again.
To remind yourself of these values, type:

```bash
echo "${KARPENTER_NAMESPACE}" "${KARPENTER_VERSION}" "${K8S_VERSION}" "${CLUSTER_NAME}" "${AWS_DEFAULT_REGION}" "${AWS_ACCOUNT_ID}" "${TEMPOUT}" "${ARM_AMI_ID}" "${AMD_AMI_ID}" "${GPU_AMI_ID}"
```

{{% /alert %}}


### 3. Create a Cluster

Create a basic cluster with `eksctl`.
The following cluster configuration will:

* Use CloudFormation to set up the infrastructure needed by the EKS cluster. See [CloudFormation]({{< relref "../../reference/cloudformation/" >}}) for a complete description of what `cloudformation.yaml` does for Karpenter.
* Create a Kubernetes service account and AWS IAM Role, and associate them using IRSA to let Karpenter launch instances.
* Add the Karpenter node role to the aws-auth configmap to allow nodes to connect.
* Use [AWS EKS managed node groups](https://docs.aws.amazon.com/eks/latest/userguide/managed-node-groups.html) for the kube-system and karpenter namespaces. Uncomment fargateProfiles settings (and comment out managedNodeGroups settings) to use Fargate for both namespaces instead.
* Set KARPENTER_IAM_ROLE_ARN variables.
* Create a role to allow spot instances.
* Run Helm to install Karpenter

{{< tabpane text=true right=false >}}
  {{% tab header="**Create cluster command**:" disabled=true /%}}
  {{% tab header="Managed NodeGroups" %}}
  {{% script file="./content/en/{VERSION}/getting-started/getting-started-with-karpenter/scripts/step02-create-cluster.sh" language="bash"%}}
  {{% /tab %}}
  {{% tab header="Fargate" %}}
  {{% script file="./content/en/{VERSION}/getting-started/getting-started-with-karpenter/scripts/step02-create-cluster-fargate.sh" language="bash"%}}
  {{% /tab %}}
{{< /tabpane >}}

Unless your AWS account has already onboarded to EC2 Spot, you will need to create the service linked role to
avoid the [`ServiceLinkedRoleCreationNotPermitted` error]({{<ref "../../troubleshooting/#missing-service-linked-role" >}}).

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-karpenter/scripts/step06-add-spot-role.sh" language="bash"%}}

{{% alert title="Windows Support Notice" color="warning" %}}
In order to run Windows workloads, Windows support should be enabled in your EKS Cluster.
See [Enabling Windows support](https://docs.aws.amazon.com/eks/latest/userguide/windows-support.html#enable-windows-support) to learn more.
{{% /alert %}}

### 4. Install Karpenter

{{< tabpane text=true right=false >}}
  {{% tab header="**Karpenter installation command**:" disabled=true /%}}
  {{% tab header="Managed NodeGroups" %}}
  {{% script file="./content/en/{VERSION}/getting-started/getting-started-with-karpenter/scripts/step08-apply-helm-chart.sh" language="bash"%}}
  {{% /tab %}}
  {{% tab header="Fargate" %}}
  {{% script file="./content/en/{VERSION}/getting-started/getting-started-with-karpenter/scripts/step08-apply-helm-chart-fargate.sh" language="bash"%}}
  {{% /tab %}}
{{< /tabpane >}}

As the OCI Helm chart is signed by [Cosign](https://github.com/sigstore/cosign) as part of the release process you can verify the chart before installing it by running the following command.

```bash
cosign verify public.ecr.aws/karpenter/karpenter:1.0.10 \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
  --certificate-identity-regexp='https://github\.com/aws/karpenter-provider-aws/\.github/workflows/release\.yaml@.+' \
  --certificate-github-workflow-repository=aws/karpenter-provider-aws \
  --certificate-github-workflow-name=Release \
  --certificate-github-workflow-ref=refs/tags/v1.0.10 \
  --annotations version=1.0.10
```

{{% alert title="DNS Policy Notice" color="warning" %}}
Karpenter uses the `ClusterFirst` pod DNS policy by default. This is the Kubernetes cluster default and this ensures that Karpenter can reach-out to internal Kubernetes services during its lifetime. There may be cases where you do not have the DNS service that you are using on your cluster up-and-running before Karpenter starts up. The most common case of this is you want Karpenter to manage the node capacity where your DNS service pods are running.

If you need Karpenter to manage the DNS service pods' capacity, this means that DNS won't be running when Karpenter starts-up. In this case, you will need to set the pod DNS policy to `Default` with `--set dnsPolicy=Default`. This will tell Karpenter to use the host's DNS resolution instead of the internal DNS resolution, ensuring that you don't have a dependency on the DNS service pods to run. More details on this issue can be found in the following Github issues: [#2186](https://github.com/aws/karpenter-provider-aws/issues/2186) and [#4947](https://github.com/aws/karpenter-provider-aws/issues/4947).
{{% /alert %}}

{{% alert title="Common Expression Language/Webhooks Notice" color="warning" %}}
Karpenter supports using [Kubernetes Common Expression Language](https://kubernetes.io/docs/reference/using-api/cel/) for validating its Custom Resource Definitions out-of-the-box; however, this feature is not supported on versions of Kubernetes < 1.25. If you are running an earlier version of Kubernetes, you will need to use the Karpenter admission webhooks for validation instead. You can enable these webhooks with `--set webhook.enabled=true` when applying the Karpenter Helm chart.
{{% /alert %}}

{{% alert title="Pod Identity Supports Notice" color="warning" %}}
Karpenter now supports using [Pod Identity](https://docs.aws.amazon.com/eks/latest/userguide/pod-identities.html) to authenticate AWS SDK to make API requests to AWS services using AWS Identity and Access Management (IAM) permissions. This feature not supported on versions of Kubernetes < 1.24.  If you are running an earlier version of Kubernetes, you will need to use the [IAM Roles for Service Accounts(IRSA)](https://docs.aws.amazon.com/emr/latest/EMR-on-EKS-DevelopmentGuide/setting-up-enable-IAM.html) for pod authentication instead. You can enable these IRSA with `--set "serviceAccount.annotations.eks\.amazonaws\.com/role-arn=${KARPENTER_IAM_ROLE_ARN}"` when applying the Karpenter Helm chart.
{{% /alert %}}

{{% alert title="Warning" color="warning" %}}
Karpenter creates a mapping between CloudProvider machines and CustomResources in the cluster for capacity tracking. To ensure this mapping is consistent, Karpenter utilizes the following tag keys:

* `karpenter.sh/managed-by`
* `karpenter.sh/nodepool`
* `kubernetes.io/cluster/${CLUSTER_NAME}`

Because Karpenter takes this dependency, any user that has the ability to Create/Delete these tags on CloudProvider machines will have the ability to orchestrate Karpenter to Create/Delete CloudProvider machines as a side effect. We recommend that you [enforce tag-based IAM policies](https://docs.aws.amazon.com/IAM/latest/UserGuide/access_tags.html) on these tags against any EC2 instance resource (`i-*`) for any users that might have [CreateTags](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateTags.html)/[DeleteTags](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DeleteTags.html) permissions but should not have [RunInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RunInstances.html)/[TerminateInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_TerminateInstances.html) permissions.
{{% /alert %}}

### 5. Create NodePool

A single Karpenter NodePool is capable of handling many different pod shapes. Karpenter makes scheduling and provisioning decisions based on pod attributes such as labels and affinity. In other words, Karpenter eliminates the need to manage many different node groups.

Create a default NodePool using the command below. This NodePool uses `securityGroupSelectorTerms` and `subnetSelectorTerms` to discover resources used to launch nodes. We applied the tag `karpenter.sh/discovery` in the `eksctl` command above. Depending on how these resources are shared between clusters, you may need to use different tagging schemes.

The `consolidationPolicy` set to `WhenEmptyOrUnderutilized` in the `disruption` block configures Karpenter to reduce cluster cost by removing and replacing nodes. As a result, consolidation will terminate any empty nodes on the cluster. This behavior can be disabled by setting `consolidateAfter` to `Never`, telling Karpenter that it should never consolidate nodes. Review the [NodePool API docs]({{<ref "../../concepts/nodepools" >}}) for more information.

Note: This NodePool will create capacity as long as the sum of all created capacity is less than the specified limit.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-karpenter/scripts/step12-add-nodepool.sh" language="bash"%}}

Karpenter is now active and ready to begin provisioning nodes.

### 6. Scale up deployment

This deployment uses the [pause image](https://www.ianlewis.org/en/almighty-pause-container) and starts with zero replicas.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-karpenter/scripts/step13-automatic-node-provisioning.sh" language="bash"%}}

### 7. Scale down deployment

Now, delete the deployment. After a short amount of time, Karpenter should terminate the empty nodes due to consolidation.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-karpenter/scripts/step14-deprovisioning.sh" language="bash"%}}

### 8. Delete Karpenter nodes manually

If you delete a node with kubectl, Karpenter will gracefully cordon, drain,
and shutdown the corresponding instance. Under the hood, Karpenter adds a
finalizer to the node object, which blocks deletion until all pods are
drained and the instance is terminated. Keep in mind, this only works for
nodes provisioned by Karpenter.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-karpenter/scripts/step16-delete-node.sh" language="bash"%}}

### 9. Delete the cluster
To avoid additional charges, remove the demo infrastructure from your AWS account.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-karpenter/scripts/step17-cleanup.sh" language="bash"%}}

## Monitoring with Grafana (optional)

This section describes optional ways to configure Karpenter to enhance its capabilities.
In particular, the following commands deploy a Prometheus and Grafana stack that is suitable for this guide but does not include persistent storage or other configurations that would be necessary for monitoring a production deployment of Karpenter.
This deployment includes two Karpenter dashboards that are automatically onboarded to Grafana. They provide a variety of visualization examples on Karpenter metrics.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-karpenter/scripts/step09-add-prometheus-grafana.sh" language="bash"%}}

The Grafana instance may be accessed using port forwarding.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-karpenter/scripts/step10-add-grafana-port-forward.sh" language="bash"%}}

The new stack has only one user, `admin`, and the password is stored in a secret. The following command will retrieve the password.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-karpenter/scripts/step11-grafana-get-password.sh" language="bash"%}}

## Advanced Installation

The section below covers advanced installation techniques for installing Karpenter. This includes things such as running Karpenter on a cluster without public internet access or ensuring that Karpenter avoids getting throttled by other components in your cluster.

### Private Clusters

You can optionally install Karpenter on a [private cluster](https://docs.aws.amazon.com/eks/latest/userguide/private-clusters.html#private-cluster-requirements) using the `eksctl` installation by setting `privateCluster.enabled` to true in your [ClusterConfig](https://eksctl.io/usage/eks-private-cluster/#eks-fully-private-cluster) and by setting `--set settings.isolatedVPC=true` when installing the `karpenter` Helm chart.

```bash
privateCluster:
  enabled: true
```

Private clusters have no outbound access to the internet. This means that in order for Karpenter to reach out to the services that it needs to access, you need to enable specific VPC private endpoints. Below shows the endpoints that you need to enable to successfully run Karpenter in a private cluster:

```text
com.amazonaws.<region>.ec2
com.amazonaws.<region>.ecr.api
com.amazonaws.<region>.ecr.dkr
com.amazonaws.<region>.s3 – For pulling container images
com.amazonaws.<region>.sts – For IAM roles for service accounts
com.amazonaws.<region>.ssm - For resolving default AMIs
com.amazonaws.<region>.sqs - For accessing SQS if using interruption handling
com.amazonaws.<region>.eks - For Karpenter to discover the cluster endpoint
```

If you do not currently have these endpoints surfaced in your VPC, you can add the endpoints by running

```bash
aws ec2 create-vpc-endpoint --vpc-id ${VPC_ID} --service-name ${SERVICE_NAME} --vpc-endpoint-type Interface --subnet-ids ${SUBNET_IDS} --security-group-ids ${SECURITY_GROUP_IDS}
```

{{% alert title="Note" color="primary" %}}

Karpenter (controller and webhook deployment) container images must be in or copied to Amazon ECR private or to another private registry accessible from inside the VPC. If these are not available from within the VPC, or from networks peered with the VPC, you will get Image pull errors when Kubernetes tries to pull these images from ECR public.

{{% /alert %}}

{{% alert title="Note" color="primary" %}}

There is currently no VPC private endpoint for the [IAM API](https://docs.aws.amazon.com/IAM/latest/APIReference/welcome.html). As a result, you cannot use the default `spec.role` field in your `EC2NodeClass`. Instead, you need to provision and manage an instance profile manually and then specify Karpenter to use this instance profile through the `spec.instanceProfile` field.

You can provision an instance profile manually and assign a Node role to it by calling the following command

```bash
aws iam create-instance-profile --instance-profile-name "KarpenterNodeInstanceProfile-${CLUSTER_NAME}"
aws iam add-role-to-instance-profile --instance-profile-name "KarpenterNodeInstanceProfile-${CLUSTER_NAME}" --role-name "KarpenterNodeRole-${CLUSTER_NAME}"
```

{{% /alert %}}

{{% alert title="Note" color="primary" %}}

There is currently no VPC private endpoint for the [Price List Query API](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/using-price-list-query-api.html). As a result, pricing data can go stale over time. By default, Karpenter ships a static price list that is updated when each binary is released.

Failed requests for pricing data will result in the following error messages

```bash
ERROR   controller.aws.pricing  updating on-demand pricing, RequestError: send request failed
caused by: Post "https://api.pricing.us-east-1.amazonaws.com/": dial tcp 52.94.231.236:443: i/o timeout; RequestError: send request failed
caused by: Post "https://api.pricing.us-east-1.amazonaws.com/": dial tcp 52.94.231.236:443: i/o timeout, using existing pricing data from 2022-08-17T00:19:52Z  {"commit": "4b5f953"}
```

{{% /alert %}}

### Preventing APIServer Request Throttling

Kubernetes uses [FlowSchemas](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/#flowschema) and [PriorityLevelConfigurations](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/#prioritylevelconfiguration) to map calls to the API server into buckets which determine each user agent's throttling limits.

By default, Karpenter is installed into the `kube-system` namespace, which leverages the `system-leader-election` and `kube-system-service-accounts` [FlowSchemas](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/#flowschema) to map calls from the `kube-system` namespace to the `leader-election` and `workload-high` PriorityLevelConfigurations respectively. By putting Karpenter in these PriorityLevelConfigurations, we ensure that Karpenter and other critical cluster components are able to run even if other components on the cluster are throttled in other PriorityLevelConfigurations.

If you install Karpenter in a different namespace than the default `kube-system` namespace, Karpenter will not be put into these higher-priority FlowSchemas by default. Instead, you will need to create custom FlowSchemas for the namespace and service account where Karpenter is installed to ensure that requests are put into this higher PriorityLevelConfiguration.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-karpenter/scripts/step15-apply-flowschemas.sh" language="bash"%}}
