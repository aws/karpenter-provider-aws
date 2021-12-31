
---
title: "Getting Started with Terraform"
linkTitle: "Getting Started with Terraform"
weight: 10
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

Karpenter is installed in clusters with a helm chart.

Karpenter additionally requires IAM Roles for Service Accounts (IRSA). IRSA
permits Karpenter (within the cluster) to make privileged requests to AWS (as
the cloud provider).

### Required Utilities

Install these tools before proceeding:

1. [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2-linux.html)
2. `kubectl` - [the Kubernetes CLI](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/)
3. `terraform` - [infrastructure-as-code tool made by HashiCorp](https://learn.hashicorp.com/tutorials/terraform/install-cli)
4. `helm` - [the package manager for Kubernetes](https://helm.sh/docs/intro/install/)

Login to the AWS CLI with a user that has sufficient privileges to create a
cluster.




### Setting up Variables

After setting up the tools, set the following environment variables to store
commonly used values.

```bash
export CLUSTER_NAME=$USER-karpenter-demo
export AWS_DEFAULT_REGION=us-west-2
```

The first thing we need to do is create our `main.tf` file and place the 
following in it. This will let us pass in a cluster name that will be used 
throughout the remainder of our config.

```hcl
variable "cluster_name" {
  description = "The name of the cluster"
  type        = string
}
```


### Create a Cluster

We're going to use two different Terraform modules to create our cluster - one
to create the VPC and another for the cluster itself. The key part of this is
that we need to tag the VPC subnets that we want to use for the worker nodes. 

Place the following Terraform config into your `main.tf` file.

```hcl
module "vpc" {
  source = "terraform-aws-modules/vpc/aws"

  name = var.cluster_name
  cidr = "10.0.0.0/16"

  azs             = ["us-east-1a", "us-east-1b", "us-east-1c"]
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]

  enable_nat_gateway     = true
  single_nat_gateway     = true
  one_nat_gateway_per_az = false

  private_subnet_tags = {
    "kubernetes.io/cluster/${var.cluster_name}" = "owned"
  }
}

module "eks" {
  source          = "terraform-aws-modules/eks/aws"

  cluster_version = "1.21"
  cluster_name    = var.cluster_name
  vpc_id          = module.vpc.vpc_id
  subnets         = module.vpc.private_subnets
  enable_irsa     = true

  # Only need one node to get Karpenter up and running
  worker_groups = [
    {
      instance_type = "t3a.medium"
      asg_max_size  = 1
    }
  ]
}
```

At this point, go ahead and apply what we've done to create the VPC and
cluster. This may take some time.

```bash
terraform init
terraform apply -var cluster_name=$CLUSTER_NAME
```

There's a good chance it will fail when trying to configure the aws-auth
ConfigMap. And that's because we need to use the kubeconfig file that was
generated during the cluster install. To use it, run the following. This will
configure both your local CLI and Terraform to use the file. Then try the apply
again.

```bash
export KUBECONFIG=${PWD}/kubeconfig_${CLUSTER_NAME}
export KUBE_CONFIG_PATH=$KUBECONFIG
terraform apply -var cluster_name=$CLUSTER_NAME
```

Everything should apply successfully now!


### Configure the KarpenterNode IAM Role

The EKS module creates an IAM role for worker nodes. We'll use that for 
Karpenter (so we don't have to reconfigure the aws-auth ConfigMap), but we need
to add one more policy and create an instance profile.

Place the following into your `main.tf` to add the policy and create an 
instance profile.

```hcl
data "aws_iam_policy" "ssm_managed_instance" {
  arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_role_policy_attachment" "karpenter_ssm_policy" {
  role       = module.eks.worker_iam_role_name
  policy_arn = data.aws_iam_policy.ssm_managed_instance.arn
}

resource "aws_iam_instance_profile" "karpenter" {
  name = "KarpenterNodeInstanceProfile-${var.cluster_name}"
  role = module.eks.worker_iam_role_name
}
```

Go ahead and apply the changes.

```bash
terraform apply -var cluster_name=$CLUSTER_NAME
```

Now, Karpenter can use this instance profile to launch new EC2 instances and 
those instances will be able to connect to your cluster.

### Create the KarpenterController IAM Role

Karpenter requires permissions like launching instances, which means it needs
an IAM role that grants it access. The config below will create an AWS IAM 
Role, attach a policy, and authorize the Service Account to assume the role 
using [IRSA](https://docs.aws.amazon.com/emr/latest/EMR-on-EKS-DevelopmentGuide/setting-up-enable-IAM.html).
We will create the ServiceAccount and connect it to this role during the Helm
chart install.

```hcl
module "iam_assumable_role_karpenter" {
  source                        = "terraform-aws-modules/iam/aws//modules/iam-assumable-role-with-oidc"
  version                       = "4.7.0"
  create_role                   = true
  role_name                     = "karpenter-controller-${var.cluster_name}"
  provider_url                  = module.eks.cluster_oidc_issuer_url
  oidc_fully_qualified_subjects = ["system:serviceaccount:karpenter:karpenter"]
}

resource "aws_iam_role_policy" "karpenter_contoller" {
  name = "karpenter-policy-${var.cluster_name}"
  role = module.iam_assumable_role_karpenter.iam_role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "ec2:CreateLaunchTemplate",
          "ec2:CreateFleet",
          "ec2:RunInstances",
          "ec2:CreateTags",
          "iam:PassRole",
          "ec2:TerminateInstances",
          "ec2:DescribeLaunchTemplates",
          "ec2:DescribeInstances",
          "ec2:DescribeSecurityGroups",
          "ec2:DescribeSubnets",
          "ec2:DescribeInstanceTypes",
          "ec2:DescribeInstanceTypeOfferings",
          "ec2:DescribeAvailabilityZones",
          "ssm:GetParameter"
        ]
        Effect   = "Allow"
        Resource = "*"
      },
    ]
  })
}
```

Since we've added a new module, you'll need to run `terraform init` again. 
Then, apply the changes.

```bash
terraform init
terraform apply -var cluster_name=$CLUSTER_NAME
```

### Install Karpenter Helm Chart

Use helm to deploy Karpenter to the cluster. We are going to use the 
`helm_release` Terraform resource to do the deploy and pass in the cluster
details and IAM role Karpenter needs to assume.

```hcl
resource "helm_release" "karpenter" {
  depends_on       = [module.eks.kubeconfig]
  namespace        = "karpenter"
  create_namespace = true

  name       = "karpenter"
  repository = "https://charts.karpenter.sh"
  chart      = "karpenter"
  version    = "{{< param "latest_release_version" >}}"

  set {
    name  = "serviceAccount.annotations.eks\\.amazonaws\\.com/role-arn"
    value = module.iam_assumable_role_karpenter.iam_role_arn
  }

  set {
    name  = "controller.clusterName"
    value = var.cluster_name
  }

  set {
    name  = "controller.clusterEndpoint"
    value = module.eks.cluster_endpoint
  }
}
```

Now, deploy Karpenter by applying the new Terraform config.

```bash
terraform init
terraform apply -var cluster_name=$CLUSTER_NAME
```


### Enable Debug Logging (optional)
```sh
kubectl patch configmap config-logging -n karpenter --patch '{"data":{"loglevel.controller":"debug"}}'
```

### Provisioner

A single Karpenter provisioner is capable of handling many different pod
shapes. Karpenter makes scheduling and provisioning decisions based on pod
attributes such as labels and affinity. In other words, Karpenter eliminates
the need to manage many different node groups.

Create a default provisioner using the command below. This provisioner
configures instances to connect to your cluster's endpoint and discovers
resources like subnets and security groups using the cluster's name.

The `ttlSecondsAfterEmpty` value configures Karpenter to terminate empty nodes.
This behavior can be disabled by leaving the value undefined.

Review the [provisioner CRD](/docs/provisioner-crd) for more information. For example,
`ttlSecondsUntilExpired` configures Karpenter to terminate nodes when a maximum age is reached.

Note: This provisioner will create capacity as long as the sum of all created capacity is less than the specified limit.

```bash
cat <<EOF | kubectl apply -f -
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
metadata:
  name: default
spec:
  requirements:
    - key: karpenter.sh/capacity-type
      operator: In
      values: ["spot"]
  limits:
    resources:
      cpu: 1000
  provider:
    instanceProfile: KarpenterNodeInstanceProfile-${CLUSTER_NAME}
  ttlSecondsAfterEmpty: 30
EOF
```

## First Use

Karpenter is now active and ready to begin provisioning nodes.
Create some pods using a deployment, and watch Karpenter provision nodes in response.

### Automatic Node Provisioning

This deployment uses the [pause image](https://www.ianlewis.org/en/almighty-pause-container) and starts with zero replicas.

```bash
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: inflate
spec:
  replicas: 0
  selector:
    matchLabels:
      app: inflate
  template:
    metadata:
      labels:
        app: inflate
    spec:
      terminationGracePeriodSeconds: 0
      containers:
        - name: inflate
          image: public.ecr.aws/eks-distro/kubernetes/pause:3.2
          resources:
            requests:
              cpu: 1
EOF
kubectl scale deployment inflate --replicas 5
kubectl logs -f -n karpenter $(kubectl get pods -n karpenter -l karpenter=controller -o name)
```

### Automatic Node Termination

Now, delete the deployment. After 30 seconds (`ttlSecondsAfterEmpty`),
Karpenter should terminate the now empty nodes.

```bash
kubectl delete deployment inflate
kubectl logs -f -n karpenter $(kubectl get pods -n karpenter -l karpenter=controller -o name)
```

### Manual Node Termination

If you delete a node with kubectl, Karpenter will gracefully cordon, drain,
and shutdown the corresponding instance. Under the hood, Karpenter adds a
finalizer to the node object, which blocks deletion until all pods are
drained and the instance is terminated. Keep in mind, this only works for
nodes provisioned by Karpenter.

```bash
kubectl delete node $NODE_NAME
```

## Cleanup

To avoid additional charges, remove the demo infrastructure from your AWS 
account. Since Karpenter is managing nodes outside of Terraform's view, we need
to remove the pods and node first (if you haven't already). Once the node is 
removed, you can remove the rest of the infrastructure.

```bash
kubectl delete deployment inflate
kubectl delete node -l karpenter.sh/provisioner-name=default
terraform destroy -var cluster_name=$CLUSTER_NAME
```
