---
title: "Getting Started"
linkTitle: "Getting Started with Karpenter"
weight: 10
description: >
  How to get started with Karpenter
cascade:
  type: docs
---

Learn more about Karpenter and how to get started below.

* [Karpenter EKS Best Practices](https://aws.github.io/aws-eks-best-practices/karpenter/) guide
* [EC2 Spot Workshop for Karpenter](https://ec2spotworkshops.com/karpenter.html)
* [EKS Karpenter Workshop](https://www.eksworkshop.com/beginner/085_scaling_karpenter/set_up_the_provisioner/)

This guide shows how to get started with Karpenter by installing and configuring it on a Kubernetes cluster.
If that cluster is currently using Cluster Autoscaler, the guide also gives you the option of migrating away from Cluster Autoscaler.

This guide assumes that you are starting with an existing Kubernetes cluster and that the following are true:

* You will use existing VPC and subnets
* You will use existing security groups
* Your workloads have pod disruption budgets that adhere to [EKS best practices](https://aws.github.io/aws-eks-best-practices/karpenter/)
* Your cluster has an [OIDC provider](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html) for service accounts
* Your nodes may be part of one or more node groups

This guide will also assume you have the `aws` and `kubectl` CLI tools installed.
You can also perform many of these steps in the console, but we will use the command line for simplicity.

## Get a cluster
To use Karpenter, you must be running a supported Kubernetes cluster on a supported cloud provider.
Currently, only EKS on AWS is supported.
Recommended ways of deploying EKS on AWS to use with Karpenter include:

* [Creating an Amazon EKS cluster](https://docs.aws.amazon.com/eks/latest/userguide/create-cluster.html): Use `eksctl` or `aws` CLI tools or the AWS Management console to deploy an EKS cluster.
* [Amazon EKS Blueprints for Terraform](https://aws-ia.github.io/terraform-aws-eks-blueprints): Follow a basic [Getting Started](https://aws-ia.github.io/terraform-aws-eks-blueprints/v4.18.0/getting-started/) guide and also add modules and add-ons. This includes a [Karpenter](https://aws-ia.github.io/terraform-aws-eks-blueprints/v4.18.0/add-ons/karpenter/) add-on that lets you bypass the instructions in this guide for setting up Karpenter.

Although not supported, Karpenter could work on other Kubernetes distributions running on AWS. For example:
* [kOps](https://kops.sigs.k8s.io/operations/karpenter/): These instructions describe how to create a kOps Kubernetes cluster in AWS that includes Karpenter.

If you have not already configured Karpenter when you set up your Kubernetes cluster in AWS, follow the instructions below to add Karpenter to your existing cluster.

## Add Karpenter to an existing cluster

To add Karpenter to your cluster, you need to create IAM roles, add tags to subnets and security groups, and update the aws-auth ConfigMap.

### Create IAM role

To get started, create a new IAM role for the Karpenter controller.

The controller will be using [IAM Roles for Service Accounts (IRSA)](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) which requires an OIDC endpoint.

If you have another option for using IAM credentials with workloads (e.g. [kube2iam](https://github.com/jtblin/kube2iam)) your steps will be different.

Set a variable for your cluster name.

```bash
CLUSTER_NAME=<your cluster name>
```

Set other variables from your cluster configuration.

{{% script file="./scripts/step04-env.sh" language="bash" %}}

Use that information to create our IAM role, inline policy, and trust relationship.

{{% script file="./scripts/step05-controller-iam.sh" language="bash" %}}

### Add tags to subnets and security groups

Next add tags to your nodegroup subnets so Karpenter will know which subnets to use.

{{% script file="./scripts/step06-tag-subnets.sh" language="bash" %}}

Add tags to the security groups.
This command only tags the security groups for the first nodegroup in the cluster.
If you have multiple nodegroups or multiple security groups you will need to decide which one Karpenter should use.

{{% script file="./scripts/step07-tag-security-groups.sh" language="bash" %}}

### Deploy Karpenter

First set the Karpenter release you want to deploy and the IAM instance profile that is used by the existing nodes in your cluster.
```bash
export KARPENTER_VERSION={{< param "latest_release_version" >}}
export IAM_INSTANCE_PROFILE_NAME=<your instance profile name>
```

We can now generate a full Karpenter deployment yaml from the helm chart.

{{% script file="./scripts/step09-generate-chart.sh" language="bash" %}}

Modify the following lines in the karpenter.yaml file.

### Set node affinity

Edit the karpenter.yaml file and find the karpenter deployment affinity rules.
Modify the affinity so karpenter will run on one of the existing node group nodes.

The rules should look something like this.
Modify the value to match your `$NODEGROUP`.

```
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: karpenter.sh/provisioner-name
                operator: DoesNotExist
            - matchExpressions:
              - key: eks.amazonaws.com/nodegroup
                operator: In
                values:
                - ${NODEGROUP}
```

Now that our deployment is ready we can create the karpenter namespace, create the provisioner CRD, and then deploy the rest of the karpenter resources.

{{% script file="./scripts/step10-deploy.sh" language="bash" %}}

## Create default provisioner

We need to create a default provisioner so Karpenter knows what types of nodes we want for unscheduled workloads.
You can refer to some of the [example provisioners](https://github.com/aws/karpenter/tree{{< githubRelRef >}}examples/provisioner) for specific needs.

{{% script file="./scripts/step11-create-provisioner.sh" language="bash" %}}

## Set nodeAffinity for critical workloads (optional)

You may also want to set a nodeAffinity for other critical cluster workloads.

Some examples are

* coredns
* metric-server

You can edit them with `kubectl edit deploy ...` and you should add node affinity for your static node group instances.
Modify the value to match your `$NODEGROUP`.

```
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

## Remove Cluster Autoscaler

Now that Karpenter is running, if your cluster is running cluster autoscaler you have the option of removing it.
To do that, scale the number of replicas to zero.

{{% script file="./scripts/step12-scale-cas.sh" language="bash" %}}

To get rid of the instances that were added from the node group we can scale our nodegroup down to a minimum size to support Karpenter and other critical services.
We suggest a minimum of 2 nodes for the node group.

> Note: If your workloads do not have [pod disruption budgets](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) set,
> the following command **will cause workloads to be unavailable.**

{{% script file="./scripts/step13-scale-ng.sh" language="bash" %}}

If you have a lot of nodes or workloads you may want to slowly scale down your node groups by a few instances at a time.
It is recommended to watch the transition carefully for workloads that may not have enough replicas running or disruption budgets configured.

### Verify Karpenter

As nodegroup nodes are drained you can verify that Karpenter is creating nodes for your workloads.

```bash
kubectl logs -f -n karpenter -c controller -l app.kubernetes.io/name=karpenter
```

You should also see new nodes created in your cluster as the old nodes are removed

```bash
kubectl get nodes
```
## First Use

Karpenter is now active and ready to begin provisioning nodes.
Create some pods using a deployment, and watch Karpenter provision nodes in response.

### Automatic Node Provisioning

This deployment uses the [pause image](https://www.ianlewis.org/en/almighty-pause-container) and starts with zero replicas.

{{% script file="./scripts/step13-automatic-node-provisioning.sh" language="bash"%}}

### Automatic Node Termination

Now, delete the deployment. After 30 seconds (`ttlSecondsAfterEmpty`),
Karpenter should terminate the now empty nodes.

{{% script file="./scripts/step14-deprovisioning.sh" language="bash"%}}

### Manual Node Termination

If you delete a node with kubectl, Karpenter will gracefully cordon, drain,
and shutdown the corresponding instance. Under the hood, Karpenter adds a
finalizer to the node object, which blocks deletion until all pods are
drained and the instance is terminated. Keep in mind, this only works for
nodes provisioned by Karpenter.

{{% script file="./scripts/step15-delete-node.sh" language="bash"%}}

## Cleanup

To remove the changes you made from this document, do the following:

{{% script file="./scripts/step16-cleanup.sh" language="bash"%}}

If you are done with the cluster, the way you remove it depends on how you created it originally.
For example, to remove a cluster created with `eksctl`, you could run:

```bash
eksctl delete cluster --name "${CLUSTER_NAME}"
