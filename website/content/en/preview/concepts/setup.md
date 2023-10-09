---
title: "Karpenter CloudFormation Setup"
linkTitle: "CloudFormation Setup"
weight: 5
description: >
  Descriptions of how Karpenter uses CloudFormation to set up permissions
---
When you create a cluster to use with Karpenter in the [Getting Started with Karpenter]({{< relref "../getting-started/getting-started-with-karpenter" >}}) guide, the procedure uses CloudFormation to prepare the cluster for Karpenter to be able to create and manage nodes, as well as gather and respond to interruption events.
This document describes the `cloudformation.yaml` file used in that guide.
These descriptions will be useful to understand:

* What Karpenter is authorized to do with your EKS cluster and AWS resources when you use the `cloudformation.yaml` file
* What permissions you need to set up if you are adding Karpenter to an existing cluster

# Review the cloudformation.yaml file

To download a particular version of `cloudformation.yaml`, set the version and use `curl` to pull the file to your local system:

```bash
export KARPENTER_VERSION=v0.29.0
curl https://raw.githubusercontent.com/aws/karpenter/"${KARPENTER_VERSION}"/website/content/en/preview/getting-started/getting-started-with-karpenter/cloudformation.yaml > cloudformation.yaml
```

The `cloudformation.yaml` file starts with the following information:

```
AWSTemplateFormatVersion: "2010-09-09"
Description: Resources used by https://github.com/aws/karpenter
Parameters:
  ClusterName:
    Type: String
    Description: "EKS cluster name"
Resources:
```

The rest of the `cloudformation.yaml` file describes the resources that CloudFormation deploys.
The sections of that file can be grouped together under the following general headings:

* **Node Authorization**: Creates a NodeInstanceProfile, attaches a NodeRole to it, and connects it to an IAM Identity Mapping that Karpenter uses. This defines the permissions each node managed by Karpenter has to access EC2 and other AWS resources.
* **Karpenter Controller Authorization**:  Creates a service account (named `karpenter`) that is combined with the KarpenterControllerPolicy. The KarpenterControllerPolicy allows the Karpenter controller to control assets within EC2 specifically and within AWS in general.
* **Interruption Handling**: The interruption handling sections of this file allow the Karpenter controller to see and respond to interruptions that occur with the nodes that Karpenter is managing. Interruptions can reflect things like a spot instance going away or physical hardware going down. Allowing the Karpenter controller to see these interruptions allows Karpenter to respond by bringing nodes up and down and moving workloads.

A lot of the object naming that is done by `cloudformation.yaml` is based on the following:

* Cluster name: With a user name of `bob` the Getting Started Guide would name your cluster `bob-karpenter-demo`
That name would then be appended to any name below where `${ClusterName}` is included.

* Partition: Any time an ARN is used, it includes the partition name to identify where the object is found. In most cases, that partition name is `aws`. However, it could also be `aws-cn` (for China Regions) or `aws-us-gov` (for AWS GovCloud US Regions).

# Node Authorization 

The following sections of the `cloudformation.yaml` file set up permissions related to what Kubernetes nodes created by Karpenter can do with EC2 and other AWS features.
In particular, this involves setting up an instance profile and attaching a node role to that profile with the following objects:

* KarpenterNodeInstanceProfile
* KarpenterNodeRole

## KarpenterNodeInstanceProfile
This section creates an [EC2 Instance Profile](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html) that includes the node role named `KarpenterNodeRole`, with the cluster name appended.
For example, with a cluster name of `bob-karpenter-demo`, the instance profile name would look like:

`KarpenterNodeInstanceProfile-bob-karpenter-demo`


```
  KarpenterNodeInstanceProfile:
    Type: "AWS::IAM::InstanceProfile"
    Properties:
      InstanceProfileName: !Sub "KarpenterNodeInstanceProfile-${ClusterName}"
      Path: "/"
      Roles:
        - !Ref "KarpenterNodeRole"
```
To do this manually for an existing cluster, you would find your cluster's InstanceProfileName and use that to attach the role needed.
To list all instance profiles, type:

```bash
aws iam list-instance-profiles
```

To see information for a particular instance profile (for example, `MyRoleForInstances`), type:

```bash
aws iam get-instance-profile --instance-profile-name "MyRoleForInstances"
```
```
INSTANCEPROFILE arn:aws:iam::973227887653:instance-profile/MyRoleForInstances  2020-12-10T19:45:31+00:00       AIPA6FGHHBQS4F2FFHYYV   MyRoleForInstances     /
ROLES   arn:aws:iam::111111111111:role/MyRoleForInstances      2020-12-10T19:45:30+00:00       /       AROA6FGHHBQSQ5NKPJT2H   MyRoleForInstances
ASSUMEROLEPOLICYDOCUMENT        2012-10-17
STATEMENT       sts:AssumeRole  Allow
PRINCIPAL       ec2.amazonaws.com
```

## KarpenterNodeRole

This section creates the node role that is attached to the `KarpenterNodeInstanceProfile` instance profile created earlier.
Given a cluster name of `bob-karpenter-demo`, this role would end up being named `"KarpenterNodeRole-bob-karpenter-demo`.

```
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
        - !Sub "arn:${AWS::Partition}:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
        - !Sub "arn:${AWS::Partition}:iam::aws:policy/AmazonSSMManagedInstanceCore"
```

The role created here includes several AWS managed policies, which are designed to provide permissions for specific uses needed by the nodes to work with EC2 and other AWS resources. These include:

* [AmazonEKS_CNI_Policy](https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonEKS_CNI_Policy.html): Provides the permissions that the Amazon VPC CNI Plugin needs to configure EKS worker nodes.
* [AmazonEKSWorkerNodePolicy](https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonEKSWorkerNodePolicy.html): Lets Amazon EKS worker nodes connect to EKS Clusters.
* [AmazonEC2ContainerRegistryReadOnly](https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonEC2ContainerRegistryReadOnly.html): Allows read-only access to repositories in the Amazon EC2 Container Registry.
* [AmazonSSMManagedInstanceCore](https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonSSMManagedInstanceCore.html): Adds AWS Systems Manager service core functions for Amazon EC2.

# Karpenter Controller Authorization 

This section sets the permissions that the Karpenter Controller has to create and manage EC2 and other AWS resources.
In particular, the section creates a service account (karpenter) that is combined with the KarpenterControllerPolicy.
The permissions here go beyond the permissions assigned to Karpenter nodes in the previous section.

The resources defined in this section are associated with:

* KarpenterControllerPolicy

Because the scope of the KarpenterControllerPolicy is an AWS region, the cluster's AWS region is included in the `AllowScopedEC2InstanceActions`.

## KarpenterControllerPolicy

A `KarpenterControllerPolicy` object sets the name of the policy, then defines a set of resources and actions allowed for those resources.
The policy creates authorization for the Karpenter Controller to AWS resources, instead of using the NodeInstanceProfile which probably has less permissions to AWS resources.
For our example, the KarpenterControllerPolicy would be named: `KarpenterControllerPolicy-bob-karpenter-demo`

```
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
Someone wanting to add Karpenter to an existing cluster, instead of using `cloudformation.yaml`, would need to find a way to create the policy and assign that policy to the service account to use IRSA.

### AllowScopedEC2InstanceActions

The AllowScopedEC2InstanceActions statement ID (Sid) identifies a set of EC2 resources that are allowed to be used with
[RunInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RunInstances.html) and [CreateFleet](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateFleet.html) actions.
For `RunInstances` and `CreateFleet` actions, the Karpenter controller can access `image`, `snapshot`, `spot-instances-request`, `security-group`, `subnet` and `launch-template` EC2 resources, scoped for the particular AWS partition and region.

```
            {
              "Sid": "AllowScopedEC2InstanceActions",
              "Effect": "Allow",
              "Resource": [
                "arn:${AWS::Partition}:ec2:${AWS::Region}::image/*",
                "arn:${AWS::Partition}:ec2:${AWS::Region}::snapshot/*",
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:spot-instances-request/*",
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:security-group/*",
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:subnet/*",
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:launch-template/*"
              ],
              "Action": [
                "ec2:RunInstances",
                "ec2:CreateFleet"
              ]
            },
```

### AllowScopedEC2InstanceActionsWithTags
The AllowScopedEC2InstanceActionsWithTags Sid allows the 
[RunInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RunInstances.html), [CreateFleet](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateFleet.html), and [CreateLaunchTemplate](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateLaunchTemplate.html)
actions requested by the Karpenter controller to access all `fleet`, `instance`, `volume`, `network-interface`, or `launch-template` EC2 resources (for the partition and region), and requires that the `kubernetes.io/cluster/${ClusterName}` tag be set to `owned` and a `karpenter.sh/nodepool` tag be set to any value with these actions.
This makes sure that these resources that are managed by the Karpenter controller are assigned these tags.

```
            {
              "Sid": "AllowScopedEC2InstanceActionsWithTags",
              "Effect": "Allow",
              "Resource": [
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:fleet/*",
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:instance/*",
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:volume/*",
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:network-interface/*",
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:launch-template/*"
              ],
              "Action": [
                "ec2:RunInstances",
                "ec2:CreateFleet",
                "ec2:CreateLaunchTemplate"
              ],
              "Condition": {
                "StringEquals": {
                  "aws:RequestTag/kubernetes.io/cluster/${ClusterName}": "owned"
                },
                "StringLike": {
                  "aws:RequestTag/karpenter.sh/nodepool": "*"
                }
              }
            },

```

### AllowScopedResourceCreationTagging
The AllowScopedResourceCreationTagging Sid allows EC2 [CreateTags](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateTags.html)
actions on `fleet`, `instance`, `volume`, `network-interface`, and `launch-template` resources, provided that a `CreateAction` of `RunInstance`, `CreateFleet`, or `CreateLaunchTemplate`
has been run, the `kubernetes.io/cluster/${ClusterName}` tag is set to `owned`, and the `karpenter.sh/nodepool` tag is set to any value with these actions.
```
            {
              "Sid": "AllowScopedResourceCreationTagging",
              "Effect": "Allow",
              "Resource": [
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:fleet/*",
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:instance/*",
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:volume/*",
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:network-interface/*",
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:launch-template/*"
              ],
              "Action": "ec2:CreateTags",
              "Condition": {
                "StringEquals": {
                  "aws:RequestTag/kubernetes.io/cluster/${ClusterName}": "owned",
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
            },
```

### AllowScopedResourceTagging
The AllowScopedResourceTagging Sid allows EC2 [CreateTags](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateTags.html) actions on instance resources associated with node migrations.
These tags are set to indicate that a successful migration has occurred.
With this action, the `karpenter.sh/cluster/${ClusterName}` tag is set to `owned`.
Likewise, the `karpenter.sh/nodepool` tag must be set to some value and any values can be set for `karpenter.k8s.aws/nodeclaim`.
```
            {
              "Sid": "AllowScopedResourceTagging",
              "Effect": "Allow",
              "Resource": "arn:${AWS::Partition}:ec2:${AWS::Region}:*:instance/*",
              "Action": "ec2:CreateTags",
              "Condition": {
                "StringEquals": {
                  "aws:ResourceTag/karpenter.sh/cluster/${ClusterName}": "owned"
                },
                "StringLike": {
                  "aws:ResourceTag/karpenter.sh/nodepool": "*"
                },
                "ForAllValues:StringEquals": {
                  "aws:TagKeys": [
                    "karpenter.k8s.aws/nodeclaim",
                    "Name"
                  ]
                }
              }
            },
```

### AllowScopedDeletion
The AllowScopedDeletion Sid allows [TerminateInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_TerminateInstances.html) and [DeleteLaunchTemplate](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DeleteLaunchTemplate.html) actions to delete instance and launch-template resources, provided that `karpenter.sh/nodepool` and `kubernetes.io/cluster/${ClusterName}` tags are set.

```
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
            },
```

### AllowRegionalReadActions

The AllowRegionalReadActions Sid allows [DescribeAvailabilityZones](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeAvailabilityZones.html), [DescribeImages](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeImages.html), [DescribeInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html), [DescribeInstanceTypeOfferings](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstanceTypeOfferings.html), [DescribeInstanceTypes](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstanceTypes.html), [DescribeLaunchTemplates](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeLaunchTemplates.html), [DescribeSecurityGroups](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSecurityGroups.html), [DescribeSpotPriceHistory](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSpotPriceHistory.html), and [DescribeSubnets](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSubnets.html) actions for the current AWS region.
This allows the Karpenter controller to do any of those read-only actions across all related resources for that AWS region.

```
            {
              "Sid": "AllowRegionalReadActions",
              "Effect": "Allow",
              "Resource": "*",
              "Action": [
                "ec2:DescribeAvailabilityZones",
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
            },
```

### AllowSSMReadActions
The AllowSSMReadActions Sid allows the Karpenter controller to read SSM parameters (`ssm:GetParameter`) from the current region.

**NOTE**: If potentially sensitive information is stored in SSM parameters, you could consider restricting access to these messages further.
```
            {
              "Sid": "AllowSSMReadActions",
              "Effect": "Allow",
              "Resource": "arn:${AWS::Partition}:ssm:${AWS::Region}::parameter/aws/service/*",
              "Action": "ssm:GetParameter"
            },
```

### AllowPricingReadActions
Because pricing information does not exist in every region at the moment, the AllowPricingReadActions Sid allows the Karpenter controller to get product pricing information (`pricing:GetProducts`) for all related resources across all regions.

```
            {
              "Sid": "AllowPricingReadActions",
              "Effect": "Allow",
              "Resource": "*",
              "Action": "pricing:GetProducts"
            },
```

### AllowInterruptionQueueActions
For the interruption queue you created (`${KarepenterInterruptionQueue.Arn}`), the AllowInterruptionQueueActions Sid lets the Karpenter controller have permission to delete messages ([DeleteMessage](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_DeleteMessage.html)), get queue attributes ([GetQueueAttributes](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_GetQueueAttributes.html)), get queue URL ([GetQueueUrl](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_GetQueueUrl.html)), and receive messages ([ReceiveMessage](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_ReceiveMessage.html)).

```
            {
              "Sid": "AllowInterruptionQueueActions",
              "Effect": "Allow",
              "Resource": "${KarpenterInterruptionQueue.Arn}",
              "Action": [
                "sqs:DeleteMessage",
                "sqs:GetQueueAttributes",
                "sqs:GetQueueUrl",
                "sqs:ReceiveMessage"
              ]
            },
```

### AllowPassingInstanceRole
The AllowPassingInstanceRole Sid gives the Karpenter controller permission to pass (`iam:PassRole`) the node role (`KarpenterNodeRole-${ClusterName}`) to the instance profile.
This allows EC2 to check those permissions when making an EC2 instance call, such as an EC2 launch call.
The `iam:PassedToService` restricts the permission to make these requests to only the EC2 service (`ec2.amazonaws.com`).

```
            {
              "Sid": "AllowPassingInstanceRole",
              "Effect": "Allow",
              "Resource": "arn:${AWS::Partition}:iam::${AWS::AccountId}:role/KarpenterNodeRole-${ClusterName}",
              "Action": "iam:PassRole",
              "Condition": {
                "StringEquals": {
                  "iam:PassedToService": "ec2.amazonaws.com"
                }
              }
            },
```

### AllowScopedInstanceProfileCreationActions
The AllowScopedInstanceProfileCreationActions Sid gives the Karpenter controller permission to create a new instance profile with [`iam:CreateInstanceProfile`](https://docs.aws.amazon.com/IAM/latest/APIReference/API_CreateInstanceProfile.html),
provided that the request is made to a cluster with `kubernetes.io/cluster/${ClusterName` set to owned and is made in the current region.
Also, `karpenter.sh/nodeclass` must be set to some value.
```
 {
              "Sid": "AllowScopedInstanceProfileCreationActions",
              "Effect": "Allow",
              "Resource": "*",
              "Action": [
                "iam:CreateInstanceProfile"
              ],
              "Condition": {
                "StringEquals": {
                  "aws:RequestTag/kubernetes.io/cluster/${ClusterName}": "owned",
                  "aws:RequestTag/topology.kubernetes.io/region": "${AWS::Region}"
                },
                "StringLike": {
                  "aws:RequestTag/karpenter.sh/nodeclass": "*"
                }
              }
            },
```

### AllowScopedInstanceProfileTagActions
The AllowScopedInstanceProfileTagActions Sid gives the Karpenter controller permission to tag an instance profile with [`iam:TagInstanceProfile`](https://docs.aws.amazon.com/IAM/latest/APIReference/API_TagInstanceProfile.html), based on the values shown below,
Also, `karpenter.sh/nodeclass` must be set to some value.

```
            {
              "Sid": "AllowScopedInstanceProfileTagActions",
              "Effect": "Allow",
              "Resource": "*",
              "Action": [
                "iam:TagInstanceProfile"
              ],
              "Condition": {
                "StringEquals": {
                  "aws:ResourceTag/kubernetes.io/cluster/${ClusterName}": "owned",
                  "aws:ResourceTag/topology.kubernetes.io/region": "${AWS::Region}",
                  "aws:RequestTag/kubernetes.io/cluster/${ClusterName}": "owned",
                  "aws:RequestTag/topology.kubernetes.io/region": "${AWS::Region}"
                },
                "StringLike": {
                  "aws:ResourceTag/karpenter.sh/nodeclass": "*",
                  "aws:RequestTag/karpenter.sh/nodeclass": "*"
                }
              }
            },
```


### AllowScopedInstanceProfileActions
The AllowScopedInstanceProfileActions Sid gives the Karpenter controller permission to perform [`iam:AddRoleToInstanceProfile`](https://docs.aws.amazon.com/IAM/latest/APIReference/API_AddRoleToInstanceProfile.html), [`iam:RemoveRoleFromInstanceProfile`](https://docs.aws.amazon.com/IAM/latest/APIReference/API_RemoveRoleFromInstanceProfile.html), and [`iam:DeleteInstanceProfile`](https://docs.aws.amazon.com/IAM/latest/APIReference/API_DeleteInstanceProfile.html) actions, 
provided that the request is made to a cluster with `kubernetes.io/cluster/${ClusterName` set to owned and is made in the current region.
Also, `karpenter.sh/nodeclass` must be set to some value.

```
            {
              "Sid": "AllowScopedInstanceProfileActions",
              "Effect": "Allow",
              "Resource": "*",
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
                  "aws:ResourceTag/karpenter.sh/nodeclass": "*"
                }
              }
            },
```
### AllowInstanceProfileActions
The AllowInstanceProfileActions Sid gives the Karpenter controller permission to perform [`iam:GetInstanceProfile`](https://docs.aws.amazon.com/IAM/latest/APIReference/API_GetInstanceProfile.html) actions to retrieve informatio about a specified instance profile.

```
            {
              "Sid": "AllowInstanceProfileReadActions",
              "Effect": "Allow",
              "Resource": "*",
              "Action": "iam:GetInstanceProfile"
            },
            {
```

### AllowAPIServerEndpointDiscovery
The Karpenter controller needs to be able to find the Kubernetes cluster's API endpoint in order to communicate with the API server.
The AllowAPIServerEndpointDiscovery Sid allows the Karpenter controller to get that information (`eks:DescribeCluster`) for the cluster (`cluster/${ClusterName}`).
```
            {
              "Sid": "AllowAPIServerEndpointDiscovery",
              "Effect": "Allow",
              "Resource": "arn:${AWS::Partition}:eks:${AWS::Region}:${AWS::AccountId}:cluster/${ClusterName}",
              "Action": "eks:DescribeCluster"
            }
          ]
        }
```

# Interruption Handling 
Settings in this section allow the Karpenter controller to interact with interruption queues.
So, for example, if Spot instances are being reclaimed or a node crashes, seeing messages from these queues allows Karpenter to be proactive in moving workloads or adding new nodes.
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

## KarpenterInterruptionQueue
The [AWS::SQS::Queue](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-sqs-queue.html) resource is used to create an Amazon SQS standard queue.
Properties of that resource set the `QueueName` to the name of your cluster, the time for which SQS retains each message (`MessageRetentionPeriod`) to 300 seconds, and enabling serverside-side encryption using SQS owned encryption keys (`SqsManagedSseEnabled`) to `true`.
See [SetQueueAttributes](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_SetQueueAttributes.html) for descriptions of some of these attributes.

```
  KarpenterInterruptionQueue:
    Type: AWS::SQS::Queue
    Properties:
      QueueName: !Sub "${ClusterName}"
      MessageRetentionPeriod: 300
      SqsManagedSseEnabled: true
```

## KarpenterInterruptionQueuePolicy
The Karpenter interruption queue policy is created and applied to allow the Karpenter controller to see messages for selected services.
In particular, the [AWS::SQS::QueuePolicy](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-sqs-queuepolicy.html) resource here applies `EC2InterruptionPolicy` to the `KarpenterInterruptionQueue`. The policy allows [sqs:SendMessage](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_SendMessage.html) actions to `events.amazonaws.com` and `sqs.amazonaws.com` services. It also allows the `GetAtt` function to get attributes from `KarpenterInterruptionQueue.Arn`.

```
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
```

## ScheduledChangeRule
This section allows Karpenter to gather [AWS Health Events](https://docs.aws.amazon.com/health/latest/ug/cloudwatch-events-health.html#about-public-events) and direct them to a queue where they can be consumed by Karpenter.
In particular, the [AWS::Events::Rule](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-events-rule.html) here creates a rule where the [EventPattern](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-event-patterns.html) is set to send events from the `aws.health` source to `KarpenterInterruptionQueue`.

```
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

## SpotInterruptionRule
An EC2 Spot Instance Interruption warning tells you that AWS is about to reclaim a Spot instance you are using.
This section allows Karpenter to gather [EC2 Spot Instance Interruption Warning](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html) events and direct them to a queue where they can be consumed by Karpenter.
In particular, the [AWS::Events::Rule](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-events-rule.html) here creates a rule where the [EventPattern](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-event-patterns.html) is set to send events from the `aws.ec2` source to `KarpenterInterruptionQueue`.

```
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

## RebalanceRule

An EC2 Instance Rebalance Recommendation signal tells you that a Spot instance is at a heightened risk of being interrupted, allowing Karpenter to get new instances or simply rebalance workloads.
This section allows Karpenter to gather [EC2 Instance Rebalance Recommendation](https://docs.aws.amazon.com/AWSEC2/latest/WindowsGuide/rebalance-recommendations.html) signals and direct them to a queue where they can be consumed by Karpenter.
In particular, the [AWS::Events::Rule](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-events-rule.html) here creates a rule where the [EventPattern](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-event-patterns.html) is set to send events from the `aws.ec2` source to `KarpenterInterruptionQueue`.

```
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

## InstanceStateChangeRule
An EC2 Instance State-change Notification signal tells you that the state of an instance has changed to one of the following states: pending, running, stopping, stopped, shutting-down, or terminated.
This section allows Karpenter to gather [EC2 Instance State-change](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/monitoring-instance-state-changes.html) signals and direct them to a queue where they can be consumed by Karpenter.
In particular, the [AWS::Events::Rule](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-events-rule.html) here creates a rule where the [EventPattern](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-event-patterns.html) is set to send events from the `aws.ec2` source to `KarpenterInterruptionQueue`.

```
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
