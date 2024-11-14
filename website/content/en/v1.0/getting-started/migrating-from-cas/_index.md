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
* You will use existing VPC and subnets
* You will use existing security groups
* Your nodes are part of one or more node groups
* Your workloads have pod disruption budgets that adhere to [EKS best practices](https://aws.github.io/aws-eks-best-practices/karpenter/)
* Your cluster has an [OIDC provider](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html) for service accounts

This guide will also assume you have the `aws` CLI and Helm client installed.
You can also perform many of these steps in the console, but we will use the command line for simplicity.

## Set environment variables
Set the Karpenter and Kubernetes version. Check the [Compatibility Matrix](https://karpenter.sh/docs/upgrading/compatibility/) to find the Karpenter version compatible with your current Amazon EKS version.

```bash
KARPENTER_NAMESPACE="kube-system"
KARPENTER_VERSION="{{< param "latest_release_version" >}}"
K8S_VERSION="{{< param "latest_k8s_version" >}}"
CLUSTER_NAME=<your cluster name>
```

Set other variables from your cluster configuration.

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step01-env.sh" language="bash" %}}

{{% alert title="Warning" color="warning" %}}
If you open a new shell to run steps in this procedure, you need to set the environment variables again.
{{% /alert %}}

## Create IAM roles

Use CloudFormation to set up the infrastructure needed by the existing EKS cluster. See [CloudFormation]({{< relref "../../reference/cloudformation/" >}}) for a complete description of what `cloudformation.yaml` does for Karpenter. The provided `cloudformation.yaml` template simplifies this setup by creating and configuring all necessary resources, including:

  - **IAM Roles and Policies**: Grants Karpenter permissions to interact with EKS, autoscaling, and EC2 services, enabling it to manage nodes dynamically.
  - **Instance Profiles**: Attaches necessary permissions to EC2 instances, allowing them to join the cluster and participate in automated scaling as managed by Karpenter.
  - **Interruption Queue and Policies**: Setup Amazon SQS queue and Event Rules for handling interruption notifications from AWS services related to EC2 Spot instances and AWS Health events.

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step02-cloudformation-setup.sh" language="bash" %}}

Now we need to create an IAM role that the Karpenter controller will use to provision new instances.
The controller will be using [IAM Roles for Service Accounts (IRSA)](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) which requires an OIDC endpoint.

If you have another option for using IAM credentials with workloads (e.g. [Amazon EKS Pod Identity Agent](https://github.com/aws/eks-pod-identity-agent)) your steps will be different.


{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step03-controller-iam.sh" language="bash" %}}

## Add tags to subnets and security groups

In order for Karpenter to know which [subnets](https://karpenter.sh/docs/concepts/nodeclasses/#specsecuritygroupselectorterms) and [security groups](https://karpenter.sh/docs/concepts/nodeclasses/#specsecuritygroupselectorterms) to use, we need to add appropriate tags to the nodegroup subnets and security groups.

### Tag nodegroup subnets

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step04-tag-subnets.sh" language="bash" %}}

This loop ensures that Karpenter will be aware of which subnets are associated with each nodegroup by tagging them with karpenter.sh/discovery.

### Tag security groups

If your EKS setup is configured to use cluster security group and additional security groups, execute the following commands to tag them for Karpenter discovery:

```bash
SECURITY_GROUPS=$(aws eks describe-cluster \
    --name "${CLUSTER_NAME}" \
    --query "cluster.resourcesVpcConfig" \
    --output json | jq -r '[.clusterSecurityGroupId] + .securityGroupIds | join(" ")')

aws ec2 create-tags \
    --tags "Key=karpenter.sh/discovery,Value=${CLUSTER_NAME}" \
    --resources ${SECURITY_GROUPS}
```

If your setup uses security groups from the Launch template of a managed nodegroup, execute the following:

Note that this command will only tag the security groups for the first nodegroup in the cluster. If you have multiple nodegroups groups, you will need to decide which ones Karpenter should use.

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step05-tag-security-groups.sh" language="bash" %}}

Alternatively, the subnets and security groups can also be defined in the [NodeClasses](https://karpenter.sh/docs/concepts/nodeclasses/) definition by specifying the [subnets](https://karpenter.sh/docs/concepts/nodeclasses/#specsubnets) and [security groups](https://karpenter.sh/docs/concepts/nodeclasses/#specsecuritygroupselectorterms) to be used.


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
To deploy Karpenter, you can use Helm, which simplifies the installation process by handling Karpenter’s dependencies and configuration files automatically. The Helm command provided below will also incorporate any customized settings, such as node affinity, to align with your specific deployment needs.

### Set Node Affinity for Karpenter

To optimize resource usage and ensure that Karpenter schedules its pods on nodes within a specific, existing node group, it is essential to configure node affinity.

Create a file named karpenter-node-affinity.yaml to define the node affinity settings and specify the node group where you want Karpenter to deploy.

Be sure to replace `${NODEGROUP}` with the actual name of your node group.

```bash
cat <<EOF > karpenter-node-affinity.yaml
affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
        - matchExpressions:
            - key: karpenter.sh/nodepool
              operator: DoesNotExist
            - key: eks.amazonaws.com/nodegroup
              operator: In
              values:
                - "${NODEGROUP}"
EOF
```

Now that you have prepared the node affinity configuration, you can proceed to install Karpenter using Helm. This command includes the affinity settings along with other necessary configurations:

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step07-deploy.sh" language="bash" %}}

Expected output:
```bash
Release "karpenter" does not exist. Installing it now.
Pulled: public.ecr.aws/karpenter/karpenter:1.0.5
Digest: sha256:98382d6406a3c85711269112fbb337c056d4debabaefb936db2d10137b58bd1b
NAME: karpenter
LAST DEPLOYED: Wed Nov  6 16:51:41 2024
NAMESPACE: kube-system
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

## Create default NodePool

We need to create a default NodePool so Karpenter knows what types of nodes we want for unscheduled workloads. You can refer to some of the [example NodePool](https://github.com/aws/karpenter/tree{{< githubRelRef >}}examples/v1) for specific needs.

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step08-create-nodepool.sh" language="bash" %}}

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

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step09-scale-cas.sh" language="bash" %}}

To get rid of the instances that were added from the node group we can scale our nodegroup down to a minimum size to support Karpenter and other critical services.

> Note: If your workloads do not have [pod disruption budgets](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) set, the following command **will cause workloads to be unavailable.**

If you have a single multi-AZ node group, we suggest a minimum of 2 instances.

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step10-scale-single-ng.sh" language="bash" %}}

Or, if you have multiple single-AZ node groups, we suggest a minimum of 1 instance each.

{{% script file="./content/en/{VERSION}/getting-started/migrating-from-cas/scripts/step11-scale-multiple-ng.sh" language="bash" %}}

{{% alert title="Note" color="warning" %}}
If you have a lot of nodes or workloads you may want to slowly scale down your node groups by a few instances at a time. It is recommended to watch the transition carefully for workloads that may not have enough replicas running or disruption budgets configured.
{{% /alert %}}


## Verify Karpenter

As nodegroup nodes are drained you can verify that Karpenter is creating nodes for your workloads.

```bash
kubectl logs -f -n $KARPENTER_NAMESPACE -c controller -l app.kubernetes.io/name=karpenter
```

You should also see new nodes created in your cluster as the old nodes are removed.

```bash
kubectl get nodes
```
