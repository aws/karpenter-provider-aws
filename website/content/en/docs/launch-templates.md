# Custom Images

## Introduction

Karpenter follows existing AWS patterns for customizing the base image of
instances. More specifically, Karpenter uses EC2 launch templates. Launch
templates may specify many values. The pivotal value is the base image (AMI).
Launch templates further specify many parameters related to networking,
authorization, instance type, and more.  

## Launch Template Configuration

### AMI

Use the AWS CLI to import virtual machine images as AMIs. 

### User Data - Autoconfigure

Importantly, the AMI must support automatically connecting to a cluster based
on "user data", or a base64 encoded string passed to the instance at startup.
The syntax and purpose of the user data varies between images. The Karpenter
default OS, Amazon Linux 2 (AL2), accepts shell scripts (bash commands). 

[AWS calls data passed to an instance at launch time "user
data".](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/user-data.html#user-data-shell-scripts)

In the default configuration, Karpenter uses AL2 and passes the hostname of the
Kubernetes API server, and a certificate. The instance subsequently uses this
information to securely join the cluster.

When building a custom image, you may reference AWS's [`bootstrap.sh`
file](https://github.com/awslabs/amazon-eks-ami/blob/master/files/bootstrap.sh)
from GitHub, and this associated `user data` startup script. 

```
#!/bin/bash
/etc/eks/bootstrap.sh <my-cluster-name> \
--kubelet-extra-args <'--max-pods=40'> \
--b64-cluster-ca <certificateAuthority> \
--apiserver-endpoint <endpoint> 
--dns-cluster-ip <serivceIpv4Cidr>.10
--use-max-pods false
```

Note, you must populate this startup script with live values. Karpenter will
not change the user data in the launch template. 

### Instance Type

The instance type should not be specified in the launch template. Karpenter
will determine the launch template at run time. 

### Instance Profile - IAM

The launch template must include an "instance profile" -- a set of IAM roles. 

The instance profile must include all the permissions of the default Karpenter
node instance profile. For example, permission to run containers and manage
networking. See the default role, `KarpenterNodeRole`, in the full example
below for more information. 

### Storage

Karpenter expects nothing of node storage. Configure as needed for your base
image.

### Security Groups - Firewall

EKS configures security groups (i.e., instance firewall rules) automatically. 

However, you may manually specify a security group. The security group must
permit communication with EKS control plane. Outbound access should be
permitted for at least: HTTPS on port 443, DNS (UDP and TCP) on port 53, and
your subnet's network access control list (network ACL). 

The security group must be associated with the virtual private cloud (VPC) of
the EKS cluster.

### Network Interfaces

EKS will configure the network interfaces. Do not configure network instances
in the launch template.

## Creating the Launch Template

Launch Templates may be created via the web console, the AWS CLI, or
CloudFormation. 

### CloudFormation


An example yaml cloudformation definition of a launch template for Karpenter is
provided below. 

Cloudformation yaml is suited for the moderately high configuration density of
launch templates, and creating the unusual InstanceProfile resource. 

```yaml
AWSTemplateFormatVersion: '2010-09-09'
Resources:
  # create InstanceProfile wrapper on NodeRole
  KarpenterNodeInstanceProfile:
    Type: "AWS::IAM::InstanceProfile"
    Properties:
      InstanceProfileName: "KarpenterNodeInstanceProfile"
      Path: "/"
      Roles:
        - Ref: "KarpenterNodeRole"
  # create role with basic permissions for EKS node
  KarpenterNodeRole:
    Type: "AWS::IAM::Role"
    Properties:
      RoleName: "KarpenterNodeRole"
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
        - !Sub "arn:${AWS::Partition}:iam::aws:policy/AmazonEKSWorkerNodePolicy"
        - !Sub "arn:${AWS::Partition}:iam::aws:policy/AmazonEKS_CNI_Policy"
        - !Sub "arn:${AWS::Partition}:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
        - !Sub "arn:${AWS::Partition}:iam::aws:policy/AmazonSSMManagedInstanceCore"
  MyLaunchTemplate:
    Type: AWS::EC2::LaunchTemplate
    Properties: 
      LaunchTemplateData: 
        IamInstanceProfile:
          # Get ARN of InstanceProfile defined above
          Arn: !GetAtt
            - KarpenterInstanceProfile
            - Arn
        ImageId: ami-074cce78125f09d61
        # UserData is Base64 Encoded
        UserData:    "IyEvYmluL2Jhc2gKL2V0Yy9la3MvYm9vdHN0cmFwLnNoIDxteS1jbHVzdGVyLW5hbWU+IFwKLS1rdWJlbGV0LWV4dHJhLWFyZ3MgPCctLW1heC1wb2RzPTQwJz4gXAotLWI2NC1jbHVzdGVyLWNhIDxjZXJ0aWZpY2F0ZUF1dGhvcml0eT4gXAotLWFwaXNlcnZlci1lbmRwb2ludCA8ZW5kcG9pbnQ+IAotLWRucy1jbHVzdGVyLWlwIDxzZXJpdmNlSXB2NENpZHI+LjEwCi0tdXNlLW1heC1wb2RzIGZhbHNl"
        BlockDeviceMappings: 
          - Ebs:
              VolumeSize: 80
              VolumeType: gp3
            DeviceName: /dev/xvda
        # The SecurityGroup must be associated with the cluster VPC
        SecurityGroupIds:
          - sg-a69adfdb
      LaunchTemplateName: KarpenterCustomLaunchTemplate
```

Create the Launch Template by uploading the CloudFormation yaml file. The
sample yaml creates an IAM Object (InstanceProfile), so `--capabilities
CAPABILITY_NAMED_IAM` must be indicated.

```
aws cloudformation create-stack \
  --stack-name KarpenterLaunchTemplateStack \
  --template-body file:///Users/gcline/Desktop/lt-cfn-demo.yaml \
  --capabilities CAPABILITY_NAMED_IAM
```

### Define LaunchTemplate for Provisioner

The LaunchTemplate is ready to be used. Specify it by name in the Provisioner
CRD. Karpenter will use this template when creating new instances.

```yaml
apiVersion: karpenter.sh/v1alpha5
kind: Provisioner
spec:
  provider:
    launchTemplate: CustomKarpenterLaunchTemplateDemo
    
```


