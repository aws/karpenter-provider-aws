---
title: "Migrating from Cluster Autoscaler"
linkTitle: "Migrating from Cluster Autoscaler"
weight: 10
---

This guide will show you how to switch from the [Kubernetes Cluster Autoscaler](https://github.com/kubernetes/autoscaler) to Karpenter for node provisioning.
We will make the following assumptions in this guide

* You will use an existing EKS cluster
* You will use existing VPC and subnets
* Your nodes are part of one or more node groups
* Your workloads have pod disruption budgets that adhere to [EKS best practices](https://aws.github.io/aws-eks-best-practices/karpenter/)
* Your cluster has an [OIDC provider](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html) for service accounts


## Create IAM roles

To get started with our migration we first need to create two new IAM roles for nodes provisioned with Karpenter and the Karpenter controller.

To create the Karpenter node role we will use the following policy and commands.

```bash
aws ...
```

Now we need to create an IAM role that the Karpenter controller will use to provision new instances.

```
# Get OIDC endpoint for trust relationship

# Create role

# Create trust
```

## Add tags to subnets

We need to add tags to our subnets so Karpenter will know which subnets to use.

```bash
aws eks get subnets
```

## Update aws-auth ConfigMap

We need to allow nodes that are using the node IAM role we just created to join the cluter.
To do that we have to modify the `aws-auth` ConfigMap in the cluster.

```
kubectl edit configmap aws-auth -n kube-system
```

Add a line to the MapRoles that looks something like this
```
blah
```

## Label existing node group

What labels already exist?

## Deploy Karpenter

We can now generate a full Karpenter deployment yaml from the helm chart.

```bash
helm template
```

Modify the following lines with information from our cluster.
Service account IAM role
Define minimum running instances
Set node affinity

```bash
kubectl create namespace karpenter
kubectl apply -f provisioner crd
kubectl apply -f karpenter.yaml
```

## Create provisioner

We need to create a default provisioner so Karpenter knows what types of nodes we want for unscheduled workloads.
You can refer to some of the [example provisioners](https://github.com/aws/karpenter/tree/main/examples/provisioner) for specific needs.

In this guide we'll use a provisioner without any restrictions.

```
kubectl apply -f 
```

## Set nodeAffinity for critical workloads

You may also want to set a nodeAffinity for other critical cluster workloads.

* coreDNS
* metric server

## Remove CAS

Now that karpenter is running we can disable the cluster autoscaler.

```bash
kubectl scale delpoy/cluster-autoscaler -n kube-system --replicas=0
```

And we can scale our nodegroup down to a minimum size to support Karpenter and other critical services.

```bash
aws asg ...
```

If you have a lot of nodes or workloads you may want to slowly step down your node groups to not inadvertidly cause an outage.
If your workloads have the necessary [pod disruption budgets](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) you should be safe to scale down the nodegroup, but it's still recommended to watch the transition carefully for workloads that may not have them configured.