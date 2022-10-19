
---
title: "Getting Started with Terraform"
linkTitle: "Getting Started with Terraform"
weight: 10
description: >
  Set up Karpenter with a Terraform cluster
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
export AWS_DEFAULT_REGION="us-east-1"
```

The first thing we need to do is create our `main.tf` file and place the following in it.

```hcl
terraform {
  required_version = "~> 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.5"
    }
    kubectl = {
      source  = "gavinbunney/kubectl"
      version = "~> 1.14"
    }
  }
}

provider "aws" {
  region = "us-east-1"
}

locals {
  cluster_name = "karpenter-demo"

  # Used to determine correct partition (i.e. - `aws`, `aws-gov`, `aws-cn`, etc.)
  partition = data.aws_partition.current.partition
}

data "aws_partition" "current" {}
```

### Create a Cluster

We're going to use two different Terraform modules to create our cluster - one
to create the VPC and another for the cluster itself. The key part of this is
that we need to tag the VPC subnets that we want to use for the worker nodes.

Add the following to your `main.tf` to create a VPC and EKS cluster.

```hcl
module "vpc" {
  # https://registry.terraform.io/modules/terraform-aws-modules/vpc/aws/latest
  source  = "terraform-aws-modules/vpc/aws"
  version = "3.14.2"

  name = local.cluster_name
  cidr = "10.0.0.0/16"

  azs             = ["us-east-1a", "us-east-1b", "us-east-1c"]
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]

  enable_nat_gateway     = true
  single_nat_gateway     = true
  one_nat_gateway_per_az = false

  public_subnet_tags = {
    "kubernetes.io/cluster/${local.cluster_name}" = "shared"
    "kubernetes.io/role/elb"                      = 1
  }

  private_subnet_tags = {
    "kubernetes.io/cluster/${local.cluster_name}" = "shared"
    "kubernetes.io/role/internal-elb"             = 1
  }
}

module "eks" {
  # https://registry.terraform.io/modules/terraform-aws-modules/eks/aws/latest
  source  = "terraform-aws-modules/eks/aws"
  version = "18.29.0"

  cluster_name    = local.cluster_name
  cluster_version = "1.22"

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  # Required for Karpenter role below
  enable_irsa = true

  node_security_group_additional_rules = {
    ingress_nodes_karpenter_port = {
      description                   = "Cluster API to Node group for Karpenter webhook"
      protocol                      = "tcp"
      from_port                     = 8443
      to_port                       = 8443
      type                          = "ingress"
      source_cluster_security_group = true
    }
  }

  node_security_group_tags = {
    # NOTE - if creating multiple security groups with this module, only tag the
    # security group that Karpenter should utilize with the following tag
    # (i.e. - at most, only one security group should have this tag in your account)
    "karpenter.sh/discovery/${local.cluster_name}" = local.cluster_name
  }

  # Only need one node to get Karpenter up and running.
  # This ensures core services such as VPC CNI, CoreDNS, etc. are up and running
  # so that Karpenter can be deployed and start managing compute capacity as required
  eks_managed_node_groups = {
    initial = {
      instance_types = ["m5.large"]
      # Not required nor used - avoid tagging two security groups with same tag as well
      create_security_group = false

      # Ensure enough capacity to run 2 Karpenter pods
      min_size     = 2
      max_size     = 3
      desired_size = 2

      iam_role_additional_policies = [
        # Required by Karpenter
        "arn:${local.partition}:iam::aws:policy/AmazonSSMManagedInstanceCore"
      ]

      tags = {
        # This will tag the launch template created for use by Karpenter
        "karpenter.sh/discovery/${local.cluster_name}" = local.cluster_name
      }
    }
  }
}
```

At this point, go ahead and apply what we've done to create the VPC and
EKS cluster. This may take some time.

```bash
terraform init
terraform apply
```

### Create the EC2 Spot Service Linked Role

This step is only necessary if this is the first time you're using EC2 Spot in this account. More details are available [here](https://docs.aws.amazon.com/batch/latest/userguide/spot_fleet_IAM_role.html).

```bash
aws iam create-service-linked-role --aws-service-name spot.amazonaws.com
# If the role has already been successfully created, you will see:
# An error occurred (InvalidInput) when calling the CreateServiceLinkedRole operation: Service role name AWSServiceRoleForEC2Spot has been taken in this account, please try a different suffix.
```

### Configure the KarpenterNode IAM Role

The EKS module creates an IAM role for the EKS managed node group nodes. We'll use that for
Karpenter (so we don't have to reconfigure the aws-auth ConfigMap), but we need
to create an instance profile we can reference.

Add the following to your `main.tf` to create the instance profile.

```hcl
resource "aws_iam_instance_profile" "karpenter" {
  name = "KarpenterNodeInstanceProfile-${local.cluster_name}"
  role = module.eks.eks_managed_node_groups["initial"].iam_role_name
}
```

Go ahead and apply the changes.

```bash
terraform apply
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

Add the following to your `main.tf` to create the IAM role for the Karpenter service account.

```hcl
module "karpenter_irsa" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "5.3.1"

  role_name                          = "karpenter-controller-${local.cluster_name}"
  attach_karpenter_controller_policy = true

  karpenter_tag_key               = "karpenter.sh/discovery/${local.cluster_name}"
  karpenter_controller_cluster_id = module.eks.cluster_id
  karpenter_controller_node_iam_role_arns = [
    module.eks.eks_managed_node_groups["initial"].iam_role_arn
  ]

  oidc_providers = {
    ex = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["karpenter:karpenter"]
    }
  }
}
```

Since we've added a new module, you'll need to run `terraform init` again before applying the changes.

```bash
terraform init
terraform apply
```

### Install Karpenter Helm Chart

Use helm to deploy Karpenter to the cluster. We are going to use the
`helm_release` Terraform resource to do the deploy and pass in the cluster
details and IAM role Karpenter needs to assume.

Add the following to your `main.tf` to provision Karpenter via a Helm chart.

```hcl
provider "helm" {
  kubernetes {
    host                   = module.eks.cluster_endpoint
    cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)

    exec {
      api_version = "client.authentication.k8s.io/v1beta1"
      command     = "aws"
      args        = ["eks", "get-token", "--cluster-name", local.cluster_name]
    }
  }
}

resource "helm_release" "karpenter" {
  namespace        = "karpenter"
  create_namespace = true

  name       = "karpenter"
  repository = "oci://public.ecr.aws/karpenter"
  chart      = "karpenter"
  version    = "v0.18.1"

  set {
    name  = "serviceAccount.annotations.eks\\.amazonaws\\.com/role-arn"
    value = module.karpenter_irsa.iam_role_arn
  }

  set {
    name  = "clusterName"
    value = module.eks.cluster_id
  }

  set {
    name  = "clusterEndpoint"
    value = module.eks.cluster_endpoint
  }

  set {
    name  = "aws.defaultInstanceProfile"
    value = aws_iam_instance_profile.karpenter.name
  }
}
```

Since we've added a new provider (helm), you'll need to run `terraform init` again
before applying the changes to deploy Karpenter.

```bash
terraform init
terraform apply
```

### Enable Debug Logging (optional)

The global log level can be modified with the `logLevel` chart value (e.g. `--set logLevel=debug`) or the individual components can have their log level set with `controller.logLevel` or `webhook.logLevel` chart values.

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

Review the [provisioner CRD]({{<ref "../../provisioner.md" >}}) for more information. For example,
`ttlSecondsUntilExpired` configures Karpenter to terminate nodes when a maximum age is reached.

Add the following to your `main.tf` to deploy the Karpenter provisioner.

Note: This provisioner will create capacity as long as the sum of all created capacity is less than the specified limit.

```hcl
provider "kubectl" {
  apply_retry_count      = 5
  host                   = module.eks.cluster_endpoint
  cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)
  load_config_file       = false

  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "aws"
    args        = ["eks", "get-token", "--cluster-name", module.eks.cluster_id]
  }
}

resource "kubectl_manifest" "karpenter_provisioner" {
  yaml_body = <<-YAML
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
      subnetSelector:
        Name: "*private*"
      securityGroupSelector:
        karpenter.sh/discovery/${module.eks.cluster_id}: ${module.eks.cluster_id}
      tags:
        karpenter.sh/discovery/${module.eks.cluster_id}: ${module.eks.cluster_id}
    ttlSecondsAfterEmpty: 30
  YAML

  depends_on = [
    helm_release.karpenter
  ]
}
```

Since we've added a new provider (kubectl), you'll need to run `terraform init` again
before applying the changes to deploy the Karpenter provisioner.

```bash
terraform init
terraform apply
```

## First Use

Karpenter is now active and ready to begin provisioning nodes.
Create some pods using a deployment, and watch Karpenter provision nodes in response.

Before we can start interacting with the cluster, we need to update our local kubeconfig:

```bash
aws eks update-kubeconfig --name karpenter-demo
```

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
kubectl logs -f -n karpenter -l app.kubernetes.io/name=karpenter -c controller
```

### Automatic Node Termination

Now, delete the deployment. After 30 seconds (`ttlSecondsAfterEmpty`),
Karpenter should terminate the now empty nodes.

```bash
kubectl delete deployment inflate
kubectl logs -f -n karpenter -l app.kubernetes.io/name=karpenter -c controller
```

### Manual Node Termination

If you delete a node with kubectl, Karpenter will gracefully cordon, drain,
and shutdown the corresponding instance. Under the hood, Karpenter adds a
finalizer to the node object, which blocks deletion until all pods are
drained and the instance is terminated. Keep in mind, this only works for
nodes provisioned by Karpenter.

```bash
kubectl delete node "${NODE_NAME}"
```

## Cleanup

To avoid additional charges, remove the demo infrastructure from your AWS
account. Since Karpenter is managing nodes outside of Terraform's view, we need
to remove the pods and node first (if you haven't already). Once the node is
removed, you can remove the rest of the infrastructure and clean up Karpenter
created LaunchTemplates.

```bash
kubectl delete deployment inflate
kubectl delete node -l karpenter.sh/provisioner-name=default
terraform destroy
aws ec2 describe-launch-templates \
    | jq -r ".LaunchTemplates[].LaunchTemplateName" \
    | grep -i "Karpenter-karpenter-demo" \
    | xargs -I{} aws ec2 delete-launch-template --launch-template-name {}
```
