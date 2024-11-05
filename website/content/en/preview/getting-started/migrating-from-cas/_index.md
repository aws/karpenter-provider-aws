---
title: "Migrating from Cluster Autoscaler"
linkTitle: "Migrating from Cluster Autoscaler"
weight: 10
description: >
  Migrate to Karpenter from Cluster Autoscaler
---

This guide will show you how to switch from the [Kubernetes Cluster Autoscaler](https://github.com/kubernetes/autoscaler) to Karpenter for automatic node provisioning.
We will make the following assumptions in this guide

* You will use an existing EKS cluster
* You will use an existing VPC and subnets
* You will use an existing security groups
* Your nodes are part of one or more node groups
* Your workloads have pod disruption budgets that adhere to [EKS best practices](https://aws.github.io/aws-eks-best-practices/karpenter/)
* Your cluster has an [OIDC provider](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html) for service accounts

This guide will also assume you have the `aws` CLI installed.
You can also perform many of these steps in the console, but we will use the command line for simplicity.

Set a variable for your cluster name.

```bash
KARPENTER_NAMESPACE=kube-system
CLUSTER_NAME=<your cluster name>
```

Set other variables from your cluster configuration.

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step01-env.sh" language="bash" %}}


## Create IAM roles

Use CloudFormation to set up the infrastructure needed by the existing EKS cluster. See [CloudFormation]({{< relref "../../reference/cloudformation/" >}}) for a complete description of what `cloudformation.yaml` does for Karpenter. The provided `cloudformation.yaml` template simplifies this setup by creating and configuring all necessary resources, including:

  - **IAM Roles and Policies**: Grants Karpenter permissions to interact with EKS, autoscaling, and EC2 services, enabling it to manage nodes dynamically.
  - **Instance Profiles**: Attaches necessary permissions to EC2 instances, allowing them to join the cluster and participate in automated scaling as managed by Karpenter.
  - **Interruption Queue and Policies**: Setup Amazon SQS queue and Event Rules for handling interruption notifications from AWS services related to EC2 Spot instances and AWS Health events.

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step-02-cloudformation-setup.sh" language="bash" %}}

Now we need to create an IAM role that the Karpenter controller will use to provision new instances.
The controller will be using [IAM Roles for Service Accounts (IRSA)](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) which requires an OIDC endpoint.

If you have another option for using IAM credentials with workloads (e.g. [Amazon EKS Pod Identity Agent](https://github.com/aws/eks-pod-identity-agent)) your steps will be different.


{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step03-controller-iam.sh" language="bash" %}}

## Add tags to subnets and security groups

We need to add tags to our nodegroup subnets so Karpenter will know which subnets to use.

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step04-tag-subnets.sh" language="bash" %}}

Add tags to our security groups.
This command only tags the security groups for the first nodegroup in the cluster.
If you have multiple nodegroups or multiple security groups you will need to decide which one Karpenter should use.

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step05-tag-security-groups.sh" language="bash" %}}

## Update aws-auth ConfigMap

We need to allow nodes that are using the node IAM role we just created to join the cluster.
To do that we have to modify the `aws-auth` ConfigMap in the cluster.

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step06-edit-aws-auth.sh" language="bash" %}}

You will need to add a section to the mapRoles that looks something like this.
Replace the `${AWS_PARTITION}` variable with the account partition, `${AWS_ACCOUNT_ID}` variable with your account ID, and `${CLUSTER_NAME}` variable with the cluster name, but do not replace the `{{EC2PrivateDNSName}}`.

```yaml
- groups:
  - system:bootstrappers
  - system:nodes
  ## If you intend to run Windows workloads, the kube-proxy group should be specified.
  # For more information, see https://github.com/aws/karpenter/issues/5099.
  # - eks:kube-proxy-windows
  rolearn: arn:${AWS_PARTITION}:iam::${AWS_ACCOUNT_ID}:role/KarpenterNodeRole-${CLUSTER_NAME}
  username: system:node:{{EC2PrivateDNSName}}
```

The full aws-auth configmap should have two groups.
One for your Karpenter node role and one for your existing node group.

## Deploy Karpenter

First set the Karpenter release you want to deploy.

```bash
export KARPENTER_VERSION="{{< param "latest_release_version" >}}"
```

### Set Node Affinity for Karpenter

To optimize resource usage and ensure Karpenter schedules its pods on nodes belonging to a specific existing node group, it is essential to set node affinity. This guide provides a step-by-step process for creating a node affinity configuration and deploying Karpenter.

#### Create the Node Affinity Configuration File

First, create a file named `karpenter-node-affinity.yaml` to define the node affinity settings. This configuration will direct Karpenter to only schedule its pods on nodes that meet specified criteria. Use the following command to create the file:

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step07-karpenter-node-affinity.sh" language="bash" %}}

**Make sure to replace ${NODEGROUP} with the name of your actual node group.**

Now that you have prepared the node affinity configuration, you can proceed to install or upgrade Karpenter using Helm. This command includes the affinity settings along with other necessary configurations:

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step08-deploy.sh" language="bash" %}}

## Create default NodePool

We need to create a default NodePool so Karpenter knows what types of nodes we want for unscheduled workloads. You can refer to some of the [example NodePool](https://github.com/aws/karpenter/tree{{< githubRelRef >}}examples/v1) for specific needs.

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step09-create-nodepool.sh" language="bash" %}}

## Set nodeAffinity for critical workloads (optional)

You may also want to set a nodeAffinity for other critical cluster workloads.

Some examples are

* coredns
* metric-server

You can edit them with `kubectl edit deploy ...` and you should add node affinity for your static node group instances.
Modify the value to match your `$NODEGROUP`, one node group per line.

```yaml
affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
      - matchExpressions:
        - key: eks.amazonaws.com/nodegroup
          operator: In
          values:
          - ${NODEGROUP}
```

## Remove CAS

Now that karpenter is running we can disable the cluster autoscaler.
To do that we will scale the number of replicas to zero.

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step10-scale-cas.sh" language="bash" %}}

To get rid of the instances that were added from the node group we can scale our nodegroup down to a minimum size to support Karpenter and other critical services.

> Note: If your workloads do not have [pod disruption budgets](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) set, the following command **will cause workloads to be unavailable.**

If you have a single multi-AZ node group, we suggest a minimum of 2 instances.

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step11-scale-single-ng.sh" language="bash" %}}

Or, if you have multiple single-AZ node groups, we suggest a minimum of 1 instance each.

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step12-scale-multiple-ng.sh" language="bash" %}}

{{% alert title="Note" color="warning" %}}
If you have a lot of nodes or workloads you may want to slowly scale down your node groups by a few instances at a time. It is recommended to watch the transition carefully for workloads that may not have enough replicas running or disruption budgets configured.
{{% /alert %}}

## Verify Karpenter

As nodegroup nodes are drained you can verify that Karpenter is creating nodes for your workloads.

```bash
kubectl logs -f -n karpenter -c controller -l app.kubernetes.io/name=karpenter
```

You should also see new nodes created in your cluster as the old nodes are removed

```bash
kubectl get nodes
```
