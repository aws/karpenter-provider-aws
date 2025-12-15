---
title: "CloudFormation"
linkTitle: "CloudFormation"
weight: 5
description: >
  A description of the Getting Started CloudFormation file and permissions
---
The [Getting Started with Karpenter]({{< relref "../getting-started/getting-started-with-karpenter" >}}) guide uses CloudFormation to bootstrap the cluster to enable Karpenter to create and manage nodes, as well as to allow Karpenter to respond to interruption events.
This document describes the `cloudformation.yaml` file used in that guide.
These descriptions should allow you to understand:

* What Karpenter is authorized to do with your EKS cluster and AWS resources when using the `cloudformation.yaml` file
* What permissions you need to set up if you are adding Karpenter to an existing cluster

## Overview

To download a particular version of `cloudformation.yaml`, set the version and use `curl` to pull the file to your local system:

```bash
export KARPENTER_VERSION="1.7.4"
curl https://raw.githubusercontent.com/aws/karpenter-provider-aws/v"${KARPENTER_VERSION}"/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml > cloudformation.yaml
```

Following some header information, the rest of the `cloudformation.yaml` file describes the resources that CloudFormation deploys.
The sections of that file can be grouped together under the following general headings:

* [**Node Authorization**]({{< relref "#node-authorization" >}}): Creates a NodeInstanceProfile, attaches a NodeRole to it, and connects it to an IAM Identity Mapping used to authorize nodes to the cluster. This defines the permissions each node managed by Karpenter has to access EC2 and other AWS resources. This doesn't actually create the IAM Identity Mapping. That part is orchestrated by `eksctl` in the Getting Started guide.
* [**Controller Authorization**]({{< relref "#controller-authorization" >}}):  Creates the `KarpenterControllerPolicy` that is attached to the service account.
Again, the actual service account creation (`karpenter`), that is combined with the `KarpenterControllerPolicy`, is orchestrated by `eksctl` in the Getting Started guide.
* [**Interruption Handling**]({{< relref "#interruption-handling" >}}): Allows the Karpenter controller to see and respond to interruptions that occur with the nodes that Karpenter is managing. See the [Interruption]({{< relref "../concepts/disruption#interruption" >}}) section of the Disruption page for details.

A lot of the object naming that is done by `cloudformation.yaml` is based on the following:

* Cluster name: With a username of `bob` the Getting Started Guide would name your cluster `bob-karpenter-demo`
That name would then be appended to any name below where `${ClusterName}` is included.

* Partition: Any time an ARN is used, it includes the [partition name](https://docs.aws.amazon.com/whitepapers/latest/aws-fault-isolation-boundaries/partitions.html) to identify where the object is found. In most cases, that partition name is `aws`. However, it could also be `aws-cn` (for China Regions) or `aws-us-gov` (for AWS GovCloud US Regions).

## Node Authorization

The following sections of the `cloudformation.yaml` file set up IAM permissions for Kubernetes nodes created by Karpenter.
In particular, this involves setting up a node role that can be attached and passed to instance profiles that Karpenter generates at runtime:

* KarpenterNodeRole

### KarpenterNodeRole

This section of the template defines the IAM role attached to generated instance profiles.
Given a cluster name of `bob-karpenter-demo`, this role would end up being named `"KarpenterNodeRole-bob-karpenter-demo`.

```yaml
KarpenterNodeRole:
  Type: "AWS::IAM::Role"
  Properties:
    RoleName: !Sub "KarpenterNodeRole-${ClusterName}"
    Path: /
    AssumeRolePolicyDocument:
      Version: "2012-10-17"
      Statement:
        - Effect: Allow
          Principal:
            Service:
              !Sub "ec2.${AWS::URLSuffix}"
          Action:
            - "sts:AssumeRole"
    ManagedPolicyArns:
      - !Sub "arn:${AWS::Partition}:iam::aws:policy/AmazonEKS_CNI_Policy"
      - !Sub "arn:${AWS::Partition}:iam::aws:policy/AmazonEKSWorkerNodePolicy"
      - !Sub "arn:${AWS::Partition}:iam::aws:policy/AmazonEC2ContainerRegistryPullOnly"
      - !Sub "arn:${AWS::Partition}:iam::aws:policy/AmazonSSMManagedInstanceCore"
```

The role created here includes several AWS managed policies, which are designed to provide permissions for specific uses needed by the nodes to work with EC2 and other AWS resources. These include:

* [AmazonEKS_CNI_Policy](https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonEKS_CNI_Policy.html): Provides the permissions that the Amazon VPC CNI Plugin needs to configure EKS worker nodes.
* [AmazonEKSWorkerNodePolicy](https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonEKSWorkerNodePolicy.html): Lets Amazon EKS worker nodes connect to EKS Clusters.
* [AmazonEC2ContainerRegistryPullOnly](https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonEC2ContainerRegistryPullOnly.html): Allows pulling images from repositories in the Amazon EC2 Container Registry.
* [AmazonSSMManagedInstanceCore](https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonSSMManagedInstanceCore.html): Adds AWS Systems Manager service core functions for Amazon EC2.

If you were to use a node role from an existing cluster, you could skip this provisioning step and pass this node role to any EC2NodeClasses that you create. Additionally, you would ensure that the [Controller Policy]({{< relref "#controllerpolicy" >}}) has `iam:PassRole` permission to the role attached to the generated instance profiles.

## Controller Authorization

This section sets the AWS permissions for the Karpenter Controller. When used in the Getting Started guide, `eksctl` uses these permissions to create a service account (karpenter) that is combined with the KarpenterControllerPolicy.

The resources defined in this section are associated with:

* KarpenterControllerPolicy

Because the scope of the KarpenterControllerPolicy is an AWS region, the cluster's AWS region is included in the `AllowScopedEC2InstanceAccessActions`.

### KarpenterControllerPolicy

A `KarpenterControllerPolicy` object sets the name of the policy, then defines a set of resources and actions allowed for those resources.
For our example, the KarpenterControllerPolicy would be named: `KarpenterControllerPolicy-bob-karpenter-demo`

```yaml
KarpenterControllerPolicy:
  Type: AWS::IAM::ManagedPolicy
  Properties:
    ManagedPolicyName: !Sub "KarpenterControllerPolicy-${ClusterName}"
    # The PolicyDocument must be in JSON string format because we use a StringEquals condition that uses an interpolated
    # value in one of its key parameters which isn't natively supported by CloudFormation
    PolicyDocument: !Sub |
      {
        "Version": "2012-10-17",
        "Statement": [
```

Someone wanting to add Karpenter to an existing cluster, instead of using `cloudformation.yaml`, would need to create the IAM policy directly and assign that policy to the role leveraged by the service account using IRSA.

#### AllowScopedEC2InstanceAccessActions

The AllowScopedEC2InstanceAccessActions statement ID (Sid) identifies a set of EC2 resources that are allowed to be accessed with
[RunInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RunInstances.html) and [CreateFleet](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateFleet.html) actions.
For `RunInstances` and `CreateFleet` actions, the Karpenter controller can read (but not create) `image`, `snapshot`, `security-group`, `subnet` and `capacity-reservation` EC2 resources, scoped for the particular AWS partition and region.

```json
{
  "Sid": "AllowScopedEC2InstanceAccessActions",
  "Effect": "Allow",
  "Resource": [
    "arn:${AWS::Partition}:ec2:${AWS::Region}::image/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}::snapshot/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:security-group/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:subnet/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:capacity-reservation/*"
  ],
  "Action": [
    "ec2:RunInstances",
    "ec2:CreateFleet"
  ]
}
```

#### AllowScopedEC2LaunchTemplateAccessActions

The AllowScopedEC2InstanceAccessActions statement ID (Sid) identifies launch templates that are allowed to be accessed with
[RunInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RunInstances.html) and [CreateFleet](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateFleet.html) actions.
For `RunInstances` and `CreateFleet` actions, the Karpenter controller can read (but not create) `launch-template` EC2 resources that have the `kubernetes.io/cluster/${ClusterName}` tag be set to `owned` and a `karpenter.sh/nodepool` tag, scoped for the particular AWS partition and region. This ensures that an instance launch can't access launch templates that weren't provisioned by Karpenter.

```json
{
  "Sid": "AllowScopedEC2LaunchTemplateAccessActions",
  "Effect": "Allow",
  "Resource": "arn:${AWS::Partition}:ec2:${AWS::Region}:*:launch-template/*",
  "Action": [
    "ec2:RunInstances",
    "ec2:CreateFleet"
  ],
  "Condition": {
    "StringEquals": {
      "aws:ResourceTag/kubernetes.io/cluster/${ClusterName}": "owned"
    },
    "StringLike": {
      "aws:ResourceTag/karpenter.sh/nodepool": "*"
    }
  }
}
```

#### AllowScopedEC2InstanceActionsWithTags

The AllowScopedEC2InstanceActionsWithTags Sid allows the
[RunInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RunInstances.html), [CreateFleet](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateFleet.html), and [CreateLaunchTemplate](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateLaunchTemplate.html)
actions requested by the Karpenter controller to create all `fleet`, `instance`, `volume`, `network-interface`, `launch-template` or `spot-instances-request` EC2 resources (for the partition and region). It also requires that the `kubernetes.io/cluster/${ClusterName}` tag be set to `owned`, `aws:RequestTag/eks:eks-cluster-name` be set to `"${ClusterName}`, and a `karpenter.sh/nodepool` tag be set to any value. This ensures that Karpenter is only allowed to create instances for a single EKS cluster.

```json
{
  "Sid": "AllowScopedEC2InstanceActionsWithTags",
  "Effect": "Allow",
  "Resource": [
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:fleet/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:instance/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:volume/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:network-interface/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:launch-template/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:spot-instances-request/*"
  ],
  "Action": [
    "ec2:RunInstances",
    "ec2:CreateFleet",
    "ec2:CreateLaunchTemplate"
  ],
  "Condition": {
    "StringEquals": {
      "aws:RequestTag/kubernetes.io/cluster/${ClusterName}": "owned"
      "aws:RequestTag/eks:eks-cluster-name": "${ClusterName}"
    },
    "StringLike": {
      "aws:RequestTag/karpenter.sh/nodepool": "*"
    }
  }
}
```

#### AllowScopedResourceCreationTagging

The AllowScopedResourceCreationTagging Sid allows EC2 [CreateTags](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateTags.html)
actions on `fleet`, `instance`, `volume`, `network-interface`, `launch-template` and `spot-instances-request` resources, While making `RunInstance`, `CreateFleet`, or `CreateLaunchTemplate` calls. Additionally, this ensures that resources can't be tagged arbitrarily by Karpenter after they are created.
Conditions that must be met include that `aws:RequestTag/kubernetes.io/cluster/${ClusterName}` be set to `owned` and `aws:RequestTag/eks:eks-cluster-name` be set to `${ClusterName}`.

```json
{
  "Sid": "AllowScopedResourceCreationTagging",
  "Effect": "Allow",
  "Resource": [
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:fleet/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:instance/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:volume/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:network-interface/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:launch-template/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:spot-instances-request/*"
  ],
  "Action": "ec2:CreateTags",
  "Condition": {
    "StringEquals": {
      "aws:RequestTag/kubernetes.io/cluster/${ClusterName}": "owned",
      "aws:RequestTag/eks:eks-cluster-name": "${ClusterName}"
      "ec2:CreateAction": [
        "RunInstances",
        "CreateFleet",
        "CreateLaunchTemplate"
      ]
    },
    "StringLike": {
      "aws:RequestTag/karpenter.sh/nodepool": "*"
    }
  }
}
```

#### AllowScopedResourceTagging

The AllowScopedResourceTagging Sid allows EC2 [CreateTags](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateTags.html) actions on all instances created by Karpenter after their creation. It enforces that Karpenter is only able to update the tags on cluster instances it is operating on through the `kubernetes.io/cluster/${ClusterName}`" and `karpenter.sh/nodepool` tags.
Likewise, `RequestTag/eks:eks-cluster-name` must be set to `${ClusterName}`, if it exists, and `TagKeys` must equal `eks:eks-cluster-name`, `karpenter.sh/nodeclaim`, and `Name`, for all values.
```json
{
  "Sid": "AllowScopedResourceTagging",
  "Effect": "Allow",
  "Resource": "arn:${AWS::Partition}:ec2:${AWS::Region}:*:instance/*",
  "Action": "ec2:CreateTags",
  "Condition": {
    "StringEquals": {
      "aws:ResourceTag/kubernetes.io/cluster/${ClusterName}": "owned"
    },
    "StringLike": {
      "aws:ResourceTag/karpenter.sh/nodepool": "*"
    },
    "StringEqualsIfExists": {
      "aws:RequestTag/eks:eks-cluster-name": "${ClusterName}"
    },
    "ForAllValues:StringEquals": {
      "aws:TagKeys": [
        "eks:eks-cluster-name",
        "karpenter.sh/nodeclaim",
        "Name"
      ]
    }
  }
}
```

#### AllowScopedDeletion

The AllowScopedDeletion Sid allows [TerminateInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_TerminateInstances.html) and [DeleteLaunchTemplate](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DeleteLaunchTemplate.html) actions to delete instance and launch-template resources, provided that `karpenter.sh/nodepool` and `kubernetes.io/cluster/${ClusterName}` tags are set. These tags must be present on all resources that Karpenter is going to delete. This ensures that Karpenter can only delete instances and launch templates that are associated with it.

```json
{
  "Sid": "AllowScopedDeletion",
  "Effect": "Allow",
  "Resource": [
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:instance/*",
    "arn:${AWS::Partition}:ec2:${AWS::Region}:*:launch-template/*"
  ],
  "Action": [
    "ec2:TerminateInstances",
    "ec2:DeleteLaunchTemplate"
  ],
  "Condition": {
    "StringEquals": {
      "aws:ResourceTag/kubernetes.io/cluster/${ClusterName}": "owned"
    },
    "StringLike": {
      "aws:ResourceTag/karpenter.sh/nodepool": "*"
    }
  }
}
```

#### AllowRegionalReadActions

The AllowRegionalReadActions Sid allows [DescribeAvailabilityZones](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeAvailabilityZones.html), [DescribeImages](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeImages.html), [DescribeInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html), [DescribeInstanceTypeOfferings](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstanceTypeOfferings.html), [DescribeInstanceTypes](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstanceTypes.html), [DescribeLaunchTemplates](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeLaunchTemplates.html), [DescribeSecurityGroups](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSecurityGroups.html), [DescribeSpotPriceHistory](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSpotPriceHistory.html), and [DescribeSubnets](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSubnets.html) actions for the current AWS region.
This allows the Karpenter controller to do any of those read-only actions across all related resources for that AWS region.

```json
{
  "Sid": "AllowRegionalReadActions",
  "Effect": "Allow",
  "Resource": "*",
  "Action": [
    "ec2:DescribeCapacityReservations",
    "ec2:DescribeImages",
    "ec2:DescribeInstances",
    "ec2:DescribeInstanceTypeOfferings",
    "ec2:DescribeInstanceTypes",
    "ec2:DescribeLaunchTemplates",
    "ec2:DescribeSecurityGroups",
    "ec2:DescribeSpotPriceHistory",
    "ec2:DescribeSubnets"
  ],
  "Condition": {
    "StringEquals": {
      "aws:RequestedRegion": "${AWS::Region}"
    }
  }
}
```

#### AllowSSMReadActions

The AllowSSMReadActions Sid allows the Karpenter controller to get SSM parameters (`ssm:GetParameter`) from the current region for SSM parameters generated by AWS services.

**NOTE**: If potentially sensitive information is stored in SSM parameters, you could consider restricting access to these messages further.
```json
{
  "Sid": "AllowSSMReadActions",
  "Effect": "Allow",
  "Resource": "arn:${AWS::Partition}:ssm:${AWS::Region}::parameter/aws/service/*",
  "Action": "ssm:GetParameter"
}
```

#### AllowPricingReadActions

Because pricing information does not exist in every region at the moment, the AllowPricingReadActions Sid allows the Karpenter controller to get product pricing information (`pricing:GetProducts`) for all related resources across all regions.

```json
{
  "Sid": "AllowPricingReadActions",
  "Effect": "Allow",
  "Resource": "*",
  "Action": "pricing:GetProducts"
}
```

#### AllowInterruptionQueueActions

Karpenter supports interruption queues, that you can create as described in the [Interruption]({{< relref "../concepts/disruption#interruption" >}}) section of the Disruption page.
This section of the cloudformation.yaml template can give Karpenter permission to access those queues by specifying the resource ARN.
For the interruption queue you created (`${KarpenterInterruptionQueue.Arn}`), the AllowInterruptionQueueActions Sid lets the Karpenter controller have permission to delete messages ([DeleteMessage](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_DeleteMessage.html)), get queue URL ([GetQueueUrl](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_GetQueueUrl.html)), and receive messages ([ReceiveMessage](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_ReceiveMessage.html)).

```json
{
  "Sid": "AllowInterruptionQueueActions",
  "Effect": "Allow",
  "Resource": "${KarpenterInterruptionQueue.Arn}",
  "Action": [
    "sqs:DeleteMessage",
    "sqs:GetQueueUrl",
    "sqs:ReceiveMessage"
  ]
}
```

#### AllowPassingInstanceRole

The AllowPassingInstanceRole Sid gives the Karpenter controller permission to pass (`iam:PassRole`) the node role (`KarpenterNodeRole-${ClusterName}`) to generated instance profiles.
This gives EC2 permission explicit permission to use the `KarpenterNodeRole-${ClusterName}` when assigning permissions to generated instance profiles while launching nodes.

```json
{
  "Sid": "AllowPassingInstanceRole",
  "Effect": "Allow",
  "Resource": "${KarpenterNodeRole.Arn}",
  "Action": "iam:PassRole",
  "Condition": {
    "StringEquals": {
      "iam:PassedToService": [
        "ec2.amazonaws.com",
        "ec2.amazonaws.com.cn"
      ]
    }
  }
}
```

#### AllowScopedInstanceProfileCreationActions

The AllowScopedInstanceProfileCreationActions Sid gives the Karpenter controller permission to create a new instance profile with [`iam:CreateInstanceProfile`](https://docs.aws.amazon.com/IAM/latest/APIReference/API_CreateInstanceProfile.html),
provided that the request is made to a cluster with `RequestTag` `kubernetes.io/cluster/${ClusterName}` set to `owned`, the `eks:eks-cluster-name` set to `${ClusterName}`, and `topology.kubernetes.io/region` set to the current region.
Also, `karpenter.k8s.aws/ec2nodeclass` must be set to some value. This ensures that Karpenter can generate instance profiles on your behalf based on roles specified in your `EC2NodeClasses` that you use to configure Karpenter.

```json
{
  "Sid": "AllowScopedInstanceProfileCreationActions",
  "Effect": "Allow",
  "Resource": "arn:${AWS::Partition}:iam::${AWS::AccountId}:instance-profile/*",
  "Action": [
    "iam:CreateInstanceProfile"
  ],
  "Condition": {
    "StringEquals": {
      "aws:RequestTag/kubernetes.io/cluster/${ClusterName}": "owned",
      "aws:RequestTag/eks:eks-cluster-name": "${ClusterName}",
      "aws:RequestTag/topology.kubernetes.io/region": "${AWS::Region}"
    },
    "StringLike": {
      "aws:RequestTag/karpenter.k8s.aws/ec2nodeclass": "*"
    }
  }
}
```

#### AllowScopedInstanceProfileTagActions

The AllowScopedInstanceProfileTagActions Sid gives the Karpenter controller permission to tag an instance profile with [`iam:TagInstanceProfile`](https://docs.aws.amazon.com/IAM/latest/APIReference/API_TagInstanceProfile.html), provided that `ResourceTag` attributes `kubernetes.io/cluster/${ClusterName}` is set to `owned` and `topology.kubernetes.io/region` is set to the current region and `RequestTag` attributes `kubernetes.io/cluster/${ClusterName}` is set to `owned`, `eks:eks-cluster-name` is set to `${ClusterName}`, and `topology.kubernetes.io/region` is set to the current region.
Also, `ResourceTag/karpenter.k8s.aws/ec2nodeclass` and `RequestTag/karpenter.k8s.aws/ec2nodeclass` must be set to some value. This ensures that Karpenter is only able to act on instance profiles that it provisions for this cluster.

```json
{
  "Sid": "AllowScopedInstanceProfileTagActions",
  "Effect": "Allow",
  "Resource": "arn:${AWS::Partition}:iam::${AWS::AccountId}:instance-profile/*",
  "Action": [
    "iam:TagInstanceProfile"
  ],
  "Condition": {
    "StringEquals": {
      "aws:ResourceTag/kubernetes.io/cluster/${ClusterName}": "owned",
      "aws:ResourceTag/topology.kubernetes.io/region": "${AWS::Region}",
      "aws:RequestTag/kubernetes.io/cluster/${ClusterName}": "owned",
      "aws:RequestTag/eks:eks-cluster-name": "${ClusterName}",
      "aws:RequestTag/topology.kubernetes.io/region": "${AWS::Region}"
    },
    "StringLike": {
      "aws:ResourceTag/karpenter.k8s.aws/ec2nodeclass": "*",
      "aws:RequestTag/karpenter.k8s.aws/ec2nodeclass": "*"
    }
  }
}
```


#### AllowScopedInstanceProfileActions

The AllowScopedInstanceProfileActions Sid gives the Karpenter controller permission to perform [`iam:AddRoleToInstanceProfile`](https://docs.aws.amazon.com/IAM/latest/APIReference/API_AddRoleToInstanceProfile.html), [`iam:RemoveRoleFromInstanceProfile`](https://docs.aws.amazon.com/IAM/latest/APIReference/API_RemoveRoleFromInstanceProfile.html), and [`iam:DeleteInstanceProfile`](https://docs.aws.amazon.com/IAM/latest/APIReference/API_DeleteInstanceProfile.html) actions,
provided that the request is made to a cluster with `kubernetes.io/cluster/${ClusterName}` set to owned and is made in the current region.
Also, `karpenter.k8s.aws/ec2nodeclass` must be set to some value. This permission is further enforced by the `iam:PassRole` permission. If Karpenter attempts to add a role to an instance profile that it doesn't have `iam:PassRole` permission on, that call will fail. Therefore, if you configure Karpenter to use a new role through the `EC2NodeClass`, ensure that you also specify that role within your `iam:PassRole` permission.

```json
{
  "Sid": "AllowScopedInstanceProfileActions",
  "Effect": "Allow",
  "Resource": "arn:${AWS::Partition}:iam::${AWS::AccountId}:instance-profile/*",
  "Action": [
    "iam:AddRoleToInstanceProfile",
    "iam:RemoveRoleFromInstanceProfile",
    "iam:DeleteInstanceProfile"
  ],
  "Condition": {
    "StringEquals": {
      "aws:ResourceTag/kubernetes.io/cluster/${ClusterName}": "owned",
      "aws:ResourceTag/topology.kubernetes.io/region": "${AWS::Region}"
    },
    "StringLike": {
      "aws:ResourceTag/karpenter.k8s.aws/ec2nodeclass": "*"
    }
  }
}
```

#### AllowInstanceProfileReadActions

The AllowInstanceProfileReadActions Sid gives the Karpenter controller permission to perform [`iam:GetInstanceProfile`](https://docs.aws.amazon.com/IAM/latest/APIReference/API_GetInstanceProfile.html) actions to retrieve information about a specified instance profile, including understanding if an instance profile has been provisioned for an `EC2NodeClass` or needs to be re-provisioned.

```json
{
  "Sid": "AllowInstanceProfileReadActions",
  "Effect": "Allow",
  "Resource": "arn:${AWS::Partition}:iam::${AWS::AccountId}:instance-profile/*",
  "Action": "iam:GetInstanceProfile"
}
```

#### AllowAPIServerEndpointDiscovery

You can optionally allow the Karpenter controller to discover the Kubernetes cluster's external API endpoint to enable EC2 nodes to successfully join the EKS cluster.

> **Note**: If you are not using an EKS control plane, you will have to specify this endpoint explicitly. See the description of the `aws.clusterEndpoint` setting in the [ConfigMap](.settings/#configmap) documentation for details.

The AllowAPIServerEndpointDiscovery Sid allows the Karpenter controller to get that information (`eks:DescribeCluster`) for the cluster (`cluster/${ClusterName}`).
```json
{
  "Sid": "AllowAPIServerEndpointDiscovery",
  "Effect": "Allow",
  "Resource": "arn:${AWS::Partition}:eks:${AWS::Region}:${AWS::AccountId}:cluster/${ClusterName}",
  "Action": "eks:DescribeCluster"
}
```

## Interruption Handling

Settings in this section allow the Karpenter controller to stand-up an interruption queue to receive notification messages from other AWS services about the health and status of instances. For example, this interruption queue allows Karpenter to be aware of spot instance interruptions that are sent 2 minutes before spot instances are reclaimed by EC2. Adding this queue allows Karpenter to be proactive in migrating workloads to new nodes.
See the [Interruption]({{< relref "../concepts/disruption#interruption" >}}) section of the Disruption page for details.

Defining the `KarpenterInterruptionQueuePolicy` allows Karpenter to see and respond to the following:

* AWS health events
* Spot interruptions
* Spot rebalance recommendations
* Instance state changes

The resources defined in this section include:

* KarpenterInterruptionQueue
* KarpenterInterruptionQueuePolicy
* ScheduledChangeRule
* SpotInterruptionRule
* RebalanceRule
* InstanceStateChangeRule

### KarpenterInterruptionQueue

The [AWS::SQS::Queue](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-sqs-queue.html) resource is used to create an Amazon SQS standard queue.
Properties of that resource set the `QueueName` to the name of your cluster, the time for which SQS retains each message (`MessageRetentionPeriod`) to 300 seconds, and enabling serverside-side encryption using SQS owned encryption keys (`SqsManagedSseEnabled`) to `true`.
See [SetQueueAttributes](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_SetQueueAttributes.html) for descriptions of some of these attributes.

```yaml
KarpenterInterruptionQueue:
  Type: AWS::SQS::Queue
  Properties:
    QueueName: !Sub "${ClusterName}"
    MessageRetentionPeriod: 300
    SqsManagedSseEnabled: true
```

### KarpenterInterruptionQueuePolicy

The Karpenter interruption queue policy is created to allow AWS services that we want to receive instance notifications from to push notification messages to the queue.
The [AWS::SQS::QueuePolicy](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-sqs-queuepolicy.html) resource here applies `EC2InterruptionPolicy` to the `KarpenterInterruptionQueue`. The policy allows [sqs:SendMessage](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_SendMessage.html) actions to `events.amazonaws.com` and `sqs.amazonaws.com` services. It also allows the `GetAtt` function to get attributes from `KarpenterInterruptionQueue.Arn`.
Additionally, it only allows access to the queue using encrypted connections over HTTPS (TLS) to adhere to [Amazon SQS Security Best Practices](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-security-best-practices.html#enforce-encryption-data-in-transit).

```yaml
KarpenterInterruptionQueuePolicy:
  Type: AWS::SQS::QueuePolicy
  Properties:
    Queues:
      - !Ref KarpenterInterruptionQueue
    PolicyDocument:
      Id: EC2InterruptionPolicy
      Statement:
        - Effect: Allow
          Principal:
            Service:
              - events.amazonaws.com
              - sqs.amazonaws.com
          Action: sqs:SendMessage
          Resource: !GetAtt KarpenterInterruptionQueue.Arn
        - Sid: DenyHTTP
          Effect: Deny
          Action: sqs:*
          Resource: !GetAtt KarpenterInterruptionQueue.Arn
          Condition:
            Bool:
              aws:SecureTransport: false
          Principal: "*"
```

### Rules

This section allows Karpenter to gather [AWS Health Events](https://docs.aws.amazon.com/health/latest/ug/cloudwatch-events-health.html#about-public-events) and direct them to a queue where they can be consumed by Karpenter.
These rules include:

* ScheduledChangeRule: The [AWS::Events::Rule](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-events-rule.html) creates a rule where the [EventPattern](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-event-patterns.html) is set to send events from the `aws.health` source to `KarpenterInterruptionQueue`.

  ```yaml
  ScheduledChangeRule:
    Type: 'AWS::Events::Rule'
    Properties:
     EventPattern:
       source:
         - aws.health
       detail-type:
         - AWS Health Event
     Targets:
       - Id: KarpenterInterruptionQueueTarget
         Arn: !GetAtt KarpenterInterruptionQueue.Arn
  ```

* SpotInterruptionRule: An EC2 Spot Instance Interruption warning tells you that AWS is about to reclaim a Spot instance you are using. This rule allows Karpenter to gather [EC2 Spot Instance Interruption Warning](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html) events and direct them to a queue where they can be consumed by Karpenter. In particular, the [AWS::Events::Rule](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-events-rule.html) here creates a rule where the [EventPattern](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-event-patterns.html) is set to send events from the `aws.ec2` source to `KarpenterInterruptionQueue`.

  ```yaml
  SpotInterruptionRule:
    Type: 'AWS::Events::Rule'
    Properties:
      EventPattern:
        source:
          - aws.ec2
        detail-type:
          - EC2 Spot Instance Interruption Warning
      Targets:
        - Id: KarpenterInterruptionQueueTarget
          Arn: !GetAtt KarpenterInterruptionQueue.Arn
  ```

* RebalanceRule: An EC2 Instance Rebalance Recommendation signal tells you that a Spot instance is at a heightened risk of being interrupted, allowing Karpenter to get new instances or simply rebalance workloads.  This rule allows Karpenter to gather [EC2 Instance Rebalance Recommendation](https://docs.aws.amazon.com/AWSEC2/latest/WindowsGuide/rebalance-recommendations.html) signals and direct them to a queue where they can be consumed by Karpenter. In particular, the [AWS::Events::Rule](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-events-rule.html) here creates a rule where the [EventPattern](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-event-patterns.html) is set to send events from the `aws.ec2` source to `KarpenterInterruptionQueue`.

  ```yaml
  RebalanceRule:
   Type: 'AWS::Events::Rule'
   Properties:
     EventPattern:
       source:
         - aws.ec2
       detail-type:
         - EC2 Instance Rebalance Recommendation
     Targets:
       - Id: KarpenterInterruptionQueueTarget
         Arn: !GetAtt KarpenterInterruptionQueue.Arn
  ```

* InstanceStateChangeRule: An EC2 Instance State-change Notification signal tells you that the state of an instance has changed to one of the following states: pending, running, stopping, stopped, shutting-down, or terminated. This rule allows Karpenter to gather [EC2 Instance State-change](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/monitoring-instance-state-changes.html) signals and direct them to a queue where they can be consumed by Karpenter. In particular, the [AWS::Events::Rule](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-events-rule.html) here creates a rule where the [EventPattern](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-event-patterns.html) is set to send events from the `aws.ec2` source to `KarpenterInterruptionQueue`.

  ```yaml
  InstanceStateChangeRule:
   Type: 'AWS::Events::Rule'
   Properties:
     EventPattern:
       source:
         - aws.ec2
       detail-type:
         - EC2 Instance State-change Notification
     Targets:
       - Id: KarpenterInterruptionQueueTarget
         Arn: !GetAtt KarpenterInterruptionQueue.Arn
  ```
