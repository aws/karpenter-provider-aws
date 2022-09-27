
---
title: "Getting Started with eksctl"
linkTitle: "Getting Started with eksctl"
weight: 10
description: >
  Set up Karpenter with an eksctl cluster
---

Karpenter automatically provisions new nodes in response to unschedulable
pods. Karpenter does this by observing events within the Kubernetes cluster,
and then sending commands to the underlying cloud provider.

In this example, the cluster is running on Amazon Web Services (AWS) Elastic
Kubernetes Service (EKS). Karpenter is designed to be cloud provider agnostic,
but currently only supports AWS. Contributions are welcomed.

This guide should take less than 1 hour to complete, and cost less than $0.25.
Follow the clean-up instructions to reduce any charges.

## Install

Karpenter is installed in clusters with a Helm chart.

Karpenter requires cloud provider permissions to provision nodes, for AWS IAM
Roles for Service Accounts (IRSA) should be used. IRSA permits Karpenter
(within the cluster) to make privileged requests to AWS (as the cloud provider)
via a ServiceAccount.

### Required Utilities

Install these tools before proceeding:

1. [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2-linux.html)
2. `kubectl` - [the Kubernetes CLI](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/)
3. `eksctl` - [the CLI for AWS EKS](https://docs.aws.amazon.com/eks/latest/userguide/eksctl.html)
4. `helm` - [the package manager for Kubernetes](https://helm.sh/docs/intro/install/)

[Configure the AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html)
with a user that has sufficient privileges to create an EKS cluster. Verify that the CLI can
authenticate properly by running `aws sts get-caller-identity`.

### Environment Variables

After setting up the tools, set the following environment variable to the Karpenter version you
would like to install.

```bash
export KARPENTER_VERSION=v0.16.3
```

Also set the following environment variables to store commonly used values.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step01-config.sh" language="bash"%}}

{{% alert title="Warning" color="warning" %}}
If you open a new shell to run steps in this procedure, you need to set some or all of the environment variables again.
To remind yourself of these values, type:

```bash
echo $KARPENTER_VERSION $CLUSTER_NAME $AWS_DEFAULT_REGION $AWS_ACCOUNT_ID
```

{{% /alert %}}


### Create a Cluster

Create a basic cluster with `eksctl`.
Each of the two examples set up an IAM OIDC provider for the cluster to enable IAM roles for pods.
The first uses [AWS EKS managed node groups](https://docs.aws.amazon.com/eks/latest/userguide/managed-node-groups.html) for the kube-system and karpenter namespaces, while the second uses Fargate for both namespaces.

**Example 1: Create basic cluster**

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step02-create-cluster.sh" language="bash"%}}

**Example 2: Create basic cluster with Karpenter on Fargate**

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step02-create-cluster-fargate.sh" language="bash"%}}

Karpenter itself can run anywhere, including on [self-managed node groups](https://docs.aws.amazon.com/eks/latest/userguide/worker.html), [managed node groups](https://docs.aws.amazon.com/eks/latest/userguide/managed-node-groups.html) (Example 1), or [AWS Fargate](https://aws.amazon.com/fargate/)(Example 2).

Karpenter will provision EC2 instances in your account.

### Create the KarpenterNode IAM Role

Instances launched by Karpenter must run with an InstanceProfile that grants permissions necessary to run containers and configure networking. Karpenter discovers the InstanceProfile using the name `KarpenterNodeRole-${ClusterName}`.

First, create the IAM resources using AWS CloudFormation.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step03-iam-cloud-formation.sh" language="bash"%}}

Second, grant access to instances using the profile to connect to the cluster. This command adds the Karpenter node role to your aws-auth configmap, allowing nodes with this role to connect to the cluster.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step04-grant-access.sh" language="bash"%}}

Now, Karpenter can launch new EC2 instances and those instances can connect to your cluster.

### Create the KarpenterController IAM Role

Karpenter requires permissions like launching instances. This will create an AWS IAM Role, Kubernetes service account, and associate them using [IRSA](https://docs.aws.amazon.com/emr/latest/EMR-on-EKS-DevelopmentGuide/setting-up-enable-IAM.html).

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step05-controller-iam.sh" language="bash"%}}

### Create the EC2 Spot Service Linked Role

This step is only necessary if this is the first time you're using EC2 Spot in this account. More details are available [here](https://docs.aws.amazon.com/batch/latest/userguide/spot_fleet_IAM_role.html).

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step06-add-spot-role.sh" language="bash"%}}

### Install Karpenter Helm Chart

Use Helm to deploy Karpenter to the cluster.

Before the chart can be installed the repo needs to be added to Helm, run the following commands to add the repo.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step07-install-helm-chart.sh" language="bash"%}}

Install the chart passing in the cluster details and the Karpenter role ARN.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step08-apply-helm-chart.sh" language="bash"%}}

#### Deploy a temporary Prometheus and Grafana stack (optional)

The following commands will deploy a Prometheus and Grafana stack that is suitable for this guide but does not include persistent storage or other configurations that would be necessary for monitoring a production deployment of Karpenter. This deployment includes two Karpenter dashboards that are automatically onboarded to Grafana. They provide a variety of visualization examples on Karpenter metrics.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step09-add-prometheus-grafana.sh" language="bash"%}}

The Grafana instance may be accessed using port forwarding.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step10-add-grafana-port-forward.sh" language="bash"%}}

The new stack has only one user, `admin`, and the password is stored in a secret. The following command will retrieve the password.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step11-grafana-get-password.sh" language="bash"%}}

#### Deploy the AWS Node Termination Handler to handle Spot interruptions gracefully (optional)

The following commands will deploy the AWS Node Termination Handler as a DaemonSet to run on Spot nodes to handle spot interruption notifications, 
spot rebalance recommendations, and EC2 scheduled maintenance events. Learn more about the AWS Node Termination Handler and more advanced configurations 
[here](https://github.com/aws/aws-node-termination-handler). 

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step12-install-nth.sh" language="bash"%}}

### Provisioner

A single Karpenter provisioner is capable of handling many different pod
shapes. Karpenter makes scheduling and provisioning decisions based on pod
attributes such as labels and affinity. In other words, Karpenter eliminates
the need to manage many different node groups.

Create a default provisioner using the command below.
This provisioner uses `securityGroupSelector` and `subnetSelector` to discover resources used to launch nodes.
We applied the tag `karpenter.sh/discovery` in the `eksctl` command above.
Depending how these resources are shared between clusters, you may need to use different tagging schemes.

The `ttlSecondsAfterEmpty` value configures Karpenter to terminate empty nodes.
This behavior can be disabled by leaving the value undefined.

Review the [provisioner CRD]({{<ref "../../provisioner.md" >}}) for more information. For example,
`ttlSecondsUntilExpired` configures Karpenter to terminate nodes when a maximum age is reached.

Note: This provisioner will create capacity as long as the sum of all created capacity is less than the specified limit.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step13-add-provisioner.sh" language="bash"%}}

## First Use

Karpenter is now active and ready to begin provisioning nodes.
Create some pods using a deployment, and watch Karpenter provision nodes in response.

### Automatic Node Provisioning

This deployment uses the [pause image](https://www.ianlewis.org/en/almighty-pause-container) and starts with zero replicas.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step14-automatic-node-provisioning.sh" language="bash"%}}

### Automatic Node Termination

Now, delete the deployment. After 30 seconds (`ttlSecondsAfterEmpty`),
Karpenter should terminate the now empty nodes.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step15-deprovisioning.sh" language="bash"%}}

### Manual Node Termination

If you delete a node with kubectl, Karpenter will gracefully cordon, drain,
and shutdown the corresponding instance. Under the hood, Karpenter adds a
finalizer to the node object, which blocks deletion until all pods are
drained and the instance is terminated. Keep in mind, this only works for
nodes provisioned by Karpenter.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step16-delete-node.sh" language="bash"%}}

## Cleanup

To avoid additional charges, remove the demo infrastructure from your AWS account.

{{% script file="./content/en/{VERSION}/getting-started/getting-started-with-eksctl/scripts/step17-cleanup.sh" language="bash"%}}
