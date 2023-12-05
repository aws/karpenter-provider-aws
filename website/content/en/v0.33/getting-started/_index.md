---
title: "Getting Started"
linkTitle: "Getting Started"
weight: 10
description: >
  Choose from different methods to get started with Karpenter
---


To get started with Karpenter, the [Getting Started with Karpenter]({{< relref "getting-started-with-karpenter" >}}) guide provides an end-to-end procedure for creating a cluster (with `eksctl`) and adding Karpenter.
If you prefer, the following instructions use Terraform to create a cluster and add Karpenter:

* [Amazon EKS Blueprints for Terraform](https://aws-ia.github.io/terraform-aws-eks-blueprints): Follow a basic [Getting Started](https://aws-ia.github.io/terraform-aws-eks-blueprints/getting-started/) guide and also add modules and add-ons. This includes a [Karpenter](https://aws-ia.github.io/terraform-aws-eks-blueprints/patterns/karpenter/) add-on that lets you bypass the instructions in this guide for setting up Karpenter.

Although not supported, you could also try Karpenter on other Kubernetes distributions running on AWS. For example:

* [kOps](https://kops.sigs.k8s.io/operations/karpenter/): These instructions describe how to create a kOps Kubernetes cluster in AWS that includes Karpenter.

Learn more about Karpenter and how to get started below.

* [Karpenter EKS Best Practices](https://aws.github.io/aws-eks-best-practices/karpenter/) guide
* [EC2 Spot Workshop for Karpenter](https://ec2spotworkshops.com/karpenter.html)
* [EKS Karpenter Workshop](https://www.eksworkshop.com/docs/autoscaling/compute/karpenter/)
* [Advanced EKS Immersion Karpenter Workshop](https://catalog.workshops.aws/eks-advanced/karpenter/)
* [Karpenter Blueprints](https://github.com/aws-samples/karpenter-blueprints)
* [Tutorial: Run Kubernetes Clusters for Less with Amazon EC2 Spot and Karpenter](https://community.aws/tutorials/run-kubernetes-clusters-for-less-with-amazon-ec2-spot-and-karpenter#step-6-optional-simulate-spot-interruption)
