---
title: "Karpenter Setup"
linkTitle: "Karpenter Setup"
weight: 5
description: >
  Descriptions of how Karpenter sets up infrastructure
---
When you create a cluster to use with Karpenter in the [Getting Started with Karpenter]({{< relref "../getting-started-with-karpenter/" >}}) guide, the procedure uses CloudFormation to prepare the cluster for Karpenter to be able to create and manage nodes.
This document describes the `cloudformation.yaml` file used in that guide.
These descriptions will be useful to understand:

* How Karpenter is integrated with your new cluster or
* What you need to do with an existing cluster if you want to add Karpenter manually.

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
Those resources include:

* KarpenterNodeInstanceProfile
* KarpenterNodeRole
* KarpenterControllerPolicy
* KarpenterInterruptionQueue
* KarpenterInterruptionQueuePolicy
* ScheduledChangeRule
* SpotInterruptionRule
* RebalanceRule
* InstanceStateChangeRule

The following text divides the `cloudformation.yaml` you just downloaded into those sections and describes them.

# Description of Karpenter cloudformation.yaml

A lot of the object naming that is done by `cloudformation.yaml` is based on the following:

* Cluster name: With a user name of `joe` the Getting Started Guide would name your cluster `joe-karpenter-demo`
That name would then be appended to any name below where `${ClusterName}` is included.

* Partition: Any time an ARN is used, it includes the partition name to identify where the object is found. In most cases, that partition name is `aws`. However, it could also be `aws-cn` (for China Regions) or `aws-us-gov` (for AWS GovCloud US Regions).


## Create instance profile (KarpenterNodeInstanceProfile)
This section creates an [EC2 Instance Profile](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html) that includes the node role named `KarpenterNodeRole`, with the cluster name appended.
For example, with a cluster name of  `joe-karpenter-demo`, the instance profile name would look like:

`KarpenterNodeInstanceProfile-joe-karpenter-demo`


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

## Create node role permissions (KarpenterNodeRole)

This section creates the node role that is attached to the `KarpenterNodeInstanceProfile` instance profile created earlier.
Given a cluster name of `joe-karpenter-demo`, this role would end up being named `"KarpenterNodeRole-joe-karpenter-demo`.

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
The role created here includes several AWS managed policies, which are designed to provide permissions for specific uses needed by Karpenter to create and manage nodes.
These include:

* [AmazonEKS_CNI_Policy](https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonEKS_CNI_Policy.html): Provides permission Amazon VPC CNI Plugin needs to configure EKS worker nodes.
* [AmazonEKSWorkerNodePolicy](https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonEKSWorkerNodePolicy.html): Lets Amazon EKS worker nodes connect to EKS Clusters.
* [AmazonEC2ContainerRegistryReadOnly](https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonEC2ContainerRegistryReadOnly.html): Allows access to repositories in the Amazon EC2 Container Registry.
* [AmazonSSMManagedInstanceCore](https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonSSMManagedInstanceCore.html): Adds AWS Systems Manager service core functions for Amazon EC2.


## KarpenterControllerPolicy

This section sets the permissions that the Karpenter Controller has to create and manage EC2 resources.
Because the scope of the KarpenterControllerPolicy is an AWS region, the cluster's AWS region is included in the AllowScopedEC2InstanceActions.

A KarpenterControllerPolicy sets the name of the policy, then defines of a set of resources and actions allowed for those resources.
For our example, the KarpenterControllerPolicy would be named: `KarpenterControllerPolicy-joe-karpenter-demo`
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

### AllowScopedEC2InstanceActions

The AllowScopedEC2InstanceActions statement ID (Sid) identifies a set of EC2 resources that are allowed to be used with
[RunInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RunInstances.html) and [CreateFleet](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateFleet.html) actions.

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

### AllowScopedEC2LaunchTemplateActions
The AllowScopedEC2LaunchTemplateActions Sid allows the [CreateLaunchTemplate](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateLaunchTemplate.html)
action for all Karpenter provisioners, provided that the request comes from the cluster owner.

```
            {
              "Sid": "AllowScopedEC2LaunchTemplateActions",
              "Effect": "Allow",
              "Resource": "arn:${AWS::Partition}:ec2:${AWS::Region}:*:launch-template/*",
              "Action": "ec2:CreateLaunchTemplate",
              "Condition": {
                "StringEquals": {
                  "aws:RequestTag/kubernetes.io/cluster/${ClusterName}": "owned"
                },
                "StringLike": {
                  "aws:RequestTag/karpenter.sh/provisioner-name": "*"
                }
              }
            },
```

### AllowScopedEC2InstanceActionsWithTags
The AllowScopedEC2InstanceActionsWithTags Sid allows the 
[RunInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RunInstances.html) and [CreateFleet](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateFleet.html)
actions for all Karpenter provisioners, provided that the request comes from the cluster owner.

```
            {
              "Sid": "AllowScopedEC2InstanceActionsWithTags",
              "Effect": "Allow",
              "Resource": [
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:fleet/*",
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:instance/*",
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:volume/*",
                "arn:${AWS::Partition}:ec2:${AWS::Region}:*:network-interface/*"
              ],
              "Action": [
                "ec2:RunInstances",
                "ec2:CreateFleet"
              ],
              "Condition": {
                "StringEquals": {
                  "aws:RequestTag/kubernetes.io/cluster/${ClusterName}": "owned"
                },
                "StringLike": {
                  "aws:RequestTag/karpenter.sh/provisioner-name": "*"
                }
              }
            },

```

### AllowScopedResourceCreationTagging
The AllowScopedResourceCreationTagging Sid allows [CreateTags](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateTags.html)
actions on fleet, instance, volume, network-interface, and launch-template resources,
 for all Karpenter provisioners, provided that the request comes from the cluster owner and that it is part of a RunInstance, CreateFleet, or CreateLaunchTemplate CreateAction.

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
                  "aws:RequestTag/karpenter.sh/provisioner-name": "*"
                }
              }
            },
```

### AllowMachineMigrationTagging
The AllowMachineMigrationTagging Sid allows [CreateTags](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateTags.html)
actions on instance resources.
The request must come from the cluster owner of the cluster and the instance must be managed by the cluster.

??? I NEED MORE INFO HERE ???

```
            {
              "Sid": "AllowMachineMigrationTagging",
              "Effect": "Allow",
              "Resource": "arn:${AWS::Partition}:ec2:${AWS::Region}:*:instance/*",
              "Action": "ec2:CreateTags",
              "Condition": {
                "StringEquals": {
                  "aws:ResourceTag/kubernetes.io/cluster/${ClusterName}": "owned",
                  "aws:RequestTag/karpenter.sh/managed-by": "${ClusterName}"
                },
                "StringLike": {
                  "aws:RequestTag/karpenter.sh/provisioner-name": "*"
                },
                "ForAllValues:StringEquals": {
                  "aws:TagKeys": [
                    "karpenter.sh/provisioner-name",
                    "karpenter.sh/managed-by"
                  ]
                }
              }
            },
```

### AllowScopedDeletion
The AllowScopedDeletion Sid allows the owner of a cluster through any provisioner name to use [TerminateInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_TerminateInstances.html) and [DeleteLaunchTemplate](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DeleteLaunchTemplate.html) actions to delete instance and launch-template resources.

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
                  "aws:ResourceTag/karpenter.sh/provisioner-name": "*"
                }
              }
            },
```

### AllowRegionalReadActions

The AllowRegionalReadActions Sid allows [DescribeAvailabilityZones](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeAvailabilityZones.html), [DescribeImages](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeImages.html), [DescribeInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html), [DescribeInstanceTypeOfferings](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstanceTypeOfferings.html), [DescribeInstanceTypes](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstanceTypes.html), [DescribeLaunchTemplates](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeLaunchTemplates.html), [DescribeSecurityGroups](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSecurityGroups.html), [DescribeSpotPriceHistory](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSpotPriceHistory.html), and [DescribeSubnets](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSubnets.html) actions for the current AWS region.

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

### AllowGlobalRead Actions

```
            {
              "Sid": "AllowGlobalReadActions",
              "Effect": "Allow",
              "Resource": "*",
              "Action": [
                "pricing:GetProducts",
                "ssm:GetParameter"
              ]
            },
```

### AllowInterruptionQueueActions

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

### AllowAPIServerEndpointDiscovery

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

## KarpenterInterruptionQueue

```
  KarpenterInterruptionQueue:
    Type: AWS::SQS::Queue
    Properties:
      QueueName: !Sub "${ClusterName}"
      MessageRetentionPeriod: 300
      SqsManagedSseEnabled: true
```

## KarpenterInterruptionQueuePolicy

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
