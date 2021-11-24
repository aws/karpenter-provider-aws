---
title: "Deployment Guide"
linkTitle: "Deployment Guide"
weight: 50
---

Introducing new software the modifies how Kubernetes operates is complex, and no single solution exists. 

This guide describes how Karpenter interacts with external resources, and modifies cluster behavior. 

## Resources Dependencies

### Cluster on AWS

Karpenter creates nodes (EC2 Instances) through the AWS cloud provider, the only such module implemented.
Node provisioning on other providers is not currently supported.
If you would like to use Karpenter with other infrastructure providers please open an issue to let us know your use case.

Theoretically, a control plane could be hosted separately from Karpenter provisioned AWS instances. However, this complicates identity and authorization substantially. The Karpenter controller (i.e., agent) would need authorization with AWS APIs to provision new instances, and permissions with the control plane to add the new nodes. 

Additionally, the intricacies of securing control plane communication between clouds presents another significant problem. 

The getting started guide covers [configuring EKS to accept newly provisioned instances as nodes](../getting-started/#create-the-karpenternode-iam-role) (see, *KarpenterNodeRole* and *system:bootstrappers*). If using a self-managed (or alternatively provided) control plane, this must be manually handled. Consult the [*User Data - Autoconfigure* section](../cloud-providers/aws/launch-templates/#user-data---autoconfigure) of *Launch Templates* for more information on joining nodes to a cluster.

### Minimum Kubernetes Version

Karpenter supports the [same versions of Kubernetes as EKS](https://docs.aws.amazon.com/eks/latest/userguide/kubernetes-versions.html).
You can also use Karpenter with non-EKS clusters so long as you configure the correct IAM permissions and use a supported Kubernetes version.
Karpenter strongly encourages promptly adopting new versions of Kubernetes as new constraints and node features are added regularly.


### Service Account
    - `Karpenter` Service Account

The `Karpenter` service account needs sufficient cluster permissions to, for example, add new nodes and bind pods to nodes. Review the ["karpenter-controller" `role` and `ClusterRole`](rbac.yaml) from the default helm chart to understand what permissions the Karpenter service account needs. The Karpenter helm chart includes other roles (`karpenter-webhook`), but the "karpenter-controller" resources require uniquely broad access.

In part:

```
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: karpenter-controller
rules:
[...]
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - create
[...]
```


### IAM Roles
- `Karpenter` Role
- `KarpenterNode` Role

Karpenter needs to authenticate with the AWS EC2 API to create and delete instances.
It is recommended to do this with [IAM Roles for Service Accounts (IRSA)](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html).
The getting started guide describes creating a cluster service account mapped to an IAM role with sufficient permissions. 


Karpenter may alternatively be configured to authenticate with AWS using the `kube2iam` package, or with locally stored Access Keys. Both practices are discouraged. 

<<<<<<< HEAD:website/content/en/pre-docs/deployment-guide/_index.md
The provisioned instances must have comparable permissions to a standard EKS self-managed node. For example, permission to configure instance networking interfaces. The getting started guide again provides a useful starting point in [the cloudformation template](../getting-started/cloudformation.yaml).
=======
The provisioned instances must comparable permissions to a standard EKS self-managed node. For example, permission to configure instance networking interfaces. The getting started guide again provides a useful starting point in [the CloudFormation template](../getting-started/cloudformation.yaml).

>>>>>>> 886d3a1 (Apply suggestions from code review):website/content/en/pre-docs/deploy-guide/_index.md

### Tagged Subnets

Karpenter needs to discover VPC (virtual private cloud) subnets on AWS, such that requests for new instances are places on the proper subnet, enabling communication with the control plane. Karpenter discovers subnets using resource tags. 

The tags must have this form:
- key: `kubernetes.io/cluster/${CLUSTER_NAME}`
- value: "" (empty string)

In some situations, EKS may automatically apply this tag to subnets. Do not rely on EKS to do this. 

### `eksctl`

The getting started guide uses `eksctl`. This tool is officially supported and sponsored by AWS. However, equivalent EKS clusters may be created using CloudFormation, Terraform, or even the web console. 


The ["cluster.yaml" file](../getting-started/#create-a-cluster) provides a suitable `eksctl` cluster configuration for Karpenter, including an OIDC provider for IAM (a requirement for IRSA). 


`eksctl` also handles interconnecting Kubernetes RBAC and AWS IAM. For example, associating the `karpenter` service account with an IAM role for creating instances. 

```
eksctl create iamserviceaccount \
  --cluster $CLUSTER_NAME --name karpenter --namespace karpenter \
  --attach-policy-arn arn:aws:iam::$AWS_ACCOUNT_ID:policy/KarpenterControllerPolicy-$CLUSTER_NAME \
  --approve
```

```
eksctl create iamidentitymapping \
  --username system:node:{{EC2PrivateDNSName}} \
  --cluster  ${CLUSTER_NAME} \
  --arn arn:aws:iam::${AWS_ACCOUNT_ID}:role/KarpenterNodeRole-${CLUSTER_NAME} \
  --group system:bootstrappers \
  --group system:nodes
```


### Launch Template

A launch template is an AWS resource that bundles every configuration value needed to properly launch a new instance, such as base image and storage.

Karpenter will create a launch template suitable for the environment described in the getting started guide (EKS, IRSA, etc) if none is specified.

To the extent possible, nodes should be customized using Karpenter provisioner CRD fields. However, many environments require using a different base image (instead of the EKS optimized flavor of Amazon Linux 2), such as to incorporate a standardized security daemon at the node level. 

Review the [detailed instructions](../cloud-providers/aws/launch-templates/) for creating a custom launch template. 

## Deploy Karpenter

The yaml files comprising the helm chart are suitable for production environments. They may be lightly modified for direct use, for example storage in an internal GitOps cluster repo (without referencing an external chart).

The general structure of the helm chart and described resources:

- [Helm Chart](https://github.com/awslabs/karpenter/tree/main/charts/karpenter)
    - `Provisioner` CRD
    - `Karpenter` Service Account
    - Controller Deployment
        - Karpenter Controller Image from ECR
        - RBAC for Karpenter Service Account
    - Webhook Deployment
        - Karpenter Webhook Image from ECR
        - RBAC for Karpenter Service Account
        - Webhooks (Mutating, Validating)
    - Default Provisioner
    - Logging Configuration

Review [`values.yaml`](https://github.com/awslabs/karpenter/blob/main/charts/karpenter/values.yaml) for helm chart configuration options.

Some important fields are:
- serviceAccount.create
- serviceAccount.name
- controller.clusterName
- controller.clusterEndpoint

## Mutations of Kubernetes Behavior
Note: This section is an incomplete draft.

- `kubectl delete node`

    - Deleting a karpenter managed node triggers deprovisioning of the cloud instance.
- High amount of pods on a single node
    - Karpenter often decides to use a large instance, and pack many pods onto the instance. 
    - This may increase the load on *per node* daemonsets
- Proactive Scheduling
    - After Karpenter initiates the provisioning of a new node with a cloud provider, it proactively binds/schedules pending pods to the forthcoming node.
- End of Node Lifecycle
    - Karpenter places a finalizer on nodes it manages. Nodes are not fully terminated until this label is removed. If the Karpenter Controller fails or is forcefully removed, nodes may be unable to fully terminate.
- Node Expiration
    - Karpenter implements node expiry. When enabled, Karpenter terminates nodes when a timer *set per unique node* terminates. This timer is started when the node joins the cluster. Karpenter managed nodes may terminate for this unusual (but predictable) reason. The natural alignment/skew of node expiration events has not been extensively analyzed. Consider that contemporaneously launched nodes will expire at similarly close times. 
- Other Topics
    - Pod Disruption Budget
    - Delete empty nodes

## Warnings

- Reallocation
    - Karpenter does not directly implement reallocation. 
    - Consider the following example:
        - New Empty Cluster
        - Karpenter Controller running on Fargate
        - New workload with 100 Pods
        - Karpenter provisions a single large instance
        - External factors reduce needed pods to 50
        - 50 pods terminated
        - Only 50 pods running on the remaining over resourced instance
    - Mitigations
        - Configure node expiry. The expiration of nodes triggers Karpenter to calculate a new instance to provision.

